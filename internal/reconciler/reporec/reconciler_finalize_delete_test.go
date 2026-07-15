package reporec

import (
	"context"
	"errors"
	"net/http"
	"net/url"

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

var _ = Describe("ReconcileDeletion", func() {
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
			FinalizeMode: reconciler.Delete,
		}

		err = rec.ReconcileDeletion(ctx)
	})

	Context("when repository exists", func() {
		It("should delete the repository successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deleteRepoCalled).To(BeTrue(), "DeleteRepository should be called")
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

		It("should treat the repository as already deleted and should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(deleteRepoCalled).To(BeTrue(), "DeleteRepository should be called")
		})
	})

	Context("when DeleteRepository fails", func() {
		BeforeEach(func() {
			mockClient.DeleteRepositoryFunc = func(ctx context.Context, owner, name string) error {
				deleteRepoCalled = true
				return errors.New("GitHub API error: failed to delete repository")
			}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to delete repository"))
		})

		It("should have attempted to call DeleteRepository", func() {
			Expect(deleteRepoCalled).To(BeTrue(), "DeleteRepository should be called before error")
		})
	})

	Context("when DeleteRepository returns a rate limit error", func() {
		BeforeEach(func() {
			mockClient.DeleteRepositoryFunc = func(ctx context.Context, owner, name string) error {
				deleteRepoCalled = true
				url, _ := url.ParseRequestURI("https://api.github.com/rate_limit")
				return &github.RateLimitError{
					Message: "API rate limit exceeded",
					Response: &http.Response{
						StatusCode: 403,
						Request: &http.Request{
							Method: "GET",
							URL:    url,
						},
					},
				}
			}
		})

		It("should return the rate limit error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API rate limit exceeded"))
		})
	})

	Context("when owner and name are different from resource spec", func() {
		BeforeEach(func() {
			mockClient.DeleteRepositoryFunc = func(ctx context.Context, owner, name string) error {
				deleteRepoCalled = true
				Expect(owner).To(Equal("test-org"), "Owner should match GitHubRepoIdentifier in delete")
				Expect(name).To(Equal("test-repo"), "Name should match GitHubRepoIdentifier in delete")
				return nil
			}
		})

		It("should use the correct owner and name from GitHubRepoIdentifier", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deleteRepoCalled).To(BeTrue())
		})
	})
})
