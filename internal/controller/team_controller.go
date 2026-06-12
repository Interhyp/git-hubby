/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"errors"
	"time"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/ratelimit"
	"github.com/Interhyp/git-hubby/internal/reconciler/reconcilerfactory"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// TeamCtl reconciles a Repository object
type TeamCtl struct {
	GithubRateLimiter      *ratelimit.GitHubRateLimiter
	Scheme                 *runtime.Scheme
	ReconcilerFactory      *reconcilerfactory.Factory
	SuccessRequeueInterval time.Duration
}

// +kubebuilder:rbac:groups=github.interhyp.de,namespace=github-configuration,resources=teams,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=github.interhyp.de,namespace=github-configuration,resources=teams/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=github.interhyp.de,namespace=github-configuration,resources=teams/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Team object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/reconcile
func (r *TeamCtl) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	rec, err := r.ReconcilerFactory.CreateForTeam(ctx, req.NamespacedName)
	if err != nil {
		return handleRequeueError(ctx, err, r.GithubRateLimiter)
	}
	if rec == nil {
		return ctrl.Result{}, nil // no requeue, k8s resource not found
	}

	if err = rec.Reconcile(ctx); err != nil {
		if !errors.Is(ctx.Err(), context.Canceled) { // only log if not a shutdown cancellation
			log.Error(err, "Reconciliation failed")
		}
		return handleRequeueError(ctx, err, r.GithubRateLimiter)
	}
	if resourceWasDeleted(rec.Reconciler) {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{RequeueAfter: r.SuccessRequeueInterval}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TeamCtl) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &githubv1alpha1.Team{}, "spec.organizationRefs", func(rawObj client.Object) []string {
		team := rawObj.(*githubv1alpha1.Team)
		orgNames := make([]string, 0, len(team.Spec.OrganizationRefs))
		for _, orgRef := range team.Spec.OrganizationRefs {
			orgNames = append(orgNames, orgRef.Name)
		}
		return orgNames
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&githubv1alpha1.Team{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		WithOptions(controller.Options{
			UsePriorityQueue:        new(true),
			MaxConcurrentReconciles: 20,
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter(
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](
					1*time.Second,    // base delay
					1000*time.Second, // max delay, ~17min
				),
				ratelimit.NewControllerRuntimeRateLimiter[reconcile.Request](r.GithubRateLimiter),
			),
		}).
		Named("team").
		Complete(r)
}
