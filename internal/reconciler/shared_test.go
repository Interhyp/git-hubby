package reconciler

import (
	"context"
	"errors"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	"github.com/google/go-github/v86/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
					Creation: new(true),
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
					ActorID:    new(int64(12345)),
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
			Expect(result.Spec.BypassActors[0].ActorID).To(Equal(new(int64(12345))))
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
					ActorSlug:  new("engineering-team"),
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
					ID:   new(int64(98765)),
					Slug: new("engineering-team"),
					Name: new("Engineering Team"),
				}, nil
			}
		})

		It("should resolve the team slug to ActorID", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.BypassActors).To(HaveLen(1))
			Expect(result.Spec.BypassActors[0].ActorID).To(Equal(new(int64(98765))))
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
					ActorSlug:  new("nonexistent-team"),
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
					ActorSlug:  new("github-actions"),
					ActorType:  "Integration",
					BypassMode: "pull_request",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{
					{
						ID:      new(int64(1)),
						AppID:   new(int64(15368)),
						AppSlug: new("github-actions"),
					},
					{
						ID:      new(int64(2)),
						AppID:   new(int64(99999)),
						AppSlug: new("other-app"),
					},
				}, nil
			}
		})

		It("should resolve the app slug to ActorID", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.BypassActors).To(HaveLen(1))
			Expect(result.Spec.BypassActors[0].ActorID).To(Equal(new(int64(15368))))
			Expect(result.Spec.BypassActors[0].ActorType).To(Equal("Integration"))
		})
	})

	Context("when bypass actor has Integration ActorSlug that doesn't exist", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorSlug:  new("nonexistent-app"),
					ActorType:  "Integration",
					BypassMode: "always",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{
					{
						ID:      new(int64(1)),
						AppID:   new(int64(15368)),
						AppSlug: new("github-actions"),
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
					ActorSlug:  new("maintain"),
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
					ID:   new(int64(54321)),
					Name: new("maintain"),
				}, nil
			}
		})

		It("should resolve the role slug to ActorID", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.BypassActors).To(HaveLen(1))
			Expect(result.Spec.BypassActors[0].ActorID).To(Equal(new(int64(54321))))
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
					ActorSlug:  new("nonexistent-role"),
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
					ActorSlug:  new("some-slug"),
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
					ActorID:    new(int64(12354)),
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
		actorID := new(int64(84354))
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
					ActorID:    new(int64(99999)), // API requires this to be null for DeployKey
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
					ActorSlug:  new("some-slug"), // API requires this to be null for DeployKey
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
					ActorID:    new(int64(111)),
					ActorType:  "Team",
					BypassMode: "always",
				},
				{
					ActorSlug:  new("security-team"),
					ActorType:  "Team",
					BypassMode: "pull_request",
				},
				{
					ActorSlug:  new("renovate"),
					ActorType:  "Integration",
					BypassMode: "always",
				},
				{
					ActorSlug:  new("admin"),
					ActorType:  "RepositoryRole",
					BypassMode: "always",
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{
					{
						ID:      new(int64(10)),
						AppID:   new(int64(29)),
						AppSlug: new("renovate"),
					},
				}, nil
			}

			mockClient.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				if slug == "security-team" {
					return &github.Team{
						ID:   new(int64(222)),
						Slug: new("security-team"),
					}, nil
				}
				return nil, errors.New("team not found")
			}

			mockClient.GetRoleByNameFunc = func(ctx context.Context, org string, roleName string) (*github.CustomOrgRole, error) {
				if roleName == "admin" {
					return &github.CustomOrgRole{
						ID:   new(int64(333)),
						Name: new("admin"),
					}, nil
				}
				return nil, errors.New("role not found")
			}
		})

		It("should resolve all actors correctly", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.BypassActors).To(HaveLen(4))

			// First actor - already has ID
			Expect(result.Spec.BypassActors[0].ActorID).To(Equal(new(int64(111))))
			Expect(result.Spec.BypassActors[0].ActorType).To(Equal("Team"))

			// Second actor - team slug resolved
			Expect(result.Spec.BypassActors[1].ActorID).To(Equal(new(int64(222))))
			Expect(result.Spec.BypassActors[1].ActorType).To(Equal("Team"))

			// Third actor - integration slug resolved
			Expect(result.Spec.BypassActors[2].ActorID).To(Equal(new(int64(29))))
			Expect(result.Spec.BypassActors[2].ActorType).To(Equal("Integration"))

			// Fourth actor - role slug resolved
			Expect(result.Spec.BypassActors[3].ActorID).To(Equal(new(int64(333))))
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
					ActorSlug:  new("platform-team"),
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
					ActorSlug:  new("dependabot"),
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
						ID:      new(int64(1)),
						AppID:   new(int64(29110)),
						AppSlug: new("dependabot"),
					},
				}, nil
			}

			mockClient.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
				if slug == "platform-team" {
					return &github.Team{
						ID:   new(int64(444)),
						Slug: new("platform-team"),
					}, nil
				}
				return nil, errors.New("team not found")
			}
		})

		It("should preserve all bypass actors correctly", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.BypassActors).To(HaveLen(5))

			// First actor - team slug resolved
			Expect(result.Spec.BypassActors[0].ActorID).To(Equal(new(int64(444))))
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
			Expect(result.Spec.BypassActors[3].ActorID).To(Equal(new(int64(29110))))
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
						IntegrationID: new(int64(77777)),
					},
				},
				StrictPolicy: new(true),
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{}, nil
			}
		})

		It("should preserve the IntegrationID without resolution", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.Rules.RequiredStatusChecks).NotTo(BeNil())
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks).To(HaveLen(1))
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].IntegrationID).To(Equal(new(int64(77777))))
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].Context).To(Equal("ci/build"))
		})
	})

	Context("when status check has AppSlug", func() {
		BeforeEach(func() {
			rulesetInput.Spec.Rules.RequiredStatusChecks = &v1alpha1.RequiredStatusChecks{
				Checks: []v1alpha1.StatusCheck{
					{
						Context: "ci/build",
						AppSlug: new("circleci"),
					},
				},
				StrictPolicy: new(false),
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{
					{
						ID:      new(int64(5)),
						AppID:   new(int64(12345)),
						AppSlug: new("circleci"),
					},
					{
						ID:      new(int64(6)),
						AppID:   new(int64(67890)),
						AppSlug: new("jenkins"),
					},
				}, nil
			}
		})

		It("should resolve the AppSlug to IntegrationID", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.Rules.RequiredStatusChecks).NotTo(BeNil())
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks).To(HaveLen(1))
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].IntegrationID).To(Equal(new(int64(12345))))
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].Context).To(Equal("ci/build"))
		})
	})

	Context("when status check has AppSlug that doesn't exist", func() {
		BeforeEach(func() {
			rulesetInput.Spec.Rules.RequiredStatusChecks = &v1alpha1.RequiredStatusChecks{
				Checks: []v1alpha1.StatusCheck{
					{
						Context: "ci/build",
						AppSlug: new("nonexistent-ci"),
					},
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{
					{
						ID:      new(int64(1)),
						AppID:   new(int64(100)),
						AppSlug: new("other-app"),
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
						IntegrationID: new(int64(999)),
					},
					{
						Context: "ci/test",
						AppSlug: new("github-actions"),
					},
					{
						Context: "ci/lint",
					},
					{
						Context: "ci/security",
						AppSlug: new("snyk"),
					},
				},
				StrictPolicy: new(true),
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{
					{
						ID:      new(int64(1)),
						AppID:   new(int64(15368)),
						AppSlug: new("github-actions"),
					},
					{
						ID:      new(int64(2)),
						AppID:   new(int64(24680)),
						AppSlug: new("snyk"),
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
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].IntegrationID).To(Equal(new(int64(999))))

			// Second check - AppSlug resolved
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[1].Context).To(Equal("ci/test"))
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[1].IntegrationID).To(Equal(new(int64(15368))))

			// Third check - no integration
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[2].Context).To(Equal("ci/lint"))
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[2].IntegrationID).To(BeNil())

			// Fourth check - AppSlug resolved
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[3].Context).To(Equal("ci/security"))
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[3].IntegrationID).To(Equal(new(int64(24680))))
		})
	})

	Context("when both bypass actors and status checks need resolution", func() {
		BeforeEach(func() {
			rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
				{
					ActorSlug:  new("dependabot"),
					ActorType:  "Integration",
					BypassMode: "always",
				},
			}
			rulesetInput.Spec.Rules.RequiredStatusChecks = &v1alpha1.RequiredStatusChecks{
				Checks: []v1alpha1.StatusCheck{
					{
						Context: "ci/build",
						AppSlug: new("github-actions"),
					},
				},
			}

			mockClient.GetGitHubAppsInstallationsFunc = func(ctx context.Context, org string) ([]*github.Installation, error) {
				return []*github.Installation{
					{
						ID:      new(int64(1)),
						AppID:   new(int64(29110)),
						AppSlug: new("dependabot"),
					},
					{
						ID:      new(int64(2)),
						AppID:   new(int64(15368)),
						AppSlug: new("github-actions"),
					},
				}, nil
			}
		})

		It("should resolve both bypass actors and status checks", func() {
			Expect(err).NotTo(HaveOccurred())

			// Check bypass actor
			Expect(result.Spec.BypassActors).To(HaveLen(1))
			Expect(result.Spec.BypassActors[0].ActorID).To(Equal(new(int64(29110))))

			// Check status check
			Expect(result.Spec.Rules.RequiredStatusChecks).NotTo(BeNil())
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks).To(HaveLen(1))
			Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].IntegrationID).To(Equal(new(int64(15368))))
		})

		It("should only fetch installations once", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(mockClient.EnterpriseAppsCalls).To(HaveLen(1))
		})
	})
})

