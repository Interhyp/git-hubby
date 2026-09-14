package teamrec

import (
	"context"
	"slices"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

type role struct {
	Name string
	ID   int64
}

func (t *GitHubTeamReconciler) reconcileTeamRoleAssignments(ctx context.Context) error {
	log := logPkg.FromContext(ctx)
	log.V(1).Info("Reconciling team role assignments on GitHub")

	for _, githubOrg := range t.Team.Organizations.Current {
		log = log.WithValues("organization", githubOrg.Resource)
		orgRoles, err := githubOrg.Client.GetAllOrgRoles(ctx, githubOrg.Resource)
		if err != nil {
			log.Error(err, "failed to get organization roles from GitHub")
			return err
		}
		orgRolesToId := make(map[string]int64)
		assignedRoles := make([]role, 0)
		for _, r := range orgRoles {
			if r != nil && r.Name != nil {
				assignedTeams, err := githubOrg.Client.GetAllTeamsAssignedToOrgRole(ctx, githubOrg.Resource, r.GetName())
				if err != nil {
					log.Error(err, "failed to get role assignments for team on GitHub")
					return err
				}
				if slices.Contains(assignedTeams, t.Team.GetSlug()) {
					assignedRoles = append(assignedRoles, role{Name: r.GetName(), ID: r.GetID()})
				}
				orgRolesToId[r.GetName()] = r.GetID()
			}
		}

		expectedRoles := t.getRoles(orgRolesToId)
		if cmp.Equal(assignedRoles, expectedRoles, cmpopts.EquateEmpty()) {
			log.V(1).Info("Team role assignments already match desired state, skipping")
			continue
		}

		for _, expectedRole := range expectedRoles {
			if !slices.Contains(assignedRoles, expectedRole) {
				err = githubOrg.Client.AddOrgRoleAssignmentForTeam(ctx, githubOrg.Resource, t.Team.GetSlug(), expectedRole.ID)
				if err != nil {
					log.Error(err, "failed to add role assignment for team on GitHub")
					return err
				}
			}
		}

		for _, assignedRole := range assignedRoles {
			if !slices.Contains(expectedRoles, assignedRole) {
				err = githubOrg.Client.RemoveOrgRoleAssignmentForTeam(ctx, githubOrg.Resource, t.Team.GetSlug(), assignedRole.ID)
				if err != nil {
					log.Error(err, "failed to remove role assignment for team on GitHub")
					return err
				}
			}
		}
	}

	log.V(1).Info("Team role assignments reconciled successfully")
	return nil
}

func (t *GitHubTeamReconciler) getRoles(orgRoleNamesToIDs map[string]int64) []role {
	wantedRoleNames := make([]string, 0)
	if t.Kubernetes.Resource.Spec.OrganizationRoles != nil { // empty is valid, thus compare to nil, not length
		wantedRoleNames = t.Kubernetes.Resource.Spec.OrganizationRoles
	}

	result := make([]role, 0)
	for _, r := range wantedRoleNames {
		id, ok := orgRoleNamesToIDs[r]
		if ok {
			result = append(result, role{Name: r, ID: id})
		}
	}

	return result
}
