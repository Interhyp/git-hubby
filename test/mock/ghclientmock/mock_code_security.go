//nolint:lll
package ghclientmock

import (
	"context"

	"github.com/google/go-github/v89/github"
)

// Code Security Configuration operations

func (m *MockGitHubClientWrapper) GetCodeSecurityConfigurationsForOrg(ctx context.Context, org string) ([]*github.CodeSecurityConfiguration, error) {
	m.recordCodeSecurityConfigurationCall(CodeSecurityConfigurationCall{Method: "GetCodeSecurityConfigurationsForOrg", Org: org})

	if m.GetCodeSecurityConfigurationsForOrgFunc != nil {
		return m.GetCodeSecurityConfigurationsForOrgFunc(ctx, org)
	}

	// Default implementation
	return []*github.CodeSecurityConfiguration{}, nil
}

func (m *MockGitHubClientWrapper) UpdateCodeSecurityConfigurationForOrg(ctx context.Context, org string, configId int64, config github.CodeSecurityConfiguration) (*github.CodeSecurityConfiguration, error) {
	m.recordCodeSecurityConfigurationCall(CodeSecurityConfigurationCall{Method: "UpdateCodeSecurityConfigurationForOrg", Org: org, ConfigID: configId})

	if m.UpdateCodeSecurityConfigurationForOrgFunc != nil {
		return m.UpdateCodeSecurityConfigurationForOrgFunc(ctx, org, configId, config)
	}

	// Default implementation
	config.ID = &configId
	return &config, nil
}

func (m *MockGitHubClientWrapper) CreateCodeSecurityConfigurationForOrg(ctx context.Context, org string, config github.CodeSecurityConfiguration) (*github.CodeSecurityConfiguration, error) {
	m.recordCodeSecurityConfigurationCall(CodeSecurityConfigurationCall{Method: "CreateCodeSecurityConfigurationForOrg", Org: org})

	if m.CreateCodeSecurityConfigurationForOrgFunc != nil {
		return m.CreateCodeSecurityConfigurationForOrgFunc(ctx, org, config)
	}

	// Default implementation
	config.ID = new(int64(1))
	return &config, nil
}

func (m *MockGitHubClientWrapper) DeleteCodeSecurityConfigurationForOrg(ctx context.Context, org string, configId int64) error {
	m.recordCodeSecurityConfigurationCall(CodeSecurityConfigurationCall{Method: "DeleteCodeSecurityConfigurationForOrg", Org: org, ConfigID: configId})

	if m.DeleteCodeSecurityConfigurationForOrgFunc != nil {
		return m.DeleteCodeSecurityConfigurationForOrgFunc(ctx, org, configId)
	}

	// Default implementation
	return nil
}

func (m *MockGitHubClientWrapper) SetCodeSecurityConfigurationAsDefaultForOrg(ctx context.Context, org string, configId int64, newReposParam string) error {
	m.recordCodeSecurityConfigurationCall(CodeSecurityConfigurationCall{Method: "SetCodeSecurityConfigurationAsDefaultForOrg", Org: org, ConfigID: configId})

	if m.SetCodeSecurityConfigurationAsDefaultForOrgFunc != nil {
		return m.SetCodeSecurityConfigurationAsDefaultForOrgFunc(ctx, org, configId, newReposParam)
	}

	// Default implementation
	return nil
}

func (m *MockGitHubClientWrapper) GetDefaultCodeSecurityConfigurationsForOrg(ctx context.Context, org string) ([]*github.CodeSecurityConfigurationWithDefaultForNewRepos, error) {
	m.recordCodeSecurityConfigurationCall(CodeSecurityConfigurationCall{Method: "GetDefaultCodeSecurityConfigurationsForOrg", Org: org})

	if m.GetDefaultCodeSecurityConfigurationsForOrgFunc != nil {
		return m.GetDefaultCodeSecurityConfigurationsForOrgFunc(ctx, org)
	}

	// Default implementation
	return []*github.CodeSecurityConfigurationWithDefaultForNewRepos{}, nil
}

func (m *MockGitHubClientWrapper) AttachCodeSecurityConfigurations(ctx context.Context, org string, cscID int64, scope string, repoIDs []int64) error {
	m.recordCodeSecurityConfigurationCall(CodeSecurityConfigurationCall{Method: "AttachCodeSecurityConfigurations", Org: org, ConfigID: cscID})

	if m.AttachCodeSecurityConfigurationsFunc != nil {
		return m.AttachCodeSecurityConfigurationsFunc(ctx, org, cscID, scope, repoIDs)
	}

	// Default implementation
	return nil
}

func (m *MockGitHubClientWrapper) GetRepositoriesAttachedToCodeSecurityConfiguration(ctx context.Context, org string, cscID int64) ([]*github.RepositoryAttachment, error) {
	m.recordCodeSecurityConfigurationCall(CodeSecurityConfigurationCall{Method: "GetRepositoriesAttachedToCodeSecurityConfiguration", Org: org, ConfigID: cscID})

	if m.GetRepositoriesAttachedToCodeSecurityConfigurationFunc != nil {
		return m.GetRepositoriesAttachedToCodeSecurityConfigurationFunc(ctx, org, cscID)
	}

	// Default implementation
	return []*github.RepositoryAttachment{}, nil
}
