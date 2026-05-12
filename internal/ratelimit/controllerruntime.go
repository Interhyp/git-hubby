package ratelimit

import (
	"time"

	"k8s.io/client-go/util/workqueue"
)

// ControllerRuntimeRateLimiter adapts the GitHub rate limiter for use with controller-runtime
type ControllerRuntimeRateLimiter[T comparable] struct {
	globalLimiter *GitHubRateLimiter
}

// NewControllerRuntimeRateLimiter creates a controller-runtime compatible rate limiter
func NewControllerRuntimeRateLimiter[T comparable](globalLimiter *GitHubRateLimiter) workqueue.TypedRateLimiter[T] {
	return &ControllerRuntimeRateLimiter[T]{
		globalLimiter: globalLimiter,
	}
}

// When implements workqueue.RateLimiter interface
func (c *ControllerRuntimeRateLimiter[T]) When(item T) time.Duration {
	reservation := c.globalLimiter.Reserve()
	if !reservation.OK() {
		// Fallback if reservation fails
		// This happens when there can not be any tokens reserved during the maximum wait time
		// It should not happen in normal operation, but we handle it gracefully
		return time.Hour
	}

	delay := reservation.Delay()

	// Add block duration if we're blocked
	if c.globalLimiter.isBlocked() {
		blockDelay := c.globalLimiter.getBlockDuration()
		if blockDelay > delay {
			delay = blockDelay
		}
	}

	return delay
}

// Forget implements workqueue.RateLimiter interface
func (c *ControllerRuntimeRateLimiter[T]) Forget(item T) {
	// Nothing to forget for token bucket
}

// NumRequeues implements workqueue.RateLimiter interface
func (c *ControllerRuntimeRateLimiter[T]) NumRequeues(item T) int {
	return 0
}
