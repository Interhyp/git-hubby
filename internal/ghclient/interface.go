package ghclient

import (
	"context"

	"github.com/google/go-github/v89/github"
)

// GitHubClient defines the interface for GitHub operations used by reconcilers
type GitHubClient interface {
	// Organization operations
	GetOrganization(ctx context.Context, org string) (*github.Organization, error)
	EditOrganization(ctx context.Context, org string, organization *github.Organization) (*github.Organization, error)
	CreateOrUpdateOrganizationCustomProperties(ctx context.Context, org string, properties []*github.CustomProperty) ([]*github.CustomProperty, error)
	GetAllCustomPropertiesForOrganization(ctx context.Context, org string) ([]*github.CustomProperty, error)
	ListMembers(ctx context.Context, org string) ([]*github.User, error)

	// Repository operations
	GetOrgRepositories(ctx context.Context, org string) ([]*github.Repository, error)
	GetRepository(ctx context.Context, owner, repo string) (*github.Repository, error)
	CreateRepository(ctx context.Context, org string, repo *github.Repository) (*github.Repository, error)
	EditRepository(ctx context.Context, owner, repo string, repository *github.Repository) (*github.Repository, error)
	DeleteRepository(ctx context.Context, owner, repo string) error
	GetAllCustomPropertyValues(ctx context.Context, org string, repo string) ([]*github.CustomPropertyValue, error)
	CreateOrUpdateRepositoryCustomProperties(ctx context.Context, owner string, name string, values []*github.CustomPropertyValue) error

	// Topics operations
	GetAllTopics(ctx context.Context, owner, repo string) ([]string, error)
	ReplaceAllTopics(ctx context.Context, owner, repo string, topics []string) error

	// Autolink operations
	ListAllAutolinks(ctx context.Context, owner, repo string) ([]*github.Autolink, error)
	DeleteAutolink(ctx context.Context, owner, repo string, id int64) error
	CreateAutolink(ctx context.Context, owner, repo string, autolink *github.AutolinkOptions) error

	// DeployKey operations
	ListAllDeployKeys(ctx context.Context, owner, repo string) ([]*github.Key, error)
	DeleteDeployKey(ctx context.Context, owner, repo string, id int64) error
	CreateDeployKey(ctx context.Context, owner, repo string, key *github.Key) error

	// Webhook operations
	ListHooks(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.Hook, error)
	CreateHook(ctx context.Context, owner, repo string, hook *github.Hook) (*github.Hook, error)
	DeleteHook(ctx context.Context, owner, repo string, id int64) error

	// Repository Ruleset operations
	GetRepositoryRuleset(ctx context.Context, owner, repo string, rulesetID int64, includesParents bool) (*github.RepositoryRuleset, error)
	GetAllRepositoryRulesets(ctx context.Context, owner, repo string, includesParents bool) ([]*github.RepositoryRuleset, error)
	CreateRepositoryRuleset(ctx context.Context, owner, repo string, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error)
	UpdateRepositoryRuleset(ctx context.Context, owner, repo string, rulesetID int64, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error)
	DeleteRepositoryRuleset(ctx context.Context, owner, repo string, rulesetID int64) error

	// Organization Ruleset operations
	GetOrganizationRuleset(ctx context.Context, org string, rulesetID int64) (*github.RepositoryRuleset, error)
	GetAllOrganizationRulesets(ctx context.Context, org string, includesParents bool) ([]*github.RepositoryRuleset, error)
	CreateOrganizationRuleset(ctx context.Context, org string, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error)
	UpdateOrganizationRuleset(ctx context.Context, org string, rulesetID int64, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error)
	DeleteOrganizationRuleset(ctx context.Context, org string, rulesetID int64) error

	// Organization Code Security operations
	GetCodeSecurityConfigurationsForOrg(ctx context.Context, org string) ([]*github.CodeSecurityConfiguration, error)
	UpdateCodeSecurityConfigurationForOrg(ctx context.Context, org string, configId int64, config github.CodeSecurityConfiguration) (*github.CodeSecurityConfiguration, error)
	CreateCodeSecurityConfigurationForOrg(ctx context.Context, org string, config github.CodeSecurityConfiguration) (*github.CodeSecurityConfiguration, error)
	DeleteCodeSecurityConfigurationForOrg(ctx context.Context, org string, configId int64) error
	SetCodeSecurityConfigurationAsDefaultForOrg(ctx context.Context, org string, configId int64, newReposParam string) error
	GetDefaultCodeSecurityConfigurationsForOrg(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error)
	AttachCodeSecurityConfigurations(ctx context.Context, org string, cscID int64, scope string, repoIDs []int64) error
	GetRepositoriesAttachedToCodeSecurityConfiguration(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error)

	// Organization Role Assignments operations
	GetAllTeamsAssignedToOrgRole(ctx context.Context, org string, role string) ([]string, error)
	AddOrgRoleAssignmentForTeam(ctx context.Context, org string, slug string, roleID int64) error
	RemoveOrgRoleAssignmentForTeam(ctx context.Context, org string, slug string, roleID int64) error
	GetAllOrgRoles(ctx context.Context, org string) ([]*github.CustomOrgRole, error)

	// Teams operations
	GetAllTeamsForOrg(ctx context.Context, org string) ([]*github.Team, error)
	GetTeamBySlug(ctx context.Context, org string, slug string) (*github.Team, error)
	EditTeamBySlug(ctx context.Context, org string, slug string, team *github.NewTeam) (*github.Team, error)
	CreateTeam(ctx context.Context, org string, team *github.NewTeam) (*github.Team, error)
	DeleteTeamBySlug(ctx context.Context, org string, slug string) error

	// Team members operations
	GetAllTeamMembers(ctx context.Context, org string, slug string) ([]*github.User, error)
	AddTeamMember(ctx context.Context, org string, slug string, username string) error
	RemoveTeamMember(ctx context.Context, org string, slug string, username string) error

	// Team external group operations
	GetExternalGroupNamesToIDForOrg(ctx context.Context, org string) (map[string]int64, error)
	GetExternalGroupsForTeamBySlug(ctx context.Context, org string, slug string) ([]*github.ExternalGroup, error)
	AddExternalGroupToTeamBySlug(ctx context.Context, org string, slug string, group *github.ExternalGroup) error

	// App operations
	GetGitHubAppsInstallations(ctx context.Context, org string) ([]*github.Installation, error)

	// Roles operations
	GetRoleByName(ctx context.Context, org string, roleName string) (*github.CustomOrgRole, error)

	// Actions permissions
	GetActionsPermissionsForOrg(ctx context.Context, org string) (*github.ActionsPermissions, error)
	SetActionsPermissionsForOrg(ctx context.Context, org string, permissions github.ActionsPermissions) (*github.ActionsPermissions, error)
	GetActionsRetentionForOrg(ctx context.Context, org string) (*github.ArtifactPeriod, error)
	SetActionsRetentionForOrg(ctx context.Context, org string, retentionInDays int) error
	GetActionsAllowedForOrg(ctx context.Context, org string) (*github.ActionsAllowed, error)
	SetActionsAllowedForOrg(ctx context.Context, org string, allowedActions github.ActionsAllowed) (*github.ActionsAllowed, error)
	GetActionsDefaultWorkflowPermissionsForOrg(ctx context.Context, org string) (*github.DefaultWorkflowPermissionOrganization, error)
	SetActionsDefaultWorkflowPermissionsForOrg(ctx context.Context, org string, permissions github.DefaultWorkflowPermissionOrganization) (*github.DefaultWorkflowPermissionOrganization, error)
	GetSelfHostedRunnersSettingsForOrg(ctx context.Context, org string) (*github.SelfHostedRunnersSettingsOrganization, error)
	SetSelfHostedRunnersSettingsForOrg(ctx context.Context, org string, settings github.SelfHostedRunnersSettingsOrganizationOpt) error
	GetActionsEnabledRepositoriesForOrg(ctx context.Context, org string) ([]*github.Repository, error)
	SetActionsEnabledRepositoriesForOrg(ctx context.Context, org string, repoIds []int64) error
	SetActionsEnabledForRepo(ctx context.Context, owner string, repoID int64, enabled bool) error
	GetAccessLevelForExternalWorkflowsForRepo(ctx context.Context, owner string, repo string) (*github.RepositoryActionsAccessLevel, error)
	SetAccessLevelForExternalWorkflowsForRepo(ctx context.Context, owner string, repo string, accessLevel github.RepositoryActionsAccessLevel) error
	GetRunnerGroupsForOrg(ctx context.Context, org string) ([]*github.RunnerGroup, error)
	CreateRunnerGroupForOrg(ctx context.Context, org string, createRequest github.CreateRunnerGroupRequest) (*github.RunnerGroup, error)
	DeleteRunnerGroupForOrg(ctx context.Context, org string, groupID int64) error
	UpdateRunnerGroupForOrg(ctx context.Context, resource string, groupID int64, request github.UpdateRunnerGroupRequest) (*github.RunnerGroup, error)
	SetSelectedRepositoriesForRunnerGroup(ctx context.Context, resource string, groupID int64, selectedRepositoryIDs []int64) error
	GetSelectedRepositoriesForRunnerGroup(ctx context.Context, resource string, groupID int64) ([]*github.Repository, error)

	// Rate Limit
	GetRateLimit(ctx context.Context) (*github.RateLimits, error)
}
