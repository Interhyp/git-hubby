package orgrec

import (
	"context"
	"errors"
	"net/http"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/config"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	"github.com/google/go-github/v92/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ReconcileRulesets with inline rulesets", func() {
	var (
		ctx              context.Context
		mockClient       *ghclientmock.MockGitHubClientWrapper
		rec              *GitHubOrgReconciler
		idResolver       *mockIdResolver
		org              *v1alpha1.Organization
		preset           *v1alpha1.RulesetPreset
		features         config.Features
		err              error
		existingRulesets []*github.RepositoryRuleset
		getAllCalled     bool
		createdRulesets  []*github.RepositoryRuleset
		updatedRulesets  map[int64]*github.RepositoryRuleset
		deletedIDs       []int64
	)

	inlineSpec := func(name string) v1alpha1.RulesetPresetSpec {
		return v1alpha1.RulesetPresetSpec{
			Name:        name,
			Target:      "branch",
			Enforcement: "active",
			Conditions: &v1alpha1.RulesetConditions{
				RefName: &v1alpha1.RefNameCondition{Include: []string{"~DEFAULT_BRANCH"}},
			},
			Rules: v1alpha1.RulesetRules{Deletion: new(true)},
		}
	}

	matchingGitHubRuleset := func(id int64, name string) *github.RepositoryRuleset {
		return &github.RepositoryRuleset{
			ID:          new(id),
			Name:        name,
			Enforcement: github.RulesetEnforcement("active"),
			Target:      new(github.RulesetTargetBranch),
			Conditions: &github.RepositoryRulesetConditions{
				RefName: &github.RepositoryRulesetRefConditionParameters{
					Include: []string{"~DEFAULT_BRANCH"},
					Exclude: []string{},
				},
				RepositoryName: &github.RepositoryRulesetRepositoryNamesConditionParameters{
					Include:   []string{"~ALL"},
					Exclude:   []string{},
					Protected: new(false),
				},
			},
			Rules: &github.RepositoryRulesetRules{Deletion: &github.EmptyRuleParameters{}},
		}
	}

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()
		idResolver = &mockIdResolver{}
		features = config.Features{}
		org = &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{Name: "test-org", Namespace: "default"},
			Spec: v1alpha1.OrganizationSpec{
				Name:                    "test-org",
				GitHubAppInstallationId: new(int64(12345)),
			},
		}
		preset = &v1alpha1.RulesetPreset{
			ObjectMeta: metav1.ObjectMeta{Name: "preset-1", Namespace: "default"},
			Spec:       inlineSpec("preset-ruleset"),
		}
		existingRulesets = nil
		getAllCalled = false
		createdRulesets = nil
		updatedRulesets = map[int64]*github.RepositoryRuleset{}
		deletedIDs = nil
	})

	JustBeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(org, preset).Build()

		rec = &GitHubOrgReconciler{
			GitHub:     reconciler.GitHub[string]{Client: mockClient, Resource: "test-org"},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{Client: k8sClient, Resource: org},
			IdResolver: idResolver,
			Features:   features,
		}

		mockClient.GetAllOrganizationRulesetsFunc = func(_ context.Context, _ string, _ bool) ([]*github.RepositoryRuleset, error) {
			getAllCalled = true
			return existingRulesets, nil
		}
		mockClient.GetOrganizationRulesetFunc = func(_ context.Context, _ string, id int64) (*github.RepositoryRuleset, error) {
			for _, rs := range existingRulesets {
				if rs.ID != nil && *rs.ID == id {
					return rs, nil
				}
			}
			return nil, &github.ErrorResponse{Response: &http.Response{StatusCode: http.StatusNotFound}}
		}
		mockClient.CreateOrganizationRulesetFunc = func(_ context.Context, _ string, rs *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
			createdRulesets = append(createdRulesets, rs)
			return rs, nil
		}
		mockClient.UpdateOrganizationRulesetFunc = func(_ context.Context, _ string, id int64, rs *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
			updatedRulesets[id] = rs
			return rs, nil
		}
		mockClient.DeleteOrganizationRulesetFunc = func(_ context.Context, _ string, id int64) error {
			deletedIDs = append(deletedIDs, id)
			return nil
		}

		err = rec.reconcileRulesets(ctx)
	})

	Context("when an inline ruleset does not exist on GitHub", func() {
		BeforeEach(func() {
			org.Spec.Rulesets = []v1alpha1.RulesetPresetSpec{inlineSpec("inline-1")}
		})

		It("creates it with the default repository name condition", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Name).To(Equal("inline-1"))
			Expect(createdRulesets[0].Conditions.RepositoryName).NotTo(BeNil())
			Expect(createdRulesets[0].Conditions.RepositoryName.Include).To(ConsistOf("~ALL"))
			Expect(updatedRulesets).To(BeEmpty())
			Expect(deletedIDs).To(BeEmpty())
		})

		It("does not modify the organization spec", func() {
			Expect(org.Spec.Rulesets[0].Conditions.RepositoryName).To(BeNil())
		})
	})

	Context("when an inline ruleset differs from GitHub", func() {
		BeforeEach(func() {
			spec := inlineSpec("inline-1")
			spec.Enforcement = "evaluate"
			org.Spec.Rulesets = []v1alpha1.RulesetPresetSpec{spec}
			existingRulesets = []*github.RepositoryRuleset{matchingGitHubRuleset(100, "inline-1")}
		})

		It("updates it", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedRulesets).To(HaveKey(int64(100)))
			Expect(updatedRulesets[100].Enforcement).To(Equal(github.RulesetEnforcement("evaluate")))
			Expect(createdRulesets).To(BeEmpty())
		})
	})

	Context("when an inline ruleset matches GitHub", func() {
		BeforeEach(func() {
			org.Spec.Rulesets = []v1alpha1.RulesetPresetSpec{inlineSpec("inline-1")}
			existingRulesets = []*github.RepositoryRuleset{matchingGitHubRuleset(100, "inline-1")}
		})

		It("does nothing", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(BeEmpty())
			Expect(updatedRulesets).To(BeEmpty())
			Expect(deletedIDs).To(BeEmpty())
		})
	})

	Context("when inline rulesets and presets are combined", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{{Name: "preset-1"}}
			org.Spec.Rulesets = []v1alpha1.RulesetPresetSpec{inlineSpec("inline-1")}
		})

		It("creates the union", func() {
			Expect(err).NotTo(HaveOccurred())
			names := []string{}
			for _, rs := range createdRulesets {
				names = append(names, rs.Name)
			}
			Expect(names).To(ConsistOf("preset-ruleset", "inline-1"))
		})
	})

	Context("when an inline ruleset was removed from the spec", func() {
		BeforeEach(func() {
			org.Spec.Rulesets = nil
			existingRulesets = []*github.RepositoryRuleset{matchingGitHubRuleset(100, "inline-1")}
		})

		It("deletes the orphaned ruleset", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deletedIDs).To(ConsistOf(int64(100)))
		})
	})

	Context("when an inline ruleset has target 'repository'", func() {
		BeforeEach(func() {
			spec := inlineSpec("inline-repo")
			spec.Target = "repository"
			org.Spec.Rulesets = []v1alpha1.RulesetPresetSpec{spec}
		})

		It("applies it at organization level", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(*createdRulesets[0].Target).To(Equal(github.RulesetTargetRepository))
		})
	})

	Context("required reviewers feature flag", func() {
		BeforeEach(func() {
			spec := inlineSpec("inline-1")
			spec.Rules.PullRequest = &v1alpha1.PullRequestRule{
				RequiredReviewers: []v1alpha1.RequiredPullRequestReviewer{
					{Reviewer: v1alpha1.PullRequestReviewerEntity{ID: new(int64(1)), Type: "Team"}, FilePatterns: []string{"*"}},
				},
			}
			org.Spec.Rulesets = []v1alpha1.RulesetPresetSpec{spec}
		})

		Context("when disabled", func() {
			It("strips required reviewers from inline rulesets without modifying the spec", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(idResolver.ResolveRulesetCalls).To(HaveLen(1))
				Expect(idResolver.ResolveRulesetCalls[0].Rules.PullRequest.RequiredReviewers).To(BeNil())
				Expect(org.Spec.Rulesets[0].Rules.PullRequest.RequiredReviewers).To(HaveLen(1))
			})
		})

		Context("when enabled", func() {
			BeforeEach(func() {
				features.EnableRequiredReviewersRules = true
			})

			It("keeps required reviewers", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(idResolver.ResolveRulesetCalls).To(HaveLen(1))
				Expect(idResolver.ResolveRulesetCalls[0].Rules.PullRequest.RequiredReviewers).To(HaveLen(1))
			})
		})
	})

	Context("when an inline ruleset conflicts with a preset", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{{Name: "preset-1"}}
			conflicting := inlineSpec("preset-ruleset")
			conflicting.Enforcement = "evaluate"
			org.Spec.Rulesets = []v1alpha1.RulesetPresetSpec{conflicting}
			existingRulesets = []*github.RepositoryRuleset{matchingGitHubRuleset(100, "orphan")}
		})

		It("returns a conflict error and makes no GitHub calls", func() {
			Expect(err).To(HaveOccurred())
			var conflictErr *reconciler.RulesetConflictError
			Expect(errors.As(err, &conflictErr)).To(BeTrue())
			Expect(conflictErr.Sources).To(ConsistOf("rulesetPreset/preset-1", "inline[0]"))
			Expect(getAllCalled).To(BeFalse())
			Expect(createdRulesets).To(BeEmpty())
			Expect(updatedRulesets).To(BeEmpty())
			Expect(deletedIDs).To(BeEmpty())
		})
	})

	Context("when an inline ruleset is identical to a preset", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{{Name: "preset-1"}}
			org.Spec.Rulesets = []v1alpha1.RulesetPresetSpec{inlineSpec("preset-ruleset")}
		})

		It("applies the ruleset once", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Name).To(Equal("preset-ruleset"))
		})
	})
})
