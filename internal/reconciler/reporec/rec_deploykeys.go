package reporec

import (
	"context"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/mapper"
	"github.com/Interhyp/git-hubby/internal/utils"
	"github.com/google/go-github/v86/github"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *GitHubRepoReconciler) reconcileDeployKeys(ctx context.Context) error {
	log := logPkg.FromContext(ctx)
	log.V(1).Info("Reconciling repository deploy keys on GitHub")

	deployKeys, err := r.GitHub.Client.ListAllDeployKeys(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name)
	if err != nil {
		log.Error(err, "failed to list deploy keys from GitHub")
		return err
	}

	existingDeployKeys := make(map[string]*github.Key)
	deployKeysToRemove := make(map[string]*github.Key)
	for _, key := range deployKeys {
		log = log.WithValues(
			"deployKeyID", key.GetID(),
		)
		if key.ID == nil {
			log.Info("WARNING: Skipping deploy key with missing ID")
			continue
		}
		if key.Key == nil {
			log.Info("WARNING: Skipping deploy key with missing key")
			continue
		}
		if key.Title == nil {
			log.Info("WARNING: Skipping deploy key with missing title")
			continue
		}
		if key.ReadOnly == nil {
			log.Info("WARNING: Skipping deploy key with missing readonly")
			continue
		}

		h := mapper.HashDeployKey(*key.Key, *key.Title, *key.ReadOnly)
		existingDeployKeys[h] = key
		deployKeysToRemove[h] = key
	}

	deployKeysToAdd := make(map[string]*v1alpha1.DeployKey)

	for _, key := range r.Kubernetes.Resource.Spec.DeployKeyList {
		h := mapper.HashDeployKey(key.Key, key.Title, utils.WithDefault(key.ReadOnly, true))
		_, exists := existingDeployKeys[h]
		if exists {
			log.V(1).Info("Deploy key exists")
			delete(deployKeysToRemove, h)
			continue
		}
		deployKeysToAdd[h] = &key
	}

	if err := r.cleanupUnusedDeployKeys(ctx, deployKeysToRemove); err != nil {
		log.Error(err, "failed to clean up unused deploy keys")
		return err
	}

	if err := r.createMissingDeployKeys(ctx, deployKeysToAdd); err != nil {
		log.Error(err, "failed to add missing deploy keys")
		return err
	}

	log.V(1).Info("Successfully reconciled repository deploy keys on GitHub")
	return nil
}

func (r *GitHubRepoReconciler) cleanupUnusedDeployKeys(ctx context.Context, deployKeysToRemove map[string]*github.Key) error {
	log := logPkg.FromContext(ctx).WithValues(
		"function", "cleanupUnusedDeployKeys",
	)

	for hash, key := range deployKeysToRemove {
		if key == nil || key.ID == nil {
			log.Info("WARNING: Skipping deploy key with missing ID or nil deploy key", "hash", hash)
			continue
		}
		log := log.WithValues("deployKeyID", key.GetID())
		log.V(1).Info("Deleting deploy key")
		if err := r.GitHub.Client.DeleteDeployKey(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, key.GetID()); err != nil {
			log.Error(err, "Failed to delete unused deploy keys")
			return err
		}
		log.V(1).Info("Successfully deleted deploy key")
	}
	return nil
}

func (r *GitHubRepoReconciler) createMissingDeployKeys(ctx context.Context, deployKeysToAdd map[string]*v1alpha1.DeployKey) error {
	log := logPkg.FromContext(ctx).WithValues(
		"function", "createMissingDeployKeys",
	)

	for _, preset := range deployKeysToAdd {
		if preset == nil {
			log.Info("WARNING: Skipping nil deploy key")
			continue
		}
		log := log.WithValues("deployKeyTitle", preset.Title)

		log.V(1).Info("Creating deploy key")
		deployKey := mapper.DeployKeyPresetToGitHubDeployKey(*preset)
		if err := r.GitHub.Client.CreateDeployKey(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, deployKey); err != nil {
			log.Error(err, "Failed to create missing deploy key")
			return err
		}
		log.V(1).Info("Successfully created deploy key")
	}
	return nil
}
