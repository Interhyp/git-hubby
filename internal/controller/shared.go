package controller

import (
	"context"
	"errors"
	"time"

	"github.com/Interhyp/git-hubby/internal/ghclient"
	"github.com/Interhyp/git-hubby/internal/ratelimit"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/internal/reconciler/spreading"
	"github.com/gofri/go-github-ratelimit/v2/github_ratelimit/github_primary_ratelimit"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

func handleRequeueError(ctx context.Context, err error, limiter *ratelimit.GitHubRateLimiter) (controllerruntime.Result, error) {
	// Context canceled during shutdown (SIGTERM) — not a real error, no requeue needed.
	// Note: context.DeadlineExceeded (from the controller's own 5-minute timeout) is a
	// legitimate error and should still be requeued, so we only check for context.Canceled.
	if errors.Is(ctx.Err(), context.Canceled) {
		logPkg.FromContext(ctx).V(1).Info("Reconciliation canceled due to shutdown")
		return controllerruntime.Result{}, nil
	}

	var resetTime *time.Time

	// requeue because rate limit was hit
	var libRateLimitErr *github_primary_ratelimit.RateLimitReachedError
	if errors.As(err, &libRateLimitErr) {
		resetTime = libRateLimitErr.ResetTime
	}
	var ownRateLimitErr *ghclient.RateLimitedError
	if resetTime == nil && errors.As(err, &ownRateLimitErr) {
		resetTime = &ownRateLimitErr.ResetTime
	}
	if resetTime != nil {
		logPkg.FromContext(ctx).V(1).Info("GitHub API rate limit reached, requeuing after reset time", "resetTime", *resetTime)
		limiter.BlockUntil(*resetTime)
		return controllerruntime.Result{RequeueAfter: time.Until(*resetTime)}, nil
	}

	// requeue because spreading requires it
	var requiresSpreadError *spreading.RequiresSpreadError
	if errors.As(err, &requiresSpreadError) {
		return controllerruntime.Result{RequeueAfter: requiresSpreadError.RequeueAfter}, nil
	}

	return controllerruntime.Result{}, err // normal error requeue with exponential backoff
}

func resourceWasDeleted[T reconciler.ReconcilableResource](r reconciler.Reconciler[T]) bool {
	return !r.K8s().Resource.GetDeletionTimestamp().IsZero() &&
		!controllerutil.ContainsFinalizer(r.K8s().Resource, r.FinalizerName())
}
