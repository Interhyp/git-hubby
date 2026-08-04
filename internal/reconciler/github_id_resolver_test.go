package reconciler

import (
	"context"
	"errors"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	"github.com/google/go-github/v90/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// emptyResolver returns a GitHubIDResolver with empty maps, for tests that don't need
// any pre-loaded data.
func emptyResolver() *GitHubIDResolver {
	return &GitHubIDResolver{
		appsBySlug:  map[string]*int64{},
		teamsBySlug: map[string]int64{},
		rolesByName: map[string]int64{},
	}
}

var _ = Describe("GitHubIDResolver", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	// ── NewGitHubIDResolver ──────────────────────────────────────────────────

	Describe("NewGitHubIDResolver", func() {
		var mockClient *ghclientmock.MockGitHubClientWrapper

		BeforeEach(func() {
			mockClient = ghclientmock.NewMockGitHubClientWrapper()
			mockClient.GetGitHubAppsInstallationsFunc = func(_ context.Context, _ string) ([]*github.Installation, error) {
				return []*github.Installation{
					{ID: new(int64(1)), AppID: new(int64(15368)), AppSlug: new("github-actions")},
				}, nil
			}
			mockClient.GetAllTeamsForOrgFunc = func(_ context.Context, _ string) ([]*github.Team, error) {
				return []*github.Team{
					{ID: new(int64(42)), Slug: new("platform-team")},
				}, nil
			}
			mockClient.GetAllOrgRolesFunc = func(_ context.Context, _ string) ([]*github.CustomOrgRole, error) {
				return []*github.CustomOrgRole{
					{ID: new(int64(99)), Name: new("maintain")},
				}, nil
			}
		})

		It("makes exactly three bulk API calls", func() {
			_, err := NewGitHubIDResolver(ctx, mockClient, "test-org")
			Expect(err).NotTo(HaveOccurred())
			Expect(mockClient.EnterpriseAppsCalls).To(HaveLen(1))
			Expect(mockClient.TeamCalls).To(HaveLen(1))
			Expect(mockClient.TeamCalls[0].Method).To(Equal("GetAllTeamsForOrg"))
			Expect(mockClient.RoleAssignmentCalls).To(HaveLen(1))
			Expect(mockClient.RoleAssignmentCalls[0].Method).To(Equal("GetAllOrgRoles"))
		})

		It("returns an error when GetGitHubAppsInstallations fails", func() {
			mockClient.GetGitHubAppsInstallationsFunc = func(_ context.Context, _ string) ([]*github.Installation, error) {
				return nil, errors.New("apps API error")
			}
			_, err := NewGitHubIDResolver(ctx, mockClient, "test-org")
			Expect(err).To(MatchError(ContainSubstring("apps API error")))
		})

		It("returns an error when GetAllTeamsForOrg fails", func() {
			mockClient.GetAllTeamsForOrgFunc = func(_ context.Context, _ string) ([]*github.Team, error) {
				return nil, errors.New("teams API error")
			}
			_, err := NewGitHubIDResolver(ctx, mockClient, "test-org")
			Expect(err).To(MatchError(ContainSubstring("teams API error")))
		})

		It("returns an error when GetAllOrgRoles fails", func() {
			mockClient.GetAllOrgRolesFunc = func(_ context.Context, _ string) ([]*github.CustomOrgRole, error) {
				return nil, errors.New("roles API error")
			}
			_, err := NewGitHubIDResolver(ctx, mockClient, "test-org")
			Expect(err).To(MatchError(ContainSubstring("roles API error")))
		})

		It("makes zero further API calls during resolution", func() {
			resolver, err := NewGitHubIDResolver(ctx, mockClient, "test-org")
			Expect(err).NotTo(HaveOccurred())
			mockClient.Reset()

			rs := v1alpha1.RulesetPreset{
				Spec: v1alpha1.RulesetPresetSpec{
					BypassActors: []v1alpha1.RulesetBypassActor{
						{ActorSlug: new("platform-team"), ActorType: "Team", BypassMode: "always"},
						{ActorSlug: new("github-actions"), ActorType: "Integration", BypassMode: "pull_request"},
						{ActorSlug: new("maintain"), ActorType: "RepositoryRole", BypassMode: "always"},
					},
				},
			}
			for range 3 {
				_, err := resolver.ResolveRuleset(ctx, rs)
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(mockClient.TeamCalls).To(BeEmpty())
			Expect(mockClient.EnterpriseAppsCalls).To(BeEmpty())
			Expect(mockClient.RoleCalls).To(BeEmpty())
		})
	})

	// ── ResolveTeamSlug ──────────────────────────────────────────────────────

	Describe("ResolveTeamSlug", func() {
		It("resolves a known slug to its ID", func() {
			resolver := &GitHubIDResolver{appsBySlug: map[string]*int64{}, teamsBySlug: map[string]int64{"platform-team": 42}, rolesByName: map[string]int64{}}
			id, err := resolver.ResolveTeamSlug("platform-team")
			Expect(err).NotTo(HaveOccurred())
			Expect(id).To(Equal(int64(42)))
		})

		It("returns an error for an unknown slug", func() {
			_, err := emptyResolver().ResolveTeamSlug("unknown-team")
			Expect(err).To(MatchError(ContainSubstring("unknown-team")))
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})
	})

	// ── ResolveRoleName ──────────────────────────────────────────────────────

	Describe("ResolveRoleName", func() {
		It("resolves a known role name to its ID", func() {
			resolver := &GitHubIDResolver{appsBySlug: map[string]*int64{}, teamsBySlug: map[string]int64{}, rolesByName: map[string]int64{"maintain": 99}}
			id, err := resolver.ResolveRoleName("maintain")
			Expect(err).NotTo(HaveOccurred())
			Expect(id).To(Equal(int64(99)))
		})

		It("returns an error for an unknown role name", func() {
			_, err := emptyResolver().ResolveRoleName("nonexistent-role")
			Expect(err).To(MatchError(ContainSubstring("nonexistent-role")))
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})
	})

	// ── ResolveCscBypassReviewers ────────────────────────────────────────────

	Describe("ResolveCscBypassReviewers", func() {
		It("resolves TEAM and ROLE reviewer names to IDs", func() {
			resolver := &GitHubIDResolver{
				appsBySlug:  map[string]*int64{},
				teamsBySlug: map[string]int64{"security-team": 42},
				rolesByName: map[string]int64{"admin": 99},
			}
			csc := &v1alpha1.CodeSecurityConfiguration{
				Spec: v1alpha1.CodeSecurityConfigurationSpec{
					SecretScanningDelegatedBypassOptions: &v1alpha1.SecretScanningDelegatedBypassOptions{
						Reviewers: []*v1alpha1.BypassReviewer{
							{ReviewerName: new("security-team"), ReviewerType: "TEAM"},
							{ReviewerName: new("admin"), ReviewerType: "ROLE"},
						},
					},
				},
			}
			result, err := resolver.ResolveCscBypassReviewers(ctx, csc)
			Expect(err).NotTo(HaveOccurred())
			Expect(*result.Spec.SecretScanningDelegatedBypassOptions.Reviewers[0].ReviewerId).To(Equal(int64(42)))
			Expect(*result.Spec.SecretScanningDelegatedBypassOptions.Reviewers[1].ReviewerId).To(Equal(int64(99)))
		})

		It("returns csc unchanged when no delegated bypass options are set", func() {
			csc := &v1alpha1.CodeSecurityConfiguration{}
			result, err := emptyResolver().ResolveCscBypassReviewers(ctx, csc)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.SecretScanningDelegatedBypassOptions).To(BeNil())
		})

		It("returns an error when a TEAM reviewer name is not found", func() {
			csc := &v1alpha1.CodeSecurityConfiguration{
				Spec: v1alpha1.CodeSecurityConfigurationSpec{
					SecretScanningDelegatedBypassOptions: &v1alpha1.SecretScanningDelegatedBypassOptions{
						Reviewers: []*v1alpha1.BypassReviewer{
							{ReviewerName: new("nonexistent-team"), ReviewerType: "TEAM"},
						},
					},
				},
			}
			_, err := emptyResolver().ResolveCscBypassReviewers(ctx, csc)
			Expect(err).To(MatchError(ContainSubstring("nonexistent-team")))
		})

		It("returns an error when a ROLE reviewer name is not found", func() {
			csc := &v1alpha1.CodeSecurityConfiguration{
				Spec: v1alpha1.CodeSecurityConfigurationSpec{
					SecretScanningDelegatedBypassOptions: &v1alpha1.SecretScanningDelegatedBypassOptions{
						Reviewers: []*v1alpha1.BypassReviewer{
							{ReviewerName: new("nonexistent-role"), ReviewerType: "ROLE"},
						},
					},
				},
			}
			_, err := emptyResolver().ResolveCscBypassReviewers(ctx, csc)
			Expect(err).To(MatchError(ContainSubstring("nonexistent-role")))
		})

		It("skips reviewers that have no ReviewerName set", func() {
			csc := &v1alpha1.CodeSecurityConfiguration{
				Spec: v1alpha1.CodeSecurityConfigurationSpec{
					SecretScanningDelegatedBypassOptions: &v1alpha1.SecretScanningDelegatedBypassOptions{
						Reviewers: []*v1alpha1.BypassReviewer{
							{ReviewerType: "TEAM"},
						},
					},
				},
			}
			result, err := emptyResolver().ResolveCscBypassReviewers(ctx, csc)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.SecretScanningDelegatedBypassOptions.Reviewers[0].ReviewerId).To(BeNil())
		})
	})

	// ── ResolveRuleset ───────────────────────────────────────────────────────

	Describe("ResolveRuleset", func() {
		var (
			resolver     *GitHubIDResolver
			rulesetInput v1alpha1.RulesetPreset
			result       v1alpha1.RulesetPreset
			err          error
		)

		BeforeEach(func() {
			resolver = emptyResolver()
			rulesetInput = v1alpha1.RulesetPreset{
				Spec: v1alpha1.RulesetPresetSpec{
					Name: "test-ruleset",
					Conditions: &v1alpha1.RulesetConditions{
						RefName: &v1alpha1.RefNameCondition{Include: []string{"refs/heads/main"}},
					},
					Enforcement: "active",
					Rules:       v1alpha1.RulesetRules{Creation: new(true)},
				},
			}
		})

		JustBeforeEach(func() {
			result, err = resolver.ResolveRuleset(ctx, rulesetInput)
		})

		// ── no bypass actors ────────────────────────────────────────────────

		Context("when ruleset has no bypass actors and no status checks", func() {
			It("succeeds without modifications", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.BypassActors).To(BeEmpty())
				Expect(result.Spec.Rules.RequiredStatusChecks).To(BeNil())
			})
		})

		// ── bypass actor: Team ───────────────────────────────────────────────

		Context("when bypass actor already has an ActorID (Team)", func() {
			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorID: new(int64(12345)), ActorType: "Team", BypassMode: "always"},
				}
			})

			It("preserves the ActorID without resolution", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.BypassActors).To(HaveLen(1))
				Expect(result.Spec.BypassActors[0].ActorID).To(Equal(new(int64(12345))))
				Expect(result.Spec.BypassActors[0].ActorType).To(Equal("Team"))
			})
		})

		Context("when bypass actor has a Team ActorSlug", func() {
			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorSlug: new("engineering-team"), ActorType: "Team", BypassMode: "always"},
				}
				resolver = &GitHubIDResolver{appsBySlug: map[string]*int64{}, teamsBySlug: map[string]int64{"engineering-team": 98765}, rolesByName: map[string]int64{}}
			})

			It("resolves the team slug to ActorID", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.BypassActors).To(HaveLen(1))
				Expect(result.Spec.BypassActors[0].ActorID).To(Equal(new(int64(98765))))
				Expect(result.Spec.BypassActors[0].ActorType).To(Equal("Team"))
			})
		})

		Context("when Team ActorSlug is not in the pre-loaded map", func() {
			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorSlug: new("nonexistent-team"), ActorType: "Team", BypassMode: "always"},
				}
			})

			It("returns an error referencing the missing slug", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("nonexistent-team"))
				Expect(err.Error()).To(ContainSubstring("not found"))
			})
		})

		Context("when Team bypass actor has both ActorID and ActorSlug nil", func() {
			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorType: "Team", BypassMode: "always"},
				}
			})

			It("returns a validation error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("bypass actor with type Team requires either actor_id or actor_slug to be set"))
			})
		})

		// ── bypass actor: Integration ────────────────────────────────────────

		Context("when bypass actor has an Integration ActorSlug", func() {
			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorSlug: new("github-actions"), ActorType: "Integration", BypassMode: "pull_request"},
				}
				resolver = &GitHubIDResolver{
					appsBySlug:  map[string]*int64{"github-actions": new(int64(15368)), "other-app": new(int64(99999))},
					teamsBySlug: map[string]int64{},
					rolesByName: map[string]int64{},
				}
			})

			It("resolves the app slug to ActorID", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.BypassActors).To(HaveLen(1))
				Expect(result.Spec.BypassActors[0].ActorID).To(Equal(new(int64(15368))))
				Expect(result.Spec.BypassActors[0].ActorType).To(Equal("Integration"))
			})
		})

		Context("when Integration ActorSlug is not in the pre-loaded map", func() {
			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorSlug: new("nonexistent-app"), ActorType: "Integration", BypassMode: "always"},
				}
				resolver = &GitHubIDResolver{appsBySlug: map[string]*int64{"github-actions": new(int64(15368))}, teamsBySlug: map[string]int64{}, rolesByName: map[string]int64{}}
			})

			It("returns an error naming the missing slug", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no GitHub App with slug nonexistent-app installed"))
			})
		})

		Context("when Integration bypass actor has both ActorID and ActorSlug nil", func() {
			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorType: "Integration", BypassMode: "always"},
				}
			})

			It("returns a validation error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("bypass actor with type Integration requires either actor_id or actor_slug to be set"))
			})
		})

		// ── bypass actor: RepositoryRole ─────────────────────────────────────

		Context("when bypass actor has a RepositoryRole ActorSlug", func() {
			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorSlug: new("maintain"), ActorType: "RepositoryRole", BypassMode: "always"},
				}
				resolver = &GitHubIDResolver{appsBySlug: map[string]*int64{}, teamsBySlug: map[string]int64{}, rolesByName: map[string]int64{"maintain": 54321}}
			})

			It("resolves the role slug to ActorID", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.BypassActors).To(HaveLen(1))
				Expect(result.Spec.BypassActors[0].ActorID).To(Equal(new(int64(54321))))
				Expect(result.Spec.BypassActors[0].ActorType).To(Equal("RepositoryRole"))
			})
		})

		Context("when RepositoryRole ActorSlug is not in the pre-loaded map", func() {
			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorSlug: new("nonexistent-role"), ActorType: "RepositoryRole", BypassMode: "always"},
				}
			})

			It("returns an error referencing the missing role", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("nonexistent-role"))
				Expect(err.Error()).To(ContainSubstring("not found"))
			})
		})

		Context("when RepositoryRole bypass actor has both ActorID and ActorSlug nil", func() {
			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorType: "RepositoryRole", BypassMode: "always"},
				}
			})

			It("returns a validation error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("bypass actor with type RepositoryRole requires either actor_id or actor_slug to be set"))
			})
		})

		// ── bypass actor: OrganizationAdmin ──────────────────────────────────

		Context("when bypass actor has OrganizationAdmin type with a slug", func() {
			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorSlug: new("some-slug"), ActorType: "OrganizationAdmin", BypassMode: "always"},
				}
			})

			It("sets ActorID to nil (OrganizationAdmin needs no ID)", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.BypassActors).To(HaveLen(1))
				Expect(result.Spec.BypassActors[0].ActorID).To(BeNil())
				Expect(result.Spec.BypassActors[0].ActorType).To(Equal("OrganizationAdmin"))
			})
		})

		Context("when bypass actor is OrganizationAdmin with no ActorID or ActorSlug", func() {
			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorType: "OrganizationAdmin", BypassMode: "always"},
				}
			})

			It("preserves the actor with nil ActorID and nil ActorSlug", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.BypassActors).To(HaveLen(1))
				Expect(result.Spec.BypassActors[0].ActorID).To(BeNil())
				Expect(result.Spec.BypassActors[0].ActorSlug).To(BeNil())
				Expect(result.Spec.BypassActors[0].ActorType).To(Equal("OrganizationAdmin"))
			})
		})

		// ── bypass actor: EnterpriseOwner ─────────────────────────────────────

		Context("when bypass actor is EnterpriseOwner with no ActorID or ActorSlug", func() {
			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorType: "EnterpriseOwner", BypassMode: "always"},
				}
			})

			It("preserves the actor unchanged", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.BypassActors).To(HaveLen(1))
				Expect(result.Spec.BypassActors[0].ActorID).To(BeNil())
				Expect(result.Spec.BypassActors[0].ActorSlug).To(BeNil())
				Expect(result.Spec.BypassActors[0].ActorType).To(Equal("EnterpriseOwner"))
			})
		})

		Context("when bypass actor is EnterpriseOwner with an ActorID", func() {
			actorID := new(int64(84354))

			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorType: "EnterpriseOwner", BypassMode: "always", ActorID: actorID},
				}
			})

			It("preserves the existing ActorID", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.BypassActors).To(HaveLen(1))
				Expect(result.Spec.BypassActors[0].ActorID).To(Equal(actorID))
				Expect(result.Spec.BypassActors[0].ActorType).To(Equal("EnterpriseOwner"))
			})
		})

		// ── bypass actor: DeployKey ───────────────────────────────────────────

		Context("when bypass actor is DeployKey with no ActorID or ActorSlug", func() {
			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorType: "DeployKey", BypassMode: "always"},
				}
			})

			It("preserves the DeployKey actor with nil ActorID and nil ActorSlug", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.BypassActors).To(HaveLen(1))
				Expect(result.Spec.BypassActors[0].ActorID).To(BeNil())
				Expect(result.Spec.BypassActors[0].ActorSlug).To(BeNil())
				Expect(result.Spec.BypassActors[0].ActorType).To(Equal("DeployKey"))
				Expect(result.Spec.BypassActors[0].BypassMode).To(Equal("always"))
			})
		})

		Context("when DeployKey has ActorID incorrectly set in the manifest", func() {
			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorID: new(int64(99999)), ActorType: "DeployKey", BypassMode: "always"},
				}
			})

			It("enforces ActorID to nil per API requirement", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.BypassActors[0].ActorID).To(BeNil())
				Expect(result.Spec.BypassActors[0].ActorSlug).To(BeNil())
				Expect(result.Spec.BypassActors[0].ActorType).To(Equal("DeployKey"))
			})
		})

		Context("when DeployKey has ActorSlug incorrectly set in the manifest", func() {
			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorSlug: new("some-slug"), ActorType: "DeployKey", BypassMode: "always"},
				}
			})

			It("enforces ActorSlug to nil per API requirement", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.BypassActors[0].ActorID).To(BeNil())
				Expect(result.Spec.BypassActors[0].ActorSlug).To(BeNil())
				Expect(result.Spec.BypassActors[0].ActorType).To(Equal("DeployKey"))
			})
		})

		// ── mixed bypass actors ───────────────────────────────────────────────

		Context("when multiple bypass actors with mixed types all need resolution", func() {
			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorID: new(int64(111)), ActorType: "Team", BypassMode: "always"},
					{ActorSlug: new("security-team"), ActorType: "Team", BypassMode: "pull_request"},
					{ActorSlug: new("renovate"), ActorType: "Integration", BypassMode: "always"},
					{ActorSlug: new("admin"), ActorType: "RepositoryRole", BypassMode: "always"},
				}
				resolver = &GitHubIDResolver{
					appsBySlug:  map[string]*int64{"renovate": new(int64(29))},
					teamsBySlug: map[string]int64{"security-team": 222},
					rolesByName: map[string]int64{"admin": 333},
				}
			})

			It("resolves all actors correctly", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.BypassActors).To(HaveLen(4))
				Expect(result.Spec.BypassActors[0].ActorID).To(Equal(new(int64(111))))
				Expect(result.Spec.BypassActors[0].ActorType).To(Equal("Team"))
				Expect(result.Spec.BypassActors[1].ActorID).To(Equal(new(int64(222))))
				Expect(result.Spec.BypassActors[1].ActorType).To(Equal("Team"))
				Expect(result.Spec.BypassActors[2].ActorID).To(Equal(new(int64(29))))
				Expect(result.Spec.BypassActors[2].ActorType).To(Equal("Integration"))
				Expect(result.Spec.BypassActors[3].ActorID).To(Equal(new(int64(333))))
				Expect(result.Spec.BypassActors[3].ActorType).To(Equal("RepositoryRole"))
			})
		})

		Context("when bypass actors include DeployKey mixed with other types", func() {
			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorSlug: new("platform-team"), ActorType: "Team", BypassMode: "always"},
					{ActorType: "DeployKey", BypassMode: "always"},
					{ActorType: "OrganizationAdmin", BypassMode: "pull_request"},
					{ActorSlug: new("dependabot"), ActorType: "Integration", BypassMode: "always"},
					{ActorType: "EnterpriseOwner", BypassMode: "always"},
				}
				resolver = &GitHubIDResolver{
					appsBySlug:  map[string]*int64{"dependabot": new(int64(29110))},
					teamsBySlug: map[string]int64{"platform-team": 444},
					rolesByName: map[string]int64{},
				}
			})

			It("resolves slug-bearing actors and preserves literal actors", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.BypassActors).To(HaveLen(5))
				Expect(result.Spec.BypassActors[0].ActorID).To(Equal(new(int64(444))))
				Expect(result.Spec.BypassActors[0].ActorType).To(Equal("Team"))
				Expect(result.Spec.BypassActors[1].ActorID).To(BeNil())
				Expect(result.Spec.BypassActors[1].ActorSlug).To(BeNil())
				Expect(result.Spec.BypassActors[1].ActorType).To(Equal("DeployKey"))
				Expect(result.Spec.BypassActors[2].ActorID).To(BeNil())
				Expect(result.Spec.BypassActors[2].ActorType).To(Equal("OrganizationAdmin"))
				Expect(result.Spec.BypassActors[3].ActorID).To(Equal(new(int64(29110))))
				Expect(result.Spec.BypassActors[3].ActorType).To(Equal("Integration"))
				Expect(result.Spec.BypassActors[4].ActorID).To(BeNil())
				Expect(result.Spec.BypassActors[4].ActorType).To(Equal("EnterpriseOwner"))
			})
		})

		// ── status checks ─────────────────────────────────────────────────────

		Context("when status check already has an IntegrationID", func() {
			BeforeEach(func() {
				rulesetInput.Spec.Rules.RequiredStatusChecks = &v1alpha1.RequiredStatusChecks{
					Checks:       []v1alpha1.StatusCheck{{Context: "ci/build", IntegrationID: new(int64(77777))}},
					StrictPolicy: new(true),
				}
			})

			It("preserves the IntegrationID without resolution", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].IntegrationID).To(Equal(new(int64(77777))))
				Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].Context).To(Equal("ci/build"))
			})
		})

		Context("when status check has an AppSlug", func() {
			BeforeEach(func() {
				rulesetInput.Spec.Rules.RequiredStatusChecks = &v1alpha1.RequiredStatusChecks{
					Checks:       []v1alpha1.StatusCheck{{Context: "ci/build", AppSlug: new("circleci")}},
					StrictPolicy: new(false),
				}
				resolver = &GitHubIDResolver{
					appsBySlug:  map[string]*int64{"circleci": new(int64(12345)), "jenkins": new(int64(67890))},
					teamsBySlug: map[string]int64{},
					rolesByName: map[string]int64{},
				}
			})

			It("resolves the AppSlug to IntegrationID", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].IntegrationID).To(Equal(new(int64(12345))))
				Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].Context).To(Equal("ci/build"))
			})
		})

		Context("when status check has an AppSlug not in the pre-loaded map", func() {
			BeforeEach(func() {
				rulesetInput.Spec.Rules.RequiredStatusChecks = &v1alpha1.RequiredStatusChecks{
					Checks: []v1alpha1.StatusCheck{{Context: "ci/build", AppSlug: new("nonexistent-ci")}},
				}
				resolver = &GitHubIDResolver{appsBySlug: map[string]*int64{"other-app": new(int64(100))}, teamsBySlug: map[string]int64{}, rolesByName: map[string]int64{}}
			})

			It("sets IntegrationID to nil (accept any check source)", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].IntegrationID).To(BeNil())
				Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].Context).To(Equal("ci/build"))
			})
		})

		Context("when status check has neither IntegrationID nor AppSlug", func() {
			BeforeEach(func() {
				rulesetInput.Spec.Rules.RequiredStatusChecks = &v1alpha1.RequiredStatusChecks{
					Checks: []v1alpha1.StatusCheck{{Context: "ci/manual-check"}},
				}
			})

			It("keeps IntegrationID as nil", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].IntegrationID).To(BeNil())
			})
		})

		Context("when multiple status checks have mixed configurations", func() {
			BeforeEach(func() {
				rulesetInput.Spec.Rules.RequiredStatusChecks = &v1alpha1.RequiredStatusChecks{
					Checks: []v1alpha1.StatusCheck{
						{Context: "ci/build", IntegrationID: new(int64(999))},
						{Context: "ci/test", AppSlug: new("github-actions")},
						{Context: "ci/lint"},
						{Context: "ci/security", AppSlug: new("snyk")},
					},
					StrictPolicy: new(true),
				}
				resolver = &GitHubIDResolver{
					appsBySlug:  map[string]*int64{"github-actions": new(int64(15368)), "snyk": new(int64(24680))},
					teamsBySlug: map[string]int64{},
					rolesByName: map[string]int64{},
				}
			})

			It("resolves all checks correctly", func() {
				Expect(err).NotTo(HaveOccurred())
				checks := result.Spec.Rules.RequiredStatusChecks.Checks
				Expect(checks).To(HaveLen(4))
				Expect(checks[0].IntegrationID).To(Equal(new(int64(999))))   // preserved
				Expect(checks[1].IntegrationID).To(Equal(new(int64(15368)))) // resolved
				Expect(checks[2].IntegrationID).To(BeNil())                  // no integration
				Expect(checks[3].IntegrationID).To(Equal(new(int64(24680)))) // resolved
			})
		})

		Context("when both bypass actors and status checks need resolution", func() {
			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorSlug: new("dependabot"), ActorType: "Integration", BypassMode: "always"},
				}
				rulesetInput.Spec.Rules.RequiredStatusChecks = &v1alpha1.RequiredStatusChecks{
					Checks: []v1alpha1.StatusCheck{{Context: "ci/build", AppSlug: new("github-actions")}},
				}
				resolver = &GitHubIDResolver{
					appsBySlug:  map[string]*int64{"dependabot": new(int64(29110)), "github-actions": new(int64(15368))},
					teamsBySlug: map[string]int64{},
					rolesByName: map[string]int64{},
				}
			})

			It("resolves both bypass actors and status checks", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.BypassActors[0].ActorID).To(Equal(new(int64(29110))))
				Expect(result.Spec.Rules.RequiredStatusChecks.Checks[0].IntegrationID).To(Equal(new(int64(15368))))
			})
		})

		// ── required pull-request reviewers ───────────────────────────────────

		Context("when there is no pull request rule", func() {
			It("succeeds without modification", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.Rules.PullRequest).To(BeNil())
			})
		})

		Context("when pull request rule has no required reviewers", func() {
			BeforeEach(func() {
				rulesetInput.Spec.Rules.PullRequest = &v1alpha1.PullRequestRule{}
			})

			It("succeeds with an empty reviewer list", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.Rules.PullRequest.RequiredReviewers).To(BeEmpty())
			})
		})

		Context("when reviewer already has an ID set", func() {
			BeforeEach(func() {
				rulesetInput.Spec.Rules.PullRequest = &v1alpha1.PullRequestRule{
					RequiredReviewers: []v1alpha1.RequiredPullRequestReviewer{
						{Reviewer: v1alpha1.PullRequestReviewerEntity{ID: new(int64(42)), Type: "Team"}},
					},
				}
			})

			It("preserves the existing ID", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.Rules.PullRequest.RequiredReviewers[0].Reviewer.ID).To(Equal(new(int64(42))))
			})
		})

		Context("when reviewer has neither ID nor slug", func() {
			BeforeEach(func() {
				rulesetInput.Spec.Rules.PullRequest = &v1alpha1.PullRequestRule{
					RequiredReviewers: []v1alpha1.RequiredPullRequestReviewer{
						{Reviewer: v1alpha1.PullRequestReviewerEntity{Type: "Team"}},
					},
				}
			})

			It("leaves the ID as nil without error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.Rules.PullRequest.RequiredReviewers[0].Reviewer.ID).To(BeNil())
			})
		})

		Context("when reviewer has a Team slug", func() {
			BeforeEach(func() {
				rulesetInput.Spec.Rules.PullRequest = &v1alpha1.PullRequestRule{
					RequiredReviewers: []v1alpha1.RequiredPullRequestReviewer{
						{
							MinimumApprovals: 1,
							FilePatterns:     []string{"*.go"},
							Reviewer:         v1alpha1.PullRequestReviewerEntity{Slug: new("platform-team"), Type: "Team"},
						},
					},
				}
				resolver = &GitHubIDResolver{appsBySlug: map[string]*int64{}, teamsBySlug: map[string]int64{"platform-team": 99}, rolesByName: map[string]int64{}}
			})

			It("resolves the slug to an ID and preserves other fields", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.Rules.PullRequest.RequiredReviewers[0].Reviewer.ID).To(Equal(new(int64(99))))
				Expect(result.Spec.Rules.PullRequest.RequiredReviewers[0].Reviewer.Type).To(Equal("Team"))
				Expect(result.Spec.Rules.PullRequest.RequiredReviewers[0].MinimumApprovals).To(Equal(1))
				Expect(result.Spec.Rules.PullRequest.RequiredReviewers[0].FilePatterns).To(Equal([]string{"*.go"}))
			})
		})

		Context("when reviewer Team slug is not in the pre-loaded map", func() {
			BeforeEach(func() {
				rulesetInput.Spec.Rules.PullRequest = &v1alpha1.PullRequestRule{
					RequiredReviewers: []v1alpha1.RequiredPullRequestReviewer{
						{Reviewer: v1alpha1.PullRequestReviewerEntity{Slug: new("nonexistent-team"), Type: "Team"}},
					},
				}
			})

			It("returns an error referencing the slug", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("nonexistent-team"))
				Expect(err.Error()).To(ContainSubstring("not found"))
			})
		})

		Context("when reviewer has an unsupported type with a slug", func() {
			BeforeEach(func() {
				rulesetInput.Spec.Rules.PullRequest = &v1alpha1.PullRequestRule{
					RequiredReviewers: []v1alpha1.RequiredPullRequestReviewer{
						{Reviewer: v1alpha1.PullRequestReviewerEntity{Slug: new("some-slug"), Type: "UnknownType"}},
					},
				}
			})

			It("returns an unsupported type error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported required reviewer type"))
				Expect(err.Error()).To(ContainSubstring("UnknownType"))
			})
		})

		Context("when multiple reviewers have mixed IDs and slugs", func() {
			BeforeEach(func() {
				rulesetInput.Spec.Rules.PullRequest = &v1alpha1.PullRequestRule{
					RequiredReviewers: []v1alpha1.RequiredPullRequestReviewer{
						{Reviewer: v1alpha1.PullRequestReviewerEntity{ID: new(int64(10)), Type: "Team"}},
						{Reviewer: v1alpha1.PullRequestReviewerEntity{Slug: new("backend-team"), Type: "Team"}},
						{Reviewer: v1alpha1.PullRequestReviewerEntity{Slug: new("security-team"), Type: "Team"}},
					},
				}
				resolver = &GitHubIDResolver{appsBySlug: map[string]*int64{}, teamsBySlug: map[string]int64{"backend-team": 20, "security-team": 30}, rolesByName: map[string]int64{}}
			})

			It("resolves slug-based reviewers and preserves ID-based ones", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.Rules.PullRequest.RequiredReviewers[0].Reviewer.ID).To(Equal(new(int64(10))))
				Expect(result.Spec.Rules.PullRequest.RequiredReviewers[1].Reviewer.ID).To(Equal(new(int64(20))))
				Expect(result.Spec.Rules.PullRequest.RequiredReviewers[2].Reviewer.ID).To(Equal(new(int64(30))))
			})
		})

		Context("when bypass actors and reviewer slugs both need resolution", func() {
			BeforeEach(func() {
				rulesetInput.Spec.BypassActors = []v1alpha1.RulesetBypassActor{
					{ActorSlug: new("platform-team"), ActorType: "Team", BypassMode: "always"},
				}
				rulesetInput.Spec.Rules.PullRequest = &v1alpha1.PullRequestRule{
					RequiredReviewers: []v1alpha1.RequiredPullRequestReviewer{
						{Reviewer: v1alpha1.PullRequestReviewerEntity{Slug: new("security-team"), Type: "Team"}},
					},
				}
				resolver = &GitHubIDResolver{appsBySlug: map[string]*int64{}, teamsBySlug: map[string]int64{"platform-team": 11, "security-team": 22}, rolesByName: map[string]int64{}}
			})

			It("resolves both bypass actor and reviewer slugs independently", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.BypassActors[0].ActorID).To(Equal(new(int64(11))))
				Expect(result.Spec.Rules.PullRequest.RequiredReviewers[0].Reviewer.ID).To(Equal(new(int64(22))))
			})
		})
	})
})
