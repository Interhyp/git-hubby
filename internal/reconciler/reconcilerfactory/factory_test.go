package reconcilerfactory

import (
	"context"
	"errors"

	"time"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/ghclient"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/internal/reconciler/orgrec"
	"github.com/Interhyp/git-hubby/internal/reconciler/reporec"
	"github.com/Interhyp/git-hubby/internal/reconciler/spreading"
	"github.com/Interhyp/git-hubby/internal/reconciler/teamrec"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Factory", func() {
	var (
		ctx            context.Context
		factory        *Factory
		k8sClient      client.Client
		scheme         *runtime.Scheme
		mockClientMgr  *mockGitHubClientManager
		mockGHClient   *ghclientmock.MockGitHubClientWrapper
		mockSpreadMgr  *mockSpreadManager
		defaultNS      string
		defaultOrgName string
		defaultAppID   int64
	)

	BeforeEach(func() {
		ctx = context.Background()
		defaultNS = "default"
		defaultOrgName = "test-org"
		defaultAppID = 12345

		// Initialize scheme
		scheme = runtime.NewScheme()
		Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())

		// Create mock GitHub client
		mockGHClient = ghclientmock.NewMockGitHubClientWrapper()

		// Create mock client manager
		mockClientMgr = &mockGitHubClientManager{
			client:          mockGHClient,
			shouldFailLimit: false,
			shouldFail:      false,
		}

		// Create mock spreading manager (default: no spreading required)
		mockSpreadMgr = &mockSpreadManager{
			shouldSpread: false,
		}

		// Create K8s client
		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&v1alpha1.Organization{}, &v1alpha1.Repository{}, &v1alpha1.Team{},
				&v1alpha1.RulesetPreset{}, &v1alpha1.WebhookPreset{}, &v1alpha1.CodeSecurityConfiguration{}).
			Build()

		// Create factory
		factory = &Factory{
			ClientManager:    mockClientMgr,
			K8sClient:        k8sClient,
			SpreadingManager: mockSpreadMgr,
		}
	})

	Describe("CreateForOrg", func() {
		var (
			org            *v1alpha1.Organization
			namespacedName types.NamespacedName
			executor       *reconciler.ReconciliationExecutor[*v1alpha1.Organization]
			err            error
		)

		BeforeEach(func() {
			namespacedName = types.NamespacedName{
				Name:      defaultOrgName,
				Namespace: defaultNS,
			}

			org = &v1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      defaultOrgName,
					Namespace: defaultNS,
				},
				Spec: v1alpha1.OrganizationSpec{
					Name:                    defaultOrgName,
					Description:             "Test Organization",
					GitHubAppInstallationId: defaultAppID,
				},
			}
		})

		JustBeforeEach(func() {
			executor, err = factory.CreateForOrg(ctx, namespacedName)
		})

		Context("when organization exists", func() {
			BeforeEach(func() {
				Expect(k8sClient.Create(ctx, org)).To(Succeed())
			})

			It("should create executor successfully", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(executor).NotTo(BeNil())
			})

			It("should return executor with correct reconciler type", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(executor.Reconciler).To(BeAssignableToTypeOf(&orgrec.GitHubOrgReconciler{}))
			})

			It("should configure reconciler with correct organization name", func() {
				Expect(err).NotTo(HaveOccurred())
				orgRec := executor.Reconciler.(*orgrec.GitHubOrgReconciler)
				Expect(orgRec.GitHub.Resource).To(Equal(defaultOrgName))
			})

			It("should configure reconciler with GitHub client", func() {
				Expect(err).NotTo(HaveOccurred())
				orgRec := executor.Reconciler.(*orgrec.GitHubOrgReconciler)
				Expect(orgRec.GitHub.Client).NotTo(BeNil())
				Expect(orgRec.GitHub.Client).To(Equal(mockGHClient))
			})

			It("should configure reconciler with K8s client", func() {
				Expect(err).NotTo(HaveOccurred())
				orgRec := executor.Reconciler.(*orgrec.GitHubOrgReconciler)
				Expect(orgRec.Kubernetes.Client).NotTo(BeNil())
			})

			It("should configure reconciler with organization resource", func() {
				Expect(err).NotTo(HaveOccurred())
				orgRec := executor.Reconciler.(*orgrec.GitHubOrgReconciler)
				Expect(orgRec.Kubernetes.Resource).NotTo(BeNil())
				Expect(orgRec.Kubernetes.Resource.Name).To(Equal(defaultOrgName))
			})

			It("should call GetGitHubClientAndCheckRateLimit with correct parameters", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(mockClientMgr.lastOrgName).To(Equal(defaultOrgName))
				Expect(mockClientMgr.lastAppID).To(Equal(defaultAppID))
				Expect(mockClientMgr.lastRateLimit).To(Equal(orgRateLimitThreshold))
			})
		})

		Context("when organization does not exist", func() {
			It("should return nil executor and nil error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(executor).To(BeNil())
			})

			It("should not call client manager", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(mockClientMgr.callCount).To(Equal(0))
			})
		})

		Context("when rate limit check fails", func() {
			BeforeEach(func() {
				Expect(k8sClient.Create(ctx, org)).To(Succeed())
				mockClientMgr.shouldFailLimit = true
				mockClientMgr.rateLimitErr = errors.New("rate limit exceeded")
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("rate limit exceeded"))
			})

			It("should not create executor", func() {
				Expect(executor).To(BeNil())
			})
		})

		Context("when client manager fails", func() {
			BeforeEach(func() {
				Expect(k8sClient.Create(ctx, org)).To(Succeed())
				mockClientMgr.shouldFail = true
				mockClientMgr.genericErr = errors.New("client creation failed")
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("client creation failed"))
			})

			It("should not create executor", func() {
				Expect(executor).To(BeNil())
			})
		})
	})

	Describe("CreateForRepo", func() {
		var (
			org            *v1alpha1.Organization
			repo           *v1alpha1.Repository
			namespacedName types.NamespacedName
			executor       *reconciler.ReconciliationExecutor[*v1alpha1.Repository]
			err            error
		)

		BeforeEach(func() {
			namespacedName = types.NamespacedName{
				Name:      "test-repo",
				Namespace: defaultNS,
			}

			org = &v1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      defaultOrgName,
					Namespace: defaultNS,
				},
				Spec: v1alpha1.OrganizationSpec{
					Name:                    defaultOrgName,
					Description:             "Test Organization",
					GitHubAppInstallationId: defaultAppID,
				},
			}

			repo = &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: defaultNS,
				},
				Spec: v1alpha1.RepositorySpec{
					Name: "test-repo",
					OrganizationRef: v1alpha1.OrganizationRef{
						Name: defaultOrgName,
					},
				},
			}
		})

		JustBeforeEach(func() {
			executor, err = factory.CreateForRepo(ctx, namespacedName)
		})

		Context("when repository and organization exist", func() {
			BeforeEach(func() {
				Expect(k8sClient.Create(ctx, org)).To(Succeed())
				Expect(k8sClient.Create(ctx, repo)).To(Succeed())
			})

			It("should create executor successfully", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(executor).NotTo(BeNil())
			})

			It("should return executor with correct reconciler type", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(executor.Reconciler).To(BeAssignableToTypeOf(&reporec.GitHubRepoReconciler{}))
			})

			It("should configure reconciler with correct repository identifier", func() {
				Expect(err).NotTo(HaveOccurred())
				repoRec := executor.Reconciler.(*reporec.GitHubRepoReconciler)
				Expect(repoRec.GitHub.Resource.Owner).To(Equal(defaultOrgName))
				Expect(repoRec.GitHub.Resource.Name).To(Equal("test-repo"))
			})

			It("should configure reconciler with GitHub client", func() {
				Expect(err).NotTo(HaveOccurred())
				repoRec := executor.Reconciler.(*reporec.GitHubRepoReconciler)
				Expect(repoRec.GitHub.Client).NotTo(BeNil())
				Expect(repoRec.GitHub.Client).To(Equal(mockGHClient))
			})

			It("should configure reconciler with K8s client", func() {
				Expect(err).NotTo(HaveOccurred())
				repoRec := executor.Reconciler.(*reporec.GitHubRepoReconciler)
				Expect(repoRec.Kubernetes.Client).NotTo(BeNil())
			})

			It("should configure reconciler with repository resource", func() {
				Expect(err).NotTo(HaveOccurred())
				repoRec := executor.Reconciler.(*reporec.GitHubRepoReconciler)
				Expect(repoRec.Kubernetes.Resource).NotTo(BeNil())
				Expect(repoRec.Kubernetes.Resource.Name).To(Equal("test-repo"))
			})

			It("should call GetGitHubClientAndCheckRateLimit with correct parameters", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(mockClientMgr.lastOrgName).To(Equal(defaultOrgName))
				Expect(mockClientMgr.lastAppID).To(Equal(defaultAppID))
				Expect(mockClientMgr.lastRateLimit).To(Equal(repoRateLimitThreshold))
			})
		})

		Context("when repository does not exist", func() {
			BeforeEach(func() {
				Expect(k8sClient.Create(ctx, org)).To(Succeed())
			})

			It("should return nil executor and nil error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(executor).To(BeNil())
			})

			It("should not call client manager", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(mockClientMgr.callCount).To(Equal(0))
			})
		})

		Context("when organization does not exist", func() {
			BeforeEach(func() {
				Expect(k8sClient.Create(ctx, repo)).To(Succeed())
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should not create executor", func() {
				Expect(executor).To(BeNil())
			})
		})

		Context("when rate limit check fails", func() {
			BeforeEach(func() {
				Expect(k8sClient.Create(ctx, org)).To(Succeed())
				Expect(k8sClient.Create(ctx, repo)).To(Succeed())
				mockClientMgr.shouldFailLimit = true
				mockClientMgr.rateLimitErr = errors.New("rate limit exceeded")
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("rate limit exceeded"))
			})

			It("should not create executor", func() {
				Expect(executor).To(BeNil())
			})
		})

		Context("when client manager fails", func() {
			BeforeEach(func() {
				Expect(k8sClient.Create(ctx, org)).To(Succeed())
				Expect(k8sClient.Create(ctx, repo)).To(Succeed())
				mockClientMgr.shouldFail = true
				mockClientMgr.genericErr = errors.New("client creation failed")
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("client creation failed"))
			})

			It("should not create executor", func() {
				Expect(executor).To(BeNil())
			})
		})

		Context("with repository in different namespace than organization", func() {
			BeforeEach(func() {
				org.Namespace = "other-namespace"
				Expect(k8sClient.Create(ctx, org)).To(Succeed())
				Expect(k8sClient.Create(ctx, repo)).To(Succeed())
			})

			It("should return error as organization not found in same namespace", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("CreateForTeam", func() {
		var (
			org1           *v1alpha1.Organization
			org2           *v1alpha1.Organization
			team           *v1alpha1.Team
			namespacedName types.NamespacedName
			executor       *reconciler.ReconciliationExecutor[*v1alpha1.Team]
			err            error
			mockGHClient2  *ghclientmock.MockGitHubClientWrapper
		)

		BeforeEach(func() {
			namespacedName = types.NamespacedName{
				Name:      "test-team",
				Namespace: defaultNS,
			}

			org1 = &v1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "org1",
					Namespace: defaultNS,
				},
				Spec: v1alpha1.OrganizationSpec{
					Name:                    "org1",
					Description:             "Test Organization 1",
					GitHubAppInstallationId: 11111,
				},
			}

			org2 = &v1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "org2",
					Namespace: defaultNS,
				},
				Spec: v1alpha1.OrganizationSpec{
					Name:                    "org2",
					Description:             "Test Organization 2",
					GitHubAppInstallationId: 22222,
				},
			}

			team = &v1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-team",
					Namespace: defaultNS,
				},
				Spec: v1alpha1.TeamSpec{
					Name:        "test-team",
					Description: "Test Team",
					Members:     []string{"user1", "user2"},
					OrganizationRefs: []v1alpha1.OrganizationRef{
						{Name: "org1"},
					},
				},
				Status: v1alpha1.TeamStatus{
					Slug: new("test-team"),
				},
			}

			// Create second mock client for multi-org scenarios
			mockGHClient2 = ghclientmock.NewMockGitHubClientWrapper()
		})

		JustBeforeEach(func() {
			executor, err = factory.CreateForTeam(ctx, namespacedName)
		})

		Context("when team with single organization exists", func() {
			BeforeEach(func() {
				Expect(k8sClient.Create(ctx, org1)).To(Succeed())
				Expect(k8sClient.Create(ctx, team)).To(Succeed())
			})

			It("should create executor successfully", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(executor).NotTo(BeNil())
			})

			It("should return executor with correct reconciler type", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(executor.Reconciler).To(BeAssignableToTypeOf(&teamrec.GitHubTeamReconciler{}))
			})

			It("should configure reconciler with correct team identifier", func() {
				Expect(err).NotTo(HaveOccurred())
				teamRec := executor.Reconciler.(*teamrec.GitHubTeamReconciler)
				Expect(teamRec.Team.Name).To(Equal("test-team"))
				Expect(teamRec.Team.GetSlug()).To(Equal("test-team"))
			})

			It("should configure reconciler with current organizations", func() {
				Expect(err).NotTo(HaveOccurred())
				teamRec := executor.Reconciler.(*teamrec.GitHubTeamReconciler)
				Expect(teamRec.Team.Organizations.Current).To(HaveLen(1))
				Expect(teamRec.Team.Organizations.Current[0].Resource).To(Equal("org1"))
			})

			It("should configure reconciler with empty previous organizations", func() {
				Expect(err).NotTo(HaveOccurred())
				teamRec := executor.Reconciler.(*teamrec.GitHubTeamReconciler)
				Expect(teamRec.Team.Organizations.Previous).To(BeEmpty())
			})

			It("should configure reconciler with K8s client", func() {
				Expect(err).NotTo(HaveOccurred())
				teamRec := executor.Reconciler.(*teamrec.GitHubTeamReconciler)
				Expect(teamRec.Kubernetes.Client).NotTo(BeNil())
			})

			It("should configure reconciler with team resource", func() {
				Expect(err).NotTo(HaveOccurred())
				teamRec := executor.Reconciler.(*teamrec.GitHubTeamReconciler)
				Expect(teamRec.Kubernetes.Resource).NotTo(BeNil())
				Expect(teamRec.Kubernetes.Resource.Name).To(Equal("test-team"))
			})

			It("should call GetGitHubClientAndCheckRateLimit with correct parameters", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(mockClientMgr.lastRateLimit).To(Equal(teamRateLimitThreshold))
			})
		})

		Context("when team with multiple organizations exists", func() {
			BeforeEach(func() {
				team.Spec.OrganizationRefs = []v1alpha1.OrganizationRef{
					{Name: "org1"},
					{Name: "org2"},
				}

				// Mock different clients for different orgs
				mockClientMgr.clientByOrg = map[string]*ghclientmock.MockGitHubClientWrapper{
					"org1": mockGHClient,
					"org2": mockGHClient2,
				}

				Expect(k8sClient.Create(ctx, org1)).To(Succeed())
				Expect(k8sClient.Create(ctx, org2)).To(Succeed())
				Expect(k8sClient.Create(ctx, team)).To(Succeed())
			})

			It("should create executor successfully", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(executor).NotTo(BeNil())
			})

			It("should configure reconciler with multiple current organizations", func() {
				Expect(err).NotTo(HaveOccurred())
				teamRec := executor.Reconciler.(*teamrec.GitHubTeamReconciler)
				Expect(teamRec.Team.Organizations.Current).To(HaveLen(2))
				Expect(teamRec.Team.Organizations.Current[0].Resource).To(Equal("org1"))
				Expect(teamRec.Team.Organizations.Current[1].Resource).To(Equal("org2"))
			})

			It("should create clients for each organization", func() {
				Expect(err).NotTo(HaveOccurred())
				teamRec := executor.Reconciler.(*teamrec.GitHubTeamReconciler)
				Expect(teamRec.Team.Organizations.Current[0].Client).NotTo(BeNil())
				Expect(teamRec.Team.Organizations.Current[1].Client).NotTo(BeNil())
			})
		})

		Context("when team has previous organization refs", func() {
			BeforeEach(func() {
				team.Status.PreviousOrganizationRefs = []v1alpha1.OrganizationRef{
					{Name: "org2"},
				}

				Expect(k8sClient.Create(ctx, org1)).To(Succeed())
				Expect(k8sClient.Create(ctx, org2)).To(Succeed())
				Expect(k8sClient.Create(ctx, team)).To(Succeed())
			})

			It("should configure reconciler with previous organizations", func() {
				Expect(err).NotTo(HaveOccurred())
				teamRec := executor.Reconciler.(*teamrec.GitHubTeamReconciler)
				Expect(teamRec.Team.Organizations.Previous).To(HaveLen(1))
				Expect(teamRec.Team.Organizations.Previous[0].Resource).To(Equal("org2"))
			})
		})

		Context("when team slug is not yet set in status", func() {
			BeforeEach(func() {
				team.Status.Slug = nil

				Expect(k8sClient.Create(ctx, org1)).To(Succeed())
				Expect(k8sClient.Create(ctx, team)).To(Succeed())
			})

			It("should create executor successfully", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(executor).NotTo(BeNil())
			})

			It("should configure reconciler with empty slug", func() {
				Expect(err).NotTo(HaveOccurred())
				teamRec := executor.Reconciler.(*teamrec.GitHubTeamReconciler)
				Expect(teamRec.Team.GetSlug()).To(Equal(""))
			})

			It("should configure reconciler with nil slug pointer", func() {
				Expect(err).NotTo(HaveOccurred())
				teamRec := executor.Reconciler.(*teamrec.GitHubTeamReconciler)
				Expect(teamRec.Team.Slug).To(BeNil())
			})

			It("should still configure other fields correctly", func() {
				Expect(err).NotTo(HaveOccurred())
				teamRec := executor.Reconciler.(*teamrec.GitHubTeamReconciler)
				Expect(teamRec.Team.Name).To(Equal("test-team"))
				Expect(teamRec.Team.Organizations.Current).To(HaveLen(1))
				Expect(teamRec.Team.Organizations.Current[0].Resource).To(Equal("org1"))
			})
		})

		Context("when team does not exist", func() {
			It("should return nil executor and nil error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(executor).To(BeNil())
			})
		})

		Context("when team has no organizations", func() {
			BeforeEach(func() {
				team.Spec.OrganizationRefs = []v1alpha1.OrganizationRef{}
				Expect(k8sClient.Create(ctx, team)).To(Succeed())
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no organizations found"))
			})
		})

		Context("when organization referenced by team does not exist", func() {
			BeforeEach(func() {
				Expect(k8sClient.Create(ctx, team)).To(Succeed())
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when rate limit check fails for organization", func() {
			BeforeEach(func() {
				Expect(k8sClient.Create(ctx, org1)).To(Succeed())
				Expect(k8sClient.Create(ctx, team)).To(Succeed())
				mockClientMgr.shouldFailLimit = true
				mockClientMgr.rateLimitErr = errors.New("rate limit exceeded")
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("rate limit exceeded"))
			})
		})

		Context("when client creation fails for organization", func() {
			BeforeEach(func() {
				Expect(k8sClient.Create(ctx, org1)).To(Succeed())
				Expect(k8sClient.Create(ctx, team)).To(Succeed())
				mockClientMgr.shouldFail = true
				mockClientMgr.genericErr = errors.New("client creation failed")
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("client creation failed"))
			})
		})

		Context("when previous organization no longer exists", func() {
			BeforeEach(func() {
				team.Status.PreviousOrganizationRefs = []v1alpha1.OrganizationRef{
					{Name: "non-existent-org"},
				}

				Expect(k8sClient.Create(ctx, org1)).To(Succeed())
				Expect(k8sClient.Create(ctx, team)).To(Succeed())
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("getOrgByRef", func() {
		var (
			org       *v1alpha1.Organization
			result    v1alpha1.Organization
			err       error
			orgRef    string
			namespace string
		)

		BeforeEach(func() {
			orgRef = "test-org-ref"
			namespace = defaultNS

			org = &v1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      orgRef,
					Namespace: namespace,
				},
				Spec: v1alpha1.OrganizationSpec{
					Name:                    orgRef,
					Description:             "Test Organization",
					GitHubAppInstallationId: defaultAppID,
				},
			}
		})

		JustBeforeEach(func() {
			result, err = factory.getOrgByRef(ctx, orgRef, namespace)
		})

		Context("when organization exists", func() {
			BeforeEach(func() {
				Expect(k8sClient.Create(ctx, org)).To(Succeed())
			})

			It("should return organization successfully", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Name).To(Equal(orgRef))
				Expect(result.Namespace).To(Equal(namespace))
			})

			It("should return organization with correct spec", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.Name).To(Equal(orgRef))
				Expect(result.Spec.GitHubAppInstallationId).To(Equal(defaultAppID))
			})
		})

		Context("when organization does not exist", func() {
			It("should return error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when namespace is different", func() {
			BeforeEach(func() {
				org.Namespace = "other-namespace"
				Expect(k8sClient.Create(ctx, org)).To(Succeed())
			})

			It("should return error as org not found in specified namespace", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("buildGitHubOrgsSlice", func() {
		var (
			org1   *v1alpha1.Organization
			org2   *v1alpha1.Organization
			team   *v1alpha1.Team
			result []reconciler.GitHub[string]
			err    error
		)

		BeforeEach(func() {
			org1 = &v1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "org1",
					Namespace: defaultNS,
				},
				Spec: v1alpha1.OrganizationSpec{
					Name:                    "org1",
					GitHubAppInstallationId: 11111,
				},
			}

			org2 = &v1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "org2",
					Namespace: defaultNS,
				},
				Spec: v1alpha1.OrganizationSpec{
					Name:                    "org2",
					GitHubAppInstallationId: 22222,
				},
			}

			team = &v1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-team",
					Namespace: defaultNS,
				},
				Spec: v1alpha1.TeamSpec{
					Name: "test-team",
					OrganizationRefs: []v1alpha1.OrganizationRef{
						{Name: "org1"},
						{Name: "org2"},
					},
				},
			}
		})

		Context("when building from current organization refs", func() {
			JustBeforeEach(func() {
				result, err = buildGitHubOrgsSlice(ctx, factory, *team, func(t v1alpha1.Team) []v1alpha1.OrganizationRef {
					return t.Spec.OrganizationRefs
				})
			})

			Context("when all organizations exist", func() {
				BeforeEach(func() {
					Expect(k8sClient.Create(ctx, org1)).To(Succeed())
					Expect(k8sClient.Create(ctx, org2)).To(Succeed())
				})

				It("should return slice with correct length", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(HaveLen(2))
				})

				It("should return organizations with correct resources", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result[0].Resource).To(Equal("org1"))
					Expect(result[1].Resource).To(Equal("org2"))
				})

				It("should return organizations with GitHub clients", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result[0].Client).NotTo(BeNil())
					Expect(result[1].Client).NotTo(BeNil())
				})
			})

			Context("when one organization does not exist", func() {
				BeforeEach(func() {
					Expect(k8sClient.Create(ctx, org1)).To(Succeed())
					// org2 not created
				})

				It("should return error", func() {
					Expect(err).To(HaveOccurred())
				})
			})

			Context("when rate limit check fails", func() {
				BeforeEach(func() {
					Expect(k8sClient.Create(ctx, org1)).To(Succeed())
					Expect(k8sClient.Create(ctx, org2)).To(Succeed())
					mockClientMgr.shouldFailLimit = true
					mockClientMgr.rateLimitErr = errors.New("rate limit exceeded")
				})

				It("should return error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("rate limit exceeded"))
				})
			})
		})

		Context("when building from previous organization refs", func() {
			BeforeEach(func() {
				team.Status.PreviousOrganizationRefs = []v1alpha1.OrganizationRef{
					{Name: "org1"},
				}
			})

			JustBeforeEach(func() {
				result, err = buildGitHubOrgsSlice(ctx, factory, *team, func(t v1alpha1.Team) []v1alpha1.OrganizationRef {
					return t.Status.PreviousOrganizationRefs
				})
			})

			Context("when previous organization exists", func() {
				BeforeEach(func() {
					Expect(k8sClient.Create(ctx, org1)).To(Succeed())
				})

				It("should return slice with correct length", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(HaveLen(1))
				})

				It("should return organization with correct resource", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result[0].Resource).To(Equal("org1"))
				})
			})
		})

		Context("when extractor returns empty slice", func() {
			JustBeforeEach(func() {
				result, err = buildGitHubOrgsSlice(ctx, factory, *team, func(t v1alpha1.Team) []v1alpha1.OrganizationRef {
					return []v1alpha1.OrganizationRef{}
				})
			})

			It("should return empty slice", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeEmpty())
			})
		})
	})

	Describe("CreateFor* methods with spreading", func() {
		// mockSpreadMgr is already initialized in the outer BeforeEach
		// We just modify its behavior in nested BeforeEach blocks

		Describe("CreateForOrg with spreading", func() {
			var (
				org            *v1alpha1.Organization
				namespacedName types.NamespacedName
			)

			BeforeEach(func() {
				namespacedName = types.NamespacedName{
					Name:      defaultOrgName,
					Namespace: defaultNS,
				}

				org = &v1alpha1.Organization{
					ObjectMeta: metav1.ObjectMeta{
						Name:      defaultOrgName,
						Namespace: defaultNS,
					},
					Spec: v1alpha1.OrganizationSpec{
						Name:                    defaultOrgName,
						GitHubAppInstallationId: defaultAppID,
					},
				}
				Expect(k8sClient.Create(ctx, org)).To(Succeed())
			})

			Context("when spreading is not required", func() {
				BeforeEach(func() {
					mockSpreadMgr.shouldSpread = false
				})

				It("should create executor successfully", func() {
					executor, err := factory.CreateForOrg(ctx, namespacedName)
					Expect(err).NotTo(HaveOccurred())
					Expect(executor).NotTo(BeNil())
				})
			})

			Context("when spreading is required", func() {
				BeforeEach(func() {
					mockSpreadMgr.shouldSpread = true
					mockSpreadMgr.spreadDelay = 5 * time.Minute
				})

				It("should return RequiresSpreadError", func() {
					executor, err := factory.CreateForOrg(ctx, namespacedName)
					Expect(err).To(HaveOccurred())
					Expect(executor).To(BeNil())

					var spreadErr *spreading.RequiresSpreadError
					Expect(errors.As(err, &spreadErr)).To(BeTrue())
					Expect(spreadErr.RequeueAfter).To(Equal(5 * time.Minute))
				})
			})
		})

		Describe("CreateForRepo with spreading", func() {
			var (
				org            *v1alpha1.Organization
				repo           *v1alpha1.Repository
				namespacedName types.NamespacedName
			)

			BeforeEach(func() {
				namespacedName = types.NamespacedName{
					Name:      "test-repo",
					Namespace: defaultNS,
				}

				org = &v1alpha1.Organization{
					ObjectMeta: metav1.ObjectMeta{
						Name:      defaultOrgName,
						Namespace: defaultNS,
					},
					Spec: v1alpha1.OrganizationSpec{
						Name:                    defaultOrgName,
						GitHubAppInstallationId: defaultAppID,
					},
				}
				Expect(k8sClient.Create(ctx, org)).To(Succeed())

				repo = &v1alpha1.Repository{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-repo",
						Namespace: defaultNS,
					},
					Spec: v1alpha1.RepositorySpec{
						Name: "test-repo",
						OrganizationRef: v1alpha1.OrganizationRef{
							Name: defaultOrgName,
						},
					},
				}
				Expect(k8sClient.Create(ctx, repo)).To(Succeed())
			})

			Context("when spreading is not required", func() {
				BeforeEach(func() {
					mockSpreadMgr.shouldSpread = false
				})

				It("should create executor successfully", func() {
					executor, err := factory.CreateForRepo(ctx, namespacedName)
					Expect(err).NotTo(HaveOccurred())
					Expect(executor).NotTo(BeNil())
				})
			})

			Context("when spreading is required", func() {
				BeforeEach(func() {
					mockSpreadMgr.shouldSpread = true
					mockSpreadMgr.spreadDelay = 10 * time.Minute
				})

				It("should return RequiresSpreadError", func() {
					executor, err := factory.CreateForRepo(ctx, namespacedName)
					Expect(err).To(HaveOccurred())
					Expect(executor).To(BeNil())

					var spreadErr *spreading.RequiresSpreadError
					Expect(errors.As(err, &spreadErr)).To(BeTrue())
					Expect(spreadErr.RequeueAfter).To(Equal(10 * time.Minute))
				})
			})
		})

		Describe("CreateForTeam with spreading", func() {
			var (
				org            *v1alpha1.Organization
				team           *v1alpha1.Team
				namespacedName types.NamespacedName
			)

			BeforeEach(func() {
				namespacedName = types.NamespacedName{
					Name:      "test-team",
					Namespace: defaultNS,
				}

				org = &v1alpha1.Organization{
					ObjectMeta: metav1.ObjectMeta{
						Name:      defaultOrgName,
						Namespace: defaultNS,
					},
					Spec: v1alpha1.OrganizationSpec{
						Name:                    defaultOrgName,
						GitHubAppInstallationId: defaultAppID,
					},
				}
				Expect(k8sClient.Create(ctx, org)).To(Succeed())

				team = &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-team",
						Namespace: defaultNS,
					},
					Spec: v1alpha1.TeamSpec{
						Name: "test-team",
						OrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: defaultOrgName},
						},
						Members: []string{"user1"},
					},
				}
				Expect(k8sClient.Create(ctx, team)).To(Succeed())
			})

			Context("when spreading is not required", func() {
				BeforeEach(func() {
					mockSpreadMgr.shouldSpread = false
				})

				It("should create executor successfully", func() {
					executor, err := factory.CreateForTeam(ctx, namespacedName)
					Expect(err).NotTo(HaveOccurred())
					Expect(executor).NotTo(BeNil())
				})
			})

			Context("when spreading is required", func() {
				BeforeEach(func() {
					mockSpreadMgr.shouldSpread = true
					mockSpreadMgr.spreadDelay = 15 * time.Minute
				})

				It("should return RequiresSpreadError", func() {
					executor, err := factory.CreateForTeam(ctx, namespacedName)
					Expect(err).To(HaveOccurred())
					Expect(executor).To(BeNil())

					var spreadErr *spreading.RequiresSpreadError
					Expect(errors.As(err, &spreadErr)).To(BeTrue())
					Expect(spreadErr.RequeueAfter).To(Equal(15 * time.Minute))
				})
			})
		})
	})

	Describe("Subresource generation collection", func() {
		Describe("fetchSubResourceGenerationsForOrg", func() {
			var org *v1alpha1.Organization

			BeforeEach(func() {
				org = &v1alpha1.Organization{
					ObjectMeta: metav1.ObjectMeta{
						Name:      defaultOrgName,
						Namespace: defaultNS,
					},
					Spec: v1alpha1.OrganizationSpec{
						Name:                    defaultOrgName,
						GitHubAppInstallationId: defaultAppID,
					},
				}
			})

			Context("when organization has no subresources", func() {
				It("should return nil", func() {
					generations, err := factory.fetchSubResourceGenerationsForOrg(ctx, *org)
					Expect(err).NotTo(HaveOccurred())
					Expect(generations).To(BeNil())
				})
			})

			Context("when organization has ruleset presets", func() {
				var rulesetPreset *v1alpha1.RulesetPreset

				BeforeEach(func() {
					rulesetPreset = &v1alpha1.RulesetPreset{
						ObjectMeta: metav1.ObjectMeta{
							Name:       "test-ruleset",
							Namespace:  defaultNS,
							Generation: 5,
						},
						Spec: v1alpha1.RulesetPresetSpec{
							Name: "test-ruleset",
						},
					}
					Expect(k8sClient.Create(ctx, rulesetPreset)).To(Succeed())

					org.Spec.RulesetPresetList = []v1.LocalObjectReference{
						{Name: "test-ruleset"},
					}
				})

				It("should collect ruleset preset generation", func() {
					generations, err := factory.fetchSubResourceGenerationsForOrg(ctx, *org)
					Expect(err).NotTo(HaveOccurred())
					Expect(generations).NotTo(BeNil())
					Expect(generations).To(HaveLen(1))
					Expect(generations["/"+defaultNS+"/test-ruleset"]).To(Equal(int64(5)))
				})
			})

			Context("when organization has code security configurations", func() {
				var csc *v1alpha1.CodeSecurityConfiguration

				BeforeEach(func() {
					csc = &v1alpha1.CodeSecurityConfiguration{
						ObjectMeta: metav1.ObjectMeta{
							Name:       "test-csc",
							Namespace:  defaultNS,
							Generation: 3,
						},
						Spec: v1alpha1.CodeSecurityConfigurationSpec{
							Name:        "test-csc",
							Description: "Test CSC",
						},
					}
					Expect(k8sClient.Create(ctx, csc)).To(Succeed())

					org.Spec.CodeSecurityConfigurations = []v1alpha1.AttachableCodeSecurityConfigurationRef{
						{Name: "test-csc"},
					}
				})

				It("should collect code security configuration generation", func() {
					generations, err := factory.fetchSubResourceGenerationsForOrg(ctx, *org)
					Expect(err).NotTo(HaveOccurred())
					Expect(generations).NotTo(BeNil())
					Expect(generations).To(HaveLen(1))
					Expect(generations["/"+defaultNS+"/test-csc"]).To(Equal(int64(3)))
				})
			})

			Context("when organization has multiple subresources", func() {
				var (
					rulesetPreset1 *v1alpha1.RulesetPreset
					rulesetPreset2 *v1alpha1.RulesetPreset
					csc1           *v1alpha1.CodeSecurityConfiguration
					csc2           *v1alpha1.CodeSecurityConfiguration
				)

				BeforeEach(func() {
					rulesetPreset1 = &v1alpha1.RulesetPreset{
						ObjectMeta: metav1.ObjectMeta{
							Name:       "ruleset-1",
							Namespace:  defaultNS,
							Generation: 1,
						},
						Spec: v1alpha1.RulesetPresetSpec{
							Name: "ruleset-1",
						},
					}
					Expect(k8sClient.Create(ctx, rulesetPreset1)).To(Succeed())

					rulesetPreset2 = &v1alpha1.RulesetPreset{
						ObjectMeta: metav1.ObjectMeta{
							Name:       "ruleset-2",
							Namespace:  defaultNS,
							Generation: 1,
						},
						Spec: v1alpha1.RulesetPresetSpec{
							Name: "ruleset-2",
						},
					}
					Expect(k8sClient.Create(ctx, rulesetPreset2)).To(Succeed())

					csc1 = &v1alpha1.CodeSecurityConfiguration{
						ObjectMeta: metav1.ObjectMeta{
							Name:       "csc-1",
							Namespace:  defaultNS,
							Generation: 1,
						},
						Spec: v1alpha1.CodeSecurityConfigurationSpec{
							Name:        "csc-1",
							Description: "CSC 1",
						},
					}
					Expect(k8sClient.Create(ctx, csc1)).To(Succeed())

					csc2 = &v1alpha1.CodeSecurityConfiguration{
						ObjectMeta: metav1.ObjectMeta{
							Name:       "csc-2",
							Namespace:  defaultNS,
							Generation: 1,
						},
						Spec: v1alpha1.CodeSecurityConfigurationSpec{
							Name:        "csc-2",
							Description: "CSC 2",
						},
					}
					Expect(k8sClient.Create(ctx, csc2)).To(Succeed())

					org.Spec.RulesetPresetList = []v1.LocalObjectReference{
						{Name: "ruleset-1"},
						{Name: "ruleset-2"},
					}
					org.Spec.CodeSecurityConfigurations = []v1alpha1.AttachableCodeSecurityConfigurationRef{
						{Name: "csc-1"},
						{Name: "csc-2"},
					}
				})

				It("should collect all subresource generations", func() {
					generations, err := factory.fetchSubResourceGenerationsForOrg(ctx, *org)
					Expect(err).NotTo(HaveOccurred())
					Expect(generations).NotTo(BeNil())
					Expect(generations).To(HaveLen(4))
					Expect(generations["/"+defaultNS+"/ruleset-1"]).To(Equal(int64(1)))
					Expect(generations["/"+defaultNS+"/ruleset-2"]).To(Equal(int64(1)))
					Expect(generations["/"+defaultNS+"/csc-1"]).To(Equal(int64(1)))
					Expect(generations["/"+defaultNS+"/csc-2"]).To(Equal(int64(1)))
				})
			})

			Context("when ruleset preset does not exist", func() {
				BeforeEach(func() {
					org.Spec.RulesetPresetList = []v1.LocalObjectReference{
						{Name: "non-existent"},
					}
				})

				It("should return error", func() {
					generations, err := factory.fetchSubResourceGenerationsForOrg(ctx, *org)
					Expect(err).To(HaveOccurred())
					Expect(generations).To(BeNil())
				})
			})

			Context("when code security configuration does not exist", func() {
				BeforeEach(func() {
					org.Spec.CodeSecurityConfigurations = []v1alpha1.AttachableCodeSecurityConfigurationRef{
						{Name: "non-existent"},
					}
				})

				It("should return error", func() {
					generations, err := factory.fetchSubResourceGenerationsForOrg(ctx, *org)
					Expect(err).To(HaveOccurred())
					Expect(generations).To(BeNil())
				})
			})
		})

		Describe("fetchSubResourceGenerationsForRepo", func() {
			var repo *v1alpha1.Repository

			BeforeEach(func() {
				repo = &v1alpha1.Repository{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-repo",
						Namespace: defaultNS,
					},
					Spec: v1alpha1.RepositorySpec{
						Name: "test-repo",
						OrganizationRef: v1alpha1.OrganizationRef{
							Name: defaultOrgName,
						},
					},
				}
			})

			Context("when repository has no subresources", func() {
				It("should return nil", func() {
					generations, err := factory.fetchSubResourceGenerationsForRepo(ctx, *repo)
					Expect(err).NotTo(HaveOccurred())
					Expect(generations).To(BeNil())
				})
			})

			Context("when repository has ruleset presets", func() {
				var rulesetPreset *v1alpha1.RulesetPreset

				BeforeEach(func() {
					rulesetPreset = &v1alpha1.RulesetPreset{
						ObjectMeta: metav1.ObjectMeta{
							Name:       "repo-ruleset",
							Namespace:  defaultNS,
							Generation: 8,
						},
						Spec: v1alpha1.RulesetPresetSpec{
							Name: "repo-ruleset",
						},
					}
					Expect(k8sClient.Create(ctx, rulesetPreset)).To(Succeed())

					repo.Spec.RulesetPresetList = []v1.LocalObjectReference{
						{Name: "repo-ruleset"},
					}
				})

				It("should collect ruleset preset generation", func() {
					generations, err := factory.fetchSubResourceGenerationsForRepo(ctx, *repo)
					Expect(err).NotTo(HaveOccurred())
					Expect(generations).NotTo(BeNil())
					Expect(generations).To(HaveLen(1))
					Expect(generations["/"+defaultNS+"/repo-ruleset"]).To(Equal(int64(8)))
				})
			})

			Context("when repository has webhook presets", func() {
				var webhookPreset *v1alpha1.WebhookPreset

				BeforeEach(func() {
					webhookPreset = &v1alpha1.WebhookPreset{
						ObjectMeta: metav1.ObjectMeta{
							Name:       "test-webhook",
							Namespace:  defaultNS,
							Generation: 6,
						},
						Spec: v1alpha1.WebhookPresetSpec{
							PayloadURL: "https://example.com/webhook",
						},
					}
					Expect(k8sClient.Create(ctx, webhookPreset)).To(Succeed())

					repo.Spec.WebhookPresetList = []v1.LocalObjectReference{
						{Name: "test-webhook"},
					}
				})

				It("should collect webhook preset generation", func() {
					generations, err := factory.fetchSubResourceGenerationsForRepo(ctx, *repo)
					Expect(err).NotTo(HaveOccurred())
					Expect(generations).NotTo(BeNil())
					Expect(generations).To(HaveLen(1))
					Expect(generations["/"+defaultNS+"/test-webhook"]).To(Equal(int64(6)))
				})
			})

			Context("when repository has attached code security configuration", func() {
				var csc *v1alpha1.CodeSecurityConfiguration

				BeforeEach(func() {
					csc = &v1alpha1.CodeSecurityConfiguration{
						ObjectMeta: metav1.ObjectMeta{
							Name:       "repo-csc",
							Namespace:  defaultNS,
							Generation: 9,
						},
						Spec: v1alpha1.CodeSecurityConfigurationSpec{
							Name:        "repo-csc",
							Description: "Repo CSC",
						},
					}
					Expect(k8sClient.Create(ctx, csc)).To(Succeed())

					repo.Spec.AttachedCodeSecurityConfiguration = &v1alpha1.CodeSecurityConfigurationRef{
						Name: "repo-csc",
					}
				})

				It("should collect attached code security configuration generation", func() {
					generations, err := factory.fetchSubResourceGenerationsForRepo(ctx, *repo)
					Expect(err).NotTo(HaveOccurred())
					Expect(generations).NotTo(BeNil())
					Expect(generations).To(HaveLen(1))
					Expect(generations["/"+defaultNS+"/repo-csc"]).To(Equal(int64(9)))
				})
			})

			Context("when repository has multiple subresources", func() {
				var (
					rulesetPreset  *v1alpha1.RulesetPreset
					webhookPreset1 *v1alpha1.WebhookPreset
					webhookPreset2 *v1alpha1.WebhookPreset
					csc            *v1alpha1.CodeSecurityConfiguration
				)

				BeforeEach(func() {
					rulesetPreset = &v1alpha1.RulesetPreset{
						ObjectMeta: metav1.ObjectMeta{
							Name:       "repo-ruleset",
							Namespace:  defaultNS,
							Generation: 11,
						},
						Spec: v1alpha1.RulesetPresetSpec{
							Name: "repo-ruleset",
						},
					}
					Expect(k8sClient.Create(ctx, rulesetPreset)).To(Succeed())

					webhookPreset1 = &v1alpha1.WebhookPreset{
						ObjectMeta: metav1.ObjectMeta{
							Name:       "webhook-1",
							Namespace:  defaultNS,
							Generation: 13,
						},
						Spec: v1alpha1.WebhookPresetSpec{
							PayloadURL: "https://example.com/webhook1",
						},
					}
					Expect(k8sClient.Create(ctx, webhookPreset1)).To(Succeed())

					webhookPreset2 = &v1alpha1.WebhookPreset{
						ObjectMeta: metav1.ObjectMeta{
							Name:       "webhook-2",
							Namespace:  defaultNS,
							Generation: 14,
						},
						Spec: v1alpha1.WebhookPresetSpec{
							PayloadURL: "https://example.com/webhook2",
						},
					}
					Expect(k8sClient.Create(ctx, webhookPreset2)).To(Succeed())

					csc = &v1alpha1.CodeSecurityConfiguration{
						ObjectMeta: metav1.ObjectMeta{
							Name:       "repo-csc",
							Namespace:  defaultNS,
							Generation: 12,
						},
						Spec: v1alpha1.CodeSecurityConfigurationSpec{
							Name:        "repo-csc",
							Description: "Repo CSC",
						},
					}
					Expect(k8sClient.Create(ctx, csc)).To(Succeed())

					repo.Spec.RulesetPresetList = []v1.LocalObjectReference{
						{Name: "repo-ruleset"},
					}
					repo.Spec.WebhookPresetList = []v1.LocalObjectReference{
						{Name: "webhook-1"},
						{Name: "webhook-2"},
					}
					repo.Spec.AttachedCodeSecurityConfiguration = &v1alpha1.CodeSecurityConfigurationRef{
						Name: "repo-csc",
					}
				})

				It("should collect all subresource generations", func() {
					generations, err := factory.fetchSubResourceGenerationsForRepo(ctx, *repo)
					Expect(err).NotTo(HaveOccurred())
					Expect(generations).NotTo(BeNil())
					Expect(generations).To(HaveLen(4))
					Expect(generations["/"+defaultNS+"/repo-ruleset"]).To(Equal(int64(11)))
					Expect(generations["/"+defaultNS+"/webhook-1"]).To(Equal(int64(13)))
					Expect(generations["/"+defaultNS+"/webhook-2"]).To(Equal(int64(14)))
					Expect(generations["/"+defaultNS+"/repo-csc"]).To(Equal(int64(12)))
				})
			})

			Context("when ruleset preset does not exist", func() {
				BeforeEach(func() {
					repo.Spec.RulesetPresetList = []v1.LocalObjectReference{
						{Name: "non-existent"},
					}
				})

				It("should return error", func() {
					generations, err := factory.fetchSubResourceGenerationsForRepo(ctx, *repo)
					Expect(err).To(HaveOccurred())
					Expect(generations).To(BeNil())
				})
			})

			Context("when webhook preset does not exist", func() {
				BeforeEach(func() {
					repo.Spec.WebhookPresetList = []v1.LocalObjectReference{
						{Name: "non-existent"},
					}
				})

				It("should return error", func() {
					generations, err := factory.fetchSubResourceGenerationsForRepo(ctx, *repo)
					Expect(err).To(HaveOccurred())
					Expect(generations).To(BeNil())
				})
			})

			Context("when attached code security configuration does not exist", func() {
				BeforeEach(func() {
					repo.Spec.AttachedCodeSecurityConfiguration = &v1alpha1.CodeSecurityConfigurationRef{
						Name: "non-existent",
					}
				})

				It("should return error", func() {
					generations, err := factory.fetchSubResourceGenerationsForRepo(ctx, *repo)
					Expect(err).To(HaveOccurred())
					Expect(generations).To(BeNil())
				})
			})
		})

		Describe("Integration with CreateForOrg", func() {
			var (
				org            *v1alpha1.Organization
				rulesetPreset  *v1alpha1.RulesetPreset
				csc            *v1alpha1.CodeSecurityConfiguration
				namespacedName types.NamespacedName
			)

			BeforeEach(func() {
				namespacedName = types.NamespacedName{
					Name:      defaultOrgName,
					Namespace: defaultNS,
				}

				rulesetPreset = &v1alpha1.RulesetPreset{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "integration-ruleset",
						Namespace:  defaultNS,
						Generation: 15,
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name: "integration-ruleset",
					},
				}
				Expect(k8sClient.Create(ctx, rulesetPreset)).To(Succeed())

				csc = &v1alpha1.CodeSecurityConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "integration-csc",
						Namespace:  defaultNS,
						Generation: 16,
					},
					Spec: v1alpha1.CodeSecurityConfigurationSpec{
						Name:        "integration-csc",
						Description: "Integration CSC",
					},
				}
				Expect(k8sClient.Create(ctx, csc)).To(Succeed())

				org = &v1alpha1.Organization{
					ObjectMeta: metav1.ObjectMeta{
						Name:      defaultOrgName,
						Namespace: defaultNS,
					},
					Spec: v1alpha1.OrganizationSpec{
						Name:                    defaultOrgName,
						GitHubAppInstallationId: defaultAppID,
						RulesetPresetList: []v1.LocalObjectReference{
							{Name: "integration-ruleset"},
						},
						CodeSecurityConfigurations: []v1alpha1.AttachableCodeSecurityConfigurationRef{
							{Name: "integration-csc"},
						},
					},
				}
				Expect(k8sClient.Create(ctx, org)).To(Succeed())

				mockSpreadMgr.shouldSpread = false
			})

			It("should pass collected generations to reconciler", func() {
				executor, err := factory.CreateForOrg(ctx, namespacedName)
				Expect(err).NotTo(HaveOccurred())
				Expect(executor).NotTo(BeNil())

				orgRec := executor.Reconciler.(*orgrec.GitHubOrgReconciler)
				Expect(orgRec.Kubernetes.CurrentSubResourceGenerations).NotTo(BeNil())
				Expect(orgRec.Kubernetes.CurrentSubResourceGenerations).To(HaveLen(2))
				Expect(orgRec.Kubernetes.CurrentSubResourceGenerations["/"+defaultNS+"/integration-ruleset"]).To(Equal(int64(15)))
				Expect(orgRec.Kubernetes.CurrentSubResourceGenerations["/"+defaultNS+"/integration-csc"]).To(Equal(int64(16)))
			})

			It("should pass collected generations to spreading manager", func() {
				mockSpreadMgr.captureGenerations = true
				executor, err := factory.CreateForOrg(ctx, namespacedName)
				Expect(err).NotTo(HaveOccurred())
				Expect(executor).NotTo(BeNil())

				Expect(mockSpreadMgr.lastGenerations).NotTo(BeNil())
				Expect(mockSpreadMgr.lastGenerations).To(HaveLen(2))
				Expect(mockSpreadMgr.lastGenerations["/"+defaultNS+"/integration-ruleset"]).To(Equal(int64(15)))
				Expect(mockSpreadMgr.lastGenerations["/"+defaultNS+"/integration-csc"]).To(Equal(int64(16)))
			})
		})

		Describe("Integration with CreateForRepo", func() {
			var (
				org            *v1alpha1.Organization
				repo           *v1alpha1.Repository
				rulesetPreset  *v1alpha1.RulesetPreset
				webhookPreset  *v1alpha1.WebhookPreset
				csc            *v1alpha1.CodeSecurityConfiguration
				namespacedName types.NamespacedName
			)

			BeforeEach(func() {
				namespacedName = types.NamespacedName{
					Name:      "integration-repo",
					Namespace: defaultNS,
				}

				org = &v1alpha1.Organization{
					ObjectMeta: metav1.ObjectMeta{
						Name:       defaultOrgName,
						Namespace:  defaultNS,
						Generation: 1,
					},
					Spec: v1alpha1.OrganizationSpec{
						Name:                    defaultOrgName,
						GitHubAppInstallationId: defaultAppID,
					},
				}
				Expect(k8sClient.Create(ctx, org)).To(Succeed())

				rulesetPreset = &v1alpha1.RulesetPreset{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "integration-repo-ruleset",
						Namespace:  defaultNS,
						Generation: 1,
					},
					Spec: v1alpha1.RulesetPresetSpec{
						Name: "integration-repo-ruleset",
					},
				}
				Expect(k8sClient.Create(ctx, rulesetPreset)).To(Succeed())

				webhookPreset = &v1alpha1.WebhookPreset{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "integration-webhook",
						Namespace:  defaultNS,
						Generation: 1,
					},
					Spec: v1alpha1.WebhookPresetSpec{
						PayloadURL: "https://example.com/integration",
					},
				}
				Expect(k8sClient.Create(ctx, webhookPreset)).To(Succeed())

				csc = &v1alpha1.CodeSecurityConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "integration-repo-csc",
						Namespace:  defaultNS,
						Generation: 1,
					},
					Spec: v1alpha1.CodeSecurityConfigurationSpec{
						Name:        "integration-repo-csc",
						Description: "Integration Repo CSC",
					},
				}
				Expect(k8sClient.Create(ctx, csc)).To(Succeed())

				repo = &v1alpha1.Repository{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "integration-repo",
						Namespace:  defaultNS,
						Generation: 1,
					},
					Spec: v1alpha1.RepositorySpec{
						Name: "integration-repo",
						OrganizationRef: v1alpha1.OrganizationRef{
							Name: defaultOrgName,
						},
						RulesetPresetList: []v1.LocalObjectReference{
							{Name: "integration-repo-ruleset"},
						},
						WebhookPresetList: []v1.LocalObjectReference{
							{Name: "integration-webhook"},
						},
						AttachedCodeSecurityConfiguration: &v1alpha1.CodeSecurityConfigurationRef{
							Name: "integration-repo-csc",
						},
					},
				}
				Expect(k8sClient.Create(ctx, repo)).To(Succeed())

				mockSpreadMgr.shouldSpread = false
			})

			It("should pass collected generations to reconciler", func() {
				executor, err := factory.CreateForRepo(ctx, namespacedName)
				Expect(err).NotTo(HaveOccurred())
				Expect(executor).NotTo(BeNil())

				repoRec := executor.Reconciler.(*reporec.GitHubRepoReconciler)
				Expect(repoRec.Kubernetes.CurrentSubResourceGenerations).NotTo(BeNil())
				Expect(repoRec.Kubernetes.CurrentSubResourceGenerations).To(HaveLen(3))
				Expect(repoRec.Kubernetes.CurrentSubResourceGenerations["/"+defaultNS+"/integration-repo-ruleset"]).To(Equal(int64(1)))
				Expect(repoRec.Kubernetes.CurrentSubResourceGenerations["/"+defaultNS+"/integration-webhook"]).To(Equal(int64(1)))
				Expect(repoRec.Kubernetes.CurrentSubResourceGenerations["/"+defaultNS+"/integration-repo-csc"]).To(Equal(int64(1)))
			})

			It("should pass collected generations to spreading manager", func() {
				mockSpreadMgr.captureGenerations = true
				executor, err := factory.CreateForRepo(ctx, namespacedName)
				Expect(err).NotTo(HaveOccurred())
				Expect(executor).NotTo(BeNil())

				Expect(mockSpreadMgr.lastGenerations).NotTo(BeNil())
				Expect(mockSpreadMgr.lastGenerations).To(HaveLen(3))
				Expect(mockSpreadMgr.lastGenerations["/"+defaultNS+"/integration-repo-ruleset"]).To(Equal(int64(1)))
				Expect(mockSpreadMgr.lastGenerations["/"+defaultNS+"/integration-webhook"]).To(Equal(int64(1)))
				Expect(mockSpreadMgr.lastGenerations["/"+defaultNS+"/integration-repo-csc"]).To(Equal(int64(1)))
			})
		})

		Describe("Integration with CreateForTeam", func() {
			var (
				org            *v1alpha1.Organization
				team           *v1alpha1.Team
				namespacedName types.NamespacedName
			)

			BeforeEach(func() {
				namespacedName = types.NamespacedName{
					Name:      "integration-team",
					Namespace: defaultNS,
				}

				org = &v1alpha1.Organization{
					ObjectMeta: metav1.ObjectMeta{
						Name:      defaultOrgName,
						Namespace: defaultNS,
					},
					Spec: v1alpha1.OrganizationSpec{
						Name:                    defaultOrgName,
						GitHubAppInstallationId: defaultAppID,
					},
				}
				Expect(k8sClient.Create(ctx, org)).To(Succeed())

				team = &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "integration-team",
						Namespace: defaultNS,
					},
					Spec: v1alpha1.TeamSpec{
						Name: "integration-team",
						OrganizationRefs: []v1alpha1.OrganizationRef{
							{Name: defaultOrgName},
						},
						Members: []string{"user1"},
					},
				}
				Expect(k8sClient.Create(ctx, team)).To(Succeed())

				mockSpreadMgr.shouldSpread = false
			})

			It("should pass nil generations to spreading manager for teams", func() {
				mockSpreadMgr.captureGenerations = true
				executor, err := factory.CreateForTeam(ctx, namespacedName)
				Expect(err).NotTo(HaveOccurred())
				Expect(executor).NotTo(BeNil())

				Expect(mockSpreadMgr.lastGenerations).To(BeNil())
			})
		})
	})
})

