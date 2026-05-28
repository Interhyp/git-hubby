package orgrec

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

// Helper structs for tracking runner group update calls
type UpdateRunnerGroupCall struct {
	GroupID       int64
	UpdateRequest github.UpdateRunnerGroupRequest
}

type SetSelectedRepositoriesCall struct {
	GroupID       int64
	RepositoryIDs []int64
}

var _ = Describe("ReconcileActionsSettings", func() {
	var (
		ctx             context.Context
		mockClient      *ghclientmock.MockGitHubClientWrapper
		k8sClient       client.Client
		rec             *GitHubOrgReconciler
		scheme          *runtime.Scheme
		org             *v1alpha1.Organization
		actionsSettings v1alpha1.ActionsSettings
		setActions      bool
		err             error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		actionsSettings = v1alpha1.ActionsSettings{}
		setActions = true
	})

	JustBeforeEach(func() {
		org = &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-org",
				Namespace: "default",
			},
			Spec: v1alpha1.OrganizationSpec{
				Name:                    "test-org",
				GitHubAppInstallationId: 12345,
			},
		}

		if setActions {
			org.Spec.ActionsSettings = actionsSettings
		}

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(org).
			WithStatusSubresource(org).
			Build()

		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				Client:   mockClient,
				Resource: "test-org",
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Client:   k8sClient,
				Resource: org,
			},
		}

		err = rec.reconcileActionsSettings(ctx)
	})

	Context("when ActionsSettings is not set", func() {
		BeforeEach(func() {
			setActions = false

			// Setup default mock responses
			mockClient.GetActionsPermissionsForOrgFunc = func(ctx context.Context, org string) (*github.ActionsPermissions, error) {
				return &github.ActionsPermissions{}, nil
			}
			mockClient.GetActionsRetentionForOrgFunc = func(ctx context.Context, org string) (*github.ArtifactPeriod, error) {
				return &github.ArtifactPeriod{Days: new(400)}, nil
			}
			mockClient.GetActionsAllowedForOrgFunc = func(ctx context.Context, org string) (*github.ActionsAllowed, error) {
				return &github.ActionsAllowed{}, nil
			}
			mockClient.GetActionsDefaultWorkflowPermissionsForOrgFunc = func(ctx context.Context, org string) (*github.DefaultWorkflowPermissionOrganization, error) {
				return &github.DefaultWorkflowPermissionOrganization{}, nil
			}
			mockClient.GetSelfHostedRunnersSettingsForOrgFunc = func(ctx context.Context, org string) (*github.SelfHostedRunnersSettingsOrganization, error) {
				return &github.SelfHostedRunnersSettingsOrganization{
					EnabledRepositories: new("none"),
				}, nil
			}
		})

		It("should apply defaults and reconcile successfully", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when ActionsSettings is empty", func() {
		BeforeEach(func() {
			actionsSettings = v1alpha1.ActionsSettings{}

			mockClient.GetActionsPermissionsForOrgFunc = func(ctx context.Context, org string) (*github.ActionsPermissions, error) {
				return &github.ActionsPermissions{
					EnabledRepositories: new("none"),
					AllowedActions:      new("selected"),
					SHAPinningRequired:  new(false),
				}, nil
			}
			mockClient.GetActionsRetentionForOrgFunc = func(ctx context.Context, org string) (*github.ArtifactPeriod, error) {
				return &github.ArtifactPeriod{Days: new(400)}, nil
			}
			mockClient.GetActionsAllowedForOrgFunc = func(ctx context.Context, org string) (*github.ActionsAllowed, error) {
				return &github.ActionsAllowed{
					GithubOwnedAllowed: new(false),
					VerifiedAllowed:    new(false),
					PatternsAllowed:    []string{},
				}, nil
			}
			mockClient.GetActionsDefaultWorkflowPermissionsForOrgFunc = func(ctx context.Context, org string) (*github.DefaultWorkflowPermissionOrganization, error) {
				return &github.DefaultWorkflowPermissionOrganization{
					DefaultWorkflowPermissions:   new("read"),
					CanApprovePullRequestReviews: new(false),
				}, nil
			}
			mockClient.GetSelfHostedRunnersSettingsForOrgFunc = func(ctx context.Context, org string) (*github.SelfHostedRunnersSettingsOrganization, error) {
				return &github.SelfHostedRunnersSettingsOrganization{
					EnabledRepositories: new("none"),
				}, nil
			}
		})

		It("should reconcile successfully with defaults", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var _ = Describe("ReconcilePermissions", func() {
	var (
		ctx             context.Context
		mockClient      *ghclientmock.MockGitHubClientWrapper
		k8sClient       client.Client
		rec             *GitHubOrgReconciler
		scheme          *runtime.Scheme
		org             *v1alpha1.Organization
		actionsSettings v1alpha1.ActionsSettings
		err             error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		actionsSettings = v1alpha1.ActionsSettings{}
	})

	JustBeforeEach(func() {
		org = &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-org",
				Namespace: "default",
			},
			Spec: v1alpha1.OrganizationSpec{
				Name:                    "test-org",
				GitHubAppInstallationId: 12345,
				ActionsSettings:         actionsSettings,
			},
		}

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(org).
			WithStatusSubresource(org).
			Build()

		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				Client:   mockClient,
				Resource: "test-org",
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Client:   k8sClient,
				Resource: org,
			},
		}

		err = rec.reconcilePermissions(ctx)
	})

	Context("when permissions match current state", func() {
		BeforeEach(func() {
			actionsSettings.EnabledRepositories = new("all")
			actionsSettings.AllowedActions = new("all")
			actionsSettings.ShaPinningRequired = new(true)

			mockClient.GetActionsPermissionsForOrgFunc = func(ctx context.Context, org string) (*github.ActionsPermissions, error) {
				return &github.ActionsPermissions{
					EnabledRepositories: new("all"),
					AllowedActions:      new("all"),
					SHAPinningRequired:  new(true),
				}, nil
			}
		})

		It("should not update permissions", func() {
			Expect(err).NotTo(HaveOccurred())

			// Verify SetActionsPermissionsForOrg was not called
			calls := mockClient.GetActionsCalls()
			for _, call := range calls {
				Expect(call.Method).NotTo(Equal("SetActionsPermissionsForOrg"))
			}
		})
	})

	Context("when permissions need to be updated", func() {
		var setPermissionsCalled bool
		var capturedPermissions github.ActionsPermissions

		BeforeEach(func() {
			setPermissionsCalled = false
			actionsSettings.EnabledRepositories = new("selected")
			actionsSettings.AllowedActions = new("local_only")
			actionsSettings.ShaPinningRequired = new(true)

			mockClient.GetActionsPermissionsForOrgFunc = func(ctx context.Context, org string) (*github.ActionsPermissions, error) {
				return &github.ActionsPermissions{
					EnabledRepositories: new("all"),
					AllowedActions:      new("all"),
					SHAPinningRequired:  new(false),
				}, nil
			}
			mockClient.SetActionsPermissionsForOrgFunc = func(ctx context.Context, org string, permissions github.ActionsPermissions) (*github.ActionsPermissions, error) {
				setPermissionsCalled = true
				capturedPermissions = permissions
				return &permissions, nil
			}
		})

		It("should update permissions", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setPermissionsCalled).To(BeTrue())
			Expect(capturedPermissions.EnabledRepositories).To(Equal(new("selected")))
			Expect(capturedPermissions.AllowedActions).To(Equal(new("local_only")))
			Expect(capturedPermissions.SHAPinningRequired).To(Equal(new(true)))
		})
	})

	Context("when using default values", func() {
		var capturedPermissions github.ActionsPermissions

		BeforeEach(func() {
			// Only set EnabledRepositories to trigger reconciliation and defaults are used for everything else
			actionsSettings = v1alpha1.ActionsSettings{
				EnabledRepositories: new("selected"),
			}

			mockClient.GetActionsPermissionsForOrgFunc = func(ctx context.Context, org string) (*github.ActionsPermissions, error) {
				return &github.ActionsPermissions{
					EnabledRepositories: new("all"),
					AllowedActions:      new("all"),
					SHAPinningRequired:  new(true),
				}, nil
			}
			mockClient.SetActionsPermissionsForOrgFunc = func(ctx context.Context, org string, permissions github.ActionsPermissions) (*github.ActionsPermissions, error) {
				capturedPermissions = permissions
				return &permissions, nil
			}
		})

		It("should use default values (<what was input>, selected, false)", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(capturedPermissions.EnabledRepositories).To(Equal(new("selected")))
			Expect(capturedPermissions.AllowedActions).To(Equal(new("selected")))
			Expect(capturedPermissions.SHAPinningRequired).To(Equal(new(false)))
		})
	})

	Context("when GitHub API returns error on get", func() {
		BeforeEach(func() {
			mockClient.GetActionsPermissionsForOrgFunc = func(ctx context.Context, org string) (*github.ActionsPermissions, error) {
				return nil, errors.New("GitHub API error on get")
			}
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GitHub API error on get"))
		})
	})

	Context("when GitHub API returns error on set", func() {
		BeforeEach(func() {
			actionsSettings.EnabledRepositories = new("selected")

			mockClient.GetActionsPermissionsForOrgFunc = func(ctx context.Context, org string) (*github.ActionsPermissions, error) {
				return &github.ActionsPermissions{
					EnabledRepositories: new("all"),
				}, nil
			}
			mockClient.SetActionsPermissionsForOrgFunc = func(ctx context.Context, org string, permissions github.ActionsPermissions) (*github.ActionsPermissions, error) {
				return nil, errors.New("GitHub API error on set")
			}
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GitHub API error on set"))
		})
	})
})

