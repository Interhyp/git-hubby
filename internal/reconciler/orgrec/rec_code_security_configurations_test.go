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

const (
	testTeam         = "test-team"
	testRepo         = "test-repo"
	testOrg          = "test-org"
	testCsc          = "test-csc"
	testRole         = "test-role"
	defaultNamespace = "default"
)

var _ = Describe("ReconcileCodeSecurityConfigurations", func() {
	var (
		ctx        context.Context
		mockClient *ghclientmock.MockGitHubClientWrapper
		k8sClient  client.Client
		rec        *GitHubOrgReconciler
		scheme     *runtime.Scheme
		org        *v1alpha1.Organization
		csc        *v1alpha1.CodeSecurityConfiguration
		repo       *v1alpha1.Repository
		err        error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		org = &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testOrg,
				Namespace: defaultNamespace,
			},
			Spec: v1alpha1.OrganizationSpec{
				Name:                    testOrg,
				GitHubAppInstallationId: 12345,
			},
		}

		csc = &v1alpha1.CodeSecurityConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testCsc,
				Namespace: defaultNamespace,
			},
			Spec: v1alpha1.CodeSecurityConfigurationSpec{
				Name:        testCsc,
				Description: "Test code security configuration",
			},
		}

		repo = &v1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testRepo,
				Namespace: defaultNamespace,
			},
			Spec: v1alpha1.RepositorySpec{
				Name: testRepo,
				OrganizationRef: v1alpha1.OrganizationRef{
					Name: testOrg,
				},
			},
			Status: v1alpha1.RepositoryStatus{
				ID: new(int64(12345)),
			},
		}
	})

	JustBeforeEach(func() {
		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(org, csc, repo).
			WithStatusSubresource(org, csc, repo).
			WithIndex(&v1alpha1.Repository{}, "spec.attachedCodeSecurityConfiguration.name", func(obj client.Object) []string {
				r := obj.(*v1alpha1.Repository)
				if r.Spec.AttachedCodeSecurityConfiguration == nil {
					return nil
				}
				return []string{r.Spec.AttachedCodeSecurityConfiguration.Name}
			}).
			Build()

		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				Client:   mockClient,
				Resource: testOrg,
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Client:   k8sClient,
				Resource: org,
			},
		}
	})

	Context("when no code security configurations are referenced", func() {
		BeforeEach(func() {
			mockClient.GetDefaultCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
				return []*github.CodeSecurityConfigurationWithDefaultForNewRepos{}, nil
			}
			mockClient.GetCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfiguration, error) {
				return []*github.CodeSecurityConfiguration{}, nil
			}
		})

		JustBeforeEach(func() {
			err = rec.reconcileCodeSecurityConfigurations(ctx)
		})

		It("should reconcile successfully with no changes", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when creating a new code security configuration", func() {
		var createdConfig *github.CodeSecurityConfiguration

		BeforeEach(func() {
			org.Spec.CodeSecurityConfigurations = []v1alpha1.AttachableCodeSecurityConfigurationRef{
				{Name: testCsc},
			}

			mockClient.GetDefaultCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
				return []*github.CodeSecurityConfigurationWithDefaultForNewRepos{}, nil
			}
			mockClient.GetCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfiguration, error) {
				return []*github.CodeSecurityConfiguration{}, nil
			}
			mockClient.CreateCodeSecurityConfigurationForOrgFunc = func(ctx context.Context, org string, config github.CodeSecurityConfiguration) (*github.CodeSecurityConfiguration, error) {
				createdConfig = &config
				createdConfig.ID = new(int64(999))
				return createdConfig, nil
			}
		})

		JustBeforeEach(func() {
			err = rec.reconcileCodeSecurityConfigurations(ctx)
		})

		It("should create the configuration successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(createdConfig).NotTo(BeNil())
			Expect(createdConfig.Name).To(Equal(testCsc))
			Expect(createdConfig.Description).To(Equal("Test code security configuration"))
		})
	})

	Context("when creating a code security configuration with default for new repos", func() {
		var setAsDefaultCalled bool
		var defaultScope string

		BeforeEach(func() {
			csc.Spec.DefaultForNewRepos = new("all")
			org.Spec.CodeSecurityConfigurations = []v1alpha1.AttachableCodeSecurityConfigurationRef{
				{Name: testCsc},
			}

			mockClient.GetDefaultCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
				return []*github.CodeSecurityConfigurationWithDefaultForNewRepos{}, nil
			}
			mockClient.GetCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfiguration, error) {
				return []*github.CodeSecurityConfiguration{}, nil
			}
			mockClient.CreateCodeSecurityConfigurationForOrgFunc = func(ctx context.Context, org string, config github.CodeSecurityConfiguration) (*github.CodeSecurityConfiguration, error) {
				return &github.CodeSecurityConfiguration{
					ID:          new(int64(999)),
					Name:        config.Name,
					Description: config.Description,
				}, nil
			}
			mockClient.SetCodeSecurityConfigurationAsDefaultForOrgFunc = func(ctx context.Context, org string, configId int64, newReposParam string) error {
				setAsDefaultCalled = true
				defaultScope = newReposParam
				return nil
			}
		})

		JustBeforeEach(func() {
			err = rec.reconcileCodeSecurityConfigurations(ctx)
		})

		It("should create and set as default", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setAsDefaultCalled).To(BeTrue())
			Expect(defaultScope).To(Equal("all"))
		})
	})

	Context("when updating an existing code security configuration", func() {
		var updatedConfig *github.CodeSecurityConfiguration

		BeforeEach(func() {
			csc.Spec.Description = "Updated description"
			org.Spec.CodeSecurityConfigurations = []v1alpha1.AttachableCodeSecurityConfigurationRef{
				{Name: testCsc},
			}

			mockClient.GetDefaultCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
				return []*github.CodeSecurityConfigurationWithDefaultForNewRepos{}, nil
			}
			mockClient.GetCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfiguration, error) {
				return []*github.CodeSecurityConfiguration{
					{
						ID:          new(int64(999)),
						Name:        testCsc,
						Description: "Old description",
						TargetType:  new(targetTypeOrganization),
					},
				}, nil
			}
			mockClient.UpdateCodeSecurityConfigurationForOrgFunc = func(ctx context.Context, org string, configId int64, config github.CodeSecurityConfiguration) (*github.CodeSecurityConfiguration, error) {
				updatedConfig = &config
				updatedConfig.ID = new(configId)
				return updatedConfig, nil
			}
		})

		JustBeforeEach(func() {
			err = rec.reconcileCodeSecurityConfigurations(ctx)
		})

		It("should update the configuration successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedConfig).NotTo(BeNil())
			Expect(updatedConfig.Description).To(Equal("Updated description"))
		})
	})

	Context("when no changes are needed", func() {
		BeforeEach(func() {
			org.Spec.CodeSecurityConfigurations = []v1alpha1.AttachableCodeSecurityConfigurationRef{
				{Name: testCsc},
			}

			mockClient.GetDefaultCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
				return []*github.CodeSecurityConfigurationWithDefaultForNewRepos{}, nil
			}
			mockClient.GetCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfiguration, error) {
				return []*github.CodeSecurityConfiguration{
					{
						ID:          new(int64(999)),
						Name:        testCsc,
						Description: "Test code security configuration",
						TargetType:  new(targetTypeOrganization),
					},
				}, nil
			}
		})

		JustBeforeEach(func() {
			err = rec.reconcileCodeSecurityConfigurations(ctx)
		})

		It("should reconcile successfully without updates", func() {
			Expect(err).NotTo(HaveOccurred())
			calls := mockClient.GetCodeSecurityConfigurationCalls()
			var updateCalled bool
			for _, call := range calls {
				if call.Method == "UpdateCodeSecurityConfigurationForOrg" {
					updateCalled = true
				}
			}
			Expect(updateCalled).To(BeFalse())
		})
	})

	Context("when deleting an orphaned code security configuration", func() {
		var deletedID int64

		BeforeEach(func() {
			// No CSCs referenced in org
			org.Spec.CodeSecurityConfigurations = []v1alpha1.AttachableCodeSecurityConfigurationRef{}

			mockClient.GetDefaultCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
				return []*github.CodeSecurityConfigurationWithDefaultForNewRepos{}, nil
			}
			mockClient.GetCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfiguration, error) {
				return []*github.CodeSecurityConfiguration{
					{
						ID:         new(int64(999)),
						Name:       "orphaned-csc",
						TargetType: new(targetTypeOrganization),
					},
				}, nil
			}
			mockClient.DeleteCodeSecurityConfigurationForOrgFunc = func(ctx context.Context, org string, configId int64) error {
				deletedID = configId
				return nil
			}
		})

		JustBeforeEach(func() {
			err = rec.reconcileCodeSecurityConfigurations(ctx)
		})

		It("should delete the orphaned configuration", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(deletedID).To(Equal(int64(999)))
		})
	})

	Context("when GitHub API returns error on get", func() {
		BeforeEach(func() {
			mockClient.GetDefaultCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
				return nil, errors.New("GitHub API error on get defaults")
			}
		})

		JustBeforeEach(func() {
			err = rec.reconcileCodeSecurityConfigurations(ctx)
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GitHub API error on get defaults"))
		})
	})

	Context("when code security configuration not found in Kubernetes", func() {
		BeforeEach(func() {
			org.Spec.CodeSecurityConfigurations = []v1alpha1.AttachableCodeSecurityConfigurationRef{
				{Name: "non-existent-csc"},
			}

			mockClient.GetDefaultCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
				return []*github.CodeSecurityConfigurationWithDefaultForNewRepos{}, nil
			}
			mockClient.GetCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfiguration, error) {
				return []*github.CodeSecurityConfiguration{}, nil
			}
		})

		JustBeforeEach(func() {
			err = rec.reconcileCodeSecurityConfigurations(ctx)
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when setting default for new repos with scope selected", func() {
		var setAsDefaultCalled bool
		var defaultScope string

		BeforeEach(func() {
			csc.Spec.DefaultForNewRepos = new("selected")
			org.Spec.CodeSecurityConfigurations = []v1alpha1.AttachableCodeSecurityConfigurationRef{
				{Name: testCsc, AttachmentScope: new("selected")},
			}

			mockClient.GetDefaultCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
				return []*github.CodeSecurityConfigurationWithDefaultForNewRepos{}, nil
			}
			mockClient.GetCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfiguration, error) {
				return []*github.CodeSecurityConfiguration{}, nil
			}
			mockClient.CreateCodeSecurityConfigurationForOrgFunc = func(ctx context.Context, org string, config github.CodeSecurityConfiguration) (*github.CodeSecurityConfiguration, error) {
				return &github.CodeSecurityConfiguration{
					ID:          new(int64(999)),
					Name:        config.Name,
					Description: config.Description,
				}, nil
			}
			mockClient.SetCodeSecurityConfigurationAsDefaultForOrgFunc = func(ctx context.Context, org string, configId int64, newReposParam string) error {
				setAsDefaultCalled = true
				defaultScope = newReposParam
				return nil
			}
			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				return []*github.RepositoryAttachment{}, nil
			}
			mockClient.AttachCodeSecurityConfigurationsFunc = func(ctx context.Context, org string, cscID int64, scope string, repoIDs []int64) error {
				return nil
			}
		})

		JustBeforeEach(func() {
			err = rec.reconcileCodeSecurityConfigurations(ctx)
		})

		It("should create and set as default for selected repos", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(setAsDefaultCalled).To(BeTrue())
			Expect(defaultScope).To(Equal("selected"))
		})
	})

	Context("when SetCodeSecurityConfigurationAsDefaultForOrg fails during creation", func() {
		BeforeEach(func() {
			csc.Spec.DefaultForNewRepos = new("all")
			org.Spec.CodeSecurityConfigurations = []v1alpha1.AttachableCodeSecurityConfigurationRef{
				{Name: testCsc},
			}

			mockClient.GetDefaultCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
				return []*github.CodeSecurityConfigurationWithDefaultForNewRepos{}, nil
			}
			mockClient.GetCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfiguration, error) {
				return []*github.CodeSecurityConfiguration{}, nil
			}
			mockClient.CreateCodeSecurityConfigurationForOrgFunc = func(ctx context.Context, org string, config github.CodeSecurityConfiguration) (*github.CodeSecurityConfiguration, error) {
				return &github.CodeSecurityConfiguration{
					ID:          new(int64(999)),
					Name:        config.Name,
					Description: config.Description,
				}, nil
			}
			mockClient.SetCodeSecurityConfigurationAsDefaultForOrgFunc = func(ctx context.Context, org string, configId int64, newReposParam string) error {
				return errors.New("failed to set as default")
			}
		})

		JustBeforeEach(func() {
			err = rec.reconcileCodeSecurityConfigurations(ctx)
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to set as default"))
		})
	})

	Context("when UpdateCodeSecurityConfigurationForOrg fails", func() {
		BeforeEach(func() {
			csc.Spec.Description = "Updated description"
			org.Spec.CodeSecurityConfigurations = []v1alpha1.AttachableCodeSecurityConfigurationRef{
				{Name: testCsc},
			}

			mockClient.GetDefaultCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
				return []*github.CodeSecurityConfigurationWithDefaultForNewRepos{}, nil
			}
			mockClient.GetCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfiguration, error) {
				return []*github.CodeSecurityConfiguration{
					{
						ID:          new(int64(999)),
						Name:        testCsc,
						Description: "Old description",
						TargetType:  new(targetTypeOrganization),
					},
				}, nil
			}
			mockClient.UpdateCodeSecurityConfigurationForOrgFunc = func(ctx context.Context, org string, configId int64, config github.CodeSecurityConfiguration) (*github.CodeSecurityConfiguration, error) {
				return nil, errors.New("update failed")
			}
		})

		JustBeforeEach(func() {
			err = rec.reconcileCodeSecurityConfigurations(ctx)
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("update failed"))
		})
	})

	Context("when DeleteCodeSecurityConfigurationForOrg fails", func() {
		BeforeEach(func() {
			org.Spec.CodeSecurityConfigurations = []v1alpha1.AttachableCodeSecurityConfigurationRef{}

			mockClient.GetDefaultCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
				return []*github.CodeSecurityConfigurationWithDefaultForNewRepos{}, nil
			}
			mockClient.GetCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfiguration, error) {
				return []*github.CodeSecurityConfiguration{
					{
						ID:         new(int64(999)),
						Name:       "orphaned-csc",
						TargetType: new(targetTypeOrganization),
					},
				}, nil
			}
			mockClient.DeleteCodeSecurityConfigurationForOrgFunc = func(ctx context.Context, org string, configId int64) error {
				return errors.New("delete failed")
			}
		})

		JustBeforeEach(func() {
			err = rec.reconcileCodeSecurityConfigurations(ctx)
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("delete failed"))
		})
	})

	Context("when reconciling multiple code security configurations", func() {
		var csc2 *v1alpha1.CodeSecurityConfiguration

		BeforeEach(func() {
			csc2 = &v1alpha1.CodeSecurityConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-csc-2",
					Namespace: defaultNamespace,
				},
				Spec: v1alpha1.CodeSecurityConfigurationSpec{
					Name:        "test-csc-2",
					Description: "Second test code security configuration",
				},
			}

			org.Spec.CodeSecurityConfigurations = []v1alpha1.AttachableCodeSecurityConfigurationRef{
				{Name: testCsc},
				{Name: "test-csc-2"},
			}

			mockClient.GetDefaultCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
				return []*github.CodeSecurityConfigurationWithDefaultForNewRepos{}, nil
			}
			mockClient.GetCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfiguration, error) {
				return []*github.CodeSecurityConfiguration{
					{
						ID:          new(int64(999)),
						Name:        testCsc,
						Description: "Test code security configuration",
						TargetType:  new(targetTypeOrganization),
					},
				}, nil
			}
			mockClient.CreateCodeSecurityConfigurationForOrgFunc = func(ctx context.Context, org string, config github.CodeSecurityConfiguration) (*github.CodeSecurityConfiguration, error) {
				return &github.CodeSecurityConfiguration{
					ID:          new(int64(1000)),
					Name:        config.Name,
					Description: config.Description,
				}, nil
			}
		})

		JustBeforeEach(func() {
			// Add csc2 to k8s client
			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(org, csc, csc2, repo).
				WithStatusSubresource(org, csc, csc2, repo).
				WithIndex(&v1alpha1.Repository{}, "spec.attachedCodeSecurityConfiguration.name", func(obj client.Object) []string {
					r := obj.(*v1alpha1.Repository)
					if r.Spec.AttachedCodeSecurityConfiguration == nil {
						return nil
					}
					return []string{r.Spec.AttachedCodeSecurityConfiguration.Name}
				}).
				Build()

			rec.Kubernetes.Client = k8sClient
			err = rec.reconcileCodeSecurityConfigurations(ctx)
		})

		It("should reconcile both configurations successfully", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when configuration with bypass reviewers needs reconciliation", func() {
		BeforeEach(func() {
			teamName := "security-team"
			csc.Spec.SecretScanningDelegatedBypassOptions = &v1alpha1.SecretScanningDelegatedBypassOptions{
				Reviewers: []*v1alpha1.BypassReviewer{
					{
						ReviewerType: "TEAM",
						ReviewerName: &teamName,
					},
				},
			}
			org.Spec.CodeSecurityConfigurations = []v1alpha1.AttachableCodeSecurityConfigurationRef{
				{Name: testCsc},
			}

			mockClient.GetDefaultCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
				return []*github.CodeSecurityConfigurationWithDefaultForNewRepos{}, nil
			}
			mockClient.GetCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfiguration, error) {
				return []*github.CodeSecurityConfiguration{}, nil
			}
			mockClient.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					ID:   new(int64(12345)),
					Slug: new("security-team"),
				}, nil
			}
			mockClient.CreateCodeSecurityConfigurationForOrgFunc = func(ctx context.Context, org string, config github.CodeSecurityConfiguration) (*github.CodeSecurityConfiguration, error) {
				return &github.CodeSecurityConfiguration{
					ID:          new(int64(999)),
					Name:        config.Name,
					Description: config.Description,
				}, nil
			}
		})

		JustBeforeEach(func() {
			err = rec.reconcileCodeSecurityConfigurations(ctx)
		})

		It("should resolve bypass reviewers and create configuration", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var _ = Describe("UnsetObsoleteDefaults", func() {
	var (
		ctx        context.Context
		mockClient *ghclientmock.MockGitHubClientWrapper
		k8sClient  client.Client
		rec        *GitHubOrgReconciler
		scheme     *runtime.Scheme
		org        *v1alpha1.Organization
		csc        *v1alpha1.CodeSecurityConfiguration
		err        error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		org = &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testOrg,
				Namespace: defaultNamespace,
			},
			Spec: v1alpha1.OrganizationSpec{
				Name:                    testOrg,
				GitHubAppInstallationId: 12345,
			},
		}

		csc = &v1alpha1.CodeSecurityConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testCsc,
				Namespace: defaultNamespace,
			},
			Spec: v1alpha1.CodeSecurityConfigurationSpec{
				Name:        testCsc,
				Description: "Test code security configuration",
			},
		}
	})

	JustBeforeEach(func() {
		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(org, csc).
			WithStatusSubresource(org, csc).
			Build()

		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				Client:   mockClient,
				Resource: testOrg,
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Client:   k8sClient,
				Resource: org,
			},
		}

		err = rec.unsetObsoleteDefaults(ctx)
	})

	Context("when a default config is no longer marked as default in k8s", func() {
		var unsetConfigID int64

		BeforeEach(func() {
			// CSC is not marked as default (or is set to "none")
			csc.Spec.DefaultForNewRepos = new("none")
			org.Spec.CodeSecurityConfigurations = []v1alpha1.AttachableCodeSecurityConfigurationRef{
				{Name: testCsc},
			}

			mockClient.GetDefaultCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
				return []*github.CodeSecurityConfigurationWithDefaultForNewRepos{
					{
						Configuration: &github.CodeSecurityConfiguration{
							ID:         new(int64(999)),
							Name:       testCsc,
							TargetType: new(targetTypeOrganization),
						},
					},
				}, nil
			}
			mockClient.SetCodeSecurityConfigurationAsDefaultForOrgFunc = func(ctx context.Context, org string, configId int64, newReposParam string) error {
				unsetConfigID = configId
				return nil
			}
		})

		It("should unset the default", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(unsetConfigID).To(Equal(int64(999)))
		})
	})

	Context("when a default config is still marked as default in k8s", func() {
		BeforeEach(func() {
			csc.Spec.DefaultForNewRepos = new("all")
			org.Spec.CodeSecurityConfigurations = []v1alpha1.AttachableCodeSecurityConfigurationRef{
				{Name: testCsc},
			}

			mockClient.GetDefaultCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
				return []*github.CodeSecurityConfigurationWithDefaultForNewRepos{
					{
						Configuration: &github.CodeSecurityConfiguration{
							ID:         new(int64(999)),
							Name:       testCsc,
							TargetType: new(targetTypeOrganization),
						},
					},
				}, nil
			}
		})

		It("should not unset the default", func() {
			Expect(err).NotTo(HaveOccurred())
			calls := mockClient.GetCodeSecurityConfigurationCalls()
			var setDefaultCalled bool
			for _, call := range calls {
				if call.Method == "SetCodeSecurityConfigurationAsDefaultForOrg" {
					setDefaultCalled = true
				}
			}
			Expect(setDefaultCalled).To(BeFalse())
		})
	})

	Context("when no defaults exist in GitHub", func() {
		BeforeEach(func() {
			mockClient.GetDefaultCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
				return []*github.CodeSecurityConfigurationWithDefaultForNewRepos{}, nil
			}
		})

		It("should reconcile successfully", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when GitHub API returns error", func() {
		BeforeEach(func() {
			mockClient.GetDefaultCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
				return nil, errors.New("API error")
			}
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API error"))
		})
	})
})

