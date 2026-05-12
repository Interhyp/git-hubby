package reporec

import (
	"context"
	"errors"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	"github.com/google/go-github/v86/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ReconcileRuleSets", func() {
	var (
		ctx                 context.Context
		mockClient          *ghclientmock.MockGitHubClientWrapper
		k8sClient           client.Client
		rec                 *GitHubRepoReconciler
		scheme              *runtime.Scheme
		repo                *v1alpha1.Repository
		rulesetPresets      []*v1alpha1.RulesetPreset
		existingGHRulesets  []*github.RepositoryRuleset
		err                 error
		getAllRulesetsError error
		getFullRulesetError error
		createRulesetCalled bool
		updateRulesetCalled bool
		deleteRulesetCalled bool
		createdRulesets     []*github.RepositoryRuleset
		updatedRulesets     map[int64]*github.RepositoryRuleset
		deletedRulesetIDs   []int64
		getRulesetFunc      func(ctx context.Context, owner, repo string, rulesetID int64, includesParents bool) (*github.RepositoryRuleset, error)
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())
		schemeErr = corev1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		// Default: no rulesets
		existingGHRulesets = []*github.RepositoryRuleset{}
		rulesetPresets = []*v1alpha1.RulesetPreset{}

		// Reset flags and errors
		getAllRulesetsError = nil
		getFullRulesetError = nil
		createRulesetCalled = false
		updateRulesetCalled = false
		deleteRulesetCalled = false
		createdRulesets = []*github.RepositoryRuleset{}
		updatedRulesets = make(map[int64]*github.RepositoryRuleset)
		deletedRulesetIDs = []int64{}

		// Set up default mock functions
		mockClient.GetAllRepositoryRulesetsFunc = func(_ context.Context, _, _ string, _ bool) ([]*github.RepositoryRuleset, error) {
			return existingGHRulesets, getAllRulesetsError
		}

		getRulesetFunc = func(_ context.Context, _, _ string, rulesetID int64, _ bool) (*github.RepositoryRuleset, error) {
			// Find the matching ruleset from existing
			for _, rs := range existingGHRulesets {
				if rs.ID != nil && *rs.ID == rulesetID {
					return rs, getFullRulesetError
				}
			}
			return nil, errors.New("ruleset not found")
		}

		mockClient.GetRepositoryRulesetFunc = func(ctx context.Context, owner, repo string, rulesetID int64, includesParents bool) (*github.RepositoryRuleset, error) {
			return getRulesetFunc(ctx, owner, repo, rulesetID, includesParents)
		}

		mockClient.CreateRepositoryRulesetFunc = func(_ context.Context, _, _ string, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
			createRulesetCalled = true
			created := *ruleset
			created.ID = github.Ptr(int64(1000 + len(createdRulesets)))
			createdRulesets = append(createdRulesets, &created)
			return &created, nil
		}

		mockClient.UpdateRepositoryRulesetFunc = func(ctx context.Context, owner, repo string, rulesetID int64, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
			updateRulesetCalled = true
			updated := *ruleset
			updated.ID = github.Ptr(rulesetID)
			updatedRulesets[rulesetID] = &updated
			return &updated, nil
		}

		mockClient.DeleteRepositoryRulesetFunc = func(ctx context.Context, owner, repo string, rulesetID int64) error {
			deleteRulesetCalled = true
			deletedRulesetIDs = append(deletedRulesetIDs, rulesetID)
			return nil
		}
	})

	JustBeforeEach(func() {
		// Create repository CR
		rulesetPresetRefs := make([]corev1.LocalObjectReference, len(rulesetPresets))
		for i, preset := range rulesetPresets {
			rulesetPresetRefs[i] = corev1.LocalObjectReference{Name: preset.Name}
		}

		repo = &v1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-repo",
				Namespace: "default",
			},
			Spec: v1alpha1.RepositorySpec{
				Name:              "test-repo",
				Archived:          github.Ptr(false),
				RulesetPresetList: rulesetPresetRefs,
				OrganizationRef: v1alpha1.OrganizationRef{
					Name: "test-org",
				},
			},
		}

		// Build k8s objects slice
		k8sObjects := make([]client.Object, 1, 1+len(rulesetPresets))
		k8sObjects[0] = repo
		for _, preset := range rulesetPresets {
			k8sObjects = append(k8sObjects, preset)
		}

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(k8sObjects...).
			WithStatusSubresource(repo).
			Build()

		rec = &GitHubRepoReconciler{
			GitHub: reconciler.GitHub[GitHubRepoIdentifier]{
				Client: mockClient,
				Resource: GitHubRepoIdentifier{
					Owner: "test-org",
					Name:  "test-repo",
				},
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Repository]{
				Client:   k8sClient,
				Resource: repo,
			},
		}

		err = rec.reconcileRuleSets(ctx)
	})

	Context("when no rulesets are defined", func() {
		It("should reconcile successfully with no changes", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRulesetCalled).To(BeFalse())
			Expect(updateRulesetCalled).To(BeFalse())
			Expect(deleteRulesetCalled).To(BeFalse())
		})
	})

	Context("when creating a new ruleset", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main-protection",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "main-protection",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							RequiredLinearHistory: github.Ptr(true),
						},
					},
				},
			}
		})

		It("should create the ruleset successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRulesetCalled).To(BeTrue())
			Expect(updateRulesetCalled).To(BeFalse())
			Expect(deleteRulesetCalled).To(BeFalse())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Name).To(Equal("main-protection"))
		})
	})

	Context("when creating multiple rulesets", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main-protection",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "main-protection",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							RequiredLinearHistory: github.Ptr(true),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "develop-protection",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "develop-protection",
						Enforcement: v1alpha1.RulesetEnforcementEvaluate,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"develop"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							Creation: github.Ptr(true),
						},
					},
				},
			}
		})

		It("should create all rulesets", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRulesetCalled).To(BeTrue())
			Expect(createdRulesets).To(HaveLen(2))
			rulesetNames := []string{createdRulesets[0].Name, createdRulesets[1].Name}
			Expect(rulesetNames).To(ConsistOf("main-protection", "develop-protection"))
		})
	})

	Context("when ruleset already exists and matches", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main-protection",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "main-protection",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Target:      "branch",
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							RequiredLinearHistory: github.Ptr(true),
						},
					},
				},
			}

			// Existing ruleset matches - important: RefName patterns must match mapper output
			existingGHRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(123)),
					Name:        "main-protection",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"main"}, // Changed from refs/heads/main to main
						},
					},
					Rules: &github.RepositoryRulesetRules{
						RequiredLinearHistory: &github.EmptyRuleParameters{},
					},
				},
			}
		})

		It("should skip update when ruleset matches", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRulesetCalled).To(BeFalse())
			Expect(updateRulesetCalled).To(BeFalse())
			Expect(deleteRulesetCalled).To(BeFalse())
		})
	})

	Context("when ruleset exists but differs", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main-protection",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "main-protection",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							RequiredLinearHistory: github.Ptr(true),
							RequiredSignatures:    github.Ptr(true),
						},
					},
				},
			}

			// Existing ruleset has different rules
			existingGHRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(123)),
					Name:        "main-protection",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"refs/heads/main"},
						},
					},
					Rules: &github.RepositoryRulesetRules{
						RequiredLinearHistory: &github.EmptyRuleParameters{},
					},
				},
			}
		})

		It("should update the ruleset", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRulesetCalled).To(BeFalse())
			Expect(updateRulesetCalled).To(BeTrue())
			Expect(deleteRulesetCalled).To(BeFalse())
			Expect(updatedRulesets).To(HaveLen(1))
			Expect(updatedRulesets[int64(123)]).NotTo(BeNil())
		})
	})

	Context("when enforcement level changes", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main-protection",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "main-protection",
						Enforcement: v1alpha1.RulesetEnforcementEvaluate,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							RequiredLinearHistory: github.Ptr(true),
						},
					},
				},
			}

			// Existing ruleset has different enforcement
			existingGHRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(123)),
					Name:        "main-protection",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"refs/heads/main"},
						},
					},
					Rules: &github.RepositoryRulesetRules{
						RequiredLinearHistory: &github.EmptyRuleParameters{},
					},
				},
			}
		})

		It("should update the ruleset with new enforcement level", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateRulesetCalled).To(BeTrue())
			Expect(updatedRulesets[int64(123)]).NotTo(BeNil())
		})
	})

	Context("when target branches change", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main-protection",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "main-protection",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main", "develop"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							RequiredLinearHistory: github.Ptr(true),
						},
					},
				},
			}

			// Existing ruleset targets only main
			existingGHRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(123)),
					Name:        "main-protection",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"refs/heads/main"},
						},
					},
					Rules: &github.RepositoryRulesetRules{
						RequiredLinearHistory: &github.EmptyRuleParameters{},
					},
				},
			}
		})

		It("should update the ruleset with new targets", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateRulesetCalled).To(BeTrue())
			Expect(updatedRulesets[int64(123)]).NotTo(BeNil())
		})
	})

	Context("when deleting unused rulesets", func() {
		BeforeEach(func() {
			// No rulesets in spec
			rulesetPresets = []*v1alpha1.RulesetPreset{}

			// But rulesets exist on GitHub
			existingGHRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(123)),
					Name:        "old-ruleset",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
				},
			}
		})

		It("should delete the orphaned ruleset", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRulesetCalled).To(BeFalse())
			Expect(updateRulesetCalled).To(BeFalse())
			Expect(deleteRulesetCalled).To(BeTrue())
			Expect(deletedRulesetIDs).To(ConsistOf(int64(123)))
		})
	})

	Context("when deleting multiple unused rulesets", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "keep-this",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "keep-this",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							Creation: github.Ptr(true),
						},
					},
				},
			}

			existingGHRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(123)),
					Name:        "keep-this",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"refs/heads/main"},
						},
					},
					Rules: &github.RepositoryRulesetRules{
						Creation: &github.EmptyRuleParameters{},
					},
				},
				{
					ID:          github.Ptr(int64(456)),
					Name:        "delete-this",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
				},
				{
					ID:          github.Ptr(int64(789)),
					Name:        "delete-this-too",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
				},
			}
		})

		It("should delete only the orphaned rulesets", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deleteRulesetCalled).To(BeTrue())
			Expect(deletedRulesetIDs).To(ConsistOf(int64(456), int64(789)))
		})
	})

	Context("when some rulesets need creation and some need update", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "existing-ruleset",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "existing-ruleset",
						Enforcement: v1alpha1.RulesetEnforcementEvaluate,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							RequiredLinearHistory: github.Ptr(true),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "new-ruleset",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "new-ruleset",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"develop"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							Creation: github.Ptr(true),
						},
					},
				},
			}

			existingGHRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(123)),
					Name:        "existing-ruleset",
					Enforcement: github.RulesetEnforcementActive, // Different enforcement
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"refs/heads/main"},
						},
					},
					Rules: &github.RepositoryRulesetRules{
						RequiredLinearHistory: &github.EmptyRuleParameters{},
					},
				},
			}
		})

		It("should update existing and create new rulesets", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRulesetCalled).To(BeTrue())
			Expect(updateRulesetCalled).To(BeTrue())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Name).To(Equal("new-ruleset"))
			Expect(updatedRulesets).To(HaveLen(1))
			Expect(updatedRulesets[int64(123)]).NotTo(BeNil())
		})
	})

	Context("when GetAllRepositoryRulesets fails", func() {
		BeforeEach(func() {
			getAllRulesetsError = errors.New("failed to get rulesets")
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main-protection",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "main-protection",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							RequiredLinearHistory: github.Ptr(true),
						},
					},
				},
			}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get rulesets"))
			Expect(createRulesetCalled).To(BeFalse())
		})
	})

	Context("when RulesetPreset CR is not found", func() {
		BeforeEach(func() {
			// Reference a preset that doesn't exist - don't add it to rulesetPresets
			// The parent JustBeforeEach will create a repo that references this preset
			// but won't add the preset object to k8s
			rulesetPresets = []*v1alpha1.RulesetPreset{}
			// Manually set up the repo to reference the nonexistent preset
		})

		JustBeforeEach(func() {
			// Manually update the repo spec to reference nonexistent preset after parent setup
			repo.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "nonexistent-preset"},
			}
			// Update the repo in k8s
			updateErr := k8sClient.Update(ctx, repo)
			Expect(updateErr).NotTo(HaveOccurred())
			// Update the reconciler's resource reference
			rec.Kubernetes.Resource = repo
			// Now call reconcile
			err = rec.reconcileRuleSets(ctx)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get ruleset preset"))
			Expect(err.Error()).To(ContainSubstring("nonexistent-preset"))
		})
	})

	Context("when mapper fails to convert preset", func() {
		BeforeEach(func() {
			// Create a preset with invalid data (empty name)
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "invalid-preset",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "", // Empty name will cause mapper error
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							Creation: github.Ptr(true),
						},
					},
				},
			}
		})

		It("should return a mapping error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to convert ruleset preset"))
		})
	})

	Context("when GetRepositoryRuleset fails", func() {
		BeforeEach(func() {
			getFullRulesetError = errors.New("failed to get full ruleset")

			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main-protection",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "main-protection",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							RequiredLinearHistory: github.Ptr(true),
						},
					},
				},
			}

			existingGHRulesets = []*github.RepositoryRuleset{
				{
					ID:   github.Ptr(int64(123)),
					Name: "main-protection",
				},
			}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get existing repository ruleset"))
		})
	})

	Context("when CreateRepositoryRuleset fails", func() {
		BeforeEach(func() {
			mockClient.CreateRepositoryRulesetFunc = func(ctx context.Context, owner, repo string, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
				return nil, errors.New("creation failed")
			}

			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main-protection",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "main-protection",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							RequiredLinearHistory: github.Ptr(true),
						},
					},
				},
			}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to create repository ruleset"))
		})
	})

	Context("when UpdateRepositoryRuleset fails", func() {
		BeforeEach(func() {
			mockClient.UpdateRepositoryRulesetFunc = func(ctx context.Context, owner, repo string, rulesetID int64, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
				return nil, errors.New("update failed")
			}

			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main-protection",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "main-protection",
						Enforcement: v1alpha1.RulesetEnforcementEvaluate,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							RequiredLinearHistory: github.Ptr(true),
						},
					},
				},
			}

			existingGHRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(123)),
					Name:        "main-protection",
					Enforcement: github.RulesetEnforcementActive, // Different
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"refs/heads/main"},
						},
					},
					Rules: &github.RepositoryRulesetRules{
						RequiredLinearHistory: &github.EmptyRuleParameters{},
					},
				},
			}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to update repository ruleset"))
		})
	})

	Context("when DeleteRepositoryRuleset fails", func() {
		BeforeEach(func() {
			mockClient.DeleteRepositoryRulesetFunc = func(ctx context.Context, owner, repo string, rulesetID int64) error {
				return errors.New("deletion failed")
			}

			rulesetPresets = []*v1alpha1.RulesetPreset{}

			existingGHRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(123)),
					Name:        "old-ruleset",
					Enforcement: github.RulesetEnforcementActive,
				},
			}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to delete repository ruleset"))
		})
	})

	Context("when existing ruleset has nil ID", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main-protection",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "main-protection",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							RequiredLinearHistory: github.Ptr(true),
						},
					},
				},
			}

			existingGHRulesets = []*github.RepositoryRuleset{
				{
					ID:          nil, // Nil ID
					Name:        "main-protection",
					Enforcement: github.RulesetEnforcementEvaluate,
					Target:      github.Ptr(github.RulesetTargetBranch),
				},
			}

			getRulesetFunc = func(_ context.Context, owner, repo string, rulesetID int64, includesParents bool) (*github.RepositoryRuleset, error) {
				return &github.RepositoryRuleset{
					ID:          nil,
					Name:        "main-protection",
					Enforcement: github.RulesetEnforcementEvaluate,
					Target:      github.Ptr(github.RulesetTargetBranch),
				}, nil
			}
		})

		It("should skip the ruleset and continue", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateRulesetCalled).To(BeFalse())
		})
	})

	Context("when existing rulesets have empty names", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{}

			existingGHRulesets = []*github.RepositoryRuleset{
				{
					ID:   github.Ptr(int64(123)),
					Name: "", // Empty name
				},
				{
					ID:   github.Ptr(int64(456)),
					Name: "valid-name",
				},
			}
		})

		It("should only delete ruleset with valid name", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deleteRulesetCalled).To(BeTrue())
			Expect(deletedRulesetIDs).To(ConsistOf(int64(456)))
		})
	})

	Context("when ruleset has complex rules", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "complex-ruleset",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "complex-ruleset",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main", "release/*"},
								Exclude: []string{"release/test"},
							},
						},
						BypassActors: []v1alpha1.RulesetBypassActor{
							{
								ActorID:    github.Ptr(int64(12345)),
								ActorType:  "Team",
								BypassMode: "always",
							},
						},
						Rules: v1alpha1.RulesetRules{
							Creation:              github.Ptr(true),
							Update:                github.Ptr(true),
							Deletion:              github.Ptr(true),
							RequiredLinearHistory: github.Ptr(true),
							RequiredSignatures:    github.Ptr(true),
							NonFastForward:        github.Ptr(true),
							PullRequest: &v1alpha1.PullRequestRule{
								DismissStaleReviewsOnPush:      github.Ptr(true),
								RequireCodeOwnerReviews:        github.Ptr(true),
								RequireLastPushApproval:        github.Ptr(true),
								RequiredApprovingReviewCount:   2,
								RequiredReviewThreadResolution: github.Ptr(true),
							},
						},
					},
				},
			}
		})

		It("should create the complex ruleset successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRulesetCalled).To(BeTrue())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Name).To(Equal("complex-ruleset"))
		})
	})

	Context("when updating ruleset with bypass actors change", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main-protection",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "main-protection",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						BypassActors: []v1alpha1.RulesetBypassActor{
							{
								ActorID:    github.Ptr(int64(999)),
								ActorType:  "Team",
								BypassMode: "always",
							},
						},
						Rules: v1alpha1.RulesetRules{
							RequiredLinearHistory: github.Ptr(true),
						},
					},
				},
			}

			existingGHRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(123)),
					Name:        "main-protection",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"refs/heads/main"},
						},
					},
					BypassActors: []*github.BypassActor{
						{
							ActorID:   github.Ptr(int64(888)),
							ActorType: github.Ptr(github.BypassActorTypeTeam),
						},
					},
					Rules: &github.RepositoryRulesetRules{
						RequiredLinearHistory: &github.EmptyRuleParameters{},
					},
				},
			}
		})

		It("should update the ruleset with new bypass actors", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateRulesetCalled).To(BeTrue())
			Expect(updatedRulesets[int64(123)]).NotTo(BeNil())
		})
	})

	Context("when managing rulesets with pattern rules", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pattern-ruleset",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "pattern-ruleset",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							CommitMessagePattern: &v1alpha1.PatternRule{
								Pattern:  "^[A-Z]+-[0-9]+:.*",
								Operator: "starts_with",
								Negate:   github.Ptr(false),
							},
							BranchNamePattern: &v1alpha1.PatternRule{
								Pattern:  "^(feature|bugfix|hotfix)/",
								Operator: "starts_with",
								Negate:   github.Ptr(false),
							},
						},
					},
				},
			}
		})

		It("should create ruleset with pattern rules", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRulesetCalled).To(BeTrue())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Name).To(Equal("pattern-ruleset"))
		})
	})

	Context("when processing multiple presets in different namespaces", func() {
		BeforeEach(func() {
			// This should work since both are in default namespace
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "preset-1",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "preset-1",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							Creation: github.Ptr(true),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "preset-2",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "preset-2",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"develop"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							Update: github.Ptr(true),
						},
					},
				},
			}
		})

		It("should create all rulesets successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRulesetCalled).To(BeTrue())
			Expect(createdRulesets).To(HaveLen(2))
		})
	})

	Context("when updating only rules while keeping other fields", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main-protection",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "main-protection",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							RequiredLinearHistory: github.Ptr(true),
							RequiredSignatures:    github.Ptr(true), // Added
						},
					},
				},
			}

			existingGHRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(123)),
					Name:        "main-protection",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"refs/heads/main"},
						},
					},
					Rules: &github.RepositoryRulesetRules{
						RequiredLinearHistory: &github.EmptyRuleParameters{},
					},
				},
			}
		})

		It("should update only the changed rules", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateRulesetCalled).To(BeTrue())
			Expect(createdRulesets).To(BeEmpty())
			Expect(deletedRulesetIDs).To(BeEmpty())
		})
	})

	Context("when status checks rules are included", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "status-check-ruleset",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "status-check-ruleset",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							RequiredStatusChecks: &v1alpha1.RequiredStatusChecks{
								Checks: []v1alpha1.StatusCheck{
									{Context: "ci/build"},
									{Context: "ci/test"},
								},
								StrictPolicy: github.Ptr(true),
							},
						},
					},
				},
			}
		})

		It("should create ruleset with status checks", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRulesetCalled).To(BeTrue())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Name).To(Equal("status-check-ruleset"))
		})
	})

	Context("when CopilotCodeReview rule is included", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "copilot-review-ruleset",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "copilot-review-ruleset",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							CopilotReview: &v1alpha1.CopilotCodeReviewRule{
								ReviewOnPush:            github.Ptr(true),
								ReviewDraftPullRequests: github.Ptr(false),
							},
						},
					},
				},
			}
		})

		It("should create ruleset with CopilotCodeReview rule", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRulesetCalled).To(BeTrue())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Name).To(Equal("copilot-review-ruleset"))
			Expect(createdRulesets[0].Rules).NotTo(BeNil())
			Expect(createdRulesets[0].Rules.CopilotCodeReview).NotTo(BeNil())
			Expect(createdRulesets[0].Rules.CopilotCodeReview.ReviewOnPush).To(BeTrue())
			Expect(createdRulesets[0].Rules.CopilotCodeReview.ReviewDraftPullRequests).To(BeFalse())
		})
	})

	Context("when CopilotCodeReview rule already exists and matches", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "copilot-review-ruleset",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "copilot-review-ruleset",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Target:      "branch",
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							CopilotReview: &v1alpha1.CopilotCodeReviewRule{
								ReviewOnPush:            github.Ptr(true),
								ReviewDraftPullRequests: github.Ptr(true),
							},
						},
					},
				},
			}

			existingGHRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(123)),
					Name:        "copilot-review-ruleset",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"main"},
						},
					},
					Rules: &github.RepositoryRulesetRules{
						CopilotCodeReview: &github.CopilotCodeReviewRuleParameters{
							ReviewOnPush:            true,
							ReviewDraftPullRequests: true,
						},
					},
				},
			}
		})

		It("should skip update when CopilotCodeReview ruleset matches", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRulesetCalled).To(BeFalse())
			Expect(updateRulesetCalled).To(BeFalse())
			Expect(deleteRulesetCalled).To(BeFalse())
		})
	})

	Context("when CopilotCodeReview rule exists but differs", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "copilot-review-ruleset",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "copilot-review-ruleset",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Target:      "branch",
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							CopilotReview: &v1alpha1.CopilotCodeReviewRule{
								ReviewOnPush:            github.Ptr(true),
								ReviewDraftPullRequests: github.Ptr(true),
							},
						},
					},
				},
			}

			existingGHRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(123)),
					Name:        "copilot-review-ruleset",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"main"},
						},
					},
					Rules: &github.RepositoryRulesetRules{
						CopilotCodeReview: &github.CopilotCodeReviewRuleParameters{
							ReviewOnPush:            false, // Different from desired
							ReviewDraftPullRequests: true,
						},
					},
				},
			}
		})

		It("should update ruleset when CopilotCodeReview rule differs", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateRulesetCalled).To(BeTrue())
			Expect(updatedRulesets).To(HaveLen(1))
			Expect(updatedRulesets[int64(123)]).NotTo(BeNil())
			Expect(updatedRulesets[int64(123)].Rules.CopilotCodeReview).NotTo(BeNil())
			Expect(updatedRulesets[int64(123)].Rules.CopilotCodeReview.ReviewOnPush).To(BeTrue())
			Expect(updatedRulesets[int64(123)].Rules.CopilotCodeReview.ReviewDraftPullRequests).To(BeTrue())
		})
	})

	Context("when CopilotCodeReview rule is removed", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "copilot-review-ruleset",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "copilot-review-ruleset",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Target:      "branch",
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							// No CopilotReview rule specified
							RequiredLinearHistory: github.Ptr(true),
						},
					},
				},
			}

			existingGHRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(123)),
					Name:        "copilot-review-ruleset",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"main"},
						},
					},
					Rules: &github.RepositoryRulesetRules{
						CopilotCodeReview: &github.CopilotCodeReviewRuleParameters{
							ReviewOnPush:            true,
							ReviewDraftPullRequests: true,
						},
					},
				},
			}
		})

		It("should update ruleset to remove CopilotCodeReview rule", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateRulesetCalled).To(BeTrue())
			Expect(updatedRulesets).To(HaveLen(1))
			Expect(updatedRulesets[int64(123)]).NotTo(BeNil())
			Expect(updatedRulesets[int64(123)].Rules.CopilotCodeReview).To(BeNil())
		})
	})

	Context("when CopilotCodeReview rule is combined with other rules", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "comprehensive-ruleset",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "comprehensive-ruleset",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Target:      "branch",
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							RequiredLinearHistory: github.Ptr(true),
							RequiredSignatures:    github.Ptr(true),
							CopilotReview: &v1alpha1.CopilotCodeReviewRule{
								ReviewOnPush:            github.Ptr(true),
								ReviewDraftPullRequests: github.Ptr(false),
							},
							PullRequest: &v1alpha1.PullRequestRule{
								RequiredApprovingReviewCount: 2,
								RequireCodeOwnerReviews:      github.Ptr(true),
							},
						},
					},
				},
			}
		})

		It("should create ruleset with CopilotCodeReview and other rules", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRulesetCalled).To(BeTrue())
			Expect(createdRulesets).To(HaveLen(1))
			created := createdRulesets[0]
			Expect(created.Name).To(Equal("comprehensive-ruleset"))
			Expect(created.Rules).NotTo(BeNil())
			Expect(created.Rules.RequiredLinearHistory).NotTo(BeNil())
			Expect(created.Rules.RequiredSignatures).NotTo(BeNil())
			Expect(created.Rules.CopilotCodeReview).NotTo(BeNil())
			Expect(created.Rules.CopilotCodeReview.ReviewOnPush).To(BeTrue())
			Expect(created.Rules.CopilotCodeReview.ReviewDraftPullRequests).To(BeFalse())
			Expect(created.Rules.PullRequest).NotTo(BeNil())
			Expect(created.Rules.PullRequest.RequiredApprovingReviewCount).To(Equal(2))
		})
	})

	Context("when rulesetPreset has targetType repository", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo-target-ruleset",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "repo-target-ruleset",
						Target:      "repository",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RepositoryName: &v1alpha1.RepositoryNameCondition{
								Include: []string{"~ALL"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							RequiredLinearHistory: github.Ptr(true),
						},
					},
				},
			}
		})

		It("should skip the ruleset and not create, update, or delete anything", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRulesetCalled).To(BeFalse())
			Expect(updateRulesetCalled).To(BeFalse())
			Expect(deleteRulesetCalled).To(BeFalse())
		})
	})

	Context("when mix of repository and branch targetType rulesetPresets", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo-target-ruleset",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "repo-target-ruleset",
						Target:      "repository",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RepositoryName: &v1alpha1.RepositoryNameCondition{
								Include: []string{"~ALL"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							RequiredLinearHistory: github.Ptr(true),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "branch-ruleset",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "branch-ruleset",
						Target:      "branch",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RefName: &v1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							Creation: github.Ptr(true),
						},
					},
				},
			}
		})

		It("should skip the repository targetType and only create the branch ruleset", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRulesetCalled).To(BeTrue())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Name).To(Equal("branch-ruleset"))
		})
	})

	Context("when repository targetType ruleset exists on GitHub but is skipped", func() {
		BeforeEach(func() {
			rulesetPresets = []*v1alpha1.RulesetPreset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo-target-ruleset",
						Namespace: "default",
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name:        "repo-target-ruleset",
						Target:      "repository",
						Enforcement: v1alpha1.RulesetEnforcementActive,
						Conditions: &v1alpha1.RulesetConditions{
							RepositoryName: &v1alpha1.RepositoryNameCondition{
								Include: []string{"~ALL"},
							},
						},
						Rules: v1alpha1.RulesetRules{
							RequiredLinearHistory: github.Ptr(true),
						},
					},
				},
			}

			// An unrelated ruleset exists on GitHub that matches the skipped preset name
			existingGHRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(999)),
					Name:        "repo-target-ruleset",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTarget("repository")),
				},
			}
		})

		It("should skip the repository targetType preset and delete the orphaned GitHub ruleset", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRulesetCalled).To(BeFalse())
			Expect(updateRulesetCalled).To(BeFalse())
			Expect(deleteRulesetCalled).To(BeTrue())
			Expect(deletedRulesetIDs).To(ConsistOf(int64(999)))
		})
	})
})
