package reporec

import (
	"context"
	"errors"
	"net/http"
	"time"

	orgac "github.com/Interhyp/git-hubby/api/v1alpha1/applyconfiguration/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/mapper"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/google/go-github/v89/github"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

// applyRepository creates or applies a repository on GitHub.
func (r *GitHubRepoReconciler) reconcileRepository(ctx context.Context) error {
	log := logPkg.FromContext(ctx)
	log.V(1).Info("Reconciling repository settings sets on GitHub")

	ghRepo, err := r.getRepo(ctx)
	if err != nil {
		return err
	}
	if ghRepo == nil {
		return errors.New("unexpected nil repository after ensuring existence")
	}
	log.V(1).Info("Existence of Repository confirmed on GitHub, checking for required updates")

	if mapper.RepoStaysArchived(r.Kubernetes.Resource, *ghRepo) {
		log.V(1).Info("Repository stays archived, preventing further reconciliations")
		return &RepoArchivedError{
			RepositoryName:  r.GitHub.Resource.Name,
			RepositoryOwner: r.GitHub.Resource.Owner,
		}
	}
	if !mapper.RepoDiffers(r.Kubernetes.Resource, *ghRepo) {
		log.V(1).Info("Repository already matches desired state, skipping update")
		return nil
	}

	log.V(1).Info("Repository differs from desired state, updating it")
	ghRepo, err = r.updateRepo(ctx)
	if err != nil {
		return err
	}
	if ghRepo.GetArchived() {
		log.V(1).Info("Repository was archived, preventing further reconciliations")
		return &RepoArchivedError{
			RepositoryName:  r.GitHub.Resource.Name,
			RepositoryOwner: r.GitHub.Resource.Owner,
		}
	}
	log.V(1).Info("Repository settings reconciled successfully")
	return nil
}

func (r *GitHubRepoReconciler) getRepo(ctx context.Context) (*github.Repository, error) {
	log := logPkg.FromContext(ctx)
	ghRepo, err := r.GitHub.Client.GetRepository(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name)
	if err != nil {
		var ghErr *github.ErrorResponse
		if errors.As(err, &ghErr) && ghErr.Response.StatusCode == http.StatusNotFound {
			log.V(1).Info("Repository does not exist, creating it")
			newRepo := mapper.RepoToGithubRepo(r.Kubernetes.Resource)
			newRepo.AutoInit = new(true)
			ghRepo, err = r.GitHub.Client.CreateRepository(ctx, r.GitHub.Resource.Owner, newRepo)
			if err != nil {
				log.Error(err, "failed to create repository on GitHub")
				return nil, err
			}
			log.V(1).Info("Repository created successfully")
			// Trigger parent Organization reconciliation
			if err = r.triggerOrganizationReconciliation(ctx); err != nil {
				log.Error(err, "failed to trigger parent organization reconciliation")
				// Don't fail the repo reconciliation, just log the error
			}
		} else {
			log.Error(err, "failed to get repository from GitHub")
			return nil, err
		}
	}
	err = r.updateID(ctx, ghRepo)
	if err != nil {
		log.Error(err, "failed to update repository status in Kubernetes")
		return nil, err
	}
	return ghRepo, nil
}

// triggerOrganizationReconciliation forces a reconciliation of the parent Organization
// by adding a timestamp annotation that triggers the Organization controller's AnnotationChangedPredicate.
// Uses Server-Side Apply to only claim ownership of the trigger annotation,
// preventing conflicts with other controllers (e.g. Argo CD) that may manage the Organization resource.
func (r *GitHubRepoReconciler) triggerOrganizationReconciliation(ctx context.Context) error {
	if r.Kubernetes.Resource.Spec.OrganizationRef.Name == "" {
		return nil // No parent organization reference
	}

	log := logPkg.FromContext(ctx)
	orgName := r.Kubernetes.Resource.Spec.OrganizationRef.Name

	applyConfig := orgac.Organization(orgName, r.Kubernetes.Resource.Namespace).
		WithAnnotations(map[string]string{
			"git-hubby.interhyp.de/reconcile-trigger": time.Now().Format(time.RFC3339Nano),
		})

	log.V(1).Info("Triggering parent organization reconciliation", "organization", orgName)
	return r.Kubernetes.Client.Apply(ctx, applyConfig, client.ForceOwnership, reconciler.FieldOwner)
}

func (r *GitHubRepoReconciler) updateRepo(ctx context.Context) (*github.Repository, error) {
	log := logPkg.FromContext(ctx)
	ghRepo, err := r.GitHub.Client.EditRepository(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, mapper.RepoToGithubRepo(r.Kubernetes.Resource))
	if err != nil {
		log.Error(err, "failed to update repository on GitHub")
		return ghRepo, err
	}
	err = r.updateID(ctx, ghRepo)
	if err != nil {
		log.Error(err, "failed to update repository status in Kubernetes")
		return ghRepo, err
	}
	return ghRepo, err
}

func (r *GitHubRepoReconciler) updateID(_ context.Context, ghRepo *github.Repository) error {
	if ghRepo == nil || ghRepo.ID == nil {
		return errors.New("unable to update repository ID with nil repository or nil ID")
	}
	r.GitHub.Resource.ID = ghRepo.ID
	r.Kubernetes.Resource.Status.ID = ghRepo.ID
	return nil
}
