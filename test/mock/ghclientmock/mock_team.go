package ghclientmock

import (
	"context"

	"github.com/google/go-github/v86/github"
)

// Team operations

func (m *MockGitHubClientWrapper) GetAllTeamsForOrg(ctx context.Context, org string) ([]*github.Team, error) {
	m.recordTeamCall(TeamCall{Method: "GetAllTeamsForOrg", Org: org})

	if m.GetAllTeamsForOrgFunc != nil {
		return m.GetAllTeamsForOrgFunc(ctx, org)
	}

	// Default implementation returns nil
	return nil, nil
}

func (m *MockGitHubClientWrapper) GetTeamBySlug(ctx context.Context, org string, slug string) (*github.Team, error) {
	m.recordTeamCall(TeamCall{Method: "GetTeamBySlug", Org: org, Slug: slug})

	if m.GetTeamBySlugFunc != nil {
		return m.GetTeamBySlugFunc(ctx, org, slug)
	}

	// Default implementation returns nil
	return nil, nil
}

func (m *MockGitHubClientWrapper) EditTeamBySlug(ctx context.Context, org string, slug string, team *github.NewTeam) (*github.Team, error) {
	m.recordTeamCall(TeamCall{Method: "EditTeamBySlug", Org: org, Slug: slug})

	if m.EditTeamBySlugFunc != nil {
		return m.EditTeamBySlugFunc(ctx, org, slug, team)
	}

	// Default implementation returns nil
	return nil, nil
}

func (m *MockGitHubClientWrapper) CreateTeam(ctx context.Context, org string, team *github.NewTeam) (*github.Team, error) {
	m.recordTeamCall(TeamCall{Method: "CreateTeam", Org: org, Slug: team.Name, Description: *team.Description})

	if m.CreateTeamFunc != nil {
		return m.CreateTeamFunc(ctx, org, team)
	}

	// Default implementation returns nil
	return nil, nil
}

func (m *MockGitHubClientWrapper) DeleteTeamBySlug(ctx context.Context, org string, slug string) error {
	m.recordTeamCall(TeamCall{Method: "DeleteTeamBySlug", Org: org, Slug: slug})

	if m.DeleteTeamBySlugFunc != nil {
		return m.DeleteTeamBySlugFunc(ctx, org, slug)
	}

	// Default implementation returns nil
	return nil
}

// Team members operations
func (m *MockGitHubClientWrapper) GetAllTeamMembers(ctx context.Context, org string, slug string) ([]*github.User, error) {
	m.recordTeamCall(TeamCall{Method: "GetAllTeamMembers", Org: org, Slug: slug})

	if m.GetAllTeamMembersFunc != nil {
		return m.GetAllTeamMembersFunc(ctx, org, slug)
	}

	// Default implementation returns nil
	return nil, nil
}

func (m *MockGitHubClientWrapper) AddTeamMember(ctx context.Context, org string, slug string, username string) error {
	m.recordTeamMemberCall(TeamMemberCall{Method: "AddTeamMember", Org: org, Slug: slug, Username: username})

	if m.AddMemberToTeamFunc != nil {
		return m.AddMemberToTeamFunc(ctx, org, slug, username)
	}

	// Default implementation returns nil
	return nil
}

func (m *MockGitHubClientWrapper) RemoveTeamMember(ctx context.Context, org string, slug string, username string) error {
	m.recordTeamMemberCall(TeamMemberCall{Method: "RemoveTeamMember", Org: org, Slug: slug, Username: username})

	if m.RemoveMemberFromTeamFunc != nil {
		return m.RemoveMemberFromTeamFunc(ctx, org, slug, username)
	}

	// Default implementation returns nil
	return nil
}

// Team IDP group operations
func (m *MockGitHubClientWrapper) GetExternalGroupsForTeamBySlug(ctx context.Context, org string, slug string) ([]*github.ExternalGroup, error) {
	m.recordExternalGroupCall(ExternalGroupCall{Method: "GetExternalGroupsForTeamBySlug", Org: org, Slug: slug})

	if m.GetExternalGroupsForTeamBySlugFunc != nil {
		return m.GetExternalGroupsForTeamBySlugFunc(ctx, org, slug)
	}

	// Default implementation returns nil
	return make([]*github.ExternalGroup, 0), nil
}

func (m *MockGitHubClientWrapper) GetExternalGroupNamesToIDForOrg(ctx context.Context, org string) (map[string]int64, error) {
	m.recordExternalGroupCall(ExternalGroupCall{Method: "GetExternalGroupNamesToIDForOrg", Org: org})

	if m.GetExternalGroupNamesToIDForOrgFunc != nil {
		return m.GetExternalGroupNamesToIDForOrgFunc(ctx, org)
	}

	// Default implementation returns nil
	return make(map[string]int64), nil
}

func (m *MockGitHubClientWrapper) AddExternalGroupToTeamBySlug(ctx context.Context, org string, slug string, group *github.ExternalGroup) error {
	m.recordExternalGroupCall(ExternalGroupCall{Method: "AddExternalGroupToTeamBySlug", Org: org, Slug: slug, GroupID: group.GetGroupID()})

	if m.AddExternalGroupToTeamBySlugFunc != nil {
		return m.AddExternalGroupToTeamBySlugFunc(ctx, org, slug, group)
	}

	// Default implementation returns nil
	return nil
}