var _ = Describe("ResolveBypassReviewerNames", func() {
	var (
		ctx        context.Context
		mockClient *ghclientmock.MockGitHubClientWrapper
		k8sClient  client.Client
		rec        *GitHubOrgReconciler
		scheme     *runtime.Scheme
		org        *v1alpha1.Organization
		csc        *v1alpha1.CodeSecurityConfiguration
		result     *v1alpha1.CodeSecurityConfiguration
		err        error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		org = &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testOrg,
				Namespace: defaultNamespace,
			},
			Spec: v1alpha1.OrganizationSpec{
				Name:                    testOrg,
				GitHubAppInstallationId: 12345,
			},
		}

		csc = &v1alpha1.CodeSecurityConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testCsc,
				Namespace: defaultNamespace,
			},
			Spec: v1alpha1.CodeSecurityConfigurationSpec{
				Name:        testCsc,
				Description: "Test code security configuration",
			},
		}
	})

	JustBeforeEach(func() {
		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(org, csc).
			WithStatusSubresource(org, csc).
			Build()

		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				Client:   mockClient,
				Resource: testOrg,
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Client:   k8sClient,
				Resource: org,
			},
		}

		result, err = rec.resolveBypassReviewerNames(ctx, csc)
	})

	Context("when resolving mixed TEAM and ROLE bypass reviewers", func() {
		BeforeEach(func() {
			teamName := testTeam
			roleName := testRole
			csc.Spec.SecretScanningDelegatedBypassOptions = &v1alpha1.SecretScanningDelegatedBypassOptions{
				Reviewers: []*v1alpha1.BypassReviewer{
					{
						ReviewerType: "TEAM",
						ReviewerName: &teamName,
					},
					{
						ReviewerType: "ROLE",
						ReviewerName: &roleName,
					},
				},
			}

			mockClient.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					ID:   new(int64(12345)),
					Slug: new(testTeam),
				}, nil
			}
			mockClient.GetRoleByNameFunc = func(ctx context.Context, org string, roleName string) (*github.CustomOrgRole, error) {
				return &github.CustomOrgRole{
					ID:   new(int64(67890)),
					Name: new(testRole),
				}, nil
			}
		})

		It("should resolve both team and role to IDs", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.SecretScanningDelegatedBypassOptions).NotTo(BeNil())
			Expect(result.Spec.SecretScanningDelegatedBypassOptions.Reviewers).To(HaveLen(2))
			Expect(*result.Spec.SecretScanningDelegatedBypassOptions.Reviewers[0].ReviewerId).To(Equal(int64(12345)))
			Expect(*result.Spec.SecretScanningDelegatedBypassOptions.Reviewers[1].ReviewerId).To(Equal(int64(67890)))
		})
	})

	Context("when no bypass options are configured", func() {
		BeforeEach(func() {
			csc.Spec.SecretScanningDelegatedBypassOptions = nil
		})

		It("should return successfully without changes", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.SecretScanningDelegatedBypassOptions).To(BeNil())
		})
	})

	Context("when resolving mixed TEAM and ROLE bypass reviewers", func() {
		BeforeEach(func() {
			teamName := testTeam
			roleName := testRole
			csc.Spec.SecretScanningDelegatedBypassOptions = &v1alpha1.SecretScanningDelegatedBypassOptions{
				Reviewers: []*v1alpha1.BypassReviewer{
					{
						ReviewerType: "TEAM",
						ReviewerName: &teamName,
					},
					{
						ReviewerType: "ROLE",
						ReviewerName: &roleName,
					},
				},
			}

			mockClient.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return &github.Team{
					ID:   new(int64(12345)),
					Slug: new(testTeam),
				}, nil
			}
			mockClient.GetRoleByNameFunc = func(ctx context.Context, org string, roleName string) (*github.CustomOrgRole, error) {
				return &github.CustomOrgRole{
					ID:   new(int64(67890)),
					Name: new(testRole),
				}, nil
			}
		})

		It("should resolve both team and role to IDs", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.SecretScanningDelegatedBypassOptions).NotTo(BeNil())
			Expect(result.Spec.SecretScanningDelegatedBypassOptions.Reviewers).To(HaveLen(2))
			Expect(*result.Spec.SecretScanningDelegatedBypassOptions.Reviewers[0].ReviewerId).To(Equal(int64(12345)))
			Expect(*result.Spec.SecretScanningDelegatedBypassOptions.Reviewers[1].ReviewerId).To(Equal(int64(67890)))
		})
	})

	Context("when GetTeamBySlug returns error", func() {
		BeforeEach(func() {
			teamName := testTeam
			csc.Spec.SecretScanningDelegatedBypassOptions = &v1alpha1.SecretScanningDelegatedBypassOptions{
				Reviewers: []*v1alpha1.BypassReviewer{
					{
						ReviewerType: "TEAM",
						ReviewerName: &teamName,
					},
				},
			}

			mockClient.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return nil, errors.New("team not found")
			}
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("team not found"))
		})
	})

	Context("when GetRoleByName returns error", func() {
		BeforeEach(func() {
			roleName := testRole
			csc.Spec.SecretScanningDelegatedBypassOptions = &v1alpha1.SecretScanningDelegatedBypassOptions{
				Reviewers: []*v1alpha1.BypassReviewer{
					{
						ReviewerType: "ROLE",
						ReviewerName: &roleName,
					},
				},
			}

			mockClient.GetRoleByNameFunc = func(ctx context.Context, org string, roleName string) (*github.CustomOrgRole, error) {
				return nil, errors.New("role not found")
			}
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("role not found"))
		})
	})
})

