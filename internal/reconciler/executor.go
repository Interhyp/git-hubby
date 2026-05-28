package reconciler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Interhyp/git-hubby/internal/conditions"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

const ReconciliationContextTimeout = 5 * time.Minute

// ReconciliationExecutor provides the common reconciliation process for all kubernetes resources
// Any resource specific logic is delegated to the Reconciler[T] implementation
type ReconciliationExecutor[T ReconcilableResource] struct {
	Reconciler Reconciler[T]
}

// reconciliationResult captures the outcome of a single reconciliation
type reconciliationResult struct {
	Condition  conditions.ConditionType
	Error      error
	FinishedAt time.Time
}

// Reconcile performs the reconciliation process for the resource of type T
func (x *ReconciliationExecutor[T]) Reconcile(ctx context.Context) error {
	logFields := x.Reconciler.GetAdditionalLogFields()
	logFields = append(logFields, "apiVersion", x.Reconciler.K8s().Resource.GetObjectKind().GroupVersionKind().Version)
	log := logPkg.FromContext(ctx, logFields...)
	ctx = logPkg.IntoContext(ctx, log)
	log.Info(fmt.Sprintf("Reconciling %s", x.Reconciler.K8s().Resource.GetTypeRepresentation()))

	x.updateStartCondition()
	if statusErr := x.applyStatus(ctx); statusErr != nil {
		log.WithValues("function", "updateStatus").Error(statusErr, fmt.Sprintf("Failed to update %s status", x.Reconciler.K8s().Resource.GetTypeRepresentation()))
	}

	if x.Reconciler.K8s().Resource.GetDeletionTimestamp() != nil {
		return x.finalize(ctx)
	}

	if err := x.applyMetadata(ctx, true); err != nil {
		log.Error(err, fmt.Sprintf("Failed to apply metadata for %s", x.Reconciler.K8s().Resource.GetTypeRepresentation()))
		return err
	}

	results, err := x.runReconciliations(ctx)

	x.updateConditionsFromResults(results)
	x.Reconciler.K8s().Resource.SetObservedSubResourceGeneration(x.Reconciler.K8s().CurrentSubResourceGenerations)
	if statusErr := x.applyStatus(ctx); statusErr != nil {
		log.WithValues("function", "updateStatus").Error(statusErr, fmt.Sprintf("Failed to update %s status", x.Reconciler.K8s().Resource.GetTypeRepresentation()))
		return statusErr
	}

	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Successfully reconciled %s", x.Reconciler.K8s().Resource.GetTypeRepresentation()))
	return nil
}

// applyStatus applies the current status via Server-Side Apply.
func (x *ReconciliationExecutor[T]) applyStatus(ctx context.Context) error {
	applyConfig := x.Reconciler.BuildStatusApplyConfig()
	return x.Reconciler.K8s().Client.Status().Apply(ctx, applyConfig, client.ForceOwnership, FieldOwner)
}

// finalize handles the deletion reconciliation process i.e. performing the necessary cleanup in GitHub before removing the finalizer
func (x *ReconciliationExecutor[T]) finalize(ctx context.Context) error {
	log := logPkg.FromContext(ctx)
	log.Info(fmt.Sprintf("%s is marked for deletion", x.Reconciler.K8s().Resource.GetTypeRepresentation()))

	if err := x.Reconciler.ReconcileDeletion(ctx); err != nil {
		return err
	}

	if err := x.applyMetadata(ctx, false); err != nil {
		log.Error(err, fmt.Sprintf("Failed to remove finalizer from %s", x.Reconciler.K8s().Resource.GetTypeRepresentation()))
		return err
	}
	controllerutil.RemoveFinalizer(x.Reconciler.K8s().Resource, x.Reconciler.FinalizerName())

	return nil
}

// applyMetadata applies labels and optionally the finalizer via Server-Side Apply.
// When includeFinalizer is true, the finalizer is included in the apply configuration.
// When false, the apply configuration omits the finalizer, causing SSA to remove it
// from the field manager's ownership (effectively removing it).
func (x *ReconciliationExecutor[T]) applyMetadata(ctx context.Context, includeFinalizer bool) error {
	defaultLabels := labels.Set{
		"app.kubernetes.io/managed-by": "git-hubby",
	}
	allLabels := labels.Merge(defaultLabels, x.Reconciler.GetAdditionalLabels())

	var finalizers []string
	if includeFinalizer {
		finalizers = []string{x.Reconciler.FinalizerName()}
	} else {
		finalizers = []string{} // empty non-nil slice signals SSA to remove finalizers
	}

	applyConfig := x.Reconciler.BuildMetadataApplyConfig(allLabels, nil, finalizers)
	if err := x.Reconciler.K8s().Client.Apply(ctx, applyConfig, client.ForceOwnership, FieldOwner); err != nil {
		return err
	}

	// Update in-memory resource to reflect the applied labels
	x.Reconciler.K8s().Resource.SetLabels(labels.Merge(x.Reconciler.K8s().Resource.GetLabels(), allLabels))
	if includeFinalizer && !controllerutil.ContainsFinalizer(x.Reconciler.K8s().Resource, x.Reconciler.FinalizerName()) {
		controllerutil.AddFinalizer(x.Reconciler.K8s().Resource, x.Reconciler.FinalizerName())
	}

	return nil
}

