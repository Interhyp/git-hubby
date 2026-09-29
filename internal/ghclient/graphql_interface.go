package ghclient

import (
	"context"
	"time"
)

// IPAllowListEnabledSetting mirrors the GitHub GraphQL IpAllowListEnabledSettingValue /
// IpAllowListForInstalledAppsEnabledSettingValue enum. Reading these settings can also yield
// values that are neither ENABLED nor DISABLED on some owner types; callers should treat any
// value other than IPAllowListEnabled as "not enabled".
type IPAllowListEnabledSetting string

const (
	// IPAllowListEnabled indicates the setting is enabled for the owner.
	IPAllowListEnabled IPAllowListEnabledSetting = "ENABLED"
	// IPAllowListDisabled indicates the setting is disabled for the owner.
	IPAllowListDisabled IPAllowListEnabledSetting = "DISABLED"
)

// IPAllowListEntry is a single entry returned from GitHub's IP allow list for an owner.
//
// Origin distinguishes entries the organization owns (and git-hubby may manage) from read-only
// entries that must never be modified: those inherited from the enterprise account and those
// automatically managed by installed GitHub Apps.
type IPAllowListEntry struct {
	// ID is the GraphQL node ID of the entry, required for update/delete mutations.
	ID string
	// Name is the optional human-readable description of the entry.
	Name string
	// AllowListValue is the IP address or CIDR range.
	AllowListValue string
	// IsActive reports whether the entry is enforced while the allow list is enabled.
	IsActive bool
	// CreatedAt / UpdatedAt are informational timestamps.
	CreatedAt time.Time
	UpdatedAt time.Time
}

// OrgIPAllowListConfig is the current organization-level IP allow list configuration read from GitHub.
type OrgIPAllowListConfig struct {
	// OwnerID is the GraphQL node ID of the organization, required as the owner for create/enable mutations.
	OwnerID string
	// EnabledSetting is the current enforcement setting.
	EnabledSetting IPAllowListEnabledSetting
	// InstalledAppsEnabledSetting is the current "configure for installed GitHub Apps" setting.
	InstalledAppsEnabledSetting IPAllowListEnabledSetting
	// Entries is the full list of entries (organization-owned, enterprise-inherited, and App-managed).
	Entries []IPAllowListEntry
}

// GraphQLClient defines the interface for GitHub GraphQL API operations used by reconcilers.
//
// It exposes the generic githubv4 Query/Mutate surface (kept thin so it is stable and easy to
// mock) plus a small set of higher-level helpers for features that are only available via
// GraphQL, such as organization IP allow list management. The helpers keep GraphQL query/struct
// construction out of the reconcilers.
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
	Mutate(ctx context.Context, m any, input GraphQLInput, variables map[string]any) error

	// GetOrgIPAllowList reads the organization's IP allow list configuration (owner ID, both
	// enabled settings, and all entries). It paginates over all entries.
	GetOrgIPAllowList(ctx context.Context, orgLogin string) (*OrgIPAllowListConfig, error)

	// CreateOrgIPAllowListEntry adds a new organization-owned entry for the given owner (organization) ID.
	CreateOrgIPAllowListEntry(ctx context.Context, ownerID string, value string, name string, isActive bool) error

	// UpdateOrgIPAllowListEntry updates an existing entry identified by its GraphQL node ID.
	UpdateOrgIPAllowListEntry(ctx context.Context, entryID string, value string, name string, isActive bool) error

	// DeleteOrgIPAllowListEntry deletes an entry identified by its GraphQL node ID.
	DeleteOrgIPAllowListEntry(ctx context.Context, entryID string) error

	// SetOrgIPAllowListEnabled sets the IP allow list enforcement setting for the given owner (organization) ID.
	SetOrgIPAllowListEnabled(ctx context.Context, ownerID string, enabled bool) error

	// SetOrgIPAllowListForInstalledAppsEnabled sets the "configure for installed GitHub Apps"
	// setting for the given owner (organization) ID.
	SetOrgIPAllowListForInstalledAppsEnabled(ctx context.Context, ownerID string, enabled bool) error
}
