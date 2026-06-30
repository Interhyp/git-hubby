package controller

import (
	"context"
	"errors"
	"time"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/ratelimit"
	"github.com/Interhyp/git-hubby/internal/reconciler/orgrec"
	"github.com/Interhyp/git-hubby/internal/reconciler/reconcilerfactory"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// OrganizationCtl reconciles a Organization object
type OrganizationCtl struct {
	GithubRateLimiter      *ratelimit.GitHubRateLimiter
	Scheme                 *runtime.Scheme
	ReconcilerFactory      *reconcilerfactory.Factory
	SuccessRequeueInterval time.Duration
	// LegacySecretName is the credential secret name used for Organizations that still
	// reference the deprecated GitHubAppInstallationId field.
	LegacySecretName string
}

// +kubebuilder:rbac:groups=github.interhyp.de,namespace=github-configuration,resources=organizations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=github.interhyp.de,namespace=github-configuration,resources=organizations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=github.interhyp.de,namespace=github-configuration,resources=organizations/finalizers,verbs=update
// +kubebuilder:rbac:groups=github.interhyp.de,namespace=github-configuration,resources=codesecurityconfigurations,verbs=get;list;watch
// +kubebuilder:rbac:groups=github.interhyp.de,namespace=github-configuration,resources=rulesetpresets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *OrganizationCtl) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	rec, err := r.ReconcilerFactory.CreateForOrg(ctx, req.NamespacedName)
	if err != nil {
		return handleRequeueError(ctx, err, r.GithubRateLimiter)
	}
	if rec == nil {
		return ctrl.Result{}, nil // no requeue, k8s resource not found
	}

	if err = rec.Reconcile(ctx); err != nil {
		if errors.Is(err, orgrec.OrganizationStillHasReposError()) || errors.Is(err, orgrec.OrganizationStillHasTeamsError()) {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
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
func (r *OrganizationCtl) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &githubv1alpha1.Organization{}, "spec.codeSecurityConfigurationAttachments.name", func(rawObj client.Object) []string {
		org := rawObj.(*githubv1alpha1.Organization)
		cscNames := make([]string, 0, len(org.Spec.CodeSecurityConfigurations))
		for _, attachment := range org.Spec.CodeSecurityConfigurations {
			cscNames = append(cscNames, attachment.Name)
		}
		return cscNames
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &githubv1alpha1.Organization{}, "spec.rulesetPresets.name", func(rawObj client.Object) []string {
		org := rawObj.(*githubv1alpha1.Organization)
		rulesets := make([]string, 0, len(org.Spec.RulesetPresetList))
		for _, ruleset := range org.Spec.RulesetPresetList {
			rulesets = append(rulesets, ruleset.Name)
		}
		return rulesets
	}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		// Apply the generation/annotation predicate only to the primary Organization resource,
		// not to the secondary watches (CSC, RulesetPreset) so each can use its own filter.
		For(&githubv1alpha1.Organization{},
			builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})),
		).
		Watches( // changes of csc should trigger reconciliation of orgs using it
			&githubv1alpha1.CodeSecurityConfiguration{},
			handler.EnqueueRequestsFromMapFunc(r.findOrganizationsForCodeSecurityConfiguration),
		).
		Watches( // changes of ruleset presets should trigger reconciliation of orgs using it
			&githubv1alpha1.RulesetPreset{},
			handler.EnqueueRequestsFromMapFunc(r.findOrganizationsForRulesetPreset),
		).
		// NOTE: credential secrets are fetched directly (non-cached) on each reconciliation;
		// no Secret watch is registered so the credentials namespace does not need to be
		// added to the manager's cache, avoiding RBAC errors in unrelated namespaces.
		// A pod restart is required to pick up rotated credentials.
		WithOptions(controller.Options{
			UsePriorityQueue: new(true),
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter(
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](
					1*time.Second,    // base delay
					1000*time.Second, // max delay, ~17min
				),
				ratelimit.NewControllerRuntimeRateLimiter[reconcile.Request](r.GithubRateLimiter),
			),
		}).
		Named("organization").
		Complete(r)
}

func (r *OrganizationCtl) findOrganizationsForCodeSecurityConfiguration(ctx context.Context, obj client.Object) []reconcile.Request {
	csc := obj.(*githubv1alpha1.CodeSecurityConfiguration)

	var orgList githubv1alpha1.OrganizationList
	if err := r.ReconcilerFactory.K8sClient.List(ctx, &orgList, client.MatchingFields{"spec.codeSecurityConfigurationAttachments.name": csc.Name}); err != nil {
		logf.FromContext(ctx).Error(err, "failed to list organizations for code security configuration", "codeSecurityConfiguration", csc.Name)
		return nil
	}

	requests := make([]reconcile.Request, 0, len(orgList.Items))
	for _, org := range orgList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      org.Name,
				Namespace: org.Namespace,
			},
		})
	}

	return requests
}

func (r *OrganizationCtl) findOrganizationsForRulesetPreset(ctx context.Context, obj client.Object) []reconcile.Request {
	rulesetPreset := obj.(*githubv1alpha1.RulesetPreset)

	var orgList githubv1alpha1.OrganizationList
	if err := r.ReconcilerFactory.K8sClient.List(ctx, &orgList, client.MatchingFields{"spec.rulesetPresets.name": rulesetPreset.Name}); err != nil {
		logf.FromContext(ctx).Error(err, "failed to list organizations for ruleset preset", "rulesetPreset", rulesetPreset.Name)
		return nil
	}

	requests := make([]reconcile.Request, 0, len(orgList.Items))
	for _, org := range orgList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      org.Name,
				Namespace: org.Namespace,
			},
		})
	}

	return requests
}
