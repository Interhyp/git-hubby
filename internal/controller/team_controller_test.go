package controller

import (
	"context"
	"fmt"
	"net/http"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/config"
	"github.com/Interhyp/git-hubby/internal/reconciler/reconcilerfactory"
	"github.com/Interhyp/git-hubby/test/mock"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	"github.com/google/go-github/v89/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("TeamController", func() {
	var (
		testEnv    *TestEnvironment
		mockClient *ghclientmock.MockGitHubClientWrapper
		factory    *reconcilerfactory.Factory
	)

	const (
		teamName      = "test-team"
		orgName       = "test-org"
		secondOrgName = "second-test-org"
		namespaceName = "test-namespace"
	)

	BeforeEach(func() {
		testEnv = SetupTestEnvironment()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()
		factory = &reconcilerfactory.Factory{
			ClientManager:    ghclientmock.NewGitHubMockClientFactory(mockClient),
			K8sClient:        testEnv.Client,
			SpreadingManager: &mock.NoOpSpreadManager{},
			Config: config.Config{
				GitHubMemberSuffix: "_memberSuffix",
			},
		}
		testEnv.CreateTestNamespace(namespaceName)
		_ = testEnv.SetupOrganizationTest(nil, namespaceName, orgName)
	})

	AfterEach(func() {
		testEnv.TeardownTestEnvironment()
	})

	Context("When reconciling a team resource", func() {
		var (
			team           *githubv1alpha1.Team
			namespacedName types.NamespacedName
		)

		BeforeEach(func() {
			namespacedName = types.NamespacedName{
				Name:      teamName,
				Namespace: namespaceName,
			}
		})

		AfterEach(func() {
			if team != nil {
				testEnv.CleanupTestResources(team)
			}
		})

		Context("End-to-end team reconciliation", func() {
			BeforeEach(func() {
				team = testEnv.SetupTeamTest(nil, namespaceName, teamName, nil, []githubv1alpha1.OrganizationRef{{Name: orgName}})
			})

			It("should successfully create and reconcile a team", func() {
				By("Setting up mock to return 404 for team not found")
				mockClient.SetTeamNotFound([]string{orgName}, teamName)
				mockClient.CreateTeamFunc = func(ctx context.Context, org string, team *github.NewTeam) (*github.Team, error) {
					return &github.Team{
						Name: github.Ptr(teamName),
						Slug: github.Ptr(teamName),
					}, nil
				}

				By("Creating reconciler from factory and reconciling")
				teamReconciler, err := factory.CreateForTeam(ctx, namespacedName)
				Expect(err).NotTo(HaveOccurred())
				err = teamReconciler.Reconcile(testEnv.Context)
				Expect(err).NotTo(HaveOccurred())

				By("Verifying finalizer was added")
				updatedTeam := &githubv1alpha1.Team{}
				err = testEnv.Client.Get(testEnv.Context, namespacedName, updatedTeam)
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedTeam.Finalizers).To(ContainElement("team.github.interhyp.de/finalizer"))
			})
		})

		Context("Multi-organization team reconciliation", func() {
			BeforeEach(func() {
				_ = testEnv.SetupOrganizationTest(nil, namespaceName, secondOrgName)
				team = testEnv.SetupTeamTest(nil, namespaceName, teamName, nil, []githubv1alpha1.OrganizationRef{{Name: orgName}, {Name: secondOrgName}})
			})

			It("should reconcile team across multiple organizations", func() {
				By("Setting up mock for multi-org team creation")
				mockClient.SetTeamNotFound([]string{orgName, secondOrgName}, teamName)
				mockClient.CreateTeamFunc = func(ctx context.Context, org string, team *github.NewTeam) (*github.Team, error) {
					return &github.Team{
						Name: github.Ptr(teamName),
						Slug: github.Ptr(teamName),
					}, nil
				}

				By("Reconciling the team")
				teamReconciler, err := factory.CreateForTeam(ctx, namespacedName)
				Expect(err).NotTo(HaveOccurred())
				err = teamReconciler.Reconcile(testEnv.Context)
				Expect(err).NotTo(HaveOccurred())

				By("Verifying both organizations were processed")
				Expect(mockClient.GetTeamCalls()).To(HaveLen(6))
			})
		})

		Context("Team deletion", func() {
			BeforeEach(func() {
				team = testEnv.SetupTeamTest(nil, namespaceName, teamName, github.Ptr(teamName), []githubv1alpha1.OrganizationRef{{Name: orgName}})
				team.Finalizers = []string{"team.github.interhyp.de/finalizer"}
				Expect(testEnv.Client.Update(testEnv.Context, team)).To(Succeed())
				Expect(testEnv.Client.Delete(testEnv.Context, team)).To(Succeed())
			})

			It("should delete team from GitHub when K8s resource is deleted", func() {
				By("Setting up mock to return existing team")
				mockClient.GetTeamBySlugFunc = func(ctx context.Context, owner, team string) (*github.Team, error) {
					return &github.Team{
						Name:                github.Ptr(teamName),
						Slug:                github.Ptr(teamName),
						Description:         new(""),
						Privacy:             new("closed"),
						Permission:          new("pull"),
						NotificationSetting: new("notifications_disabled"),
					}, nil
				}

				By("Reconciling the deletion")
				teamReconciler, err := factory.CreateForTeam(ctx, namespacedName)
				Expect(err).NotTo(HaveOccurred())
				err = teamReconciler.Reconcile(testEnv.Context)
				Expect(err).NotTo(HaveOccurred())

				By("Verifying team was deleted from GitHub")
				Expect(mockClient.GetTeamCalls()).To(ContainElement(HaveField("Method", Equal("DeleteTeamBySlug"))))
			})
		})

		Context("Team member management", func() {
			BeforeEach(func() {
				team = testEnv.SetupTeamWithMembersTest(nil, namespaceName, teamName, orgName)
			})

			It("should add and remove members correctly", func() {
				By("Setting up mock for member reconciliation")
				mockClient.GetTeamBySlugFunc = func(ctx context.Context, owner, team string) (*github.Team, error) {
					return &github.Team{
						Name:                github.Ptr(teamName),
						Slug:                github.Ptr(teamName),
						Description:         new(""),
						Privacy:             new("closed"),
						Permission:          new("pull"),
						NotificationSetting: new("notifications_disabled"),
					}, nil
				}
				mockClient.ListMembersFunc = func(ctx context.Context, org string) ([]*github.User, error) {
					return []*github.User{
						{Login: new("new-member_memberSuffix")},
						{Login: new("existing-member_memberSuffix")},
					}, nil
				}
				mockClient.GetAllTeamMembersFunc = func(ctx context.Context, org string, slug string) ([]*github.User, error) {
					return []*github.User{
						{Login: new("existing-member_memberSuffix")},
					}, nil
				}

				By("Reconciling the team members")
				teamReconciler, err := factory.CreateForTeam(ctx, namespacedName)
				Expect(err).NotTo(HaveOccurred())
				err = teamReconciler.Reconcile(testEnv.Context)
				Expect(err).NotTo(HaveOccurred())

				By("Verifying member was added")
				Expect(mockClient.GetTeamMemberCalls()).To(ContainElement(HaveField("Method", Equal("AddTeamMember"))))
				Expect(mockClient.GetTeamMemberCalls()).To(ContainElement(HaveField("Username", Equal("new-member_memberSuffix"))))
			})
		})

		Context("Error handling", func() {
			BeforeEach(func() {
				team = testEnv.SetupTeamTest(nil, namespaceName, teamName, github.Ptr(teamName), []githubv1alpha1.OrganizationRef{{Name: orgName}})
			})

			It("should handle GitHub API errors gracefully", func() {
				By("Setting up mock to return errors")
				mockClient.SetError(fmt.Errorf("GitHub API unavailable"))

				By("Attempting reconciliation")
				teamReconciler, err := factory.CreateForTeam(ctx, namespacedName)
				Expect(err).NotTo(HaveOccurred())
				err = teamReconciler.Reconcile(testEnv.Context)

				By("Verifying error is propagated")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("GitHub API unavailable"))
			})

			It("should handle rate limiting errors", func() {
				By("Setting up mock to return rate limit error")
				mockClient.GetTeamBySlugFunc = func(ctx context.Context, owner, team string) (*github.Team, error) {
					return nil, &github.RateLimitError{
						Response: &http.Response{
							StatusCode: http.StatusTooManyRequests,
							Request:    &http.Request{},
						},
						Message: "expected Rate Limit Error",
					}
				}

				By("Attempting reconciliation")
				teamReconciler, err := factory.CreateForTeam(ctx, namespacedName)
				Expect(err).NotTo(HaveOccurred())
				err = teamReconciler.Reconcile(testEnv.Context)

				By("Verifying error is propagated")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("429 expected Rate Limit Error"))
			})
		})
	})
})
