package teamrec

import (
	"context"
	"errors"
	"net/http"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	"github.com/google/go-github/v89/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ReconcileTeam", func() {
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
				},
			},
		}

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(team).
			WithStatusSubresource(team).
			Build()
	})

	Context("when team does not exist on GitHub", func() {
		BeforeEach(func() {
			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return nil, &github.ErrorResponse{
					Message: "Not Found",
					Response: &http.Response{
						StatusCode: http.StatusNotFound,
					},
				}
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

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: new("test-team"),
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

			err = rec.reconcileTeam(ctx)
		})

		It("should create the team", func() {
			Expect(err).NotTo(HaveOccurred())
			teamCalls := mockClient1.GetTeamCalls()

			Expect(teamCalls).To(HaveLen(2))
			Expect(teamCalls[0].Method).To(Equal("GetTeamBySlug"))
			Expect(teamCalls[1].Method).To(Equal("CreateTeam"))
		})
	})

	Context("when team exists and matches desired state", func() {
		BeforeEach(func() {
			// Team has a custom description "Test team description", so that's what should be used
			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					Name:                new("test-team"),
					Slug:                new("test-team"),
					Description:         new("Test team description"),
					Privacy:             new("closed"),
					Permission:          new("pull"),
					NotificationSetting: new("notifications_disabled"),
				}, nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: new("test-team"),
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

			err = rec.reconcileTeam(ctx)
		})

		It("should not update the team", func() {
			Expect(err).NotTo(HaveOccurred())
			teamCalls := mockClient1.GetTeamCalls()

			Expect(teamCalls).To(HaveLen(1))
			Expect(teamCalls[0].Method).To(Equal("GetTeamBySlug"))
			// No EditTeamBySlug call should be made
		})
	})

	Context("when team exists but has different description", func() {
		BeforeEach(func() {
			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					Name:                new("test-team"),
					Slug:                new("test-team"),
					Description:         new("Old description"),
					Privacy:             new("closed"),
					Permission:          new("pull"),
					NotificationSetting: new("notifications_disabled"),
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

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: new("test-team"),
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

			err = rec.reconcileTeam(ctx)
		})

		It("should update the team", func() {
			Expect(err).NotTo(HaveOccurred())
			teamCalls := mockClient1.GetTeamCalls()

			Expect(teamCalls).To(HaveLen(2))
			Expect(teamCalls[0].Method).To(Equal("GetTeamBySlug"))
			Expect(teamCalls[1].Method).To(Equal("EditTeamBySlug"))
		})
	})

	Context("when team exists but has different privacy setting", func() {
		BeforeEach(func() {
			expectedDescription := "⚠️ To join the team, create a pull request here: https://github.com/org1/github-configuration-deployment/blob/main/teams/test-team.yaml"
			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					Name:                new("test-team"),
					Slug:                new("test-team"),
					Description:         &expectedDescription,
					Privacy:             new("secret"),
					Permission:          new("pull"),
					NotificationSetting: new("notifications_disabled"),
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

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: new("test-team"),
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

			err = rec.reconcileTeam(ctx)
		})

		It("should update the team to closed privacy", func() {
			Expect(err).NotTo(HaveOccurred())
			teamCalls := mockClient1.GetTeamCalls()

			Expect(teamCalls).To(HaveLen(2))
			Expect(teamCalls[1].Method).To(Equal("EditTeamBySlug"))
		})
	})

	Context("when team exists but has different permission", func() {
		BeforeEach(func() {
			expectedDescription := "⚠️ To join the team, create a pull request here: https://github.com/org1/github-configuration-deployment/blob/main/teams/test-team.yaml"
			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					Name:                new("test-team"),
					Slug:                new("test-team"),
					Description:         &expectedDescription,
					Privacy:             new("closed"),
					Permission:          new("push"),
					NotificationSetting: new("notifications_disabled"),
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

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: new("test-team"),
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

			err = rec.reconcileTeam(ctx)
		})

		It("should update the team to pull permission", func() {
			Expect(err).NotTo(HaveOccurred())
			teamCalls := mockClient1.GetTeamCalls()

			Expect(teamCalls).To(HaveLen(2))
			Expect(teamCalls[1].Method).To(Equal("EditTeamBySlug"))
		})
	})

	Context("when team exists but has different notification setting", func() {
		BeforeEach(func() {
			expectedDescription := "⚠️ To join the team, create a pull request here: https://github.com/org1/github-configuration-deployment/blob/main/teams/test-team.yaml"
			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					Name:                new("test-team"),
					Slug:                new("test-team"),
					Description:         &expectedDescription,
					Privacy:             new("closed"),
					Permission:          new("pull"),
					NotificationSetting: new("notifications_enabled"),
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

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: new("test-team"),
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

			err = rec.reconcileTeam(ctx)
		})

		It("should update the team to notifications_disabled", func() {
			Expect(err).NotTo(HaveOccurred())
			teamCalls := mockClient1.GetTeamCalls()

			Expect(teamCalls).To(HaveLen(2))
			Expect(teamCalls[1].Method).To(Equal("EditTeamBySlug"))
		})
	})

	Context("when GetTeamBySlug fails with non-404 error", func() {
		BeforeEach(func() {
			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return nil, errors.New("API error")
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: new("test-team"),
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

			err = rec.reconcileTeam(ctx)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API error"))
		})
	})

	Context("when CreateTeam fails", func() {
		BeforeEach(func() {
			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return nil, &github.ErrorResponse{
					Message: "Not Found",
					Response: &http.Response{
						StatusCode: http.StatusNotFound,
					},
				}
			}

			mockClient1.CreateTeamFunc = func(ctx context.Context, org string, newTeam *github.NewTeam) (*github.Team, error) {
				return nil, errors.New("failed to create team")
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: new("test-team"),
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

			err = rec.reconcileTeam(ctx)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to create team"))
		})
	})

	Context("when EditTeamBySlug fails", func() {
		BeforeEach(func() {
			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					Name:                new("test-team"),
					Slug:                new("test-team"),
					Description:         new("Old description"),
					Privacy:             new("closed"),
					Permission:          new("pull"),
					NotificationSetting: new("notifications_disabled"),
				}, nil
			}

			mockClient1.EditTeamBySlugFunc = func(ctx context.Context, org string, slug string, newTeam *github.NewTeam) (*github.Team, error) {
				return nil, errors.New("failed to update team")
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: new("test-team"),
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

			err = rec.reconcileTeam(ctx)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to update team"))
		})
	})

	Context("when reconciling team in multiple organizations", func() {
		BeforeEach(func() {
			team.Spec.OrganizationRefs = []v1alpha1.OrganizationRef{
				{Name: "org1"},
				{Name: "org2"},
			}

			// Team has custom description "Test team description", so that's what should be used
			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					Name:                new("test-team"),
					Slug:                new("test-team"),
					Description:         new("Test team description"),
					Privacy:             new("closed"),
					Permission:          new("pull"),
					NotificationSetting: new("notifications_disabled"),
				}, nil
			}

			mockClient2.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					Name:                new("test-team"),
					Slug:                new("test-team"),
					Description:         new("Old description"),
					Privacy:             new("closed"),
					Permission:          new("pull"),
					NotificationSetting: new("notifications_disabled"),
				}, nil
			}

			mockClient2.EditTeamBySlugFunc = func(ctx context.Context, org string, slug string, newTeam *github.NewTeam) (*github.Team, error) {
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
					Slug: new("test-team"),
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

			err = rec.reconcileTeam(ctx)
		})

		It("should reconcile team in both organizations", func() {
			Expect(err).NotTo(HaveOccurred())

			// org1 should only query (no changes needed)
			org1Calls := mockClient1.GetTeamCalls()
			Expect(org1Calls).To(HaveLen(1))
			Expect(org1Calls[0].Method).To(Equal("GetTeamBySlug"))

			// org2 should query and update
			org2Calls := mockClient2.GetTeamCalls()
			Expect(org2Calls).To(HaveLen(2))
			Expect(org2Calls[0].Method).To(Equal("GetTeamBySlug"))
			Expect(org2Calls[1].Method).To(Equal("EditTeamBySlug"))
		})
	})

	Context("when team is an IDP team", func() {
		BeforeEach(func() {
			idpGroup := "test-idp-group"
			team.Spec.IDPGroup = &idpGroup
			team.Spec.Members = nil

			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					Name:                new("test-team"),
					Slug:                new("test-team"),
					Description:         new("Test team description"),
					Privacy:             new("closed"),
					Permission:          new("pull"),
					NotificationSetting: new("notifications_disabled"),
				}, nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: new("test-team"),
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

			err = rec.reconcileTeam(ctx)
		})

		It("should use IDP team description (not generated)", func() {
			Expect(err).NotTo(HaveOccurred())
			// IDP teams use the spec.Description directly, not the generated one
			teamCalls := mockClient1.GetTeamCalls()
			Expect(teamCalls).To(HaveLen(1))
		})
	})

	Context("when team has empty description for non-IDP team", func() {
		BeforeEach(func() {
			team.Spec.Description = ""

			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					Name:                new("test-team"),
					Slug:                new("test-team"),
					Description:         new(""),
					Privacy:             new("closed"),
					Permission:          new("pull"),
					NotificationSetting: new("notifications_disabled"),
				}, nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: new("test-team"),
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

			err = rec.reconcileTeam(ctx)
		})

		It("should not update team when description matches", func() {
			Expect(err).NotTo(HaveOccurred())
			teamCalls := mockClient1.GetTeamCalls()
			Expect(teamCalls).To(HaveLen(1))
			// Should not update because empty description matches
		})
	})

	Context("when reconciling with empty organization list", func() {
		BeforeEach(func() {
			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: new("test-team"),
					Organizations: reconciler.ReferencedOrganizations{
						Current: []reconciler.GitHub[string]{},
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Team]{
					Client:   k8sClient,
					Resource: team,
				},
			}

			err = rec.reconcileTeam(ctx)
		})

		It("should succeed without any API calls", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(mockClient1.GetTeamCalls()).To(BeEmpty())
		})
	})

	Context("when GitHub team has nil fields", func() {
		BeforeEach(func() {
			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					Name: new("test-team"),
					Slug: new("test-team"),
					// All other fields are nil
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

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: new("test-team"),
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

			err = rec.reconcileTeam(ctx)
		})

		It("should update the team with all required fields", func() {
			Expect(err).NotTo(HaveOccurred())
			teamCalls := mockClient1.GetTeamCalls()

			Expect(teamCalls).To(HaveLen(2))
			Expect(teamCalls[1].Method).To(Equal("EditTeamBySlug"))
		})
	})

	Context("when first organization succeeds but second fails", func() {
		BeforeEach(func() {
			team.Spec.OrganizationRefs = []v1alpha1.OrganizationRef{
				{Name: "org1"},
				{Name: "org2"},
			}

			// Team has custom description "Test team description"
			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					Name:                new("test-team"),
					Slug:                new("test-team"),
					Description:         new("Test team description"),
					Privacy:             new("closed"),
					Permission:          new("pull"),
					NotificationSetting: new("notifications_disabled"),
				}, nil
			}

			mockClient2.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return nil, errors.New("API error in org2")
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: new("test-team"),
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

			err = rec.reconcileTeam(ctx)
		})

		It("should fail after processing first org", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API error in org2"))

			// org1 should have been processed successfully
			org1Calls := mockClient1.GetTeamCalls()
			Expect(org1Calls).To(HaveLen(1))

			// org2 should have attempted the query
			org2Calls := mockClient2.GetTeamCalls()
			Expect(org2Calls).To(HaveLen(1))
		})
	})
})
