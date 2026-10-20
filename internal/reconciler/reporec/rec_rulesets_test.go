package reporec

import (
	"context"
	"errors"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/config"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/internal/reconciler/common/rulesets"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	"github.com/google/go-github/v93/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// mockRepoResourceNameResolver is a test double for RepoResourceNameResolver.
// ResolveRulesetCalls records every RulesetSpec argument passed to ResolveRuleset.
// Set ResolveRulesetError to make that method return an error.
type mockRepoResourceNameResolver struct {
	ResolveRulesetError error
	ResolveRulesetCalls []v1alpha1.RulesetSpec
}

func (m *mockRepoResourceNameResolver) ResolveRuleset(_ context.Context, rs v1alpha1.RulesetSpec) (v1alpha1.RulesetSpec, error) {
	m.ResolveRulesetCalls = append(m.ResolveRulesetCalls, rs)
	return rs, m.ResolveRulesetError
}

// The generic ruleset sync logic (create/update/delete, diffing, error handling, feature flags) is tested in
// internal/reconciler/rulesets_test.go. These tests only cover the repository specific wiring.
var _ = Describe("ReconcileRuleSets", func() {
	var (
		ctx              context.Context
		mockClient       *ghclientmock.MockGitHubClientWrapper
		rec              *GitHubRepoReconciler
		nameResolver     *mockRepoResourceNameResolver
		repo             *v1alpha1.Repository
		orgPlan          string
		preset           *v1alpha1.RulesetPreset
		features         config.Features
		err              error
		existingRulesets []*github.RepositoryRuleset
		getAllCalled     bool
		getAllError      error
		getCalls         []int64
		createError      error
		updateError      error
		deleteError      error
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
		nameResolver = &mockRepoResourceNameResolver{}
		features = config.Features{}
		orgPlan = v1alpha1.PlanEnterprise
		repo = &v1alpha1.Repository{
			Name: "test-repo", Namespace: "default",
			Spec: v1alpha1.RepositorySpec{
				Name:            "test-repo",
				Archived:        new(false),
				OrganizationRef: v1alpha1.OrganizationRef{Name: "test-org"},
			},
		}
		preset = &v1alpha1.RulesetPreset{
			Name: "preset-1", Namespace: "default",
			Spec: rulesetSpec("preset-ruleset"),
		}
		existingRulesets = nil
		getAllCalled = false
		getAllError = nil
		getCalls = nil
		createError = nil
		updateError = nil
		deleteError = nil
		createdRulesets = nil
		updatedRulesets = map[int64]*github.RepositoryRuleset{}
		deletedIDs = nil
	})

	JustBeforeEach(func() {
		org := &v1alpha1.Organization{
			Name: "test-org", Namespace: "default",
			Spec: v1alpha1.OrganizationSpec{Plan: orgPlan},
		}
		scheme := runtime.NewScheme()
		Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(repo, org, preset).Build()

		rec = &GitHubRepoReconciler{
			GitHub: reconciler.GitHub[GitHubRepoIdentifier]{
				Client:   mockClient,
				Resource: GitHubRepoIdentifier{Owner: "test-org", Name: "test-repo"},
			},
			Kubernetes:   reconciler.Kubernetes[*v1alpha1.Repository]{Client: k8sClient, Resource: repo},
			NameResolver: nameResolver,
			Features:     features,
		}

		mockClient.GetAllRepositoryRulesetsFunc = func(_ context.Context, _, _ string, _ bool) ([]*github.RepositoryRuleset, error) {
			getAllCalled = true
			return existingRulesets, getAllError
		}
		mockClient.GetRepositoryRulesetFunc = func(_ context.Context, _, _ string, id int64, _ bool) (*github.RepositoryRuleset, error) {
			getCalls = append(getCalls, id)
			for _, rs := range existingRulesets {
				if rs.ID != nil && *rs.ID == id {
					return rs, nil
				}
			}
			return nil, errors.New("ruleset not found")
		}
		mockClient.CreateRepositoryRulesetFunc = func(_ context.Context, _, _ string, rs *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
			if createError != nil {
				return nil, createError
			}
			createdRulesets = append(createdRulesets, rs)
			return rs, nil
		}
		mockClient.UpdateRepositoryRulesetFunc = func(_ context.Context, _, _ string, id int64, rs *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
			if updateError != nil {
				return nil, updateError
			}
			updatedRulesets[id] = rs
			return rs, nil
		}
		mockClient.DeleteRepositoryRulesetFunc = func(_ context.Context, _, _ string, id int64) error {
			if deleteError != nil {
				return deleteError
			}
			deletedIDs = append(deletedIDs, id)
			return nil
		}

		err = rec.reconcileRuleSets(ctx)
	})

	Context("when the organization is on the free plan and the repository is not public", func() {
		BeforeEach(func() {
			orgPlan = v1alpha1.PlanFree
			repo.Spec.Visibility = v1alpha1.VisibilityPrivate
			repo.Spec.Rulesets = []v1alpha1.RulesetSpec{rulesetSpec("inline-1")}
		})

		It("skips reconciliation without any GitHub calls", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(getAllCalled).To(BeFalse())
			Expect(createdRulesets).To(BeEmpty())
		})
	})

	Context("when the organization is on the free plan and the repository is public", func() {
		BeforeEach(func() {
			orgPlan = v1alpha1.PlanFree
			repo.Spec.Visibility = v1alpha1.VisibilityPublic
			repo.Spec.Rulesets = []v1alpha1.RulesetSpec{rulesetSpec("inline-1")}
		})

		It("reconciles the rulesets", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdNames()).To(ConsistOf("inline-1"))
		})
	})

	Context("when an inline ruleset does not exist on GitHub", func() {
		BeforeEach(func() {
			repo.Spec.Rulesets = []v1alpha1.RulesetSpec{rulesetSpec("inline-1")}
		})

		It("creates it without adding repository conditions", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Name).To(Equal("inline-1"))
			Expect(createdRulesets[0].Conditions.RepositoryName).To(BeNil())
			Expect(updatedRulesets).To(BeEmpty())
			Expect(deletedIDs).To(BeEmpty())
		})
	})

	Context("when inline rulesets and presets are combined", func() {
		BeforeEach(func() {
			repo.Spec.RulesetPresetList = []corev1.LocalObjectReference{{Name: "preset-1"}}
			repo.Spec.Rulesets = []v1alpha1.RulesetSpec{rulesetSpec("inline-1")}
		})

		It("creates the union", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdNames()).To(ConsistOf("preset-ruleset", "inline-1"))
		})
	})

	Context("when an existing ruleset matches", func() {
		BeforeEach(func() {
			repo.Spec.Rulesets = []v1alpha1.RulesetSpec{rulesetSpec("inline-1")}
			existingRulesets = []*github.RepositoryRuleset{matchingGitHubRuleset(100, "inline-1")}
		})

		It("fetches the repository ruleset and does nothing", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(getCalls).To(ConsistOf(int64(100)))
			Expect(createdRulesets).To(BeEmpty())
			Expect(updatedRulesets).To(BeEmpty())
			Expect(deletedIDs).To(BeEmpty())
		})
	})

	Context("when rulesets need to be created, updated and deleted", func() {
		BeforeEach(func() {
			repo.Spec.RulesetPresetList = []corev1.LocalObjectReference{{Name: "preset-1"}}
			repo.Spec.Rulesets = []v1alpha1.RulesetSpec{rulesetSpec("inline-1")}
			outdated := matchingGitHubRuleset(200, "preset-ruleset")
			outdated.Enforcement = github.RulesetEnforcement("evaluate")
			existingRulesets = []*github.RepositoryRuleset{outdated, matchingGitHubRuleset(300, "orphan")}
		})

		It("uses the repository ruleset API", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdNames()).To(ConsistOf("inline-1"))
			Expect(updatedRulesets).To(HaveLen(1))
			Expect(updatedRulesets[200].Enforcement).To(Equal(github.RulesetEnforcement("active")))
			Expect(deletedIDs).To(ConsistOf(int64(300)))
		})
	})

	Context("when inline and preset rulesets have target 'repository'", func() {
		BeforeEach(func() {
			preset.Spec.Target = "repository"
			repoTarget := rulesetSpec("inline-repo")
			repoTarget.Target = "repository"
			repo.Spec.RulesetPresetList = []corev1.LocalObjectReference{{Name: "preset-1"}}
			repo.Spec.Rulesets = []v1alpha1.RulesetSpec{repoTarget, rulesetSpec("inline-branch")}
			// An existing ruleset with the name of a skipped ruleset is treated as orphaned.
			existingRulesets = []*github.RepositoryRuleset{matchingGitHubRuleset(999, "inline-repo")}
		})

		It("filters them out without resolving them", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdNames()).To(ConsistOf("inline-branch"))
			Expect(nameResolver.ResolveRulesetCalls).To(HaveLen(1))
			Expect(nameResolver.ResolveRulesetCalls[0].Name).To(Equal("inline-branch"))
		})

		It("deletes an existing ruleset with the name of a filtered ruleset", func() {
			Expect(deletedIDs).To(ConsistOf(int64(999)))
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
			repo.Spec.Rulesets = []v1alpha1.RulesetSpec{spec}
		})

		Context("when disabled", func() {
			It("strips required reviewers without modifying the spec", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(nameResolver.ResolveRulesetCalls).To(HaveLen(1))
				Expect(nameResolver.ResolveRulesetCalls[0].Rules.PullRequest.RequiredReviewers).To(BeNil())
				Expect(repo.Spec.Rulesets[0].Rules.PullRequest.RequiredReviewers).To(HaveLen(1))
			})
		})

		Context("when enabled", func() {
			BeforeEach(func() {
				features.EnableRequiredReviewersRules = true
			})

			It("keeps required reviewers", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(nameResolver.ResolveRulesetCalls).To(HaveLen(1))
				Expect(nameResolver.ResolveRulesetCalls[0].Rules.PullRequest.RequiredReviewers).To(HaveLen(1))
			})
		})
	})

	Context("when an inline ruleset conflicts with a preset", func() {
		BeforeEach(func() {
			repo.Spec.RulesetPresetList = []corev1.LocalObjectReference{{Name: "preset-1"}}
			conflicting := rulesetSpec("preset-ruleset")
			conflicting.Enforcement = "evaluate"
			repo.Spec.Rulesets = []v1alpha1.RulesetSpec{conflicting}
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

	Context("error handling", func() {
		Context("when GetAllRepositoryRulesets fails", func() {
			BeforeEach(func() {
				getAllError = errors.New("GitHub API error")
			})

			It("returns a repository scoped error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to get existing repository rulesets")))
				Expect(err).To(MatchError(ContainSubstring("GitHub API error")))
			})
		})

		Context("when CreateRepositoryRuleset fails", func() {
			BeforeEach(func() {
				repo.Spec.Rulesets = []v1alpha1.RulesetSpec{rulesetSpec("inline-1")}
				createError = errors.New("creation failed")
			})

			It("returns an error naming the ruleset source", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to create repository ruleset inline-1 (inline[0])")))
			})
		})

		Context("when UpdateRepositoryRuleset fails", func() {
			BeforeEach(func() {
				spec := rulesetSpec("inline-1")
				spec.Enforcement = "evaluate"
				repo.Spec.Rulesets = []v1alpha1.RulesetSpec{spec}
				existingRulesets = []*github.RepositoryRuleset{matchingGitHubRuleset(100, "inline-1")}
				updateError = errors.New("update failed")
			})

			It("returns an error naming the ruleset source", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to update repository ruleset inline-1 (inline[0])")))
			})
		})

		Context("when DeleteRepositoryRuleset fails", func() {
			BeforeEach(func() {
				existingRulesets = []*github.RepositoryRuleset{matchingGitHubRuleset(100, "orphan")}
				deleteError = errors.New("deletion failed")
			})

			It("returns an error naming the ruleset", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to delete repository ruleset orphan")))
			})
		})
	})
})