var _ = Describe("ReconcileRetention", func() {
	var (
		ctx             context.Context
		mockClient      *ghclientmock.MockGitHubClientWrapper
		k8sClient       client.Client
		rec             *GitHubOrgReconciler
		scheme          *runtime.Scheme
		org             *v1alpha1.Organization
		actionsSettings v1alpha1.ActionsSettings
		err             error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		actionsSettings = v1alpha1.ActionsSettings{}
	})

	JustBeforeEach(func() {
		org = &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-org",
				Namespace: "default",
			},
			Spec: v1alpha1.OrganizationSpec{
				Name:                    "test-org",
				GitHubAppInstallationId: 12345,
				ActionsSettings:         actionsSettings,
			},
		}

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(org).
			WithStatusSubresource(org).
			Build()

		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				Client:   mockClient,
				Resource: "test-org",
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Client:   k8sClient,
				Resource: org,
			},
		}

		err = rec.reconcileRetention(ctx)
	})

	Context("when retention matches current state", func() {
		BeforeEach(func() {
			actionsSettings.ArtifactAndLogRetentionDays = new(90)

			mockClient.GetActionsRetentionForOrgFunc = func(ctx context.Context, org string) (*github.ArtifactPeriod, error) {
				return &github.ArtifactPeriod{
					Days: new(90),
				}, nil
			}
		})

		It("should not update retention", func() {
			Expect(err).NotTo(HaveOccurred())

			// Verify SetActionsRetentionForOrg was not called
			calls := mockClient.GetActionsCalls()
			for _, call := range calls {
				Expect(call.Method).NotTo(Equal("SetActionsRetentionForOrg"))
			}
		})
	})

	Context("when retention needs to be updated", func() {
		var setRetentionCalled bool
		var capturedRetentionDays int

		BeforeEach(func() {
			setRetentionCalled = false
			actionsSettings.ArtifactAndLogRetentionDays = new(30)

			mockClient.GetActionsRetentionForOrgFunc = func(ctx context.Context, org string) (*github.ArtifactPeriod, error) {
				return &github.ArtifactPeriod{
					Days: new(90),
				}, nil
			}
			mockClient.SetActionsRetentionForOrgFunc = func(ctx context.Context, org string, retentionInDays int) error {
				setRetentionCalled = true
				capturedRetentionDays = retentionInDays
				return nil
			}
		})

		It("should update retention", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setRetentionCalled).To(BeTrue())
			Expect(capturedRetentionDays).To(Equal(30))
		})
	})

	Context("when using default retention value", func() {
		var capturedRetentionDays int

		BeforeEach(func() {
			// Don't set retention, so default 400 is used
			actionsSettings.ArtifactAndLogRetentionDays = nil

			mockClient.GetActionsRetentionForOrgFunc = func(ctx context.Context, org string) (*github.ArtifactPeriod, error) {
				return &github.ArtifactPeriod{
					Days: new(90),
				}, nil
			}
			mockClient.SetActionsRetentionForOrgFunc = func(ctx context.Context, org string, retentionInDays int) error {
				capturedRetentionDays = retentionInDays
				return nil
			}
		})

		It("should use default value of 400 days", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(capturedRetentionDays).To(Equal(400))
		})
	})

	Context("when GitHub API returns error on get", func() {
		BeforeEach(func() {
			mockClient.GetActionsRetentionForOrgFunc = func(ctx context.Context, org string) (*github.ArtifactPeriod, error) {
				return nil, errors.New("GitHub API error on get retention")
			}
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GitHub API error on get retention"))
		})
	})

	Context("when GitHub API returns error on set", func() {
		BeforeEach(func() {
			actionsSettings.ArtifactAndLogRetentionDays = new(30)

			mockClient.GetActionsRetentionForOrgFunc = func(ctx context.Context, org string) (*github.ArtifactPeriod, error) {
				return &github.ArtifactPeriod{
					Days: new(90),
				}, nil
			}
			mockClient.SetActionsRetentionForOrgFunc = func(ctx context.Context, org string, retentionInDays int) error {
				return errors.New("GitHub API error on set retention")
			}
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GitHub API error on set retention"))
		})
	})
})

var _ = Describe("ReconcileAllowedActions", func() {
	var (
		ctx             context.Context
		mockClient      *ghclientmock.MockGitHubClientWrapper
		k8sClient       client.Client
		rec             *GitHubOrgReconciler
		scheme          *runtime.Scheme
		org             *v1alpha1.Organization
		actionsSettings v1alpha1.ActionsSettings
		err             error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		actionsSettings = v1alpha1.ActionsSettings{
			EnabledRepositories: new("all"),
		}
	})

	JustBeforeEach(func() {
		org = &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-org",
				Namespace: "default",
			},
			Spec: v1alpha1.OrganizationSpec{
				Name:                    "test-org",
				GitHubAppInstallationId: 12345,
				ActionsSettings:         actionsSettings,
			},
		}

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(org).
			WithStatusSubresource(org).
			Build()

		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				Client:   mockClient,
				Resource: "test-org",
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Client:   k8sClient,
				Resource: org,
			},
		}

		err = rec.reconcileAllowedActions(ctx)
	})

	Context("when SelectedAllowedActions is nil", func() {
		var capturedAllowedActions github.ActionsAllowed

		BeforeEach(func() {
			actionsSettings.SelectedAllowedActions = nil

			mockClient.GetActionsAllowedForOrgFunc = func(ctx context.Context, org string) (*github.ActionsAllowed, error) {
				return &github.ActionsAllowed{
					GithubOwnedAllowed: new(true),
					VerifiedAllowed:    new(true),
					PatternsAllowed:    []string{"some/pattern@*"},
				}, nil
			}
			mockClient.SetActionsAllowedForOrgFunc = func(ctx context.Context, org string, allowedActions github.ActionsAllowed) (*github.ActionsAllowed, error) {
				capturedAllowedActions = allowedActions
				return &allowedActions, nil
			}
		})

		It("should set all flags to false and empty patterns", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(capturedAllowedActions.GithubOwnedAllowed).To(Equal(new(false)))
			Expect(capturedAllowedActions.VerifiedAllowed).To(Equal(new(false)))
			Expect(capturedAllowedActions.PatternsAllowed).To(BeEmpty())
		})
	})

	Context("when SelectedAllowedActions is specified", func() {
		var setAllowedCalled bool
		var capturedAllowedActions github.ActionsAllowed

		BeforeEach(func() {
			setAllowedCalled = false
			actionsSettings.SelectedAllowedActions = &v1alpha1.SelectedAllowedActions{
				GitHubOwnedAllowed: new(true),
				VerifiedAllowed:    new(true),
				PatternsAllowed:    []string{"org/action@*", "another/action@v1"},
			}

			mockClient.GetActionsAllowedForOrgFunc = func(ctx context.Context, org string) (*github.ActionsAllowed, error) {
				return &github.ActionsAllowed{
					GithubOwnedAllowed: new(false),
					VerifiedAllowed:    new(false),
					PatternsAllowed:    []string{},
				}, nil
			}
			mockClient.SetActionsAllowedForOrgFunc = func(ctx context.Context, org string, allowedActions github.ActionsAllowed) (*github.ActionsAllowed, error) {
				setAllowedCalled = true
				capturedAllowedActions = allowedActions
				return &allowedActions, nil
			}
		})

		It("should update allowed actions with specified values", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setAllowedCalled).To(BeTrue())
			Expect(capturedAllowedActions.GithubOwnedAllowed).To(Equal(new(true)))
			Expect(capturedAllowedActions.VerifiedAllowed).To(Equal(new(true)))
			Expect(capturedAllowedActions.PatternsAllowed).To(ConsistOf("org/action@*", "another/action@v1"))
		})
	})

	Context("when allowed actions match current state", func() {
		BeforeEach(func() {
			actionsSettings.SelectedAllowedActions = &v1alpha1.SelectedAllowedActions{
				GitHubOwnedAllowed: new(true),
				VerifiedAllowed:    new(false),
				PatternsAllowed:    []string{"org/action@*"},
			}

			mockClient.GetActionsAllowedForOrgFunc = func(ctx context.Context, org string) (*github.ActionsAllowed, error) {
				return &github.ActionsAllowed{
					GithubOwnedAllowed: new(true),
					VerifiedAllowed:    new(false),
					PatternsAllowed:    []string{"org/action@*"},
				}, nil
			}
		})

		It("should not update allowed actions", func() {
			Expect(err).NotTo(HaveOccurred())

			// Verify SetActionsAllowedForOrg was not called
			calls := mockClient.GetActionsCalls()
			for _, call := range calls {
				Expect(call.Method).NotTo(Equal("SetActionsAllowedForOrg"))
			}
		})
	})

	Context("when using default values in SelectedAllowedActions", func() {
		var capturedAllowedActions github.ActionsAllowed

		BeforeEach(func() {
			actionsSettings.SelectedAllowedActions = &v1alpha1.SelectedAllowedActions{
				// All fields are nil, so defaults should be used
			}

			mockClient.GetActionsAllowedForOrgFunc = func(ctx context.Context, org string) (*github.ActionsAllowed, error) {
				return &github.ActionsAllowed{
					GithubOwnedAllowed: new(true),
					VerifiedAllowed:    new(true),
					PatternsAllowed:    []string{"some/pattern@*"},
				}, nil
			}
			mockClient.SetActionsAllowedForOrgFunc = func(ctx context.Context, org string, allowedActions github.ActionsAllowed) (*github.ActionsAllowed, error) {
				capturedAllowedActions = allowedActions
				return &allowedActions, nil
			}
		})

		It("should use default values (false, false, empty)", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(capturedAllowedActions.GithubOwnedAllowed).To(Equal(new(false)))
			Expect(capturedAllowedActions.VerifiedAllowed).To(Equal(new(false)))
			Expect(capturedAllowedActions.PatternsAllowed).To(BeEmpty())
		})
	})

	Context("when GitHub API returns error on get", func() {
		BeforeEach(func() {
			mockClient.GetActionsAllowedForOrgFunc = func(ctx context.Context, org string) (*github.ActionsAllowed, error) {
				return nil, errors.New("GitHub API error on get allowed actions")
			}
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GitHub API error on get allowed actions"))
		})
	})

	Context("when GitHub API returns error on set", func() {
		BeforeEach(func() {
			actionsSettings.SelectedAllowedActions = &v1alpha1.SelectedAllowedActions{
				GitHubOwnedAllowed: new(true),
			}

			mockClient.GetActionsAllowedForOrgFunc = func(ctx context.Context, org string) (*github.ActionsAllowed, error) {
				return &github.ActionsAllowed{
					GithubOwnedAllowed: new(false),
				}, nil
			}
			mockClient.SetActionsAllowedForOrgFunc = func(ctx context.Context, org string, allowedActions github.ActionsAllowed) (*github.ActionsAllowed, error) {
				return nil, errors.New("GitHub API error on set allowed actions")
			}
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GitHub API error on set allowed actions"))
		})
	})
})

