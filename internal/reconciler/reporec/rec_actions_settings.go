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

	// Access Level is not supported for public and free repos
	if org.HasEnterpriseFeatures() && r.Kubernetes.Resource.Spec.Visibility != githubv1alpha1.VisibilityPublic {
		if err := r.reconcileActionsAccessLevelForExternalWorkflows(ctx); err != nil {
			return err
		}
	} else {
		log.V(1).Info("Skipping reconciliation of actions access policies on GitHub as repository is either public or belongs to an organization on the free plan")
	}

	log.V(1).Info("Successfully reconciled repository GitHub actions configurations on GitHub")
	return nil
}

func (r *GitHubRepoReconciler) reconcileActionsAccessLevelForExternalWorkflows(ctx context.Context) error {
	log := logPkg.FromContext(ctx)
	log.V(1).Info("Reconciling repository GitHub actions access policies on GitHub")

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

	log.V(1).Info("Successfully reconciled GitHub actions access policies on GitHub")
	return nil
}
