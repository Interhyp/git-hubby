package orgrec

import (
	"context"
	"errors"
	"net/http"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/config"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/internal/reconciler/common/rulesets"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	"github.com/google/go-github/v92/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// The generic ruleset sync logic (create/update/delete, diffing, error handling, feature flags) is tested in
// internal/reconciler/rulesets_test.go. These tests only cover the organization specific wiring.
var _ = Describe("ReconcileRulesets", func() {
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
		getAllError      error
		getCalls         []int64
		createdRulesets  []*github.RepositoryRuleset
		updatedRulesets  map[int64]*github.RepositoryRuleset
		deletedIDs       []int64
	)

	rulesetSpec := func(name string) v1alpha1.RulesetSpec {
		return v1alpha1.RulesetSpec{
			Name:        name,
			Target:      "branch",
			Enforcement: "active",
			Conditions: &v1alpha1.RulesetConditions{
				RefName: &v1alpha1.RefNameCondition{Include: []string{"~DEFAULT_BRANCH"}},
			},
			Rules: v1alpha1.RulesetRules{Deletion: new(true)},
		}
	}

	// matchingGitHubRuleset matches rulesetSpec(name) including the default ~ALL repository name condition.
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

	createdNames := func() []string {
		names := []string{}
		for _, rs := range createdRulesets {
			names = append(names, rs.Name)
		}
		return names
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
			Spec:       rulesetSpec("preset-ruleset"),
		}
		existingRulesets = nil
		getAllCalled = false
		getAllError = nil
		getCalls = nil
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
			return existingRulesets, getAllError
		}
		mockClient.GetOrganizationRulesetFunc = func(_ context.Context, _ string, id int64) (*github.RepositoryRuleset, error) {
			getCalls = append(getCalls, id)
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

	Context("when plan is 'free'", func() {
		BeforeEach(func() {
			org.Spec.Plan = "free"
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{{Name: "preset-1"}}
			existingRulesets = []*github.RepositoryRuleset{matchingGitHubRuleset(100, "orphan")}
		})

		It("skips reconciliation without any GitHub calls", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(getAllCalled).To(BeFalse())
			Expect(createdRulesets).To(BeEmpty())
			Expect(updatedRulesets).To(BeEmpty())
			Expect(deletedIDs).To(BeEmpty())
		})
	})

	Context("when an inline ruleset does not exist on GitHub", func() {
		BeforeEach(func() {
			org.Spec.Rulesets = []v1alpha1.RulesetSpec{rulesetSpec("inline-1")}
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

	Context("when inline rulesets and presets are combined", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{{Name: "preset-1"}}
			org.Spec.Rulesets = []v1alpha1.RulesetSpec{rulesetSpec("inline-1")}
		})

		It("creates the union", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdNames()).To(ConsistOf("preset-ruleset", "inline-1"))
		})
	})

	Context("when an existing ruleset matches including the default repository name condition", func() {
		BeforeEach(func() {
			org.Spec.Rulesets = []v1alpha1.RulesetSpec{rulesetSpec("inline-1")}
			existingRulesets = []*github.RepositoryRuleset{matchingGitHubRuleset(100, "inline-1")}
		})

		It("fetches the organization ruleset and does nothing", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(getCalls).To(ConsistOf(int64(100)))
			Expect(createdRulesets).To(BeEmpty())
			Expect(updatedRulesets).To(BeEmpty())
			Expect(deletedIDs).To(BeEmpty())
		})
	})

	Context("when rulesets need to be created, updated and deleted", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{{Name: "preset-1"}}
			org.Spec.Rulesets = []v1alpha1.RulesetSpec{rulesetSpec("inline-1")}
			outdated := matchingGitHubRuleset(200, "preset-ruleset")
			outdated.Enforcement = github.RulesetEnforcement("evaluate")
			existingRulesets = []*github.RepositoryRuleset{outdated, matchingGitHubRuleset(300, "orphan")}
		})

		It("uses the organization ruleset API", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdNames()).To(ConsistOf("inline-1"))
			Expect(updatedRulesets).To(HaveLen(1))
			Expect(updatedRulesets[200].Enforcement).To(Equal(github.RulesetEnforcement("active")))
			Expect(deletedIDs).To(ConsistOf(int64(300)))
		})
	})

	Context("when an inline ruleset has target 'repository'", func() {
		BeforeEach(func() {
			spec := rulesetSpec("inline-repo")
			spec.Target = "repository"
			org.Spec.Rulesets = []v1alpha1.RulesetSpec{spec}
		})

		It("applies it at organization level", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(*createdRulesets[0].Target).To(Equal(github.RulesetTargetRepository))
		})
	})

	Context("required reviewers feature flag", func() {
		BeforeEach(func() {
			spec := rulesetSpec("inline-1")
			spec.Rules.PullRequest = &v1alpha1.PullRequestRule{
				RequiredReviewers: []v1alpha1.RequiredPullRequestReviewer{
					{Reviewer: v1alpha1.PullRequestReviewerEntity{ID: new(int64(1)), Type: "Team"}, FilePatterns: []string{"*"}},
				},
			}
			org.Spec.Rulesets = []v1alpha1.RulesetSpec{spec}
		})

		Context("when disabled", func() {
			It("strips required reviewers without modifying the spec", func() {
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
			conflicting := rulesetSpec("preset-ruleset")
			conflicting.Enforcement = "evaluate"
			org.Spec.Rulesets = []v1alpha1.RulesetSpec{conflicting}
			existingRulesets = []*github.RepositoryRuleset{matchingGitHubRuleset(100, "orphan")}
		})

		It("returns a conflict error and makes no GitHub calls", func() {
			Expect(err).To(HaveOccurred())
			var conflictErr *rulesets.ConflictError
			Expect(errors.As(err, &conflictErr)).To(BeTrue())
			Expect(conflictErr.Sources).To(ConsistOf("rulesetPreset/preset-1", "inline[0]"))
			Expect(getAllCalled).To(BeFalse())
			Expect(createdRulesets).To(BeEmpty())
			Expect(updatedRulesets).To(BeEmpty())
			Expect(deletedIDs).To(BeEmpty())
		})
	})

	Context("when GetAllOrganizationRulesets fails", func() {
		BeforeEach(func() {
			getAllError = errors.New("GitHub API error")
		})

		It("returns an organization scoped error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get existing organization rulesets"))
			Expect(err.Error()).To(ContainSubstring("GitHub API error"))
		})
	})
})

