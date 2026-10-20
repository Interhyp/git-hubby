package controller

import (
	"context"
	"errors"
	"time"

	"github.com/Interhyp/git-hubby/internal/config"
	"github.com/Interhyp/git-hubby/internal/ghclient"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/internal/reconciler/spreading"
	"github.com/gofri/go-github-ratelimit/v2/github_ratelimit/github_primary_ratelimit"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

// minRateLimitRequeueDelay is the minimum delay before a rate-limited resource is reconciled again, to avoid tight
// loops if the reset time is already in the past.
const minRateLimitRequeueDelay = 30 * time.Second

// RequeuePolicy decides when the controllers reconcile a resource again. It is shared by all controllers, so that
// resources of all kinds are requeued consistently.
type RequeuePolicy struct {
	// successInterval is used after successful reconciliations (drift detection) and for errors that are fixed by
	// changing Kubernetes resources (watch events trigger the reconciliation earlier).
	successInterval time.Duration
	// gitHubStateInterval is used for errors that are fixed by a change on GitHub or by creating a referenced
	// Kubernetes resource. Capped at successInterval; ignored if <= 0.
	gitHubStateInterval time.Duration
}

// NewRequeuePolicy creates the requeue policy from the operator configuration.
func NewRequeuePolicy(cfg config.Config) *RequeuePolicy {
	return &RequeuePolicy{
		successInterval:     cfg.SuccessRequeueInterval(),
		gitHubStateInterval: cfg.GitHubStateRequeueInterval(),
	}
}

// OnSuccess returns the result after a successful reconciliation: the resource is reconciled again after the success
// interval to detect drift on GitHub.
func (p *RequeuePolicy) OnSuccess() ctrl.Result {
	return ctrl.Result{RequeueAfter: p.successInterval}
}

// OnFactoryError returns the result for errors that occur before the reconciliation, i.e. while the reconciler is
// created (see reconcilerfactory.Factory):
//   - canceled context (shutdown): no requeue
//   - GitHub rate limit reached: requeue after the reset time (at least minRateLimitRequeueDelay)
//   - startup spreading: requeue after the calculated delay
//   - any other error: returned for a requeue with exponential backoff
func (p *RequeuePolicy) OnFactoryError(ctx context.Context, err error) (ctrl.Result, error) {
	// Context canceled during shutdown (SIGTERM) — not a real error, no requeue needed.
	// Note: context.DeadlineExceeded (from the controller's own 5-minute timeout) is a
	// legitimate error and should still be requeued, so we only check for context.Canceled.
	if errors.Is(ctx.Err(), context.Canceled) {
		logPkg.FromContext(ctx).Info("Reconciliation canceled due to shutdown")
		return ctrl.Result{}, nil
	}

	var resetTime *time.Time
	if libRateLimitErr, ok := errors.AsType[*github_primary_ratelimit.RateLimitReachedError](err); ok {
		resetTime = libRateLimitErr.ResetTime
	}
	if ownRateLimitErr, ok := errors.AsType[*ghclient.RateLimitedError](err); resetTime == nil && ok {
		resetTime = &ownRateLimitErr.ResetTime
	}
	if resetTime != nil {
		delay := max(time.Until(*resetTime), minRateLimitRequeueDelay)
		logPkg.FromContext(ctx).Info("GitHub API rate limit reached, requeuing after reset time", "resetTime", *resetTime, "requeueAfter", delay)
		return ctrl.Result{RequeueAfter: delay}, nil
	}

	if requiresSpreadError, ok := errors.AsType[*spreading.RequiresSpreadError](err); ok {
		return ctrl.Result{RequeueAfter: requiresSpreadError.RequeueAfter}, nil
	}

	return ctrl.Result{}, err // normal error requeue with exponential backoff
}

// OnReconcileError returns the result for errors of the reconciliation. Errors that only require user action (see
// reconciler.RequiresUserAction), e.g. conflicting definitions, unresolvable ruleset chains, missing references or a
// missing default branch, are requeued without error (no exponential backoff); the affected conditions have already
// been set to False with reason UserActionRequired by the executor. The requeue interval depends on where the user
// has to act:
//   - Kubernetes resources only (spec, presets): watches reconcile the resource once they change, so the success
//     interval is used.
//   - GitHub (see reconciler.AwaitsGitHubChange, e.g. a missing team or GitHub App): no watch event signals the
//     change, so the shorter GitHub state interval is used.
//   - Missing referenced Kubernetes resources (NotFound): watched kinds (presets, code security configurations)
//     reconcile the resource once they are created, but not all referenced kinds are watched (e.g. secrets). The
//     GitHub state interval is used as well, so that resources created later (e.g. in a later Argo CD sync wave)
//     are picked up without tracking which kinds are watched.
//
// All other errors are logged and handled like errors of the factory, see OnFactoryError.
func (p *RequeuePolicy) OnReconcileError(ctx context.Context, err error) (ctrl.Result, error) {
	log := logPkg.FromContext(ctx)
	if reconciler.RequiresUserAction(err) {
		requeueAfter := p.userActionRequeueInterval(err)
		log.Info("Reconciliation requires user action, waiting for a change", "reason", err.Error(), "requeueAfter", requeueAfter)
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}
	if !errors.Is(ctx.Err(), context.Canceled) { // only log if not a shutdown cancellation
		log.Error(err, "Reconciliation failed")
	}
	return p.OnFactoryError(ctx, err)
}

// userActionRequeueInterval returns the requeue interval for errors requiring user action, see OnReconcileError.
func (p *RequeuePolicy) userActionRequeueInterval(err error) time.Duration {
	if (reconciler.AwaitsGitHubChange(err) || apierrors.IsNotFound(err)) && p.gitHubStateInterval > 0 {
		return min(p.successInterval, p.gitHubStateInterval)
	}
	return p.successInterval
}
