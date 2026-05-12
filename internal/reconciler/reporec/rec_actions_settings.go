package reporec

import (
	"context"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/internal/utils"
	"github.com/google/go-github/v86/github"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *GitHubRepoReconciler) reconcileActionsSettings(ctx context.Context) error {
	log := logPkg.FromContext(ctx)
	log.V(1).Info("Reconciling repository GitHub actions configurations on GitHub")

	var org githubv1alpha1.Organization
	if err := r.Kubernetes.Client.Get(ctx, client.ObjectKey{Name: r.Kubernetes.Resource.Spec.OrganizationRef.Name, Namespace: r.Kubernetes.Resource.Namespace}, &org); err != nil {
		log.Error(err, "unable to fetch Organization for Repository", "organization", r.Kubernetes.Resource.Spec.OrganizationRef.Name)
		return err
	}

	if reconciler.IsActionsDisabledForOrgSpec(&org) {
		// If actions are disabled for the organization, we skip further reconciliation of action settings for repos
		return nil
	}

	err := r.reconcileActionsAccessLevelForExternalWorkflows(ctx)
	if err != nil {
		return err
	}

	log.V(1).Info("Successfully reconciled repository GitHub actions configurations on GitHub")
	return nil
}

func (r *GitHubRepoReconciler) reconcileActionsAccessLevelForExternalWorkflows(ctx context.Context) error {
	expectedActionsAccessLevel := utils.WithDefaultAsPtr(r.Kubernetes.Resource.Spec.AccessLevelForExternalWorkflows, "none")
	currentAccessLevel, err := r.GitHub.Client.GetAccessLevelForExternalWorkflowsForRepo(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name)
	if err != nil {
		return err
	}
	if currentAccessLevel.GetAccessLevel() != *expectedActionsAccessLevel {
		err = r.GitHub.Client.SetAccessLevelForExternalWorkflowsForRepo(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, github.RepositoryActionsAccessLevel{
			AccessLevel: expectedActionsAccessLevel,
		})
		if err != nil {
			return err
		}
	}
	return nil
}
