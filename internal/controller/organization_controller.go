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
	"bytes"
	"context"
	"errors"
	"time"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/ratelimit"
	"github.com/Interhyp/git-hubby/internal/reconciler/orgrec"
	"github.com/Interhyp/git-hubby/internal/reconciler/reconcilerfactory"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// CredentialsInvalidator is implemented by client factories that support per-secret
// credential cache invalidation. When a secret changes, all cached clients that were
// built from that secret are evicted so fresh credentials are fetched on the next request.
type CredentialsInvalidator interface {
	InvalidateCredentials(secretName string)
}

// OrganizationCtl reconciles a Organization object
type OrganizationCtl struct {
	GithubRateLimiter      *ratelimit.GitHubRateLimiter
	Scheme                 *runtime.Scheme
	ReconcilerFactory      *reconcilerfactory.Factory
	SuccessRequeueInterval time.Duration
	// LegacySecretName is the credential secret name used for Organizations that still
	// reference the deprecated GitHubAppInstallationId field.
	LegacySecretName string
	// ClientFactory is used to invalidate cached credentials when a credential secret changes.
	ClientFactory CredentialsInvalidator
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

// secretDataChangedPredicate is a predicate that fires only when a Secret's Data changes
// (not on metadata-only updates) and on deletions. Creates are intentionally ignored because
// a newly-created secret is not yet referenced by any cached client.
type secretDataChangedPredicate struct{}

func (secretDataChangedPredicate) Create(_ event.CreateEvent) bool { return false }

func (secretDataChangedPredicate) Delete(_ event.DeleteEvent) bool { return true }

func (secretDataChangedPredicate) Update(e event.UpdateEvent) bool {
	oldSecret, ok := e.ObjectOld.(*v1.Secret)
	if !ok {
		return false
	}
	newSecret, ok := e.ObjectNew.(*v1.Secret)
	if !ok {
		return false
	}
	return !secretDataEqual(oldSecret.Data, newSecret.Data)
}

func (secretDataChangedPredicate) Generic(_ event.GenericEvent) bool { return false }

// secretDataEqual reports whether two Secret data maps are equal.
func secretDataEqual(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok || !bytes.Equal(va, vb) {
			return false
		}
	}
	return true
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
		// not to the secondary watches (CSC, RulesetPreset, Secret) so each can use its own filter.
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
		Watches( // changes of credential secrets should invalidate caches and trigger re-reconciliation
			&v1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findOrganizationsForSecret),
			builder.WithPredicates(secretDataChangedPredicate{}),
		).
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

// findOrganizationsForSecret is called when a credential Secret changes. It invalidates
// any cached clients that used that secret and returns reconcile requests for all
// Organizations that reference the secret so they re-authenticate with fresh credentials.
func (r *OrganizationCtl) findOrganizationsForSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	log := logf.FromContext(ctx)
	secretName := obj.GetName()

	// Invalidate cached credentials so the next reconciliation fetches fresh ones.
	if r.ClientFactory != nil {
		r.ClientFactory.InvalidateCredentials(secretName)
	}

	// Find all Organizations that reference this secret (directly or via legacy fallback).
	var orgList githubv1alpha1.OrganizationList
	if err := r.ReconcilerFactory.K8sClient.List(ctx, &orgList); err != nil {
		log.Error(err, "failed to list organizations for secret", "secretName", secretName)
		return nil
	}

	var requests []reconcile.Request
	for _, org := range orgList.Items {
		cfg, err := org.ResolveGitHubAppConfig(r.LegacySecretName)
		if err != nil {
			continue
		}
		if cfg.CredentialsSecretName == secretName {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      org.Name,
					Namespace: org.Namespace,
				},
			})
		}
	}

	if len(requests) > 0 {
		log.Info("Enqueuing Organization reconciliations due to secret change", "secretName", secretName, "count", len(requests))
	}
	return requests
}
