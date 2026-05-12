//nolint:lll
package ghclientmock

import (
	"context"

	"github.com/google/go-github/v86/github"
)

// Actions operations

func (m *MockGitHubClientWrapper) GetActionsPermissionsForOrg(ctx context.Context, org string) (*github.ActionsPermissions, error) {
	m.recordActionsCall(ActionsCall{Method: "GetActionsPermissionsForOrg", Org: org})

	if m.GetActionsPermissionsForOrgFunc != nil {
		return m.GetActionsPermissionsForOrgFunc(ctx, org)
	}

	return &github.ActionsPermissions{}, nil
}

func (m *MockGitHubClientWrapper) SetActionsPermissionsForOrg(ctx context.Context, org string, permissions github.ActionsPermissions) (*github.ActionsPermissions, error) {
	m.recordActionsCall(ActionsCall{Method: "SetActionsPermissionsForOrg", Org: org})

	if m.SetActionsPermissionsForOrgFunc != nil {
		return m.SetActionsPermissionsForOrgFunc(ctx, org, permissions)
	}

	return &permissions, nil
}

func (m *MockGitHubClientWrapper) GetActionsRetentionForOrg(ctx context.Context, org string) (*github.ArtifactPeriod, error) {
	m.recordActionsCall(ActionsCall{Method: "GetActionsRetentionForOrg", Org: org})

	if m.GetActionsRetentionForOrgFunc != nil {
		return m.GetActionsRetentionForOrgFunc(ctx, org)
	}

	return &github.ArtifactPeriod{}, nil
}

func (m *MockGitHubClientWrapper) SetActionsRetentionForOrg(ctx context.Context, org string, retentionInDays int) error {
	m.recordActionsCall(ActionsCall{Method: "SetActionsRetentionForOrg", Org: org})

	if m.SetActionsRetentionForOrgFunc != nil {
		return m.SetActionsRetentionForOrgFunc(ctx, org, retentionInDays)
	}

	return nil
}

func (m *MockGitHubClientWrapper) GetActionsAllowedForOrg(ctx context.Context, org string) (*github.ActionsAllowed, error) {
	m.recordActionsCall(ActionsCall{Method: "GetActionsAllowedForOrg", Org: org})

	if m.GetActionsAllowedForOrgFunc != nil {
		return m.GetActionsAllowedForOrgFunc(ctx, org)
	}

	return &github.ActionsAllowed{}, nil
}

func (m *MockGitHubClientWrapper) SetActionsAllowedForOrg(ctx context.Context, org string, allowedActions github.ActionsAllowed) (*github.ActionsAllowed, error) {
	m.recordActionsCall(ActionsCall{Method: "SetActionsAllowedForOrg", Org: org})

	if m.SetActionsAllowedForOrgFunc != nil {
		return m.SetActionsAllowedForOrgFunc(ctx, org, allowedActions)
	}

	return &allowedActions, nil
}

func (m *MockGitHubClientWrapper) GetActionsDefaultWorkflowPermissionsForOrg(ctx context.Context, org string) (*github.DefaultWorkflowPermissionOrganization, error) {
	m.recordActionsCall(ActionsCall{Method: "GetActionsDefaultWorkflowPermissionsForOrg", Org: org})

	if m.GetActionsDefaultWorkflowPermissionsForOrgFunc != nil {
		return m.GetActionsDefaultWorkflowPermissionsForOrgFunc(ctx, org)
	}

	return &github.DefaultWorkflowPermissionOrganization{}, nil
}

func (m *MockGitHubClientWrapper) SetActionsDefaultWorkflowPermissionsForOrg(ctx context.Context, org string, permissions github.DefaultWorkflowPermissionOrganization) (*github.DefaultWorkflowPermissionOrganization, error) {
	m.recordActionsCall(ActionsCall{Method: "SetActionsDefaultWorkflowPermissionsForOrg", Org: org})

	if m.SetActionsDefaultWorkflowPermissionsForOrgFunc != nil {
		return m.SetActionsDefaultWorkflowPermissionsForOrgFunc(ctx, org, permissions)
	}

	return &permissions, nil
}

func (m *MockGitHubClientWrapper) GetSelfHostedRunnersSettingsForOrg(ctx context.Context, org string) (*github.SelfHostedRunnersSettingsOrganization, error) {
	m.recordActionsCall(ActionsCall{Method: "GetSelfHostedRunnersSettingsForOrg", Org: org})

	if m.GetSelfHostedRunnersSettingsForOrgFunc != nil {
		return m.GetSelfHostedRunnersSettingsForOrgFunc(ctx, org)
	}

	return &github.SelfHostedRunnersSettingsOrganization{}, nil
}

func (m *MockGitHubClientWrapper) SetSelfHostedRunnersSettingsForOrg(ctx context.Context, org string, settings github.SelfHostedRunnersSettingsOrganizationOpt) error {
	m.recordActionsCall(ActionsCall{Method: "SetSelfHostedRunnersSettingsForOrg", Org: org})

	if m.SetSelfHostedRunnersSettingsForOrgFunc != nil {
		return m.SetSelfHostedRunnersSettingsForOrgFunc(ctx, org, settings)
	}

	return nil
}

