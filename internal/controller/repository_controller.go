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

	"github.com/Interhyp/git-hubby/internal/ratelimit"
	"github.com/Interhyp/git-hubby/internal/reconciler/reconcilerfactory"
	"github.com/Interhyp/git-hubby/internal/reconciler/reporec"
	"github.com/google/go-github/v86/github"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
)

// RepositoryCtl reconciles a Repository object
type RepositoryCtl struct {
	GithubRateLimiter      *ratelimit.GitHubRateLimiter
	Scheme                 *runtime.Scheme
	ReconcilerFactory      *reconcilerfactory.Factory
	SuccessRequeueInterval time.Duration
}

// +kubebuilder:rbac:groups=github.interhyp.de,namespace=github-configuration,resources=repositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=github.interhyp.de,namespace=github-configuration,resources=repositories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=github.interhyp.de,namespace=github-configuration,resources=repositories/finalizers,verbs=update
// +kubebuilder:rbac:groups="",namespace=github-configuration,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=github.interhyp.de,namespace=github-configuration,resources=webhookpresets,verbs=get;list;watch
// +kubebuilder:rbac:groups=github.interhyp.de,namespace=github-configuration,resources=webhookignorepresets,verbs=get;list;watch
// +kubebuilder:rbac:groups=github.interhyp.de,namespace=github-configuration,resources=rulesetpresets,verbs=get;list;watch
// +kubebuilder:rbac:groups=github.interhyp.de,namespace=github-configuration,resources=autolinkspresets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Repository object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/reconcile
func (r *RepositoryCtl) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	rec, err := r.ReconcilerFactory.CreateForRepo(ctx, req.NamespacedName)
	if err != nil {
		return handleRequeueError(ctx, err, r.GithubRateLimiter)
	}
	if rec == nil {
		return ctrl.Result{}, nil // no requeue, k8s resource not found
	}

	if err = rec.Reconcile(ctx); err != nil {
		if isArchivedRepoError(err) {
			log.Info("Repository is in read-only state, skipping reconciliation until next change")
			return ctrl.Result{}, nil
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

func isArchivedRepoError(err error) bool {
	var archivedErr *reporec.RepoArchivedError
	if errors.As(err, &archivedErr) {
		return true
	}
	var ghErr *github.ErrorResponse
	if errors.As(err, &ghErr) {
		return ghErr.Response != nil &&
			ghErr.Response.StatusCode == 403 &&
			ghErr.Message == "Repository was archived so is read-only."
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *RepositoryCtl) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &githubv1alpha1.Repository{}, "spec.organizationRef.name", func(rawObj client.Object) []string {
		repo := rawObj.(*githubv1alpha1.Repository)
		return []string{repo.Spec.OrganizationRef.Name}
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &githubv1alpha1.Repository{}, "spec.attachedCodeSecurityConfiguration.name", func(rawObj client.Object) []string {
		repo := rawObj.(*githubv1alpha1.Repository)
		result := make([]string, 0)
		if repo.Spec.AttachedCodeSecurityConfiguration != nil {
			result = append(result, repo.Spec.AttachedCodeSecurityConfiguration.Name)
		}
		return result
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &githubv1alpha1.Repository{}, "spec.rulesetPresets.name", func(rawObj client.Object) []string {
		repo := rawObj.(*githubv1alpha1.Repository)
		rulesets := make([]string, 0, len(repo.Spec.RulesetPresetList))
		for _, ruleset := range repo.Spec.RulesetPresetList {
			rulesets = append(rulesets, ruleset.Name)
		}
		return rulesets
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &githubv1alpha1.Repository{}, "spec.webhookPresets.name", func(rawObj client.Object) []string {
		repo := rawObj.(*githubv1alpha1.Repository)
		webhooks := make([]string, 0, len(repo.Spec.WebhookPresetList))
		for _, webhook := range repo.Spec.WebhookPresetList {
			webhooks = append(webhooks, webhook.Name)
		}
		return webhooks
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &githubv1alpha1.Repository{}, "spec.webhookIgnorePresets.name", func(rawObj client.Object) []string {
		repo := rawObj.(*githubv1alpha1.Repository)
		webhhokIgnores := make([]string, 0, len(repo.Spec.WebhookIgnorePresetsList))
		for _, preset := range repo.Spec.WebhookIgnorePresetsList {
			webhhokIgnores = append(webhhokIgnores, preset.Name)
		}
		return webhhokIgnores
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &githubv1alpha1.Repository{}, "spec.autolinksPresets.name", func(rawObj client.Object) []string {
		repo := rawObj.(*githubv1alpha1.Repository)
		autolinksPresets := make([]string, 0, len(repo.Spec.AutolinksPresetList))
		for _, preset := range repo.Spec.AutolinksPresetList {
			autolinksPresets = append(autolinksPresets, preset.Name)
		}
		return autolinksPresets
	}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&githubv1alpha1.Repository{}).
		Watches( // changes of webhook presets should trigger reconciliation of repos using it
			&githubv1alpha1.WebhookPreset{},
			handler.EnqueueRequestsFromMapFunc(r.findRepositoriesForWebhookPreset),
		).
		Watches( // changes of webhook ignore presets should trigger reconciliation of repos using it
			&githubv1alpha1.WebhookIgnorePreset{},
			handler.EnqueueRequestsFromMapFunc(r.findRepositoriesForWebhookIgnorePreset),
		).
		Watches( // changes of ruleset presets should trigger reconciliation of repos using it
			&githubv1alpha1.RulesetPreset{},
			handler.EnqueueRequestsFromMapFunc(r.findRepositoriesForRulesetPreset),
		).
		Watches( // changes of autolinks presets should trigger reconciliation of repos using it
			&githubv1alpha1.AutolinksPreset{},
			handler.EnqueueRequestsFromMapFunc(r.findRepositoriesForAutolinksPreset),
		).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		WithOptions(controller.Options{
			UsePriorityQueue:        ptr.To[bool](true),
			MaxConcurrentReconciles: 20,
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter(
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](
					1*time.Second,    // base delay
					1000*time.Second, // max delay, ~17min
				),
				ratelimit.NewControllerRuntimeRateLimiter[reconcile.Request](r.GithubRateLimiter),
			),
		}).
		Named("repository").
		Complete(r)
}

func (r *RepositoryCtl) findRepositoriesForWebhookPreset(ctx context.Context, obj client.Object) []reconcile.Request {
	webhookPreset := obj.(*githubv1alpha1.WebhookPreset)

	var repoList githubv1alpha1.RepositoryList
	if err := r.ReconcilerFactory.K8sClient.List(ctx, &repoList, client.MatchingFields{"spec.webhookPresets.name": webhookPreset.Name}); err != nil {
		logf.FromContext(ctx).Error(err, "failed to list repositories for webhook preset", "webhookPreset", webhookPreset.Name)
		return nil
	}

	requests := make([]reconcile.Request, 0, len(repoList.Items))
	for _, repo := range repoList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      repo.Name,
				Namespace: repo.Namespace,
			},
		})
	}

	return requests
}

func (r *RepositoryCtl) findRepositoriesForWebhookIgnorePreset(ctx context.Context, obj client.Object) []reconcile.Request {
	webhookIgnorePreset := obj.(*githubv1alpha1.WebhookIgnorePreset)

	var repoList githubv1alpha1.RepositoryList
	if err := r.ReconcilerFactory.K8sClient.List(ctx, &repoList, client.MatchingFields{"spec.webhookIgnorePresets.name": webhookIgnorePreset.Name}); err != nil {
		logf.FromContext(ctx).Error(err, "failed to list repositories for webhook ignore preset", "webhookIgnorePreset", webhookIgnorePreset.Name)
		return nil
	}

	requests := make([]reconcile.Request, 0, len(repoList.Items))
	for _, repo := range repoList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      repo.Name,
				Namespace: repo.Namespace,
			},
		})
	}

	return requests
}

