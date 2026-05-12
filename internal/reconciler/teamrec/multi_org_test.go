package teamrec

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Multi-Organization Team Scenarios", func() {
	var (
		ctx         context.Context
		mockClient1 *ghclientmock.MockGitHubClientWrapper
		mockClient2 *ghclientmock.MockGitHubClientWrapper
		mockClient3 *ghclientmock.MockGitHubClientWrapper
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
		mockClient3 = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())
	})

	Describe("Adding organizations to spec.organizationRefs", func() {
		Context("when adding a new organization to an existing team", func() {
			BeforeEach(func() {
				team = &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-team",
						Namespace: "default",
					},
					Spec: v1alpha1.TeamSpec{
						Name:        "test-team",
						Description: "Test team description",
						Members:     []string{"user1", "user2"},
						OrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
							{Name: "org2"}, // newly added
						},
					},
					Status: v1alpha1.TeamStatus{
						Slug: github.Ptr("test-team"),
						PreviousOrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
						},
					},
				}

				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(team).
					WithStatusSubresource(team).
					Build()

				// org1 already has the team
				mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
					return &github.Team{
						Name:                github.Ptr("test-team"),
						Slug:                github.Ptr("test-team"),
						Description:         github.Ptr("Test team description"),
						Privacy:             github.Ptr("closed"),
						Permission:          github.Ptr("pull"),
						NotificationSetting: github.Ptr("notifications_disabled"),
					}, nil
				}

				// org2 doesn't have the team yet
				mockClient2.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
					return nil, &github.ErrorResponse{
						Message: "Not Found",
						Response: &http.Response{
							StatusCode: http.StatusNotFound,
						},
					}
				}

				mockClient2.CreateTeamFunc = func(ctx context.Context, org string, newTeam *github.NewTeam) (*github.Team, error) {
					return &github.Team{
						Name:                &newTeam.Name,
						Slug:                &newTeam.Name,
						Description:         newTeam.Description,
						Privacy:             newTeam.Privacy,
						Permission:          newTeam.Permission, //nolint:staticcheck
						NotificationSetting: newTeam.NotificationSetting,
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
							Previous: []reconciler.GitHub[string]{
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

				err = rec.reconcileTeam(ctx)
			})

			It("should succeed without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not modify existing org1 team", func() {
				org1Calls := mockClient1.GetTeamCalls()
				Expect(org1Calls).To(HaveLen(1))
				Expect(org1Calls[0].Method).To(Equal("GetTeamBySlug"))
			})

			It("should create team in new org2", func() {
				org2Calls := mockClient2.GetTeamCalls()
				Expect(org2Calls).To(HaveLen(2))
				Expect(org2Calls[0].Method).To(Equal("GetTeamBySlug"))
				Expect(org2Calls[1].Method).To(Equal("CreateTeam"))
				Expect(org2Calls[1].Org).To(Equal("org2"))
			})
		})

		Context("when adding multiple new organizations at once", func() {
			BeforeEach(func() {
				team = &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-team",
						Namespace: "default",
					},
					Spec: v1alpha1.TeamSpec{
						Name:        "test-team",
						Description: "Test team description",
						Members:     []string{"user1"},
						OrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
							{Name: "org2"}, // newly added
							{Name: "org3"}, // newly added
						},
					},
					Status: v1alpha1.TeamStatus{
						Slug: github.Ptr("test-team"),
						PreviousOrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
						},
					},
				}

				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(team).
					WithStatusSubresource(team).
					Build()

				// org1 already has the team
				mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
					return &github.Team{
						Name:                github.Ptr("test-team"),
						Slug:                github.Ptr("test-team"),
						Description:         github.Ptr("Test team description"),
						Privacy:             github.Ptr("closed"),
						Permission:          github.Ptr("pull"),
						NotificationSetting: github.Ptr("notifications_disabled"),
					}, nil
				}

				// org2 and org3 don't have the team yet
				createTeamFunc := func(ctx context.Context, org string, newTeam *github.NewTeam) (*github.Team, error) {
					return &github.Team{
						Name:                &newTeam.Name,
						Slug:                &newTeam.Name,
						Description:         newTeam.Description,
						Privacy:             newTeam.Privacy,
						Permission:          newTeam.Permission, //nolint:staticcheck
						NotificationSetting: newTeam.NotificationSetting,
					}, nil
				}

				notFoundFunc := func(ctx context.Context, org string, slug string) (*github.Team, error) {
					return nil, &github.ErrorResponse{
						Message: "Not Found",
						Response: &http.Response{
							StatusCode: http.StatusNotFound,
						},
					}
				}

				mockClient2.GetTeamBySlugFunc = notFoundFunc
				mockClient2.CreateTeamFunc = createTeamFunc

				mockClient3.GetTeamBySlugFunc = notFoundFunc
				mockClient3.CreateTeamFunc = createTeamFunc

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
								{
									Client:   mockClient3,
									Resource: "org3",
								},
							},
							Previous: []reconciler.GitHub[string]{
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

				err = rec.reconcileTeam(ctx)
			})

			It("should succeed without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create team in both new organizations", func() {
				org2Calls := mockClient2.GetTeamCalls()
				Expect(org2Calls).To(HaveLen(2))
				Expect(org2Calls[1].Method).To(Equal("CreateTeam"))

				org3Calls := mockClient3.GetTeamCalls()
				Expect(org3Calls).To(HaveLen(2))
				Expect(org3Calls[1].Method).To(Equal("CreateTeam"))
			})
		})

		Context("when adding organization and team creation fails", func() {
			BeforeEach(func() {
				team = &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-team",
						Namespace: "default",
					},
					Spec: v1alpha1.TeamSpec{
						Name:    "test-team",
						Members: []string{"user1"},
						OrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
							{Name: "org2"}, // newly added, will fail
						},
					},
					Status: v1alpha1.TeamStatus{
						Slug: github.Ptr("test-team"),
						PreviousOrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
						},
					},
				}

				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(team).
					WithStatusSubresource(team).
					Build()

				mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
					return &github.Team{
						Name:                github.Ptr("test-team"),
						Slug:                github.Ptr("test-team"),
						Description:         github.Ptr(""),
						Privacy:             github.Ptr("closed"),
						Permission:          github.Ptr("pull"),
						NotificationSetting: github.Ptr("notifications_disabled"),
					}, nil
				}

				mockClient2.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
					return nil, &github.ErrorResponse{
						Message: "Not Found",
						Response: &http.Response{
							StatusCode: http.StatusNotFound,
						},
					}
				}

				mockClient2.CreateTeamFunc = func(ctx context.Context, org string, newTeam *github.NewTeam) (*github.Team, error) {
					return nil, errors.New("insufficient permissions to create team")
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
							Previous: []reconciler.GitHub[string]{
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

				err = rec.reconcileTeam(ctx)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("insufficient permissions to create team"))
			})

			It("should have processed org1 successfully before failing on org2", func() {
				org1Calls := mockClient1.GetTeamCalls()
				Expect(org1Calls).To(HaveLen(1))
			})
		})
	})

	Describe("Removing organizations from spec.organizationRefs", func() {
		Context("when removing one organization from a multi-org team", func() {
			BeforeEach(func() {
				team = &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-team",
						Namespace: "default",
					},
					Spec: v1alpha1.TeamSpec{
						Name:    "test-team",
						Members: []string{"user1"},
						OrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"}, // org2 removed
						},
					},
					Status: v1alpha1.TeamStatus{
						Slug: github.Ptr("test-team"),
						PreviousOrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
							{Name: "org2"},
						},
					},
				}

				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(team).
					WithStatusSubresource(team).
					Build()

				mockClient2.DeleteTeamBySlugFunc = func(ctx context.Context, org string, slug string) error {
					return nil
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
							Previous: []reconciler.GitHub[string]{
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

				err = rec.reconcileRemovedOrgRefs(ctx)
			})

			It("should succeed without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should delete team from removed org2", func() {
				org2Calls := mockClient2.GetTeamCalls()
				Expect(org2Calls).To(HaveLen(1))
				Expect(org2Calls[0].Method).To(Equal("DeleteTeamBySlug"))
				Expect(org2Calls[0].Org).To(Equal("org2"))
				Expect(org2Calls[0].Slug).To(Equal("test-team"))
			})

			It("should not touch remaining org1", func() {
				org1Calls := mockClient1.GetTeamCalls()
				Expect(org1Calls).To(BeEmpty())
			})

			It("should update previousOrganizationRefs status", func() {

				Expect(team.Status.PreviousOrganizationRefs).To(HaveLen(1))
				Expect(team.Status.PreviousOrganizationRefs[0].Name).To(Equal("org1"))
			})
		})

		Context("when removing all organizations except one", func() {
			BeforeEach(func() {
				team = &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-team",
						Namespace: "default",
					},
					Spec: v1alpha1.TeamSpec{
						Name:    "test-team",
						Members: []string{"user1"},
						OrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"}, // org2 and org3 removed
						},
					},
					Status: v1alpha1.TeamStatus{
						Slug: github.Ptr("test-team"),
						PreviousOrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
							{Name: "org2"},
							{Name: "org3"},
						},
					},
				}

				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(team).
					WithStatusSubresource(team).
					Build()

				mockClient2.DeleteTeamBySlugFunc = func(ctx context.Context, org string, slug string) error {
					return nil
				}

				mockClient3.DeleteTeamBySlugFunc = func(ctx context.Context, org string, slug string) error {
					return nil
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
							Previous: []reconciler.GitHub[string]{
								{
									Client:   mockClient1,
									Resource: "org1",
								},
								{
									Client:   mockClient2,
									Resource: "org2",
								},
								{
									Client:   mockClient3,
									Resource: "org3",
								},
							},
						},
					},
					Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
						Client:   k8sClient,
						Resource: team,
					},
				}

				err = rec.reconcileRemovedOrgRefs(ctx)
			})

			It("should succeed without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should delete team from both removed organizations", func() {
				org2Calls := mockClient2.GetTeamCalls()
				Expect(org2Calls).To(HaveLen(1))
				Expect(org2Calls[0].Method).To(Equal("DeleteTeamBySlug"))

				org3Calls := mockClient3.GetTeamCalls()
				Expect(org3Calls).To(HaveLen(1))
				Expect(org3Calls[0].Method).To(Equal("DeleteTeamBySlug"))
			})

			It("should update previousOrganizationRefs to only remaining org", func() {

				Expect(team.Status.PreviousOrganizationRefs).To(HaveLen(1))
				Expect(team.Status.PreviousOrganizationRefs[0].Name).To(Equal("org1"))
			})
		})

		Context("when deletion fails in one of the removed organizations", func() {
			BeforeEach(func() {
				team = &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-team",
						Namespace: "default",
					},
					Spec: v1alpha1.TeamSpec{
						Name:    "test-team",
						Members: []string{"user1"},
						OrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
						},
					},
					Status: v1alpha1.TeamStatus{
						Slug: github.Ptr("test-team"),
						PreviousOrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
							{Name: "org2"},
						},
					},
				}

				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(team).
					WithStatusSubresource(team).
					Build()

				mockClient2.DeleteTeamBySlugFunc = func(ctx context.Context, org string, slug string) error {
					return errors.New("deletion failed: insufficient permissions")
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
							Previous: []reconciler.GitHub[string]{
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

				err = rec.reconcileRemovedOrgRefs(ctx)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("deletion failed: insufficient permissions"))
			})

			It("should have attempted deletion", func() {
				org2Calls := mockClient2.GetTeamCalls()
				Expect(org2Calls).To(HaveLen(1))
				Expect(org2Calls[0].Method).To(Equal("DeleteTeamBySlug"))
			})

			It("should not update previousOrganizationRefs on error", func() {

				// Status should remain unchanged due to error
				Expect(team.Status.PreviousOrganizationRefs).To(HaveLen(2))
			})
		})
	})

	Describe("PreviousOrganizationRefs tracking", func() {
		Context("when first reconciliation with no previous refs", func() {
			BeforeEach(func() {
				team = &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-team",
						Namespace: "default",
					},
					Spec: v1alpha1.TeamSpec{
						Name:    "test-team",
						Members: []string{"user1"},
						OrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
							{Name: "org2"},
						},
					},
					Status: v1alpha1.TeamStatus{
						PreviousOrganizationRefs: nil, // first reconciliation
					},
				}

				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(team).
					WithStatusSubresource(team).
					Build()

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
							Previous: []reconciler.GitHub[string]{},
						},
					},
					Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
						Client:   k8sClient,
						Resource: team,
					},
				}

				err = rec.reconcileRemovedOrgRefs(ctx)
			})

			It("should succeed without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not attempt any deletions", func() {
				Expect(mockClient1.GetTeamCalls()).To(BeEmpty())
				Expect(mockClient2.GetTeamCalls()).To(BeEmpty())
			})

			It("should initialize previousOrganizationRefs with current refs", func() {

				Expect(team.Status.PreviousOrganizationRefs).To(HaveLen(2))
				Expect(team.Status.PreviousOrganizationRefs).To(ContainElement(v1alpha1.OrganizationRef{Name: "org1"}))
				Expect(team.Status.PreviousOrganizationRefs).To(ContainElement(v1alpha1.OrganizationRef{Name: "org2"}))
			})
		})

		Context("when previousOrganizationRefs matches current refs", func() {
			BeforeEach(func() {
				team = &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-team",
						Namespace: "default",
					},
					Spec: v1alpha1.TeamSpec{
						Name:    "test-team",
						Members: []string{"user1"},
						OrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
							{Name: "org2"},
						},
					},
					Status: v1alpha1.TeamStatus{
						Slug: github.Ptr("test-team"),
						PreviousOrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
							{Name: "org2"},
						},
					},
				}

				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(team).
					WithStatusSubresource(team).
					Build()

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
							Previous: []reconciler.GitHub[string]{
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

				err = rec.reconcileRemovedOrgRefs(ctx)
			})

			It("should succeed without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not attempt any deletions", func() {
				Expect(mockClient1.GetTeamCalls()).To(BeEmpty())
				Expect(mockClient2.GetTeamCalls()).To(BeEmpty())
			})

			It("should update status even when unchanged", func() {

				Expect(team.Status.PreviousOrganizationRefs).To(HaveLen(2))
			})
		})

		Context("when organization order changes but content is same", func() {
			BeforeEach(func() {
				team = &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-team",
						Namespace: "default",
					},
					Spec: v1alpha1.TeamSpec{
						Name:    "test-team",
						Members: []string{"user1"},
						OrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org2"}, // order swapped
							{Name: "org1"},
						},
					},
					Status: v1alpha1.TeamStatus{
						Slug: github.Ptr("test-team"),
						PreviousOrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
							{Name: "org2"},
						},
					},
				}

				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(team).
					WithStatusSubresource(team).
					Build()

				rec = &GitHubTeamReconciler{
					Team: reconciler.GitHubTeamIdentifier{
						Name: "test-team",
						Slug: github.Ptr("test-team"),
						Organizations: reconciler.ReferencedOrganizations{
							Current: []reconciler.GitHub[string]{
								{
									Client:   mockClient2,
									Resource: "org2",
								},
								{
									Client:   mockClient1,
									Resource: "org1",
								},
							},
							Previous: []reconciler.GitHub[string]{
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

				err = rec.reconcileRemovedOrgRefs(ctx)
			})

			It("should succeed without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not attempt any deletions when only order changes", func() {
				Expect(mockClient1.GetTeamCalls()).To(BeEmpty())
				Expect(mockClient2.GetTeamCalls()).To(BeEmpty())
			})

			It("should update previousOrganizationRefs with new order", func() {

				Expect(team.Status.PreviousOrganizationRefs).To(HaveLen(2))
				// Should reflect the new order from spec
				Expect(team.Status.PreviousOrganizationRefs[0].Name).To(Equal("org2"))
				Expect(team.Status.PreviousOrganizationRefs[1].Name).To(Equal("org1"))
			})
		})
	})

	Describe("Partial failure scenarios", func() {
		Context("when team creation succeeds in first org but fails in second", func() {
			BeforeEach(func() {
				team = &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-team",
						Namespace: "default",
					},
					Spec: v1alpha1.TeamSpec{
						Name:    "test-team",
						Members: []string{"user1"},
						OrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
							{Name: "org2"},
						},
					},
					Status: v1alpha1.TeamStatus{
						PreviousOrganizationRefs: []v1alpha1.OrganizationRef{},
					},
				}

				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(team).
					WithStatusSubresource(team).
					Build()

				// org1 - team doesn't exist, creation succeeds
				// When slug is nil, GetAllTeamsForOrg is called first
				mockClient1.GetAllTeamsForOrgFunc = func(ctx context.Context, org string) ([]*github.Team, error) {
					return []*github.Team{}, nil // team not found in list
				}

				mockClient1.CreateTeamFunc = func(ctx context.Context, org string, newTeam *github.NewTeam) (*github.Team, error) {
					return &github.Team{
						Name:                &newTeam.Name,
						Slug:                &newTeam.Name,
						Description:         newTeam.Description,
						Privacy:             newTeam.Privacy,
						Permission:          newTeam.Permission, //nolint:staticcheck
						NotificationSetting: newTeam.NotificationSetting,
					}, nil
				}

				// org2 - team doesn't exist, creation fails
				// After org1 succeeds and sets slug, GetTeamBySlug is called first
				mockClient2.GetTeamBySlugFunc = func(ctx context.Context, org, slug string) (*github.Team, error) {
					return nil, &github.ErrorResponse{
						Response: &http.Response{StatusCode: http.StatusNotFound},
						Message:  "Not Found",
					}
				}

				mockClient2.CreateTeamFunc = func(ctx context.Context, org string, newTeam *github.NewTeam) (*github.Team, error) {
					return nil, errors.New("API rate limit exceeded")
				}

				rec = &GitHubTeamReconciler{
					Team: reconciler.GitHubTeamIdentifier{
						Name: "test-team",
						Slug: nil, // no slug yet
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
							Previous: []reconciler.GitHub[string]{},
						},
					},
					Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
						Client:   k8sClient,
						Resource: team,
					},
				}

				err = rec.reconcileTeam(ctx)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("API rate limit exceeded"))
			})

			It("should have created team in org1 before failing", func() {
				org1Calls := mockClient1.GetTeamCalls()
				Expect(org1Calls).To(HaveLen(2))
				Expect(org1Calls[0].Method).To(Equal("GetAllTeamsForOrg"))
				Expect(org1Calls[1].Method).To(Equal("CreateTeam"))
			})

			It("should have attempted but failed to create team in org2", func() {
				org2Calls := mockClient2.GetTeamCalls()
				Expect(org2Calls).To(HaveLen(2))
				Expect(org2Calls[0].Method).To(Equal("GetTeamBySlug")) // slug is set after org1, so we expect GetTeamBySlug
				Expect(org2Calls[1].Method).To(Equal("CreateTeam"))
			})

			It("should have updated team slug from successful org1 creation", func() {
				Expect(team.Status.Slug).NotTo(BeNil())
				Expect(*team.Status.Slug).To(Equal("test-team"))
			})
		})

		Context("when team update succeeds in first org but fails in second", func() {
			BeforeEach(func() {
				team = &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-team",
						Namespace: "default",
					},
					Spec: v1alpha1.TeamSpec{
						Name:        "test-team",
						Description: "New description",
						Members:     []string{"user1"},
						OrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
							{Name: "org2"},
						},
					},
					Status: v1alpha1.TeamStatus{
						Slug: github.Ptr("test-team"),
						PreviousOrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
							{Name: "org2"},
						},
					},
				}

				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(team).
					WithStatusSubresource(team).
					Build()

				// org1 - team exists with old description, update succeeds
				mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
					return &github.Team{
						Name:                github.Ptr("test-team"),
						Slug:                github.Ptr("test-team"),
						Description:         github.Ptr("Old description"),
						Privacy:             github.Ptr("closed"),
						Permission:          github.Ptr("pull"),
						NotificationSetting: github.Ptr("notifications_disabled"),
					}, nil
				}

				mockClient1.EditTeamBySlugFunc = func(ctx context.Context, org string, slug string, newTeam *github.NewTeam) (*github.Team, error) {
					return &github.Team{
						Name:                &newTeam.Name,
						Slug:                &newTeam.Name,
						Description:         newTeam.Description,
						Privacy:             newTeam.Privacy,
						Permission:          newTeam.Permission, //nolint:staticcheck
						NotificationSetting: newTeam.NotificationSetting,
					}, nil
				}

				// org2 - team exists with old description, update fails
				mockClient2.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
					return &github.Team{
						Name:                github.Ptr("test-team"),
						Slug:                github.Ptr("test-team"),
						Description:         github.Ptr("Old description"),
						Privacy:             github.Ptr("closed"),
						Permission:          github.Ptr("pull"),
						NotificationSetting: github.Ptr("notifications_disabled"),
					}, nil
				}

				mockClient2.EditTeamBySlugFunc = func(ctx context.Context, org string, slug string, newTeam *github.NewTeam) (*github.Team, error) {
					return nil, errors.New("conflict: team was modified by another process")
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
							Previous: []reconciler.GitHub[string]{
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

				err = rec.reconcileTeam(ctx)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("conflict: team was modified by another process"))
			})

			It("should have updated team in org1 before failing", func() {
				org1Calls := mockClient1.GetTeamCalls()
				Expect(org1Calls).To(HaveLen(2))
				Expect(org1Calls[0].Method).To(Equal("GetTeamBySlug"))
				Expect(org1Calls[1].Method).To(Equal("EditTeamBySlug"))
			})

			It("should have attempted but failed to update team in org2", func() {
				org2Calls := mockClient2.GetTeamCalls()
				Expect(org2Calls).To(HaveLen(2))
				Expect(org2Calls[0].Method).To(Equal("GetTeamBySlug"))
				Expect(org2Calls[1].Method).To(Equal("EditTeamBySlug"))
			})
		})

		Context("when deletion succeeds in first removed org but fails in second", func() {
			BeforeEach(func() {
				team = &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-team",
						Namespace: "default",
					},
					Spec: v1alpha1.TeamSpec{
						Name:    "test-team",
						Members: []string{"user1"},
						OrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
						},
					},
					Status: v1alpha1.TeamStatus{
						Slug: github.Ptr("test-team"),
						PreviousOrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
							{Name: "org2"},
							{Name: "org3"},
						},
					},
				}

				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(team).
					WithStatusSubresource(team).
					Build()

				// org2 deletion succeeds
				mockClient2.DeleteTeamBySlugFunc = func(ctx context.Context, org string, slug string) error {
					return nil
				}

				// org3 deletion fails
				mockClient3.DeleteTeamBySlugFunc = func(ctx context.Context, org string, slug string) error {
					return errors.New("team is referenced by branch protection rules")
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
							Previous: []reconciler.GitHub[string]{
								{
									Client:   mockClient1,
									Resource: "org1",
								},
								{
									Client:   mockClient2,
									Resource: "org2",
								},
								{
									Client:   mockClient3,
									Resource: "org3",
								},
							},
						},
					},
					Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
						Client:   k8sClient,
						Resource: team,
					},
				}

				err = rec.reconcileRemovedOrgRefs(ctx)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("team is referenced by branch protection rules"))
			})

			It("should have deleted from org2 before failing on org3", func() {
				org2Calls := mockClient2.GetTeamCalls()
				Expect(org2Calls).To(HaveLen(1))
				Expect(org2Calls[0].Method).To(Equal("DeleteTeamBySlug"))
			})

			It("should have attempted deletion from org3", func() {
				org3Calls := mockClient3.GetTeamCalls()
				Expect(org3Calls).To(HaveLen(1))
				Expect(org3Calls[0].Method).To(Equal("DeleteTeamBySlug"))
			})

			It("should not update previousOrganizationRefs due to failure", func() {

				// Status should remain unchanged
				Expect(team.Status.PreviousOrganizationRefs).To(HaveLen(3))
			})
		})

		Context("when mix of add, keep, and remove operations with partial failure", func() {
			BeforeEach(func() {
				team = &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-team",
						Namespace: "default",
					},
					Spec: v1alpha1.TeamSpec{
						Name:    "test-team",
						Members: []string{"user1"},
						OrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"}, // keep
							{Name: "org3"}, // add
							// org2 removed
						},
					},
					Status: v1alpha1.TeamStatus{
						Slug: github.Ptr("test-team"),
						PreviousOrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
							{Name: "org2"},
						},
					},
				}

				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(team).
					WithStatusSubresource(team).
					Build()

				// org1 - already exists, no change needed
				mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
					return &github.Team{
						Name:                github.Ptr("test-team"),
						Slug:                github.Ptr("test-team"),
						Description:         github.Ptr(""),
						Privacy:             github.Ptr("closed"),
						Permission:          github.Ptr("pull"),
						NotificationSetting: github.Ptr("notifications_disabled"),
					}, nil
				}

				// org2 - should be deleted
				mockClient2.DeleteTeamBySlugFunc = func(ctx context.Context, org string, slug string) error {
					return nil
				}

				// org3 - should be created, but fails
				mockClient3.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
					return nil, &github.ErrorResponse{
						Message: "Not Found",
						Response: &http.Response{
							StatusCode: http.StatusNotFound,
						},
					}
				}

				mockClient3.CreateTeamFunc = func(ctx context.Context, org string, newTeam *github.NewTeam) (*github.Team, error) {
					return nil, errors.New("organization has reached team limit")
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
									Client:   mockClient3,
									Resource: "org3",
								},
							},
							Previous: []reconciler.GitHub[string]{
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

				// First reconcile the team (will fail on org3)
				err = rec.reconcileTeam(ctx)
			})

			It("should fail on team reconciliation", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("organization has reached team limit"))
			})

			It("should have kept org1 unchanged", func() {
				org1Calls := mockClient1.GetTeamCalls()
				Expect(org1Calls).To(HaveLen(1))
				Expect(org1Calls[0].Method).To(Equal("GetTeamBySlug"))
			})

			It("should have attempted to create team in org3", func() {
				org3Calls := mockClient3.GetTeamCalls()
				Expect(org3Calls).To(HaveLen(2))
				Expect(org3Calls[0].Method).To(Equal("GetTeamBySlug"))
				Expect(org3Calls[1].Method).To(Equal("CreateTeam"))
			})

			It("should not have deleted org2 yet (deletion happens in separate reconciliation)", func() {
				// reconcileRemovedOrgRefs is a separate step that wasn't called in this test
				org2Calls := mockClient2.GetTeamCalls()
				Expect(org2Calls).To(BeEmpty())
			})
		})
	})

	Describe("Multi-organization deletion scenarios", func() {
		Context("when deleting team from all organizations successfully", func() {
			BeforeEach(func() {
				team = &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-team",
						Namespace: "default",
					},
					Spec: v1alpha1.TeamSpec{
						Name:    "test-team",
						Members: []string{"user1"},
						OrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
							{Name: "org2"},
						},
					},
					Status: v1alpha1.TeamStatus{
						Slug: github.Ptr("test-team"),
					},
				}

				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(team).
					WithStatusSubresource(team).
					Build()

				mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
					return &github.Team{
						Name: github.Ptr("test-team"),
						Slug: github.Ptr("test-team"),
					}, nil
				}

				mockClient1.DeleteTeamBySlugFunc = func(ctx context.Context, org string, slug string) error {
					return nil
				}

				mockClient2.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
					return &github.Team{
						Name: github.Ptr("test-team"),
						Slug: github.Ptr("test-team"),
					}, nil
				}

				mockClient2.DeleteTeamBySlugFunc = func(ctx context.Context, org string, slug string) error {
					return nil
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

				err = rec.ReconcileDeletion(ctx)
			})

			It("should succeed without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should delete team from both organizations", func() {
				org1Calls := mockClient1.GetTeamCalls()
				Expect(org1Calls).To(HaveLen(2))
				Expect(org1Calls[0].Method).To(Equal("GetTeamBySlug"))
				Expect(org1Calls[1].Method).To(Equal("DeleteTeamBySlug"))

				org2Calls := mockClient2.GetTeamCalls()
				Expect(org2Calls).To(HaveLen(2))
				Expect(org2Calls[0].Method).To(Equal("GetTeamBySlug"))
				Expect(org2Calls[1].Method).To(Equal("DeleteTeamBySlug"))
			})
		})

		Context("when deletion fails in second organization", func() {
			BeforeEach(func() {
				team = &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-team",
						Namespace: "default",
					},
					Spec: v1alpha1.TeamSpec{
						Name:    "test-team",
						Members: []string{"user1"},
						OrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
							{Name: "org2"},
						},
					},
					Status: v1alpha1.TeamStatus{
						Slug: github.Ptr("test-team"),
					},
				}

				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(team).
					WithStatusSubresource(team).
					Build()

				// org1 deletion succeeds
				mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
					return &github.Team{
						Name: github.Ptr("test-team"),
						Slug: github.Ptr("test-team"),
					}, nil
				}

				mockClient1.DeleteTeamBySlugFunc = func(ctx context.Context, org string, slug string) error {
					return nil
				}

				// org2 deletion fails
				mockClient2.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
					return &github.Team{
						Name: github.Ptr("test-team"),
						Slug: github.Ptr("test-team"),
					}, nil
				}

				mockClient2.DeleteTeamBySlugFunc = func(ctx context.Context, org string, slug string) error {
					return errors.New("cannot delete team: team is assigned to repositories")
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

				err = rec.ReconcileDeletion(ctx)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot delete team: team is assigned to repositories"))
			})

			It("should have deleted from org1 before failing on org2", func() {
				org1Calls := mockClient1.GetTeamCalls()
				Expect(org1Calls).To(HaveLen(2))
				Expect(org1Calls[0].Method).To(Equal("GetTeamBySlug"))
				Expect(org1Calls[1].Method).To(Equal("DeleteTeamBySlug"))
			})

			It("should have attempted deletion from org2", func() {
				org2Calls := mockClient2.GetTeamCalls()
				Expect(org2Calls).To(HaveLen(2))
				Expect(org2Calls[0].Method).To(Equal("GetTeamBySlug"))
				Expect(org2Calls[1].Method).To(Equal("DeleteTeamBySlug"))
			})
		})

		Context("when team is already deleted from one organization", func() {
			BeforeEach(func() {
				team = &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-team",
						Namespace: "default",
					},
					Spec: v1alpha1.TeamSpec{
						Name:    "test-team",
						Members: []string{"user1"},
						OrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
							{Name: "org2"},
						},
					},
					Status: v1alpha1.TeamStatus{
						Slug: github.Ptr("test-team"),
					},
				}

				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(team).
					WithStatusSubresource(team).
					Build()

				// org1 - team exists and should be deleted
				mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
					return &github.Team{
						Name: github.Ptr("test-team"),
						Slug: github.Ptr("test-team"),
					}, nil
				}

				mockClient1.DeleteTeamBySlugFunc = func(ctx context.Context, org string, slug string) error {
					return nil
				}

				// org2 - team already deleted (returns nil)
				mockClient2.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
					return nil, nil // team not found
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

				err = rec.ReconcileDeletion(ctx)
			})

			It("should succeed without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should delete team from org1 first", func() {
				org1Calls := mockClient1.GetTeamCalls()
				Expect(org1Calls).To(HaveLen(2))
				Expect(org1Calls[0].Method).To(Equal("GetTeamBySlug"))
				Expect(org1Calls[1].Method).To(Equal("DeleteTeamBySlug"))
			})

			It("should process org2 and find team already deleted", func() {
				org2Calls := mockClient2.GetTeamCalls()
				Expect(org2Calls).To(HaveLen(1))
				Expect(org2Calls[0].Method).To(Equal("GetTeamBySlug"))
				// No DeleteTeamBySlug call because team is already deleted
			})
		})

		Context("when GetTeamBySlug fails during deletion", func() {
			BeforeEach(func() {
				team = &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-team",
						Namespace: "default",
					},
					Spec: v1alpha1.TeamSpec{
						Name:    "test-team",
						Members: []string{"user1"},
						OrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: "org1"},
						},
					},
					Status: v1alpha1.TeamStatus{
						Slug: github.Ptr("test-team"),
					},
				}

				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(team).
					WithStatusSubresource(team).
					Build()

				mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
					return nil, errors.New("API timeout")
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

				err = rec.ReconcileDeletion(ctx)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("API timeout"))
			})

			It("should not attempt deletion when get fails", func() {
				org1Calls := mockClient1.GetTeamCalls()
				Expect(org1Calls).To(HaveLen(1))
				Expect(org1Calls[0].Method).To(Equal("GetTeamBySlug"))
				// No DeleteTeamBySlug call should have been made
			})
		})
	})
})
