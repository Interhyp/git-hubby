package controller

import (
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func resourceWasDeleted[T reconciler.ReconcilableResource](r reconciler.Reconciler[T]) bool {
	return !r.K8s().Resource.GetDeletionTimestamp().IsZero() &&
		!controllerutil.ContainsFinalizer(r.K8s().Resource, r.FinalizerName())
}