var _ = Describe("AttachToRepos", func() {
	var (
		ctx        context.Context
		mockClient *ghclientmock.MockGitHubClientWrapper
		k8sClient  client.Client
		rec        *GitHubOrgReconciler
		scheme     *runtime.Scheme
		org        *v1alpha1.Organization
		csc        *v1alpha1.CodeSecurityConfiguration
		repo       *v1alpha1.Repository
		err        error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		org = &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testOrg,
				Namespace: defaultNamespace,
			},
			Spec: v1alpha1.OrganizationSpec{
				Name:                    testOrg,
				GitHubAppInstallationId: 12345,
			},
		}

		csc = &v1alpha1.CodeSecurityConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testCsc,
				Namespace: defaultNamespace,
			},
			Spec: v1alpha1.CodeSecurityConfigurationSpec{
				Name:        testCsc,
				Description: "Test code security configuration",
			},
		}

		repo = &v1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testRepo,
				Namespace: defaultNamespace,
			},
			Spec: v1alpha1.RepositorySpec{
				Name: testRepo,
				OrganizationRef: v1alpha1.OrganizationRef{
					Name: testOrg,
				},
			},
			Status: v1alpha1.RepositoryStatus{
				ID: new(int64(12345)),
			},
		}
	})

	JustBeforeEach(func() {
		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(org, csc, repo).
			WithStatusSubresource(org, csc, repo).
			WithIndex(&v1alpha1.Repository{}, "spec.attachedCodeSecurityConfigurations.name", func(obj client.Object) []string {
				r := obj.(*v1alpha1.Repository)
				// Note: The API field is singular AttachedCodeSecurityConfiguration, but the query uses plural
				if r.Spec.AttachedCodeSecurityConfiguration == nil {
					return nil
				}
				return []string{r.Spec.AttachedCodeSecurityConfiguration.Name}
			}).
			Build()

		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				Client:   mockClient,
				Resource: testOrg,
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Client:   k8sClient,
				Resource: org,
			},
		}
	})

	Context("when attaching to all repositories", func() {
		var attachScope string

		BeforeEach(func() {
			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				// Return empty to force a re-attach
				return []*github.RepositoryAttachment{}, nil
			}
			mockClient.GetOrgRepositoriesFunc = func(ctx context.Context, org string) ([]*github.Repository, error) {
				return []*github.Repository{
					{ID: new(int64(1)), Name: new("repo1"), Visibility: new("public")},
					{ID: new(int64(2)), Name: new("repo2"), Visibility: new("private")},
				}, nil
			}
			mockClient.AttachCodeSecurityConfigurationsFunc = func(ctx context.Context, org string, cscID int64, scope string, repoIDs []int64) error {
				attachScope = scope
				return nil
			}
		})

		It("should attach to all repos", func() {
			err = rec.attachCSC(ctx, "all", "some-code-security-configuration", 999)
			Expect(err).NotTo(HaveOccurred())
			Expect(attachScope).To(Equal("all"))
		})
	})

	Context("when attaching to selected repositories", func() {
		var attachedRepoIDs []int64
		var cscK8sName = testCsc
		var scope = "selected"

		BeforeEach(func() {
			repo.Spec.AttachedCodeSecurityConfiguration = &v1alpha1.CodeSecurityConfigurationRef{
				Name: cscK8sName,
			}

			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				return []*github.RepositoryAttachment{}, nil
			}
			mockClient.AttachCodeSecurityConfigurationsFunc = func(ctx context.Context, org string, cscID int64, scope string, repoIDs []int64) error {
				attachedRepoIDs = repoIDs
				return nil
			}
		})

		It("should attach to selected repos", func() {
			// Rebuild k8sClient with the updated repo so the index picks up the changes
			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(org, csc, repo).
				WithStatusSubresource(org, csc, repo).
				WithIndex(&v1alpha1.Repository{}, "spec.attachedCodeSecurityConfiguration.name", func(obj client.Object) []string {
					r := obj.(*v1alpha1.Repository)
					if r.Spec.AttachedCodeSecurityConfiguration == nil {
						return nil
					}
					return []string{r.Spec.AttachedCodeSecurityConfiguration.Name}
				}).
				Build()
			rec.Kubernetes.Client = k8sClient

			err = rec.attachCSC(ctx, scope, cscK8sName, 999)
			Expect(err).NotTo(HaveOccurred())
			Expect(attachedRepoIDs).To(ContainElement(int64(12345)))
		})
	})

	Context("when repository ID is not in status", func() {
		var attachedRepoIDs []int64
		var cscK8sName = testCsc
		var scope = "selected"
		BeforeEach(func() {
			repo.Spec.AttachedCodeSecurityConfiguration = &v1alpha1.CodeSecurityConfigurationRef{
				Name: cscK8sName,
			}
			repo.Status.ID = nil

			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				return []*github.RepositoryAttachment{}, nil
			}
			mockClient.GetRepositoryFunc = func(ctx context.Context, owner, repoName string) (*github.Repository, error) {
				return &github.Repository{
					ID:   new(int64(54321)),
					Name: new(testRepo),
				}, nil
			}
			mockClient.AttachCodeSecurityConfigurationsFunc = func(ctx context.Context, org string, cscID int64, scope string, repoIDs []int64) error {
				attachedRepoIDs = repoIDs
				return nil
			}
		})

		JustBeforeEach(func() {
			// Update the repo in the k8s client after modifying it in BeforeEach
			err := k8sClient.Update(ctx, repo)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fetch repo ID from GitHub and attach", func() {
			// Rebuild k8sClient with the updated repo so the index picks up the changes
			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(org, csc, repo).
				WithStatusSubresource(org, csc, repo).
				WithIndex(&v1alpha1.Repository{}, "spec.attachedCodeSecurityConfiguration.name", func(obj client.Object) []string {
					r := obj.(*v1alpha1.Repository)
					if r.Spec.AttachedCodeSecurityConfiguration == nil {
						return nil
					}
					return []string{r.Spec.AttachedCodeSecurityConfiguration.Name}
				}).
				Build()
			rec.Kubernetes.Client = k8sClient

			err = rec.attachCSC(ctx, scope, cscK8sName, 999)
			Expect(err).NotTo(HaveOccurred())
			Expect(attachedRepoIDs).To(ContainElement(int64(54321)))
		})
	})

	Context("when attachment API returns error", func() {
		BeforeEach(func() {
			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				return []*github.RepositoryAttachment{}, nil
			}
			mockClient.GetOrgRepositoriesFunc = func(ctx context.Context, org string) ([]*github.Repository, error) {
				return []*github.Repository{
					{ID: new(int64(1)), Name: new("repo1"), Visibility: new("public")},
				}, nil
			}
			mockClient.AttachCodeSecurityConfigurationsFunc = func(ctx context.Context, org string, cscID int64, scope string, repoIDs []int64) error {
				return errors.New("attachment error")
			}
		})

		It("should return error", func() {
			err = rec.attachCSC(ctx, "all", "some-code-security-configuration-name", 999)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("attachment error"))
		})
	})
})

