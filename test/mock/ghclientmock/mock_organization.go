package ghclientmock

import (
	"context"

	"github.com/Interhyp/git-hubby/internal/mapper"
	"github.com/google/go-github/v89/github"
)

// Organization operations

func (m *MockGitHubClientWrapper) GetOrganization(ctx context.Context, org string) (*github.Organization, error) {
	m.recordOrgCall(OrgCall{Method: "GetOrganization", Org: org})

	if m.GetOrganizationFunc != nil {
		return m.GetOrganizationFunc(ctx, org)
	}

	// Default implementation
	return &github.Organization{
		Login: new(org),
		Name:  new(org),
	}, nil
}

func (m *MockGitHubClientWrapper) EditOrganization(ctx context.Context, org string, organization *github.Organization) (*github.Organization, error) {
	m.recordOrgCall(OrgCall{Method: "EditOrganization", Org: org})

	if m.EditOrganizationFunc != nil {
		return m.EditOrganizationFunc(ctx, org, organization)
	}

	// Default implementation - return the updated organization
	return organization, nil
}

func (m *MockGitHubClientWrapper) CreateOrUpdateOrganizationCustomProperties(ctx context.Context, org string, properties []*github.CustomProperty) ([]*github.CustomProperty, error) {
	m.recordCustomPropCall(CustomPropCall{Method: "CreateOrUpdateOrganizationCustomProperties", Org: org})

	if m.CreateOrUpdateOrganizationCustomPropertiesFunc != nil {
		return m.CreateOrUpdateOrganizationCustomPropertiesFunc(ctx, org, properties)
	}

	// Default implementation - return the properties that were sent
	resultProperties := make([]*github.CustomProperty, len(properties))
	for i, prop := range properties {
		// Create a copy to avoid modifying the original
		resultProp := *prop
		// Set source type to organization if not set
		if resultProp.SourceType == nil {
			resultProp.SourceType = github.Ptr(mapper.CustomPropertySourceTypeOrganization)
		}
		resultProperties[i] = &resultProp
	}

	return resultProperties, nil
}

func (m *MockGitHubClientWrapper) GetAllCustomPropertiesForOrganization(ctx context.Context, org string) ([]*github.CustomProperty, error) {
	m.recordCustomPropCall(CustomPropCall{Method: "GetAllCustomPropertiesForOrganization", Org: org})

	if m.GetAllOrganizationCustomPropertiesFunc != nil {
		return m.GetAllOrganizationCustomPropertiesFunc(ctx, org)
	}

	// Default implementation - return empty list (no custom properties)
	return []*github.CustomProperty{}, nil
}

func (m *MockGitHubClientWrapper) ListMembers(ctx context.Context, org string) ([]*github.User, error) {
	m.recordOrgCall(OrgCall{Method: "ListMembers", Org: org})

	if m.ListMembersFunc != nil {
		return m.ListMembersFunc(ctx, org)
	}

	// Default implementation - return empty list (no members)
	return []*github.User{}, nil

}

// Role operations

func (m *MockGitHubClientWrapper) GetRoleByName(ctx context.Context, org string, roleName string) (*github.CustomOrgRole, error) {
	m.recordRoleCall(RoleCall{Method: "GetRoleByName", Org: org, RoleName: roleName})

	if m.GetRoleByNameFunc != nil {
		return m.GetRoleByNameFunc(ctx, org, roleName)
	}

	// Default implementation returns nil
	return nil, nil
}

// Organization Role Assignments operations

func (m *MockGitHubClientWrapper) GetAllTeamsAssignedToOrgRole(ctx context.Context, org string, role string) ([]string, error) {
	m.recordRoleAssignmentCall(RoleAssignmentCall{Method: "GetAllTeamsAssignedToOrgRole", Org: org})

	if m.GetAllTeamsAssignedToOrgRoleFunc != nil {
		return m.GetAllTeamsAssignedToOrgRoleFunc(ctx, org, role)
	}

	// Default implementation
	return []string{}, nil
}

func (m *MockGitHubClientWrapper) AddOrgRoleAssignmentForTeam(ctx context.Context, org string, slug string, roleID int64) error {
	m.recordRoleAssignmentCall(RoleAssignmentCall{Method: "AddOrgRoleAssignmentForTeam", Org: org})

	if m.AddOrgRoleAssignmentForTeamFunc != nil {
		return m.AddOrgRoleAssignmentForTeamFunc(ctx, org, slug, roleID)
	}

	// Default implementation
	return nil
}
func (m *MockGitHubClientWrapper) RemoveOrgRoleAssignmentForTeam(ctx context.Context, org string, slug string, roleID int64) error {
	m.recordRoleAssignmentCall(RoleAssignmentCall{Method: "RemoveOrgRoleAssignmentForTeam", Org: org})

	if m.RemoveOrgRoleAssignmentForTeamFunc != nil {
		return m.RemoveOrgRoleAssignmentForTeamFunc(ctx, org, slug, roleID)
	}

	// Default implementation
	return nil
}

func (m *MockGitHubClientWrapper) GetAllOrgRoles(ctx context.Context, org string) ([]*github.CustomOrgRole, error) {
	m.recordRoleAssignmentCall(RoleAssignmentCall{Method: "GetAllOrgRoles", Org: org})

	if m.GetAllOrgRolesFunc != nil {
		return m.GetAllOrgRolesFunc(ctx, org)
	}

	// Default implementation
	return nil, nil
}
func (m *MockGitHubClientWrapper) GetGitHubAppsInstallations(ctx context.Context, org string) ([]*github.Installation, error) {
	m.recordAppsCall(AppsCall{Method: "GetGitHubAppsInstallations", Org: org})

	if m.GetGitHubAppsInstallationsFunc != nil {
		return m.GetGitHubAppsInstallationsFunc(ctx, org)
	}

	// Default implementation
	return nil, nil
}

// Rate limit operations

func (m *MockGitHubClientWrapper) GetRateLimit(ctx context.Context) (*github.RateLimits, error) {
	m.recordRateLimitCall(RateLimitCall{Method: "GetRateLimit"})

	if m.GetRateLimitFunc != nil {
		return m.GetRateLimitFunc(ctx)
	}

	// Default implementation
	return &github.RateLimits{}, nil
}