// runReconciliations executes all reconciliation groups sequentially, running reconciliations within each group in parallel
// All errors are collected and returned at the end to avoid inconsistent states
func (x *ReconciliationExecutor[T]) runReconciliations(ctx context.Context) ([]reconciliationResult, error) {
	log := logPkg.FromContext(ctx)
	var results []reconciliationResult
	var allErrors []error

	for groupIdx, group := range x.Reconciler.RequiredReconciliations(ctx) {
		log.V(1).Info(fmt.Sprintf("Executing reconciliation group %d with %d reconciliations", groupIdx, len(group)))

		groupResults := x.runReconciliationGroup(ctx, group)
		results = append(results, groupResults...)

		// Process groupResults: collect errors
		for _, result := range groupResults {
			if result.Error != nil {
				allErrors = append(allErrors, result.Error)
			}
		}
		if len(allErrors) > 0 {
			break // Stop processing further groups on first group with error
		}
	}

	if len(allErrors) > 0 {
		return results, errors.Join(allErrors...)
	}

	return results, nil
}

// runReconciliationGroup executes all reconciliations in a group concurrently
func (x *ReconciliationExecutor[T]) runReconciliationGroup(ctx context.Context, group ParallelReconciliationGroup) []reconciliationResult {
	var wg sync.WaitGroup
	results := make([]reconciliationResult, len(group))

	for i, reconciliation := range group {
		wg.Add(1)
		go func(idx int, rec Reconciliation) {
			defer wg.Done()
			recCtx, cancel := context.WithTimeout(ctx, ReconciliationContextTimeout)
			defer cancel()
			recCtx = logPkg.IntoContext(recCtx, logPkg.FromContext(ctx).WithValues(
				"reconciliation", rec.Condition,
			))
			err := rec.Function(recCtx)
			results[idx] = reconciliationResult{
				Condition:  rec.Condition,
				Error:      err,
				FinishedAt: time.Now(),
			}
		}(i, reconciliation)
	}

	wg.Wait()
	return results
}

// conditionFromResult creates a new metav1.Condition with the provided values
func conditionFromResult(result reconciliationResult) metav1.Condition {
	condition := metav1.Condition{
		Type:               string(result.Condition),
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.NewTime(result.FinishedAt),
		Reason:             conditions.ReasonSyncCompleted,
		Message:            fmt.Sprintf("Sync completed successfully at %s", result.FinishedAt.Format(time.RFC3339)),
	}
	if result.Error != nil {
		condition.Message = fmt.Sprintf("Sync failed with %v at %s", result.Error, result.FinishedAt.Format(time.RFC3339))
		condition.Status = metav1.ConditionFalse
		condition.Reason = conditions.ReasonSyncFailed
	}
	return condition
}

func (x *ReconciliationExecutor[T]) updateConditionsFromResults(results []reconciliationResult) {
	allReady := true
	for _, result := range results {
		if result.Error != nil {
			allReady = false
		}
		meta.SetStatusCondition(x.Reconciler.K8s().Resource.GetConditions(), conditionFromResult(result))
	}
	readyCondition := metav1.Condition{
		Type:               string(conditions.TypeReady),
		ObservedGeneration: x.Reconciler.K8s().Resource.GetGeneration(),
	}
	if allReady {
		readyCondition.Status = metav1.ConditionTrue
		readyCondition.Reason = conditions.ReasonReconcileCompleted
		readyCondition.Message = fmt.Sprintf("All components synced successfully at %s", time.Now().Format(time.RFC3339))
	} else {
		readyCondition.Status = metav1.ConditionFalse
		readyCondition.Reason = conditions.ReasonReconcileFailed
		readyCondition.Message = fmt.Sprintf("Some components failed to sync during reconciliation at %s", time.Now().Format(time.RFC3339))
	}
	meta.SetStatusCondition(x.Reconciler.K8s().Resource.GetConditions(), readyCondition)
}

func (x *ReconciliationExecutor[T]) updateStartCondition() {
	meta.SetStatusCondition(x.Reconciler.K8s().Resource.GetConditions(), metav1.Condition{
		Type:    string(conditions.TypeReady),
		Status:  metav1.ConditionUnknown,
		Reason:  conditions.ReasonReconcileStarted,
		Message: fmt.Sprintf("Started reconciliation at %s", time.Now().Format(time.RFC3339)),
	})
}
