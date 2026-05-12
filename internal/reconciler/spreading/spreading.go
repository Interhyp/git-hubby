package spreading

import (
	"context"
	"hash/fnv"
	"math/rand/v2"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
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

// NewManager creates a new spreading manager
func NewManager(config Config) *Manager {
	return &Manager{
		Config: config,
	}
}

// NewDefaultManager creates a manager with default configuration from environment variables
func NewDefaultManager() *Manager {
	config := Config{
		SpreadSeed:     uuid.New().String(),
		StartTime:      time.Now(),
		Enabled:        getBoolEnv("ENABLE_STARTUP_SPREADING", true),
		SpreadPeriod:   getDurationEnv("STARTUP_SPREAD_PERIOD_MINUTES", 5) * time.Minute,
		SpreadInterval: getDurationEnv("SPREAD_INTERVAL_MINUTES", 180) * time.Minute,
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

// getBoolEnv reads a boolean from environment variable with a default
func getBoolEnv(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

// getDurationEnv reads a duration in minutes from environment variable with a default
func getDurationEnv(key string, defaultMinutes int) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return time.Duration(defaultMinutes)
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return time.Duration(defaultMinutes)
	}
	return time.Duration(parsed)
}
