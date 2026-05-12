package teamrec

import (
	"context"
	"errors"
	"os"

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

var _ = Describe("ReconcileTeamMembers", func() {
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
				Name:    "test-team",
				Members: []string{"user1", "user2"},
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

		// Unset GITHUB_MEMBER_SUFFIX for tests
		os.Unsetenv("GITHUB_MEMBER_SUFFIX")
	})

	Context("when team is an IDP team", func() {
		BeforeEach(func() {
			idpGroup := "test-idp-group"
			team.Spec.IDPGroup = &idpGroup
			team.Spec.Members = nil

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

			err = rec.reconcileTeamMembers(ctx)
		})

		It("should skip member reconciliation for IDP teams", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(mockClient1.GetTeamMemberCalls()).To(BeEmpty())
		})
	})

	Context("when team has no existing members", func() {
		BeforeEach(func() {
			mockClient1.GetAllTeamMembersFunc = func(ctx context.Context, org string, slug string) ([]*github.User, error) {
				return []*github.User{}, nil
			}

			mockClient1.ListMembersFunc = func(ctx context.Context, org string) ([]*github.User, error) {
				return []*github.User{
					{Login: github.Ptr("user1")},
					{Login: github.Ptr("user2")},
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

			err = rec.reconcileTeamMembers(ctx)
		})

		It("should add all specified members", func() {
			Expect(err).NotTo(HaveOccurred())
			teamMemberCalls := mockClient1.GetTeamMemberCalls()
			Expect(teamMemberCalls).To(HaveLen(2))

			addCalls := 0
			for _, call := range teamMemberCalls {
				if call.Method == "AddTeamMember" {
					addCalls++
				}
			}
			Expect(addCalls).To(Equal(2))
		})
	})

	Context("when team has existing members that match spec", func() {
		BeforeEach(func() {
			mockClient1.GetAllTeamMembersFunc = func(ctx context.Context, org string, slug string) ([]*github.User, error) {
				return []*github.User{
					{Login: github.Ptr("user1")},
					{Login: github.Ptr("user2")},
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

			err = rec.reconcileTeamMembers(ctx)
		})

		It("should not modify members", func() {
			Expect(err).NotTo(HaveOccurred())
			teamMemberCalls := mockClient1.GetTeamMemberCalls()

			// No add or remove operations should be performed
			for _, call := range teamMemberCalls {
				Expect(call.Method).NotTo(Equal("AddTeamMember"))
				Expect(call.Method).NotTo(Equal("RemoveTeamMember"))
			}
		})
	})

	Context("when team has extra members not in spec", func() {
		BeforeEach(func() {
			mockClient1.GetAllTeamMembersFunc = func(ctx context.Context, org string, slug string) ([]*github.User, error) {
				return []*github.User{
					{Login: github.Ptr("user1")},
					{Login: github.Ptr("user2")},
					{Login: github.Ptr("user3")},
					{Login: github.Ptr("user4")},
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

			err = rec.reconcileTeamMembers(ctx)
		})

		It("should remove extra members", func() {
			Expect(err).NotTo(HaveOccurred())
			teamMemberCalls := mockClient1.GetTeamMemberCalls()

			removeCalls := 0
			removedUsers := []string{}
			for _, call := range teamMemberCalls {
				if call.Method == "RemoveTeamMember" {
					removeCalls++
					removedUsers = append(removedUsers, call.Username)
				}
			}
			Expect(removeCalls).To(Equal(2))
			Expect(removedUsers).To(ConsistOf("user3", "user4"))
		})
	})

	Context("when team is missing some members from spec", func() {
		BeforeEach(func() {
			mockClient1.GetAllTeamMembersFunc = func(ctx context.Context, org string, slug string) ([]*github.User, error) {
				return []*github.User{
					{Login: github.Ptr("user1")},
				}, nil
			}

			mockClient1.ListMembersFunc = func(ctx context.Context, org string) ([]*github.User, error) {
				return []*github.User{
					{Login: github.Ptr("user1")},
					{Login: github.Ptr("user2")},
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

			err = rec.reconcileTeamMembers(ctx)
		})

		It("should add missing members", func() {
			Expect(err).NotTo(HaveOccurred())
			teamMemberCalls := mockClient1.GetTeamMemberCalls()

			addCalls := 0
			for _, call := range teamMemberCalls {
				if call.Method == "AddTeamMember" && call.Username == "user2" {
					addCalls++
				}
			}
			Expect(addCalls).To(Equal(1))
		})
	})

	Context("when member is not found in GitHub organization", func() {
		BeforeEach(func() {
			mockClient1.GetAllTeamMembersFunc = func(ctx context.Context, org string, slug string) ([]*github.User, error) {
				return []*github.User{}, nil
			}

			mockClient1.ListMembersFunc = func(ctx context.Context, org string) ([]*github.User, error) {
				// Only user1 exists in the org
				return []*github.User{
					{Login: github.Ptr("user1")},
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

			err = rec.reconcileTeamMembers(ctx)
		})

		It("should skip adding non-existent members and succeed", func() {
			Expect(err).NotTo(HaveOccurred())
			teamMemberCalls := mockClient1.GetTeamMemberCalls()

			// Only user1 should be added
			addCalls := 0
			for _, call := range teamMemberCalls {
				if call.Method == "AddTeamMember" {
					addCalls++
					Expect(call.Username).To(Equal("user1"))
				}
			}
			Expect(addCalls).To(Equal(1))
		})
	})

	Context("when GetAllTeamMembers fails", func() {
		BeforeEach(func() {
			mockClient1.GetAllTeamMembersFunc = func(ctx context.Context, org string, slug string) ([]*github.User, error) {
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

			err = rec.reconcileTeamMembers(ctx)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get existing team members"))
		})
	})

	Context("when ListMembers fails", func() {
		BeforeEach(func() {
			mockClient1.GetAllTeamMembersFunc = func(ctx context.Context, org string, slug string) ([]*github.User, error) {
				return []*github.User{}, nil
			}

			mockClient1.ListMembersFunc = func(ctx context.Context, org string) ([]*github.User, error) {
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

			err = rec.reconcileTeamMembers(ctx)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API error"))
		})
	})

	Context("when AddTeamMember fails", func() {
		BeforeEach(func() {
			mockClient1.GetAllTeamMembersFunc = func(ctx context.Context, org string, slug string) ([]*github.User, error) {
				return []*github.User{}, nil
			}

			mockClient1.ListMembersFunc = func(ctx context.Context, org string) ([]*github.User, error) {
				return []*github.User{
					{Login: github.Ptr("user1")},
				}, nil
			}

			mockClient1.AddMemberToTeamFunc = func(ctx context.Context, org string, slug string, username string) error {
				return errors.New("failed to add member")
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

			err = rec.reconcileTeamMembers(ctx)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to add member"))
		})
	})

	Context("when RemoveTeamMember fails", func() {
		BeforeEach(func() {
			mockClient1.GetAllTeamMembersFunc = func(ctx context.Context, org string, slug string) ([]*github.User, error) {
				return []*github.User{
					{Login: github.Ptr("user1")},
					{Login: github.Ptr("user3")},
				}, nil
			}

			mockClient1.RemoveMemberFromTeamFunc = func(ctx context.Context, org string, slug string, username string) error {
				return errors.New("failed to remove member")
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

			err = rec.reconcileTeamMembers(ctx)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to remove member"))
		})
	})

	Context("when GITHUB_MEMBER_SUFFIX is set", func() {
		BeforeEach(func() {
			os.Setenv("GITHUB_MEMBER_SUFFIX", "@example.com")

			mockClient1.GetAllTeamMembersFunc = func(ctx context.Context, org string, slug string) ([]*github.User, error) {
				return []*github.User{}, nil
			}

			mockClient1.ListMembersFunc = func(ctx context.Context, org string) ([]*github.User, error) {
				return []*github.User{
					{Login: github.Ptr("user1@example.com")},
					{Login: github.Ptr("user2@example.com")},
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

			err = rec.reconcileTeamMembers(ctx)
		})

		AfterEach(func() {
			os.Unsetenv("GITHUB_MEMBER_SUFFIX")
		})

		It("should append suffix to member names", func() {
			Expect(err).NotTo(HaveOccurred())
			teamMemberCalls := mockClient1.GetTeamMemberCalls()

			for _, call := range teamMemberCalls {
				if call.Method == "AddTeamMember" {
					Expect(call.Username).To(HaveSuffix("@example.com"))
				}
			}
		})
	})

	Context("when reconciling team in multiple organizations", func() {
		BeforeEach(func() {
			team.Spec.OrganizationRefs = []v1alpha1.OrganizationRef{
				{Name: "org1"},
				{Name: "org2"},
			}

			mockClient1.GetAllTeamMembersFunc = func(ctx context.Context, org string, slug string) ([]*github.User, error) {
				return []*github.User{
					{Login: github.Ptr("user1")},
				}, nil
			}

			mockClient1.ListMembersFunc = func(ctx context.Context, org string) ([]*github.User, error) {
				return []*github.User{
					{Login: github.Ptr("user1")},
					{Login: github.Ptr("user2")},
				}, nil
			}

			mockClient2.GetAllTeamMembersFunc = func(ctx context.Context, org string, slug string) ([]*github.User, error) {
				return []*github.User{
					{Login: github.Ptr("user1")},
					{Login: github.Ptr("user2")},
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

			err = rec.reconcileTeamMembers(ctx)
		})

		It("should reconcile members in both organizations", func() {
			Expect(err).NotTo(HaveOccurred())

			// org1 should have added user2
			org1Calls := mockClient1.GetTeamMemberCalls()
			org1AddCalls := 0
			for _, call := range org1Calls {
				if call.Method == "AddTeamMember" {
					org1AddCalls++
				}
			}
			Expect(org1AddCalls).To(Equal(1))

			// org2 should have no changes
			org2Calls := mockClient2.GetTeamMemberCalls()
			org2AddCalls := 0
			org2RemoveCalls := 0
			for _, call := range org2Calls {
				if call.Method == "AddTeamMember" {
					org2AddCalls++
				}
				if call.Method == "RemoveTeamMember" {
					org2RemoveCalls++
				}
			}
			Expect(org2AddCalls).To(Equal(0))
			Expect(org2RemoveCalls).To(Equal(0))
		})
	})

	Context("when team has no members specified", func() {
		BeforeEach(func() {
			team.Spec.Members = []string{}

			mockClient1.GetAllTeamMembersFunc = func(ctx context.Context, org string, slug string) ([]*github.User, error) {
				return []*github.User{
					{Login: github.Ptr("user1")},
					{Login: github.Ptr("user2")},
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

			err = rec.reconcileTeamMembers(ctx)
		})

		It("should remove all existing members", func() {
			Expect(err).NotTo(HaveOccurred())
			teamMemberCalls := mockClient1.GetTeamMemberCalls()

			removeCalls := 0
			for _, call := range teamMemberCalls {
				if call.Method == "RemoveTeamMember" {
					removeCalls++
				}
			}
			Expect(removeCalls).To(Equal(2))
		})
	})

	Context("when existing members have nil login", func() {
		BeforeEach(func() {
			mockClient1.GetAllTeamMembersFunc = func(ctx context.Context, org string, slug string) ([]*github.User, error) {
				return []*github.User{
					{Login: nil},
					{Login: github.Ptr("user1")},
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

			err = rec.reconcileTeamMembers(ctx)
		})

		It("should handle nil logins gracefully", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when organization has no members at all", func() {
		BeforeEach(func() {
			mockClient1.GetAllTeamMembersFunc = func(ctx context.Context, org string, slug string) ([]*github.User, error) {
				return []*github.User{}, nil
			}

			mockClient1.ListMembersFunc = func(ctx context.Context, org string) ([]*github.User, error) {
				return []*github.User{}, nil
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

			err = rec.reconcileTeamMembers(ctx)
		})

		It("should succeed without adding any members", func() {
			Expect(err).NotTo(HaveOccurred())
			teamMemberCalls := mockClient1.GetTeamMemberCalls()

			addCalls := 0
			for _, call := range teamMemberCalls {
				if call.Method == "AddTeamMember" {
					addCalls++
				}
			}
			Expect(addCalls).To(Equal(0))
		})
	})
})
