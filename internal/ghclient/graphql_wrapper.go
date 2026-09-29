package ghclient

import (
	"context"

	"github.com/shurcooL/githubv4"
)

// GraphQLInput is the input type for GraphQL mutations. It aliases githubv4.Input so callers and
// mocks do not need to import githubv4 directly for the generic Mutate method.
type GraphQLInput = githubv4.Input

// GraphQLClientWrapper is the production implementation of GraphQLClient. It wraps a
// *githubv4.Client and delegates directly to it. Error handling is left to the githubv4
// client, which already surfaces GraphQL and HTTP transport errors; the HTTP-level concerns
// (auth, rate limiting, retry, tracing) are handled by the transport stack the factory
// installs on the underlying *http.Client, exactly as for the REST wrapper.
type GraphQLClientWrapper struct {
	client *githubv4.Client
}

// NewGraphQLClientWrapper wraps a githubv4 client with the GraphQLClient interface.
func NewGraphQLClientWrapper(client *githubv4.Client) *GraphQLClientWrapper {
	return &GraphQLClientWrapper{client: client}
}

// Query executes a GraphQL query against the GitHub GraphQL API.
func (g *GraphQLClientWrapper) Query(ctx context.Context, q any, variables map[string]any) error {
	return g.client.Query(ctx, q, variables)
}

// Mutate executes a GraphQL mutation against the GitHub GraphQL API.
func (g *GraphQLClientWrapper) Mutate(ctx context.Context, m any, input GraphQLInput, variables map[string]any) error {
	return g.client.Mutate(ctx, m, input, variables)
}
