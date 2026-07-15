package teamrec

import (
	"context"
	"errors"
	"net/http"

	"github.com/Interhyp/git-hubby/internal/mapper"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/google/go-github/v89/github"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

func (t *GitHubTeamReconciler) reconcileTeam(ctx context.Context) error {
	log := logPkg.FromContext(ctx)
	log.V(1).Info("Reconciling team settings sets on GitHub")

	for _, githubOrg := range t.Team.Organizations.Current {
		err := t.reconcileTeamForOrg(ctx, githubOrg)
		if err != nil {
			return err
		}
	}
	log.V(1).Info("Team settings reconciled successfully")

	return nil
}

// reconcileTeamForOrg ensures that the team exists in the given GitHub organization and matches the desired state.
// If the method returns without an error, the team is guaranteed to exist in the organization and
// its slug is updated in both the Kubernetes status and the reconciler's Team struct for the next reconciliation runs.
func (t *GitHubTeamReconciler) reconcileTeamForOrg(ctx context.Context, ghOrg reconciler.GitHub[string]) error {
	log := logPkg.FromContext(ctx).WithValues("organization", ghOrg.Resource)
	var ghTeam *github.Team

	log.V(1).Info("Trying to find team in GitHub")
	ghTeam, err := t.findTeam(ctx, ghOrg)
	if err != nil {
		var ghErr *github.ErrorResponse
		if errors.As(err, &ghErr) && ghErr.Response.StatusCode == http.StatusNotFound {
			return t.createTeam(ctx, ghOrg)
		}
		log.Error(err, "failed to get team from GitHub")
		return err
	}
	if ghTeam == nil {
		// team not found by list
		return t.createTeam(ctx, ghOrg)
	}
	log.V(1).Info("Team found in GitHub")

	if !mapper.TeamDiffers(t.Kubernetes.Resource, ghTeam, ghOrg.Resource) {
		log.V(1).Info("Team already matches desired state, skipping update")
		return nil // nothing to update, continue with next org
	}

	log.V(1).Info("Team differs from desired state, updating it")
	_, err = t.updateTeam(ctx, ghOrg)
	if err != nil {
		log.Error(err, "failed to update team on GitHub")
		return err
	}

	log.V(1).Info("Team updated for organization successfully")
	return nil
}

func (t *GitHubTeamReconciler) updateTeam(ctx context.Context, ghOrg reconciler.GitHub[string]) (*github.Team, error) {
	ghTeam, err := ghOrg.Client.EditTeamBySlug(ctx, ghOrg.Resource, t.Team.GetSlug(), mapper.TeamToNewGitHubTeam(t.Kubernetes.Resource))
	if err != nil {
		return nil, err
	}
	err = t.updateSlugs(ctx, ghTeam)
	if err != nil {
		logPkg.FromContext(ctx).Error(err, "failed to update team slugs")
		return nil, err
	}
	return ghTeam, nil
}

func (t *GitHubTeamReconciler) findTeam(ctx context.Context, ghOrg reconciler.GitHub[string]) (*github.Team, error) {
	if t.Team.Slug != nil {
		return ghOrg.Client.GetTeamBySlug(ctx, ghOrg.Resource, t.Team.GetSlug())
	}
	teams, err := ghOrg.Client.GetAllTeamsForOrg(ctx, ghOrg.Resource)
	if err != nil {
		return nil, err
	}
	for _, fetchedTeam := range teams {
		if fetchedTeam.GetName() == t.Kubernetes.Resource.Spec.Name {
			err = t.updateSlugs(ctx, fetchedTeam)
			if err != nil {
				return nil, err
			}
			return fetchedTeam, nil
		}
	}
	return nil, nil // no match
}

func (t *GitHubTeamReconciler) createTeam(ctx context.Context, ghOrg reconciler.GitHub[string]) error {
	log := logPkg.FromContext(ctx)
	log.V(1).Info("Team not found in GitHub, creating it")

	ghTeam, err := ghOrg.Client.CreateTeam(ctx, ghOrg.Resource, mapper.TeamToNewGitHubTeam(t.Kubernetes.Resource))
	if err != nil {
		log.Error(err, "failed to create team on GitHub")
		return err
	}
	err = t.updateSlugs(ctx, ghTeam)
	if err != nil {
		logPkg.FromContext(ctx).Error(err, "failed to update team status.slug in Kubernetes")
		return err
	}
	log.V(1).Info("Team created successfully")
	return nil
}

func (t *GitHubTeamReconciler) updateSlugs(ctx context.Context, ghTeam *github.Team) error {
	log := logPkg.FromContext(ctx)
	log.V(1).Info("Updating team's status.slug")
	returnedSlug := ghTeam.Slug
	if returnedSlug == nil {
		return errors.New("missing slug in team returned from GitHub")
	}

	t.Team.Slug = ghTeam.Slug
	t.Kubernetes.Resource.Status.Slug = returnedSlug
	log.V(1).Info("Updated team's slug for reconciliation run")

	return nil
}