func (r *RepositoryCtl) findRepositoriesForRulesetPreset(ctx context.Context, obj client.Object) []reconcile.Request {
	rulesetPreset := obj.(*githubv1alpha1.RulesetPreset)

	var repoList githubv1alpha1.RepositoryList
	if err := r.ReconcilerFactory.K8sClient.List(ctx, &repoList, client.MatchingFields{"spec.rulesetPresets.name": rulesetPreset.Name}); err != nil {
		logf.FromContext(ctx).Error(err, "failed to list repositories for webhook preset", "rulesetPreset", rulesetPreset.Name)
		return nil
	}

	requests := make([]reconcile.Request, 0, len(repoList.Items))
	for _, repo := range repoList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      repo.Name,
				Namespace: repo.Namespace,
			},
		})
	}

	return requests
}
func (r *RepositoryCtl) findRepositoriesForAutolinksPreset(ctx context.Context, obj client.Object) []reconcile.Request {
	autolinksPreset := obj.(*githubv1alpha1.AutolinksPreset)

	var repoList githubv1alpha1.RepositoryList
	if err := r.ReconcilerFactory.K8sClient.List(ctx, &repoList, client.MatchingFields{"spec.autolinksPresets.name": autolinksPreset.Name}); err != nil {
		logf.FromContext(ctx).Error(err, "failed to list repositories for autolinks preset", "autolinksPreset", autolinksPreset.Name)
		return nil
	}

	requests := make([]reconcile.Request, 0, len(repoList.Items))
	for _, repo := range repoList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      repo.Name,
				Namespace: repo.Namespace,
			},
		})
	}

	return requests
}