var _ = Describe("ReconcileDefaultWorkflowPermissions", func() {
	var (
		ctx             context.Context
		mockClient      *ghclientmock.MockGitHubClientWrapper
		k8sClient       client.Client
		rec             *GitHubOrgReconciler
		scheme          *runtime.Scheme
		org             *v1alpha1.Organization
		actionsSettings v1alpha1.ActionsSettings
		err             error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		actionsSettings = v1alpha1.ActionsSettings{}
	})

	JustBeforeEach(func() {
		org = &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-org",
				Namespace: "default",
			},
			Spec: v1alpha1.OrganizationSpec{
				Name:                    "test-org",
				GitHubAppInstallationId: 12345,
				ActionsSettings:         actionsSettings,
			},
		}

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(org).
			WithStatusSubresource(org).
			Build()

		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				Client:   mockClient,
				Resource: "test-org",
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Client:   k8sClient,
				Resource: org,
			},
		}

		err = rec.reconcileDefaultWorkflowPermissions(ctx)
	})

	Context("when workflow permissions match current state", func() {
		BeforeEach(func() {
			actionsSettings.DefaultWorkflowPermissions = new("write")
			actionsSettings.CanApprovePullRequestReviews = new(true)

			mockClient.GetActionsDefaultWorkflowPermissionsForOrgFunc = func(ctx context.Context, org string) (*github.DefaultWorkflowPermissionOrganization, error) {
				return &github.DefaultWorkflowPermissionOrganization{
					DefaultWorkflowPermissions:   new("write"),
					CanApprovePullRequestReviews: new(true),
				}, nil
			}
		})

		It("should not update workflow permissions", func() {
			Expect(err).NotTo(HaveOccurred())

			// Verify SetActionsDefaultWorkflowPermissionsForOrg was not called
			calls := mockClient.GetActionsCalls()
			for _, call := range calls {
				Expect(call.Method).NotTo(Equal("SetActionsDefaultWorkflowPermissionsForOrg"))
			}
		})
	})

	Context("when workflow permissions need to be updated", func() {
		var setPermissionsCalled bool
		var capturedPermissions github.DefaultWorkflowPermissionOrganization

		BeforeEach(func() {
			setPermissionsCalled = false
			actionsSettings.DefaultWorkflowPermissions = new("write")
			actionsSettings.CanApprovePullRequestReviews = new(true)

			mockClient.GetActionsDefaultWorkflowPermissionsForOrgFunc = func(ctx context.Context, org string) (*github.DefaultWorkflowPermissionOrganization, error) {
				return &github.DefaultWorkflowPermissionOrganization{
					DefaultWorkflowPermissions:   new("read"),
					CanApprovePullRequestReviews: new(false),
				}, nil
			}
			mockClient.SetActionsDefaultWorkflowPermissionsForOrgFunc = func(ctx context.Context, org string, permissions github.DefaultWorkflowPermissionOrganization) (*github.DefaultWorkflowPermissionOrganization, error) {
				setPermissionsCalled = true
				capturedPermissions = permissions
				return &permissions, nil
			}
		})

		It("should update workflow permissions", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setPermissionsCalled).To(BeTrue())
			Expect(capturedPermissions.DefaultWorkflowPermissions).To(Equal(new("write")))
			Expect(capturedPermissions.CanApprovePullRequestReviews).To(Equal(new(true)))
		})
	})

	Context("when using default values", func() {
		var capturedPermissions github.DefaultWorkflowPermissionOrganization

		BeforeEach(func() {
			// Don't set any values, so defaults are used
			actionsSettings.DefaultWorkflowPermissions = nil
			actionsSettings.CanApprovePullRequestReviews = nil

			mockClient.GetActionsDefaultWorkflowPermissionsForOrgFunc = func(ctx context.Context, org string) (*github.DefaultWorkflowPermissionOrganization, error) {
				return &github.DefaultWorkflowPermissionOrganization{
					DefaultWorkflowPermissions:   new("write"),
					CanApprovePullRequestReviews: new(true),
				}, nil
			}
			mockClient.SetActionsDefaultWorkflowPermissionsForOrgFunc = func(ctx context.Context, org string, permissions github.DefaultWorkflowPermissionOrganization) (*github.DefaultWorkflowPermissionOrganization, error) {
				capturedPermissions = permissions
				return &permissions, nil
			}
		})

		It("should use default values (read, false)", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(capturedPermissions.DefaultWorkflowPermissions).To(Equal(new("read")))
			Expect(capturedPermissions.CanApprovePullRequestReviews).To(Equal(new(false)))
		})
	})

	Context("when GitHub API returns error on get", func() {
		BeforeEach(func() {
			mockClient.GetActionsDefaultWorkflowPermissionsForOrgFunc = func(ctx context.Context, org string) (*github.DefaultWorkflowPermissionOrganization, error) {
				return nil, errors.New("GitHub API error on get workflow permissions")
			}
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GitHub API error on get workflow permissions"))
		})
	})

	Context("when GitHub API returns error on set", func() {
		BeforeEach(func() {
			actionsSettings.DefaultWorkflowPermissions = new("write")

			mockClient.GetActionsDefaultWorkflowPermissionsForOrgFunc = func(ctx context.Context, org string) (*github.DefaultWorkflowPermissionOrganization, error) {
				return &github.DefaultWorkflowPermissionOrganization{
					DefaultWorkflowPermissions: new("read"),
				}, nil
			}
			mockClient.SetActionsDefaultWorkflowPermissionsForOrgFunc = func(ctx context.Context, org string, permissions github.DefaultWorkflowPermissionOrganization) (*github.DefaultWorkflowPermissionOrganization, error) {
				return nil, errors.New("GitHub API error on set workflow permissions")
			}
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GitHub API error on set workflow permissions"))
		})
	})
})

