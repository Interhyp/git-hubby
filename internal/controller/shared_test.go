package controller

import (
	"context"
	"errors"
	"time"

	"github.com/Interhyp/git-hubby/internal/ghclient"
	"github.com/Interhyp/git-hubby/internal/ratelimit"
	"github.com/Interhyp/git-hubby/internal/reconciler/spreading"
	"github.com/gofri/go-github-ratelimit/v2/github_ratelimit/github_primary_ratelimit"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

var _ = Describe("Shared Controller Functions", func() {
	var (
		ctx     context.Context
		limiter *ratelimit.GitHubRateLimiter
	)

	BeforeEach(func() {
		ctx = context.Background()
		limiter = ratelimit.NewGitHubRateLimiter(ratelimit.GitHubRateLimiterConfig{
			RequestsPerHour: 5000,
			BurstSize:       100,
			EnableBlocking:  true,
		})
	})

	Describe("handleRequeueError", func() {
		Context("with rate limit errors", func() {
			It("should handle library rate limit error", func() {
				resetTime := time.Now().Add(10 * time.Minute)
				err := &github_primary_ratelimit.RateLimitReachedError{
					ResetTime: &resetTime,
				}

				result, returnErr := handleRequeueError(ctx, err, limiter)

				Expect(returnErr).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(BeNumerically("~", time.Until(resetTime), time.Second))
			})

			It("should handle custom rate limit error", func() {
				resetTime := time.Now().Add(5 * time.Minute)
				err := &ghclient.RateLimitedError{
					ResetTime: resetTime,
				}

				result, returnErr := handleRequeueError(ctx, err, limiter)

				Expect(returnErr).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(BeNumerically("~", time.Until(resetTime), time.Second))
			})

			It("should prefer library rate limit error over custom error", func() {
				resetTime1 := time.Now().Add(10 * time.Minute)
				resetTime2 := time.Now().Add(5 * time.Minute)

				// Wrap both errors
				libErr := &github_primary_ratelimit.RateLimitReachedError{
					ResetTime: &resetTime1,
				}
				customErr := &ghclient.RateLimitedError{
					ResetTime: resetTime2,
				}
				err := errors.Join(libErr, customErr)

				result, returnErr := handleRequeueError(ctx, err, limiter)

				Expect(returnErr).ToNot(HaveOccurred())
				// Should use the library error's reset time
				Expect(result.RequeueAfter).To(BeNumerically("~", time.Until(resetTime1), time.Second))
			})
		})

		Context("with spreading errors", func() {
			It("should handle RequiresSpreadError", func() {
				spreadErr := &spreading.RequiresSpreadError{
					RequeueAfter: 15 * time.Minute,
				}

				result, returnErr := handleRequeueError(ctx, spreadErr, limiter)

				Expect(returnErr).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(15 * time.Minute))
			})

			It("should handle wrapped RequiresSpreadError", func() {
				spreadErr := &spreading.RequiresSpreadError{
					RequeueAfter: 20 * time.Minute,
				}
				wrappedErr := errors.Join(errors.New("some context"), spreadErr)

				result, returnErr := handleRequeueError(ctx, wrappedErr, limiter)

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

				result, returnErr := handleRequeueError(ctx, err, limiter)

				Expect(returnErr).ToNot(HaveOccurred())
				// Should use rate limit reset time, not spreading delay
				Expect(result.RequeueAfter).To(BeNumerically("~", time.Until(resetTime), time.Second))
			})
		})

		Context("with generic errors", func() {
			It("should return generic error for exponential backoff", func() {
				genericErr := errors.New("some reconciliation error")

				result, returnErr := handleRequeueError(ctx, genericErr, limiter)

				Expect(returnErr).To(Equal(genericErr))
				Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			})

			It("should return wrapped generic error", func() {
				innerErr := errors.New("inner error")
				outerErr := errors.New("outer error")
				wrappedErr := errors.Join(outerErr, innerErr)

				result, returnErr := handleRequeueError(ctx, wrappedErr, limiter)

				Expect(returnErr).To(Equal(wrappedErr))
				Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			})
		})

		Context("with nil error", func() {
			It("should return empty result with no error", func() {
				result, returnErr := handleRequeueError(ctx, nil, limiter)

				Expect(returnErr).ToNot(HaveOccurred())
				Expect(result).To(Equal(controllerruntime.Result{}))
			})
		})

		Context("with canceled context (shutdown)", func() {
			It("should return empty result without error when context is canceled", func() {
				cancelCtx, cancel := context.WithCancel(ctx)
				cancel() // simulate SIGTERM shutdown

				genericErr := errors.New("context canceled")

				result, returnErr := handleRequeueError(cancelCtx, genericErr, limiter)

				Expect(returnErr).ToNot(HaveOccurred())
				Expect(result).To(Equal(controllerruntime.Result{}))
			})

			It("should suppress rate limit error when context is canceled", func() {
				cancelCtx, cancel := context.WithCancel(ctx)
				cancel()

				resetTime := time.Now().Add(10 * time.Minute)
				err := &github_primary_ratelimit.RateLimitReachedError{
					ResetTime: &resetTime,
				}

				result, returnErr := handleRequeueError(cancelCtx, err, limiter)

				Expect(returnErr).ToNot(HaveOccurred())
				Expect(result).To(Equal(controllerruntime.Result{}))
			})

			It("should suppress spreading error when context is canceled", func() {
				cancelCtx, cancel := context.WithCancel(ctx)
				cancel()

				spreadErr := &spreading.RequiresSpreadError{
					RequeueAfter: 15 * time.Minute,
				}

				result, returnErr := handleRequeueError(cancelCtx, spreadErr, limiter)

				Expect(returnErr).ToNot(HaveOccurred())
				Expect(result).To(Equal(controllerruntime.Result{}))
			})
		})

		Context("with deadline exceeded context (timeout)", func() {
			It("should NOT suppress error when context deadline exceeded", func() {
				deadlineCtx, cancel := context.WithDeadline(ctx, time.Now().Add(-1*time.Second))
				defer cancel()

				genericErr := errors.New("context deadline exceeded")

				result, returnErr := handleRequeueError(deadlineCtx, genericErr, limiter)

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

				result, returnErr := handleRequeueError(deadlineCtx, err, limiter)

				Expect(returnErr).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(BeNumerically("~", time.Until(resetTime), time.Second))
			})
		})

		Context("rate limit error with past reset time", func() {
			It("should handle reset time in the past gracefully", func() {
				resetTime := time.Now().Add(-1 * time.Minute)
				err := &ghclient.RateLimitedError{
					ResetTime: resetTime,
				}

				result, returnErr := handleRequeueError(ctx, err, limiter)

				Expect(returnErr).ToNot(HaveOccurred())
				// RequeueAfter will be negative, which controller-runtime interprets as immediate
				Expect(result.RequeueAfter).To(BeNumerically("<", 0))
			})
		})
	})

	Describe("resourceWasDeleted", func() {
		// This is a simple helper function already well-tested by integration tests
		// but we can add basic unit tests for completeness
		It("is tested via controller integration tests", func() {
			// The resourceWasDeleted function is thoroughly tested in the controller
			// integration tests where finalizer and deletion timestamp logic is exercised
			// See organization_controller_test.go, repository_controller_test.go, team_controller_test.go
			Skip("Covered by controller integration tests")
		})
	})

	Describe("getRequeueDuration", func() {
		It("is tested via controller integration tests", func() {
			// The getRequeueDuration function is straightforward and tested via
			// integration tests where reconciliation loops verify proper requeue behavior
			Skip("Covered by controller integration tests")
		})
	})
})
