package ratelimit

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/Interhyp/git-hubby/internal/config"
	"github.com/google/go-github/v89/github"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// DefaultCoreStallThreshold is the minimum core API calls remaining before stalling.
	DefaultCoreStallThreshold = 100
	// DefaultResetGracePeriod is added to the GitHub reset time before resuming reconciliation.
	DefaultResetGracePeriod = 10 * time.Second
	// DefaultStalenessThreshold is how old registry data can be before it is considered stale.
	DefaultStalenessThreshold = 5 * time.Minute
)

// CategoryState holds the rate limit state for one category of one organization.
type CategoryState struct {
	Remaining   int
	Limit       int
	ResetTime   time.Time
	LastUpdated time.Time
}

// OrgState holds the rate limit state for a single organization across all monitored categories.
type OrgState struct {
	OrgLogin   string
	AppID      int64
	Categories map[Category]*CategoryState
}

// OrgRegistryConfig configures the OrgRateLimitRegistry.
type OrgRegistryConfig struct {
	// CategoryThresholds maps each category to its minimum remaining budget before reconciliation
	// is stalled. Categories not present in this map are tracked but never cause stalling.
	CategoryThresholds map[Category]int
	// ResetGracePeriod is added to the GitHub-reported reset time before allowing reconciliation
	// to resume, providing a safety buffer.
	ResetGracePeriod time.Duration
	// StalenessThreshold is the maximum age of registry data. If the most recent update for an
	// org is older than this threshold, IsStale returns true and the caller should refresh via
	// GET /rate_limit.
	StalenessThreshold time.Duration
}

// OrgRateLimitRegistry tracks per-organization, per-category GitHub API rate limit state.
// It is safe for concurrent use by multiple goroutines.
type OrgRateLimitRegistry struct {
	mu     sync.RWMutex
	states map[string]*OrgState // keyed by org login
	config OrgRegistryConfig
	// warnedUncovered tracks org+category pairs for which a "no threshold" warning has already
	// been logged, so we emit at most one warning per pair per process lifetime.
	warnedUncovered map[string]struct{}
}

// NewOrgRateLimitRegistry creates a new registry with the given configuration.
func NewOrgRateLimitRegistry(registryConfig OrgRegistryConfig) *OrgRateLimitRegistry {
	return &OrgRateLimitRegistry{
		states:          make(map[string]*OrgState),
		config:          registryConfig,
		warnedUncovered: make(map[string]struct{}),
	}
}

// DefaultOrgRegistryConfig returns an OrgRegistryConfig with sensible defaults.
func DefaultOrgRegistryConfig() OrgRegistryConfig {
	return OrgRegistryConfig{
		CategoryThresholds: map[Category]int{
			CategoryCore: DefaultCoreStallThreshold,
		},
		ResetGracePeriod:   DefaultResetGracePeriod,
		StalenessThreshold: DefaultStalenessThreshold,
	}
}

// ConfiguredThresholds returns the category→threshold mapping derived from operator
// configuration. Every entry in MonitoredCategories must appear as a key in this map
// (with value 0 meaning "track only, never stall"). This invariant is enforced by a
// test so that adding a new MonitoredCategory without wiring its config field fails CI.
//
// When adding a new category that requires stalling, add a corresponding
// RATE_LIMIT_STALL_THRESHOLD_<CATEGORY> env var to config.RateLimitConfig and wire it here.
func ConfiguredThresholds(cfg config.Config) map[Category]int {
	return map[Category]int{
		CategoryCore: cfg.RateLimitConfig.StallThresholdCore,
		// Non-core categories are monitored (tracked in the registry and warned about at
		// runtime) but have no env-configurable threshold yet. When the operator starts
		// using search/graphql endpoints, add their threshold fields to config.RateLimitConfig
		// and wire them here.
		CategorySearch:     0,
		CategoryCodeSearch: 0,
		CategoryGraphQL:    0,
	}
}

// NewOrgRateLimitRegistryFromConfig creates a registry from operator configuration.
// Only categories with a non-zero threshold are added to the stall-check map;
// categories with threshold 0 are still tracked but never cause stalling.
func NewOrgRateLimitRegistryFromConfig(cfg config.Config) *OrgRateLimitRegistry {
	thresholds := map[Category]int{}
	for cat, val := range ConfiguredThresholds(cfg) {
		if val > 0 {
			thresholds[cat] = val
		}
	}

	registryConfig := OrgRegistryConfig{
		CategoryThresholds: thresholds,
		ResetGracePeriod:   time.Duration(cfg.RateLimitConfig.ResetGracePeriod) * time.Second,
		StalenessThreshold: time.Duration(cfg.RateLimitConfig.StalenessThresholdMinutes) * time.Minute,
	}
	return NewOrgRateLimitRegistry(registryConfig)
}