var _ = Describe("ReconcileSelfHostedRunnerSettings", func() {
	var (
		ctx             context.Context
		mockClient      *ghclientmock.MockGitHubClientWrapper
		k8sClient       client.Client
		rec             *GitHubOrgReconciler
		scheme          *runtime.Scheme
		org             *v1alpha1.Organization
		actionsSettings v1alpha1.ActionsSettings
		err             error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		actionsSettings = v1alpha1.ActionsSettings{}
	})

	JustBeforeEach(func() {
		org = &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-org",
				Namespace: "default",
			},
			Spec: v1alpha1.OrganizationSpec{
				Name:                    "test-org",
				GitHubAppInstallationId: 12345,
				ActionsSettings:         actionsSettings,
			},
		}

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(org).
			WithStatusSubresource(org).
			Build()

		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				Client:   mockClient,
				Resource: "test-org",
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Client:   k8sClient,
				Resource: org,
			},
		}

		err = rec.reconcileSelfHostedRunnerSettings(ctx)
	})

	Context("when enabled repositories is already none", func() {
		BeforeEach(func() {
			mockClient.GetSelfHostedRunnersSettingsForOrgFunc = func(ctx context.Context, org string) (*github.SelfHostedRunnersSettingsOrganization, error) {
				return &github.SelfHostedRunnersSettingsOrganization{
					EnabledRepositories: new("none"),
				}, nil
			}
		})

		It("should not update settings", func() {
			Expect(err).NotTo(HaveOccurred())

			// Verify SetSelfHostedRunnersSettingsForOrg was not called
			calls := mockClient.GetActionsCalls()
			for _, call := range calls {
				Expect(call.Method).NotTo(Equal("SetSelfHostedRunnersSettingsForOrg"))
			}
		})
	})

	Context("when enabled repositories needs to be set to none", func() {
		var setSettingsCalled bool
		var capturedSettings github.SelfHostedRunnersSettingsOrganizationOpt

		BeforeEach(func() {
			setSettingsCalled = false

			mockClient.GetSelfHostedRunnersSettingsForOrgFunc = func(ctx context.Context, org string) (*github.SelfHostedRunnersSettingsOrganization, error) {
				return &github.SelfHostedRunnersSettingsOrganization{
					EnabledRepositories: new("all"),
				}, nil
			}
			mockClient.SetSelfHostedRunnersSettingsForOrgFunc = func(ctx context.Context, org string, settings github.SelfHostedRunnersSettingsOrganizationOpt) error {
				setSettingsCalled = true
				capturedSettings = settings
				return nil
			}
		})

		It("should update settings to none", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setSettingsCalled).To(BeTrue())
			Expect(capturedSettings.EnabledRepositories).To(Equal(new("none")))
		})
	})

	Context("when enabled repositories is selected", func() {
		var setSettingsCalled bool

		BeforeEach(func() {
			setSettingsCalled = false

			mockClient.GetSelfHostedRunnersSettingsForOrgFunc = func(ctx context.Context, org string) (*github.SelfHostedRunnersSettingsOrganization, error) {
				return &github.SelfHostedRunnersSettingsOrganization{
					EnabledRepositories: new("selected"),
				}, nil
			}
			mockClient.SetSelfHostedRunnersSettingsForOrgFunc = func(ctx context.Context, org string, settings github.SelfHostedRunnersSettingsOrganizationOpt) error {
				setSettingsCalled = true
				return nil
			}
		})

		It("should update settings to none", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setSettingsCalled).To(BeTrue())
		})
	})

	Context("when GitHub API returns error on get", func() {
		BeforeEach(func() {
			mockClient.GetSelfHostedRunnersSettingsForOrgFunc = func(ctx context.Context, org string) (*github.SelfHostedRunnersSettingsOrganization, error) {
				return nil, errors.New("GitHub API error on get runner settings")
			}
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GitHub API error on get runner settings"))
		})
	})

	Context("when GitHub API returns error on set", func() {
		BeforeEach(func() {
			mockClient.GetSelfHostedRunnersSettingsForOrgFunc = func(ctx context.Context, org string) (*github.SelfHostedRunnersSettingsOrganization, error) {
				return &github.SelfHostedRunnersSettingsOrganization{
					EnabledRepositories: new("all"),
				}, nil
			}
			mockClient.SetSelfHostedRunnersSettingsForOrgFunc = func(ctx context.Context, org string, settings github.SelfHostedRunnersSettingsOrganizationOpt) error {
				return errors.New("GitHub API error on set runner settings")
			}
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GitHub API error on set runner settings"))
		})
	})
})

var _ = Describe("ReconcileActionsEnabled", func() {
	var (
		ctx                             context.Context
		mockClient                      *ghclientmock.MockGitHubClientWrapper
		k8sClient                       client.Client
		rec                             *GitHubOrgReconciler
		scheme                          *runtime.Scheme
		org                             *v1alpha1.Organization
		repos                           []*v1alpha1.Repository
		currentEnabledRepos             []*github.Repository
		err                             error
		getActionsEnabledReposError     error
		setActionsEnabledReposError     error
		setActionsEnabledReposCalled    bool
		setActionsEnabledReposWithValue []int64
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		// Default values
		repos = []*v1alpha1.Repository{}
		currentEnabledRepos = []*github.Repository{}
		getActionsEnabledReposError = nil
		setActionsEnabledReposError = nil
		setActionsEnabledReposCalled = false
		setActionsEnabledReposWithValue = nil

		// Set up default mock functions
		mockClient.GetActionsEnabledRepositoriesForOrgFunc = func(ctx context.Context, org string) ([]*github.Repository, error) {
			return currentEnabledRepos, getActionsEnabledReposError
		}

		mockClient.SetActionsEnabledRepositoriesForOrgFunc = func(ctx context.Context, org string, repoIDs []int64) error {
			setActionsEnabledReposCalled = true
			setActionsEnabledReposWithValue = repoIDs
			return setActionsEnabledReposError
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
				Name:                    "test-org",
				GitHubAppInstallationId: 12345,
				ActionsSettings: v1alpha1.ActionsSettings{
					EnabledRepositories: new("all"),
				},
			},
		}

		// Create k8s client with org and repos
		objects := make([]client.Object, 1, 1+len(repos))
		objects[0] = org
		for _, repo := range repos {
			objects = append(objects, repo)
		}

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(objects...).
			WithStatusSubresource(objects...).
			WithIndex(&v1alpha1.Repository{}, "spec.organizationRef.name", func(obj client.Object) []string {
				repo := obj.(*v1alpha1.Repository)
				return []string{repo.Spec.OrganizationRef.Name}
			}).
			Build()

		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				Client:   mockClient,
				Resource: "test-org",
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Client:   k8sClient,
				Resource: org,
			},
		}

		err = rec.reconcileActionsEnabledRepositories(ctx)
	})

	Context("with no repositories", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{}
			currentEnabledRepos = []*github.Repository{}
		})

		It("should not call API when current state matches (both empty)", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setActionsEnabledReposCalled).To(BeFalse())
		})
	})

	Context("with no repositories but current has repos", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{}
			currentEnabledRepos = []*github.Repository{
				{ID: new(int64(999))},
			}
		})

		It("should set empty list to remove current repos", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setActionsEnabledReposCalled).To(BeTrue())
			Expect(setActionsEnabledReposWithValue).To(BeEmpty())
		})
	})

	Context("with repositories that have actions enabled", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:            "repo1",
						ActionsEnabled:  new(true),
						OrganizationRef: v1alpha1.OrganizationRef{Name: "test-org"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(111)),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo2",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:            "repo2",
						ActionsEnabled:  new(true),
						OrganizationRef: v1alpha1.OrganizationRef{Name: "test-org"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(222)),
					},
				},
			}
			currentEnabledRepos = []*github.Repository{}
		})

		It("should enable actions for all repos", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setActionsEnabledReposCalled).To(BeTrue())
			Expect(setActionsEnabledReposWithValue).To(ConsistOf(int64(111), int64(222)))
		})
	})

	Context("with repositories that have actions disabled", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:            "repo1",
						ActionsEnabled:  new(false),
						OrganizationRef: v1alpha1.OrganizationRef{Name: "test-org"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(111)),
					},
				},
			}
			currentEnabledRepos = []*github.Repository{
				{ID: new(int64(111))},
			}
		})

		It("should not include disabled repos", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setActionsEnabledReposCalled).To(BeTrue())
			Expect(setActionsEnabledReposWithValue).To(BeEmpty())
		})
	})

	Context("with mixed repositories", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:            "repo1",
						ActionsEnabled:  new(true),
						OrganizationRef: v1alpha1.OrganizationRef{Name: "test-org"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(111)),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo2",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:            "repo2",
						ActionsEnabled:  new(false),
						OrganizationRef: v1alpha1.OrganizationRef{Name: "test-org"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(222)),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo3",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:            "repo3",
						ActionsEnabled:  new(true),
						OrganizationRef: v1alpha1.OrganizationRef{Name: "test-org"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(333)),
					},
				},
			}
			currentEnabledRepos = []*github.Repository{}
		})

		It("should only enable actions for repos with ActionsEnabled=true", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setActionsEnabledReposCalled).To(BeTrue())
			Expect(setActionsEnabledReposWithValue).To(ConsistOf(int64(111), int64(333)))
		})
	})

	Context("when current state matches desired state", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:            "repo1",
						ActionsEnabled:  new(true),
						OrganizationRef: v1alpha1.OrganizationRef{Name: "test-org"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(111)),
					},
				},
			}
			currentEnabledRepos = []*github.Repository{
				{ID: new(int64(111))},
			}
		})

		It("should not call SetActionsEnabledRepositoriesForOrg", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setActionsEnabledReposCalled).To(BeFalse())
		})
	})

	Context("when repository has nil ID", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:            "repo1",
						ActionsEnabled:  new(true),
						OrganizationRef: v1alpha1.OrganizationRef{Name: "test-org"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: nil,
					},
				},
			}
			currentEnabledRepos = []*github.Repository{}
		})

		It("should skip repos with nil ID and not call API when result matches current state", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setActionsEnabledReposCalled).To(BeFalse())
		})
	})

	Context("when repository has nil ID but current has repos", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:            "repo1",
						ActionsEnabled:  new(true),
						OrganizationRef: v1alpha1.OrganizationRef{Name: "test-org"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: nil,
					},
				},
			}
			currentEnabledRepos = []*github.Repository{
				{ID: new(int64(999))},
			}
		})

		It("should clear current repos since no valid repos exist", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setActionsEnabledReposCalled).To(BeTrue())
			Expect(setActionsEnabledReposWithValue).To(BeEmpty())
		})
	})

	Context("when GetActionsEnabledRepositoriesForOrg fails", func() {
		BeforeEach(func() {
			getActionsEnabledReposError = errors.New("API error: failed to get enabled repositories")
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API error"))
			Expect(setActionsEnabledReposCalled).To(BeFalse())
		})
	})

	Context("when SetActionsEnabledRepositoriesForOrg fails", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:            "repo1",
						ActionsEnabled:  new(true),
						OrganizationRef: v1alpha1.OrganizationRef{Name: "test-org"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(111)),
					},
				},
			}
			currentEnabledRepos = []*github.Repository{}
			setActionsEnabledReposError = errors.New("API error: failed to set enabled repositories")
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API error"))
			Expect(setActionsEnabledReposCalled).To(BeTrue())
		})
	})

	Context("when org has repositories from different orgs", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:            "repo1",
						ActionsEnabled:  new(true),
						OrganizationRef: v1alpha1.OrganizationRef{Name: "test-org"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(111)),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo2",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:            "repo2",
						ActionsEnabled:  new(true),
						OrganizationRef: v1alpha1.OrganizationRef{Name: "other-org"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(222)),
					},
				},
			}
			currentEnabledRepos = []*github.Repository{}
		})

		It("should only include repos from test-org", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setActionsEnabledReposCalled).To(BeTrue())
			Expect(setActionsEnabledReposWithValue).To(ConsistOf(int64(111)))
		})
	})

	Context("when current enabled repos has different IDs", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:            "repo1",
						ActionsEnabled:  new(true),
						OrganizationRef: v1alpha1.OrganizationRef{Name: "test-org"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(111)),
					},
				},
			}
			currentEnabledRepos = []*github.Repository{
				{ID: new(int64(222))}, // Different repo enabled
			}
		})

		It("should update to match desired state", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setActionsEnabledReposCalled).To(BeTrue())
			Expect(setActionsEnabledReposWithValue).To(ConsistOf(int64(111)))
		})
	})

	Context("when current has more repos than desired", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:            "repo1",
						ActionsEnabled:  new(true),
						OrganizationRef: v1alpha1.OrganizationRef{Name: "test-org"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(111)),
					},
				},
			}
			currentEnabledRepos = []*github.Repository{
				{ID: new(int64(111))},
				{ID: new(int64(222))},
				{ID: new(int64(333))},
			}
		})

		It("should update to match desired state with fewer repos", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setActionsEnabledReposCalled).To(BeTrue())
			Expect(setActionsEnabledReposWithValue).To(ConsistOf(int64(111)))
		})
	})

	Context("when repository has nil ActionsEnabled (defaults to true)", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:            "repo1",
						ActionsEnabled:  nil, // should default to true
						OrganizationRef: v1alpha1.OrganizationRef{Name: "test-org"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(111)),
					},
				},
			}
			currentEnabledRepos = []*github.Repository{}
		})

		It("should include the repo in the enabled list", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setActionsEnabledReposCalled).To(BeTrue())
			Expect(setActionsEnabledReposWithValue).To(ConsistOf(int64(111)))
		})
	})
})

