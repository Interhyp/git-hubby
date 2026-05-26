package teamrec

import (
	"context"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/google/go-github/v86/github"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

// reconcileIDPTeam creates or applies a IDPTeam on GitHub.
func (t *GitHubTeamReconciler) reconcileIDPGroup(ctx context.Context) error {
	log := logPkg.FromContext(ctx)
	log.V(1).Info("Reconciling idp team group settings sets on GitHub")
	if !t.Kubernetes.Resource.IsIDPTeam() {
		log.V(1).Info("Skipping IDP team group settings for non IDP team")
		return nil // nothing to do
	}
	for _, githubOrg := range t.Team.Organizations.Current {
		var org githubv1alpha1.Organization
		if err := t.Kubernetes.Client.Get(ctx, client.ObjectKey{
			Name:      githubOrg.Resource,
			Namespace: t.Kubernetes.Resource.Namespace,
		}, &org); err != nil {
			log.Error(err, "unable to fetch Organization for Team IDP group", "organization", githubOrg.Resource)
			continue
		}

		if org.GetPlan() != githubv1alpha1.PlanEnterprise {
			log.V(1).Info("Skipping IDP team group settings for organization because plan does not support it",
				"organization", githubOrg.Resource, "plan", org.GetPlan())
			continue
		}
		err := t.reconcileIDPGroupForOrg(ctx, githubOrg)
		if err != nil {
			return err
		}
	}

	log.V(1).Info("IDPTeam group settings reconciled successfully")

	return nil
}

func (t *GitHubTeamReconciler) reconcileIDPGroupForOrg(ctx context.Context, ghOrg reconciler.GitHub[string]) error {
	log := logPkg.FromContext(ctx).WithValues("organization", ghOrg.Resource)
	teamExtGroups, err := ghOrg.Client.GetExternalGroupsForTeamBySlug(ctx, ghOrg.Resource, t.Team.GetSlug())
	if err != nil {
		log.Error(err, "failed to get external groups for team from GitHub")
		return err
	}
	for _, teamExtGroup := range teamExtGroups {
		if teamExtGroup.GroupName != nil && *teamExtGroup.GroupName == *t.Kubernetes.Resource.Spec.IDPGroup {
			log.V(1).Info("External group already linked to team on GitHub")
			return nil
		}
	}

	availableExternalGroups, err := ghOrg.Client.GetExternalGroupNamesToIDForOrg(ctx, ghOrg.Resource)
	if err != nil {
		log.Error(err, "failed to get available external groups from GitHub")
		return err
	}
	groupID, ok := availableExternalGroups[*t.Kubernetes.Resource.Spec.IDPGroup]
	if !ok {
		log.Error(nil, "specified external group not found in available external groups from GitHub")
		return nil
	}
	if err := ghOrg.Client.AddExternalGroupToTeamBySlug(ctx, ghOrg.Resource, t.Team.GetSlug(), &github.ExternalGroup{
		GroupID: &groupID,
	}); err != nil {
		log.Error(err, "failed to add external group to team on GitHub")
		return err
	}

	return nil
}
