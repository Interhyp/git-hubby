package reporec

import (
	"context"
	"errors"
	"net/http"
	"time"

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

var _ = Describe("ReconcileRepository", func() {
	var (
		ctx              context.Context
		mockClient       *ghclientmock.MockGitHubClientWrapper
		k8sClient        client.Client
		rec              *GitHubRepoReconciler
		scheme           *runtime.Scheme
		repo             *v1alpha1.Repository
		err              error
		currentGHRepo    *github.Repository
		editedRepo       *github.Repository
		createdRepo      *github.Repository
		editRepoCalled   bool
		createRepoCalled bool
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		repo = &v1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-repo",
				Namespace:   "default",
				Annotations: map[string]string{},
			},
			Spec: v1alpha1.RepositorySpec{
				Name:                "test-repo",
				Archived:            github.Ptr(false),
				Visibility:          "internal",
				HasIssues:           github.Ptr(true),
				HasProjects:         github.Ptr(false),
				HasWiki:             github.Ptr(false),
				HasDownloads:        github.Ptr(false),
				IsTemplate:          github.Ptr(false),
				DeleteBranchOnMerge: github.Ptr(true),
				MergeCommitMessage:  "PR_TITLE",
				MergeCommitTitle:    "MERGE_MESSAGE",
				OrganizationRef: v1alpha1.OrganizationRef{
					Name: "test-org",
				},
			},
		}

		editedRepo = nil
		createdRepo = nil
		editRepoCalled = false
		createRepoCalled = false

		// Default: current GitHub repo matches desired state
		currentGHRepo = &github.Repository{
			Name:                github.Ptr("test-repo"),
			Visibility:          github.Ptr("internal"),
			Archived:            github.Ptr(false),
			HasIssues:           repo.Spec.HasIssues,
			HasProjects:         github.Ptr(false),
			HasWiki:             github.Ptr(false),
			HasDownloads:        github.Ptr(false),
			IsTemplate:          github.Ptr(false),
			AutoInit:            github.Ptr(true),
			AllowSquashMerge:    github.Ptr(false),
			AllowRebaseMerge:    github.Ptr(false),
			AllowMergeCommit:    github.Ptr(false),
			DeleteBranchOnMerge: repo.Spec.DeleteBranchOnMerge,
			MergeCommitTitle:    github.Ptr("MERGE_MESSAGE"),
			MergeCommitMessage:  github.Ptr("PR_TITLE"),
			Homepage:            github.Ptr(""),
			Description:         github.Ptr(""),
			ID:                  github.Ptr(int64(12345)),
			DefaultBranch:       github.Ptr(repo.Spec.DefaultBranch),
		}

		// Set default mock functions (can be overridden in nested BeforeEach)
		mockClient.GetRepositoryFunc = func(ctx context.Context, owner, name string) (*github.Repository, error) {
			return currentGHRepo, nil
		}

		mockClient.EditRepositoryFunc = func(ctx context.Context, owner, name string, repository *github.Repository) (*github.Repository, error) {
			editRepoCalled = true
			editedRepo = repository
			// Return the edited repo with the same ID
			result := *repository
			if result.ID == nil {
				result.ID = github.Ptr(int64(12345))
			}
			return &result, nil
		}

		mockClient.CreateRepositoryFunc = func(ctx context.Context, org string, repository *github.Repository) (*github.Repository, error) {
			createRepoCalled = true
			createdRepo = repository
			// Return the created repo with an ID
			result := *repository
			result.ID = github.Ptr(int64(12345))
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
				},
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Repository]{
				Client:   k8sClient,
				Resource: repo,
			},
		}

		err = rec.reconcileRepository(ctx)
	})

	Context("when repository is up to date", func() {
		It("should skip update and return no error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editRepoCalled).To(BeFalse())
			Expect(createRepoCalled).To(BeFalse())
			Expect(editedRepo).To(BeNil())
			Expect(createdRepo).To(BeNil())
		})

		It("should update the repository ID in status", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.Status.ID).NotTo(BeNil())
			Expect(*repo.Status.ID).To(Equal(int64(12345)))
		})
	})

	Context("when repository name differs", func() {
		BeforeEach(func() {
			currentGHRepo = &github.Repository{
				Name:       github.Ptr("old-name"),
				Visibility: github.Ptr("internal"),
				Archived:   github.Ptr(false),
				ID:         github.Ptr(int64(12345)),
			}
		})

		It("should update the repository", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editRepoCalled).To(BeTrue())
			Expect(editedRepo).NotTo(BeNil())
			Expect(editedRepo.GetName()).To(Equal("test-repo"))
		})
	})

	Context("when repository archived status differs", func() {
		BeforeEach(func() {
			currentGHRepo = &github.Repository{
				Name:       github.Ptr("test-repo"),
				Visibility: github.Ptr("internal"),
				Archived:   github.Ptr(true),
				ID:         github.Ptr(int64(12345)),
			}
		})

		It("should update the repository", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editRepoCalled).To(BeTrue())
			Expect(editedRepo).NotTo(BeNil())
			Expect(editedRepo.GetArchived()).To(BeFalse())
		})
	})

	Context("when repository visibility differs", func() {
		BeforeEach(func() {
			currentGHRepo = &github.Repository{
				Name:       github.Ptr("test-repo"),
				Visibility: github.Ptr("public"),
				Archived:   github.Ptr(false),
				ID:         github.Ptr(int64(12345)),
			}
		})

		It("should update the repository", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editRepoCalled).To(BeTrue())
			Expect(editedRepo).NotTo(BeNil())
			Expect(editedRepo.GetVisibility()).To(Equal("internal"))
		})
	})

	Context("when multiple fields differ", func() {
		BeforeEach(func() {
			currentGHRepo = &github.Repository{
				Name:       github.Ptr("old-name"),
				Visibility: github.Ptr("public"),
				Archived:   github.Ptr(true),
				ID:         github.Ptr(int64(12345)),
			}
		})

		It("should update all fields", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editRepoCalled).To(BeTrue())
			Expect(editedRepo).NotTo(BeNil())
			Expect(editedRepo.GetName()).To(Equal("test-repo"))
			Expect(editedRepo.GetVisibility()).To(Equal("internal"))
			Expect(editedRepo.GetArchived()).To(BeFalse())
		})
	})

	Context("when repository does not exist on GitHub", func() {
		BeforeEach(func() {
			mockClient.GetRepositoryFunc = func(ctx context.Context, owner, name string) (*github.Repository, error) {
				return nil, &github.ErrorResponse{
					Message: "Not Found",
					Response: &http.Response{
						StatusCode: http.StatusNotFound,
					},
				}
			}
		})

		It("should create the repository", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRepoCalled).To(BeTrue())
			Expect(createdRepo).NotTo(BeNil())
			Expect(createdRepo.GetName()).To(Equal("test-repo"))
			Expect(createdRepo.GetVisibility()).To(Equal("internal"))
			Expect(createdRepo.GetArchived()).To(BeFalse())
		})

		It("should not call edit repository", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editRepoCalled).To(BeFalse())
		})

		It("should update the repository ID in status after creation", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.Status.ID).NotTo(BeNil())
			Expect(*repo.Status.ID).To(Equal(int64(12345)))
		})

		Context("and parent organization exists", func() {
			var org *v1alpha1.Organization

			BeforeEach(func() {
				org = &v1alpha1.Organization{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test-org",
						Namespace:   "default",
						Annotations: map[string]string{},
					},
					Spec: v1alpha1.OrganizationSpec{
						Name:                    "test-org",
						GitHubAppInstallationId: 123,
					},
				}
			})

			JustBeforeEach(func() {
				// Create org before running the reconciliation
				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(repo, org).
					WithStatusSubresource(repo, org).
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

				err = rec.reconcileRepository(ctx)
			})

			It("should trigger parent organization reconciliation", func() {
				Expect(err).NotTo(HaveOccurred())

				// Fetch the updated organization
				var updatedOrg v1alpha1.Organization
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-org", Namespace: "default"}, &updatedOrg)
				Expect(err).NotTo(HaveOccurred())

				// Verify ReconcileTrigger was set
				Expect(updatedOrg.Annotations).NotTo(BeEmpty())
			})

			It("should set a valid timestamp in reconcile-trigger annotation", func() {
				Expect(err).NotTo(HaveOccurred())

				var updatedOrg v1alpha1.Organization
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-org", Namespace: "default"}, &updatedOrg)
				Expect(err).NotTo(HaveOccurred())

				// Verify the timestamp is parseable
				_, parseErr := time.Parse(time.RFC3339Nano, updatedOrg.Annotations["git-hubby.interhyp.de/reconcile-trigger"])
				Expect(parseErr).NotTo(HaveOccurred())
			})
		})

		Context("and parent organization does not exist", func() {
			It("should still succeed repository creation without triggering org reconciliation", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(createRepoCalled).To(BeTrue())
			})
		})
	})

	Context("when GetRepository fails with non-404 error", func() {
		BeforeEach(func() {
			mockClient.GetRepositoryFunc = func(ctx context.Context, owner, name string) (*github.Repository, error) {
				return nil, &github.ErrorResponse{
					Message: "Internal Server Error",
					Response: &http.Response{
						StatusCode: http.StatusInternalServerError,
					},
				}
			}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Internal Server Error"))
		})

		It("should not create or edit the repository", func() {
			Expect(createRepoCalled).To(BeFalse())
			Expect(editRepoCalled).To(BeFalse())
		})
	})

	Context("when CreateRepository fails", func() {
		BeforeEach(func() {
			mockClient.GetRepositoryFunc = func(ctx context.Context, owner, name string) (*github.Repository, error) {
				return nil, &github.ErrorResponse{
					Message: "Not Found",
					Response: &http.Response{
						StatusCode: http.StatusNotFound,
					},
				}
			}
			mockClient.CreateRepositoryFunc = func(ctx context.Context, org string, repository *github.Repository) (*github.Repository, error) {
				return nil, errors.New("failed to create repository")
			}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to create repository"))
		})
	})

	Context("when EditRepository fails", func() {
		BeforeEach(func() {
			currentGHRepo = &github.Repository{
				Name:       github.Ptr("old-name"),
				Visibility: github.Ptr("internal"),
				Archived:   github.Ptr(false),
				ID:         github.Ptr(int64(12345)),
			}
			mockClient.EditRepositoryFunc = func(ctx context.Context, owner, name string, repository *github.Repository) (*github.Repository, error) {
				return nil, errors.New("failed to edit repository")
			}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to edit repository"))
		})
	})

	Context("when GitHub returns repository with nil ID", func() {
		BeforeEach(func() {
			currentGHRepo = &github.Repository{
				Name:       github.Ptr("test-repo"),
				Visibility: github.Ptr("internal"),
				Archived:   github.Ptr(false),
				ID:         nil,
			}
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unable to update repository ID"))
		})
	})

	Context("when GitHub returns nil repository", func() {
		BeforeEach(func() {
			mockClient.GetRepositoryFunc = func(ctx context.Context, owner, name string) (*github.Repository, error) {
				return nil, nil
			}
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unable to update repository ID"))
		})
	})

	Context("when repository spec has archived=true", func() {
		BeforeEach(func() {
			repo.Spec.Archived = github.Ptr(true)
			currentGHRepo = &github.Repository{
				Name:       github.Ptr("test-repo"),
				Visibility: github.Ptr("internal"),
				Archived:   github.Ptr(false),
				ID:         github.Ptr(int64(12345)),
			}
		})

		It("should update the repository to archived", func() {
			// After archiving, the reconciler should return a RepoArchivedError
			Expect(err).To(HaveOccurred())
			var archivedErr *RepoArchivedError
			Expect(errors.As(err, &archivedErr)).To(BeTrue())
			Expect(editRepoCalled).To(BeTrue())
			Expect(editedRepo).NotTo(BeNil())
			Expect(editedRepo.GetArchived()).To(BeTrue())
		})

		It("should include repository details in error", func() {
			Expect(err).To(HaveOccurred())
			var archivedErr *RepoArchivedError
			Expect(errors.As(err, &archivedErr)).To(BeTrue())
			Expect(archivedErr.RepositoryName).To(Equal("test-repo"))
			Expect(archivedErr.RepositoryOwner).To(Equal("test-org"))
			Expect(err.Error()).To(ContainSubstring("test-org/test-repo"))
			Expect(err.Error()).To(ContainSubstring("archived"))
			Expect(err.Error()).To(ContainSubstring("read-only"))
		})
	})

	Context("when repository is already archived on GitHub", func() {
		BeforeEach(func() {
			repo.Spec.Archived = github.Ptr(true)
			currentGHRepo = &github.Repository{
				Name:                github.Ptr("test-repo"),
				Visibility:          github.Ptr("internal"),
				Archived:            github.Ptr(true),
				ID:                  github.Ptr(int64(12345)),
				HasIssues:           repo.Spec.HasIssues,
				HasProjects:         github.Ptr(false),
				HasWiki:             github.Ptr(false),
				HasDownloads:        github.Ptr(false),
				IsTemplate:          github.Ptr(false),
				AutoInit:            github.Ptr(true),
				AllowSquashMerge:    github.Ptr(false),
				AllowRebaseMerge:    github.Ptr(false),
				AllowMergeCommit:    github.Ptr(false),
				DeleteBranchOnMerge: repo.Spec.DeleteBranchOnMerge,
				MergeCommitTitle:    github.Ptr("MERGE_MESSAGE"),
				MergeCommitMessage:  github.Ptr("PR_TITLE"),
				Homepage:            github.Ptr(""),
				Description:         github.Ptr(""),
				DefaultBranch:       github.Ptr(repo.Spec.DefaultBranch),
			}
		})

		It("should return RepoArchivedError without attempting update", func() {
			Expect(err).To(HaveOccurred())
			var archivedErr *RepoArchivedError
			Expect(errors.As(err, &archivedErr)).To(BeTrue())
			// Should not call EditRepository since repo is already up-to-date
			Expect(editRepoCalled).To(BeFalse())
		})

		It("should still update repository ID in status", func() {
			// Even though we error, ID should be updated before checking archived status
			Expect(repo.Status.ID).NotTo(BeNil())
			Expect(*repo.Status.ID).To(Equal(int64(12345)))
		})
	})

	Context("when repository tries to unarchive an archived repo", func() {
		BeforeEach(func() {
			// Spec wants to unarchive (archived=false), but repo IS archived on GitHub
			repo.Spec.Archived = github.Ptr(false)
			repo.Spec.About = v1alpha1.About{
				Description: "New description",
			}
			currentGHRepo = &github.Repository{
				Name:        github.Ptr("test-repo"),
				Visibility:  github.Ptr("internal"),
				Archived:    github.Ptr(true),
				Description: github.Ptr("Old description"),
				ID:          github.Ptr(int64(12345)),
			}
		})

		It("should attempt update and succeed without error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editRepoCalled).To(BeTrue())
		})
	})

	Context("when repository is not archived", func() {
		BeforeEach(func() {
			repo.Spec.Archived = github.Ptr(false)
			currentGHRepo = &github.Repository{
				Name:       github.Ptr("test-repo"),
				Visibility: github.Ptr("internal"),
				Archived:   github.Ptr(false),
				ID:         github.Ptr(int64(12345)),
			}
		})

		It("should not return RepoArchivedError", func() {
			Expect(err).NotTo(HaveOccurred())
			var archivedErr *RepoArchivedError
			Expect(errors.As(err, &archivedErr)).To(BeFalse())
		})
	})

	Context("when repository has nil bool fields (testing defaults)", func() {
		BeforeEach(func() {
			// Set all bool fields to nil to test default behavior
			repo.Spec.HasIssues = nil
			repo.Spec.HasProjects = nil
			repo.Spec.HasWiki = nil
			repo.Spec.HasDownloads = nil
			repo.Spec.IsTemplate = nil
			repo.Spec.DeleteBranchOnMerge = nil
			repo.Spec.Archived = nil

			// GitHub repo matches the expected defaults
			currentGHRepo = &github.Repository{
				Name:                github.Ptr("test-repo"),
				Visibility:          github.Ptr("internal"),
				Archived:            github.Ptr(false), // default
				HasIssues:           github.Ptr(true),  // default
				HasProjects:         github.Ptr(false), // default
				HasWiki:             github.Ptr(false), // default
				HasDownloads:        github.Ptr(false), // default
				IsTemplate:          github.Ptr(false), // default
				AutoInit:            github.Ptr(true),
				AllowSquashMerge:    github.Ptr(false),
				AllowRebaseMerge:    github.Ptr(false),
				AllowMergeCommit:    github.Ptr(false),
				DeleteBranchOnMerge: github.Ptr(true), // default
				MergeCommitTitle:    github.Ptr("MERGE_MESSAGE"),
				MergeCommitMessage:  github.Ptr("PR_TITLE"),
				Homepage:            github.Ptr(""),
				Description:         github.Ptr(""),
				ID:                  github.Ptr(int64(12345)),
				DefaultBranch:       github.Ptr(repo.Spec.DefaultBranch),
			}
		})

		It("should not update when GitHub matches defaults", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editRepoCalled).To(BeFalse())
		})

		It("should update repository ID in status", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.Status.ID).NotTo(BeNil())
			Expect(*repo.Status.ID).To(Equal(int64(12345)))
		})
	})

	Context("when repository has nil bool fields and GitHub differs from defaults", func() {
		BeforeEach(func() {
			// Set all bool fields to nil to test default behavior
			repo.Spec.HasIssues = nil           // default would be true
			repo.Spec.DeleteBranchOnMerge = nil // default would be true
			repo.Spec.Archived = nil            // default would be false

			// GitHub repo has different values than defaults
			currentGHRepo = &github.Repository{
				Name:                github.Ptr("test-repo"),
				Visibility:          github.Ptr("internal"),
				Archived:            github.Ptr(false),
				HasIssues:           github.Ptr(false), // differs from default (true)
				HasProjects:         github.Ptr(false),
				HasWiki:             github.Ptr(false),
				HasDownloads:        github.Ptr(false),
				IsTemplate:          github.Ptr(false),
				AutoInit:            github.Ptr(true),
				AllowSquashMerge:    github.Ptr(false),
				AllowRebaseMerge:    github.Ptr(false),
				AllowMergeCommit:    github.Ptr(false),
				DeleteBranchOnMerge: github.Ptr(false), // differs from default (true)
				MergeCommitTitle:    github.Ptr("MERGE_MESSAGE"),
				MergeCommitMessage:  github.Ptr("PR_TITLE"),
				Homepage:            github.Ptr(""),
				Description:         github.Ptr(""),
				ID:                  github.Ptr(int64(12345)),
				DefaultBranch:       github.Ptr(repo.Spec.DefaultBranch),
			}
		})

		It("should update repository when GitHub differs from defaults", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editRepoCalled).To(BeTrue())
			Expect(editedRepo).NotTo(BeNil())
			// Note: RepoToGithubRepo passes nil values through, so the update request won't include these fields
			// This is intentional - GitHub API interprets missing fields as "don't change"
		})
	})

	Context("when creating repository with nil bool fields", func() {
		BeforeEach(func() {
			// Set all bool fields to nil
			repo.Spec.HasIssues = nil
			repo.Spec.HasProjects = nil
			repo.Spec.HasWiki = nil
			repo.Spec.HasDownloads = nil
			repo.Spec.IsTemplate = nil
			repo.Spec.DeleteBranchOnMerge = nil
			repo.Spec.Archived = nil

			mockClient.GetRepositoryFunc = func(ctx context.Context, owner, name string) (*github.Repository, error) {
				return nil, &github.ErrorResponse{
					Message: "Not Found",
					Response: &http.Response{
						StatusCode: http.StatusNotFound,
					},
				}
			}
		})

		It("should create repository with nil bool fields (GitHub applies its own defaults)", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRepoCalled).To(BeTrue())
			Expect(createdRepo).NotTo(BeNil())
			// The mapper passes nil values through - GitHub will apply its own defaults during creation
			Expect(createdRepo.HasIssues).To(BeNil())
			Expect(createdRepo.HasProjects).To(BeNil())
			Expect(createdRepo.HasWiki).To(BeNil())
			Expect(createdRepo.HasDownloads).To(BeNil())
			Expect(createdRepo.IsTemplate).To(BeNil())
			Expect(createdRepo.DeleteBranchOnMerge).To(BeNil())
			Expect(createdRepo.Archived).To(BeNil())
		})
	})
})

