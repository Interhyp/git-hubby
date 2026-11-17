package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// RequeueReason explains why a resource is reconciled again, see requeuesTotal.
type RequeueReason string

const (
	// RequeueReasonSuccess is the regular requeue after a successful reconciliation (drift detection).
	RequeueReasonSuccess RequeueReason = "success"
	// RequeueReasonRateLimit is used if the GitHub API rate limit is (nearly) exhausted.
	RequeueReasonRateLimit RequeueReason = "rate_limit"
	// RequeueReasonStartupSpread is used for reconciliations delayed by the startup spreading.
	RequeueReasonStartupSpread RequeueReason = "startup_spread"
	// RequeueReasonUserAction is used for errors that require a change of Kubernetes resources.
	RequeueReasonUserAction RequeueReason = "user_action"
	// RequeueReasonAwaitsGitHub is used for errors that require a change on GitHub or a referenced resource to be created.
	RequeueReasonAwaitsGitHub RequeueReason = "awaits_github"
	// RequeueReasonRetryLater is used for transient states on GitHub that resolve on their own.
	RequeueReasonRetryLater RequeueReason = "retry_later"
	// RequeueReasonArchivedRepository is used if a repository was archived on GitHub during the reconciliation.
	RequeueReasonArchivedRepository RequeueReason = "archived_repository"
	// RequeueReasonNewRepository is used while GitHub initializes a newly created repository.
	RequeueReasonNewRepository RequeueReason = "new_repository"
	// RequeueReasonDependentsExist is used if an organization cannot be deleted while repositories or teams still exist.
	RequeueReasonDependentsExist RequeueReason = "dependents_exist"
	// RequeueReasonError is used for errors that are retried by controller-runtime with exponential backoff.
	RequeueReasonError RequeueReason = "error"
	// RequeueReasonShutdown is used if the reconciliation was canceled by a shutdown (no requeue).
	RequeueReasonShutdown RequeueReason = "shutdown"
)

// requeuesTotal counts the decisions of the RequeuePolicy. controller-runtime only reports the result
// (requeue_after, error, ...), which does not tell drift detection apart from rate limits or required user action.
var requeuesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "git_hubby_requeues_total",
	Help: "Number of requeue decisions per controller and reason",
}, []string{"controller", "reason"})

func init() {
	ctrlmetrics.Registry.MustRegister(requeuesTotal)
}

func countRequeue(controllerName string, reason RequeueReason) {
	requeuesTotal.WithLabelValues(controllerName, string(reason)).Inc()
}