var _ = Describe("Organization ruleset helper functions", func() {
	var (
		ctx        context.Context
		mockClient *ghclientmock.MockGitHubClientWrapper
		rec        *GitHubOrgReconciler
		desired    rulesets.Desired
		ghRuleset  *github.RepositoryRuleset
		err        error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()
		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{Client: mockClient, Resource: "test-org"},
		}
		desired = rulesets.Desired{
			Source: rulesets.PresetSource("test-preset"),
			Spec:   v1alpha1.RulesetSpec{Name: "test-ruleset", Enforcement: "active"},
		}
		ghRuleset = &github.RepositoryRuleset{Name: "test-ruleset", Enforcement: github.RulesetEnforcement("active")}
	})

	Describe("deleteRuleSet", func() {
		var calledWithID int64

		BeforeEach(func() {
			calledWithID = 0
			mockClient.DeleteOrganizationRulesetFunc = func(_ context.Context, _ string, id int64) error {
				calledWithID = id
				return nil
			}
		})

		JustBeforeEach(func() {
			err = rec.deleteRuleSet(ctx, 123, "test-ruleset")
		})

		It("calls DeleteOrganizationRuleset", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(calledWithID).To(Equal(int64(123)))
		})

		Context("when delete fails", func() {
			BeforeEach(func() {
				mockClient.DeleteOrganizationRulesetFunc = func(_ context.Context, _ string, _ int64) error {
					return errors.New("delete failed")
				}
			})

			It("returns an error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to delete organization ruleset test-ruleset")))
			})
		})
	})

	Describe("updateRuleset", func() {
		var calledWithID int64

		BeforeEach(func() {
			calledWithID = 0
			mockClient.UpdateOrganizationRulesetFunc = func(_ context.Context, _ string, id int64, rs *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
				calledWithID = id
				return rs, nil
			}
		})

		JustBeforeEach(func() {
			err = rec.updateRuleset(ctx, 456, ghRuleset, desired)
		})

		It("calls UpdateOrganizationRuleset", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(calledWithID).To(Equal(int64(456)))
		})

		Context("when update fails", func() {
			BeforeEach(func() {
				mockClient.UpdateOrganizationRulesetFunc = func(_ context.Context, _ string, _ int64, _ *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
					return nil, errors.New("update failed")
				}
			})

			It("returns an error naming the ruleset source", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to update organization ruleset test-ruleset (rulesetPreset/test-preset)")))
			})
		})
	})

	Describe("createRuleset", func() {
		var createWasCalled bool

		BeforeEach(func() {
			createWasCalled = false
			mockClient.CreateOrganizationRulesetFunc = func(_ context.Context, _ string, rs *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
				createWasCalled = true
				return rs, nil
			}
		})

		JustBeforeEach(func() {
			err = rec.createRuleset(ctx, ghRuleset, desired)
		})

		It("calls CreateOrganizationRuleset", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createWasCalled).To(BeTrue())
		})

		Context("when create fails", func() {
			BeforeEach(func() {
				mockClient.CreateOrganizationRulesetFunc = func(_ context.Context, _ string, _ *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
					return nil, errors.New("create failed")
				}
			})

			It("returns an error naming the ruleset source", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to create organization ruleset test-ruleset (rulesetPreset/test-preset)")))
			})
		})
	})

	Describe("addDefaultOrgRepositoryConditions", func() {
		It("sets ~ALL when no repository conditions are specified", func() {
			conditions := &v1alpha1.RulesetConditions{}
			addDefaultOrgRepositoryConditions(conditions)
			Expect(conditions.RepositoryName).NotTo(BeNil())
			Expect(conditions.RepositoryName.Include).To(Equal([]string{"~ALL"}))
			Expect(conditions.RepositoryName.Exclude).To(Equal([]string{}))
			Expect(*conditions.RepositoryName.Protected).To(BeFalse())
		})

		It("does not override existing RepositoryName conditions", func() {
			conditions := &v1alpha1.RulesetConditions{
				RepositoryName: &v1alpha1.RepositoryNameCondition{Include: []string{"my-repo-*"}},
			}
			addDefaultOrgRepositoryConditions(conditions)
			Expect(conditions.RepositoryName.Include).To(Equal([]string{"my-repo-*"}))
		})

		It("does not add RepositoryName when RepositoryProperty conditions exist", func() {
			conditions := &v1alpha1.RulesetConditions{
				RepositoryProperty: &v1alpha1.RepositoryPropertyCondition{
					Include: []v1alpha1.RepositoryPropertyTarget{
						{Name: "type", PropertyValues: []string{"helm-deployment"}},
					},
				},
			}
			addDefaultOrgRepositoryConditions(conditions)
			Expect(conditions.RepositoryName).To(BeNil())
			Expect(conditions.RepositoryProperty).NotTo(BeNil())
		})

		It("returns nil without panicking when conditions is nil", func() {
			Expect(addDefaultOrgRepositoryConditions(nil)).To(BeNil())
		})
	})
})
