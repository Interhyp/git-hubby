package ghclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/gofri/go-github-pagination/githubpagination"
	"github.com/google/go-github/v89/github"
)

// GitHubClientWrapper is the production implementation of GitHubClient using the real GitHub API.
// It handles closing of the response body and supplying http error codes in the case of http errors.
type GitHubClientWrapper struct {
	client *github.Client
}

// Organization operations

func (g *GitHubClientWrapper) GetOrganization(ctx context.Context, org string) (*github.Organization, error) {
	result, response, err := g.client.Organizations.Get(ctx, org)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) EditOrganization(ctx context.Context, org string, organization *github.Organization) (*github.Organization, error) {
	result, response, err := g.client.Organizations.Edit(ctx, org, organization)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) CreateOrUpdateOrganizationCustomProperties(ctx context.Context, org string, properties []*github.CustomProperty) ([]*github.CustomProperty, error) {
	result, response, err := g.client.Organizations.CreateOrUpdateCustomProperties(ctx, org, properties)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) GetAllCustomPropertiesForOrganization(ctx context.Context, org string) ([]*github.CustomProperty, error) {
	result, response, err := g.client.Organizations.GetAllCustomProperties(ctx, org)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) ListMembers(ctx context.Context, org string) ([]*github.User, error) {
	members, response, err := g.client.Organizations.ListMembers(ctx, org, nil)
	defer _closeBody(response)
	return members, _handleErrorResponse(response, err)
}

// Repository operations

