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

var _ = Describe("ReconcileArchive", func() {
	var (
		ctx                context.Context
		mockClient         *ghclientmock.MockGitHubClientWrapper
		k8sClient          client.Client
		rec                *GitHubRepoReconciler
		scheme             *runtime.Scheme
		repo               *v1alpha1.Repository
		err                error
		currentGHRepo      *github.Repository
		editedRepo         *github.Repository
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

		editedRepo = nil
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
			editedRepo = repository
			// Return the edited repo with the same ID but archived status updated
			result := *currentGHRepo
			result.Archived = repository.Archived
			return &result, nil
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
			FinalizeMode: reconciler.Archive,
		}

		err = rec.ReconcileDeletion(ctx)
	})

	Context("when repository is not archived", func() {
		It("should archive the repository successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editRepoCalled).To(BeTrue(), "EditRepository should be called")
			Expect(editedRepo).NotTo(BeNil(), "Edited repo should not be nil")
			Expect(editedRepo.GetArchived()).To(BeTrue(), "Repository should be marked as archived")
		})

		It("should call GetRepository once", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(getRepositoryCalls).To(Equal(1), "GetRepository should be called exactly once")
		})

		It("should only set the Archived field in the edit request", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editedRepo).NotTo(BeNil())
			Expect(editedRepo.Archived).NotTo(BeNil())
			Expect(editedRepo.GetArchived()).To(BeTrue())
			// Verify only Archived is set (other fields should be nil)
			Expect(editedRepo.Name).To(BeNil(), "Name should not be set in edit request")
			Expect(editedRepo.Visibility).To(BeNil(), "Visibility should not be set in edit request")
		})
	})

	Context("when repository is already archived", func() {
		BeforeEach(func() {
			currentGHRepo = &github.Repository{
				Name:       new("test-repo"),
				Visibility: new("internal"),
				Archived:   new(true),
				ID:         new(int64(12345)),
			}
		})

		It("should skip archiving and return no error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editRepoCalled).To(BeFalse(), "EditRepository should not be called")
		})

		It("should still call GetRepository to check status", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(getRepositoryCalls).To(Equal(1), "GetRepository should be called to check status")
		})
	})

	Context("when GetRepository fails", func() {
		BeforeEach(func() {
			mockClient.GetRepositoryFunc = func(ctx context.Context, owner, name string) (*github.Repository, error) {
				getRepositoryCalls++
				return nil, errors.New("GitHub API error: failed to get repository")
			}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GitHub API error"))
		})

		It("should not attempt to archive the repository", func() {
			Expect(editRepoCalled).To(BeFalse(), "EditRepository should not be called when GetRepository fails")
		})
	})

	Context("when GetRepository returns a 404 error", func() {
		BeforeEach(func() {
			mockClient.GetRepositoryFunc = func(ctx context.Context, owner, name string) (*github.Repository, error) {
				getRepositoryCalls++
				return nil, &github.ErrorResponse{
					Message: "Not Found",
					Response: &http.Response{
						StatusCode: 404,
					},
				}
			}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Not Found"))
		})

		It("should not attempt to archive the repository", func() {
			Expect(editRepoCalled).To(BeFalse(), "EditRepository should not be called when repository doesn't exist")
		})
	})

	Context("when EditRepository fails", func() {
		BeforeEach(func() {
			mockClient.EditRepositoryFunc = func(ctx context.Context, owner, name string, repository *github.Repository) (*github.Repository, error) {
				editRepoCalled = true
				editedRepo = repository
				return nil, errors.New("GitHub API error: failed to archive repository")
			}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to archive repository"))
		})

		It("should have attempted to call EditRepository", func() {
			Expect(editRepoCalled).To(BeTrue(), "EditRepository should be called before error")
		})

		It("should have passed the correct archive value", func() {
			Expect(editedRepo).NotTo(BeNil())
			Expect(editedRepo.GetArchived()).To(BeTrue())
		})
	})

	Context("when EditRepository returns a rate limit error", func() {
		BeforeEach(func() {
			mockClient.EditRepositoryFunc = func(ctx context.Context, owner, name string, repository *github.Repository) (*github.Repository, error) {
				editRepoCalled = true
				editedRepo = repository
				url, _ := url.ParseRequestURI("https://api.github.com/rate_limit")
				return nil, &github.RateLimitError{
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

	Context("when repository is partially archived", func() {
		BeforeEach(func() {
			// Edge case: Archived pointer exists but is nil (shouldn't happen, but let's be defensive)
			currentGHRepo = &github.Repository{
				Name:       new("test-repo"),
				Visibility: new("internal"),
				Archived:   nil, // nil pointer
				ID:         new(int64(12345)),
			}
		})

		It("should treat nil Archived as false and archive the repository", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editRepoCalled).To(BeTrue(), "EditRepository should be called when Archived is nil")
			Expect(editedRepo).NotTo(BeNil())
			Expect(editedRepo.GetArchived()).To(BeTrue())
		})
	})

	Context("when owner and name are different from resource spec", func() {
		BeforeEach(func() {
			// Test that the reconciler uses the correct owner/name from GitHubRepoIdentifier
			mockClient.GetRepositoryFunc = func(ctx context.Context, owner, name string) (*github.Repository, error) {
				getRepositoryCalls++
				Expect(owner).To(Equal("test-org"), "Owner should match GitHubRepoIdentifier")
				Expect(name).To(Equal("test-repo"), "Name should match GitHubRepoIdentifier")
				return currentGHRepo, nil
			}

			mockClient.EditRepositoryFunc = func(ctx context.Context, owner, name string, repository *github.Repository) (*github.Repository, error) {
				editRepoCalled = true
				editedRepo = repository
				Expect(owner).To(Equal("test-org"), "Owner should match GitHubRepoIdentifier in edit")
				Expect(name).To(Equal("test-repo"), "Name should match GitHubRepoIdentifier in edit")
				result := *currentGHRepo
				result.Archived = repository.Archived
				return &result, nil
			}
		})

		It("should use the correct owner and name from GitHubRepoIdentifier", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editRepoCalled).To(BeTrue())
		})
	})
})
