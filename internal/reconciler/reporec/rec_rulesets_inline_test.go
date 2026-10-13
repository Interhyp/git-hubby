package reporec

import (
	"context"
	"errors"

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

var _ = Describe("ReconcileRuleSets with inline rulesets", func() {
	var (
		ctx              context.Context
		mockClient       *ghclientmock.MockGitHubClientWrapper
		rec              *GitHubRepoReconciler
		nameResolver     *mockRepoResourceNameResolver
		repo             *v1alpha1.Repository
		preset           *v1alpha1.RulesetPreset
		features         config.Features
		err              error
		existingRulesets []*github.RepositoryRuleset
		getAllCalled     bool
		createdRulesets  []*github.RepositoryRuleset
		updatedRulesets  map[int64]*github.RepositoryRuleset
		deletedIDs       []int64
	)

	inlineSpec := func(name string) v1alpha1.RulesetSpec {
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

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()
		nameResolver = &mockRepoResourceNameResolver{}
		features = config.Features{}
		repo = &v1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{Name: "test-repo", Namespace: "default"},
			Spec: v1alpha1.RepositorySpec{
				Name:            "test-repo",
				Archived:        new(false),
				OrganizationRef: v1alpha1.OrganizationRef{Name: "test-org"},
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
		org := &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{Name: "test-org", Namespace: "default"},
			Spec:       v1alpha1.OrganizationSpec{Plan: v1alpha1.PlanEnterprise},
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
			return existingRulesets, nil
		}
		mockClient.GetRepositoryRulesetFunc = func(_ context.Context, _, _ string, id int64, _ bool) (*github.RepositoryRuleset, error) {
			for _, rs := range existingRulesets {
				if rs.ID != nil && *rs.ID == id {
					return rs, nil
				}
			}
			return nil, errors.New("ruleset not found")
		}
		mockClient.CreateRepositoryRulesetFunc = func(_ context.Context, _, _ string, rs *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
			createdRulesets = append(createdRulesets, rs)
			return rs, nil
		}
		mockClient.UpdateRepositoryRulesetFunc = func(_ context.Context, _, _ string, id int64, rs *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
			updatedRulesets[id] = rs
			return rs, nil
		}
		mockClient.DeleteRepositoryRulesetFunc = func(_ context.Context, _, _ string, id int64) error {
			deletedIDs = append(deletedIDs, id)
			return nil
		}

		err = rec.reconcileRuleSets(ctx)
	})

	Context("when an inline ruleset does not exist on GitHub", func() {
		BeforeEach(func() {
			repo.Spec.Rulesets = []v1alpha1.RulesetSpec{inlineSpec("inline-1")}
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

	Context("when an inline ruleset differs from GitHub", func() {
		BeforeEach(func() {
			spec := inlineSpec("inline-1")
			spec.Enforcement = "evaluate"
			repo.Spec.Rulesets = []v1alpha1.RulesetSpec{spec}
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
			repo.Spec.Rulesets = []v1alpha1.RulesetSpec{inlineSpec("inline-1")}
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
			repo.Spec.RulesetPresetList = []corev1.LocalObjectReference{{Name: "preset-1"}}
			repo.Spec.Rulesets = []v1alpha1.RulesetSpec{inlineSpec("inline-1")}
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
			existingRulesets = []*github.RepositoryRuleset{matchingGitHubRuleset(100, "inline-1")}
		})

		It("deletes the orphaned ruleset", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deletedIDs).To(ConsistOf(int64(100)))
		})
	})

	Context("when inline and preset rulesets have target 'repository'", func() {
		BeforeEach(func() {
			preset.Spec.Target = "repository"
			repoTarget := inlineSpec("inline-repo")
			repoTarget.Target = "repository"
			repo.Spec.RulesetPresetList = []corev1.LocalObjectReference{{Name: "preset-1"}}
			repo.Spec.Rulesets = []v1alpha1.RulesetSpec{repoTarget, inlineSpec("inline-branch")}
		})

		It("filters them out", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Name).To(Equal("inline-branch"))
			Expect(nameResolver.ResolveRulesetCalls).To(HaveLen(1))
			Expect(nameResolver.ResolveRulesetCalls[0].Name).To(Equal("inline-branch"))
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
			repo.Spec.Rulesets = []v1alpha1.RulesetSpec{spec}
		})

		Context("when disabled", func() {
			It("strips required reviewers from inline rulesets without modifying the spec", func() {
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
			conflicting := inlineSpec("preset-ruleset")
			conflicting.Enforcement = "evaluate"
			repo.Spec.Rulesets = []v1alpha1.RulesetSpec{conflicting}
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
			repo.Spec.RulesetPresetList = []corev1.LocalObjectReference{{Name: "preset-1"}}
			repo.Spec.Rulesets = []v1alpha1.RulesetSpec{inlineSpec("preset-ruleset")}
		})

		It("applies the ruleset once", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Name).To(Equal("preset-ruleset"))
		})
	})
})
