//nolint:lll
package ghclientmock

import (
	"context"
	"sync"

	"github.com/shurcooL/githubv4"
)

// GraphQLCall records a single invocation of the mock GraphQL client for assertions in tests.
type GraphQLCall struct {
	Method string // "Query" or "Mutate"
}

// MockGraphQLClient is a hand-written, function-field mock of ghclient.GraphQLClient, following
// the same pattern as MockGitHubClientWrapper: each method records the call and delegates to an
// injected Func if set, otherwise returns a sensible default (nil error / no-op).
type MockGraphQLClient struct {
	QueryFunc  func(ctx context.Context, q any, variables map[string]any) error
	MutateFunc func(ctx context.Context, m any, input githubv4.Input, variables map[string]any) error

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
func (m *MockGraphQLClient) Mutate(ctx context.Context, mutation any, input githubv4.Input, variables map[string]any) error {
	m.recordCall(GraphQLCall{Method: "Mutate"})
	if m.MutateFunc != nil {
		return m.MutateFunc(ctx, mutation, input, variables)
	}
	return nil
}