var _ = Describe("getRepo", func() {
	var (
		ctx           context.Context
		mockClient    *ghclientmock.MockGitHubClientWrapper
		k8sClient     client.Client
		rec           *GitHubRepoReconciler
		scheme        *runtime.Scheme
		repo          *v1alpha1.Repository
		ghRepo        *github.Repository
		err           error
		getRepoCalled bool
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
				Archived: github.Ptr(false),
				OrganizationRef: v1alpha1.OrganizationRef{
					Name: "test-org",
				},
			},
		}

		getRepoCalled = false

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
				},
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Repository]{
				Client:   k8sClient,
				Resource: repo,
			},
		}
	})

	Context("when repository exists on GitHub", func() {
		BeforeEach(func() {
			mockClient.GetRepositoryFunc = func(ctx context.Context, owner, name string) (*github.Repository, error) {
				getRepoCalled = true
				return &github.Repository{
					Name:       github.Ptr("test-repo"),
					Visibility: github.Ptr("internal"),
					Archived:   github.Ptr(false),
					ID:         github.Ptr(int64(12345)),
				}, nil
			}

			ghRepo, err = rec.getRepo(ctx)
		})

		It("should return the repository", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(ghRepo).NotTo(BeNil())
			Expect(ghRepo.GetName()).To(Equal("test-repo"))
			Expect(ghRepo.GetID()).To(Equal(int64(12345)))
		})

		It("should update the repository ID in status", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(rec.GitHub.Resource.ID).NotTo(BeNil())
			Expect(*rec.GitHub.Resource.ID).To(Equal(int64(12345)))
			Expect(rec.Kubernetes.Resource.Status.ID).NotTo(BeNil())
			Expect(*rec.Kubernetes.Resource.Status.ID).To(Equal(int64(12345)))
		})

		It("should have called GetRepository", func() {
			Expect(getRepoCalled).To(BeTrue())
		})
	})

	Context("when repository does not exist on GitHub", func() {
		BeforeEach(func() {
			mockClient.GetRepositoryFunc = func(ctx context.Context, owner, name string) (*github.Repository, error) {
				return nil, &github.ErrorResponse{
					Message: "Not Found",
					Response: &http.Response{
						StatusCode: http.StatusNotFound,
					},
				}
			}
			mockClient.CreateRepositoryFunc = func(ctx context.Context, org string, repository *github.Repository) (*github.Repository, error) {
				return &github.Repository{
					Name:       repository.Name,
					Visibility: repository.Visibility,
					Archived:   repository.Archived,
					ID:         github.Ptr(int64(67890)),
				}, nil
			}

			ghRepo, err = rec.getRepo(ctx)
		})

		It("should create and return the repository", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(ghRepo).NotTo(BeNil())
			Expect(ghRepo.GetID()).To(Equal(int64(67890)))
		})

		It("should update the repository ID in status", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(rec.GitHub.Resource.ID).NotTo(BeNil())
			Expect(*rec.GitHub.Resource.ID).To(Equal(int64(67890)))
		})
	})

	Context("when GetRepository fails with non-404 error", func() {
		BeforeEach(func() {
			mockClient.GetRepositoryFunc = func(ctx context.Context, owner, name string) (*github.Repository, error) {
				return nil, errors.New("API error")
			}

			ghRepo, err = rec.getRepo(ctx)
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API error"))
		})

		It("should not return a repository", func() {
			Expect(ghRepo).To(BeNil())
		})
	})

	Context("when CreateRepository fails", func() {
		BeforeEach(func() {
			mockClient.GetRepositoryFunc = func(ctx context.Context, owner, name string) (*github.Repository, error) {
				return nil, &github.ErrorResponse{
					Message: "Not Found",
					Response: &http.Response{
						StatusCode: http.StatusNotFound,
					},
				}
			}
			mockClient.CreateRepositoryFunc = func(ctx context.Context, org string, repository *github.Repository) (*github.Repository, error) {
				return nil, errors.New("creation failed")
			}

			ghRepo, err = rec.getRepo(ctx)
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("creation failed"))
		})

		It("should not return a repository", func() {
			Expect(ghRepo).To(BeNil())
		})
	})
})

