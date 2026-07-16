package orgrec

import (
	"context"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	ac "github.com/Interhyp/git-hubby/api/v1alpha1/applyconfiguration/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/conditions"
	"github.com/Interhyp/git-hubby/internal/config"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

type FinalizationFailedError struct {
	message string
}

func (e FinalizationFailedError) Error() string {
	return e.message
}

func OrganizationStillHasReposError() error {
	return FinalizationFailedError{message: "organization still has repositories, cannot delete it"}
}

func OrganizationStillHasTeamsError() error {
	return FinalizationFailedError{message: "organization still has teams, cannot delete it"}
}

type GitHubOrgReconciler struct {
	Kubernetes reconciler.Kubernetes[*githubv1alpha1.Organization]
	GitHub     reconciler.GitHub[string]
	Features   config.Features
}

func (o *GitHubOrgReconciler) GetAdditionalLogFields() []any {
	return []any{
		"github.organization", o.GitHub.Resource,
	}
}

func (o *GitHubOrgReconciler) GetAdditionalLabels() labels.Set {
	return labels.Set{
		"git-hubby.interhyp.de/organization": o.Kubernetes.Resource.GetLogin(),
	}
}

func (o *GitHubOrgReconciler) RequiredReconciliations() []reconciler.ParallelReconciliationGroup {
	return []reconciler.ParallelReconciliationGroup{
		{ // org reconciliations are all independent and can run in parallel as no orgs are created - they need to exist beforehand
			{Function: o.reconcileOrganization, Condition: conditions.TypeBaseSettingsSynced},
			{Function: o.reconcileCustomProperties, Condition: conditions.TypeCustomPropertyDefinitionsSynced},
			{Function: o.reconcileRulesetPresets, Condition: conditions.TypeRulesetsSynced},
			{Function: o.reconcileCodeSecurityConfigurations, Condition: conditions.TypeCodeSecurityConfigurationsSynced},
			{Function: o.reconcileActionsSettings, Condition: conditions.TypeActionsConfigurationSynced},
		},
	}
}

func (o *GitHubOrgReconciler) K8s() reconciler.Kubernetes[*githubv1alpha1.Organization] {
	return o.Kubernetes
}

func (o *GitHubOrgReconciler) FinalizerName() string {
	return "organization.github.interhyp.de/finalizer"
}

func (o *GitHubOrgReconciler) ReconcileDeletion(ctx context.Context) error {
	if err := o.checkForExistingRepos(ctx); err != nil {
		return err
	}
	if err := o.checkForExistingTeams(ctx); err != nil {
		return err
	}
	return nil
}

func (o *GitHubOrgReconciler) checkForExistingRepos(ctx context.Context) error {
	repos, err := o.getReferencingK8sRepos(ctx)
	if err != nil {
		return err
	}
	if len(repos.Items) > 0 {
		logPkg.FromContext(ctx).Info("WARNING: Organization has repositories, cannot delete", "repositories", len(repos.Items))
		return OrganizationStillHasReposError()
	}
	return nil
}

func (o *GitHubOrgReconciler) getReferencingK8sRepos(ctx context.Context) (githubv1alpha1.RepositoryList, error) {
	var repos githubv1alpha1.RepositoryList
	if err := o.Kubernetes.Client.List(ctx, &repos, client.InNamespace(o.Kubernetes.Resource.Namespace), client.MatchingFields{"spec.organizationRef.name": o.Kubernetes.Resource.Name}); err != nil {
		logPkg.FromContext(ctx).Error(err, "unable to list repositories for organization")
		return githubv1alpha1.RepositoryList{}, err
	}
	return repos, nil
}

func (o *GitHubOrgReconciler) checkForExistingTeams(ctx context.Context) error {
	log := logPkg.FromContext(ctx)
	var teams githubv1alpha1.TeamList
	if err := o.Kubernetes.Client.List(ctx, &teams, client.InNamespace(o.Kubernetes.Resource.Namespace), client.MatchingFields{"spec.organizationRefs.name": o.Kubernetes.Resource.Name}); err != nil {
		log.Error(err, "unable to list teams for organization")
		return err
	}
	if len(teams.Items) > 0 {
		log.Info("WARNING: Organization has teams, cannot delete", "teams", len(teams.Items))
		return OrganizationStillHasTeamsError()
	}
	return nil
}

func (o *GitHubOrgReconciler) BuildMetadataApplyConfig(lbls map[string]string, annotations map[string]string, finalizers []string) runtime.ApplyConfiguration {
	cfg := ac.Organization(o.Kubernetes.Resource.Name, o.Kubernetes.Resource.Namespace)
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

func (o *GitHubOrgReconciler) BuildStatusApplyConfig() runtime.ApplyConfiguration {
	status := ac.OrganizationStatus().
		WithConditions(reconciler.ConditionsToApplyConfigs(*o.Kubernetes.Resource.GetConditions())...)
	if gens := o.Kubernetes.Resource.Status.ObservedSubResourceGenerations; gens != nil {
		status.WithObservedSubResourceGenerations(gens)
	}
	return ac.Organization(o.Kubernetes.Resource.Name, o.Kubernetes.Resource.Namespace).WithStatus(status)
}
