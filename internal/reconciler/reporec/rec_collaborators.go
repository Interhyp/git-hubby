package reporec

import (
	"context"
	"fmt"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

const defaultRepositoryCollaboratorPermission = "pull"

func (r *GitHubRepoReconciler) reconcileCollaborators(ctx context.Context) error {
	log := logPkg.FromContext(ctx)

	if r.Kubernetes.Resource.Spec.Collaborators == nil {
		log.V(1).Info("Collaborators not specified in repository spec, skipping collaborator reconciliation")
		return nil
	}

	log.V(1).Info("Reconciling repository collaborators on GitHub")

	memberSuffix := r.MemberSuffix
	if memberSuffix == "" {
		var org v1alpha1.Organization
		if err := r.Kubernetes.Client.Get(ctx, client.ObjectKey{
			Name:      r.Kubernetes.Resource.Spec.OrganizationRef.Name,
			Namespace: r.Kubernetes.Resource.Namespace,
		}, &org); err != nil {
			log.Error(err, "unable to fetch Organization referenced in repository spec", "organization", r.Kubernetes.Resource.Spec.OrganizationRef.Name)
			return fmt.Errorf("failed to get organization %s: %w", r.Kubernetes.Resource.Spec.OrganizationRef.Name, err)
		}
		memberSuffix = org.Spec.MemberSuffix
	}

	existingCollaborators, err := r.GitHub.Client.GetAllRepositoryCollaborators(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name)
	if err != nil {
		log.Error(err, "failed to get existing repository collaborators")
		return fmt.Errorf("failed to get existing repository collaborators: %w", err)
	}

	collaboratorsToDelete := make(map[string]string)
	existingCollaboratorsByName := make(map[string]string)
	for _, collaborator := range existingCollaborators {
		if collaborator == nil || collaborator.Login == nil {
			continue
		}
		existingCollaboratorsByName[*collaborator.Login] = collaborator.GetRoleName()
		collaboratorsToDelete[*collaborator.Login] = *collaborator.Login
	}

	ghMembers, err := r.GitHub.Client.ListMembers(ctx, r.GitHub.Resource.Owner)
	if err != nil {
		log.Error(err, "failed to get members from GitHub")
		return err
	}
	orgMembersByName := make(map[string]string)
	for _, ghMember := range ghMembers {
		if ghMember != nil && ghMember.Login != nil {
			orgMembersByName[*ghMember.Login] = *ghMember.Login
		}
	}

	for _, collaboratorRef := range r.Kubernetes.Resource.Spec.Collaborators {
		collaboratorRef.Username += memberSuffix
		log := log.WithValues("collaborator", collaboratorRef.Username)
		log.V(1).Info("Processing collaborator", "permission", collaboratorRef.Permission)

		desiredPermission := repositoryCollaboratorPermission(collaboratorRef)
		currentPermission, alreadyCollaborator := existingCollaboratorsByName[collaboratorRef.Username]

		if !alreadyCollaborator {
			if _, found := orgMembersByName[collaboratorRef.Username]; !found {
				log.Info("WARNING: Collaborator not found as a member of the organization on GitHub", "username", collaboratorRef.Username)
				continue
			}
		}

		if currentPermission != desiredPermission {
			if err := r.GitHub.Client.AddRepositoryCollaborator(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, collaboratorRef.Username, desiredPermission); err != nil {
				log.Error(err, "failed to add or update repository collaborator", "username", collaboratorRef.Username, "permission", desiredPermission)
				return err
			}
		}
		delete(collaboratorsToDelete, collaboratorRef.Username)
	}

	for _, username := range collaboratorsToDelete {
		if err := r.GitHub.Client.RemoveRepositoryCollaborator(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, username); err != nil {
			log.Error(err, "failed to remove repository collaborator", "username", username)
			return err
		}
	}

	log.V(1).Info("Successfully reconciled repository collaborators on GitHub")
	return nil
}

func repositoryCollaboratorPermission(collaborator v1alpha1.RepositoryCollaboratorPermission) string {
	if collaborator.Permission == "" {
		return defaultRepositoryCollaboratorPermission
	}
	return collaborator.Permission
}