// mockSpreadManager implements reconciler.SpreadManager for testing
type mockSpreadManager struct {
	shouldSpread       bool
	spreadDelay        time.Duration
	captureGenerations bool
	lastGenerations    map[string]int64
}

func (m *mockSpreadManager) Spread(_ context.Context, _ spreading.SpreadableResource, generations map[string]int64) error {
	if m == nil {
		return nil
	}
	if m.captureGenerations {
		m.lastGenerations = generations
	}
	if m.shouldSpread {
		return &spreading.RequiresSpreadError{
			RequeueAfter: m.spreadDelay,
		}
	}
	return nil
}

// mockGitHubClientManager is a mock implementation of reconciler.GitHubClientManager for testing
type mockGitHubClientManager struct {
	client          *ghclientmock.MockGitHubClientWrapper
	clientByOrg     map[string]*ghclientmock.MockGitHubClientWrapper
	shouldFailLimit bool
	shouldFail      bool
	rateLimitErr    error
	genericErr      error
	callCount       int
	lastOrgName     string
	lastAppID       int64
	lastRateLimit   int
}

func (m *mockGitHubClientManager) GetClient(_ context.Context, orgName string, appInstallationID int64) (ghclient.GitHubClient, error) {
	m.callCount++
	m.lastOrgName = orgName
	m.lastAppID = appInstallationID

	if m.shouldFail {
		return nil, m.genericErr
	}

	// Return org-specific client if configured
	if m.clientByOrg != nil {
		if ghClient, ok := m.clientByOrg[orgName]; ok {
			return ghClient, nil
		}
	}

	return m.client, nil
}

func (m *mockGitHubClientManager) GetGitHubClientAndCheckRateLimit(_ context.Context, orgName string, appInstallationID int64, rateLimitMinimum int) (ghclient.GitHubClient, error) {
	m.callCount++
	m.lastOrgName = orgName
	m.lastAppID = appInstallationID
	m.lastRateLimit = rateLimitMinimum

	if m.shouldFailLimit {
		return nil, m.rateLimitErr
	}

	if m.shouldFail {
		return nil, m.genericErr
	}

	// Return org-specific client if configured
	if m.clientByOrg != nil {
		if ghClient, ok := m.clientByOrg[orgName]; ok {
			return ghClient, nil
		}
	}

	return m.client, nil
}
