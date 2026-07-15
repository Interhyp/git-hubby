package reporec

import (
	"context"
	"fmt"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/mapper"
	"github.com/google/go-github/v89/github"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *GitHubRepoReconciler) reconcileAutolinks(ctx context.Context) error {
	log := logPkg.FromContext(ctx)
	log.V(1).Info("Reconciling repository autolinks on GitHub")

	autolinks, err := r.GitHub.Client.ListAllAutolinks(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name)
	if err != nil {
		log.Error(err, "failed to list current autolinks from GitHub")
		return err
	}

	existingAutolinks := make(map[string]*github.Autolink)
	autolinksToRemove := make(map[string]*github.Autolink)
	for _, autolink := range autolinks {
		log = log.WithValues(
			"autolinkID", autolink.GetID(),
		)
		if autolink.KeyPrefix == nil {
			log.Info("WARNING: Skipping autolink with missing KeyPrefix")
			continue
		}
		if autolink.URLTemplate == nil {
			log.Info("WARNING: Skipping autolink with missing URLTemplate")
			continue
		}
		if autolink.IsAlphanumeric == nil {
			log.Info("WARNING: Skipping autolink with missing IsAlphanumeric")
			continue
		}
		h := mapper.HashAutolink(*autolink.KeyPrefix, *autolink.URLTemplate, *autolink.IsAlphanumeric)
		existingAutolinks[h] = autolink
		autolinksToRemove[h] = autolink
	}

	autolinksToAdd := make(map[string]*githubv1alpha1.Autolink)
	for _, autolinkRef := range r.Kubernetes.Resource.Spec.AutolinksPresetList {
		var autolinksPreset githubv1alpha1.AutolinksPreset
		if err := r.Kubernetes.Client.Get(ctx, client.ObjectKey{
			Name:      autolinkRef.Name,
			Namespace: r.Kubernetes.Resource.Namespace,
		}, &autolinksPreset); err != nil {
			log.Error(err, "unable to get ruleset preset")
			return fmt.Errorf("failed to get autolinks preset %s: %w", autolinkRef.Name, err)
		}
		for _, autolink := range autolinksPreset.Spec.AutolinkList {
			h := mapper.HashAutolink(autolink.KeyPrefix, autolink.URLTemplate, autolink.IsAlphanumeric)
			_, exists := existingAutolinks[h]
			if exists {
				log.V(1).Info("Autolink exists")
				delete(autolinksToRemove, h)
				continue
			}
			autolinksToAdd[h] = &autolink
		}
	}

	if err := r.cleanupUnusedAutolinks(ctx, autolinksToRemove); err != nil {
		log.Error(err, "failed to clean up unused autolinks")
		return err
	}

	if err := r.createMissingAutolinks(ctx, autolinksToAdd); err != nil {
		log.Error(err, "failed to create missing autolinks")
		return err
	}

	log.V(1).Info("Successfully reconciled repository autolinks on GitHub")
	return nil
}

func (r *GitHubRepoReconciler) cleanupUnusedAutolinks(ctx context.Context, autolinksToRemove map[string]*github.Autolink) error {
	log := logPkg.FromContext(ctx).WithValues(
		"function", "cleanupUnusedAutolinks",
	)

	for hash, autolink := range autolinksToRemove {
		if autolink == nil || autolink.ID == nil {
			log.Info("WARNING: Skipping autolink with missing ID or nil autolink")
			continue
		}
		log := log.WithValues("autolinkID", *autolink.ID, "hash", hash)
		log.V(1).Info("Deleting unused autolink")
		if err := r.GitHub.Client.DeleteAutolink(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, *autolink.ID); err != nil {
			log.Error(err, "Failed to delete unused autolink")
			return err
		}
		log.V(1).Info("Successfully removed unused autolink")
	}
	return nil
}

func (r *GitHubRepoReconciler) createMissingAutolinks(ctx context.Context, autolinksToAdd map[string]*githubv1alpha1.Autolink) error {
	log := logPkg.FromContext(ctx).WithValues(
		"function", "createMissingAutolinks",
	)

	for _, kubernetesAutolink := range autolinksToAdd {
		if kubernetesAutolink == nil {
			log.Info("WARNING: Skipping nil autolink")
			continue
		}
		log := log.WithValues("autolink", kubernetesAutolink.KeyPrefix)

		log.V(1).Info("Creating missing autolink")
		autolink := mapper.KubernetesAutolinkToGitHubAutolink(*kubernetesAutolink)
		if err := r.GitHub.Client.CreateAutolink(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, autolink); err != nil {
			log.Error(err, "failed to create missing autolink for kubernetesAutolink")
			return err
		}
		log.V(1).Info("Successfully created missing autolink")
	}
	return nil
}
