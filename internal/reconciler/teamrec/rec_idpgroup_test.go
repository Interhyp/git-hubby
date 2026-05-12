package teamrec

import (
	"context"
	"errors"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	"github.com/google/go-github/v86/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ReconcileIDPGroup", func() {
	var (
		ctx         context.Context
		mockClient1 *ghclientmock.MockGitHubClientWrapper
		mockClient2 *ghclientmock.MockGitHubClientWrapper
		k8sClient   client.Client
		rec         *GitHubTeamReconciler
		scheme      *runtime.Scheme
		team        *v1alpha1.Team
		err         error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient1 = ghclientmock.NewMockGitHubClientWrapper()
		mockClient2 = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		idpGroup := "test-idp-group"
		team = &v1alpha1.Team{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-team",
				Namespace: "default",
			},
			Spec: v1alpha1.TeamSpec{
				Name:        "test-team",
				Description: "IDP Team",
				IDPGroup:    &idpGroup,
				OrganizationRefs: []v1alpha1.OrganizationRef{
					{Name: "org1"},
				},
			},
		}

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(team).
			WithStatusSubresource(team).
			Build()
	})

	Context("when team is not an IDP team", func() {
		BeforeEach(func() {
			team.Spec.IDPGroup = nil
			team.Spec.Members = []string{"user1", "user2"}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: github.Ptr("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileIDPGroup(ctx)
		})

		It("should skip IDP group reconciliation for non-IDP teams", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(mockClient1.GetExternalGroupCalls()).To(BeEmpty())
		})
	})

	Context("when external group is already linked to team", func() {
		BeforeEach(func() {
			groupName := "test-idp-group"
			mockClient1.GetExternalGroupsForTeamBySlugFunc = func(ctx context.Context, org string, slug string) ([]*github.ExternalGroup, error) {
				return []*github.ExternalGroup{
					{
						GroupID:   github.Ptr(int64(12345)),
						GroupName: &groupName,
					},
				}, nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: github.Ptr("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileIDPGroup(ctx)
		})

		It("should not attempt to add the external group again", func() {
			Expect(err).NotTo(HaveOccurred())
			externalGroupCalls := mockClient1.GetExternalGroupCalls()

			Expect(externalGroupCalls).To(HaveLen(1))
			Expect(externalGroupCalls[0].Method).To(Equal("GetExternalGroupsForTeamBySlug"))
		})
	})

	Context("when external group is not linked to team", func() {
		BeforeEach(func() {
			mockClient1.GetExternalGroupsForTeamBySlugFunc = func(ctx context.Context, org string, slug string) ([]*github.ExternalGroup, error) {
				return []*github.ExternalGroup{}, nil
			}

			mockClient1.GetExternalGroupNamesToIDForOrgFunc = func(ctx context.Context, org string) (map[string]int64, error) {
				return map[string]int64{
					"test-idp-group": 12345,
					"other-group":    67890,
				}, nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: github.Ptr("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileIDPGroup(ctx)
		})

		It("should add the external group to the team", func() {
			Expect(err).NotTo(HaveOccurred())
			externalGroupCalls := mockClient1.GetExternalGroupCalls()

			Expect(externalGroupCalls).To(HaveLen(3))
			Expect(externalGroupCalls[0].Method).To(Equal("GetExternalGroupsForTeamBySlug"))
			Expect(externalGroupCalls[1].Method).To(Equal("GetExternalGroupNamesToIDForOrg"))
			Expect(externalGroupCalls[2].Method).To(Equal("AddExternalGroupToTeamBySlug"))
			Expect(externalGroupCalls[2].GroupID).To(Equal(int64(12345)))
		})
	})

	Context("when external group is not found in available external groups", func() {
		BeforeEach(func() {
			mockClient1.GetExternalGroupsForTeamBySlugFunc = func(ctx context.Context, org string, slug string) ([]*github.ExternalGroup, error) {
				return []*github.ExternalGroup{}, nil
			}

			mockClient1.GetExternalGroupNamesToIDForOrgFunc = func(ctx context.Context, org string) (map[string]int64, error) {
				return map[string]int64{
					"other-group": 67890,
					// test-idp-group is not in the available groups
				}, nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: github.Ptr("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileIDPGroup(ctx)
		})

		It("should succeed without adding the group", func() {
			Expect(err).NotTo(HaveOccurred())
			externalGroupCalls := mockClient1.GetExternalGroupCalls()

			// Should query but not add
			Expect(externalGroupCalls).To(HaveLen(2))
			Expect(externalGroupCalls[0].Method).To(Equal("GetExternalGroupsForTeamBySlug"))
			Expect(externalGroupCalls[1].Method).To(Equal("GetExternalGroupNamesToIDForOrg"))
		})
	})

	Context("when GetExternalGroupsForTeamBySlug fails", func() {
		BeforeEach(func() {
			mockClient1.GetExternalGroupsForTeamBySlugFunc = func(ctx context.Context, org string, slug string) ([]*github.ExternalGroup, error) {
				return nil, errors.New("API error")
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: github.Ptr("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileIDPGroup(ctx)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API error"))
		})
	})

	Context("when GetExternalGroupNamesToIDForOrg fails", func() {
		BeforeEach(func() {
			mockClient1.GetExternalGroupsForTeamBySlugFunc = func(ctx context.Context, org string, slug string) ([]*github.ExternalGroup, error) {
				return []*github.ExternalGroup{}, nil
			}

			mockClient1.GetExternalGroupNamesToIDForOrgFunc = func(ctx context.Context, org string) (map[string]int64, error) {
				return nil, errors.New("API error")
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: github.Ptr("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileIDPGroup(ctx)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API error"))
		})
	})

	Context("when AddExternalGroupToTeamBySlug fails", func() {
		BeforeEach(func() {
			mockClient1.GetExternalGroupsForTeamBySlugFunc = func(ctx context.Context, org string, slug string) ([]*github.ExternalGroup, error) {
				return []*github.ExternalGroup{}, nil
			}

			mockClient1.GetExternalGroupNamesToIDForOrgFunc = func(ctx context.Context, org string) (map[string]int64, error) {
				return map[string]int64{
					"test-idp-group": 12345,
				}, nil
			}

			mockClient1.AddExternalGroupToTeamBySlugFunc = func(ctx context.Context, org string, slug string, group *github.ExternalGroup) error {
				return errors.New("failed to add external group")
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: github.Ptr("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileIDPGroup(ctx)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to add external group"))
		})
	})

	Context("when team is linked to different external group", func() {
		BeforeEach(func() {
			otherGroupName := "other-idp-group"
			mockClient1.GetExternalGroupsForTeamBySlugFunc = func(ctx context.Context, org string, slug string) ([]*github.ExternalGroup, error) {
				return []*github.ExternalGroup{
					{
						GroupID:   github.Ptr(int64(99999)),
						GroupName: &otherGroupName,
					},
				}, nil
			}

			mockClient1.GetExternalGroupNamesToIDForOrgFunc = func(ctx context.Context, org string) (map[string]int64, error) {
				return map[string]int64{
					"test-idp-group":  12345,
					"other-idp-group": 99999,
				}, nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: github.Ptr("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileIDPGroup(ctx)
		})

		It("should add the correct external group", func() {
			Expect(err).NotTo(HaveOccurred())
			externalGroupCalls := mockClient1.GetExternalGroupCalls()

			Expect(externalGroupCalls).To(HaveLen(3))
			Expect(externalGroupCalls[2].Method).To(Equal("AddExternalGroupToTeamBySlug"))
			Expect(externalGroupCalls[2].GroupID).To(Equal(int64(12345)))
		})
	})

	Context("when reconciling IDP team in multiple organizations", func() {
		BeforeEach(func() {
			team.Spec.OrganizationRefs = []v1alpha1.OrganizationRef{
				{Name: "org1"},
				{Name: "org2"},
			}

			groupName := "test-idp-group"
			mockClient1.GetExternalGroupsForTeamBySlugFunc = func(ctx context.Context, org string, slug string) ([]*github.ExternalGroup, error) {
				return []*github.ExternalGroup{
					{
						GroupID:   github.Ptr(int64(12345)),
						GroupName: &groupName,
					},
				}, nil
			}

			mockClient2.GetExternalGroupsForTeamBySlugFunc = func(ctx context.Context, org string, slug string) ([]*github.ExternalGroup, error) {
				return []*github.ExternalGroup{}, nil
			}

			mockClient2.GetExternalGroupNamesToIDForOrgFunc = func(ctx context.Context, org string) (map[string]int64, error) {
				return map[string]int64{
					"test-idp-group": 67890,
				}, nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: github.Ptr("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
							{
								Client:   mockClient2,
								Resource: "org2",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileIDPGroup(ctx)
		})

		It("should reconcile IDP group in both organizations", func() {
			Expect(err).NotTo(HaveOccurred())

			// org1 already has the group linked
			org1Calls := mockClient1.GetExternalGroupCalls()
			Expect(org1Calls).To(HaveLen(1))
			Expect(org1Calls[0].Method).To(Equal("GetExternalGroupsForTeamBySlug"))

			// org2 needs to add the group
			org2Calls := mockClient2.GetExternalGroupCalls()
			Expect(org2Calls).To(HaveLen(3))
			Expect(org2Calls[0].Method).To(Equal("GetExternalGroupsForTeamBySlug"))
			Expect(org2Calls[1].Method).To(Equal("GetExternalGroupNamesToIDForOrg"))
			Expect(org2Calls[2].Method).To(Equal("AddExternalGroupToTeamBySlug"))
		})
	})

	Context("when team has multiple external groups already linked", func() {
		BeforeEach(func() {
			groupName := "test-idp-group"
			otherGroupName := "other-group"
			mockClient1.GetExternalGroupsForTeamBySlugFunc = func(ctx context.Context, org string, slug string) ([]*github.ExternalGroup, error) {
				return []*github.ExternalGroup{
					{
						GroupID:   github.Ptr(int64(99999)),
						GroupName: &otherGroupName,
					},
					{
						GroupID:   github.Ptr(int64(12345)),
						GroupName: &groupName,
					},
				}, nil
			}

			mockClient1.GetExternalGroupNamesToIDForOrgFunc = func(ctx context.Context, org string) (map[string]int64, error) {
				return map[string]int64{
					"test-idp-group": 12345,
					"other-group":    99999,
				}, nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: github.Ptr("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileIDPGroup(ctx)
		})

		It("should detect that the correct group is already linked", func() {
			Expect(err).NotTo(HaveOccurred())
			externalGroupCalls := mockClient1.GetExternalGroupCalls()

			Expect(externalGroupCalls).To(HaveLen(1))
			Expect(externalGroupCalls[0].Method).To(Equal("GetExternalGroupsForTeamBySlug"))
		})
	})

	Context("when external group has nil GroupName", func() {
		BeforeEach(func() {
			mockClient1.GetExternalGroupsForTeamBySlugFunc = func(ctx context.Context, org string, slug string) ([]*github.ExternalGroup, error) {
				return []*github.ExternalGroup{
					{
						GroupID:   github.Ptr(int64(99999)),
						GroupName: nil, // nil group name
					},
				}, nil
			}

			mockClient1.GetExternalGroupNamesToIDForOrgFunc = func(ctx context.Context, org string) (map[string]int64, error) {
				return map[string]int64{
					"test-idp-group": 12345,
				}, nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: github.Ptr("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileIDPGroup(ctx)
		})

		It("should add the correct external group", func() {
			Expect(err).NotTo(HaveOccurred())
			externalGroupCalls := mockClient1.GetExternalGroupCalls()

			Expect(externalGroupCalls).To(HaveLen(3))
			Expect(externalGroupCalls[2].Method).To(Equal("AddExternalGroupToTeamBySlug"))
		})
	})

	Context("when reconciling with empty organization list", func() {
		BeforeEach(func() {
			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: github.Ptr("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileIDPGroup(ctx)
		})

		It("should succeed without any API calls", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(mockClient1.GetExternalGroupCalls()).To(BeEmpty())
		})
	})

	Context("when first organization succeeds but second fails", func() {
		BeforeEach(func() {
			team.Spec.OrganizationRefs = []v1alpha1.OrganizationRef{
				{Name: "org1"},
				{Name: "org2"},
			}

			groupName := "test-idp-group"
			mockClient1.GetExternalGroupsForTeamBySlugFunc = func(ctx context.Context, org string, slug string) ([]*github.ExternalGroup, error) {
				return []*github.ExternalGroup{
					{
						GroupID:   github.Ptr(int64(12345)),
						GroupName: &groupName,
					},
				}, nil
			}

			mockClient1.GetExternalGroupNamesToIDForOrgFunc = func(ctx context.Context, org string) (map[string]int64, error) {
				return map[string]int64{
					"test-idp-group": 12345,
				}, nil
			}

			mockClient2.GetExternalGroupsForTeamBySlugFunc = func(ctx context.Context, org string, slug string) ([]*github.ExternalGroup, error) {
				return nil, errors.New("API error in org2")
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: github.Ptr("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
							{
								Client:   mockClient2,
								Resource: "org2",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileIDPGroup(ctx)
		})

		It("should fail after processing first org", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API error in org2"))

			// org1 should have been processed successfully (both get calls)
			org1Calls := mockClient1.GetExternalGroupCalls()
			Expect(org1Calls).To(HaveLen(1))
			Expect(org1Calls[0].Method).To(Equal("GetExternalGroupsForTeamBySlug"))

			// org2 should have attempted the query
			org2Calls := mockClient2.GetExternalGroupCalls()
			Expect(org2Calls).To(HaveLen(1))
		})
	})

	Context("when available external groups map is empty", func() {
		BeforeEach(func() {
			mockClient1.GetExternalGroupsForTeamBySlugFunc = func(ctx context.Context, org string, slug string) ([]*github.ExternalGroup, error) {
				return []*github.ExternalGroup{}, nil
			}

			mockClient1.GetExternalGroupNamesToIDForOrgFunc = func(ctx context.Context, org string) (map[string]int64, error) {
				return map[string]int64{}, nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: github.Ptr("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{
							{
								Client:   mockClient1,
								Resource: "org1",
							},
						},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileIDPGroup(ctx)
		})

		It("should succeed without adding any group", func() {
			Expect(err).NotTo(HaveOccurred())
			externalGroupCalls := mockClient1.GetExternalGroupCalls()

			Expect(externalGroupCalls).To(HaveLen(2))
			Expect(externalGroupCalls[0].Method).To(Equal("GetExternalGroupsForTeamBySlug"))
			Expect(externalGroupCalls[1].Method).To(Equal("GetExternalGroupNamesToIDForOrg"))
		})
	})
})
