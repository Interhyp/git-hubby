package spreading

import (
	"context"
	"hash/fnv"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// DefaultSpreadPeriodMinutes is the default spread-period window in minutes.
	// Matches the envDefault in internal/config.Config.SpreadPeriodMinutes.
	DefaultSpreadPeriodMinutes = 5
	// DefaultSpreadIntervalMinutes is the default spread-interval window in minutes.
	// Matches the envDefault in internal/config.Config.SpreadIntervalMinutes.
	DefaultSpreadIntervalMinutes = 180
)

// SpreadableResource defines the interface for resources that support startup spreading
type SpreadableResource interface {
	metav1.Object
	GetGeneration() int64
	GetObservedGeneration() int64
	GetObservedSubResourceGenerations() map[string]int64
	IsHealthy() bool
}

// Config holds the configuration for startup spreading
type Config struct {
	// StartTime is when this pod started
	StartTime time.Time
	// SpreadSeed is a random seed used for hashing resources to spread them differently on each pod restart
	SpreadSeed string
	// SpreadPeriod is the duration after startup during which spreadable incoming reconciliation requests are requeued
	// to spread their execution over time.
	SpreadPeriod time.Duration
	// SpreadInterval is the length of the interval across which the requeued reconciliations are spread.
	// Reconciliations are distributed from (StartTime + SpreadPeriod) to (StartTime + SpreadPeriod + SpreadInterval)
	// This value should be a lower bound on the requeue duration on a successful reconciliation to
	// avoid amassing reconciliations at the end of the spreading window.
	SpreadInterval time.Duration
	// Enabled controls whether spreading is active
	Enabled bool
}

// Manager handles startup spreading decisions
type Manager struct {
	Config Config
}

// Option is a functional option for configuring a spreading Manager.
type Option func(*Config)

// WithEnabled overrides the Enabled field of the default manager config.
// Pass spreading.WithEnabled(features.EnableStartupSpreading) from cmd/main.go.
func WithEnabled(enabled bool) Option {
	return func(c *Config) {
		c.Enabled = enabled
	}
}

// NewManager creates a new spreading manager
func NewManager(config Config) *Manager {
	return &Manager{
		Config: config,
	}
}

// WithSpreadPeriod overrides the SpreadPeriod field of the default manager config.
// Pass spreading.WithSpreadPeriod(cfg.SpreadPeriodMinutes) from cmd/main.go.
func WithSpreadPeriod(minutes int) Option {
	return func(c *Config) {
		c.SpreadPeriod = time.Duration(minutes) * time.Minute
	}
}

// WithSpreadInterval overrides the SpreadInterval field of the default manager config.
// Pass spreading.WithSpreadInterval(cfg.SpreadIntervalMinutes) from cmd/main.go.
func WithSpreadInterval(minutes int) Option {
	return func(c *Config) {
		c.SpreadInterval = time.Duration(minutes) * time.Minute
	}
}

// NewDefaultManager creates a manager with default configuration from environment variables.
// The Enabled flag and numeric tuning values are not read from env here; pass the relevant
// With* opts from cmd/main.go so that configuration ownership stays in internal/config.
func NewDefaultManager(opts ...Option) *Manager {
	config := Config{
		SpreadSeed:     uuid.New().String(),
		StartTime:      time.Now(),
		Enabled:        true,
		SpreadPeriod:   DefaultSpreadPeriodMinutes * time.Minute,
		SpreadInterval: DefaultSpreadIntervalMinutes * time.Minute,
	}
	for _, opt := range opts {
		opt(&config)
	}
	return &Manager{Config: config}
}

// Spread returns a RequiresSpreadError if a reconciliation should be delayed for spreading
func (m *Manager) Spread(ctx context.Context, resource SpreadableResource, currentSubResourceGenerations map[string]int64) error {
	log := logf.FromContext(ctx)

	if m == nil || !m.Config.Enabled {
		return nil
	}

	// We're outside the spreading window - reconcile immediately
	if time.Since(m.Config.StartTime) > m.Config.SpreadPeriod {
		log.V(1).Info("Outside grace period, not spreading")
		return nil
	}
	// Spec has changed since last reconciliation
	if resource.GetGeneration() != resource.GetObservedGeneration() {
		log.V(1).Info("Resource generation changed, not spreading",
			"generation", resource.GetGeneration(),
			"observedGeneration", resource.GetObservedGeneration())
		return nil
	}
	// Resource is unhealthy (degraded state)
	if !resource.IsHealthy() {
		log.V(1).Info("Resource is unhealthy, not spreading")
		return nil
	}
	// Resource is being deleted
	if !resource.GetDeletionTimestamp().IsZero() {
		log.V(1).Info("Resource is being deleted, not spreading")
		return nil
	}

	// Subresources changed
	if subResourcesChanged(currentSubResourceGenerations, resource.GetObservedSubResourceGenerations()) {
		log.V(1).Info("Resources Subresource generations changed, not spreading",
			"currentSubResourceGenerations", currentSubResourceGenerations,
			"observedSubResourceGenerations", resource.GetObservedSubResourceGenerations())
		return nil
	}

	delay := m.calculateSpreadDelay()
	if delay <= 0 {
		log.V(1).Info("Calculated non-positive spread delay, not spreading")
		return nil
	}

	// All checks passed - this is a warm-start reconciliation that should be spread
	log.V(1).Info("Resource reconciliation should be spread out",
		"spreadDelay", delay,
	)

	return &RequiresSpreadError{
		RequeueAfter: delay,
	}
}

func subResourcesChanged(generations map[string]int64, generations2 map[string]int64) bool {
	if generations == nil && generations2 == nil {
		return false
	}
	if generations == nil || generations2 == nil {
		return true
	}
	if len(generations) != len(generations2) {
		return true
	}
	for key, gen := range generations {
		if gen2, exists := generations2[key]; !exists || gen != gen2 {
			return true
		}
	}
	return false
}

func (m *Manager) GetSpreadInterval() time.Duration {
	return m.Config.SpreadInterval
}

// calculateSpreadDelay calculates how long to delay a reconciliation for spreading
func (m *Manager) calculateSpreadDelay() time.Duration {
	// offset is x minute with x element of [0, SpreadInterval in minutes)
	intervalLength := int(m.Config.SpreadInterval.Minutes())
	if intervalLength < 1 {
		// prevent rand.IntN panic
		return 0
	}
	offset := time.Duration(rand.IntN(intervalLength)) * time.Minute
	spreadIntervalStart := m.Config.StartTime.Add(m.Config.SpreadPeriod)
	targetTime := spreadIntervalStart.Add(offset)
	// Calculate delay from now until target time
	delay := time.Until(targetTime)
	if delay < 0 {
		// Target time has passed - reconcile immediately
		// This can happen if we're checked late or if the spreading window has expired
		return 0
	}
	return delay
}

// hashString creates a stable hash of a string
func hashString(s string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return int(h.Sum32())
}