func (g *GitHubClientWrapper) GetOrgRepositories(ctx context.Context, org string) ([]*github.Repository, error) {
	result, response, err := g.client.Repositories.ListByOrg(ctx, org, nil)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) GetRepository(ctx context.Context, owner, repo string) (*github.Repository, error) {
	result, response, err := g.client.Repositories.Get(ctx, owner, repo)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) CreateRepository(ctx context.Context, org string, repo *github.Repository) (*github.Repository, error) {
	result, response, err := g.client.Repositories.Create(ctx, org, repo)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) EditRepository(ctx context.Context, owner, repo string, repository *github.Repository) (*github.Repository, error) {
	result, response, err := g.client.Repositories.Edit(ctx, owner, repo, repository)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) DeleteRepository(ctx context.Context, owner, repo string) error {
	response, err := g.client.Repositories.Delete(ctx, owner, repo)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) GetAllCustomPropertyValues(ctx context.Context, org string, repo string) ([]*github.CustomPropertyValue, error) {
	result, response, err := g.client.Repositories.GetAllCustomPropertyValues(ctx, org, repo)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) CreateOrUpdateRepositoryCustomProperties(ctx context.Context, org string, name string, values []*github.CustomPropertyValue) error {
	response, err := g.client.Repositories.CreateOrUpdateCustomProperties(ctx, org, name, values)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

// Topics operations

func (g *GitHubClientWrapper) GetAllTopics(ctx context.Context, owner, repo string) ([]string, error) {
	result, response, err := g.client.Repositories.ListAllTopics(ctx, owner, repo, nil)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) ReplaceAllTopics(ctx context.Context, owner, repo string, topics []string) error {
	_, response, err := g.client.Repositories.ReplaceAllTopics(ctx, owner, repo, topics)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

// Autolink operations

func (g *GitHubClientWrapper) ListAllAutolinks(ctx context.Context, owner, repo string) ([]*github.Autolink, error) {
	result, response, err := g.client.Repositories.ListAutolinks(ctx, owner, repo)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) DeleteAutolink(ctx context.Context, owner, repo string, id int64) error {
	response, err := g.client.Repositories.DeleteAutolink(ctx, owner, repo, id)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) CreateAutolink(ctx context.Context, owner, repo string, autolink *github.AutolinkOptions) error {
	_, response, err := g.client.Repositories.AddAutolink(ctx, owner, repo, autolink)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

// DeployKeys operations

func (g *GitHubClientWrapper) ListAllDeployKeys(ctx context.Context, owner, repo string) ([]*github.Key, error) {
	result, response, err := g.client.Repositories.ListKeys(ctx, owner, repo, nil)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) DeleteDeployKey(ctx context.Context, owner, repo string, id int64) error {
	response, err := g.client.Repositories.DeleteKey(ctx, owner, repo, id)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) CreateDeployKey(ctx context.Context, owner, repo string, key *github.Key) error {
	_, response, err := g.client.Repositories.CreateKey(ctx, owner, repo, key)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

// Webhook operations

func (g *GitHubClientWrapper) ListHooks(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.Hook, error) {
	result, response, err := g.client.Repositories.ListHooks(ctx, owner, repo, opts)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) CreateHook(ctx context.Context, owner, repo string, hook *github.Hook) (*github.Hook, error) {
	result, response, err := g.client.Repositories.CreateHook(ctx, owner, repo, hook)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) DeleteHook(ctx context.Context, owner, repo string, id int64) error {
	response, err := g.client.Repositories.DeleteHook(ctx, owner, repo, id)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

// Repository Ruleset operations

func (g *GitHubClientWrapper) GetRepositoryRuleset(ctx context.Context, owner, repo string, rulesetID int64, includesParents bool) (*github.RepositoryRuleset, error) {
	ruleset, response, err := g.client.Repositories.GetRuleset(ctx, owner, repo, rulesetID, includesParents)
	defer _closeBody(response)
	return ruleset, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) GetAllRepositoryRulesets(ctx context.Context, owner, repo string, includesParents bool) ([]*github.RepositoryRuleset, error) {
	opts := &github.RepositoryListRulesetsOptions{IncludesParents: &includesParents}
	rulesets, response, err := g.client.Repositories.GetAllRulesets(ctx, owner, repo, opts)
	defer _closeBody(response)
	return rulesets, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) CreateRepositoryRuleset(ctx context.Context, owner, repo string, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
	result, response, err := g.client.Repositories.CreateRuleset(ctx, owner, repo, *ruleset)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) UpdateRepositoryRuleset(ctx context.Context, owner, repo string, rulesetID int64, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
	result, response, err := g.client.Repositories.UpdateRuleset(ctx, owner, repo, rulesetID, *ruleset)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) DeleteRepositoryRuleset(ctx context.Context, owner, repo string, rulesetID int64) error {
	response, err := g.client.Repositories.DeleteRuleset(ctx, owner, repo, rulesetID)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

// Organization Ruleset operations

func (g *GitHubClientWrapper) GetOrganizationRuleset(ctx context.Context, org string, rulesetID int64) (*github.RepositoryRuleset, error) {
	ruleset, response, err := g.client.Organizations.GetRepositoryRuleset(ctx, org, rulesetID)
	defer _closeBody(response)
	return ruleset, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) GetAllOrganizationRulesets(ctx context.Context, org string, includeParents bool) ([]*github.RepositoryRuleset, error) {
	rulesets, response, err := g.client.Organizations.ListAllRepositoryRulesets(ctx, org, nil)
	defer _closeBody(response)
	if !includeParents {
		var filteredRulesets []*github.RepositoryRuleset
		for _, ruleset := range rulesets {
			if ruleset.SourceType != nil && *ruleset.SourceType == github.RulesetSourceTypeOrganization {
				filteredRulesets = append(filteredRulesets, ruleset)
			}
		}
		rulesets = filteredRulesets
	}
	return rulesets, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) CreateOrganizationRuleset(ctx context.Context, org string, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
	result, response, err := g.client.Organizations.CreateRepositoryRuleset(ctx, org, *ruleset)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) UpdateOrganizationRuleset(ctx context.Context, org string, rulesetID int64, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
	result, response, err := g.client.Organizations.UpdateRepositoryRuleset(ctx, org, rulesetID, *ruleset)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) DeleteOrganizationRuleset(ctx context.Context, org string, rulesetID int64) error {
	response, err := g.client.Organizations.DeleteRepositoryRuleset(ctx, org, rulesetID)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

// Code Security operations

func (g *GitHubClientWrapper) GetCodeSecurityConfigurationsForOrg(ctx context.Context, org string) ([]*github.CodeSecurityConfiguration, error) {
	result, response, err := g.client.Organizations.ListCodeSecurityConfigurations(ctx, org, nil)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) UpdateCodeSecurityConfigurationForOrg(ctx context.Context, org string, configId int64, config github.CodeSecurityConfiguration) (*github.CodeSecurityConfiguration, error) {
	result, response, err := g.client.Organizations.UpdateCodeSecurityConfiguration(ctx, org, configId, config)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) CreateCodeSecurityConfigurationForOrg(ctx context.Context, org string, config github.CodeSecurityConfiguration) (*github.CodeSecurityConfiguration, error) {
	result, response, err := g.client.Organizations.CreateCodeSecurityConfiguration(ctx, org, config)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) DeleteCodeSecurityConfigurationForOrg(ctx context.Context, org string, configId int64) error {
	response, err := g.client.Organizations.DeleteCodeSecurityConfiguration(ctx, org, configId)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) SetCodeSecurityConfigurationAsDefaultForOrg(ctx context.Context, org string, configId int64, newReposParam string) error {
	_, response, err := g.client.Organizations.SetDefaultCodeSecurityConfiguration(ctx, org, configId, newReposParam)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) GetDefaultCodeSecurityConfigurationsForOrg(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
	result, response, err := g.client.Organizations.ListDefaultCodeSecurityConfigurations(ctx, org)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) AttachCodeSecurityConfigurations(ctx context.Context, org string, cscID int64, scope string, repoIDs []int64) error {
	response, err := g.client.Organizations.AttachCodeSecurityConfigurationToRepositories(ctx, org, cscID, scope, repoIDs)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) GetRepositoriesAttachedToCodeSecurityConfiguration(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
	result, response, err := g.client.Organizations.ListCodeSecurityConfigurationRepositories(ctx, org, cscID, nil)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

// Organization Role Assignments operations
func (g *GitHubClientWrapper) GetAllTeamsAssignedToOrgRole(ctx context.Context, org string, roleName string) ([]string, error) {
	result := make([]string, 0)
	roleID, err := g.getRoleIDForRoleName(ctx, org, roleName)
	if err != nil {
		return result, err
	}
	teams, response, err := g.client.Organizations.ListTeamsAssignedToOrgRole(ctx, org, roleID, nil)
	defer _closeBody(response)
	if err != nil {
		return result, _handleErrorResponse(response, err)
	}
	for _, team := range teams {
		result = append(result, team.GetSlug())
	}
	return result, nil
}

func (g *GitHubClientWrapper) GetAllOrgRoles(ctx context.Context, org string) ([]*github.CustomOrgRole, error) {
	roles, response, err := g.client.Organizations.ListRoles(ctx, org)
	defer _closeBody(response)
	if err != nil {
		return nil, _handleErrorResponse(response, err)
	}
	return roles.CustomRepoRoles, nil
}

func (g *GitHubClientWrapper) AddOrgRoleAssignmentForTeam(ctx context.Context, org string, team string, roleID int64) error {
	response, err := g.client.Organizations.AssignOrgRoleToTeam(ctx, org, team, roleID)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) RemoveOrgRoleAssignmentForTeam(ctx context.Context, org string, team string, roleID int64) error {
	response, err := g.client.Organizations.RemoveOrgRoleFromTeam(ctx, org, team, roleID)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) getRoleIDForRoleName(ctx context.Context, org string, role string) (int64, error) {
	var roleID int64
	roles, response, err := g.client.Organizations.ListRoles(ctx, org)
	defer _closeBody(response)
	if err != nil {
		return roleID, err
	}
	for _, currentRole := range roles.CustomRepoRoles {
		if *currentRole.Name == role {
			roleID = *currentRole.ID
			break
		}
	}
	if roleID == 0 {
		return roleID, fmt.Errorf("role %s not found for organization %s", role, org)
	}
	return roleID, nil
}

// Team operations

func (g *GitHubClientWrapper) GetAllTeamsForOrg(ctx context.Context, org string) ([]*github.Team, error) {
	result, response, err := g.client.Teams.ListTeams(ctx, org, nil)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) GetTeamBySlug(ctx context.Context, org string, slug string) (*github.Team, error) {
	result, response, err := g.client.Teams.GetTeamBySlug(ctx, org, slug)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) EditTeamBySlug(ctx context.Context, org string, slug string, team *github.NewTeam) (*github.Team, error) {
	result, response, err := g.client.Teams.EditTeamBySlug(ctx, org, slug, *team, false)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) CreateTeam(ctx context.Context, org string, team *github.NewTeam) (*github.Team, error) {
	result, response, err := g.client.Teams.CreateTeam(ctx, org, *team)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) DeleteTeamBySlug(ctx context.Context, org string, slug string) error {
	response, err := g.client.Teams.DeleteTeamBySlug(ctx, org, slug)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

// Team members operations

func (g *GitHubClientWrapper) GetAllTeamMembers(ctx context.Context, org string, slug string) ([]*github.User, error) {
	users, response, err := g.client.Teams.ListTeamMembersBySlug(ctx, org, slug, nil)
	defer _closeBody(response)
	return users, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) AddTeamMember(ctx context.Context, org string, slug string, username string) error {
	_, response, err := g.client.Teams.AddTeamMembershipBySlug(ctx, org, slug, username, nil)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) RemoveTeamMember(ctx context.Context, org string, slug string, username string) error {
	response, err := g.client.Teams.RemoveTeamMembershipBySlug(ctx, org, slug, username)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

// Team IDP group operations
func (g *GitHubClientWrapper) GetExternalGroupsForTeamBySlug(ctx context.Context, org string, slug string) ([]*github.ExternalGroup, error) {
	result, response, err := g.client.Teams.ListExternalGroupsForTeamBySlug(ctx, org, slug)
	defer _closeBody(response)
	if err != nil {
		return nil, _handleErrorResponse(response, err)
	}
	return result.Groups, nil
}

func (g *GitHubClientWrapper) GetExternalGroupNamesToIDForOrg(ctx context.Context, org string) (map[string]int64, error) {
	result, response, err := g.client.Teams.ListExternalGroups(ctx, org, nil)
	defer _closeBody(response)
	groupNamesToId := make(map[string]int64)
	if err != nil {
		return groupNamesToId, _handleErrorResponse(response, err)
	}
	for _, group := range result.Groups {
		if group.GroupName != nil && group.GroupID != nil {
			groupNamesToId[*group.GroupName] = *group.GroupID
		}
	}
	return groupNamesToId, nil
}

func (g *GitHubClientWrapper) AddExternalGroupToTeamBySlug(ctx context.Context, org string, slug string, group *github.ExternalGroup) error {
	_, response, err := g.client.Teams.UpdateConnectedExternalGroup(ctx, org, slug, group)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

// GitHub App operations
func (g *GitHubClientWrapper) GetGitHubAppsInstallations(ctx context.Context, org string) ([]*github.Installation, error) {
	// github pagination creates EOF errors when listing enabled repos, so disabling pagination and handling it manually in the wrapper
	ctx = githubpagination.WithOverrideConfig(ctx, githubpagination.WithPaginationDisabled())
	installations := make([]*github.Installation, 0)
	result, response, err := g.client.Organizations.ListInstallations(ctx, org, &github.ListOptions{PerPage: 100})
	_closeBody(response)
	if err != nil {
		return nil, _handleErrorResponse(response, err)
	}
	if result != nil {
		installations = result.Installations
		nextPage := response.NextPage
		for nextPage != 0 && nextPage <= response.LastPage {
			result, response, err = g.client.Organizations.ListInstallations(ctx, org, &github.ListOptions{Page: nextPage, PerPage: 100})
			_closeBody(response)
			if err != nil {
				return nil, _handleErrorResponse(response, err)
			}
			nextPage = response.NextPage
			if result != nil {
				installations = append(installations, result.Installations...)
			}
		}
	}
	return installations, _handleErrorResponse(response, err)
}

// Role operations

func (g *GitHubClientWrapper) GetRoleByName(ctx context.Context, org string, roleName string) (*github.CustomOrgRole, error) {
	// GitHub allows for up to 20 custom roles per org + 8 built-in roles. so fetching all and filtering is acceptable here.
	roles, response, err := g.client.Organizations.ListRoles(ctx, org)
	defer _closeBody(response)
	if err != nil {
		return nil, _handleErrorResponse(response, err)
	}
	for _, role := range roles.CustomRepoRoles {
		if role.GetName() == roleName {
			return role, _handleErrorResponse(response, err)
		}
	}
	return nil, fmt.Errorf("failed to find role named %s in organization %s", roleName, org)
}

// Actions permissions
func (g *GitHubClientWrapper) GetActionsPermissionsForOrg(ctx context.Context, org string) (*github.ActionsPermissions, error) {
	result, response, err := g.client.Actions.GetActionsPermissions(ctx, org)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}
func (g *GitHubClientWrapper) SetActionsPermissionsForOrg(ctx context.Context, org string, permissions github.ActionsPermissions) (*github.ActionsPermissions, error) {
	result, response, err := g.client.Actions.UpdateActionsPermissions(ctx, org, permissions)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) GetActionsRetentionForOrg(ctx context.Context, org string) (*github.ArtifactPeriod, error) {
	result, response, err := g.client.Actions.GetArtifactAndLogRetentionPeriodInOrganization(ctx, org)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}
func (g *GitHubClientWrapper) SetActionsRetentionForOrg(ctx context.Context, org string, retentionInDays int) error {
	response, err := g.client.Actions.UpdateArtifactAndLogRetentionPeriodInOrganization(ctx, org, github.ArtifactPeriodOpt{Days: new(retentionInDays)})
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) GetActionsAllowedForOrg(ctx context.Context, org string) (*github.ActionsAllowed, error) {
	result, response, err := g.client.Actions.GetActionsAllowed(ctx, org)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}
func (g *GitHubClientWrapper) SetActionsAllowedForOrg(ctx context.Context, org string, allowedActions github.ActionsAllowed) (*github.ActionsAllowed, error) {
	result, response, err := g.client.Actions.UpdateActionsAllowed(ctx, org, allowedActions)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) GetActionsDefaultWorkflowPermissionsForOrg(ctx context.Context, org string) (*github.DefaultWorkflowPermissionOrganization, error) {
	result, response, err := g.client.Actions.GetDefaultWorkflowPermissionsInOrganization(ctx, org)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}
func (g *GitHubClientWrapper) SetActionsDefaultWorkflowPermissionsForOrg(ctx context.Context, org string, permissions github.DefaultWorkflowPermissionOrganization) (*github.DefaultWorkflowPermissionOrganization, error) {
	result, response, err := g.client.Actions.UpdateDefaultWorkflowPermissionsInOrganization(ctx, org, permissions)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) GetSelfHostedRunnersSettingsForOrg(ctx context.Context, org string) (*github.SelfHostedRunnersSettingsOrganization, error) {
	result, response, err := g.client.Actions.GetSelfHostedRunnersSettingsInOrganization(ctx, org)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}
func (g *GitHubClientWrapper) SetSelfHostedRunnersSettingsForOrg(ctx context.Context, org string, settings github.SelfHostedRunnersSettingsOrganizationOpt) error {
	response, err := g.client.Actions.UpdateSelfHostedRunnersSettingsInOrganization(ctx, org, settings)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) GetActionsEnabledRepositoriesForOrg(ctx context.Context, org string) ([]*github.Repository, error) {
	// github pagination creates EOF errors when listing enabled repos, so disabling pagination and handling it manually in the wrapper
	ctx = githubpagination.WithOverrideConfig(ctx, githubpagination.WithPaginationDisabled())
	repos := make([]*github.Repository, 0)
	result, response, err := g.client.Actions.ListEnabledReposInOrg(ctx, org, &github.ListOptions{PerPage: 100})
	_closeBody(response)
	if err != nil {
		return nil, _handleErrorResponse(response, err)
	}
	if result != nil {
		repos = result.Repositories
		nextPage := response.NextPage
		for nextPage != 0 && nextPage <= response.LastPage {
			result, response, err = g.client.Actions.ListEnabledReposInOrg(ctx, org, &github.ListOptions{Page: nextPage, PerPage: 100})
			_closeBody(response)
			if err != nil {
				return nil, _handleErrorResponse(response, err)
			}
			nextPage = response.NextPage
			repos = append(repos, result.Repositories...)
		}
	}
	return repos, nil
}

func (g *GitHubClientWrapper) SetActionsEnabledRepositoriesForOrg(ctx context.Context, org string, repoIds []int64) error {
	response, err := g.client.Actions.SetEnabledReposInOrg(ctx, org, repoIds)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) SetActionsEnabledForRepo(ctx context.Context, owner string, repoID int64, enabled bool) error {
	var response *github.Response
	var err error
	if enabled {
		response, err = g.client.Actions.AddEnabledReposInOrg(ctx, owner, repoID)
	} else {
		response, err = g.client.Actions.RemoveEnabledReposInOrg(ctx, owner, repoID)
	}
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) GetAccessLevelForExternalWorkflowsForRepo(ctx context.Context, owner string, repo string) (*github.RepositoryActionsAccessLevel, error) {
	result, response, err := g.client.Repositories.GetActionsAccessLevel(ctx, owner, repo)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}
func (g *GitHubClientWrapper) SetAccessLevelForExternalWorkflowsForRepo(ctx context.Context, owner string, repo string, accessLevel github.RepositoryActionsAccessLevel) error {
	response, err := g.client.Repositories.EditActionsAccessLevel(ctx, owner, repo, accessLevel)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) GetRunnerGroupsForOrg(ctx context.Context, org string) ([]*github.RunnerGroup, error) {
	result, response, err := g.client.Actions.ListOrganizationRunnerGroups(ctx, org, nil)
	defer _closeBody(response)
	rgs := make([]*github.RunnerGroup, 0)
	if result != nil {
		rgs = result.RunnerGroups
	}
	return rgs, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) CreateRunnerGroupForOrg(ctx context.Context, org string, createRequest github.CreateRunnerGroupRequest) (*github.RunnerGroup, error) {
	result, response, err := g.client.Actions.CreateOrganizationRunnerGroup(ctx, org, createRequest)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) UpdateRunnerGroupForOrg(ctx context.Context, org string, groupID int64, updateRequest github.UpdateRunnerGroupRequest) (*github.RunnerGroup, error) {
	result, response, err := g.client.Actions.UpdateOrganizationRunnerGroup(ctx, org, groupID, updateRequest)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) DeleteRunnerGroupForOrg(ctx context.Context, org string, groupID int64) error {
	runnersList, _, err := g.client.Actions.ListRunnerGroupRunners(ctx, org, groupID, nil)
	if err != nil {
		return _handleErrorResponse(nil, err)
	}
	for _, runner := range runnersList.Runners {
		_, err := g.client.Actions.RemoveRunnerGroupRunners(ctx, org, groupID, runner.GetID())
		if err != nil {
			return _handleErrorResponse(nil, err)
		}
	}
	response, err := g.client.Actions.DeleteOrganizationRunnerGroup(ctx, org, groupID)
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}

func (g *GitHubClientWrapper) SetSelectedRepositoriesForRunnerGroup(ctx context.Context, resource string, groupID int64, selectedRepositoryIDs []int64) error {
	response, err := g.client.Actions.SetRepositoryAccessRunnerGroup(ctx, resource, groupID, github.SetRepoAccessRunnerGroupRequest{SelectedRepositoryIDs: selectedRepositoryIDs})
	defer _closeBody(response)
	return _handleErrorResponse(response, err)
}
func (g *GitHubClientWrapper) GetSelectedRepositoriesForRunnerGroup(ctx context.Context, resource string, groupID int64) ([]*github.Repository, error) {
	result, response, err := g.client.Actions.ListRepositoryAccessRunnerGroup(ctx, resource, groupID, nil)
	defer _closeBody(response)
	repos := make([]*github.Repository, 0)
	if result != nil {
		repos = result.Repositories
	}
	return repos, _handleErrorResponse(response, err)
}

// Rate Limit operations

func (g *GitHubClientWrapper) GetRateLimit(ctx context.Context) (*github.RateLimits, error) {
	result, response, err := g.client.RateLimit.Get(ctx)
	defer _closeBody(response)
	return result, _handleErrorResponse(response, err)
}

// helpers/utils

func _closeBody(response *github.Response) {
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
}

func _handleErrorResponse(response *github.Response, err error) error {
	if response == nil {
		return err
	}
	var ghErr error
	httpResponse := _getHTTPResponse(response, err)
	if httpResponse != nil {
		ghErr = github.CheckResponse(httpResponse)
		if err != nil && ghErr != nil {
			err = errors.Join(ghErr, err)
		}
	}
	defer func() {
		if httpResponse != nil && httpResponse.Body != nil {
			_ = httpResponse.Body.Close()
		}
	}()
	return err
}

func _getHTTPResponse(response *github.Response, err error) *http.Response {
	var result *http.Response
	if response != nil {
		result = response.Response
	}
	var httpErr *ghinstallation.HTTPError
	if response == nil && errors.As(err, &httpErr) && httpErr != nil {
		result = httpErr.Response
	}
	return result
}
