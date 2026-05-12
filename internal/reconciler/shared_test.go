package reconciler

import (
	"context"
	"errors"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	"github.com/google/go-github/v86/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

var _ = Describe("ResolveNamesToIDsInRuleset", func() {
	var (
		ctx          context.Context
		mockClient   *ghclientmock.MockGitHubClientWrapper
		orgName      string
		rulesetInput v1alpha1.RulesetPreset
		result       v1alpha1.RulesetPreset
		err          error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()
		orgName = "test-org"

		// Base ruleset preset
		rulesetInput = v1alpha1.RulesetPreset{
			Spec: v1alpha1.RulesetPresetSpec{
				Name: "test-ruleset",
				Conditions: &v1alpha1.RulesetConditions{
					RefName: &v1alpha1.RefNameCondition{
						Include: []string{"refs/heads/main"},
					},
				},
				Enforcement: "active",
				Rules: v1alpha1.RulesetRules{
					Creation: github.Ptr(true),
				},
			},
		}
	})

	JustBeforeEach(func() {
		result, err = ResolveNamesToIDsInRuleset(ctx, mockClient, orgName, rulesetInput)
	})

	Context("when ruleset has no bypass actors and no status checks", func() {
		BeforeEach(func() {
			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{}, nil
			}
		})

		It("should succeed without modifications", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.BypassActors).To(BeEmpty())
			Expect(result.Spec.Rules.RequiredStatusChecks).To(BeNil())
		})

		It("should call GetGitHubAppsInstallations once", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(mockClient.EnterpriseAppsCalls).To(HaveLen(1))
			Expect(mockClient.EnterpriseAppsCalls[0].Method).To(Equal("GetGitHubAppsInstallations"))
			Expect(mockClient.EnterpriseAppsCalls[0].Org).To(Equal(orgName))
		})
	})

	Context("when GetGitHubAppsInstallations returns an error", func() {
		BeforeEach(func() {
			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return nil, errors.New("installation fetch failed")
			}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("installation fetch failed"))
		})
	})

	Context("when bypass actor has ActorID", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorID:    ptr.To(int64(12345)),
					ActorType:  "Team",
					BypassMode: "always",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{}, nil
			}
		})

		It("should preserve the ActorID without resolution", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.BypassActors).To(HaveLen(1))
			Expect(result.Spec.BypassActors[0].ActorID).To(Equal(ptr.To(int64(12345))))
			Expect(result.Spec.BypassActors[0].ActorType).To(Equal("Team"))
		})

		It("should not call GetTeamBySlug", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(mockClient.TeamCalls).To(BeEmpty())
		})
	})

	Context("when bypass actor has Team ActorSlug", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorSlug:  ptr.To("engineering-team"),
					ActorType:  "Team",
					BypassMode: "always",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{}, nil
			}

			mockClient.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				Expect(org).To(Equal(orgName))
				Expect(slug).To(Equal("engineering-team"))
				return &github.Team{
					ID:   ptr.To(int64(98765)),
					Slug: ptr.To("engineering-team"),
					Name: ptr.To("Engineering Team"),
				}, nil
			}
		})

		It("should resolve the team slug to ActorID", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.BypassActors).To(HaveLen(1))
			Expect(result.Spec.BypassActors[0].ActorID).To(Equal(ptr.To(int64(98765))))
			Expect(result.Spec.BypassActors[0].ActorType).To(Equal("Team"))
		})

		It("should call GetTeamBySlug once", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(mockClient.TeamCalls).To(HaveLen(1))
			Expect(mockClient.TeamCalls[0].Method).To(Equal("GetTeamBySlug"))
			Expect(mockClient.TeamCalls[0].Org).To(Equal(orgName))
			Expect(mockClient.TeamCalls[0].Slug).To(Equal("engineering-team"))
		})
	})

	Context("when GetTeamBySlug returns an error", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorSlug:  ptr.To("nonexistent-team"),
					ActorType:  "Team",
					BypassMode: "always",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{}, nil
			}

			mockClient.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				return nil, errors.New("team not found")
			}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("team not found"))
		})
	})

	Context("when bypass actor has Integration ActorSlug", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorSlug:  ptr.To("github-actions"),
					ActorType:  "Integration",
					BypassMode: "pull_request",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{
					{
						ID:      ptr.To(int64(1)),
						AppID:   ptr.To(int64(15368)),
						AppSlug: ptr.To("github-actions"),
					},
					{
						ID:      ptr.To(int64(2)),
						AppID:   ptr.To(int64(99999)),
						AppSlug: ptr.To("other-app"),
					},
				}, nil
			}
		})

		It("should resolve the app slug to ActorID", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.BypassActors).To(HaveLen(1))
			Expect(result.Spec.BypassActors[0].ActorID).To(Equal(ptr.To(int64(15368))))
			Expect(result.Spec.BypassActors[0].ActorType).To(Equal("Integration"))
		})
	})

	Context("when bypass actor has Integration ActorSlug that doesn't exist", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorSlug:  ptr.To("nonexistent-app"),
					ActorType:  "Integration",
					BypassMode: "always",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{
					{
						ID:      ptr.To(int64(1)),
						AppID:   ptr.To(int64(15368)),
						AppSlug: ptr.To("github-actions"),
					},
				}, nil
			}
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no GitHub App with slug nonexistent-app installed"))
			Expect(err.Error()).To(ContainSubstring(orgName))
		})
	})

	Context("when bypass actor has RepositoryRole ActorSlug", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorSlug:  ptr.To("maintain"),
					ActorType:  "RepositoryRole",
					BypassMode: "always",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{}, nil
			}

			mockClient.GetRoleByNameFunc = func(ctx context.Context, org string, roleName string) (*github.CustomOrgRole, error) {
				Expect(org).To(Equal(orgName))
				Expect(roleName).To(Equal("maintain"))
				return &github.CustomOrgRole{
					ID:   ptr.To(int64(54321)),
					Name: ptr.To("maintain"),
				}, nil
			}
		})

		It("should resolve the role slug to ActorID", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.BypassActors).To(HaveLen(1))
			Expect(result.Spec.BypassActors[0].ActorID).To(Equal(ptr.To(int64(54321))))
			Expect(result.Spec.BypassActors[0].ActorType).To(Equal("RepositoryRole"))
		})

		It("should call GetRoleByName once", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(mockClient.RoleCalls).To(HaveLen(1))
			Expect(mockClient.RoleCalls[0].Method).To(Equal("GetRoleByName"))
			Expect(mockClient.RoleCalls[0].Org).To(Equal(orgName))
			Expect(mockClient.RoleCalls[0].RoleName).To(Equal("maintain"))
		})
	})

	Context("when GetRoleByName returns an error", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorSlug:  ptr.To("nonexistent-role"),
					ActorType:  "RepositoryRole",
					BypassMode: "always",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{}, nil
			}

			mockClient.GetRoleByNameFunc = func(ctx context.Context, org string, roleName string) (*github.CustomOrgRole, error) {
				return nil, errors.New("role not found")
			}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("role not found"))
		})
	})

	Context("when bypass actor has OrganizationAdmin type with slug", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorSlug:  ptr.To("some-slug"),
					ActorType:  "OrganizationAdmin",
					BypassMode: "always",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{}, nil
			}
		})

		It("should set ActorID to nil for OrganizationAdmin type", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.BypassActors).To(HaveLen(1))
			Expect(result.Spec.BypassActors[0].ActorID).To(BeNil())
			Expect(result.Spec.BypassActors[0].ActorType).To(Equal("OrganizationAdmin"))
		})
	})

	Context("when bypass actor is DeployKey with no ActorID or ActorSlug", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorType:  "DeployKey",
					BypassMode: "always",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{}, nil
			}
		})

		It("should preserve the DeployKey bypass actor without ActorID or ActorSlug", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.BypassActors).To(HaveLen(1))
			Expect(result.Spec.BypassActors[0].ActorID).To(BeNil())
			Expect(result.Spec.BypassActors[0].ActorSlug).To(BeNil())
			Expect(result.Spec.BypassActors[0].ActorType).To(Equal("DeployKey"))
			Expect(result.Spec.BypassActors[0].BypassMode).To(Equal("always"))
		})
	})

	Context("when bypass actor is DeployKey with any ActorID", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorType:  "DeployKey",
					BypassMode: "always",
					ActorID:    github.Ptr(int64(12354)),
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{}, nil
			}
		})

		It("should preserve the DeployKey bypass actor but without ActorID or ActorSlug", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.BypassActors).To(HaveLen(1))
			Expect(result.Spec.BypassActors[0].ActorID).To(BeNil())
			Expect(result.Spec.BypassActors[0].ActorSlug).To(BeNil())
			Expect(result.Spec.BypassActors[0].ActorType).To(Equal("DeployKey"))
			Expect(result.Spec.BypassActors[0].BypassMode).To(Equal("always"))
		})
	})

	Context("when bypass actor is EnterpriseOwner with no ActorID or ActorSlug", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorType:  "EnterpriseOwner",
					BypassMode: "always",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{}, nil
			}
		})

		It("should preserve the EnterpriseOwner bypass actor without ActorID or ActorSlug", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.BypassActors).To(HaveLen(1))
			Expect(result.Spec.BypassActors[0].ActorID).To(BeNil())
			Expect(result.Spec.BypassActors[0].ActorSlug).To(BeNil())
			Expect(result.Spec.BypassActors[0].ActorType).To(Equal("EnterpriseOwner"))
			Expect(result.Spec.BypassActors[0].BypassMode).To(Equal("always"))
		})
	})

	Context("when bypass actor is EnterpriseOwner with any ActorID", func() {
		actorID := github.Ptr(int64(84354))
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorType:  "EnterpriseOwner",
					BypassMode: "always",
					ActorID:    actorID,
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{}, nil
			}
		})

		It("should preserve the EnterpriseOwner bypass actor while keeping the ActorID", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.BypassActors).To(HaveLen(1))
			Expect(result.Spec.BypassActors[0].ActorID).To(Equal(actorID))
			Expect(result.Spec.BypassActors[0].ActorSlug).To(BeNil())
			Expect(result.Spec.BypassActors[0].ActorType).To(Equal("EnterpriseOwner"))
			Expect(result.Spec.BypassActors[0].BypassMode).To(Equal("always"))
		})
	})

	Context("when bypass actor is OrganizationAdmin with no ActorID or ActorSlug", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorType:  "OrganizationAdmin",
					BypassMode: "always",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{}, nil
			}
		})

		It("should preserve the OrganizationAdmin bypass actor without ActorID or ActorSlug", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.BypassActors).To(HaveLen(1))
			Expect(result.Spec.BypassActors[0].ActorID).To(BeNil())
			Expect(result.Spec.BypassActors[0].ActorSlug).To(BeNil())
			Expect(result.Spec.BypassActors[0].ActorType).To(Equal("OrganizationAdmin"))
			Expect(result.Spec.BypassActors[0].BypassMode).To(Equal("always"))
		})
	})

	Context("when Team bypass actor has both ActorID and ActorSlug nil", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorType:  "Team",
					BypassMode: "always",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{}, nil
			}
		})

		It("should return validation error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("bypass actor with type Team requires either actor_id or actor_slug to be set"))
		})
	})

	Context("when Integration bypass actor has both ActorID and ActorSlug nil", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorType:  "Integration",
					BypassMode: "always",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{}, nil
			}
		})

		It("should return validation error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("bypass actor with type Integration requires either actor_id or actor_slug to be set"))
		})
	})

	Context("when RepositoryRole bypass actor has both ActorID and ActorSlug nil", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorType:  "RepositoryRole",
					BypassMode: "always",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{}, nil
			}
		})

		It("should return validation error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("bypass actor with type RepositoryRole requires either actor_id or actor_slug to be set"))
		})
	})

	Context("when DeployKey has ActorID incorrectly set in manifest", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorID:    ptr.To(int64(99999)), // API requires this to be null for DeployKey
					ActorType:  "DeployKey",
					BypassMode: "always",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{}, nil
			}
		})

		It("should enforce ActorID to be nil per API requirement", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.BypassActors).To(HaveLen(1))
			Expect(result.Spec.BypassActors[0].ActorID).To(BeNil())
			Expect(result.Spec.BypassActors[0].ActorSlug).To(BeNil())
			Expect(result.Spec.BypassActors[0].ActorType).To(Equal("DeployKey"))
		})
	})

	Context("when DeployKey has ActorSlug incorrectly set in manifest", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorSlug:  ptr.To("some-slug"), // API requires this to be null for DeployKey
					ActorType:  "DeployKey",
					BypassMode: "always",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{}, nil
			}
		})

		It("should enforce ActorSlug to be nil per API requirement", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.BypassActors).To(HaveLen(1))
			Expect(result.Spec.BypassActors[0].ActorID).To(BeNil())
			Expect(result.Spec.BypassActors[0].ActorSlug).To(BeNil())
			Expect(result.Spec.BypassActors[0].ActorType).To(Equal("DeployKey"))
		})
	})

	Context("when multiple bypass actors with mixed types", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorID:    ptr.To(int64(111)),
					ActorType:  "Team",
					BypassMode: "always",
				},
				{
					ActorSlug:  ptr.To("security-team"),
					ActorType:  "Team",
					BypassMode: "pull_request",
				},
				{
					ActorSlug:  ptr.To("renovate"),
					ActorType:  "Integration",
					BypassMode: "always",
				},
				{
					ActorSlug:  ptr.To("admin"),
					ActorType:  "RepositoryRole",
					BypassMode: "always",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{
					{
						ID:      ptr.To(int64(10)),
						AppID:   ptr.To(int64(29)),
						AppSlug: ptr.To("renovate"),
					},
				}, nil
			}

			mockClient.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				if slug == "security-team" {
					return &github.Team{
						ID:   ptr.To(int64(222)),
						Slug: ptr.To("security-team"),
					}, nil
				}
				return nil, errors.New("team not found")
			}

			mockClient.GetRoleByNameFunc = func(ctx context.Context, org string, roleName string) (*github.CustomOrgRole, error) {
				if roleName == "admin" {
					return &github.CustomOrgRole{
						ID:   ptr.To(int64(333)),
						Name: ptr.To("admin"),
					}, nil
				}
				return nil, errors.New("role not found")
			}
		})

		It("should resolve all actors correctly", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.BypassActors).To(HaveLen(4))

			// First actor - already has ID
			Expect(result.Spec.BypassActors[0].ActorID).To(Equal(ptr.To(int64(111))))
			Expect(result.Spec.BypassActors[0].ActorType).To(Equal("Team"))

			// Second actor - team slug resolved
			Expect(result.Spec.BypassActors[1].ActorID).To(Equal(ptr.To(int64(222))))
			Expect(result.Spec.BypassActors[1].ActorType).To(Equal("Team"))

			// Third actor - integration slug resolved
			Expect(result.Spec.BypassActors[2].ActorID).To(Equal(ptr.To(int64(29))))
			Expect(result.Spec.BypassActors[2].ActorType).To(Equal("Integration"))

			// Fourth actor - role slug resolved
			Expect(result.Spec.BypassActors[3].ActorID).To(Equal(ptr.To(int64(333))))
			Expect(result.Spec.BypassActors[3].ActorType).To(Equal("RepositoryRole"))
		})

		It("should call all resolution methods", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(mockClient.TeamCalls).To(HaveLen(1))
			Expect(mockClient.RoleCalls).To(HaveLen(1))
			Expect(mockClient.EnterpriseAppsCalls).To(HaveLen(1))
		})
	})

	Context("when bypass actors include DeployKey mixed with other types", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorSlug:  ptr.To("platform-team"),
					ActorType:  "Team",
					BypassMode: "always",
				},
				{
					ActorType:  "DeployKey",
					BypassMode: "always",
				},
				{
					ActorType:  "OrganizationAdmin",
					BypassMode: "pull_request",
				},
				{
					ActorSlug:  ptr.To("dependabot"),
					ActorType:  "Integration",
					BypassMode: "always",
				},
				{
					ActorType:  "EnterpriseOwner",
					BypassMode: "always",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{
					{
						ID:      ptr.To(int64(1)),
						AppID:   ptr.To(int64(29110)),
						AppSlug: ptr.To("dependabot"),
					},
				}, nil
			}

			mockClient.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				if slug == "platform-team" {
					return &github.Team{
						ID:   ptr.To(int64(444)),
						Slug: ptr.To("platform-team"),
					}, nil
				}
				return nil, errors.New("team not found")
			}
		})

		It("should preserve all bypass actors correctly", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.BypassActors).To(HaveLen(5))

			// First actor - team slug resolved
			Expect(result.Spec.BypassActors[0].ActorID).To(Equal(ptr.To(int64(444))))
			Expect(result.Spec.BypassActors[0].ActorType).To(Equal("Team"))

			// Second actor - DeployKey with no ActorID/ActorSlug
			Expect(result.Spec.BypassActors[1].ActorID).To(BeNil())
			Expect(result.Spec.BypassActors[1].ActorSlug).To(BeNil())
			Expect(result.Spec.BypassActors[1].ActorType).To(Equal("DeployKey"))
			Expect(result.Spec.BypassActors[1].BypassMode).To(Equal("always"))

			// Third actor - OrganizationAdmin with no ActorID/ActorSlug
			Expect(result.Spec.BypassActors[2].ActorID).To(BeNil())
			Expect(result.Spec.BypassActors[2].ActorSlug).To(BeNil())
			Expect(result.Spec.BypassActors[2].ActorType).To(Equal("OrganizationAdmin"))
			Expect(result.Spec.BypassActors[2].BypassMode).To(Equal("pull_request"))

			// Fourth actor - integration slug resolved
			Expect(result.Spec.BypassActors[3].ActorID).To(Equal(ptr.To(int64(29110))))
			Expect(result.Spec.BypassActors[3].ActorType).To(Equal("Integration"))

			// Fifth actor - EnterpriseOwner with no ActorID/ActorSlug
			Expect(result.Spec.BypassActors[4].ActorID).To(BeNil())
			Expect(result.Spec.BypassActors[4].ActorSlug).To(BeNil())
			Expect(result.Spec.BypassActors[4].ActorType).To(Equal("EnterpriseOwner"))
			Expect(result.Spec.BypassActors[4].BypassMode).To(Equal("always"))
		})
	})

	Context("when status check has IntegrationID", func() {
		BeforeEach(func() {
			rulesetInput.Spec.Rules.RequiredStatusChecks = &v1alpha1.RequiredStatusChecks{
				Checks: []v1alpha1.StatusCheck{
					{
						Context:       "ci/build",
						IntegrationID: ptr.To(int64(77777)),
					},
				},
				StrictPolicy: github.Ptr(true),
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{}, nil
			}
		})

		It("should preserve the IntegrationID without resolution", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.Rules.RequiredStatusChecks).NotTo(BeNil())
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks).To(HaveLen(1))
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].IntegrationID).To(Equal(ptr.To(int64(77777))))
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].Context).To(Equal("ci/build"))
		})
	})

	Context("when status check has AppSlug", func() {
		BeforeEach(func() {
			rulesetInput.Spec.Rules.RequiredStatusChecks = &v1alpha1.RequiredStatusChecks{
				Checks: []v1alpha1.StatusCheck{
					{
						Context: "ci/build",
						AppSlug: ptr.To("circleci"),
					},
				},
				StrictPolicy: github.Ptr(false),
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{
					{
						ID:      ptr.To(int64(5)),
						AppID:   ptr.To(int64(12345)),
						AppSlug: ptr.To("circleci"),
					},
					{
						ID:      ptr.To(int64(6)),
						AppID:   ptr.To(int64(67890)),
						AppSlug: ptr.To("jenkins"),
					},
				}, nil
			}
		})

		It("should resolve the AppSlug to IntegrationID", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.Rules.RequiredStatusChecks).NotTo(BeNil())
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks).To(HaveLen(1))
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].IntegrationID).To(Equal(ptr.To(int64(12345))))
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].Context).To(Equal("ci/build"))
		})
	})

	Context("when status check has AppSlug that doesn't exist", func() {
		BeforeEach(func() {
			rulesetInput.Spec.Rules.RequiredStatusChecks = &v1alpha1.RequiredStatusChecks{
				Checks: []v1alpha1.StatusCheck{
					{
						Context: "ci/build",
						AppSlug: ptr.To("nonexistent-ci"),
					},
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{
					{
						ID:      ptr.To(int64(1)),
						AppID:   ptr.To(int64(100)),
						AppSlug: ptr.To("other-app"),
					},
				}, nil
			}
		})

		It("should set IntegrationID to nil when app slug not found", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.Rules.RequiredStatusChecks).NotTo(BeNil())
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks).To(HaveLen(1))
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].IntegrationID).To(BeNil())
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].Context).To(Equal("ci/build"))
		})
	})

	Context("when status check has neither IntegrationID nor AppSlug", func() {
		BeforeEach(func() {
			rulesetInput.Spec.Rules.RequiredStatusChecks = &v1alpha1.RequiredStatusChecks{
				Checks: []v1alpha1.StatusCheck{
					{
						Context: "ci/manual-check",
					},
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{}, nil
			}
		})

		It("should keep IntegrationID as nil", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.Rules.RequiredStatusChecks).NotTo(BeNil())
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks).To(HaveLen(1))
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].IntegrationID).To(BeNil())
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].Context).To(Equal("ci/manual-check"))
		})
	})

	Context("when multiple status checks with mixed configurations", func() {
		BeforeEach(func() {
			rulesetInput.Spec.Rules.RequiredStatusChecks = &v1alpha1.RequiredStatusChecks{
				Checks: []v1alpha1.StatusCheck{
					{
						Context:       "ci/build",
						IntegrationID: ptr.To(int64(999)),
					},
					{
						Context: "ci/test",
						AppSlug: ptr.To("github-actions"),
					},
					{
						Context: "ci/lint",
					},
					{
						Context: "ci/security",
						AppSlug: ptr.To("snyk"),
					},
				},
				StrictPolicy: github.Ptr(true),
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{
					{
						ID:      ptr.To(int64(1)),
						AppID:   ptr.To(int64(15368)),
						AppSlug: ptr.To("github-actions"),
					},
					{
						ID:      ptr.To(int64(2)),
						AppID:   ptr.To(int64(24680)),
						AppSlug: ptr.To("snyk"),
					},
				}, nil
			}
		})

		It("should resolve all status checks correctly", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.Rules.RequiredStatusChecks).NotTo(BeNil())
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks).To(HaveLen(4))

			// First check - already has IntegrationID
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].Context).To(Equal("ci/build"))
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].IntegrationID).To(Equal(ptr.To(int64(999))))

			// Second check - AppSlug resolved
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[1].Context).To(Equal("ci/test"))
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[1].IntegrationID).To(Equal(ptr.To(int64(15368))))

			// Third check - no integration
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[2].Context).To(Equal("ci/lint"))
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[2].IntegrationID).To(BeNil())

			// Fourth check - AppSlug resolved
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[3].Context).To(Equal("ci/security"))
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[3].IntegrationID).To(Equal(ptr.To(int64(24680))))
		})
	})

	Context("when both bypass actors and status checks need resolution", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorSlug:  ptr.To("dependabot"),
					ActorType:  "Integration",
					BypassMode: "always",
				},
			}
			rulesetInput.Spec.Rules.RequiredStatusChecks = &v1alpha1.RequiredStatusChecks{
				Checks: []v1alpha1.StatusCheck{
					{
						Context: "ci/build",
						AppSlug: ptr.To("github-actions"),
					},
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{
					{
						ID:      ptr.To(int64(1)),
						AppID:   ptr.To(int64(29110)),
						AppSlug: ptr.To("dependabot"),
					},
					{
						ID:      ptr.To(int64(2)),
						AppID:   ptr.To(int64(15368)),
						AppSlug: ptr.To("github-actions"),
					},
				}, nil
			}
		})

		It("should resolve both bypass actors and status checks", func() {
			Expect(err).NotTo(HaveOccurred())

			// Check bypass actor
			Expect(result.Spec.BypassActors).To(HaveLen(1))
			Expect(result.Spec.BypassActors[0].ActorID).To(Equal(ptr.To(int64(29110))))

			// Check status check
			Expect(result.Spec.Rules.RequiredStatusChecks).NotTo(BeNil())
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks).To(HaveLen(1))
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].IntegrationID).To(Equal(ptr.To(int64(15368))))
		})

		It("should only fetch installations once", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(mockClient.EnterpriseAppsCalls).To(HaveLen(1))
		})
	})
})