func (m *MockGitHubClientWrapper) GetActionsEnabledRepositoriesForOrg(ctx context.Context, org string) ([]*github.Repository, error) {
	m.recordActionsCall(ActionsCall{Method: "GetActionsEnabledRepositoriesForOrg", Org: org})

	if m.GetActionsEnabledRepositoriesForOrgFunc != nil {
		return m.GetActionsEnabledRepositoriesForOrgFunc(ctx, org)
	}

	return nil, nil
}
func (m *MockGitHubClientWrapper) SetActionsEnabledRepositoriesForOrg(ctx context.Context, org string, repoIDs []int64) error {
	m.recordActionsCall(ActionsCall{Method: "SetActionsEnabledRepositoriesForOrg", Org: org})

	if m.SetActionsEnabledRepositoriesForOrgFunc != nil {
		return m.SetActionsEnabledRepositoriesForOrgFunc(ctx, org, repoIDs)
	}

	return nil
}

func (m *MockGitHubClientWrapper) SetActionsEnabledForRepo(ctx context.Context, owner string, repoID int64, enabled bool) error {
	m.recordActionsCall(ActionsCall{Method: "SetActionsEnabledForRepo", Owner: owner, RepoID: repoID})

	if m.SetActionsEnabledForRepoFunc != nil {
		return m.SetActionsEnabledForRepoFunc(ctx, owner, repoID, enabled)
	}

	return nil
}

func (m *MockGitHubClientWrapper) GetAccessLevelForExternalWorkflowsForRepo(ctx context.Context, owner string, repo string) (*github.RepositoryActionsAccessLevel, error) {
	m.recordActionsCall(ActionsCall{Method: "GetAccessLevelForExternalWorkflowsForRepo", Owner: owner, Repo: repo})

	if m.GetAccessLevelForExternalWorkflowsForRepoFunc != nil {
		return m.GetAccessLevelForExternalWorkflowsForRepoFunc(ctx, owner, repo)
	}

	return &github.RepositoryActionsAccessLevel{}, nil
}

func (m *MockGitHubClientWrapper) SetAccessLevelForExternalWorkflowsForRepo(ctx context.Context, owner string, repo string, accessLevel github.RepositoryActionsAccessLevel) error {
	m.recordActionsCall(ActionsCall{Method: "SetAccessLevelForExternalWorkflowsForRepo", Owner: owner, Repo: repo})

	if m.SetAccessLevelForExternalWorkflowsForRepoFunc != nil {
		return m.SetAccessLevelForExternalWorkflowsForRepoFunc(ctx, owner, repo, accessLevel)
	}

	return nil
}

func (m *MockGitHubClientWrapper) GetRunnerGroupsForOrg(ctx context.Context, org string) ([]*github.RunnerGroup, error) {
	m.recordActionsCall(ActionsCall{Method: "GetRunnerGroupsForOrg", Org: org})

	if m.GetRunnerGroupsForOrgFunc != nil {
		return m.GetRunnerGroupsForOrgFunc(ctx, org)
	}

	return []*github.RunnerGroup{}, nil
}

func (m *MockGitHubClientWrapper) CreateRunnerGroupForOrg(ctx context.Context, org string, createRequest github.CreateRunnerGroupRequest) (*github.RunnerGroup, error) {
	m.recordActionsCall(ActionsCall{Method: "CreateRunnerGroupForOrg", Org: org})

	if m.CreateRunnerGroupForOrgFunc != nil {
		return m.CreateRunnerGroupForOrgFunc(ctx, org, createRequest)
	}

	return &github.RunnerGroup{
		ID:   github.Ptr(int64(1)),
		Name: createRequest.Name,
	}, nil
}

func (m *MockGitHubClientWrapper) UpdateRunnerGroupForOrg(ctx context.Context, org string, groupID int64, updateRequest github.UpdateRunnerGroupRequest) (*github.RunnerGroup, error) {
	m.recordActionsCall(ActionsCall{Method: "UpdateRunnerGroupForOrg", Org: org})

	if m.UpdateRunnerGroupForOrgFunc != nil {
		return m.UpdateRunnerGroupForOrgFunc(ctx, org, groupID, updateRequest)
	}

	return &github.RunnerGroup{
		ID:                    github.Ptr(groupID),
		Name:                  updateRequest.Name,
		Visibility:            updateRequest.Visibility,
		RestrictedToWorkflows: updateRequest.RestrictedToWorkflows,
		SelectedWorkflows:     updateRequest.SelectedWorkflows,
	}, nil
}

func (m *MockGitHubClientWrapper) DeleteRunnerGroupForOrg(ctx context.Context, org string, groupID int64) error {
	m.recordActionsCall(ActionsCall{Method: "DeleteRunnerGroupForOrg", Org: org})

	if m.DeleteRunnerGroupForOrgFunc != nil {
		return m.DeleteRunnerGroupForOrgFunc(ctx, org, groupID)
	}

	return nil
}

func (m *MockGitHubClientWrapper) GetSelectedRepositoriesForRunnerGroup(ctx context.Context, org string, groupID int64) ([]*github.Repository, error) {
	m.recordActionsCall(ActionsCall{Method: "GetSelectedRepositoriesForRunnerGroup", Org: org})

	if m.GetSelectedRepositoriesForRunnerGroupFunc != nil {
		return m.GetSelectedRepositoriesForRunnerGroupFunc(ctx, org, groupID)
	}

	return []*github.Repository{}, nil
}

func (m *MockGitHubClientWrapper) SetSelectedRepositoriesForRunnerGroup(ctx context.Context, org string, groupID int64, selectedRepositoryIDs []int64) error {
	m.recordActionsCall(ActionsCall{Method: "SetSelectedRepositoriesForRunnerGroup", Org: org})

	if m.SetSelectedRepositoriesForRunnerGroupFunc != nil {
		return m.SetSelectedRepositoriesForRunnerGroupFunc(ctx, org, groupID, selectedRepositoryIDs)
	}

	return nil
}
