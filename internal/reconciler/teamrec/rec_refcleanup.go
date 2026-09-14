package teamrec

import (
	"context"
	"fmt"
	"slices"

	"github.com/Interhyp/git-hubby/internal/reconciler"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

func (t *GitHubTeamReconciler) reconcileRemovedOrgRefs(ctx context.Context) error {
	log := logPkg.FromContext(ctx)
	log.V(1).Info("Removing team from previous organizations if no longer referenced")
	for _, previousOrg := range t.Team.Organizations.Previous {
		log = log.WithValues("previousOrganization", previousOrg.Resource)
		if !slices.ContainsFunc(t.Team.Organizations.Current, func(a reconciler.GitHub[string]) bool {
			return a.Resource == previousOrg.Resource
		}) {
			log.V(1).Info("Previous organization is no longer referenced, deleting team from it")
			err := previousOrg.Client.DeleteTeamBySlug(ctx, previousOrg.Resource, t.Team.GetSlug())
			if err != nil {
				log.Error(err, fmt.Sprintf("failed to delete team from organization %s on GitHub after it no longer references it", previousOrg.Resource))
				return err
			}
			log.V(1).Info("Successfully deleted team from previous organization")
		}
	}
	log.V(1).Info("Updating previousOrganizationRefs status with current organization refs")
	t.Kubernetes.Resource.Status.PreviousOrganizationRefs = t.Kubernetes.Resource.Spec.OrganizationRefs
	log.V(1).Info("Successfully updated previousOrganizationRefs status with current organization refs")
	log.V(1).Info("Successfully removed team from previous organizations")
	return nil
}