var _ = Describe("containsSameIDs", func() {
	It("should return true for matching IDs", func() {
		current := []*github.Repository{
			{ID: new(int64(111))},
			{ID: new(int64(222))},
		}
		desired := map[int64]any{
			111: nil,
			222: nil,
		}
		Expect(containsSameIDs(current, desired)).To(BeTrue())
	})

	It("should return false for different counts", func() {
		current := []*github.Repository{
			{ID: new(int64(111))},
		}
		desired := map[int64]any{
			111: nil,
			222: nil,
		}
		Expect(containsSameIDs(current, desired)).To(BeFalse())
	})

	It("should return false for different IDs", func() {
		current := []*github.Repository{
			{ID: new(int64(111))},
			{ID: new(int64(333))},
		}
		desired := map[int64]any{
			111: nil,
			222: nil,
		}
		Expect(containsSameIDs(current, desired)).To(BeFalse())
	})

	It("should handle empty lists", func() {
		current := []*github.Repository{}
		desired := map[int64]any{}
		Expect(containsSameIDs(current, desired)).To(BeTrue())
	})

	It("should return false when current is empty but desired is not", func() {
		current := []*github.Repository{}
		desired := map[int64]any{
			111: nil,
		}
		Expect(containsSameIDs(current, desired)).To(BeFalse())
	})

	It("should return false when desired is empty but current is not", func() {
		current := []*github.Repository{
			{ID: new(int64(111))},
		}
		desired := map[int64]any{}
		Expect(containsSameIDs(current, desired)).To(BeFalse())
	})
})

