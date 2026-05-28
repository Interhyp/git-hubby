package ghclientmock

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v86/github"
)

// Helper methods for testing

// SetOrganizationNotFound configures the mock to return a 404 error for GetOrganization
func (m *MockGitHubClientWrapper) SetOrganizationNotFound(org string) {
	m.GetOrganizationFunc = func(ctx context.Context, orgName string) (*github.Organization, error) {
		if orgName == org {
			return nil, &github.ErrorResponse{
				Message: "expected 404 Not Found",
				Response: &http.Response{
					StatusCode: http.StatusNotFound,
				},
			}
		}
		return nil, nil
	}
}

// SetRepositoryNotFound configures the mock to return a 404 error for GetRepository
func (m *MockGitHubClientWrapper) SetRepositoryNotFound(owner, repo string) {
	m.GetRepositoryFunc = func(ctx context.Context, repoOwner, repoName string) (*github.Repository, error) {
		if repoOwner == owner && repoName == repo {
			return nil, &github.ErrorResponse{
				Message: "expected 404 Not Found",
				Response: &http.Response{
					StatusCode: http.StatusNotFound,
				},
			}
		}
		return nil, nil
	}
}

// SetRepositoryArchived configures the mock to return a repository that is archived
func (m *MockGitHubClientWrapper) SetRepositoryArchived(owner, repo string, archived bool) {
	m.GetRepositoryFunc = func(ctx context.Context, repoOwner, repoName string) (*github.Repository, error) {
		if repoOwner == owner && repoName == repo {
			return &github.Repository{
				ID:       new(int64(12345)),
				Name:     new(repo),
				FullName: new(fmt.Sprintf("%s/%s", owner, repo)),
				Owner:    &github.User{Login: new(owner)},
				Archived: new(archived),
			}, nil
		}
		return m.GetRepository(ctx, repoOwner, repoName)
	}
}

// SetTeamNotFound configures the mock to return a 404 error for GetRepository and an empty list for GetAllTeamsForOrg,
// simulating the case where a team does not exist in the organization.
func (m *MockGitHubClientWrapper) SetTeamNotFound(owners []string, team string) {
	m.GetTeamBySlugFunc = func(ctx context.Context, teamOwner, teamName string) (*github.Team, error) {
		for _, owner := range owners {
			if teamOwner == owner && teamName == team {
				return nil, &github.ErrorResponse{
					Message: "expected 404 Not Found",
					Response: &http.Response{
						StatusCode: http.StatusNotFound,
					},
				}
			}
			continue
		}
		return nil, nil
	}
	m.GetAllTeamsForOrgFunc = func(ctx context.Context, org string) ([]*github.Team, error) {
		return []*github.Team{}, nil
	}
}

// SetError configures the mock to return an error for all operations
func (m *MockGitHubClientWrapper) SetError(err error) {
	m.GetOrganizationFunc = func(ctx context.Context, org string) (*github.Organization, error) {
		return nil, err
	}
	m.EditOrganizationFunc = func(ctx context.Context, org string, organization *github.Organization) (*github.Organization, error) {
		return nil, err
	}
	m.CreateOrUpdateOrganizationCustomPropertiesFunc = func(ctx context.Context, org string, properties []*github.CustomProperty) ([]*github.CustomProperty, error) {
		return nil, err
	}
	m.GetAllOrganizationCustomPropertiesFunc = func(ctx context.Context, org string) ([]*github.CustomProperty, error) {
		return nil, err
	}
	m.GetRepositoryFunc = func(ctx context.Context, owner, repo string) (*github.Repository, error) {
		return nil, err
	}
	m.CreateRepositoryFunc = func(ctx context.Context, org string, repo *github.Repository) (*github.Repository, error) {
		return nil, err
	}
	m.EditRepositoryFunc = func(ctx context.Context, owner, repo string, repository *github.Repository) (*github.Repository, error) {
		return nil, err
	}
	m.ListHooksFunc = func(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.Hook, error) {
		return nil, err
	}
	m.CreateHookFunc = func(ctx context.Context, owner, repo string, hook *github.Hook) (*github.Hook, error) {
		return nil, err
	}
	m.DeleteHookFunc = func(ctx context.Context, owner, repo string, id int64) error {
		return err
	}
	m.GetRepositoryRulesetFunc = func(ctx context.Context, owner, repo string, rulesetID int64, includesParents bool) (*github.RepositoryRuleset, error) {
		return nil, err
	}
	m.GetAllRepositoryRulesetsFunc = func(ctx context.Context, owner, repo string, includesParents bool) ([]*github.RepositoryRuleset, error) {
		return nil, err
	}
	m.CreateRepositoryRulesetFunc = func(ctx context.Context, owner, repo string, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
		return nil, err
	}
	m.UpdateRepositoryRulesetFunc = func(ctx context.Context, owner, repo string, rulesetID int64, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
		return nil, err
	}
	m.DeleteRepositoryRulesetFunc = func(ctx context.Context, owner, repo string, rulesetID int64) error {
		return err
	}
	m.GetOrganizationRulesetFunc = func(ctx context.Context, org string, rulesetID int64) (*github.RepositoryRuleset, error) {
		return nil, err
	}
	m.GetAllOrganizationRulesetsFunc = func(ctx context.Context, org string, includesParents bool) ([]*github.RepositoryRuleset, error) {
		return nil, err
	}
	m.GetCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfiguration, error) {
		return nil, err
	}
	m.UpdateCodeSecurityConfigurationForOrgFunc = func(ctx context.Context, org string, configId int64, config github.CodeSecurityConfiguration) (*github.CodeSecurityConfiguration, error) {
		return nil, err
	}
	m.CreateCodeSecurityConfigurationForOrgFunc = func(ctx context.Context, org string, config github.CodeSecurityConfiguration) (*github.CodeSecurityConfiguration, error) {
		return nil, err
	}
	m.DeleteCodeSecurityConfigurationForOrgFunc = func(ctx context.Context, org string, configId int64) error {
		return err
	}
	m.SetCodeSecurityConfigurationAsDefaultForOrgFunc = func(ctx context.Context, org string, configId int64, newReposParam string) error {
		return err
	}
	m.GetDefaultCodeSecurityConfigurationsForOrgFunc = func(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
		return nil, err
	}
	m.GetTeamBySlugFunc = func(ctx context.Context, org string, slug string) (*github.Team, error) {
		return nil, err
	}
	m.GetAllTeamsForOrgFunc = func(ctx context.Context, org string) ([]*github.Team, error) {
		return nil, err
	}
}

