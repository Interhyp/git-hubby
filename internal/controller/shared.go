package controller

import (
	"context"
	"errors"
	"time"

	"github.com/Interhyp/git-hubby/internal/ghclient"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/internal/reconciler/spreading"
	"github.com/gofri/go-github-ratelimit/v2/github_ratelimit/github_primary_ratelimit"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

func handleRequeueError(ctx context.Context, err error) (controllerruntime.Result, error) {
	// Context canceled during shutdown (SIGTERM) — not a real error, no requeue needed.
	// Note: context.DeadlineExceeded (from the controller's own 5-minute timeout) is a
	// legitimate error and should still be requeued, so we only check for context.Canceled.
	if errors.Is(ctx.Err(), context.Canceled) {
		logPkg.FromContext(ctx).Info("Reconciliation canceled due to shutdown")
		return controllerruntime.Result{}, nil
	}

	var resetTime *time.Time

	// requeue because rate limit was hit
	if libRateLimitErr, ok := errors.AsType[*github_primary_ratelimit.RateLimitReachedError](err); ok {
		resetTime = libRateLimitErr.ResetTime
	}
	var ownRateLimitErr *ghclient.RateLimitedError
	if resetTime == nil && errors.As(err, &ownRateLimitErr) {
		resetTime = &ownRateLimitErr.ResetTime
	}
	if resetTime != nil {
		// requeue after at least 30 seconds
		delay := max(time.Until(*resetTime), 30*time.Second)
		logPkg.FromContext(ctx).Info("GitHub API rate limit reached, requeuing after reset time", "resetTime", *resetTime, "requeueAfter", delay)
		return controllerruntime.Result{RequeueAfter: delay}, nil
	}

	// requeue because spreading requires it
	if requiresSpreadError, ok := errors.AsType[*spreading.RequiresSpreadError](err); ok {
		return controllerruntime.Result{RequeueAfter: requiresSpreadError.RequeueAfter}, nil
	}

	return controllerruntime.Result{}, err // normal error requeue with exponential backoff
}

// handleUserActionRequired returns a requeue result without error (no exponential backoff) if err only consists of
// errors that require user action (see reconciler.RequiresUserAction), e.g. conflicting definitions, unresolvable
// ruleset chains, missing references or a missing default branch. The affected conditions have already been set to
// False with reason UserActionRequired by the executor.
//
// The requeue interval depends on where the user has to act:
//   - Kubernetes resources only (spec, presets): watches reconcile the resource once they change, so the regular
//     successRequeueInterval is used.
//   - GitHub (see reconciler.AwaitsGitHubChange, e.g. a missing team or GitHub App): no watch event signals the
//     change, so the shorter gitHubStateRequeueInterval is used (capped at successRequeueInterval; ignored if <= 0).
//   - Missing referenced Kubernetes resources (NotFound): watched kinds (presets, code security configurations)
//     reconcile the resource once they are created, but not all referenced kinds are watched (e.g. secrets).
//     gitHubStateRequeueInterval is used as well, so that resources created later (e.g. in a later Argo CD sync
//     wave) are picked up without tracking which kinds are watched.
func handleUserActionRequired(ctx context.Context, err error, successRequeueInterval, gitHubStateRequeueInterval time.Duration) (controllerruntime.Result, bool) {
	if !reconciler.RequiresUserAction(err) {
		return controllerruntime.Result{}, false
	}
	requeueAfter := successRequeueInterval
	if (reconciler.AwaitsGitHubChange(err) || apierrors.IsNotFound(err)) && gitHubStateRequeueInterval > 0 {
		requeueAfter = min(requeueAfter, gitHubStateRequeueInterval)
	}
	logPkg.FromContext(ctx).Info("Reconciliation requires user action, waiting for a change", "reason", err.Error(), "requeueAfter", requeueAfter)
	return controllerruntime.Result{RequeueAfter: requeueAfter}, true
}

func resourceWasDeleted[T reconciler.ReconcilableResource](r reconciler.Reconciler[T]) bool {
	return !r.K8s().Resource.GetDeletionTimestamp().IsZero() &&
		!controllerutil.ContainsFinalizer(r.K8s().Resource, r.FinalizerName())
}
