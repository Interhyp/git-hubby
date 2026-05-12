package orgrec

import (
	"context"
	"errors"
	"net/http"

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

var _ = Describe("ReconcileRulesetPresets", func() {
	var (
		ctx                context.Context
		mockClient         *ghclientmock.MockGitHubClientWrapper
		k8sClient          client.Client
		rec                *GitHubOrgReconciler
		scheme             *runtime.Scheme
		org                *v1alpha1.Organization
		rulesetPreset1     *v1alpha1.RulesetPreset
		rulesetPreset2     *v1alpha1.RulesetPreset
		err                error
		existingRulesets   []*github.RepositoryRuleset
		createdRulesets    []*github.RepositoryRuleset
		updatedRulesets    map[int64]*github.RepositoryRuleset
		deletedRulesetIDs  []int64
		getOrgRulesetCalls map[int64]int
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		org = &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-org",
				Namespace: "default",
			},
			Spec: v1alpha1.OrganizationSpec{
				Name:                    "test-org",
				GitHubAppInstallationId: 12345,
				RulesetPresetList:       []corev1.LocalObjectReference{},
			},
		}

		rulesetPreset1 = &v1alpha1.RulesetPreset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ruleset-1",
				Namespace: "default",
			},
			Spec: v1alpha1.RulesetPresetSpec{
				Name: "ruleset-1",
				Conditions: &v1alpha1.RulesetConditions{
					RefName: &v1alpha1.RefNameCondition{Include: []string{"refs/heads/main"}},
				},
				Target:      "branch",
				Enforcement: "active",
				Rules: v1alpha1.RulesetRules{
					Creation: github.Ptr(true),
					Deletion: github.Ptr(true),
				},
			},
		}

		rulesetPreset2 = &v1alpha1.RulesetPreset{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ruleset-2",
				Namespace: "default",
			},
			Spec: v1alpha1.RulesetPresetSpec{
				Name: "ruleset-2",
				Conditions: &v1alpha1.RulesetConditions{
					RefName: &v1alpha1.RefNameCondition{Include: []string{"refs/heads/develop"}},
				},
				Target:      "branch",
				Enforcement: "active",
				Rules: v1alpha1.RulesetRules{
					Update: github.Ptr(true),
				},
			},
		}

		existingRulesets = []*github.RepositoryRuleset{}
		createdRulesets = []*github.RepositoryRuleset{}
		updatedRulesets = make(map[int64]*github.RepositoryRuleset)
		deletedRulesetIDs = []int64{}
		getOrgRulesetCalls = make(map[int64]int)
	})

	JustBeforeEach(func() {
		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(org, rulesetPreset1, rulesetPreset2).
			WithStatusSubresource(org).
			Build()

		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				Client:   mockClient,
				Resource: "test-org",
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Client:   k8sClient,
				Resource: org,
			},
		}

		mockClient.GetAllOrganizationRulesetsFunc = func(ctx context.Context, org string, includesParents bool) ([]*github.RepositoryRuleset, error) {
			return existingRulesets, nil
		}

		mockClient.GetOrganizationRulesetFunc = func(ctx context.Context, org string, rulesetID int64) (*github.RepositoryRuleset, error) {
			getOrgRulesetCalls[rulesetID]++
			for _, rs := range existingRulesets {
				if rs.ID != nil && *rs.ID == rulesetID {
					return rs, nil
				}
			}
			return nil, &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
			}
		}

		mockClient.CreateOrganizationRulesetFunc = func(ctx context.Context, org string, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
			created := *ruleset
			newID := int64(len(createdRulesets) + 1000)
			created.ID = &newID
			createdRulesets = append(createdRulesets, &created)
			return &created, nil
		}

		mockClient.UpdateOrganizationRulesetFunc = func(ctx context.Context, org string, rulesetID int64, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
			updated := *ruleset
			updated.ID = &rulesetID
			updatedRulesets[rulesetID] = &updated
			return &updated, nil
		}

		mockClient.DeleteOrganizationRulesetFunc = func(ctx context.Context, org string, rulesetID int64) error {
			deletedRulesetIDs = append(deletedRulesetIDs, rulesetID)
			return nil
		}

		err = rec.reconcileRulesetPresets(ctx)
	})

	Context("when no rulesets are configured", func() {
		It("should reconcile successfully with no changes", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(BeEmpty())
			Expect(updatedRulesets).To(BeEmpty())
			Expect(deletedRulesetIDs).To(BeEmpty())
		})
	})

	Context("when creating a new ruleset", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "ruleset-1"},
			}
		})

		It("should create the ruleset successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Name).To(Equal("ruleset-1"))
			Expect(createdRulesets[0].Enforcement).To(Equal(github.RulesetEnforcement("active")))
			Expect(updatedRulesets).To(BeEmpty())
			Expect(deletedRulesetIDs).To(BeEmpty())
		})

		It("should set repository name condition to ~ALL", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Conditions).NotTo(BeNil())
			Expect(createdRulesets[0].Conditions.RepositoryName).NotTo(BeNil())
			Expect(createdRulesets[0].Conditions.RepositoryName.Include).To(ContainElement("~ALL"))
		})
	})

	Context("when creating multiple rulesets", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "ruleset-1"},
				{Name: "ruleset-2"},
			}
		})

		It("should create all rulesets successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(HaveLen(2))

			names := []string{createdRulesets[0].Name, createdRulesets[1].Name}
			Expect(names).To(ConsistOf("ruleset-1", "ruleset-2"))
			Expect(updatedRulesets).To(BeEmpty())
			Expect(deletedRulesetIDs).To(BeEmpty())
		})
	})

	Context("when updating an existing ruleset", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "ruleset-1"},
			}

			// Existing ruleset with different configuration
			existingRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(100)),
					Name:        "ruleset-1",
					Enforcement: github.RulesetEnforcement("evaluate"), // Different enforcement
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"refs/heads/main"},
							Exclude: []string{},
						},
					},
					Rules: &github.RepositoryRulesetRules{
						Creation: &github.EmptyRuleParameters{},
					},
				},
			}
		})

		It("should update the ruleset successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(BeEmpty())
			Expect(updatedRulesets).To(HaveLen(1))
			Expect(updatedRulesets[100]).NotTo(BeNil())
			Expect(updatedRulesets[100].Name).To(Equal("ruleset-1"))
			Expect(updatedRulesets[100].Enforcement).To(Equal(github.RulesetEnforcement("active")))
			Expect(deletedRulesetIDs).To(BeEmpty())
		})

		It("should call GetOrganizationRuleset to get full details", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(getOrgRulesetCalls[100]).To(Equal(1))
		})
	})

	Context("when ruleset already matches desired state", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "ruleset-1"},
			}

			// Existing ruleset that matches desired state (including default ~ALL repository name condition)
			existingRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(100)),
					Name:        "ruleset-1",
					Enforcement: github.RulesetEnforcement("active"),
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"refs/heads/main"},
							Exclude: []string{},
						},
						RepositoryName: &github.RepositoryRulesetRepositoryNamesConditionParameters{
							Include:   []string{"~ALL"},
							Exclude:   []string{},
							Protected: github.Ptr(false),
						},
					},
					Rules: &github.RepositoryRulesetRules{
						Creation: &github.EmptyRuleParameters{},
						Deletion: &github.EmptyRuleParameters{},
					},
				},
			}
		})

		It("should skip update", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(BeEmpty())
			Expect(updatedRulesets).To(BeEmpty())
			Expect(deletedRulesetIDs).To(BeEmpty())
		})
	})

	Context("when deleting orphaned rulesets", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "ruleset-1"},
			}

			// Two existing rulesets, but only one is referenced
			existingRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(100)),
					Name:        "ruleset-1",
					Enforcement: github.RulesetEnforcement("active"),
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"refs/heads/main"},
							Exclude: []string{},
						},
					},
					Rules: &github.RepositoryRulesetRules{
						Creation: &github.EmptyRuleParameters{},
						Deletion: &github.EmptyRuleParameters{},
					},
				},
				{
					ID:          github.Ptr(int64(200)),
					Name:        "orphaned-ruleset",
					Enforcement: github.RulesetEnforcement("active"),
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions:  &github.RepositoryRulesetConditions{},
					Rules:       &github.RepositoryRulesetRules{},
				},
			}
		})

		It("should delete the orphaned ruleset", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deletedRulesetIDs).To(HaveLen(1))
			Expect(deletedRulesetIDs).To(ContainElement(int64(200)))
		})

		It("should not delete the referenced ruleset", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deletedRulesetIDs).NotTo(ContainElement(int64(100)))
		})
	})

	Context("when GetAllOrganizationRulesets fails", func() {
		JustBeforeEach(func() {
			mockClient.GetAllOrganizationRulesetsFunc = func(ctx context.Context, org string, includesParents bool) ([]*github.RepositoryRuleset, error) {
				return nil, errors.New("GitHub API error")
			}
			err = rec.reconcileRulesetPresets(ctx)
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get existing organization rulesets"))
			Expect(err.Error()).To(ContainSubstring("GitHub API error"))
		})
	})

	Context("when ruleset preset not found in Kubernetes", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "non-existent-ruleset"},
			}
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get ruleset preset"))
		})
	})

	Context("when GetOrganizationRuleset fails", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "ruleset-1"},
			}

			existingRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(100)),
					Name:        "ruleset-1",
					Enforcement: github.RulesetEnforcement("evaluate"),
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions:  &github.RepositoryRulesetConditions{},
					Rules:       &github.RepositoryRulesetRules{},
				},
			}
		})

		JustBeforeEach(func() {
			mockClient.GetOrganizationRulesetFunc = func(ctx context.Context, org string, rulesetID int64) (*github.RepositoryRuleset, error) {
				return nil, errors.New("GitHub API error getting ruleset")
			}
			err = rec.reconcileRulesetPresets(ctx)
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get existing organization ruleset"))
		})
	})

	Context("when CreateOrganizationRuleset fails", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "ruleset-1"},
			}
		})

		JustBeforeEach(func() {
			mockClient.CreateOrganizationRulesetFunc = func(ctx context.Context, org string, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
				return nil, errors.New("GitHub API error creating ruleset")
			}
			err = rec.reconcileRulesetPresets(ctx)
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to create organization ruleset"))
		})
	})

	Context("when UpdateOrganizationRuleset fails", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "ruleset-1"},
			}

			existingRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(100)),
					Name:        "ruleset-1",
					Enforcement: github.RulesetEnforcement("evaluate"),
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"refs/heads/main"},
							Exclude: []string{},
						},
					},
					Rules: &github.RepositoryRulesetRules{},
				},
			}
		})

		JustBeforeEach(func() {
			mockClient.UpdateOrganizationRulesetFunc = func(ctx context.Context, org string, rulesetID int64, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
				return nil, errors.New("GitHub API error updating ruleset")
			}
			err = rec.reconcileRulesetPresets(ctx)
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to update organization ruleset"))
		})
	})

	Context("when DeleteOrganizationRuleset fails", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{}

			existingRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(100)),
					Name:        "orphaned-ruleset",
					Enforcement: github.RulesetEnforcement("active"),
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions:  &github.RepositoryRulesetConditions{},
					Rules:       &github.RepositoryRulesetRules{},
				},
			}
		})

		JustBeforeEach(func() {
			mockClient.DeleteOrganizationRulesetFunc = func(ctx context.Context, org string, rulesetID int64) error {
				return errors.New("GitHub API error deleting ruleset")
			}
			err = rec.reconcileRulesetPresets(ctx)
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to delete organization ruleset"))
		})
	})

	Context("when existing ruleset has nil ID", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "ruleset-1"},
			}

			existingRulesets = []*github.RepositoryRuleset{
				{
					ID:          nil, // Nil ID
					Name:        "ruleset-1",
					Enforcement: github.RulesetEnforcement("evaluate"),
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"refs/heads/main"},
							Exclude: []string{},
						},
					},
					Rules: &github.RepositoryRulesetRules{
						Creation: &github.EmptyRuleParameters{},
					},
				},
			}
		})

		It("should skip update and not crash", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedRulesets).To(BeEmpty())
		})
	})

	Context("when existing ruleset to delete has nil ID", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{}

			existingRulesets = []*github.RepositoryRuleset{
				{
					ID:          nil, // Nil ID
					Name:        "orphaned-ruleset",
					Enforcement: github.RulesetEnforcement("active"),
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions:  &github.RepositoryRulesetConditions{},
					Rules:       &github.RepositoryRulesetRules{},
				},
			}
		})

		It("should skip deletion and not crash", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deletedRulesetIDs).To(BeEmpty())
		})
	})

	Context("when existing ruleset has empty name", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{}

			existingRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(100)),
					Name:        "", // Empty name
					Enforcement: github.RulesetEnforcement("active"),
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions:  &github.RepositoryRulesetConditions{},
					Rules:       &github.RepositoryRulesetRules{},
				},
			}
		})

		It("should skip ruleset with empty name", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deletedRulesetIDs).To(BeEmpty())
		})
	})

	Context("when mapper fails to convert ruleset preset", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "ruleset-1"},
			}

			// Create a preset with empty name which will cause mapper to fail
			rulesetPreset1.Spec.Name = ""
		})

		JustBeforeEach(func() {
			// Need to update the object in the k8s client
			Expect(k8sClient.Update(ctx, rulesetPreset1)).To(Succeed())
			err = rec.reconcileRulesetPresets(ctx)
		})

		It("should return error from mapper", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to convert ruleset preset"))
		})
	})

	Context("when reconciling with bypass actors", func() {
		BeforeEach(func() {
			rulesetPreset1.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorID:    github.Ptr(int64(12345)),
					ActorType:  "Team",
					BypassMode: "always",
				},
			}

			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "ruleset-1"},
			}
		})

		It("should create ruleset with bypass actors", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].BypassActors).To(HaveLen(1))
			Expect(createdRulesets[0].BypassActors[0].GetActorID()).To(Equal(int64(12345)))
		})
	})

	Context("when reconciling rulesets with different ref name conditions", func() {
		BeforeEach(func() {
			rulesetPreset1.Spec.Conditions = &v1alpha1.RulesetConditions{
				RefName: &v1alpha1.RefNameCondition{
					Include: []string{"refs/heads/main", "refs/heads/develop"},
					Exclude: []string{"refs/heads/feature/*"},
				},
			}

			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "ruleset-1"},
			}
		})

		It("should create ruleset with include and exclude patterns", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Conditions).NotTo(BeNil())
			Expect(createdRulesets[0].Conditions.RefName).NotTo(BeNil())
			Expect(createdRulesets[0].Conditions.RefName.Include).To(ContainElements("refs/heads/main", "refs/heads/develop"))
			Expect(createdRulesets[0].Conditions.RefName.Exclude).To(ContainElement("refs/heads/feature/*"))
		})

		It("should add default ~ALL repository name condition", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Conditions.RepositoryName).NotTo(BeNil())
			Expect(createdRulesets[0].Conditions.RepositoryName.Include).To(Equal([]string{"~ALL"}))
			Expect(createdRulesets[0].Conditions.RepositoryName.Exclude).To(Equal([]string{}))
		})
	})

	Context("when reconciling rulesets with custom repository name conditions", func() {
		BeforeEach(func() {
			rulesetPreset1.Spec.Conditions = &v1alpha1.RulesetConditions{
				RefName: &v1alpha1.RefNameCondition{
					Include: []string{"refs/heads/main"},
				},
				RepositoryName: &v1alpha1.RepositoryNameCondition{
					Include:   []string{"backend-*", "frontend-*"},
					Exclude:   []string{"backend-legacy"},
					Protected: github.Ptr(true),
				},
			}

			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "ruleset-1"},
			}
		})

		It("should create ruleset with custom repository name conditions", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Conditions).NotTo(BeNil())
			Expect(createdRulesets[0].Conditions.RepositoryName).NotTo(BeNil())
			Expect(createdRulesets[0].Conditions.RepositoryName.Include).To(ConsistOf("backend-*", "frontend-*"))
			Expect(createdRulesets[0].Conditions.RepositoryName.Exclude).To(ConsistOf("backend-legacy"))
			Expect(createdRulesets[0].Conditions.RepositoryName.Protected).To(Equal(github.Ptr(true)))
		})

		It("should not override with default ~ALL repository name condition", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Conditions.RepositoryName.Include).NotTo(ContainElement("~ALL"))
		})

		It("should preserve ref name conditions", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Conditions.RefName).NotTo(BeNil())
			Expect(createdRulesets[0].Conditions.RefName.Include).To(ContainElement("refs/heads/main"))
		})
	})

	Context("when reconciling rulesets with repository property conditions", func() {
		BeforeEach(func() {
			rulesetPreset1.Spec.Conditions = &v1alpha1.RulesetConditions{
				RefName: &v1alpha1.RefNameCondition{
					Include: []string{"refs/heads/main"},
				},
				RepositoryProperty: &v1alpha1.RepositoryPropertyCondition{
					Include: []v1alpha1.RepositoryPropertyTarget{
						{
							Name:           "type",
							PropertyValues: []string{"helm-deployment", "service"},
						},
					},
					Exclude: []v1alpha1.RepositoryPropertyTarget{
						{
							Name:           "environment",
							PropertyValues: []string{"staging"},
						},
					},
				},
			}

			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "ruleset-1"},
			}
		})

		It("should create ruleset with repository property conditions", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Conditions).NotTo(BeNil())
			Expect(createdRulesets[0].Conditions.RepositoryProperty).NotTo(BeNil())
			Expect(createdRulesets[0].Conditions.RepositoryProperty.Include).To(HaveLen(1))
			Expect(createdRulesets[0].Conditions.RepositoryProperty.Include[0].Name).To(Equal("type"))
			Expect(createdRulesets[0].Conditions.RepositoryProperty.Include[0].PropertyValues).To(ConsistOf("helm-deployment", "service"))
			Expect(createdRulesets[0].Conditions.RepositoryProperty.Exclude).To(HaveLen(1))
			Expect(createdRulesets[0].Conditions.RepositoryProperty.Exclude[0].Name).To(Equal("environment"))
			Expect(createdRulesets[0].Conditions.RepositoryProperty.Exclude[0].PropertyValues).To(ConsistOf("staging"))
		})

		It("should not add default ~ALL repository name condition", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Conditions.RepositoryName).To(BeNil())
		})

		It("should preserve ref name conditions", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Conditions.RefName).NotTo(BeNil())
			Expect(createdRulesets[0].Conditions.RefName.Include).To(ContainElement("refs/heads/main"))
		})
	})

	Context("when reconciling rulesets with no repository conditions (default ~ALL)", func() {
		BeforeEach(func() {
			rulesetPreset1.Spec.Conditions = &v1alpha1.RulesetConditions{
				RefName: &v1alpha1.RefNameCondition{
					Include: []string{"~DEFAULT_BRANCH"},
				},
			}

			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "ruleset-1"},
			}
		})

		It("should add default ~ALL repository name condition when no repo conditions specified", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Conditions).NotTo(BeNil())
			Expect(createdRulesets[0].Conditions.RepositoryName).NotTo(BeNil())
			Expect(createdRulesets[0].Conditions.RepositoryName.Include).To(Equal([]string{"~ALL"}))
			Expect(createdRulesets[0].Conditions.RepositoryName.Exclude).To(Equal([]string{}))
			Expect(*createdRulesets[0].Conditions.RepositoryName.Protected).To(BeFalse())
		})
	})

	Context("when updating existing ruleset with custom repository name conditions", func() {
		BeforeEach(func() {
			rulesetPreset1.Spec.Conditions = &v1alpha1.RulesetConditions{
				RefName: &v1alpha1.RefNameCondition{
					Include: []string{"refs/heads/main"},
				},
				RepositoryName: &v1alpha1.RepositoryNameCondition{
					Include: []string{"backend-*"},
				},
			}

			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "ruleset-1"},
			}

			existingRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(100)),
					Name:        "ruleset-1",
					Enforcement: github.RulesetEnforcement("active"),
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"refs/heads/main"},
							Exclude: []string{},
						},
						RepositoryName: &github.RepositoryRulesetRepositoryNamesConditionParameters{
							Include: []string{"~ALL"},
							Exclude: []string{},
						},
					},
					Rules: &github.RepositoryRulesetRules{
						Creation: &github.EmptyRuleParameters{},
						Deletion: &github.EmptyRuleParameters{},
					},
				},
			}
		})

		It("should update the ruleset with new repository name conditions", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedRulesets).To(HaveLen(1))
			Expect(updatedRulesets[100]).NotTo(BeNil())
			Expect(updatedRulesets[100].Conditions.RepositoryName).NotTo(BeNil())
			Expect(updatedRulesets[100].Conditions.RepositoryName.Include).To(ConsistOf("backend-*"))
		})
	})

	Context("when updating existing ruleset with repository property conditions", func() {
		BeforeEach(func() {
			rulesetPreset1.Spec.Conditions = &v1alpha1.RulesetConditions{
				RefName: &v1alpha1.RefNameCondition{
					Include: []string{"refs/heads/main"},
				},
				RepositoryProperty: &v1alpha1.RepositoryPropertyCondition{
					Include: []v1alpha1.RepositoryPropertyTarget{
						{Name: "team", PropertyValues: []string{"platform"}},
					},
				},
			}

			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "ruleset-1"},
			}

			existingRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(100)),
					Name:        "ruleset-1",
					Enforcement: github.RulesetEnforcement("active"),
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"refs/heads/main"},
							Exclude: []string{},
						},
						RepositoryName: &github.RepositoryRulesetRepositoryNamesConditionParameters{
							Include: []string{"~ALL"},
							Exclude: []string{},
						},
					},
					Rules: &github.RepositoryRulesetRules{
						Creation: &github.EmptyRuleParameters{},
						Deletion: &github.EmptyRuleParameters{},
					},
				},
			}
		})

		It("should update the ruleset with repository property conditions", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedRulesets).To(HaveLen(1))
			Expect(updatedRulesets[100]).NotTo(BeNil())
			Expect(updatedRulesets[100].Conditions.RepositoryProperty).NotTo(BeNil())
			Expect(updatedRulesets[100].Conditions.RepositoryProperty.Include).To(HaveLen(1))
			Expect(updatedRulesets[100].Conditions.RepositoryProperty.Include[0].Name).To(Equal("team"))
			Expect(updatedRulesets[100].Conditions.RepositoryName).To(BeNil())
		})
	})

	Context("when CreateOrganizationRuleset returns 403 Forbidden", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "ruleset-1"},
			}
		})

		JustBeforeEach(func() {
			mockClient.CreateOrganizationRulesetFunc = func(ctx context.Context, org string, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
				return nil, &github.ErrorResponse{
					Message: "Forbidden",
					Response: &http.Response{
						StatusCode: http.StatusForbidden,
					},
				}
			}
			err = rec.reconcileRulesetPresets(ctx)
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to create organization ruleset"))
		})
	})

	Context("when UpdateOrganizationRuleset returns 422 Unprocessable Entity", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "ruleset-1"},
			}

			existingRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(100)),
					Name:        "ruleset-1",
					Enforcement: github.RulesetEnforcement("evaluate"),
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"refs/heads/main"},
							Exclude: []string{},
						},
					},
					Rules: &github.RepositoryRulesetRules{},
				},
			}
		})

		JustBeforeEach(func() {
			mockClient.UpdateOrganizationRulesetFunc = func(ctx context.Context, org string, rulesetID int64, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
				return nil, &github.ErrorResponse{
					Message: "Validation failed",
					Response: &http.Response{
						StatusCode: http.StatusUnprocessableEntity,
					},
				}
			}
			err = rec.reconcileRulesetPresets(ctx)
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to update organization ruleset"))
		})
	})

	Context("when multiple rulesets need different actions", func() {
		BeforeEach(func() {
			org.Spec.RulesetPresetList = []corev1.LocalObjectReference{
				{Name: "ruleset-1"}, // Will be created
				{Name: "ruleset-2"}, // Will be updated
			}

			// Ruleset-2 already exists but needs update
			existingRulesets = []*github.RepositoryRuleset{
				{
					ID:          github.Ptr(int64(200)),
					Name:        "ruleset-2",
					Enforcement: github.RulesetEnforcement("evaluate"), // Different
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{
							Include: []string{"refs/heads/develop"},
							Exclude: []string{},
						},
					},
					Rules: &github.RepositoryRulesetRules{},
				},
				{
					ID:          github.Ptr(int64(300)),
					Name:        "orphaned-ruleset", // Will be deleted
					Enforcement: github.RulesetEnforcement("active"),
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions:  &github.RepositoryRulesetConditions{},
					Rules:       &github.RepositoryRulesetRules{},
				},
			}
		})

		It("should create, update, and delete appropriately", func() {
			Expect(err).NotTo(HaveOccurred())

			// Should create ruleset-1
			Expect(createdRulesets).To(HaveLen(1))
			Expect(createdRulesets[0].Name).To(Equal("ruleset-1"))

			// Should update ruleset-2
			Expect(updatedRulesets).To(HaveLen(1))
			Expect(updatedRulesets[200]).NotTo(BeNil())
			Expect(updatedRulesets[200].Name).To(Equal("ruleset-2"))

			// Should delete orphaned ruleset
			Expect(deletedRulesetIDs).To(HaveLen(1))
			Expect(deletedRulesetIDs).To(ContainElement(int64(300)))
		})
	})
})

