package reporec

import (
	"context"
	"errors"

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

var _ = Describe("ReconcileCollaborators", func() {
	var (
		ctx        context.Context
		mockClient *ghclientmock.MockGitHubClientWrapper
		k8sClient  client.Client
		rec        *GitHubRepoReconciler
		scheme     *runtime.Scheme
		repo       *v1alpha1.Repository
		org        *v1alpha1.Organization

		desiredCollaborators  []v1alpha1.RepositoryCollaboratorPermission
		existingCollaborators []*github.User
		orgMembers            []*github.User

		addCalls    []ghclientmock.CollaboratorCall
		removeCalls []ghclientmock.CollaboratorCall

		listErr   error
		addErr    error
		removeErr error

		err error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()
		org = nil

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		desiredCollaborators = []v1alpha1.RepositoryCollaboratorPermission{}
		existingCollaborators = []*github.User{}
		orgMembers = []*github.User{}
		addCalls = []ghclientmock.CollaboratorCall{}
		removeCalls = []ghclientmock.CollaboratorCall{}
		listErr = nil
		addErr = nil
		removeErr = nil

		mockClient.GetAllRepositoryCollaboratorsFunc = func(ctx context.Context, owner, repo string) ([]*github.User, error) {
			return existingCollaborators, listErr
		}
		mockClient.ListMembersFunc = func(ctx context.Context, org string) ([]*github.User, error) {
			return orgMembers, nil
		}
		mockClient.AddRepositoryCollaboratorFunc = func(ctx context.Context, owner, repo, username, permission string) error {
			addCalls = append(addCalls, ghclientmock.CollaboratorCall{Method: "AddRepositoryCollaborator", Owner: owner, Repo: repo, Username: username, Permission: permission})
			return addErr
		}
		mockClient.RemoveRepositoryCollaboratorFunc = func(ctx context.Context, owner, repo, username string) error {
			removeCalls = append(removeCalls, ghclientmock.CollaboratorCall{Method: "RemoveRepositoryCollaborator", Owner: owner, Repo: repo, Username: username})
			return removeErr
		}
	})

	JustBeforeEach(func() {
		repo = &v1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-repo",
				Namespace: "default",
			},
			Spec: v1alpha1.RepositorySpec{
				Name:            "test-repo",
				OrganizationRef: v1alpha1.OrganizationRef{Name: "acme-corp"},
				Collaborators:   desiredCollaborators,
			},
		}

		if org == nil {
			org = &v1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "acme-corp",
					Namespace: "default",
				},
				Spec: v1alpha1.OrganizationSpec{
					Name: "acme-corp",
				},
			}
		}

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(repo, org).
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

		err = rec.reconcileCollaborators(ctx)
	})

	Context("when desired collaborators match current collaborators", func() {
		BeforeEach(func() {
			desiredCollaborators = []v1alpha1.RepositoryCollaboratorPermission{{
				Username:   "alice",
				Permission: "push",
			}}
			existingCollaborators = []*github.User{{
				Login:    new("alice"),
				RoleName: new("push"),
			}}
		})

		It("should not call add or remove", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(addCalls).To(BeEmpty())
			Expect(removeCalls).To(BeEmpty())
		})
	})

	Context("when collaborator is missing", func() {
		BeforeEach(func() {
			desiredCollaborators = []v1alpha1.RepositoryCollaboratorPermission{{
				Username:   "alice",
				Permission: "maintain",
			}}
			orgMembers = []*github.User{{Login: new("alice")}}
		})

		It("should add collaborator with requested permission", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(addCalls).To(HaveLen(1))
			Expect(addCalls[0].Username).To(Equal("alice"))
			Expect(addCalls[0].Permission).To(Equal("maintain"))
			Expect(removeCalls).To(BeEmpty())
		})
	})

	Context("when collaborator is missing and not an organization member", func() {
		BeforeEach(func() {
			desiredCollaborators = []v1alpha1.RepositoryCollaboratorPermission{{
				Username:   "alice",
				Permission: "maintain",
			}}
			orgMembers = []*github.User{{Login: new("bob")}}
		})

		It("should not add the collaborator", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(addCalls).To(BeEmpty())
			Expect(removeCalls).To(BeEmpty())
		})
	})

	Context("when permission differs", func() {
		BeforeEach(func() {
			desiredCollaborators = []v1alpha1.RepositoryCollaboratorPermission{{
				Username:   "alice",
				Permission: "admin",
			}}
			existingCollaborators = []*github.User{{
				Login:    new("alice"),
				RoleName: new("push"),
			}}
		})

		It("should update collaborator permission", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(addCalls).To(HaveLen(1))
			Expect(addCalls[0].Username).To(Equal("alice"))
			Expect(addCalls[0].Permission).To(Equal("admin"))
		})
	})

	Context("when permission is omitted", func() {
		BeforeEach(func() {
			desiredCollaborators = []v1alpha1.RepositoryCollaboratorPermission{{
				Username: "alice",
			}}
			orgMembers = []*github.User{{Login: new("alice")}}
		})

		It("should default to pull", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(addCalls).To(HaveLen(1))
			Expect(addCalls[0].Username).To(Equal("alice"))
			Expect(addCalls[0].Permission).To(Equal("pull"))
		})
	})

	Context("when repository has extra collaborators", func() {
		BeforeEach(func() {
			desiredCollaborators = []v1alpha1.RepositoryCollaboratorPermission{{
				Username:   "alice",
				Permission: "push",
			}}
			existingCollaborators = []*github.User{
				{Login: new("alice"), RoleName: new("push")},
				{Login: new("bob"), RoleName: new("pull")},
			}
		})

		It("should remove collaborators not in desired spec", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(addCalls).To(BeEmpty())
			Expect(removeCalls).To(HaveLen(1))
			Expect(removeCalls[0].Username).To(Equal("bob"))
		})
	})

	Context("when collaborators is nil (not specified)", func() {
		BeforeEach(func() {
			desiredCollaborators = nil
			existingCollaborators = []*github.User{
				{Login: new("bob"), RoleName: new("pull")},
			}
		})

		It("should skip reconciliation and leave GitHub untouched", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(addCalls).To(BeEmpty())
			Expect(removeCalls).To(BeEmpty())
		})
	})

	Context("when collaborators is an explicit empty list", func() {
		BeforeEach(func() {
			desiredCollaborators = []v1alpha1.RepositoryCollaboratorPermission{}
			existingCollaborators = []*github.User{
				{Login: new("bob"), RoleName: new("pull")},
			}
		})

		It("should remove all existing collaborators", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(addCalls).To(BeEmpty())
			Expect(removeCalls).To(HaveLen(1))
			Expect(removeCalls[0].Username).To(Equal("bob"))
		})
	})

	Context("when listing collaborators fails", func() {
		BeforeEach(func() {
			listErr = errors.New("list collaborators failed")
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("list collaborators failed"))
		})
	})

	Context("when add collaborator fails", func() {
		BeforeEach(func() {
			addErr = errors.New("add collaborator failed")
			desiredCollaborators = []v1alpha1.RepositoryCollaboratorPermission{{
				Username:   "alice",
				Permission: "pull",
			}}
			orgMembers = []*github.User{{Login: new("alice")}}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("add collaborator failed"))
		})
	})

	Context("when remove collaborator fails", func() {
		BeforeEach(func() {
			removeErr = errors.New("remove collaborator failed")
			existingCollaborators = []*github.User{{
				Login:    new("bob"),
				RoleName: new("pull"),
			}}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("remove collaborator failed"))
		})
	})
})