var _ = Describe("AttachmentsDiffer", func() {
	Context("when attachments have different counts", func() {
		It("should return true", func() {
			current := []*github.RepositoryAttachment{
				{Repository: &github.Repository{Name: new("repo1")}},
			}
			desired := map[string]int64{
				"repo1": 1,
				"repo2": 2,
			}
			Expect(attachmentsDiffer(current, desired)).To(BeTrue())
		})
	})

	Context("when all current attachments exist in desired", func() {
		It("should return false (no difference when repos match)", func() {
			current := []*github.RepositoryAttachment{
				{Repository: &github.Repository{Name: new("repo1")}},
				{Repository: &github.Repository{Name: new("repo2")}},
			}
			desired := map[string]int64{
				"repo1": 1,
				"repo2": 2,
			}
			Expect(attachmentsDiffer(current, desired)).To(BeFalse())
		})
	})

	Context("when a current attachment is not in desired", func() {
		It("should return true (counts differ)", func() {
			current := []*github.RepositoryAttachment{
				{Repository: &github.Repository{Name: new("repo3")}},
			}
			desired := map[string]int64{
				"repo1": 1,
				"repo2": 2,
			}
			// This has different count (1 vs 2) so returns true
			Expect(attachmentsDiffer(current, desired)).To(BeTrue())
		})
	})

	Context("when current is empty and desired has items", func() {
		It("should return true due to count mismatch", func() {
			current := []*github.RepositoryAttachment{}
			desired := map[string]int64{
				"repo1": 1,
			}
			Expect(attachmentsDiffer(current, desired)).To(BeTrue())
		})
	})

	Context("when current attachments match desired exactly", func() {
		It("should return false", func() {
			current := []*github.RepositoryAttachment{
				{Repository: &github.Repository{Name: new("repo1"), ID: new(int64(1))}},
				{Repository: &github.Repository{Name: new("repo2"), ID: new(int64(2))}},
			}
			desired := map[string]int64{
				"repo1": 1,
				"repo2": 2,
			}
			Expect(attachmentsDiffer(current, desired)).To(BeFalse())
		})
	})
})