var _ = Describe("updateRepo", func() {
	var (
		ctx        context.Context
		mockClient *ghclientmock.MockGitHubClientWrapper
		k8sClient  client.Client
		rec        *GitHubRepoReconciler
		scheme     *runtime.Scheme
		repo       *v1alpha1.Repository
		err        error
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
				Archived: github.Ptr(false),
				OrganizationRef: v1alpha1.OrganizationRef{
					Name: "test-org",
				},
			},
		}

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
				},
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Repository]{
				Client:   k8sClient,
				Resource: repo,
			},
		}
	})

	Context("when update succeeds", func() {
		BeforeEach(func() {
			mockClient.EditRepositoryFunc = func(ctx context.Context, owner, name string, repository *github.Repository) (*github.Repository, error) {
				result := *repository
				result.ID = github.Ptr(int64(99999))
				return &result, nil
			}

			_, err = rec.updateRepo(ctx)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should update the repository ID", func() {
			Expect(rec.GitHub.Resource.ID).NotTo(BeNil())
			Expect(*rec.GitHub.Resource.ID).To(Equal(int64(99999)))
		})
	})

	Context("when EditRepository fails", func() {
		BeforeEach(func() {
			mockClient.EditRepositoryFunc = func(ctx context.Context, owner, name string, repository *github.Repository) (*github.Repository, error) {
				return nil, errors.New("edit failed")
			}

			_, err = rec.updateRepo(ctx)
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("edit failed"))
		})
	})
})

