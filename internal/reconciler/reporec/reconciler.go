package reporec

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	ac "github.com/Interhyp/git-hubby/api/v1alpha1/applyconfiguration/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/conditions"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/google/go-github/v86/github"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

type GitHubRepoIdentifier struct {
	Owner string
	Name  string
	// id is set once the repository is created/fetched from GitHub as part of reconcileRepository(ctx context.Context)
	ID *int64
}

func (g *GitHubRepoIdentifier) GetID() int64 {
	if g == nil || g.ID == nil {
		return 0
	}
	return *g.ID
}

type GitHubRepoReconciler struct {
	GitHub       reconciler.GitHub[GitHubRepoIdentifier]
	Kubernetes   reconciler.Kubernetes[*githubv1alpha1.Repository]
	FinalizeMode reconciler.RepositoryFinalizerMode
}

func (r *GitHubRepoReconciler) K8s() reconciler.Kubernetes[*githubv1alpha1.Repository] {
	return r.Kubernetes
}

func (r *GitHubRepoReconciler) GetAdditionalLogFields() []any {
	return []any{
		"github.organization", r.GitHub.Resource.Owner,
		"github.repository", r.GitHub.Resource.Name,
	}
}

func (r *GitHubRepoReconciler) GetAdditionalLabels() labels.Set {
	return labels.Set{
		"git-hubby.interhyp.de/organization": r.Kubernetes.Resource.Spec.OrganizationRef.Name,
	}
}

func (r *GitHubRepoReconciler) FinalizerName() string {
	return "repository.github.interhyp.de/finalizer"
}

func (r *GitHubRepoReconciler) RequiredReconciliations(_ context.Context) []reconciler.ParallelReconciliationGroup {
	return []reconciler.ParallelReconciliationGroup{
		{
			{ // must run in own group before all others because it creates the repo if it doesn't exist
				Function:  r.reconcileRepository,
				Condition: conditions.TypeBaseSettingsSynced,
			},
		},
		{
			{
				Function:  r.reconcileCustomProperties,
				Condition: conditions.TypeCustomPropertiesValuesSynced,
			},
			{
				Function:  r.reconcileWebhooks,
				Condition: conditions.TypeWebhooksSynced,
			},
			{
				Function:  r.reconcileRuleSets,
				Condition: conditions.TypeRulesetsSynced,
			},
			{
				Function:  r.reconcileActionsSettings,
				Condition: conditions.TypeActionsConfigurationSynced,
			},
			{
				Function:  r.reconcileTopics,
				Condition: conditions.TypeTopicsSynced,
			},
			{
				Function:  r.reconcileAutolinks,
				Condition: conditions.TypeAutolinksSynced,
			},
			{
				Function:  r.reconcileDeployKeys,
				Condition: conditions.TypeDeployKeysSynced,
			},
		},
	}
}

func (r *GitHubRepoReconciler) ReconcileDeletion(ctx context.Context) error {
	log := logPkg.FromContext(ctx)

	switch r.FinalizeMode {
	case "", reconciler.Ignore:
		log.V(1).Info("Finalize mode is set to 'ignore' or not set, skipping deletion")
		return nil
	case reconciler.Delete:
		log.V(1).Info("Finalize mode is set to 'delete', start deletion")
		return r.deleteRepository(ctx)
	case reconciler.Archive:
		log.V(1).Info("Finalize mode is set to 'archive', start archive")
		return r.archiveRepository(ctx)
	default:
		return fmt.Errorf("invalid finalize mode: %s", r.FinalizeMode)
	}
}

func (r *GitHubRepoReconciler) deleteRepository(ctx context.Context) error {
	log := logPkg.FromContext(ctx)

	log.V(1).Info("Deleting repository")
	if err := r.GitHub.Client.DeleteRepository(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name); err != nil {
		var ghErr *github.ErrorResponse
		if errors.As(err, &ghErr) && ghErr.Response != nil && ghErr.Response.StatusCode == http.StatusNotFound {
			log.V(1).Info("Repository already deleted or does not exist, skipping")
			return nil
		}
		log.Error(err, "failed to delete repository on GitHub")
		return err
	}
	log.V(1).Info("Repository deleted successfully")
	return nil
}

func (r *GitHubRepoReconciler) archiveRepository(ctx context.Context) error {
	log := logPkg.FromContext(ctx)

	ghRepo, err := r.GitHub.Client.GetRepository(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name)
	if err != nil {
		log.Error(err, "failed to get repository from GitHub")
		return err
	}

	if ghRepo.GetArchived() {
		log.V(1).Info("Repository is already archived")
		return nil
	}

	log.V(1).Info("Archiving repository")
	_, err = r.GitHub.Client.EditRepository(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, &github.Repository{
		Archived: new(true),
	})
	if err != nil {
		log.Error(err, "failed to archive repository on GitHub")
		return err
	}
	log.V(1).Info("Repository archived successfully")
	return nil
}

func (r *GitHubRepoReconciler) BuildMetadataApplyConfig(lbls map[string]string, annotations map[string]string, finalizers []string) runtime.ApplyConfiguration {
	cfg := ac.Repository(r.Kubernetes.Resource.Name, r.Kubernetes.Resource.Namespace)
	if lbls != nil {
		cfg.WithLabels(lbls)
	}
	if annotations != nil {
		cfg.WithAnnotations(annotations)
	}
	if finalizers != nil {
		cfg.WithFinalizers(finalizers...)
	}
	return cfg
}

func (r *GitHubRepoReconciler) BuildStatusApplyConfig() runtime.ApplyConfiguration {
	status := ac.RepositoryStatus().
		WithConditions(reconciler.ConditionsToApplyConfigs(*r.Kubernetes.Resource.GetConditions())...)
	if id := r.Kubernetes.Resource.Status.ID; id != nil {
		status.WithID(*id)
	}
	if webhooks := r.Kubernetes.Resource.Status.Webhooks; webhooks != nil {
		whAC := make(map[string]ac.WebhookStatusApplyConfiguration, len(webhooks))
		for k, v := range webhooks {
			whAC[k] = *ac.WebhookStatus().WithSecretHash(v.SecretHash)
		}
		status.WithWebhooks(whAC)
	}
	if gens := r.Kubernetes.Resource.Status.ObservedSubResourceGenerations; gens != nil {
		status.WithObservedSubResourceGenerations(gens)
	}
	return ac.Repository(r.Kubernetes.Resource.Name, r.Kubernetes.Resource.Namespace).WithStatus(status)
}
