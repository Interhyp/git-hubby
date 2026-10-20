package reporec

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	"github.com/google/go-github/v93/github"
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
		createdRepo      *github.CreateRepositoryRequest
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
				Archived:            new(false),
				Visibility:          "internal",
				HasIssues:           new(true),
				HasProjects:         new(false),
				HasWiki:             new(false),
				HasDownloads:        new(false),
				IsTemplate:          new(false),
				DeleteBranchOnMerge: new(true),
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
			Name:                new("test-repo"),
			Visibility:          new("internal"),
			Archived:            new(false),
			HasIssues:           repo.Spec.HasIssues,
			HasProjects:         new(false),
			HasWiki:             new(false),
			HasDownloads:        new(false),
			IsTemplate:          new(false),
			AutoInit:            new(true),
			AllowSquashMerge:    new(false),
			AllowRebaseMerge:    new(false),
			AllowMergeCommit:    new(false),
			DeleteBranchOnMerge: repo.Spec.DeleteBranchOnMerge,
			MergeCommitTitle:    new("MERGE_MESSAGE"),
			MergeCommitMessage:  new("PR_TITLE"),
			Homepage:            new(""),
			Description:         new(""),
			ID:                  new(int64(12345)),
			DefaultBranch:       new(repo.Spec.DefaultBranch),
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
				result.ID = new(int64(12345))
			}
			return &result, nil
		}

		mockClient.CreateRepositoryFunc = func(ctx context.Context, org string, request github.CreateRepositoryRequest) (*github.Repository, error) {
			createRepoCalled = true
			createdRepo = &request
			// Return the created repo with an ID; fields that are not part of the create request
			// are simulated to already match the spec
			result := ghclientmock.RepositoryFromCreateRequest(org, request)
			result.HasDownloads = repo.Spec.HasDownloads
			result.DefaultBranch = new(repo.Spec.DefaultBranch)
			return result, nil
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
				Name:       new("old-name"),
				Visibility: new("internal"),
				Archived:   new(false),
				ID:         new(int64(12345)),
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
				Name:       new("test-repo"),
				Visibility: new("internal"),
				Archived:   new(true),
				ID:         new(int64(12345)),
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
				Name:       new("test-repo"),
				Visibility: new("public"),
				Archived:   new(false),
				ID:         new(int64(12345)),
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
				Name:       new("old-name"),
				Visibility: new("public"),
				Archived:   new(true),
				ID:         new(int64(12345)),
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
			Expect(createdRepo.Name).To(Equal("test-repo"))
			Expect(createdRepo.GetVisibility()).To(Equal("internal"))
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
						GitHubAppInstallationId: new(int64(123)),
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
			mockClient.CreateRepositoryFunc = func(ctx context.Context, org string, repository github.CreateRepositoryRequest) (*github.Repository, error) {
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
				Name:       new("old-name"),
				Visibility: new("internal"),
				Archived:   new(false),
				ID:         new(int64(12345)),
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

	Context("when EditRepository fails with 422", func() {
		BeforeEach(func() {
			repo.Spec.Archived = new(true)
			mockClient.EditRepositoryFunc = func(ctx context.Context, owner, name string, repository *github.Repository) (*github.Repository, error) {
				return nil, &github.ErrorResponse{
					Message:  "Validation Failed",
					Response: &http.Response{StatusCode: http.StatusUnprocessableEntity},
				}
			}
		})

		Context("and the repository was created recently", func() {
			BeforeEach(func() {
				currentGHRepo.CreatedAt = &github.Timestamp{Time: time.Now().Add(-10 * time.Second)}
			})

			It("should return a retryable error", func() {
				var newRepoErr *NewRepositoryUpdateError
				Expect(errors.As(err, &newRepoErr)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("Validation Failed"))
				Expect(reconciler.RequiresUserAction(err)).To(BeFalse())
			})
		})

		Context("and the repository is older than the grace period", func() {
			BeforeEach(func() {
				currentGHRepo.CreatedAt = &github.Timestamp{Time: time.Now().Add(-newRepositoryGracePeriod - time.Minute)}
			})

			It("should require user action", func() {
				var newRepoErr *NewRepositoryUpdateError
				Expect(errors.As(err, &newRepoErr)).To(BeFalse())
				Expect(reconciler.RequiresUserAction(err)).To(BeTrue())
			})
		})
	})

	Context("when GitHub returns repository with nil ID", func() {
		BeforeEach(func() {
			currentGHRepo = &github.Repository{
				Name:       new("test-repo"),
				Visibility: new("internal"),
				Archived:   new(false),
				ID:         nil,
			}
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unable to update reconciler fields with nil data"))
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
			Expect(err.Error()).To(ContainSubstring("unable to update reconciler fields with nil data"))
		})
	})

	Context("when repository spec has archived=true", func() {
		BeforeEach(func() {
			repo.Spec.Archived = new(true)
			currentGHRepo = &github.Repository{
				Name:       new("test-repo"),
				Visibility: new("internal"),
				Archived:   new(false),
				ID:         new(int64(12345)),
			}
		})

		It("should update the repository to archived", func() {
			// After archiving, the reconciler stops the reconciliation on purpose
			Expect(err).To(HaveOccurred())
			var stop *reconciler.StopReconciliation
			Expect(errors.As(err, &stop)).To(BeTrue())
			Expect(editRepoCalled).To(BeTrue())
			Expect(editedRepo).NotTo(BeNil())
			Expect(editedRepo.GetArchived()).To(BeTrue())
		})

		It("should explain the stop in the message", func() {
			var stop *reconciler.StopReconciliation
			Expect(errors.As(err, &stop)).To(BeTrue())
			Expect(stop.Message).To(Equal(ArchivedMessage))
			Expect(stop.Message).To(ContainSubstring("archived"))
			Expect(stop.Message).To(ContainSubstring("read-only"))
		})
	})

	Context("when repository is already archived on GitHub", func() {
		BeforeEach(func() {
			repo.Spec.Archived = new(true)
			currentGHRepo = &github.Repository{
				Name:                new("test-repo"),
				Visibility:          new("internal"),
				Archived:            new(true),
				ID:                  new(int64(12345)),
				HasIssues:           repo.Spec.HasIssues,
				HasProjects:         new(false),
				HasWiki:             new(false),
				HasDownloads:        new(false),
				IsTemplate:          new(false),
				AutoInit:            new(true),
				AllowSquashMerge:    new(false),
				AllowRebaseMerge:    new(false),
				AllowMergeCommit:    new(false),
				DeleteBranchOnMerge: repo.Spec.DeleteBranchOnMerge,
				MergeCommitTitle:    new("MERGE_MESSAGE"),
				MergeCommitMessage:  new("PR_TITLE"),
				Homepage:            new(""),
				Description:         new(""),
				DefaultBranch:       new(repo.Spec.DefaultBranch),
			}
		})

		It("should stop the reconciliation without attempting update", func() {
			Expect(err).To(HaveOccurred())
			var stop *reconciler.StopReconciliation
			Expect(errors.As(err, &stop)).To(BeTrue())
			Expect(stop.Message).To(Equal(ArchivedMessage))
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
			repo.Spec.Archived = new(false)
			repo.Spec.About = v1alpha1.About{
				Description: "New description",
			}
			currentGHRepo = &github.Repository{
				Name:        new("test-repo"),
				Visibility:  new("internal"),
				Archived:    new(true),
				Description: new("Old description"),
				ID:          new(int64(12345)),
			}
		})

		It("should attempt update and succeed without error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editRepoCalled).To(BeTrue())
		})
	})

	Context("when repository is not archived", func() {
		BeforeEach(func() {
			repo.Spec.Archived = new(false)
			currentGHRepo = &github.Repository{
				Name:       new("test-repo"),
				Visibility: new("internal"),
				Archived:   new(false),
				ID:         new(int64(12345)),
			}
		})

		It("should not stop the reconciliation", func() {
			Expect(err).NotTo(HaveOccurred())
			var stop *reconciler.StopReconciliation
			Expect(errors.As(err, &stop)).To(BeFalse())
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
				Name:                new("test-repo"),
				Visibility:          new("internal"),
				Archived:            new(false), // default
				HasIssues:           new(true),  // default
				HasProjects:         new(false), // default
				HasWiki:             new(false), // default
				HasDownloads:        new(false), // default
				IsTemplate:          new(false), // default
				AutoInit:            new(true),
				AllowSquashMerge:    new(false),
				AllowRebaseMerge:    new(false),
				AllowMergeCommit:    new(false),
				DeleteBranchOnMerge: new(true), // default
				MergeCommitTitle:    new("MERGE_MESSAGE"),
				MergeCommitMessage:  new("PR_TITLE"),
				Homepage:            new(""),
				Description:         new(""),
				ID:                  new(int64(12345)),
				DefaultBranch:       new(repo.Spec.DefaultBranch),
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
				Name:                new("test-repo"),
				Visibility:          new("internal"),
				Archived:            new(false),
				HasIssues:           new(false), // differs from default (true)
				HasProjects:         new(false),
				HasWiki:             new(false),
				HasDownloads:        new(false),
				IsTemplate:          new(false),
				AutoInit:            new(true),
				AllowSquashMerge:    new(false),
				AllowRebaseMerge:    new(false),
				AllowMergeCommit:    new(false),
				DeleteBranchOnMerge: new(false), // differs from default (true)
				MergeCommitTitle:    new("MERGE_MESSAGE"),
				MergeCommitMessage:  new("PR_TITLE"),
				Homepage:            new(""),
				Description:         new(""),
				ID:                  new(int64(12345)),
				DefaultBranch:       new(repo.Spec.DefaultBranch),
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
			Expect(createdRepo.IsTemplate).To(BeNil())
			Expect(createdRepo.DeleteBranchOnMerge).To(BeNil())
		})
	})

	Context("when the desired default branch does not exist", func() {
		var getBranchCalled bool

		BeforeEach(func() {
			getBranchCalled = false
			repo.Spec.DefaultBranch = "develop"
			currentGHRepo.DefaultBranch = new("main")
			mockClient.GetBranchFunc = func(ctx context.Context, owner, name, branch string) (*github.Branch, error) {
				getBranchCalled = true
				Expect(branch).To(Equal("develop"))
				return nil, &github.ErrorResponse{
					Message:  "Branch not found",
					Response: &http.Response{StatusCode: http.StatusNotFound},
				}
			}
		})

		It("should return a DefaultBranchNotFoundError without updating the repository", func() {
			Expect(getBranchCalled).To(BeTrue())
			Expect(editRepoCalled).To(BeFalse())
			var dbErr *DefaultBranchNotFoundError
			Expect(errors.As(err, &dbErr)).To(BeTrue())
			Expect(dbErr.Branch).To(Equal("develop"))
			Expect(reconciler.RequiresUserAction(errors.Join(err))).To(BeTrue())
		})
	})

	Context("when the desired default branch exists but is not yet the default", func() {
		BeforeEach(func() {
			repo.Spec.DefaultBranch = "develop"
			currentGHRepo.DefaultBranch = new("main")
		})

		It("should update the repository", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editRepoCalled).To(BeTrue())
			Expect(editedRepo.GetDefaultBranch()).To(Equal("develop"))
		})
	})

	Context("when checking the default branch fails with a non-404 error", func() {
		BeforeEach(func() {
			repo.Spec.DefaultBranch = "develop"
			currentGHRepo.DefaultBranch = new("main")
			mockClient.GetBranchFunc = func(ctx context.Context, owner, name, branch string) (*github.Branch, error) {
				return nil, errors.New("API error")
			}
		})

		It("should return the error as a regular (retryable) error", func() {
			Expect(err).To(HaveOccurred())
			Expect(editRepoCalled).To(BeFalse())
			Expect(reconciler.RequiresUserAction(err)).To(BeFalse())
		})
	})

	Context("when the default branch already matches", func() {
		var getBranchCalled bool

		BeforeEach(func() {
			getBranchCalled = false
			repo.Spec.DefaultBranch = "main"
			currentGHRepo.DefaultBranch = new("main")
			currentGHRepo.HasWiki = new(true) // force an update
			mockClient.GetBranchFunc = func(ctx context.Context, owner, name, branch string) (*github.Branch, error) {
				getBranchCalled = true
				return &github.Branch{Name: new(branch)}, nil
			}
		})

		It("should not check the branch and update the repository", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(getBranchCalled).To(BeFalse())
			Expect(editRepoCalled).To(BeTrue())
		})
	})

	Context("when the repository was renamed via spec.name", func() {
		// Simulates the factory setting rec.GitHub.Resource.Name from repo.Spec.Name (the new desired name),
		// while the repository on GitHub still exists under its old name and Status.ID is already populated.
		BeforeEach(func() {
			repo.Spec.Name = "new-repo-name"
			repo.Status.ID = new(int64(12345))

			currentGHRepo = &github.Repository{
				Name:       new("old-repo-name"),
				Visibility: new("internal"),
				Archived:   new(false),
				ID:         new(int64(12345)),
			}

			mockClient.GetRepositoryByIDFunc = func(ctx context.Context, id int64) (*github.Repository, error) {
				return currentGHRepo, nil
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
						Name:  repo.Spec.Name, // mirrors what the factory does
					},
				},
				Kubernetes: reconciler.Kubernetes[*v1alpha1.Repository]{
					Client:   k8sClient,
					Resource: repo,
				},
			}

			err = rec.reconcileRepository(ctx)
		})

		It("should not create a duplicate repository", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRepoCalled).To(BeFalse())
		})

		It("should call EditRepository using the current (old) name and the new name in the body", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editRepoCalled).To(BeTrue())
			Expect(editedRepo).NotTo(BeNil())
			Expect(editedRepo.GetName()).To(Equal("new-repo-name"))
		})

		It("should end up with the new name on the reconciler's GitHub resource", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(rec.GitHub.Resource.Name).To(Equal("new-repo-name"))
		})

		It("should keep the same repository ID in status", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.Status.ID).NotTo(BeNil())
			Expect(*repo.Status.ID).To(Equal(int64(12345)))
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
				Archived: new(false),
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
					Name:       new("test-repo"),
					Visibility: new("internal"),
					Archived:   new(false),
					ID:         new(int64(12345)),
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
			mockClient.CreateRepositoryFunc = func(ctx context.Context, org string, repository github.CreateRepositoryRequest) (*github.Repository, error) {
				return &github.Repository{
					Name:       new(repository.Name),
					Visibility: repository.Visibility,
					Archived:   new(false),
					ID:         new(int64(67890)),
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
			mockClient.CreateRepositoryFunc = func(ctx context.Context, org string, repository github.CreateRepositoryRequest) (*github.Repository, error) {
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

	Context("when a repository ID is stored in status", func() {
		var (
			getByIDCalled   bool
			getByNameCalled bool
		)

		BeforeEach(func() {
			repo.Status.ID = new(int64(12345))
			getByIDCalled = false
			getByNameCalled = false

			mockClient.GetRepositoryByIDFunc = func(ctx context.Context, id int64) (*github.Repository, error) {
				getByIDCalled = true
				Expect(id).To(Equal(int64(12345)))
				return &github.Repository{
					Name:       new("test-repo"),
					Visibility: new("internal"),
					Archived:   new(false),
					ID:         new(int64(12345)),
				}, nil
			}
			mockClient.GetRepositoryFunc = func(ctx context.Context, owner, name string) (*github.Repository, error) {
				getByNameCalled = true
				return &github.Repository{
					Name:       new("test-repo"),
					Visibility: new("internal"),
					Archived:   new(false),
					ID:         new(int64(12345)),
				}, nil
			}

			ghRepo, err = rec.getRepo(ctx)
		})

		It("should look up the repository by ID and skip the name-based lookup", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(getByIDCalled).To(BeTrue())
			Expect(getByNameCalled).To(BeFalse())
			Expect(ghRepo).NotTo(BeNil())
			Expect(ghRepo.GetID()).To(Equal(int64(12345)))
		})
	})

	Context("when the stored repository ID lookup does not find a repository", func() {
		BeforeEach(func() {
			repo.Status.ID = new(int64(99999))

			mockClient.GetRepositoryByIDFunc = func(ctx context.Context, id int64) (*github.Repository, error) {
				return nil, nil
			}
			mockClient.GetRepositoryFunc = func(ctx context.Context, owner, name string) (*github.Repository, error) {
				getRepoCalled = true
				return &github.Repository{
					Name:       new("test-repo"),
					Visibility: new("internal"),
					Archived:   new(false),
					ID:         new(int64(12345)),
				}, nil
			}

			ghRepo, err = rec.getRepo(ctx)
		})

		It("should fall back to the name-based lookup", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(getRepoCalled).To(BeTrue())
			Expect(ghRepo).NotTo(BeNil())
			Expect(ghRepo.GetID()).To(Equal(int64(12345)))
		})
	})

	Context("when the repository was renamed on GitHub", func() {
		BeforeEach(func() {
			// rec.GitHub.Resource.Name still holds the desired (new) name from spec.Name,
			// but the repository was renamed on GitHub, so the ID-based lookup must be used
			// to find it and the actual (old) current name must be adopted.
			repo.Status.ID = new(int64(12345))

			mockClient.GetRepositoryByIDFunc = func(ctx context.Context, id int64) (*github.Repository, error) {
				return &github.Repository{
					Name:       new("old-name"),
					Visibility: new("internal"),
					Archived:   new(false),
					ID:         new(int64(12345)),
				}, nil
			}

			ghRepo, err = rec.getRepo(ctx)
		})

		It("should return the repository under its current (old) name", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(ghRepo).NotTo(BeNil())
			Expect(ghRepo.GetName()).To(Equal("old-name"))
		})

		It("should update the GitHub resource identifier to the current name", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(rec.GitHub.Resource.Name).To(Equal("old-name"))
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
				Archived: new(false),
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
				result.ID = new(int64(99999))
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

var _ = Describe("updateRecFieldsFromGitHub", func() {
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
				Archived: new(false),
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
				Name: new("test-repo"),
				ID:   new(int64(54321)),
			}

			err = rec.updateRecFieldsFromGitHub(ctx, ghRepo)
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

	Context("when repository has a different name than currently stored", func() {
		BeforeEach(func() {
			ghRepo = &github.Repository{
				Name: new("renamed-repo"),
				ID:   new(int64(54321)),
			}

			err = rec.updateRecFieldsFromGitHub(ctx, ghRepo)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should update the Name in GitHub resource", func() {
			Expect(rec.GitHub.Resource.Name).To(Equal("renamed-repo"))
		})
	})

	Context("when repository is nil", func() {
		BeforeEach(func() {
			err = rec.updateRecFieldsFromGitHub(ctx, nil)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unable to update reconciler fields with nil data"))
		})
	})

	Context("when repository ID is nil", func() {
		BeforeEach(func() {
			ghRepo = &github.Repository{
				Name: new("test-repo"),
				ID:   nil,
			}

			err = rec.updateRecFieldsFromGitHub(ctx, ghRepo)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unable to update reconciler fields with nil data"))
		})
	})

	Context("when repository Name is nil", func() {
		BeforeEach(func() {
			ghRepo = &github.Repository{
				Name: nil,
				ID:   new(int64(54321)),
			}

			err = rec.updateRecFieldsFromGitHub(ctx, ghRepo)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unable to update reconciler fields with nil data"))
		})
	})
})