// Update records the rate limit state for a single org/category pair.
// Only monitored categories are stored; unrecognized categories are silently ignored.
// If the category is monitored but has no stall threshold configured, a warning is logged
// once per org+category to alert operators that a new API category is in use without protection.
func (r *OrgRateLimitRegistry) Update(orgLogin string, category Category, remaining, limit int, resetTime time.Time, appID int64) {
	if !isMonitored(category) {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.warnIfUncovered(orgLogin, category)

	state := r.getOrCreateOrgState(orgLogin, appID)
	state.Categories[category] = &CategoryState{
		Remaining:   remaining,
		Limit:       limit,
		ResetTime:   resetTime,
		LastUpdated: time.Now(),
	}
}

// UpdateFromRateLimitResponse bulk-updates all monitored categories from a GET /rate_limit response.
// Since GET /rate_limit is not counted against any quota, callers can use it freely for cold-start
// population and staleness recovery.
func (r *OrgRateLimitRegistry) UpdateFromRateLimitResponse(orgLogin string, rateLimits *github.RateLimits, appID int64) {
	if rateLimits == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	state := r.getOrCreateOrgState(orgLogin, appID)
	now := time.Now()

	if rateLimits.Core != nil {
		state.Categories[CategoryCore] = &CategoryState{
			Remaining:   rateLimits.Core.Remaining,
			Limit:       rateLimits.Core.Limit,
			ResetTime:   rateLimits.Core.Reset.Time,
			LastUpdated: now,
		}
	}
	if rateLimits.GraphQL != nil {
		state.Categories[CategoryGraphQL] = &CategoryState{
			Remaining:   rateLimits.GraphQL.Remaining,
			Limit:       rateLimits.GraphQL.Limit,
			ResetTime:   rateLimits.GraphQL.Reset.Time,
			LastUpdated: now,
		}
	}
	if rateLimits.Search != nil {
		state.Categories[CategorySearch] = &CategoryState{
			Remaining:   rateLimits.Search.Remaining,
			Limit:       rateLimits.Search.Limit,
			ResetTime:   rateLimits.Search.Reset.Time,
			LastUpdated: now,
		}
	}
	if rateLimits.CodeSearch != nil {
		state.Categories[CategoryCodeSearch] = &CategoryState{
			Remaining:   rateLimits.CodeSearch.Remaining,
			Limit:       rateLimits.CodeSearch.Limit,
			ResetTime:   rateLimits.CodeSearch.Reset.Time,
			LastUpdated: now,
		}
	}
}

// ShouldStall reports whether reconciliation for the given org should be paused because
// at least one of the requested categories is below its configured stall threshold.
//
// Returns (true, delay) if stalling is needed, where delay is the maximum wait duration
// across all stalled categories. Returns (false, 0) otherwise.
//
// If categories is empty, all monitored categories with configured thresholds are evaluated.
func (r *OrgRateLimitRegistry) ShouldStall(orgLogin string, categories ...Category) (bool, time.Duration) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, exists := r.states[orgLogin]
	if !exists {
		return false, 0
	}

	if len(categories) == 0 {
		categories = MonitoredCategories
	}

	var maxDelay time.Duration
	stalled := false

	for _, cat := range categories {
		threshold, hasThreshold := r.config.CategoryThresholds[cat]
		if !hasThreshold {
			continue
		}
		catState, ok := state.Categories[cat]
		if !ok {
			continue
		}
		if catState.Remaining < threshold {
			stalled = true
			delay := max(time.Until(catState.ResetTime.Add(r.config.ResetGracePeriod)), 0)
			if delay > maxDelay {
				maxDelay = delay
			}
		}
	}

	return stalled, maxDelay
}

// IsStale returns true if the registry has no data for the org or if the most recent
// update for any category is older than StalenessThreshold.
// A stale result should trigger a GET /rate_limit refresh.
func (r *OrgRateLimitRegistry) IsStale(orgLogin string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, exists := r.states[orgLogin]
	if !exists {
		return true
	}

	var mostRecent time.Time
	for _, catState := range state.Categories {
		if catState.LastUpdated.After(mostRecent) {
			mostRecent = catState.LastUpdated
		}
	}
	if mostRecent.IsZero() {
		return true
	}
	return time.Since(mostRecent) > r.config.StalenessThreshold
}

// GetState returns a snapshot of the rate limit state for an org, or nil if none exists.
func (r *OrgRateLimitRegistry) GetState(orgLogin string) *OrgState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.states[orgLogin]
}

// GetCategoryState returns the rate limit state for a specific org and category, or nil if none exists.
func (r *OrgRateLimitRegistry) GetCategoryState(orgLogin string, category Category) *CategoryState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, exists := r.states[orgLogin]
	if !exists {
		return nil
	}
	return state.Categories[category]
}

// getOrCreateOrgState returns the OrgState for the given org, creating it if necessary.
// Caller must hold the write lock.
func (r *OrgRateLimitRegistry) getOrCreateOrgState(orgLogin string, appID int64) *OrgState {
	if state, exists := r.states[orgLogin]; exists {
		return state
	}
	state := &OrgState{
		OrgLogin:   orgLogin,
		AppID:      appID,
		Categories: make(map[Category]*CategoryState),
	}
	r.states[orgLogin] = state
	return state
}

// isMonitored returns whether the given category is in MonitoredCategories.
func isMonitored(cat Category) bool {
	return slices.Contains(MonitoredCategories, cat)
}

// warnIfUncovered logs a warning (once per org+category) when the registry observes API
// traffic for a monitored category that has no stall threshold configured. This acts as a
// safety net: if someone adds a /search/ or /graphql call without configuring a threshold,
// the operator logs will flag it immediately.
// Caller must hold the write lock.
func (r *OrgRateLimitRegistry) warnIfUncovered(orgLogin string, category Category) {
	if _, hasThreshold := r.config.CategoryThresholds[category]; hasThreshold {
		return
	}
	key := fmt.Sprintf("%s/%s", orgLogin, category)
	if _, warned := r.warnedUncovered[key]; warned {
		return
	}
	r.warnedUncovered[key] = struct{}{}
	logf.Log.Info("Rate limit category observed without a configured stall threshold — "+
		"consider adding a threshold to prevent rate limit exhaustion",
		"org", orgLogin, "category", category)
}