var _ = Describe("ReconcileRunnerGroups", func() {
	var (
		ctx                          context.Context
		mockClient                   *ghclientmock.MockGitHubClientWrapper
		k8sClient                    client.Client
		rec                          *GitHubOrgReconciler
		scheme                       *runtime.Scheme
		org                          *v1alpha1.Organization
		repos                        []*v1alpha1.Repository
		actionsSettings              v1alpha1.ActionsSettings
		currentRunnerGroups          []*github.RunnerGroup
		err                          error
		getRunnerGroupsError         error
		createRunnerGroupError       error
		updateRunnerGroupError       error
		deleteRunnerGroupError       error
		setSelectedRepositoriesError error
		createRunnerGroupCalls       []github.CreateRunnerGroupRequest
		updateRunnerGroupCalls       []UpdateRunnerGroupCall
		deleteRunnerGroupCalls       []int64
		setSelectedRepositoriesCalls []SetSelectedRepositoriesCall
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		// Default values
		repos = []*v1alpha1.Repository{}
		actionsSettings = v1alpha1.ActionsSettings{}
		currentRunnerGroups = []*github.RunnerGroup{}
		getRunnerGroupsError = nil
		createRunnerGroupError = nil
		updateRunnerGroupError = nil
		deleteRunnerGroupError = nil
		setSelectedRepositoriesError = nil
		createRunnerGroupCalls = []github.CreateRunnerGroupRequest{}
		updateRunnerGroupCalls = []UpdateRunnerGroupCall{}
		deleteRunnerGroupCalls = []int64{}
		setSelectedRepositoriesCalls = []SetSelectedRepositoriesCall{}

		// Set up mock functions
		mockClient.GetRunnerGroupsForOrgFunc = func(ctx context.Context, org string) ([]*github.RunnerGroup, error) {
			return currentRunnerGroups, getRunnerGroupsError
		}

		mockClient.CreateRunnerGroupForOrgFunc = func(ctx context.Context, org string, createRequest github.CreateRunnerGroupRequest) (*github.RunnerGroup, error) {
			createRunnerGroupCalls = append(createRunnerGroupCalls, createRequest)
			if createRunnerGroupError != nil {
				return nil, createRunnerGroupError
			}
			return &github.RunnerGroup{
				ID:                    new(int64(999)),
				Name:                  createRequest.Name,
				Visibility:            createRequest.Visibility,
				RestrictedToWorkflows: createRequest.RestrictedToWorkflows,
				SelectedWorkflows:     createRequest.SelectedWorkflows,
			}, nil
		}

		mockClient.UpdateRunnerGroupForOrgFunc = func(ctx context.Context, org string, groupID int64, updateRequest github.UpdateRunnerGroupRequest) (*github.RunnerGroup, error) {
			updateRunnerGroupCalls = append(updateRunnerGroupCalls, UpdateRunnerGroupCall{
				GroupID:       groupID,
				UpdateRequest: updateRequest,
			})
			if updateRunnerGroupError != nil {
				return nil, updateRunnerGroupError
			}
			return &github.RunnerGroup{
				ID:                    new(groupID),
				Name:                  updateRequest.Name,
				Visibility:            updateRequest.Visibility,
				RestrictedToWorkflows: updateRequest.RestrictedToWorkflows,
				SelectedWorkflows:     updateRequest.SelectedWorkflows,
			}, nil
		}

		mockClient.DeleteRunnerGroupForOrgFunc = func(ctx context.Context, org string, groupID int64) error {
			deleteRunnerGroupCalls = append(deleteRunnerGroupCalls, groupID)
			return deleteRunnerGroupError
		}

		mockClient.SetSelectedRepositoriesForRunnerGroupFunc = func(ctx context.Context, org string, groupID int64, selectedRepositoryIDs []int64) error {
			setSelectedRepositoriesCalls = append(setSelectedRepositoriesCalls, SetSelectedRepositoriesCall{
				GroupID:       groupID,
				RepositoryIDs: selectedRepositoryIDs,
			})
			return setSelectedRepositoriesError
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
				Name:                    "test-org",
				GitHubAppInstallationId: 12345,
				ActionsSettings:         actionsSettings,
			},
		}

		// Create k8s client with org and repos
		objects := make([]client.Object, 1, 1+len(repos))
		objects[0] = org
		for _, repo := range repos {
			objects = append(objects, repo)
		}

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(objects...).
			WithStatusSubresource(objects...).
			WithIndex(&v1alpha1.Repository{}, "spec.organizationRef.name", func(obj client.Object) []string {
				repo := obj.(*v1alpha1.Repository)
				return []string{repo.Spec.OrganizationRef.Name}
			}).
			Build()

		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				Client:   mockClient,
				Resource: "test-org",
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Client:   k8sClient,
				Resource: org,
			},
		}

		err = rec.reconcileRunnerGroups(ctx)
	})

	Context("with no runner groups", func() {
		BeforeEach(func() {
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{}
			currentRunnerGroups = []*github.RunnerGroup{}
		})

		It("should not call any API when both are empty", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRunnerGroupCalls).To(BeEmpty())
			Expect(deleteRunnerGroupCalls).To(BeEmpty())
		})
	})

	Context("with new runner group to create", func() {
		BeforeEach(func() {
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:                  "test-group",
					Visibility:            new("all"),
					RestrictedToWorkflows: new(false),
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{}
		})

		It("should create the runner group", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRunnerGroupCalls).To(HaveLen(1))
			Expect(createRunnerGroupCalls[0].Name).To(Equal(new("test-group")))
			Expect(createRunnerGroupCalls[0].Visibility).To(Equal(new("all")))
			Expect(deleteRunnerGroupCalls).To(BeEmpty())
		})
	})

	Context("with existing runner group to delete", func() {
		BeforeEach(func() {
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{}
			currentRunnerGroups = []*github.RunnerGroup{
				{
					ID:                    new(int64(111)),
					Name:                  new("old-group"),
					Visibility:            new("all"),
					RestrictedToWorkflows: new(false),
				},
			}
		})

		It("should delete the runner group", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deleteRunnerGroupCalls).To(HaveLen(1))
			Expect(deleteRunnerGroupCalls[0]).To(Equal(int64(111)))
			Expect(createRunnerGroupCalls).To(BeEmpty())
		})
	})

	Context("with matching runner group by name", func() {
		BeforeEach(func() {
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:                  "test-group",
					Visibility:            new("all"),
					RestrictedToWorkflows: new(false),
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{
				{
					ID:                    new(int64(111)),
					Name:                  new("test-group"),
					Visibility:            new("all"),
					RestrictedToWorkflows: new(false),
				},
			}
		})

		It("should not create or delete anything", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRunnerGroupCalls).To(BeEmpty())
			Expect(deleteRunnerGroupCalls).To(BeEmpty())
		})
	})

	Context("when runner group properties differ", func() {
		BeforeEach(func() {
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:                  "test-group",
					Visibility:            new("selected"),
					RestrictedToWorkflows: new(true),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/ci.yaml@main"},
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{
				{
					ID:                    new(int64(111)),
					Name:                  new("test-group"),
					Visibility:            new("all"),
					RestrictedToWorkflows: new(false),
				},
			}
		})

		It("should update the runner group but not set repositories when IDs match", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deleteRunnerGroupCalls).To(BeEmpty())
			Expect(createRunnerGroupCalls).To(BeEmpty())
			Expect(updateRunnerGroupCalls).To(HaveLen(1))
			Expect(updateRunnerGroupCalls[0].GroupID).To(Equal(int64(111)))
			Expect(updateRunnerGroupCalls[0].UpdateRequest.Name).To(Equal(new("test-group")))
			Expect(updateRunnerGroupCalls[0].UpdateRequest.Visibility).To(Equal(new("selected")))
			Expect(updateRunnerGroupCalls[0].UpdateRequest.RestrictedToWorkflows).To(Equal(new(true)))
			// Repository IDs are equal (both empty: current via mock default, desired has no repos), so no SetSelectedRepositories call
			Expect(setSelectedRepositoriesCalls).To(BeEmpty())
		})
	})

	Context("with multiple runner groups", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:                         "repo1",
						OrganizationRef:              v1alpha1.OrganizationRef{Name: "test-org"},
						AvailableActionsRunnerGroups: []string{"group1"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(100)),
					},
				},
			}
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:       "group1",
					Visibility: new("selected"),
				},
				{
					Name:       "group2",
					Visibility: new("all"),
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{
				{
					ID:   new(int64(222)),
					Name: new("old-group"),
				},
			}
		})

		It("should delete old group and create new groups", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deleteRunnerGroupCalls).To(HaveLen(1))
			Expect(deleteRunnerGroupCalls[0]).To(Equal(int64(222)))
			Expect(createRunnerGroupCalls).To(HaveLen(2))

			names := []string{*createRunnerGroupCalls[0].Name, *createRunnerGroupCalls[1].Name}
			Expect(names).To(ConsistOf("group1", "group2"))
		})
	})

	Context("when GetRunnerGroupsForOrg fails", func() {
		BeforeEach(func() {
			getRunnerGroupsError = errors.New("API error: failed to get runner groups")
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API error"))
			Expect(createRunnerGroupCalls).To(BeEmpty())
			Expect(deleteRunnerGroupCalls).To(BeEmpty())
		})
	})

	Context("when DeleteRunnerGroupForOrg fails", func() {
		BeforeEach(func() {
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{}
			currentRunnerGroups = []*github.RunnerGroup{
				{
					ID:   new(int64(111)),
					Name: new("old-group"),
				},
			}
			deleteRunnerGroupError = errors.New("API error: failed to delete runner group")
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API error"))
			Expect(deleteRunnerGroupCalls).To(HaveLen(1))
		})
	})

	Context("when CreateRunnerGroupForOrg fails", func() {
		BeforeEach(func() {
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:       "test-group",
					Visibility: new("all"),
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{}
			createRunnerGroupError = errors.New("API error: failed to create runner group")
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API error"))
			Expect(createRunnerGroupCalls).To(HaveLen(1))
		})
	})

	Context("with runner group name change", func() {
		BeforeEach(func() {
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:       "new-name",
					Visibility: new("all"),
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{
				{
					ID:         new(int64(111)),
					Name:       new("old-name"),
					Visibility: new("all"),
				},
			}
		})

		It("should delete old and create new runner group", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deleteRunnerGroupCalls).To(HaveLen(1))
			Expect(deleteRunnerGroupCalls[0]).To(Equal(int64(111)))
			Expect(createRunnerGroupCalls).To(HaveLen(1))
			Expect(createRunnerGroupCalls[0].Name).To(Equal(new("new-name")))
		})
	})

	Context("with runner group with workflows restriction", func() {
		BeforeEach(func() {
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:                  "restricted-group",
					Visibility:            new("private"),
					RestrictedToWorkflows: new(true),
					SelectedWorkflows: []string{
						"org/repo/.github/workflows/deploy.yaml@refs/heads/main",
						"org/repo/.github/workflows/test.yaml@refs/tags/v1.0.0",
					},
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{}
		})

		It("should create runner group with workflow restrictions", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRunnerGroupCalls).To(HaveLen(1))
			Expect(createRunnerGroupCalls[0].Name).To(Equal(new("restricted-group")))
			Expect(createRunnerGroupCalls[0].RestrictedToWorkflows).To(Equal(new(true)))
			Expect(createRunnerGroupCalls[0].SelectedWorkflows).To(ConsistOf(
				"org/repo/.github/workflows/deploy.yaml@refs/heads/main",
				"org/repo/.github/workflows/test.yaml@refs/tags/v1.0.0",
			))
		})
	})

	Context("with runner group with selected repositories", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:                         "repo1",
						OrganizationRef:              v1alpha1.OrganizationRef{Name: "test-org"},
						AvailableActionsRunnerGroups: []string{"selected-group"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(100)),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo2",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:                         "repo2",
						OrganizationRef:              v1alpha1.OrganizationRef{Name: "test-org"},
						AvailableActionsRunnerGroups: []string{"selected-group"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(200)),
					},
				},
			}
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:       "selected-group",
					Visibility: new("selected"),
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{}
		})

		It("should create runner group with selected repositories", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRunnerGroupCalls).To(HaveLen(1))
			Expect(createRunnerGroupCalls[0].Name).To(Equal(new("selected-group")))
			Expect(createRunnerGroupCalls[0].Visibility).To(Equal(new("selected")))
			Expect(createRunnerGroupCalls[0].SelectedRepositoryIDs).To(ConsistOf(int64(100), int64(200)))
		})
	})

	Context("with runner group referencing repos from different orgs", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:                         "repo1",
						OrganizationRef:              v1alpha1.OrganizationRef{Name: "test-org"},
						AvailableActionsRunnerGroups: []string{"my-group"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(100)),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo2",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:                         "repo2",
						OrganizationRef:              v1alpha1.OrganizationRef{Name: "other-org"},
						AvailableActionsRunnerGroups: []string{"my-group"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(200)),
					},
				},
			}
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:       "my-group",
					Visibility: new("selected"),
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{}
		})

		It("should only include repos from the same org", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createRunnerGroupCalls).To(HaveLen(1))
			Expect(createRunnerGroupCalls[0].SelectedRepositoryIDs).To(ConsistOf(int64(100)))
		})
	})

	Context("with multiple updates needed", func() {
		BeforeEach(func() {
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:       "group-keep",
					Visibility: new("all"),
				},
				{
					Name:       "group-update",
					Visibility: new("selected"),
				},
				{
					Name:       "group-new",
					Visibility: new("private"),
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{
				{
					ID:         new(int64(111)),
					Name:       new("group-keep"),
					Visibility: new("all"),
				},
				{
					ID:         new(int64(222)),
					Name:       new("group-update"),
					Visibility: new("all"), // Different visibility
				},
				{
					ID:   new(int64(333)),
					Name: new("group-delete"),
				},
			}
		})

		It("should handle all changes correctly", func() {
			Expect(err).NotTo(HaveOccurred())
			// Should delete: group-delete only
			Expect(deleteRunnerGroupCalls).To(HaveLen(1))
			Expect(deleteRunnerGroupCalls[0]).To(Equal(int64(333)))
			// Should update: group-update
			Expect(updateRunnerGroupCalls).To(HaveLen(1))
			Expect(updateRunnerGroupCalls[0].GroupID).To(Equal(int64(222)))
			Expect(updateRunnerGroupCalls[0].UpdateRequest.Name).To(Equal(new("group-update")))
			Expect(updateRunnerGroupCalls[0].UpdateRequest.Visibility).To(Equal(new("selected")))
			// Should create: group-new
			Expect(createRunnerGroupCalls).To(HaveLen(1))
			Expect(createRunnerGroupCalls[0].Name).To(Equal(new("group-new")))
			// Repository IDs are equal (both empty) for group-update, so no SetSelectedRepositories call
			// group-keep has visibility='all' so it also skips repository operations
			Expect(setSelectedRepositoriesCalls).To(BeEmpty())
		})
	})

	Context("when updating runner group with repository selection changes", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:                         "repo1",
						OrganizationRef:              v1alpha1.OrganizationRef{Name: "test-org"},
						AvailableActionsRunnerGroups: []string{"test-group"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(100)),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo2",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:                         "repo2",
						OrganizationRef:              v1alpha1.OrganizationRef{Name: "test-org"},
						AvailableActionsRunnerGroups: []string{"test-group"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(200)),
					},
				},
			}
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:       "test-group",
					Visibility: new("selected"),
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{
				{
					ID:         new(int64(111)),
					Name:       new("test-group"),
					Visibility: new("selected"),
				},
			}
		})

		It("should update repository selection via SetSelectedRepositoriesForRunnerGroup", func() {
			Expect(err).NotTo(HaveOccurred())
			// Runner group properties are equal (both visibility='selected'), so no update call
			Expect(updateRunnerGroupCalls).To(BeEmpty())
			// But repositories differ (current=empty, desired=[100,200]), so set repositories
			Expect(setSelectedRepositoriesCalls).To(HaveLen(1))
			Expect(setSelectedRepositoriesCalls[0].GroupID).To(Equal(int64(111)))
			Expect(setSelectedRepositoriesCalls[0].RepositoryIDs).To(ConsistOf(int64(100), int64(200)))
		})
	})

	Context("when UpdateRunnerGroupForOrg fails", func() {
		BeforeEach(func() {
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:       "test-group",
					Visibility: new("selected"),
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{
				{
					ID:         new(int64(111)),
					Name:       new("test-group"),
					Visibility: new("all"),
				},
			}
			updateRunnerGroupError = errors.New("API error: failed to update runner group")
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API error"))
			Expect(updateRunnerGroupCalls).To(HaveLen(1))
		})
	})

	Context("when SetSelectedRepositoriesForRunnerGroup fails", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:                         "repo1",
						OrganizationRef:              v1alpha1.OrganizationRef{Name: "test-org"},
						AvailableActionsRunnerGroups: []string{"test-group"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(100)),
					},
				},
			}
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:       "test-group",
					Visibility: new("selected"),
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{
				{
					ID:         new(int64(111)),
					Name:       new("test-group"),
					Visibility: new("all"),
				},
			}
			setSelectedRepositoriesError = errors.New("API error: failed to set repositories")
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API error"))
			Expect(updateRunnerGroupCalls).To(HaveLen(1))
			Expect(setSelectedRepositoriesCalls).To(HaveLen(1))
		})
	})

	Context("when runner group only differs in selected workflows", func() {
		BeforeEach(func() {
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:                  "test-group",
					Visibility:            new("private"),
					RestrictedToWorkflows: new(true),
					SelectedWorkflows: []string{
						"org/repo/.github/workflows/ci.yaml@main",
						"org/repo/.github/workflows/deploy.yaml@main",
					},
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{
				{
					ID:                    new(int64(111)),
					Name:                  new("test-group"),
					Visibility:            new("private"),
					RestrictedToWorkflows: new(true),
					SelectedWorkflows: []string{
						"org/repo/.github/workflows/ci.yaml@main",
					},
				},
			}
		})

		It("should update the runner group with new workflows", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateRunnerGroupCalls).To(HaveLen(1))
			Expect(updateRunnerGroupCalls[0].UpdateRequest.SelectedWorkflows).To(ConsistOf(
				"org/repo/.github/workflows/ci.yaml@main",
				"org/repo/.github/workflows/deploy.yaml@main",
			))
		})
	})

	Context("when runner group changes from restricted to unrestricted workflows", func() {
		BeforeEach(func() {
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:                  "test-group",
					Visibility:            new("private"),
					RestrictedToWorkflows: new(false),
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{
				{
					ID:                    new(int64(111)),
					Name:                  new("test-group"),
					Visibility:            new("private"),
					RestrictedToWorkflows: new(true),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/ci.yaml@main"},
				},
			}
		})

		It("should update the runner group", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateRunnerGroupCalls).To(HaveLen(1))
			Expect(updateRunnerGroupCalls[0].UpdateRequest.RestrictedToWorkflows).To(Equal(new(false)))
		})
	})

	Context("when runner group changes visibility from all to selected", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:                         "repo1",
						OrganizationRef:              v1alpha1.OrganizationRef{Name: "test-org"},
						AvailableActionsRunnerGroups: []string{"test-group"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(100)),
					},
				},
			}
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:       "test-group",
					Visibility: new("selected"),
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{
				{
					ID:         new(int64(111)),
					Name:       new("test-group"),
					Visibility: new("all"),
				},
			}
		})

		It("should update visibility and set selected repositories", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateRunnerGroupCalls).To(HaveLen(1))
			Expect(updateRunnerGroupCalls[0].UpdateRequest.Visibility).To(Equal(new("selected")))
			Expect(setSelectedRepositoriesCalls).To(HaveLen(1))
			Expect(setSelectedRepositoriesCalls[0].RepositoryIDs).To(ConsistOf(int64(100)))
		})
	})

	Context("when runner group changes visibility from selected to all", func() {
		BeforeEach(func() {
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:       "test-group",
					Visibility: new("all"),
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{
				{
					ID:         new(int64(111)),
					Name:       new("test-group"),
					Visibility: new("selected"),
				},
			}
		})

		It("should update visibility but not set repositories", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateRunnerGroupCalls).To(HaveLen(1))
			Expect(updateRunnerGroupCalls[0].UpdateRequest.Visibility).To(Equal(new("all")))
			// When visibility is 'all', SetSelectedRepositories should not be called
			Expect(setSelectedRepositoriesCalls).To(BeEmpty())
		})
	})

	Context("when runner group with visibility=selected has matching repositories", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:                         "repo1",
						OrganizationRef:              v1alpha1.OrganizationRef{Name: "test-org"},
						AvailableActionsRunnerGroups: []string{"test-group"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(100)),
					},
				},
			}
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:       "test-group",
					Visibility: new("selected"),
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{
				{
					ID:         new(int64(111)),
					Name:       new("test-group"),
					Visibility: new("selected"),
				},
			}
			// Mock GetSelectedRepositories to return the same repo that's desired
			mockClient.GetSelectedRepositoriesForRunnerGroupFunc = func(ctx context.Context, org string, groupID int64) ([]*github.Repository, error) {
				return []*github.Repository{
					{ID: new(int64(100))},
				}, nil
			}
		})

		It("should not call SetSelectedRepositories when IDs match", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateRunnerGroupCalls).To(BeEmpty())
			// Repository IDs match (current=[100], desired=[100]), so no set call
			Expect(setSelectedRepositoriesCalls).To(BeEmpty())
		})
	})

	Context("when runner group with visibility=selected has differing repositories", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:                         "repo1",
						OrganizationRef:              v1alpha1.OrganizationRef{Name: "test-org"},
						AvailableActionsRunnerGroups: []string{"test-group"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(100)),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo2",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:                         "repo2",
						OrganizationRef:              v1alpha1.OrganizationRef{Name: "test-org"},
						AvailableActionsRunnerGroups: []string{"test-group"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(200)),
					},
				},
			}
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:       "test-group",
					Visibility: new("selected"),
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{
				{
					ID:         new(int64(111)),
					Name:       new("test-group"),
					Visibility: new("selected"),
				},
			}
			// Mock GetSelectedRepositories to return different repos
			mockClient.GetSelectedRepositoriesForRunnerGroupFunc = func(ctx context.Context, org string, groupID int64) ([]*github.Repository, error) {
				return []*github.Repository{
					{ID: new(int64(300))}, // Different repo
				}, nil
			}
		})

		It("should call SetSelectedRepositories when IDs differ", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateRunnerGroupCalls).To(BeEmpty())
			// Repository IDs differ (current=[300], desired=[100,200]), so set should be called
			Expect(setSelectedRepositoriesCalls).To(HaveLen(1))
			Expect(setSelectedRepositoriesCalls[0].GroupID).To(Equal(int64(111)))
			Expect(setSelectedRepositoriesCalls[0].RepositoryIDs).To(ConsistOf(int64(100), int64(200)))
		})
	})

	Context("when runner group with visibility=private does not call SetSelectedRepositories", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:                         "repo1",
						OrganizationRef:              v1alpha1.OrganizationRef{Name: "test-org"},
						AvailableActionsRunnerGroups: []string{"test-group"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(100)),
					},
				},
			}
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:       "test-group",
					Visibility: new("private"),
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{
				{
					ID:         new(int64(111)),
					Name:       new("test-group"),
					Visibility: new("private"),
				},
			}
		})

		It("should not call SetSelectedRepositories for visibility=private", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateRunnerGroupCalls).To(BeEmpty())
			// Visibility is 'private', so no repository selection operations
			Expect(setSelectedRepositoriesCalls).To(BeEmpty())
		})
	})

	Context("when GetSelectedRepositoriesForRunnerGroup fails", func() {
		BeforeEach(func() {
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:       "test-group",
					Visibility: new("selected"),
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{
				{
					ID:         new(int64(111)),
					Name:       new("test-group"),
					Visibility: new("selected"),
				},
			}
			mockClient.GetSelectedRepositoriesForRunnerGroupFunc = func(ctx context.Context, org string, groupID int64) ([]*github.Repository, error) {
				return nil, errors.New("API error: failed to get selected repositories")
			}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API error"))
		})
	})

	Context("when runner group changes from no selected repos to some selected repos", func() {
		BeforeEach(func() {
			repos = []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "repo1",
						Namespace: "default",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:                         "repo1",
						OrganizationRef:              v1alpha1.OrganizationRef{Name: "test-org"},
						AvailableActionsRunnerGroups: []string{"test-group"},
					},
					Status: v1alpha1.RepositoryStatus{
						ID: new(int64(100)),
					},
				},
			}
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:       "test-group",
					Visibility: new("selected"),
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{
				{
					ID:         new(int64(111)),
					Name:       new("test-group"),
					Visibility: new("selected"),
				},
			}
			// Current has no repositories
			mockClient.GetSelectedRepositoriesForRunnerGroupFunc = func(ctx context.Context, org string, groupID int64) ([]*github.Repository, error) {
				return []*github.Repository{}, nil
			}
		})

		It("should call SetSelectedRepositories to add repositories", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateRunnerGroupCalls).To(BeEmpty())
			// Repository IDs differ (current=[], desired=[100])
			Expect(setSelectedRepositoriesCalls).To(HaveLen(1))
			Expect(setSelectedRepositoriesCalls[0].GroupID).To(Equal(int64(111)))
			Expect(setSelectedRepositoriesCalls[0].RepositoryIDs).To(ConsistOf(int64(100)))
		})
	})

	Context("when runner group changes from some selected repos to no selected repos", func() {
		BeforeEach(func() {
			// No repos reference this runner group
			repos = []*v1alpha1.Repository{}
			actionsSettings.RunnerGroups = []v1alpha1.RunnerGroup{
				{
					Name:       "test-group",
					Visibility: new("selected"),
				},
			}
			currentRunnerGroups = []*github.RunnerGroup{
				{
					ID:         new(int64(111)),
					Name:       new("test-group"),
					Visibility: new("selected"),
				},
			}
			// Current has a repository
			mockClient.GetSelectedRepositoriesForRunnerGroupFunc = func(ctx context.Context, org string, groupID int64) ([]*github.Repository, error) {
				return []*github.Repository{
					{ID: new(int64(100))},
				}, nil
			}
		})

		It("should call SetSelectedRepositories to clear repositories", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updateRunnerGroupCalls).To(BeEmpty())
			// Repository IDs differ (current=[100], desired=[])
			Expect(setSelectedRepositoriesCalls).To(HaveLen(1))
			Expect(setSelectedRepositoriesCalls[0].GroupID).To(Equal(int64(111)))
			Expect(setSelectedRepositoriesCalls[0].RepositoryIDs).To(BeEmpty())
		})
	})
})

