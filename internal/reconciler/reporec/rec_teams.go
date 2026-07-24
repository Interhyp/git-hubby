package reporec

import (
	"context"
	"errors"
	"fmt"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

const defaultRepositoryTeamPermission = "pull"

func (r *GitHubRepoReconciler) reconcileTeams(ctx context.Context) error {
	log := logPkg.FromContext(ctx)

	if r.Kubernetes.Resource.Spec.Teams == nil {
		log.V(1).Info("Teams not specified in repository spec, skipping team reconciliation")
		return nil
	}

	log.V(1).Info("Reconciling repository teams on GitHub")

	existingTeams, err := r.GitHub.Client.GetAllRepositoryTeams(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name)
	if err != nil {
		log.Error(err, "failed to list repository teams from GitHub")
		return err
	}

	existingPermissionsBySlug := make(map[string]string)
	teamsToDelete := make(map[string]string)
	for _, team := range existingTeams {
		if team == nil || team.Slug == nil {
			continue
		}
		slug := team.GetSlug()
		permission := team.GetPermission()
		existingPermissionsBySlug[slug] = permission
		teamsToDelete[slug] = slug
	}

	var errs []error
	for _, desiredTeam := range r.Kubernetes.Resource.Spec.Teams {
		var team v1alpha1.Team
		if err := r.Kubernetes.Client.Get(ctx, client.ObjectKey{
			Name:      desiredTeam.TeamRef.Name,
			Namespace: r.Kubernetes.Resource.Namespace,
		}, &team); err != nil {
			log.Error(err, "unable to fetch Team referenced in repository spec", "teamRef", desiredTeam.TeamRef.Name)
			errs = append(errs, fmt.Errorf("failed to get team %s: %w", desiredTeam.TeamRef.Name, err))
			continue
		}

		if !teamReferencesOrganization(team, r.Kubernetes.Resource.Spec.OrganizationRef) {
			log.Info("WARNING: Team referenced in repository spec does not reference the repository's organization", "teamRef", desiredTeam.TeamRef.Name, "organization", r.Kubernetes.Resource.Spec.OrganizationRef.Name)
			continue
		}

		if team.Status.Slug == nil || *team.Status.Slug == "" {
			log.Info("WARNING: Team referenced in repository spec has not been synced to GitHub yet", "teamRef", desiredTeam.TeamRef.Name)
			continue
		}
		slug := *team.Status.Slug

		desiredPermission := repositoryTeamPermission(desiredTeam)
		if currentPermission, exists := existingPermissionsBySlug[slug]; exists && currentPermission == desiredPermission {
			delete(teamsToDelete, slug)
			continue
		}

		if err := r.GitHub.Client.AddRepositoryTeam(ctx, r.GitHub.Resource.Owner, slug, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, desiredPermission); err != nil {
			log.Error(err, "failed to add or update team permission on repository", "teamSlug", slug, "permission", desiredPermission)
			return err
		}
		delete(teamsToDelete, slug)
	}

	for _, slug := range teamsToDelete {
		if err := r.GitHub.Client.RemoveRepositoryTeam(ctx, r.GitHub.Resource.Owner, slug, r.GitHub.Resource.Owner, r.GitHub.Resource.Name); err != nil {
			log.Error(err, "failed to remove team from repository", "teamSlug", slug)
			return err
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	log.V(1).Info("Successfully reconciled repository teams on GitHub")
	return nil
}

// teamReferencesOrganization reports whether the given Team references the same Organization CRD as orgRef.
func teamReferencesOrganization(team v1alpha1.Team, orgRef v1alpha1.OrganizationRef) bool {
	for _, ref := range team.Spec.OrganizationRefs {
		if ref.Name == orgRef.Name {
			return true
		}
	}
	return false
}

func repositoryTeamPermission(team v1alpha1.RepositoryTeamPermission) string {
	if team.Permission == "" {
		return defaultRepositoryTeamPermission
	}
	return team.Permission
}