var _ = Describe("GetDesiredAttachmentsForScope", func() {
	var (
		ctx        context.Context
		mockClient *ghclientmock.MockGitHubClientWrapper
		rec        *GitHubOrgReconciler
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()
		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				Client:   mockClient,
				Resource: testOrg,
			},
		}
	})

	Context("when scope is 'all'", func() {
		It("should return all repositories", func() {
			mockClient.GetOrgRepositoriesFunc = func(ctx context.Context, org string) ([]*github.Repository, error) {
				return []*github.Repository{
					{ID: new(int64(1)), Name: new("repo1"), Visibility: new("public")},
					{ID: new(int64(2)), Name: new("repo2"), Visibility: new("private")},
					{ID: new(int64(3)), Name: new("repo3"), Visibility: new("internal")},
				}, nil
			}

			result, err := rec.getDesiredAttachmentsForScope(ctx, "all", testCsc)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(3))
			Expect(result).To(HaveKeyWithValue("repo1", int64(1)))
			Expect(result).To(HaveKeyWithValue("repo2", int64(2)))
			Expect(result).To(HaveKeyWithValue("repo3", int64(3)))
		})
	})

	Context("when scope is 'public'", func() {
		It("should return only public repositories", func() {
			mockClient.GetOrgRepositoriesFunc = func(ctx context.Context, org string) ([]*github.Repository, error) {
				return []*github.Repository{
					{ID: new(int64(1)), Name: new("repo1"), Visibility: new("public")},
					{ID: new(int64(2)), Name: new("repo2"), Visibility: new("private")},
					{ID: new(int64(3)), Name: new("repo3"), Visibility: new("internal")},
				}, nil
			}

			result, err := rec.getDesiredAttachmentsForScope(ctx, "public", testCsc)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result).To(HaveKeyWithValue("repo1", int64(1)))
		})
	})

	Context("when scope is 'private_or_internal'", func() {
		It("should return only private and internal repositories", func() {
			mockClient.GetOrgRepositoriesFunc = func(ctx context.Context, org string) ([]*github.Repository, error) {
				return []*github.Repository{
					{ID: new(int64(1)), Name: new("repo1"), Visibility: new("public")},
					{ID: new(int64(2)), Name: new("repo2"), Visibility: new("private")},
					{ID: new(int64(3)), Name: new("repo3"), Visibility: new("internal")},
				}, nil
			}

			result, err := rec.getDesiredAttachmentsForScope(ctx, "private_or_internal", testCsc)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(2))
			Expect(result).To(HaveKeyWithValue("repo2", int64(2)))
			Expect(result).To(HaveKeyWithValue("repo3", int64(3)))
		})
	})

	Context("when scope is 'selected'", func() {
		It("should delegate to getSelectedAttachedRepoNamesToIDs", func() {
			// This would require setting up K8s client with repos that reference the CSC
			// For now, just verify it doesn't error when no repos are found
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithIndex(&v1alpha1.Repository{}, "spec.attachedCodeSecurityConfiguration.name", func(obj client.Object) []string {
					r := obj.(*v1alpha1.Repository)
					if r.Spec.AttachedCodeSecurityConfiguration == nil {
						return nil
					}
					return []string{r.Spec.AttachedCodeSecurityConfiguration.Name}
				}).
				Build()
			rec.Kubernetes.Client = k8sClient
			rec.Kubernetes.Resource = &v1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testOrg,
					Namespace: defaultNamespace,
				},
			}

			result, err := rec.getDesiredAttachmentsForScope(ctx, "selected", testCsc)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeEmpty())
		})
	})

	Context("when GetOrgRepositories returns error", func() {
		It("should return error", func() {
			mockClient.GetOrgRepositoriesFunc = func(ctx context.Context, org string) ([]*github.Repository, error) {
				return nil, errors.New("API error")
			}

			_, err := rec.getDesiredAttachmentsForScope(ctx, "all", testCsc)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API error"))
		})
	})

	Context("when repository has nil ID or name", func() {
		It("should skip that repository", func() {
			mockClient.GetOrgRepositoriesFunc = func(ctx context.Context, org string) ([]*github.Repository, error) {
				return []*github.Repository{
					{ID: new(int64(1)), Name: new("repo1"), Visibility: new("public")},
					{ID: nil, Name: new("repo2"), Visibility: new("private")},
					{ID: new(int64(3)), Name: nil, Visibility: new("internal")},
				}, nil
			}

			result, err := rec.getDesiredAttachmentsForScope(ctx, "all", testCsc)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result).To(HaveKeyWithValue("repo1", int64(1)))
		})
	})
})