// Reset clears all recorded calls and resets all functions to nil
func (m *MockGitHubClientWrapper) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.OrganizationCalls = make([]OrgCall, 0)
	m.RoleAssignmentCalls = make([]RoleAssignmentCall, 0)
	m.CustomPropertiesCalls = make([]CustomPropCall, 0)
	m.RepositoryCalls = make([]RepoCall, 0)
	m.WebhookCalls = make([]WebhookCall, 0)
	m.RulesetCalls = make([]RulesetCall, 0)
	m.OrganizationRulesetCalls = make([]OrgRulesetCall, 0)
	m.RateLimitCalls = make([]RateLimitCall, 0)
	m.CodeSecurityConfigurationCalls = make([]CodeSecurityConfigurationCall, 0)
	m.TeamCalls = make([]TeamCall, 0)
	m.RoleCalls = make([]RoleCall, 0)
	m.ActionsCalls = make([]ActionsCall, 0)

	m.GetOrganizationFunc = nil
	m.EditOrganizationFunc = nil
	m.CreateOrUpdateOrganizationCustomPropertiesFunc = nil
	m.GetAllOrganizationCustomPropertiesFunc = nil
	m.GetRepositoryFunc = nil
	m.CreateRepositoryFunc = nil
	m.EditRepositoryFunc = nil
	m.DeleteRepositoryFunc = nil
	m.ListHooksFunc = nil
	m.CreateHookFunc = nil
	m.DeleteHookFunc = nil
	m.GetRepositoryRulesetFunc = nil
	m.GetAllRepositoryRulesetsFunc = nil
	m.CreateRepositoryRulesetFunc = nil
	m.UpdateRepositoryRulesetFunc = nil
	m.DeleteRepositoryRulesetFunc = nil
	m.GetOrganizationRulesetFunc = nil
	m.GetAllOrganizationRulesetsFunc = nil
	m.CreateOrganizationRulesetFunc = nil
	m.UpdateOrganizationRulesetFunc = nil
	m.DeleteOrganizationRulesetFunc = nil
	m.GetCodeSecurityConfigurationsForOrgFunc = nil
	m.UpdateCodeSecurityConfigurationForOrgFunc = nil
	m.CreateCodeSecurityConfigurationForOrgFunc = nil
	m.DeleteCodeSecurityConfigurationForOrgFunc = nil
	m.SetCodeSecurityConfigurationAsDefaultForOrgFunc = nil
	m.GetDefaultCodeSecurityConfigurationsForOrgFunc = nil
	m.GetTeamBySlugFunc = nil
	m.GetRoleByNameFunc = nil
	m.GetActionsPermissionsForOrgFunc = nil
	m.SetActionsPermissionsForOrgFunc = nil
	m.GetActionsRetentionForOrgFunc = nil
	m.SetActionsRetentionForOrgFunc = nil
	m.GetActionsAllowedForOrgFunc = nil
	m.SetActionsAllowedForOrgFunc = nil
	m.GetActionsDefaultWorkflowPermissionsForOrgFunc = nil
	m.SetActionsDefaultWorkflowPermissionsForOrgFunc = nil
	m.GetSelfHostedRunnersSettingsForOrgFunc = nil
	m.SetSelfHostedRunnersSettingsForOrgFunc = nil
	m.GetActionsEnabledRepositoriesForOrgFunc = nil
	m.SetActionsEnabledForRepoFunc = nil
	m.GetAccessLevelForExternalWorkflowsForRepoFunc = nil
	m.SetAccessLevelForExternalWorkflowsForRepoFunc = nil
	m.GetRunnerGroupsForOrgFunc = nil
	m.CreateRunnerGroupForOrgFunc = nil
	m.DeleteRunnerGroupForOrgFunc = nil
}
