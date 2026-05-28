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

var _ = Describe("ReconcileRemovedOrgRefs", func() {
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

	Context("when no organizations have been removed", func() {
		JustBeforeEach(func() {
			team = &v1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-team",
					Namespace: "default",
				},
				Spec: v1alpha1.TeamSpec{
					Name: "test-team",
					OrganizationRefs: []v1alpha1.OrganizationRef{
						{Name: "org1"},
						{Name: "org2"},
					},
				},
				Status: v1alpha1.TeamStatus{
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

		It("should not delete any teams", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(mockClient1.GetTeamCalls()).To(BeEmpty())
			Expect(mockClient2.GetTeamCalls()).To(BeEmpty())
		})

		It("should update previousOrganizationRefs status", func() {
			Expect(err).NotTo(HaveOccurred())

			Expect(team.Status.PreviousOrganizationRefs).To(HaveLen(2))
			Expect(team.Status.PreviousOrganizationRefs).To(ContainElement(v1alpha1.OrganizationRef{Name: "org1"}))
			Expect(team.Status.PreviousOrganizationRefs).To(ContainElement(v1alpha1.OrganizationRef{Name: "org2"}))
		})
	})

	Context("when one organization has been removed", func() {
		JustBeforeEach(func() {
			team = &v1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-team",
					Namespace: "default",
				},
				Spec: v1alpha1.TeamSpec{
					Name: "test-team",
					OrganizationRefs: []v1alpha1.OrganizationRef{
						{Name: "org1"},
					},
				},
				Status: v1alpha1.TeamStatus{
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
					Slug: new("test-team"),
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

		It("should delete the team from the removed organization", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(mockClient2.GetTeamCalls()).To(HaveLen(1))
			Expect(mockClient2.GetTeamCalls()[0].Method).To(Equal("DeleteTeamBySlug"))
			Expect(mockClient2.GetTeamCalls()[0].Org).To(Equal("org2"))
			Expect(mockClient2.GetTeamCalls()[0].Slug).To(Equal("test-team"))
		})

		It("should not delete the team from organizations still referenced", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(mockClient1.GetTeamCalls()).To(BeEmpty())
		})

		It("should update previousOrganizationRefs status with current refs", func() {
			Expect(err).NotTo(HaveOccurred())

			Expect(team.Status.PreviousOrganizationRefs).To(HaveLen(1))
			Expect(team.Status.PreviousOrganizationRefs).To(ContainElement(v1alpha1.OrganizationRef{Name: "org1"}))
		})
	})

	Context("when multiple organizations have been removed", func() {
		JustBeforeEach(func() {
			team = &v1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-team",
					Namespace: "default",
				},
				Spec: v1alpha1.TeamSpec{
					Name: "test-team",
					OrganizationRefs: []v1alpha1.OrganizationRef{
						{Name: "org1"},
					},
				},
				Status: v1alpha1.TeamStatus{
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

		It("should delete the team from all removed organizations", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(mockClient2.GetTeamCalls()).To(HaveLen(1))
			Expect(mockClient2.GetTeamCalls()[0].Method).To(Equal("DeleteTeamBySlug"))
			Expect(mockClient3.GetTeamCalls()).To(HaveLen(1))
			Expect(mockClient3.GetTeamCalls()[0].Method).To(Equal("DeleteTeamBySlug"))
		})

		It("should not delete the team from the remaining organization", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(mockClient1.GetTeamCalls()).To(BeEmpty())
		})

		It("should update previousOrganizationRefs status correctly", func() {
			Expect(err).NotTo(HaveOccurred())

			Expect(team.Status.PreviousOrganizationRefs).To(HaveLen(1))
			Expect(team.Status.PreviousOrganizationRefs).To(ContainElement(v1alpha1.OrganizationRef{Name: "org1"}))
		})
	})

	Context("when team deletion fails", func() {
		JustBeforeEach(func() {
			team = &v1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-team",
					Namespace: "default",
				},
				Spec: v1alpha1.TeamSpec{
					Name: "test-team",
					OrganizationRefs: []v1alpha1.OrganizationRef{
						{Name: "org1"},
					},
				},
				Status: v1alpha1.TeamStatus{
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
				return errors.New("deletion failed")
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
			Expect(err.Error()).To(ContainSubstring("deletion failed"))
		})

		It("should have attempted to delete the team", func() {
			Expect(mockClient2.GetTeamCalls()).To(HaveLen(1))
			Expect(mockClient2.GetTeamCalls()[0].Method).To(Equal("DeleteTeamBySlug"))
		})
	})

	Context("when team deletion returns 404 (already deleted)", func() {
		JustBeforeEach(func() {
			team = &v1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-team",
					Namespace: "default",
				},
				Spec: v1alpha1.TeamSpec{
					Name: "test-team",
					OrganizationRefs: []v1alpha1.OrganizationRef{
						{Name: "org1"},
					},
				},
				Status: v1alpha1.TeamStatus{
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
				return &github.ErrorResponse{
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

		It("should return the 404 error (not idempotent by design)", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Not Found"))
		})
	})

	Context("when previousOrganizationRefs is empty (first reconciliation)", func() {
		JustBeforeEach(func() {
			team = &v1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-team",
					Namespace: "default",
				},
				Spec: v1alpha1.TeamSpec{
					Name: "test-team",
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

		It("should not delete any teams", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(mockClient1.GetTeamCalls()).To(BeEmpty())
			Expect(mockClient2.GetTeamCalls()).To(BeEmpty())
		})

		It("should set previousOrganizationRefs to current refs", func() {
			Expect(err).NotTo(HaveOccurred())

			Expect(team.Status.PreviousOrganizationRefs).To(HaveLen(2))
			Expect(team.Status.PreviousOrganizationRefs).To(ContainElement(v1alpha1.OrganizationRef{Name: "org1"}))
			Expect(team.Status.PreviousOrganizationRefs).To(ContainElement(v1alpha1.OrganizationRef{Name: "org2"}))
		})
	})
})
