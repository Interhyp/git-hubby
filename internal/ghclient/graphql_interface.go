package ghclient

import (
	"context"

	"github.com/shurcooL/githubv4"
)

// GraphQLClient defines the interface for GitHub GraphQL API operations used by reconcilers.
//
// It intentionally mirrors the shape of the shurcooL/githubv4 client (Query/Mutate) rather
// than exposing one method per business operation the way the REST GitHubClient does. GraphQL
// queries are defined by the caller as tagged Go structs, so a thin, generic surface keeps the
// interface stable while still allowing it to be mocked in tests.
//
// RATE LIMIT CATEGORY: All calls made through this interface hit the GitHub GraphQL API, which
// GitHub accounts for under the dedicated "graphql" rate limit category (5000 points/hour by
// default), separate from the REST "core" category. The HTTP transport built for this client in
// the factory records those responses in the OrgRateLimitRegistry under CategoryGraphQL, so the
// same per-org stall gating that protects REST calls also protects GraphQL calls. If you wire a
// non-zero RATE_LIMIT_STALL_THRESHOLD_GRAPHQL, GraphQL exhaustion will requeue reconciliations
// just like core exhaustion does.
type GraphQLClient interface {
	// Query executes a GraphQL query. q must be a pointer to a struct that uses githubv4/graphql
	// struct tags. variables may be nil when the query takes no arguments.
	Query(ctx context.Context, q any, variables map[string]any) error

	// Mutate executes a GraphQL mutation. m must be a pointer to a struct describing the mutation
	// payload, input is the mutation's input object, and variables may be nil.
	Mutate(ctx context.Context, m any, input githubv4.Input, variables map[string]any) error
}