var _ = Describe("AttachCSC - Comprehensive Scenarios", func() {
	var (
		ctx        context.Context
		mockClient *ghclientmock.MockGitHubClientWrapper
		k8sClient  client.Client
		rec        *GitHubOrgReconciler
		scheme     *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()
		scheme = runtime.NewScheme()
		_ = v1alpha1.AddToScheme(scheme)
		k8sClient = fake.NewClientBuilder().WithScheme(scheme).Build()

		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				Client:   mockClient,
				Resource: testOrg,
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Client: k8sClient,
				Resource: &v1alpha1.Organization{
					ObjectMeta: metav1.ObjectMeta{Name: testOrg, Namespace: defaultNamespace},
				},
			},
		}
	})

	Context("when scope is 'all_without_configurations'", func() {
		It("should immediately attach without checking current attachments", func() {
			var capturedScope string
			var capturedRepoIds []int64
			mockClient.AttachCodeSecurityConfigurationsFunc = func(ctx context.Context, org string, cscID int64, scope string, repoIDs []int64) error {
				capturedScope = scope
				capturedRepoIds = repoIDs
				return nil
			}

			err := rec.attachCSC(ctx, "all_without_configurations", testCsc, 999)
			Expect(err).NotTo(HaveOccurred())
			Expect(capturedScope).To(Equal("all_without_configurations"))
			Expect(capturedRepoIds).To(BeNil())
		})
	})

	Context("when current attachments match desired", func() {
		It("should not call attach API", func() {
			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				return []*github.RepositoryAttachment{
					{
						Repository: &github.Repository{Name: new("repo1"), ID: new(int64(1))},
						Status:     new("attached"),
					},
					{
						Repository: &github.Repository{Name: new("repo2"), ID: new(int64(2))},
						Status:     new("attached"),
					},
				}, nil
			}
			mockClient.GetOrgRepositoriesFunc = func(ctx context.Context, org string) ([]*github.Repository, error) {
				return []*github.Repository{
					{ID: new(int64(1)), Name: new("repo1"), Visibility: new("public")},
					{ID: new(int64(2)), Name: new("repo2"), Visibility: new("private")},
				}, nil
			}

			attachCalled := false
			mockClient.AttachCodeSecurityConfigurationsFunc = func(ctx context.Context, org string, cscID int64, scope string, repoIDs []int64) error {
				attachCalled = true
				return nil
			}

			err := rec.attachCSC(ctx, "all", testCsc, 999)
			Expect(err).NotTo(HaveOccurred())
			Expect(attachCalled).To(BeFalse())
		})
	})

	Context("when current attachments differ from desired", func() {
		It("should call attach API with correct scope", func() {
			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				return []*github.RepositoryAttachment{
					{
						Repository: &github.Repository{Name: new("repo1"), ID: new(int64(1))},
						Status:     new("attached"),
					},
				}, nil
			}
			mockClient.GetOrgRepositoriesFunc = func(ctx context.Context, org string) ([]*github.Repository, error) {
				return []*github.Repository{
					{ID: new(int64(1)), Name: new("repo1"), Visibility: new("public")},
					{ID: new(int64(2)), Name: new("repo2"), Visibility: new("private")},
				}, nil
			}

			var capturedScope string
			mockClient.AttachCodeSecurityConfigurationsFunc = func(ctx context.Context, org string, cscID int64, scope string, repoIDs []int64) error {
				capturedScope = scope
				return nil
			}

			err := rec.attachCSC(ctx, "all", testCsc, 999)
			Expect(err).NotTo(HaveOccurred())
			Expect(capturedScope).To(Equal("all"))
		})
	})

	Context("when scope is 'public' and attachments differ", func() {
		It("should attach with 'public' scope and nil repo IDs", func() {
			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				return []*github.RepositoryAttachment{}, nil
			}
			mockClient.GetOrgRepositoriesFunc = func(ctx context.Context, org string) ([]*github.Repository, error) {
				return []*github.Repository{
					{ID: new(int64(1)), Name: new("repo1"), Visibility: new("public")},
					{ID: new(int64(2)), Name: new("repo2"), Visibility: new("private")},
				}, nil
			}

			var capturedScope string
			var capturedRepoIds []int64
			mockClient.AttachCodeSecurityConfigurationsFunc = func(ctx context.Context, org string, cscID int64, scope string, repoIDs []int64) error {
				capturedScope = scope
				capturedRepoIds = repoIDs
				return nil
			}

			err := rec.attachCSC(ctx, "public", testCsc, 999)
			Expect(err).NotTo(HaveOccurred())
			Expect(capturedScope).To(Equal("public"))
			Expect(capturedRepoIds).To(BeNil())
		})
	})

	Context("when scope is 'selected'", func() {
		It("should pass specific repo IDs to attach API", func() {
			repo := &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{Name: testRepo, Namespace: defaultNamespace},
				Spec: v1alpha1.RepositorySpec{
					Name: testRepo,
					AttachedCodeSecurityConfiguration: &v1alpha1.CodeSecurityConfigurationRef{
						Name: testCsc,
					},
				},
				Status: v1alpha1.RepositoryStatus{
					ID: new(int64(12345)),
				},
			}

			k8sClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(repo).
				WithIndex(&v1alpha1.Repository{}, "spec.attachedCodeSecurityConfiguration.name", func(obj client.Object) []string {
					r := obj.(*v1alpha1.Repository)
					if r.Spec.AttachedCodeSecurityConfiguration == nil {
						return nil
					}
					return []string{r.Spec.AttachedCodeSecurityConfiguration.Name}
				}).
				Build()
			rec.Kubernetes.Client = k8sClient

			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				return []*github.RepositoryAttachment{}, nil
			}

			var capturedRepoIds []int64
			mockClient.AttachCodeSecurityConfigurationsFunc = func(ctx context.Context, org string, cscID int64, scope string, repoIDs []int64) error {
				capturedRepoIds = repoIDs
				return nil
			}

			err := rec.attachCSC(ctx, "selected", testCsc, 999)
			Expect(err).NotTo(HaveOccurred())
			Expect(capturedRepoIds).To(ContainElement(int64(12345)))
		})
	})

	Context("when GetRepositoriesAttachedToCodeSecurityConfiguration fails", func() {
		It("should return error", func() {
			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				return nil, errors.New("GitHub API error")
			}

			err := rec.attachCSC(ctx, "all", testCsc, 999)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GitHub API error"))
		})
	})

	Context("when getDesiredAttachmentsForScope fails", func() {
		It("should return error", func() {
			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				return []*github.RepositoryAttachment{}, nil
			}
			mockClient.GetOrgRepositoriesFunc = func(ctx context.Context, org string) ([]*github.Repository, error) {
				return nil, errors.New("failed to list repos")
			}

			err := rec.attachCSC(ctx, "all", testCsc, 999)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to list repos"))
		})
	})
})

