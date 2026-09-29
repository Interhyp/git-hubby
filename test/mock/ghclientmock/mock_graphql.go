//nolint:lll
package ghclientmock

import (
	"context"
	"sync"

	"github.com/Interhyp/git-hubby/internal/ghclient"
)

// GraphQLCall records a single invocation of the mock GraphQL client for assertions in tests.
type GraphQLCall struct {
	// Method is the invoked method name, e.g. "Query", "Mutate",
	// "CreateOrgIPAllowListEntry", "SetOrgIPAllowListEnabled".
	Method string
	// The following fields carry the salient arguments for IP allow list methods; unused fields
	// remain zero-valued for other calls.
	OrgLogin string
	OwnerID  string
	EntryID  string
	Value    string
	Name     string
	IsActive bool
	Enabled  bool
}

// MockGraphQLClient is a hand-written, function-field mock of ghclient.GraphQLClient, following
// the same pattern as MockGitHubClientWrapper: each method records the call and delegates to an
// injected Func if set, otherwise returns a sensible default (nil error / no-op).
type MockGraphQLClient struct {
	QueryFunc  func(ctx context.Context, q any, variables map[string]any) error
	MutateFunc func(ctx context.Context, m any, input ghclient.GraphQLInput, variables map[string]any) error

	GetOrgIPAllowListFunc                        func(ctx context.Context, orgLogin string) (*ghclient.OrgIPAllowListConfig, error)
	CreateOrgIPAllowListEntryFunc                func(ctx context.Context, ownerID string, value string, name string, isActive bool) error
	UpdateOrgIPAllowListEntryFunc                func(ctx context.Context, entryID string, value string, name string, isActive bool) error
	DeleteOrgIPAllowListEntryFunc                func(ctx context.Context, entryID string) error
	SetOrgIPAllowListEnabledFunc                 func(ctx context.Context, ownerID string, enabled bool) error
	SetOrgIPAllowListForInstalledAppsEnabledFunc func(ctx context.Context, ownerID string, enabled bool) error

	mu           sync.Mutex
	GraphQLCalls []GraphQLCall
}

// NewMockGraphQLClient creates a new mock GraphQL client with empty call tracking.
func NewMockGraphQLClient() *MockGraphQLClient {
	return &MockGraphQLClient{
		GraphQLCalls: make([]GraphQLCall, 0),
	}
}

func (m *MockGraphQLClient) recordCall(call GraphQLCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GraphQLCalls = append(m.GraphQLCalls, call)
}

// Query records the call and delegates to QueryFunc if configured.
func (m *MockGraphQLClient) Query(ctx context.Context, q any, variables map[string]any) error {
	m.recordCall(GraphQLCall{Method: "Query"})
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, q, variables)
	}
	return nil
}

// Mutate records the call and delegates to MutateFunc if configured.
func (m *MockGraphQLClient) Mutate(ctx context.Context, mutation any, input ghclient.GraphQLInput, variables map[string]any) error {
	m.recordCall(GraphQLCall{Method: "Mutate"})
	if m.MutateFunc != nil {
		return m.MutateFunc(ctx, mutation, input, variables)
	}
	return nil
}

// GetOrgIPAllowList records the call and delegates to GetOrgIPAllowListFunc if configured.
func (m *MockGraphQLClient) GetOrgIPAllowList(ctx context.Context, orgLogin string) (*ghclient.OrgIPAllowListConfig, error) {
	m.recordCall(GraphQLCall{Method: "GetOrgIPAllowList", OrgLogin: orgLogin})
	if m.GetOrgIPAllowListFunc != nil {
		return m.GetOrgIPAllowListFunc(ctx, orgLogin)
	}
	return &ghclient.OrgIPAllowListConfig{}, nil
}

// CreateOrgIPAllowListEntry records the call and delegates to CreateOrgIPAllowListEntryFunc if configured.
func (m *MockGraphQLClient) CreateOrgIPAllowListEntry(ctx context.Context, ownerID string, value string, name string, isActive bool) error {
	m.recordCall(GraphQLCall{Method: "CreateOrgIPAllowListEntry", OwnerID: ownerID, Value: value, Name: name, IsActive: isActive})
	if m.CreateOrgIPAllowListEntryFunc != nil {
		return m.CreateOrgIPAllowListEntryFunc(ctx, ownerID, value, name, isActive)
	}
	return nil
}

// UpdateOrgIPAllowListEntry records the call and delegates to UpdateOrgIPAllowListEntryFunc if configured.
func (m *MockGraphQLClient) UpdateOrgIPAllowListEntry(ctx context.Context, entryID string, value string, name string, isActive bool) error {
	m.recordCall(GraphQLCall{Method: "UpdateOrgIPAllowListEntry", EntryID: entryID, Value: value, Name: name, IsActive: isActive})
	if m.UpdateOrgIPAllowListEntryFunc != nil {
		return m.UpdateOrgIPAllowListEntryFunc(ctx, entryID, value, name, isActive)
	}
	return nil
}

// DeleteOrgIPAllowListEntry records the call and delegates to DeleteOrgIPAllowListEntryFunc if configured.
func (m *MockGraphQLClient) DeleteOrgIPAllowListEntry(ctx context.Context, entryID string) error {
	m.recordCall(GraphQLCall{Method: "DeleteOrgIPAllowListEntry", EntryID: entryID})
	if m.DeleteOrgIPAllowListEntryFunc != nil {
		return m.DeleteOrgIPAllowListEntryFunc(ctx, entryID)
	}
	return nil
}

// SetOrgIPAllowListEnabled records the call and delegates to SetOrgIPAllowListEnabledFunc if configured.
func (m *MockGraphQLClient) SetOrgIPAllowListEnabled(ctx context.Context, ownerID string, enabled bool) error {
	m.recordCall(GraphQLCall{Method: "SetOrgIPAllowListEnabled", OwnerID: ownerID, Enabled: enabled})
	if m.SetOrgIPAllowListEnabledFunc != nil {
		return m.SetOrgIPAllowListEnabledFunc(ctx, ownerID, enabled)
	}
	return nil
}

// SetOrgIPAllowListForInstalledAppsEnabled records the call and delegates to the configured func if set.
func (m *MockGraphQLClient) SetOrgIPAllowListForInstalledAppsEnabled(ctx context.Context, ownerID string, enabled bool) error {
	m.recordCall(GraphQLCall{Method: "SetOrgIPAllowListForInstalledAppsEnabled", OwnerID: ownerID, Enabled: enabled})
	if m.SetOrgIPAllowListForInstalledAppsEnabledFunc != nil {
		return m.SetOrgIPAllowListForInstalledAppsEnabledFunc(ctx, ownerID, enabled)
	}
	return nil
}
