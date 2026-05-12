package ratelimit

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// GitHubRateLimiterConfig holds the configuration for the rate limiter
type GitHubRateLimiterConfig struct {
	// RequestsPerHour defines how many requests are allowed per hour
	RequestsPerHour int
	// BurstSize defines the burst capacity (how many requests can be made immediately)
	BurstSize int
	// EnableBlocking enables/disables blocking behavior
	EnableBlocking bool
}

// GitHubRateLimiter implements a global rate limiter for GitHub API calls
type GitHubRateLimiter struct {
	config     GitHubRateLimiterConfig
	limiter    *rate.Limiter
	blockUntil time.Time
	blockMutex sync.RWMutex

	// Metrics
	totalRequests   int64
	blockedRequests int64
	metricsMutex    sync.RWMutex
}

// NewGitHubRateLimiter creates a new global rate limiter
func NewGitHubRateLimiter(config GitHubRateLimiterConfig) *GitHubRateLimiter {
	// Convert requests per hour to requests per second
	requestsPerSecond := rate.Limit(float64(config.RequestsPerHour) / 3600.0)

	return &GitHubRateLimiter{
		config:  config,
		limiter: rate.NewLimiter(requestsPerSecond, config.BurstSize),
	}
}

// Wait blocks until a request can be made according to the rate limit
func (g *GitHubRateLimiter) Wait(ctx context.Context) error {
	g.incrementTotalRequests()

	// Check if we're in a blocking period
	if g.isBlocked() {
		g.incrementBlockedRequests()
		if g.config.EnableBlocking {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(g.getBlockDuration()):
				// Continue after block period
			}
		}
	}

	// Use the token bucket limiter
	return g.limiter.Wait(ctx)
}

// Allow checks if a request can be made immediately without blocking
func (g *GitHubRateLimiter) Allow() bool {
	g.incrementTotalRequests()

	// Check if we're in a blocking period
	if g.isBlocked() {
		g.incrementBlockedRequests()
		return false
	}

	return g.limiter.Allow()
}

// Reserve reserves a token and returns the delay until it's available
func (g *GitHubRateLimiter) Reserve() *rate.Reservation {
	g.incrementTotalRequests()

	// Check if we're in a blocking period
	if g.isBlocked() {
		g.incrementBlockedRequests()
		// Return a reservation that delays for the block duration
		return &rate.Reservation{}
	}

	return g.limiter.Reserve()
}

// BlockFor blocks the rate limiter for a specified duration
// This is useful when you hit GitHub's rate limit and need to wait
func (g *GitHubRateLimiter) BlockFor(duration time.Duration) {
	g.blockMutex.Lock()
	defer g.blockMutex.Unlock()

	g.blockUntil = time.Now().Add(duration)
}

// BlockUntil blocks the rate limiter until a specific time
func (g *GitHubRateLimiter) BlockUntil(until time.Time) {
	g.blockMutex.Lock()
	defer g.blockMutex.Unlock()

	g.blockUntil = until
}

// ClearBlock removes any active blocking
func (g *GitHubRateLimiter) ClearBlock() {
	g.blockMutex.Lock()
	defer g.blockMutex.Unlock()

	g.blockUntil = time.Time{}
}

// isBlocked checks if we're currently in a blocking period
func (g *GitHubRateLimiter) isBlocked() bool {
	g.blockMutex.RLock()
	defer g.blockMutex.RUnlock()

	return time.Now().Before(g.blockUntil)
}

// getBlockDuration returns how long we need to wait for the block to end
func (g *GitHubRateLimiter) getBlockDuration() time.Duration {
	g.blockMutex.RLock()
	defer g.blockMutex.RUnlock()

	if time.Now().Before(g.blockUntil) {
		return time.Until(g.blockUntil)
	}
	return 0
}

// GetStats returns current statistics
func (g *GitHubRateLimiter) GetStats() (total, blocked int64) {
	g.metricsMutex.RLock()
	defer g.metricsMutex.RUnlock()

	return g.totalRequests, g.blockedRequests
}

// incrementTotalRequests increments the total request counter
func (g *GitHubRateLimiter) incrementTotalRequests() {
	g.metricsMutex.Lock()
	defer g.metricsMutex.Unlock()

	g.totalRequests++
}

// incrementBlockedRequests increments the blocked request counter
func (g *GitHubRateLimiter) incrementBlockedRequests() {
	g.metricsMutex.Lock()
	defer g.metricsMutex.Unlock()

	g.blockedRequests++
}