var _ = Describe("Helper Functions", func() {
	var (
		ctx        context.Context
		mockClient *ghclientmock.MockGitHubClientWrapper
		k8sClient  client.Client
		rec        *GitHubOrgReconciler
		scheme     *runtime.Scheme
		org        *v1alpha1.Organization
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		org = &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-org",
				Namespace: "default",
			},
			Spec: v1alpha1.OrganizationSpec{
				Name:                    "test-org",
				GitHubAppInstallationId: 12345,
			},
		}

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(org).
			WithStatusSubresource(org).
			Build()

		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				Client:   mockClient,
				Resource: "test-org",
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Client:   k8sClient,
				Resource: org,
			},
		}
	})

	Describe("deleteRuleSet", func() {
		var (
			err             error
			deleteWasCalled bool
		)

		JustBeforeEach(func() {
			deleteWasCalled = false
			mockClient.DeleteOrganizationRulesetFunc = func(ctx context.Context, org string, rulesetID int64) error {
				deleteWasCalled = true
				return nil
			}
			err = rec.deleteRuleSet(ctx, 123, "test-ruleset")
		})

		It("should call DeleteOrganizationRuleset", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deleteWasCalled).To(BeTrue())
		})

		Context("when delete fails", func() {
			JustBeforeEach(func() {
				mockClient.DeleteOrganizationRulesetFunc = func(ctx context.Context, org string, rulesetID int64) error {
					return errors.New("delete failed")
				}
				err = rec.deleteRuleSet(ctx, 123, "test-ruleset")
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to delete organization ruleset"))
			})
		})
	})

	Describe("updateRuleset", func() {
		var (
			err             error
			updateWasCalled bool
			rulesetPreset   v1alpha1.RulesetPreset
			githubRuleset   *github.RepositoryRuleset
		)

		BeforeEach(func() {
			rulesetPreset = v1alpha1.RulesetPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-preset",
				},
				Spec: v1alpha1.RulesetPresetSpec{
					Name:        "test-ruleset",
					Enforcement: "active",
				},
			}

			githubRuleset = &github.RepositoryRuleset{
				Name:        "test-ruleset",
				Enforcement: github.RulesetEnforcement("active"),
			}
		})

		JustBeforeEach(func() {
			updateWasCalled = false
			mockClient.UpdateOrganizationRulesetFunc = func(ctx context.Context, org string, rulesetID int64, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
				updateWasCalled = true
				return ruleset, nil
			}
			err = rec.updateRuleset(ctx, 456, githubRuleset, rulesetPreset)
		})

		It("should call UpdateOrganizationRuleset", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateWasCalled).To(BeTrue())
		})

		Context("when update fails", func() {
			JustBeforeEach(func() {
				mockClient.UpdateOrganizationRulesetFunc = func(ctx context.Context, org string, rulesetID int64, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
					return nil, errors.New("update failed")
				}
				err = rec.updateRuleset(ctx, 456, githubRuleset, rulesetPreset)
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to update organization ruleset"))
			})
		})
	})

	Describe("createRuleset", func() {
		var (
			err             error
			createWasCalled bool
			rulesetPreset   v1alpha1.RulesetPreset
			githubRuleset   *github.RepositoryRuleset
		)

		BeforeEach(func() {
			rulesetPreset = v1alpha1.RulesetPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-preset",
				},
				Spec: v1alpha1.RulesetPresetSpec{
					Name:        "test-ruleset",
					Enforcement: "active",
				},
			}

			githubRuleset = &github.RepositoryRuleset{
				Name:        "test-ruleset",
				Enforcement: github.RulesetEnforcement("active"),
			}
		})

		JustBeforeEach(func() {
			createWasCalled = false
			mockClient.CreateOrganizationRulesetFunc = func(ctx context.Context, org string, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
				createWasCalled = true
				return ruleset, nil
			}
			err = rec.createRuleset(ctx, githubRuleset, rulesetPreset)
		})

		It("should call CreateOrganizationRuleset", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createWasCalled).To(BeTrue())
		})

		Context("when create fails", func() {
			JustBeforeEach(func() {
				mockClient.CreateOrganizationRulesetFunc = func(ctx context.Context, org string, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
					return nil, errors.New("create failed")
				}
				err = rec.createRuleset(ctx, githubRuleset, rulesetPreset)
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to create organization ruleset"))
			})
		})
	})

	Context("addDefaultOrgRepositoryConditions", func() {
		It("should set ~ALL when no repository conditions are specified", func() {
			conditions := &v1alpha1.RulesetConditions{}
			addDefaultOrgRepositoryConditions(conditions)
			Expect(conditions.RepositoryName).NotTo(BeNil())
			Expect(conditions.RepositoryName.Include).To(Equal([]string{"~ALL"}))
			Expect(conditions.RepositoryName.Exclude).To(Equal([]string{}))
			Expect(*conditions.RepositoryName.Protected).To(BeFalse())
		})

		It("should not override existing RepositoryName conditions", func() {
			conditions := &v1alpha1.RulesetConditions{
				RepositoryName: &v1alpha1.RepositoryNameCondition{
					Include: []string{"my-repo-*"},
				},
			}
			addDefaultOrgRepositoryConditions(conditions)
			Expect(conditions.RepositoryName.Include).To(Equal([]string{"my-repo-*"}))
		})

		It("should not override existing RepositoryProperty conditions", func() {
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

		It("should not panic when conditions is nil", func() {
			Expect(func() {
				addDefaultOrgRepositoryConditions(nil)
			}).NotTo(Panic())
		})

		It("should return nil when conditions is nil", func() {
			result := addDefaultOrgRepositoryConditions(nil)
			Expect(result).To(BeNil())
		})
	})
})
