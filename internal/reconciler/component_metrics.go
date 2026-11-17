package reconciler

import (
	"time"

	"github.com/Interhyp/git-hubby/internal/conditions"
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Outcomes of a component reconciliation, see componentDuration.
const (
	componentOutcomeSuccess    = "success"
	componentOutcomeError      = "error"
	componentOutcomeUserAction = "user_action"
	componentOutcomeRetryLater = "retry_later"
	componentOutcomeStopped    = "stopped"
)

var (
	// componentDuration measures the reconciliation of the components (rulesets, webhooks, team members, ...) of a
	// resource. Its buckets reach the ReconciliationContextTimeout, unlike those of controller-runtime (60s).
	componentDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "git_hubby_component_reconcile_duration_seconds",
		Help:    "Duration of the reconciliation of a component of a resource per kind, component and outcome",
		Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20, 30, 60, 120, 300},
	}, []string{"kind", "component", "outcome"})

	// componentSkipped counts components that were not reconciled because an earlier component stopped the run.
	componentSkipped = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "git_hubby_component_reconcile_skipped_total",
		Help: "Number of component reconciliations skipped because an earlier component stopped the run",
	}, []string{"kind", "component"})
)

func init() {
	ctrlmetrics.Registry.MustRegister(componentDuration, componentSkipped)
}

func observeComponent(kind string, component conditions.ConditionType, err error, duration time.Duration) {
	componentDuration.WithLabelValues(kind, string(component), componentOutcome(err)).Observe(duration.Seconds())
}

func countSkippedComponents(kind string, results []reconciliationResult) {
	for _, result := range results {
		if result.SkippedBy != nil {
			componentSkipped.WithLabelValues(kind, string(result.Condition)).Inc()
		}
	}
}

func componentOutcome(err error) string {
	if err == nil {
		return componentOutcomeSuccess
	}
	if _, ok := asStopReconciliation(err); ok {
		return componentOutcomeStopped
	}
	if _, ok := RetryLater(err); ok {
		return componentOutcomeRetryLater
	}
	if RequiresUserAction(err) {
		return componentOutcomeUserAction
	}
	return componentOutcomeError
}
