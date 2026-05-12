package reporec

import (
	"context"
	"errors"

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

var _ = Describe("ReconcileActionsSettings", func() {
	var (
		ctx                             context.Context
		mockClient                      *ghclientmock.MockGitHubClientWrapper
		k8sClient                       client.Client
		rec                             *GitHubRepoReconciler
		scheme                          *runtime.Scheme
		repo                            *v1alpha1.Repository
		org                             *v1alpha1.Organization
		err                             error
		accessLevelForExternalWorkflows *string
		orgActionsEnabled               *string
		currentAccessLevel              *github.RepositoryActionsAccessLevel
		getAccessLevelError             error
		setAccessLevelError             error
		setAccessLevelCalled            bool
		setAccessLevelWithValue         *string
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		// Default values
		accessLevelForExternalWorkflows = github.Ptr("none")
		orgActionsEnabled = github.Ptr("all")
		currentAccessLevel = &github.RepositoryActionsAccessLevel{
			AccessLevel: github.Ptr("none"),
		}

		// Reset flags and errors
		getAccessLevelError = nil
		setAccessLevelError = nil
		setAccessLevelCalled = false
		setAccessLevelWithValue = nil

		// Set up default mock functions
		mockClient.GetAccessLevelForExternalWorkflowsForRepoFunc = func(ctx context.Context, owner string, repo string) (*github.RepositoryActionsAccessLevel, error) {
			return currentAccessLevel, getAccessLevelError
		}

		mockClient.SetAccessLevelForExternalWorkflowsForRepoFunc = func(ctx context.Context, owner string, repo string, accessLevel github.RepositoryActionsAccessLevel) error {
			setAccessLevelCalled = true
			setAccessLevelWithValue = accessLevel.AccessLevel
			return setAccessLevelError
		}
	})

	JustBeforeEach(func() {
		// Create organization CR
		org = &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-org",
				Namespace: "default",
			},
			Spec: v1alpha1.OrganizationSpec{
				Name: "test-org",
				ActionsSettings: v1alpha1.ActionsSettings{
					EnabledRepositories: orgActionsEnabled,
				},
			},
		}

		// Create repository CR
		repo = &v1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-repo",
				Namespace: "default",
			},
			Spec: v1alpha1.RepositorySpec{
				Name:                            "test-repo",
				Archived:                        github.Ptr(false),
				AccessLevelForExternalWorkflows: accessLevelForExternalWorkflows,
				OrganizationRef: v1alpha1.OrganizationRef{
					Name: "test-org",
				},
			},
			Status: v1alpha1.RepositoryStatus{
				ID: github.Ptr(int64(123456)),
			},
		}

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
					ID:    github.Ptr(int64(123456)),
				},
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Repository]{
				Client:   k8sClient,
				Resource: repo,
			},
		}

		err = rec.reconcileActionsSettings(ctx)
	})

	Context("when actions are disabled for the organization", func() {
		BeforeEach(func() {
			orgActionsEnabled = github.Ptr("none")
		})

		It("should skip reconciliation and not call GitHub API", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setAccessLevelCalled).To(BeFalse())
		})
	})

	Context("when actions are disabled for the organization with nil value", func() {
		BeforeEach(func() {
			orgActionsEnabled = nil
		})

		It("should skip reconciliation and not call GitHub API", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setAccessLevelCalled).To(BeFalse())
		})
	})

	Context("when updating access level for external workflows", func() {
		BeforeEach(func() {
			accessLevelForExternalWorkflows = github.Ptr("organization")
			orgActionsEnabled = github.Ptr("all")
			currentAccessLevel = &github.RepositoryActionsAccessLevel{
				AccessLevel: github.Ptr("none"),
			}
		})

		It("should update the access level", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setAccessLevelCalled).To(BeTrue())
			Expect(setAccessLevelWithValue).To(Equal(github.Ptr("organization")))
		})
	})

	Context("when access level is already at desired state", func() {
		BeforeEach(func() {
			accessLevelForExternalWorkflows = github.Ptr("user")
			orgActionsEnabled = github.Ptr("all")
			currentAccessLevel = &github.RepositoryActionsAccessLevel{
				AccessLevel: github.Ptr("user"),
			}
		})

		It("should not update the access level", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setAccessLevelCalled).To(BeFalse())
		})
	})

	Context("when access level is nil in spec", func() {
		BeforeEach(func() {
			accessLevelForExternalWorkflows = nil
			orgActionsEnabled = github.Ptr("all")
			currentAccessLevel = &github.RepositoryActionsAccessLevel{
				AccessLevel: github.Ptr("user"),
			}
		})

		It("should set access level to default 'none'", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setAccessLevelCalled).To(BeTrue())
			Expect(setAccessLevelWithValue).To(Equal(github.Ptr("none")))
		})
	})

	Context("access level value variations", func() {
		BeforeEach(func() {
			orgActionsEnabled = github.Ptr("all")
			currentAccessLevel = &github.RepositoryActionsAccessLevel{
				AccessLevel: github.Ptr("none"),
			}
		})

		Context("setting access level to 'none'", func() {
			BeforeEach(func() {
				accessLevelForExternalWorkflows = github.Ptr("none")
				currentAccessLevel = &github.RepositoryActionsAccessLevel{
					AccessLevel: github.Ptr("organization"),
				}
			})

			It("should set access level to none", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(setAccessLevelCalled).To(BeTrue())
				Expect(setAccessLevelWithValue).To(Equal(github.Ptr("none")))
			})
		})

		Context("setting access level to 'organization'", func() {
			BeforeEach(func() {
				accessLevelForExternalWorkflows = github.Ptr("organization")
			})

			It("should set access level to organization", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(setAccessLevelCalled).To(BeTrue())
				Expect(setAccessLevelWithValue).To(Equal(github.Ptr("organization")))
			})
		})

		Context("setting access level to 'enterprise'", func() {
			BeforeEach(func() {
				accessLevelForExternalWorkflows = github.Ptr("enterprise")
			})

			It("should set access level to enterprise", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(setAccessLevelCalled).To(BeTrue())
				Expect(setAccessLevelWithValue).To(Equal(github.Ptr("enterprise")))
			})
		})

		Context("setting access level to 'user'", func() {
			BeforeEach(func() {
				accessLevelForExternalWorkflows = github.Ptr("user")
			})

			It("should set access level to user", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(setAccessLevelCalled).To(BeTrue())
				Expect(setAccessLevelWithValue).To(Equal(github.Ptr("user")))
			})
		})
	})

	Context("error handling", func() {
		BeforeEach(func() {
			orgActionsEnabled = github.Ptr("all")
		})

		Context("when GetAccessLevelForExternalWorkflowsForRepo fails", func() {
			BeforeEach(func() {
				getAccessLevelError = errors.New("API error: failed to get access level")
			})

			It("should return the error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("API error"))
			})
		})

		Context("when SetAccessLevelForExternalWorkflowsForRepo fails", func() {
			BeforeEach(func() {
				accessLevelForExternalWorkflows = github.Ptr("organization")
				currentAccessLevel = &github.RepositoryActionsAccessLevel{
					AccessLevel: github.Ptr("none"),
				}
				setAccessLevelError = errors.New("API error: failed to set access level")
			})

			It("should return the error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("API error"))
				Expect(setAccessLevelCalled).To(BeTrue())
			})
		})

		Context("when unable to fetch Organization CR", func() {
			var customErr error

			It("should return the error", func() {
				// Create repository CR without organization
				repoWithoutOrg := &v1alpha1.Repository{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-repo",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:                            "test-repo",
						Archived:                        github.Ptr(false),
						AccessLevelForExternalWorkflows: github.Ptr("none"),
						OrganizationRef: v1alpha1.OrganizationRef{
							Name: "test-org",
						},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: github.Ptr(int64(123456)),
					},
				}

				// Create k8s client without the organization
				customK8sClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(repoWithoutOrg).
					WithStatusSubresource(repoWithoutOrg).
					Build()

				customRec := &GitHubRepoReconciler{
					GitHub: reconciler.GitHub[GitHubRepoIdentifier]{
						Client: mockClient,
						Resource: GitHubRepoIdentifier{
							Owner: "test-org",
							Name:  "test-repo",
							ID:    github.Ptr(int64(123456)),
						},
					},
					Kubernetes: reconciler.Kubernetes[*v1alpha1.Repository]{
						Client:   customK8sClient,
						Resource: repoWithoutOrg,
					},
				}

				customErr = customRec.reconcileActionsSettings(ctx)
				Expect(customErr).To(HaveOccurred())
				Expect(customErr.Error()).To(ContainSubstring("not found"))
			})
		})
	})

	Context("edge cases", func() {
		BeforeEach(func() {
			orgActionsEnabled = github.Ptr("all")
		})

		Context("when current access level is nil", func() {
			BeforeEach(func() {
				accessLevelForExternalWorkflows = github.Ptr("organization")
				currentAccessLevel = &github.RepositoryActionsAccessLevel{
					AccessLevel: nil,
				}
			})

			It("should update access level", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(setAccessLevelCalled).To(BeTrue())
				Expect(setAccessLevelWithValue).To(Equal(github.Ptr("organization")))
			})
		})

		Context("when both spec and current access level are nil", func() {
			BeforeEach(func() {
				accessLevelForExternalWorkflows = nil
				currentAccessLevel = &github.RepositoryActionsAccessLevel{
					AccessLevel: nil,
				}
			})

			It("should set default access level 'none'", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(setAccessLevelCalled).To(BeTrue())
				Expect(setAccessLevelWithValue).To(Equal(github.Ptr("none")))
			})
		})
	})
})
