//nolint:lll
package ghclientmock

import (
	"context"
	"errors"
	"sync"

	"github.com/Interhyp/git-hubby/internal/ghclient"
	"github.com/google/go-github/v89/github"
)

type GitHubMockClientFactory struct {
	mockClient ghclient.GitHubClient
}

func NewGitHubMockClientFactory(mockClient *MockGitHubClientWrapper) *GitHubMockClientFactory {
	return &GitHubMockClientFactory{
		mockClient: mockClient,
	}
}

func (m *GitHubMockClientFactory) GetClient(_ context.Context, _ string, _ ghclient.AppConfig) (ghclient.GitHubClient, error) {
	if m.mockClient == nil {
		return nil, errors.New("mock GitHub client not set")
	}
	return m.mockClient, nil
}

func (m *GitHubMockClientFactory) GetGitHubClientAndCheckRateLimit(_ context.Context, _ string, _ ghclient.AppConfig, _ int) (ghclient.GitHubClient, error) {
	if m.mockClient == nil {
		return nil, errors.New("mock GitHub client not set")
	}
	return m.mockClient, nil
}

// MockGitHubClientWrapper is a mock implementation of GitHubClientWrapper for testing
type MockGitHubClientWrapper struct {
	// Organization mocks
	GetOrganizationFunc                            func(ctx context.Context, org string) (*github.Organization, error)
	EditOrganizationFunc                           func(ctx context.Context, org string, organization *github.Organization) (*github.Organization, error)
	CreateOrUpdateOrganizationCustomPropertiesFunc func(ctx context.Context, org string, properties []*github.CustomProperty) ([]*github.CustomProperty, error)
	GetAllOrganizationCustomPropertiesFunc         func(ctx context.Context, org string) ([]*github.CustomProperty, error)
	GetAllCustomPropertyValuesFunc                 func(ctx context.Context, org string, repo string) ([]*github.CustomPropertyValue, error)
	CreateOrUpdateRepositoryCustomPropertiesFunc   func(ctx context.Context, owner string, name string, values []*github.CustomPropertyValue) error
	ListMembersFunc                                func(ctx context.Context, org string) ([]*github.User, error)

	// Repository mocks
	GetRepositoryFunc                 func(ctx context.Context, owner, repo string) (*github.Repository, error)
	CreateRepositoryFunc              func(ctx context.Context, org string, repo *github.Repository) (*github.Repository, error)
	EditRepositoryFunc                func(ctx context.Context, owner, repo string, repository *github.Repository) (*github.Repository, error)
	DeleteRepositoryFunc              func(ctx context.Context, owner, repo string) error
	GetOrgRepositoriesFunc            func(ctx context.Context, org string) ([]*github.Repository, error)
	GetAllRepositoryTeamsFunc         func(ctx context.Context, owner, repo string) ([]*github.Team, error)
	AddRepositoryTeamFunc             func(ctx context.Context, org, slug, owner, repo, permission string) error
	RemoveTeamFromRepoFunc            func(ctx context.Context, org, slug, owner, repo string) error
	GetAllRepositoryCollaboratorsFunc func(ctx context.Context, owner, repo string) ([]*github.User, error)
	AddRepositoryCollaboratorFunc     func(ctx context.Context, owner, repo, username, permission string) error
	RemoveRepositoryCollaboratorFunc  func(ctx context.Context, owner, repo, username string) error

	// Topic mocks
	GetAllTopicsFunc     func(ctx context.Context, owner, repo string) ([]string, error)
	ReplaceAllTopicsFunc func(ctx context.Context, owner, repo string, topics []string) error

	// Autolink mocks
	ListAllAutolinksFunc func(ctx context.Context, owner, repo string) ([]*github.Autolink, error)
	DeleteAutolinkFunc   func(ctx context.Context, owner, repo string, id int64) error
	CreateAutolinkFunc   func(ctx context.Context, owner, repo string, autolink *github.AutolinkOptions) error

	// Deploy key mocks
	ListAllDeployKeysFunc func(ctx context.Context, owner, repo string) ([]*github.Key, error)
	CreateDeployKeyFunc   func(ctx context.Context, owner, repo string, key *github.Key) error
	DeleteDeployKeyFunc   func(ctx context.Context, owner, repo string, id int64) error

	// Webhook mocks
	ListHooksFunc  func(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.Hook, error)
	CreateHookFunc func(ctx context.Context, owner, repo string, hook *github.Hook) (*github.Hook, error)
	DeleteHookFunc func(ctx context.Context, owner, repo string, id int64) error

	// Rate limit mock
	GetRateLimitFunc func(ctx context.Context) (*github.RateLimits, error)

	// Ruleset mocks
	GetRepositoryRulesetFunc     func(ctx context.Context, owner, repo string, rulesetID int64, includesParents bool) (*github.RepositoryRuleset, error)
	GetAllRepositoryRulesetsFunc func(ctx context.Context, owner, repo string, includesParents bool) ([]*github.RepositoryRuleset, error)
	CreateRepositoryRulesetFunc  func(ctx context.Context, owner, repo string, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error)
	UpdateRepositoryRulesetFunc  func(ctx context.Context, owner, repo string, rulesetID int64, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error)
	DeleteRepositoryRulesetFunc  func(ctx context.Context, owner, repo string, rulesetID int64) error

	// Organization ruleset mocks
	GetOrganizationRulesetFunc     func(ctx context.Context, org string, rulesetID int64) (*github.RepositoryRuleset, error)
	GetAllOrganizationRulesetsFunc func(ctx context.Context, org string, includesParents bool) ([]*github.RepositoryRuleset, error)
	CreateOrganizationRulesetFunc  func(ctx context.Context, org string, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error)
	UpdateOrganizationRulesetFunc  func(ctx context.Context, org string, rulesetID int64, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error)
	DeleteOrganizationRulesetFunc  func(ctx context.Context, org string, rulesetID int64) error

	// Code Security Configuration mocks
	GetCodeSecurityConfigurationsForOrgFunc                func(ctx context.Context, org string) ([]*github.CodeSecurityConfiguration, error)
	UpdateCodeSecurityConfigurationForOrgFunc              func(ctx context.Context, org string, configId int64, config github.CodeSecurityConfiguration) (*github.CodeSecurityConfiguration, error)
	CreateCodeSecurityConfigurationForOrgFunc              func(ctx context.Context, org string, config github.CodeSecurityConfiguration) (*github.CodeSecurityConfiguration, error)
	DeleteCodeSecurityConfigurationForOrgFunc              func(ctx context.Context, org string, configId int64) error
	SetCodeSecurityConfigurationAsDefaultForOrgFunc        func(ctx context.Context, org string, configId int64, newReposParam string) error
	GetDefaultCodeSecurityConfigurationsForOrgFunc         func(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error)
	AttachCodeSecurityConfigurationsFunc                   func(ctx context.Context, org string, cscID int64, scope string, repoIDs []int64) error
	GetRepositoriesAttachedToCodeSecurityConfigurationFunc func(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error)

	// Team and Role mocks
	GetAllTeamsForOrgFunc func(ctx context.Context, org string) ([]*github.Team, error)
	GetTeamBySlugFunc     func(ctx context.Context, org string, slug string) (*github.Team, error)
	EditTeamBySlugFunc    func(ctx context.Context, org string, slug string, team *github.NewTeam) (*github.Team, error)
	CreateTeamFunc        func(ctx context.Context, org string, team *github.NewTeam) (*github.Team, error)
	DeleteTeamBySlugFunc  func(ctx context.Context, org string, slug string) error
	GetRoleByNameFunc     func(ctx context.Context, org string, roleName string) (*github.CustomOrgRole, error)

	// Team members operations
	GetAllTeamMembersFunc    func(ctx context.Context, org string, slug string) ([]*github.User, error)
	AddMemberToTeamFunc      func(ctx context.Context, org string, slug string, username string) error
	RemoveMemberFromTeamFunc func(ctx context.Context, org string, slug string, username string) error

	// Team IDP group operations
	GetExternalGroupsForTeamBySlugFunc  func(ctx context.Context, org string, slug string) ([]*github.ExternalGroup, error)
	GetExternalGroupNamesToIDForOrgFunc func(ctx context.Context, org string) (map[string]int64, error)
	AddExternalGroupToTeamBySlugFunc    func(ctx context.Context, org string, slug string, group *github.ExternalGroup) error

	// Organization role assignments
	GetAllTeamsAssignedToOrgRoleFunc   func(ctx context.Context, org string, role string) ([]string, error)
	AddOrgRoleAssignmentForTeamFunc    func(ctx context.Context, org string, slug string, role int64) error
	RemoveOrgRoleAssignmentForTeamFunc func(ctx context.Context, org string, slug string, roleID int64) error
	GetAllOrgRolesFunc                 func(ctx context.Context, org string) ([]*github.CustomOrgRole, error)

	// Apps mocks
	GetGitHubAppsInstallationsFunc func(ctx context.Context, org string) ([]*github.Installation, error)

	// Actions mocks
	GetActionsPermissionsForOrgFunc                func(ctx context.Context, org string) (*github.ActionsPermissions, error)
	SetActionsPermissionsForOrgFunc                func(ctx context.Context, org string, permissions github.ActionsPermissions) (*github.ActionsPermissions, error)
	GetActionsRetentionForOrgFunc                  func(ctx context.Context, org string) (*github.ArtifactPeriod, error)
	SetActionsRetentionForOrgFunc                  func(ctx context.Context, org string, retentionInDays int) error
	GetActionsAllowedForOrgFunc                    func(ctx context.Context, org string) (*github.ActionsAllowed, error)
	SetActionsAllowedForOrgFunc                    func(ctx context.Context, org string, allowedActions github.ActionsAllowed) (*github.ActionsAllowed, error)
	GetActionsDefaultWorkflowPermissionsForOrgFunc func(ctx context.Context, org string) (*github.DefaultWorkflowPermissionOrganization, error)
	SetActionsDefaultWorkflowPermissionsForOrgFunc func(ctx context.Context, org string, permissions github.DefaultWorkflowPermissionOrganization) (*github.DefaultWorkflowPermissionOrganization, error)
	GetSelfHostedRunnersSettingsForOrgFunc         func(ctx context.Context, org string) (*github.SelfHostedRunnersSettingsOrganization, error)
	SetSelfHostedRunnersSettingsForOrgFunc         func(ctx context.Context, org string, settings github.SelfHostedRunnersSettingsOrganizationOpt) error
	GetActionsEnabledRepositoriesForOrgFunc        func(ctx context.Context, org string) ([]*github.Repository, error)
	SetActionsEnabledRepositoriesForOrgFunc        func(ctx context.Context, org string, repoIDs []int64) error
	SetActionsEnabledForRepoFunc                   func(ctx context.Context, owner string, repoID int64, enabled bool) error
	GetAccessLevelForExternalWorkflowsForRepoFunc  func(ctx context.Context, owner string, repo string) (*github.RepositoryActionsAccessLevel, error)
	SetAccessLevelForExternalWorkflowsForRepoFunc  func(ctx context.Context, owner string, repo string, accessLevel github.RepositoryActionsAccessLevel) error
	GetRunnerGroupsForOrgFunc                      func(ctx context.Context, org string) ([]*github.RunnerGroup, error)
	CreateRunnerGroupForOrgFunc                    func(ctx context.Context, org string, createRequest github.CreateRunnerGroupRequest) (*github.RunnerGroup, error)
	UpdateRunnerGroupForOrgFunc                    func(ctx context.Context, org string, groupID int64, updateRequest github.UpdateRunnerGroupRequest) (*github.RunnerGroup, error)
	DeleteRunnerGroupForOrgFunc                    func(ctx context.Context, org string, groupID int64) error
	GetSelectedRepositoriesForRunnerGroupFunc      func(ctx context.Context, org string, groupID int64) ([]*github.Repository, error)
	SetSelectedRepositoriesForRunnerGroupFunc      func(ctx context.Context, org string, groupID int64, selectedRepositoryIDs []int64) error

	// Call tracking
	mu                             sync.Mutex
	OrganizationCalls              []OrgCall
	RoleAssignmentCalls            []RoleAssignmentCall
	CustomPropertiesCalls          []CustomPropCall
	RepositoryCalls                []RepoCall
	WebhookCalls                   []WebhookCall
	RulesetCalls                   []RulesetCall
	OrganizationRulesetCalls       []OrgRulesetCall
	RateLimitCalls                 []RateLimitCall
	CodeSecurityConfigurationCalls []CodeSecurityConfigurationCall
	TeamCalls                      []TeamCall
	TeamMemberCalls                []TeamMemberCall
	ExternalGroupCalls             []ExternalGroupCall
	RoleCalls                      []RoleCall
	ActionsCalls                   []ActionsCall
	EnterpriseAppsCalls            []AppsCall
}

// NewMockGitHubClientWrapper creates a new mock repository with default implementations
func NewMockGitHubClientWrapper() *MockGitHubClientWrapper {
	return &MockGitHubClientWrapper{
		OrganizationCalls:              make([]OrgCall, 0),
		RoleAssignmentCalls:            make([]RoleAssignmentCall, 0),
		CustomPropertiesCalls:          make([]CustomPropCall, 0),
		RepositoryCalls:                make([]RepoCall, 0),
		WebhookCalls:                   make([]WebhookCall, 0),
		RulesetCalls:                   make([]RulesetCall, 0),
		OrganizationRulesetCalls:       make([]OrgRulesetCall, 0),
		RateLimitCalls:                 make([]RateLimitCall, 0),
		CodeSecurityConfigurationCalls: make([]CodeSecurityConfigurationCall, 0),
		TeamCalls:                      make([]TeamCall, 0),
		RoleCalls:                      make([]RoleCall, 0),
		ActionsCalls:                   make([]ActionsCall, 0),
		EnterpriseAppsCalls:            make([]AppsCall, 0),
	}
}
