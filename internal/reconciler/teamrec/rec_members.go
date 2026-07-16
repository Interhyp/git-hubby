package teamrec

import (
	"context"
	"fmt"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

func (t *GitHubTeamReconciler) reconcileTeamMembers(ctx context.Context) error {
	log := logPkg.FromContext(ctx)
	log.V(1).Info("Reconciling team members sets on GitHub")
	if t.Kubernetes.Resource.IsIDPTeam() {
		return nil // IDP teams manage members via the identity provider
	}

	envMemberSuffix := t.MemberSuffix
	k8sOrgs := make(map[string]githubv1alpha1.Organization)
	if envMemberSuffix == "" {
		// need to fetch orgs to check for org specific memberSuffixes later
		for _, orgRef := range t.Kubernetes.Resource.Spec.OrganizationRefs {
			var org githubv1alpha1.Organization
			if err := t.Kubernetes.Client.Get(ctx, client.ObjectKey{Name: orgRef.Name, Namespace: t.Kubernetes.Resource.Namespace}, &org); err != nil {
				log.Error(err, "unable to fetch Organization for Team", "organization", orgRef)
				return err
			}
			// index by GitHub login for easier mapping
			k8sOrgs[org.GetLogin()] = org
		}
	}

	for _, githubOrg := range t.Team.Organizations.Current {
		log = log.WithValues("organization", githubOrg.Resource)

		existingMembers, err := githubOrg.Client.GetAllTeamMembers(ctx, githubOrg.Resource, t.Team.GetSlug())
		if err != nil {
			log.Error(err, "failed to get existing team members")
			return fmt.Errorf("failed to get existing team members: %w", err)
		}

		membersToDelete := make(map[string]string)
		existingMembersByName := make(map[string]string)
		for _, member := range existingMembers {
			if member != nil && member.Login != nil {
				existingMembersByName[*member.Login] = *member.Login
				membersToDelete[*member.Login] = *member.Login
			}
		}

		memberSuffix := envMemberSuffix
		if org, ok := k8sOrgs[githubOrg.Resource]; ok {
			memberSuffix = org.Spec.MemberSuffix
		}

		for _, memberRef := range t.Kubernetes.Resource.Spec.Members {
			memberRef += memberSuffix
			log := log.WithValues("member", memberRef)
			log.V(1).Info("Processing member")

			// Check if the member already exists
			if _, exists := existingMembersByName[memberRef]; exists {
				log.V(1).Info("Member already exists in the team, skipping addition")
				// Remove from membersToDelete as it already exists
				delete(membersToDelete, memberRef)
				continue
			} else {
				ghMembers, err := githubOrg.Client.ListMembers(ctx, githubOrg.Resource)
				if err != nil {
					log.Error(err, "failed to get members from GitHub")
					return err
				}

				found := false
				for _, ghMember := range ghMembers {
					if ghMember.Login != nil && *ghMember.Login == memberRef {
						found = true
						break
					}
				}

				if found {
					if err := githubOrg.Client.AddTeamMember(ctx, githubOrg.Resource, t.Team.GetSlug(), memberRef); err != nil {
						log.Error(err, "failed to add member to team", "member", memberRef)
						return err
					}
				} else {
					log.Info("WARNING: Member not found on GitHub", "member", memberRef)
				}
			}
		}

		for _, member := range membersToDelete {
			log.V(1).Info("Removing member from team", "member", member)
			if err := githubOrg.Client.RemoveTeamMember(ctx, githubOrg.Resource, t.Team.GetSlug(), member); err != nil {
				log.Error(err, "failed to remove member from team", "member", member)
				return err
			}
		}
	}

	log.V(1).Info("Successfully reconciled team member sets on GitHub")
	return nil
}