var _ = Describe("updateID", func() {
	var (
		ctx        context.Context
		mockClient *ghclientmock.MockGitHubClientWrapper
		k8sClient  client.Client
		rec        *GitHubRepoReconciler
		scheme     *runtime.Scheme
		repo       *v1alpha1.Repository
		ghRepo     *github.Repository
		err        error
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
				Archived: github.Ptr(false),
				OrganizationRef: v1alpha1.OrganizationRef{
					Name: "test-org",
				},
			},
		}

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
				},
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Repository]{
				Client:   k8sClient,
				Resource: repo,
			},
		}
	})

	Context("when updating with valid repository", func() {
		BeforeEach(func() {
			ghRepo = &github.Repository{
				Name: github.Ptr("test-repo"),
				ID:   github.Ptr(int64(54321)),
			}

			err = rec.updateID(ctx, ghRepo)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should update the ID in GitHub resource", func() {
			Expect(rec.GitHub.Resource.ID).NotTo(BeNil())
			Expect(*rec.GitHub.Resource.ID).To(Equal(int64(54321)))
		})

		It("should update the ID in Kubernetes status", func() {
			Expect(rec.Kubernetes.Resource.Status.ID).NotTo(BeNil())
			Expect(*rec.Kubernetes.Resource.Status.ID).To(Equal(int64(54321)))
		})

		It("should persist the status update", func() {
			Expect(repo.Status.ID).NotTo(BeNil())
			Expect(*repo.Status.ID).To(Equal(int64(54321)))
		})
	})

	Context("when repository is nil", func() {
		BeforeEach(func() {
			err = rec.updateID(ctx, nil)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unable to update repository ID"))
			Expect(err.Error()).To(ContainSubstring("nil repository"))
		})
	})

	Context("when repository ID is nil", func() {
		BeforeEach(func() {
			ghRepo = &github.Repository{
				Name: github.Ptr("test-repo"),
				ID:   nil,
			}

			err = rec.updateID(ctx, ghRepo)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unable to update repository ID"))
			Expect(err.Error()).To(ContainSubstring("nil ID"))
		})
	})
})
