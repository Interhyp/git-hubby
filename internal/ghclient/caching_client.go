package ghclient

import (
	"context"
	"sync"
	"time"

	"github.com/google/go-github/v89/github"
)

// contextKey is an unexported type used for context keys in this package.
type contextKey struct{}

// withCacheKey is the context key that enables response caching for the request.
var withCacheKey = contextKey{}

// WithCache returns a context that enables response caching on CachingClient methods.
// Methods called with a cache-enabled context will return cached results if available
// and fresh, avoiding expensive paginated API calls.
//
// Use this when the caller tolerates slightly stale data (e.g., resolving slugs to IDs
// during reconciliation). Omit it when freshness is critical (e.g., team membership
// reconciliation that needs the live state).
func WithCache(ctx context.Context) context.Context {
	return context.WithValue(ctx, withCacheKey, true)
}

// isCacheEnabled checks whether the context has caching enabled.
func isCacheEnabled(ctx context.Context) bool {
	v, _ := ctx.Value(withCacheKey).(bool)
	return v
}

// cachedValue holds a cached result with its expiry time.
type cachedValue[T any] struct {
	data      T
	expiresAt time.Time
}

// responseCache is a generic, thread-safe, single-entry TTL cache for one API response.
type responseCache[T any] struct {
	mu    sync.Mutex
	entry *cachedValue[T]
	ttl   time.Duration
}

// newResponseCache creates a new response cache with the given TTL.
func newResponseCache[T any](ttl time.Duration) *responseCache[T] {
	return &responseCache[T]{ttl: ttl}
}

// GetOrFetch returns cached data if still valid, otherwise calls fetch and caches the result.
// Thread-safe: concurrent callers may trigger parallel fetches on cache miss (both results
// are valid; last write wins). The fetch function is called outside the lock to avoid
// blocking other goroutines during potentially slow paginated API calls.
func (c *responseCache[T]) GetOrFetch(fetch func() (T, error)) (T, error) {
	c.mu.Lock()
	if c.entry != nil && time.Now().Before(c.entry.expiresAt) {
		data := c.entry.data
		c.mu.Unlock()
		return data, nil
	}
	c.mu.Unlock()

	data, err := fetch()
	if err != nil {
		var zero T
		return zero, err
	}

	c.mu.Lock()
	c.entry = &cachedValue[T]{data: data, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
	return data, nil
}

// Invalidate expires the cached entry, forcing the next GetOrFetch to call the API.
func (c *responseCache[T]) Invalidate() {
	c.mu.Lock()
	c.entry = nil
	c.mu.Unlock()
}

// CachingClient wraps a GitHubClient and caches expensive list operations when the
// calling context has caching enabled via WithCache(ctx).
//
// Without WithCache in the context, all methods delegate directly to the inner client
// with no caching — this is the default behavior ensuring backwards compatibility.
//
// The cached results are immutable slices populated by the inner client. They are safe
// to read concurrently from multiple goroutines.
type CachingClient struct {
	GitHubClient // embed — all non-overridden methods delegate directly

	teams *responseCache[[]*github.Team]
	apps  *responseCache[[]*github.Installation]
	roles *responseCache[[]*github.CustomOrgRole]
}

// NewCachingClient wraps the given client with opt-in response caching.
// TTL controls how long cached results are considered fresh.
// A TTL of 0 effectively disables caching (results expire immediately).
func NewCachingClient(inner GitHubClient, ttl time.Duration) *CachingClient {
	return &CachingClient{
		GitHubClient: inner,
		teams:        newResponseCache[[]*github.Team](ttl),
		apps:         newResponseCache[[]*github.Installation](ttl),
		roles:        newResponseCache[[]*github.CustomOrgRole](ttl),
	}
}

// GetAllTeamsForOrg returns all teams for the org. When ctx has caching enabled
// (via WithCache), returns cached results if fresh. Otherwise calls the API directly.
func (c *CachingClient) GetAllTeamsForOrg(ctx context.Context, org string) ([]*github.Team, error) {
	if !isCacheEnabled(ctx) {
		return c.GitHubClient.GetAllTeamsForOrg(ctx, org)
	}
	return c.teams.GetOrFetch(func() ([]*github.Team, error) {
		return c.GitHubClient.GetAllTeamsForOrg(ctx, org)
	})
}

// GetGitHubAppsInstallations returns all app installations for the org. When ctx has
// caching enabled (via WithCache), returns cached results if fresh.
func (c *CachingClient) GetGitHubAppsInstallations(ctx context.Context, org string) ([]*github.Installation, error) {
	if !isCacheEnabled(ctx) {
		return c.GitHubClient.GetGitHubAppsInstallations(ctx, org)
	}
	return c.apps.GetOrFetch(func() ([]*github.Installation, error) {
		return c.GitHubClient.GetGitHubAppsInstallations(ctx, org)
	})
}

// GetAllOrgRoles returns all custom org roles. When ctx has caching enabled
// (via WithCache), returns cached results if fresh.
func (c *CachingClient) GetAllOrgRoles(ctx context.Context, org string) ([]*github.CustomOrgRole, error) {
	if !isCacheEnabled(ctx) {
		return c.GitHubClient.GetAllOrgRoles(ctx, org)
	}
	return c.roles.GetOrFetch(func() ([]*github.CustomOrgRole, error) {
		return c.GitHubClient.GetAllOrgRoles(ctx, org)
	})
}

// InvalidateAll expires all cached entries, forcing the next cached call to hit the API.
// Use this when a lookup fails due to a stale cache (e.g., a newly created team is not
// found in the cached teams list).
func (c *CachingClient) InvalidateAll() {
	c.teams.Invalidate()
	c.apps.Invalidate()
	c.roles.Invalidate()
}

// CacheInvalidator is an optional interface that GitHubClient implementations may satisfy.
// Callers can type-assert to check if cache invalidation is supported.
type CacheInvalidator interface {
	InvalidateAll()
}

// InvalidateCache invalidates the response cache on the given client if it supports it.
// Returns true if the cache was invalidated, false if the client doesn't support caching.
func InvalidateCache(client GitHubClient) bool {
	if inv, ok := client.(CacheInvalidator); ok {
		inv.InvalidateAll()
		return true
	}
	return false
}
