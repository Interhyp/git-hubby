package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Interhyp/git-hubby/internal/config"
	"github.com/Interhyp/git-hubby/internal/ghclient"
	"github.com/Interhyp/git-hubby/internal/reconciler/common/idresolver"
	"github.com/Interhyp/git-hubby/internal/reconciler/common/rulesets"
	"github.com/Interhyp/git-hubby/internal/reconciler/reporec"
	"github.com/Interhyp/git-hubby/internal/reconciler/spreading"
	"github.com/gofri/go-github-ratelimit/v2/github_ratelimit/github_primary_ratelimit"
	"github.com/google/go-github/v93/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("RequeuePolicy", func() {
	var (
		ctx    context.Context
		policy *RequeuePolicy
	)

	BeforeEach(func() {
		ctx = context.Background()
		policy = &RequeuePolicy{successInterval: 3 * time.Hour, gitHubStateInterval: 15 * time.Minute}
	})

	Describe("NewRequeuePolicy", func() {
		It("takes the intervals from the config", func() {
			p := NewRequeuePolicy(config.Config{SpreadIntervalMinutes: 180, GitHubStateRequeueIntervalMinutes: 15})

			Expect(p.successInterval).To(Equal(180 * time.Minute))
			Expect(p.gitHubStateInterval).To(Equal(15 * time.Minute))
		})
	})

	Describe("OnSuccess", func() {
		It("requeues after the success interval", func() {
			Expect(policy.OnSuccess()).To(Equal(ctrl.Result{RequeueAfter: 3 * time.Hour}))
		})
	})

	Describe("OnFactoryError", func() {
		Context("with rate limit errors", func() {
			It("should handle library rate limit error", func() {
				resetTime := time.Now().Add(10 * time.Minute)
				err := &github_primary_ratelimit.RateLimitReachedError{
					ResetTime: &resetTime,
				}

				result, returnErr := policy.OnFactoryError(ctx, err)

				Expect(returnErr).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(BeNumerically("~", time.Until(resetTime), time.Second))
			})

			It("should handle custom rate limit error", func() {
				resetTime := time.Now().Add(5 * time.Minute)
				err := &ghclient.RateLimitedError{
					ResetTime: resetTime,
				}

				result, returnErr := policy.OnFactoryError(ctx, err)

				Expect(returnErr).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(BeNumerically("~", time.Until(resetTime), time.Second))
			})

			It("should prefer library rate limit error over custom error", func() {
				resetTime1 := time.Now().Add(10 * time.Minute)
				resetTime2 := time.Now().Add(5 * time.Minute)

				libErr := &github_primary_ratelimit.RateLimitReachedError{
					ResetTime: &resetTime1,
				}
				customErr := &ghclient.RateLimitedError{
					ResetTime: resetTime2,
				}
				err := errors.Join(libErr, customErr)

				result, returnErr := policy.OnFactoryError(ctx, err)

				Expect(returnErr).ToNot(HaveOccurred())
				// Should use the library error's reset time
				Expect(result.RequeueAfter).To(BeNumerically("~", time.Until(resetTime1), time.Second))
			})

			It("should apply minimum backoff when reset time is in the past", func() {
				resetTime := time.Now().Add(-1 * time.Minute)
				err := &ghclient.RateLimitedError{
					ResetTime: resetTime,
				}

				result, returnErr := policy.OnFactoryError(ctx, err)

				Expect(returnErr).ToNot(HaveOccurred())
				// Minimum backoff applied to avoid tight-loop
				Expect(result.RequeueAfter).To(Equal(minRateLimitRequeueDelay))
			})
		})

		Context("with spreading errors", func() {
			It("should handle RequiresSpreadError", func() {
				spreadErr := &spreading.RequiresSpreadError{
					RequeueAfter: 15 * time.Minute,
				}

				result, returnErr := policy.OnFactoryError(ctx, spreadErr)

				Expect(returnErr).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(15 * time.Minute))
			})

			It("should handle wrapped RequiresSpreadError", func() {
				spreadErr := &spreading.RequiresSpreadError{
					RequeueAfter: 20 * time.Minute,
				}
				wrappedErr := errors.Join(errors.New("some context"), spreadErr)

				result, returnErr := policy.OnFactoryError(ctx, wrappedErr)

				Expect(returnErr).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(20 * time.Minute))
			})
		})

		Context("with rate limit error taking precedence over spreading", func() {
			It("should handle rate limit error first", func() {
				resetTime := time.Now().Add(10 * time.Minute)
				rateLimitErr := &github_primary_ratelimit.RateLimitReachedError{
					ResetTime: &resetTime,
				}
				spreadErr := &spreading.RequiresSpreadError{
					RequeueAfter: 5 * time.Minute,
				}
				err := errors.Join(rateLimitErr, spreadErr)

				result, returnErr := policy.OnFactoryError(ctx, err)

				Expect(returnErr).ToNot(HaveOccurred())
				// Should use rate limit reset time, not spreading delay
				Expect(result.RequeueAfter).To(BeNumerically("~", time.Until(resetTime), time.Second))
			})
		})

		Context("with generic errors", func() {
			It("should return generic error for exponential backoff", func() {
				genericErr := errors.New("some reconciliation error")

				result, returnErr := policy.OnFactoryError(ctx, genericErr)

				Expect(returnErr).To(Equal(genericErr))
				Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			})

			It("should return wrapped generic error", func() {
				innerErr := errors.New("inner error")
				outerErr := errors.New("outer error")
				wrappedErr := errors.Join(outerErr, innerErr)

				result, returnErr := policy.OnFactoryError(ctx, wrappedErr)

				Expect(returnErr).To(Equal(wrappedErr))
				Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			})
		})

		Context("with nil error", func() {
			It("should return empty result with no error", func() {
				result, returnErr := policy.OnFactoryError(ctx, nil)

				Expect(returnErr).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))
			})
		})

		Context("with canceled context (shutdown)", func() {
			It("should return empty result without error when context is canceled", func() {
				cancelCtx, cancel := context.WithCancel(ctx)
				cancel() // simulate SIGTERM shutdown

				result, returnErr := policy.OnFactoryError(cancelCtx, errors.New("context canceled"))

				Expect(returnErr).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))
			})

			It("should suppress rate limit error when context is canceled", func() {
				cancelCtx, cancel := context.WithCancel(ctx)
				cancel()

				resetTime := time.Now().Add(10 * time.Minute)
				err := &github_primary_ratelimit.RateLimitReachedError{
					ResetTime: &resetTime,
				}

				result, returnErr := policy.OnFactoryError(cancelCtx, err)

				Expect(returnErr).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))
			})

			It("should suppress spreading error when context is canceled", func() {
				cancelCtx, cancel := context.WithCancel(ctx)
				cancel()

				result, returnErr := policy.OnFactoryError(cancelCtx, &spreading.RequiresSpreadError{RequeueAfter: 15 * time.Minute})

				Expect(returnErr).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))
			})
		})

		Context("with deadline exceeded context (timeout)", func() {
			It("should NOT suppress error when context deadline exceeded", func() {
				deadlineCtx, cancel := context.WithDeadline(ctx, time.Now().Add(-1*time.Second))
				defer cancel()

				genericErr := errors.New("context deadline exceeded")

				result, returnErr := policy.OnFactoryError(deadlineCtx, genericErr)

				Expect(returnErr).To(Equal(genericErr))
				Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			})

			It("should still handle rate limit error when deadline exceeded", func() {
				deadlineCtx, cancel := context.WithDeadline(ctx, time.Now().Add(-1*time.Second))
				defer cancel()

				resetTime := time.Now().Add(10 * time.Minute)
				err := &github_primary_ratelimit.RateLimitReachedError{
					ResetTime: &resetTime,
				}

				result, returnErr := policy.OnFactoryError(deadlineCtx, err)

				Expect(returnErr).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(BeNumerically("~", time.Until(resetTime), time.Second))
			})
		})
	})

	Describe("OnReconcileError", func() {
		It("requeues after the success interval for errors fixable in Kubernetes resources only", func() {
			err := fmt.Errorf("rulesets: %w", errors.Join(
				&rulesets.UnresolvedError{Ruleset: "a", Reason: rulesets.ReasonParentNotFound},
				&rulesets.ConflictError{Name: "b"},
				&idresolver.UnresolvableReferenceError{Message: "invalid bypass actor"},
			))

			result, returnErr := policy.OnReconcileError(ctx, err)

			Expect(returnErr).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{RequeueAfter: 3 * time.Hour}))
		})

		DescribeTable("requeues after the GitHub state interval if any error awaits a change on GitHub",
			func(gitHubErr error) {
				err := fmt.Errorf("sync: %w", errors.Join(&rulesets.ConflictError{Name: "b"}, gitHubErr))

				result, returnErr := policy.OnReconcileError(ctx, err)

				Expect(returnErr).NotTo(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{RequeueAfter: 15 * time.Minute}))
			},
			Entry("missing default branch", &reporec.DefaultBranchNotFoundError{Branch: "develop"}),
			Entry("missing team", &idresolver.UnresolvableReferenceError{Message: "team not found", NotFound: true}),
			Entry("GitHub 422", &github.ErrorResponse{Response: &http.Response{StatusCode: http.StatusUnprocessableEntity}, Message: "Validation Failed"}),
			Entry("missing referenced Kubernetes resource (watched or not)",
				fmt.Errorf("failed to get webhook preset x: %w", apierrors.NewNotFound(schema.GroupResource{Group: "github.interhyp.de", Resource: "webhookpresets"}, "x"))),
		)

		It("caps the GitHub state interval at the success interval and ignores unset values", func() {
			err := &reporec.DefaultBranchNotFoundError{Branch: "develop"}

			result, _ := (&RequeuePolicy{successInterval: 10 * time.Minute, gitHubStateInterval: 15 * time.Minute}).OnReconcileError(ctx, err)
			Expect(result.RequeueAfter).To(Equal(10 * time.Minute))

			result, _ = (&RequeuePolicy{successInterval: 3 * time.Hour}).OnReconcileError(ctx, err)
			Expect(result.RequeueAfter).To(Equal(3 * time.Hour))
		})

		It("returns other errors for a requeue with exponential backoff", func() {
			for _, err := range []error{
				errors.New("boom"),
				&rulesets.UnresolvedError{Ruleset: "a", Reason: rulesets.ReasonParentUnavailable},
				errors.Join(&rulesets.UnresolvedError{Ruleset: "a", Reason: rulesets.ReasonParentNotFound}, errors.New("boom")),
				errors.Join(&reporec.DefaultBranchNotFoundError{Branch: "develop"}, errors.New("boom")),
				&github.ErrorResponse{Response: &http.Response{StatusCode: http.StatusBadGateway}},
			} {
				result, returnErr := policy.OnReconcileError(ctx, err)

				Expect(returnErr).To(BeIdenticalTo(err), "error: %v", err)
				Expect(result).To(Equal(ctrl.Result{}), "error: %v", err)
			}
		})

		It("handles rate limits like factory errors", func() {
			resetTime := time.Now().Add(10 * time.Minute)

			result, returnErr := policy.OnReconcileError(ctx, &ghclient.RateLimitedError{ResetTime: resetTime})

			Expect(returnErr).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically("~", time.Until(resetTime), time.Second))
		})
	})
})
