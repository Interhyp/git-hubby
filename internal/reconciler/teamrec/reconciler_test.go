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

var _ = Describe("ReconcileDeletion", func() {
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
				Description: "Test team",
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

	JustBeforeEach(func() {
		err = rec.ReconcileDeletion(ctx)
	})

	Context("when team exists in single organization", func() {
		BeforeEach(func() {
			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					Name: new("test-team"),
					Slug: new("test-team"),
				}, nil
			}

			mockClient1.DeleteTeamBySlugFunc = func(ctx context.Context, org string, slug string) error {
				return nil
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
		})

		It("should delete the team successfully", func() {
			Expect(err).NotTo(HaveOccurred())

			teamCalls := mockClient1.GetTeamCalls()
			Expect(teamCalls).To(HaveLen(2))
			Expect(teamCalls[0].Method).To(Equal("GetTeamBySlug"))
			Expect(teamCalls[0].Org).To(Equal("org1"))
			Expect(teamCalls[0].Slug).To(Equal("test-team"))

			Expect(teamCalls[1].Method).To(Equal("DeleteTeamBySlug"))
			Expect(teamCalls[1].Org).To(Equal("org1"))
			Expect(teamCalls[1].Slug).To(Equal("test-team"))
		})
	})

	Context("when team exists in multiple organizations", func() {
		BeforeEach(func() {
			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					Name: new("test-team"),
					Slug: new("test-team"),
				}, nil
			}

			mockClient1.DeleteTeamBySlugFunc = func(ctx context.Context, org string, slug string) error {
				return nil
			}

			mockClient2.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					Name: new("test-team"),
					Slug: new("test-team"),
				}, nil
			}

			mockClient2.DeleteTeamBySlugFunc = func(ctx context.Context, org string, slug string) error {
				return nil
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
		})

		It("should delete the team from all organizations", func() {
			Expect(err).NotTo(HaveOccurred())

			teamCalls1 := mockClient1.GetTeamCalls()
			Expect(teamCalls1).To(HaveLen(2))
			Expect(teamCalls1[0].Method).To(Equal("GetTeamBySlug"))
			Expect(teamCalls1[0].Org).To(Equal("org1"))
			Expect(teamCalls1[1].Method).To(Equal("DeleteTeamBySlug"))
			Expect(teamCalls1[1].Org).To(Equal("org1"))

			teamCalls2 := mockClient2.GetTeamCalls()
			Expect(teamCalls2).To(HaveLen(2))
			Expect(teamCalls2[0].Method).To(Equal("GetTeamBySlug"))
			Expect(teamCalls2[0].Org).To(Equal("org2"))
			Expect(teamCalls2[1].Method).To(Equal("DeleteTeamBySlug"))
			Expect(teamCalls2[1].Org).To(Equal("org2"))
		})
	})

	Context("when team is already deleted from GitHub", func() {
		BeforeEach(func() {
			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return nil, &github.ErrorResponse{
					Message: "Not Found",
					Response: &http.Response{
						StatusCode: http.StatusNotFound,
					},
				}
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
		})

		It("should not return an error", func() {
			Expect(err).To(Not(HaveOccurred()))

			teamCalls := mockClient1.GetTeamCalls()
			Expect(teamCalls).To(HaveLen(1))
			Expect(teamCalls[0].Method).To(Equal("GetTeamBySlug"))
			// DeleteTeamBySlug should not be called when GetTeamBySlug fails
		})
	})

	Context("when GetTeamBySlug returns nil team without error", func() {
		BeforeEach(func() {
			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return nil, nil
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
		})

		It("should consider team already deleted and return no error", func() {
			Expect(err).NotTo(HaveOccurred())

			teamCalls := mockClient1.GetTeamCalls()
			Expect(teamCalls).To(HaveLen(1))
			Expect(teamCalls[0].Method).To(Equal("GetTeamBySlug"))
			// DeleteTeamBySlug should not be called when team is nil
		})
	})

	Context("when GetTeamBySlug fails with generic error", func() {
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
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("API error"))

			teamCalls := mockClient1.GetTeamCalls()
			Expect(teamCalls).To(HaveLen(1))
			Expect(teamCalls[0].Method).To(Equal("GetTeamBySlug"))
			// DeleteTeamBySlug should not be called when GetTeamBySlug fails
		})
	})

	Context("when DeleteTeamBySlug fails", func() {
		BeforeEach(func() {
			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					Name: new("test-team"),
					Slug: new("test-team"),
				}, nil
			}

			mockClient1.DeleteTeamBySlugFunc = func(ctx context.Context, org string, slug string) error {
				return errors.New("delete failed")
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
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("delete failed"))

			teamCalls := mockClient1.GetTeamCalls()
			Expect(teamCalls).To(HaveLen(2))
			Expect(teamCalls[0].Method).To(Equal("GetTeamBySlug"))
			Expect(teamCalls[1].Method).To(Equal("DeleteTeamBySlug"))
		})
	})

	Context("when team exists in multiple organizations but deletion fails in first organization", func() {
		BeforeEach(func() {
			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					Name: new("test-team"),
					Slug: new("test-team"),
				}, nil
			}

			mockClient1.DeleteTeamBySlugFunc = func(ctx context.Context, org string, slug string) error {
				return errors.New("delete failed in org1")
			}

			mockClient2.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					Name: new("test-team"),
					Slug: new("test-team"),
				}, nil
			}

			mockClient2.DeleteTeamBySlugFunc = func(ctx context.Context, org string, slug string) error {
				return nil
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
		})

		It("should fail early and not attempt deletion in second organization", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("delete failed in org1"))

			teamCalls1 := mockClient1.GetTeamCalls()
			Expect(teamCalls1).To(HaveLen(2))
			Expect(teamCalls1[0].Method).To(Equal("GetTeamBySlug"))
			Expect(teamCalls1[1].Method).To(Equal("DeleteTeamBySlug"))

			// Second org should not be processed due to early return on error
			teamCalls2 := mockClient2.GetTeamCalls()
			Expect(teamCalls2).To(BeEmpty())
		})
	})

	Context("when team slug is different from team name", func() {
		BeforeEach(func() {
			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					Name: new("Test Team"),
					Slug: new("test-team"),
				}, nil
			}

			mockClient1.DeleteTeamBySlugFunc = func(ctx context.Context, org string, slug string) error {
				Expect(slug).To(Equal("test-team"))
				return nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "Test Team",
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
		})

		It("should use the team slug for deletion", func() {
			Expect(err).NotTo(HaveOccurred())

			teamCalls := mockClient1.GetTeamCalls()
			Expect(teamCalls).To(HaveLen(2))
			Expect(teamCalls[0].Method).To(Equal("GetTeamBySlug"))
			Expect(teamCalls[0].Slug).To(Equal("test-team"))
			Expect(teamCalls[1].Method).To(Equal("DeleteTeamBySlug"))
			Expect(teamCalls[1].Slug).To(Equal("test-team"))
		})
	})

	Context("when team has empty slug", func() {
		BeforeEach(func() {
			mockClient1.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				Expect(slug).To(Equal(""))
				return &github.Team{
					Name: new("test-team"),
					Slug: new(""),
				}, nil
			}

			mockClient1.DeleteTeamBySlugFunc = func(ctx context.Context, org string, slug string) error {
				return nil
			}

			rec = &GitHubTeamReconciler{
				Team: reconciler.GitHubTeamIdentifier{
					Name: "test-team",
					Slug: new(""),
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
		})

		It("should handle empty slug gracefully", func() {
			Expect(err).NotTo(HaveOccurred())

			teamCalls := mockClient1.GetTeamCalls()
			Expect(teamCalls).To(HaveLen(2))
			Expect(teamCalls[0].Method).To(Equal("GetTeamBySlug"))
			Expect(teamCalls[0].Slug).To(Equal(""))
			Expect(teamCalls[1].Method).To(Equal("DeleteTeamBySlug"))
			Expect(teamCalls[1].Slug).To(Equal(""))
		})
	})

	Context("when team has no organizations", func() {
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
		})

		It("should succeed without making any API calls", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