var _ = Describe("GetOrgPlan", func() {
	It("should return 'enterprise' when org is nil", func() {
		Expect(GetOrgPlan(nil)).To(Equal("enterprise"))
	})

	It("should return 'enterprise' when plan is empty", func() {
		org := &v1alpha1.Organization{
			Spec: v1alpha1.OrganizationSpec{Plan: ""},
		}
		Expect(GetOrgPlan(org)).To(Equal("enterprise"))
	})

	It("should return 'enterprise' when plan is 'enterprise'", func() {
		org := &v1alpha1.Organization{
			Spec: v1alpha1.OrganizationSpec{Plan: "enterprise"},
		}
		Expect(GetOrgPlan(org)).To(Equal("enterprise"))
	})

	It("should return 'team' when plan is 'team'", func() {
		org := &v1alpha1.Organization{
			Spec: v1alpha1.OrganizationSpec{Plan: "team"},
		}
		Expect(GetOrgPlan(org)).To(Equal("team"))
	})

	It("should return 'free' when plan is 'free'", func() {
		org := &v1alpha1.Organization{
			Spec: v1alpha1.OrganizationSpec{Plan: "free"},
		}
		Expect(GetOrgPlan(org)).To(Equal("free"))
	})
})

var _ = Describe("GetOrgPlanByRef", func() {
	var (
		ctx       context.Context
		k8sClient client.Client
		scheme    *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())
	})

	It("should return 'enterprise' when organization CR is not found", func() {
		k8sClient = fake.NewClientBuilder().WithScheme(scheme).Build()
		Expect(GetOrgPlanByRef(ctx, k8sClient, "default", "non-existent")).To(Equal("enterprise"))
	})

	It("should return 'enterprise' when plan field is empty", func() {
		org := &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{Name: "my-org", Namespace: "default"},
			Spec:       v1alpha1.OrganizationSpec{Name: "my-org", Plan: ""},
		}
		k8sClient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(org).Build()
		Expect(GetOrgPlanByRef(ctx, k8sClient, "default", "my-org")).To(Equal("enterprise"))
	})

	It("should return 'team' when org plan is 'team'", func() {
		org := &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{Name: "my-org", Namespace: "default"},
			Spec:       v1alpha1.OrganizationSpec{Name: "my-org", Plan: "team"},
		}
		k8sClient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(org).Build()
		Expect(GetOrgPlanByRef(ctx, k8sClient, "default", "my-org")).To(Equal("team"))
	})

	It("should return 'free' when org plan is 'free'", func() {
		org := &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{Name: "my-org", Namespace: "default"},
			Spec:       v1alpha1.OrganizationSpec{Name: "my-org", Plan: "free"},
		}
		k8sClient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(org).Build()
		Expect(GetOrgPlanByRef(ctx, k8sClient, "default", "my-org")).To(Equal("free"))
	})

	It("should return 'enterprise' when namespace does not match", func() {
		org := &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{Name: "my-org", Namespace: "other"},
			Spec:       v1alpha1.OrganizationSpec{Name: "my-org", Plan: "team"},
		}
		k8sClient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(org).Build()
		Expect(GetOrgPlanByRef(ctx, k8sClient, "default", "my-org")).To(Equal("enterprise"))
	})
})
