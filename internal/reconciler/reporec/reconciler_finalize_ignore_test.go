package reporec

import (
	"context"
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

var _ = Describe("ReconcileIgnore", func() {
	var (
		ctx                context.Context
		mockClient         *ghclientmock.MockGitHubClientWrapper
		k8sClient          client.Client
		rec                *GitHubRepoReconciler
		scheme             *runtime.Scheme
		repo               *v1alpha1.Repository
		err                error
		currentGHRepo      *github.Repository
		deleteRepoCalled   bool
		editRepoCalled     bool
		getRepositoryCalls int
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		repo = &v1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-repo",
				Namespace: "default",
			},
			Spec: v1alpha1.RepositorySpec{
				Name:     "test-repo",
				Archived: new(false),
				OrganizationRef: v1alpha1.OrganizationRef{
					Name: "test-org",
				},
			},
		}

		deleteRepoCalled = false
		editRepoCalled = false
		getRepositoryCalls = 0

		// Default: repository is not archived
		currentGHRepo = &github.Repository{
			Name:       new("test-repo"),
			Visibility: new("internal"),
			Archived:   new(false),
			ID:         new(int64(12345)),
		}

		// Set default mock functions (can be overridden in nested BeforeEach)
		mockClient.GetRepositoryFunc = func(ctx context.Context, owner, name string) (*github.Repository, error) {
			getRepositoryCalls++
			return currentGHRepo, nil
		}

		mockClient.EditRepositoryFunc = func(ctx context.Context, owner, name string, repository *github.Repository) (*github.Repository, error) {
			editRepoCalled = true
			// Return the edited repo with the same ID but archived status updated
			result := *currentGHRepo
			result.Archived = repository.Archived
			return &result, nil
		}

		mockClient.DeleteRepositoryFunc = func(ctx context.Context, owner, name string) error {
			deleteRepoCalled = true
			return nil
		}
	})

	JustBeforeEach(func() {
		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(repo).
			WithStatusSubresource(repo).
			Build()

		rec = &GitHubRepoReconciler{
			GitHub: reconciler.GitHub[GitHubRepoIdentifier]{
				Client: mockClient,
				Resource: GitHubRepoIdentifier{
					Owner: "test-org",
					Name:  "test-repo",
					ID:    new(int64(12345)),
				},
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Repository]{
				Client:   k8sClient,
				Resource: repo,
			},
			FinalizeMode: reconciler.Ignore,
		}

		err = rec.ReconcileDeletion(ctx)
	})

	Context("when repository exists", func() {
		It("should not delete/archive the repository successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deleteRepoCalled).To(BeFalse(), "DeleteRepository should be not called")
			Expect(editRepoCalled).To(BeFalse(), "EditRepository should be not called")
		})

		It("should not call GetRepository", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(getRepositoryCalls).To(Equal(0), "GetRepository should not be called")
		})
	})

	Context("when repository is already deleted (404)", func() {
		BeforeEach(func() {
			mockClient.DeleteRepositoryFunc = func(ctx context.Context, owner, name string) error {
				deleteRepoCalled = true
				return &github.ErrorResponse{
					Message: "Not Found",
					Response: &http.Response{
						StatusCode: 404,
					},
				}
			}
		})

		It("should not delete/archive the repository successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deleteRepoCalled).To(BeFalse(), "DeleteRepository should not be called")
			Expect(editRepoCalled).To(BeFalse(), "EditRepository should not be called")
		})
	})

	Context("when FinalizeMode is not set", func() {
		BeforeEach(func() {
			rec = &GitHubRepoReconciler{
				GitHub: reconciler.GitHub[GitHubRepoIdentifier]{
					Client: mockClient,
					Resource: GitHubRepoIdentifier{
						Owner: "test-org",
						Name:  "test-repo",
						ID:    new(int64(12345)),
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Repository]{
					Client:   k8sClient,
					Resource: repo,
				},
				FinalizeMode: "",
			}
		})

		It("should not delete/archive the repository successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deleteRepoCalled).To(BeFalse(), "DeleteRepository should be not called")
			Expect(editRepoCalled).To(BeFalse(), "EditRepository should be not called")
		})

		It("should not call GetRepository", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(getRepositoryCalls).To(Equal(0), "GetRepository should not be called")
		})
	})
})
