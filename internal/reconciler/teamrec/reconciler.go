package teamrec

import (
	"context"
	"errors"
	"net/http"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	ac "github.com/Interhyp/git-hubby/api/v1alpha1/applyconfiguration/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/conditions"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/google/go-github/v89/github"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

// GitHubTeamReconciler reconciles a Team object
type GitHubTeamReconciler struct {
	Team       reconciler.GitHubTeamIdentifier
	Kubernetes reconciler.Kubernetes[*githubv1alpha1.Team]
}

func (t *GitHubTeamReconciler) K8s() reconciler.Kubernetes[*githubv1alpha1.Team] {
	return t.Kubernetes
}

func (t *GitHubTeamReconciler) GetAdditionalLogFields() []any {
	return []any{
		"github.team", t.Team.Name,
	}
}
func (t *GitHubTeamReconciler) GetAdditionalLabels() labels.Set {
	return labels.Set{}
}

func (t *GitHubTeamReconciler) FinalizerName() string {
	return "team.github.interhyp.de/finalizer"
}

func (t *GitHubTeamReconciler) RequiredReconciliations() []reconciler.ParallelReconciliationGroup {
	return []reconciler.ParallelReconciliationGroup{
		{
			{ // must run in first group because it creates the team in the referenced orgs if it doesn't exist there
				Function:  t.reconcileTeam,
				Condition: conditions.TypeBaseSettingsSynced,
			},
		},
		{ // these require the team to exist in the referenced orgs
			{ // can run in first group as it only removes the team from orgs that are no longer referenced
				Function:  t.reconcileRemovedOrgRefs,
				Condition: conditions.TypeOutdatedOrganizationRefsSynced,
			},
			{
				Function:  t.reconcileTeamMembers,
				Condition: conditions.TypeTeamMembersSynced,
			},
			{
				Function:  t.reconcileTeamRoleAssignments,
				Condition: conditions.TypeTeamRoleAssignmentsSynced,
			},
			{
				Function:  t.reconcileIDPGroup,
				Condition: conditions.TypeIDPTeamGroupSettingsSynced,
			},
		},
	}
}

func (t *GitHubTeamReconciler) ReconcileDeletion(ctx context.Context) error {
	log := logPkg.FromContext(ctx)
	for _, githubOrg := range t.Team.Organizations.Current {
		log = log.WithValues("organization", githubOrg.Resource)

		ghTeam, err := githubOrg.Client.GetTeamBySlug(ctx, githubOrg.Resource, t.Team.GetSlug())
		if err != nil {
			var ghErr *github.ErrorResponse
			log.Error(err, "failed to get team from GitHub")
			if errors.As(err, &ghErr) && ghErr.Response.StatusCode == http.StatusNotFound {
				log.V(1).Info("Error caused by team not found, assuming it is already deleted")
				continue
			}
			return err
		}

		if ghTeam == nil {
			log.V(1).Info("Team is already deleted")
			continue
		}

		log.V(1).Info("Deleting team")
		if err := githubOrg.Client.DeleteTeamBySlug(ctx, githubOrg.Resource, *ghTeam.Slug); err != nil {
			log.Error(err, "failed to delete team on GitHub")
			return err
		}
	}

	log.V(1).Info("Team deleted successfully")
	return nil
}

func (t *GitHubTeamReconciler) BuildMetadataApplyConfig(lbls map[string]string, annotations map[string]string, finalizers []string) runtime.ApplyConfiguration {
	cfg := ac.Team(t.Kubernetes.Resource.Name, t.Kubernetes.Resource.Namespace)
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

func (t *GitHubTeamReconciler) BuildStatusApplyConfig() runtime.ApplyConfiguration {
	status := ac.TeamStatus().
		WithConditions(reconciler.ConditionsToApplyConfigs(*t.Kubernetes.Resource.GetConditions())...)
	if slug := t.Kubernetes.Resource.Status.Slug; slug != nil {
		status.WithSlug(*slug)
	}
	if refs := t.Kubernetes.Resource.Status.PreviousOrganizationRefs; refs != nil {
		refACs := make([]*ac.OrganizationRefApplyConfiguration, len(refs))
		for i, ref := range refs {
			refACs[i] = ac.OrganizationRef().WithName(ref.Name)
		}
		status.WithPreviousOrganizationRefs(refACs...)
	}
	return ac.Team(t.Kubernetes.Resource.Name, t.Kubernetes.Resource.Namespace).WithStatus(status)
}
