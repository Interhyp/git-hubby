package spreading

import (
	"context"
	"hash/fnv"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Interhyp/git-hubby/internal/config"
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
	// StartTime is the start of the spreading window. It is initialized when the manager is created and reset when
	// the pod becomes leader (see Manager.Start), because the controllers only start reconciling then.
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

// Manager handles startup spreading decisions. It is a manager.Runnable that needs leader election: when the pod
// becomes leader, the spreading window starts anew (see Start).
type Manager struct {
	Config Config

	// mu guards Config.StartTime, which is reset on leader election while reconciliations read it.
	mu sync.RWMutex
}

// NewManager creates a spreading manager from the operator configuration. The start time is the time of creation and
// the seed is random, so that resources are spread differently on each pod restart.
func NewManager(cfg config.Config) *Manager {
	return &Manager{
		Config: Config{
			StartTime:      time.Now(),
			SpreadSeed:     uuid.New().String(),
			SpreadPeriod:   cfg.SpreadPeriod(),
			SpreadInterval: cfg.SpreadInterval(),
			Enabled:        cfg.Features.EnableStartupSpreading,
		},
	}
}

// Start implements manager.Runnable. Because it needs leader election (see NeedLeaderElection), it is called when
// this pod becomes leader, i.e. right before the controllers start to reconcile all resources. The spreading window
// is restarted at that point: a standby pod that becomes leader long after its process start (e.g. after the leader
// was evicted) would otherwise already be outside of the spread period and reconcile all resources at once.
//
// Start blocks until the context is canceled, as required for runnables.
func (m *Manager) Start(ctx context.Context) error {
	if m != nil {
		m.ResetStartTime(ctx)
	}
	<-ctx.Done()
	return nil
}

// NeedLeaderElection implements manager.LeaderElectionRunnable.
func (m *Manager) NeedLeaderElection() bool {
	return true
}

// ResetStartTime restarts the spreading window now.
func (m *Manager) ResetStartTime(ctx context.Context) {
	now := time.Now()
	m.mu.Lock()
	m.Config.StartTime = now
	m.mu.Unlock()
	logf.FromContext(ctx).Info("Startup spreading window started",
		"startTime", now,
		"spreadPeriod", m.Config.SpreadPeriod,
		"spreadInterval", m.Config.SpreadInterval,
		"enabled", m.Config.Enabled)
}

func (m *Manager) startTime() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Config.StartTime
}

// Spread returns a RequiresSpreadError if a reconciliation should be delayed for spreading
func (m *Manager) Spread(ctx context.Context, resource SpreadableResource, currentSubResourceGenerations map[string]int64) error {
	log := logf.FromContext(ctx)

	if m == nil || !m.Config.Enabled {
		return nil
	}

	// We're outside the spreading window - reconcile immediately
	if time.Since(m.startTime()) > m.Config.SpreadPeriod {
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

// calculateSpreadDelay calculates how long to delay a reconciliation for spreading
func (m *Manager) calculateSpreadDelay() time.Duration {
	// offset is x minute with x element of [0, SpreadInterval in minutes)
	intervalLength := int(m.Config.SpreadInterval.Minutes())
	if intervalLength < 1 {
		// prevent rand.IntN panic
		return 0
	}
	offset := time.Duration(rand.IntN(intervalLength)) * time.Minute
	spreadIntervalStart := m.startTime().Add(m.Config.SpreadPeriod)
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