var _ = Describe("ReconcilePermissions with selected repositories", func() {
	var (
		ctx             context.Context
		mockClient      *ghclientmock.MockGitHubClientWrapper
		k8sClient       client.Client
		rec             *GitHubOrgReconciler
		scheme          *runtime.Scheme
		org             *v1alpha1.Organization
		repos           []*v1alpha1.Repository
		actionsSettings v1alpha1.ActionsSettings
		err             error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		mockClient.GetActionsPermissionsForOrgFunc = func(ctx context.Context, org string) (*github.ActionsPermissions, error) {
			return &github.ActionsPermissions{
				EnabledRepositories: new("selected"),
				AllowedActions:      new("selected"),
				SHAPinningRequired:  new(false),
			}, nil
		}

		mockClient.GetActionsEnabledRepositoriesForOrgFunc = func(ctx context.Context, org string) ([]*github.Repository, error) {
			return []*github.Repository{}, nil
		}
	})

	JustBeforeEach(func() {
		org = &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-org",
				Namespace: "default",
			},
			Spec: v1alpha1.OrganizationSpec{
				Name:                    "test-org",
				GitHubAppInstallationId: 12345,
				ActionsSettings:         actionsSettings,
			},
		}

		objects := make([]client.Object, 1, 1+len(repos))
		objects[0] = org
		for _, repo := range repos {
			objects = append(objects, repo)
		}

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(objects...).
			WithStatusSubresource(objects...).
			WithIndex(&v1alpha1.Repository{}, "spec.organizationRef.name", func(obj client.Object) []string {
				repo := obj.(*v1alpha1.Repository)
				return []string{repo.Spec.OrganizationRef.Name}
			}).
			Build()

		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				Client:   mockClient,
				Resource: "test-org",
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Client:   k8sClient,
				Resource: org,
			},
		}

		err = rec.reconcilePermissions(ctx)
	})

	Context("when enabled repositories is 'selected'", func() {
		BeforeEach(func() {
			actionsSettings = v1alpha1.ActionsSettings{
				EnabledRepositories: new("selected"),
			}
		})

		It("should call reconcileActionsEnabledRepositories", func() {
			Expect(err).NotTo(HaveOccurred())

			calls := mockClient.GetActionsCalls()
			found := false
			for _, call := range calls {
				if call.Method == "GetActionsEnabledRepositoriesForOrg" {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "GetActionsEnabledRepositoriesForOrg should have been called")
		})
	})

	Context("when enabled repositories is not 'selected'", func() {
		BeforeEach(func() {
			actionsSettings = v1alpha1.ActionsSettings{
				EnabledRepositories: new("all"),
			}
		})

		It("should not call reconcileActionsEnabledRepositories", func() {
			Expect(err).NotTo(HaveOccurred())

			calls := mockClient.GetActionsCalls()
			for _, call := range calls {
				Expect(call.Method).NotTo(Equal("GetActionsEnabledRepositoriesForOrg"), "GetActionsEnabledRepositoriesForOrg should not have been called")
			}
		})
	})
})