var _ = Describe("GetAttachmentsIfAllAttached", func() {
	var (
		ctx        context.Context
		mockClient *ghclientmock.MockGitHubClientWrapper
		rec        *GitHubOrgReconciler
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()
		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				Client:   mockClient,
				Resource: testOrg,
			},
		}
	})

	Context("when all attachments are in 'attached' status", func() {
		It("should return attachments", func() {
			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				return []*github.RepositoryAttachment{
					{Repository: &github.Repository{Name: new("repo1")}, Status: new("attached")},
					{Repository: &github.Repository{Name: new("repo2")}, Status: new("attached")},
				}, nil
			}

			result, err := rec.getAttachmentsIfAllAttached(ctx, 999, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(2))
		})
	})

	Context("when all attachments are in 'enforced' status", func() {
		It("should return attachments", func() {
			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				return []*github.RepositoryAttachment{
					{Repository: &github.Repository{Name: new("repo1")}, Status: new("enforced")},
				}, nil
			}

			result, err := rec.getAttachmentsIfAllAttached(ctx, 999, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
		})
	})

	Context("when all attachments are in 'removed_by_enterprise' status", func() {
		It("should return attachments", func() {
			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				return []*github.RepositoryAttachment{
					{Repository: &github.Repository{Name: new("repo1")}, Status: new("removed_by_enterprise")},
				}, nil
			}

			result, err := rec.getAttachmentsIfAllAttached(ctx, 999, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
		})
	})

	Context("when some attachments are in 'attaching' status", func() {
		It("should return nil without error", func() {
			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				return []*github.RepositoryAttachment{
					{Repository: &github.Repository{Name: new("repo1")}, Status: new("attached")},
					{Repository: &github.Repository{Name: new("repo2")}, Status: new("attaching")},
				}, nil
			}

			result, err := rec.getAttachmentsIfAllAttached(ctx, 999, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Context("when some attachments are in 'updating' status", func() {
		It("should return nil without error", func() {
			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				return []*github.RepositoryAttachment{
					{Repository: &github.Repository{Name: new("repo1")}, Status: new("updating")},
				}, nil
			}

			result, err := rec.getAttachmentsIfAllAttached(ctx, 999, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Context("when an attachment has 'failed' status and returnErrorOnFailedStatus is false", func() {
		It("should return nil without error", func() {
			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				return []*github.RepositoryAttachment{
					{Repository: &github.Repository{Name: new("repo1")}, Status: new("failed")},
				}, nil
			}

			result, err := rec.getAttachmentsIfAllAttached(ctx, 999, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Context("when an attachment has 'failed' status and returnErrorOnFailedStatus is true", func() {
		It("should return error", func() {
			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				return []*github.RepositoryAttachment{
					{Repository: &github.Repository{Name: new("repo1")}, Status: new("failed")},
				}, nil
			}

			opts := &GetAttachmentsOpts{returnErrorOnFailedStatus: true}
			result, err := rec.getAttachmentsIfAllAttached(ctx, 999, opts)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed attachment status"))
			Expect(result).To(BeNil())
		})
	})

	Context("when an attachment has 'detached' status", func() {
		It("should return nil without error", func() {
			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				return []*github.RepositoryAttachment{
					{Repository: &github.Repository{Name: new("repo1")}, Status: new("detached")},
				}, nil
			}

			result, err := rec.getAttachmentsIfAllAttached(ctx, 999, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Context("when an attachment has 'removed' status", func() {
		It("should return nil without error", func() {
			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				return []*github.RepositoryAttachment{
					{Repository: &github.Repository{Name: new("repo1")}, Status: new("removed")},
				}, nil
			}

			result, err := rec.getAttachmentsIfAllAttached(ctx, 999, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Context("when GitHub API returns error", func() {
		It("should return error", func() {
			mockClient.GetRepositoriesAttachedToCodeSecurityConfigurationFunc = func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
				return nil, errors.New("API error")
			}

			result, err := rec.getAttachmentsIfAllAttached(ctx, 999, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API error"))
			Expect(result).To(BeNil())
		})
	})
})

var _ = Describe("ByName", func() {
	Context("when filtering organization-level configs", func() {
		It("should return only organization configs", func() {
			configs := []*github.CodeSecurityConfiguration{
				{
					Name:       "org-config",
					TargetType: new(targetTypeOrganization),
				},
				{
					Name:       "enterprise-config",
					TargetType: new("enterprise"),
				},
			}

			result := byName(configs,
				func(c github.CodeSecurityConfiguration) string { return c.Name },
				func(c github.CodeSecurityConfiguration) bool { return c.GetTargetType() != "organization" },
			)

			Expect(result).To(HaveLen(1))
			Expect(result["org-config"]).NotTo(BeNil())
			Expect(result["enterprise-config"]).To(BeZero())
		})
	})

	Context("when multiple filters are applied", func() {
		It("should exclude items matching any filter", func() {
			configs := []*github.CodeSecurityConfiguration{
				{
					Name:       "org-config-1",
					TargetType: new(targetTypeOrganization),
				},
				{
					Name:       "enterprise-config",
					TargetType: new("enterprise"),
				},
				{
					Name:       "org-config-excluded",
					TargetType: new(targetTypeOrganization),
				},
			}

			result := byName(configs,
				func(c github.CodeSecurityConfiguration) string { return c.Name },
				func(c github.CodeSecurityConfiguration) bool { return c.GetTargetType() != "organization" },
				func(c github.CodeSecurityConfiguration) bool { return c.Name == "org-config-excluded" },
			)

			Expect(result).To(HaveLen(1))
			Expect(result["org-config-1"]).NotTo(BeNil())
			Expect(result["enterprise-config"]).To(BeZero())
			Expect(result["org-config-excluded"]).To(BeZero())
		})
	})

	Context("when no filters are applied", func() {
		It("should return all items", func() {
			configs := []*github.CodeSecurityConfiguration{
				{Name: "config1"},
				{Name: "config2"},
			}

			result := byName(configs,
				func(c github.CodeSecurityConfiguration) string { return c.Name },
			)

			Expect(result).To(HaveLen(2))
			Expect(result["config1"]).NotTo(BeNil())
			Expect(result["config2"]).NotTo(BeNil())
		})
	})

	Context("when multiple filters are applied", func() {
		It("should exclude items matching any filter", func() {
			configs := []*github.CodeSecurityConfiguration{
				{
					Name:       "org-config-1",
					TargetType: new(targetTypeOrganization),
				},
				{
					Name:       "enterprise-config",
					TargetType: new("enterprise"),
				},
				{
					Name:       "org-config-excluded",
					TargetType: new(targetTypeOrganization),
				},
			}

			result := byName(configs,
				func(c github.CodeSecurityConfiguration) string { return c.Name },
				func(c github.CodeSecurityConfiguration) bool { return c.GetTargetType() != "organization" },
				func(c github.CodeSecurityConfiguration) bool { return c.Name == "org-config-excluded" },
			)

			Expect(result).To(HaveLen(1))
			Expect(result["org-config-1"]).NotTo(BeNil())
			Expect(result["enterprise-config"]).To(BeZero())
			Expect(result["org-config-excluded"]).To(BeZero())
		})
	})

	Context("when list contains nil items", func() {
		It("should skip nil items", func() {
			configs := []*github.CodeSecurityConfiguration{
				{Name: "config1"},
				nil,
				{Name: "config2"},
			}

			result := byName(configs,
				func(c github.CodeSecurityConfiguration) string { return c.Name },
			)

			Expect(result).To(HaveLen(2))
			Expect(result["config1"]).NotTo(BeNil())
			Expect(result["config2"]).NotTo(BeNil())
		})
	})
})
