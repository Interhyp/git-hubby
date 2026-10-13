package reporec

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/mapper"
	"github.com/Interhyp/git-hubby/internal/reconciler/common/merge"
	"github.com/google/go-github/v92/github"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

// DesiredAutolink is an autolink that should exist on GitHub, together with the source it was defined in.
type DesiredAutolink struct {
	// Source describes where the autolink was defined, e.g. "autolinksPreset/<name>[<idx>]" or "inline[<idx>]".
	Source string
	// Spec is a copy of the autolink definition.
	Spec v1alpha1.Autolink
}

// AutolinkConflictError is returned when multiple sources define an autolink with the same key prefix but different content.
type AutolinkConflictError struct {
	KeyPrefix string
	Sources   []string
}

func (e *AutolinkConflictError) Error() string {
	return fmt.Sprintf("conflicting autolink definitions for keyPrefix %q in sources [%s]", e.KeyPrefix, strings.Join(e.Sources, ", "))
}

// AutolinksPresetSource returns the source description of an autolink defined by an AutolinksPreset.
func AutolinksPresetSource(name string, idx int) string {
	return fmt.Sprintf("autolinksPreset/%s[%d]", name, idx)
}

func (r *GitHubRepoReconciler) reconcileAutolinks(ctx context.Context) error {
	log := logPkg.FromContext(ctx)
	log.V(1).Info("Reconciling repository autolinks on GitHub")

	// Collect the desired autolinks first, so that conflicts are detected before any GitHub API call is made.
	desiredAutolinks, err := collectDesiredAutolinks(ctx, r.Kubernetes.Client, r.Kubernetes.Resource.Namespace,
		r.Kubernetes.Resource.Spec.AutolinksPresetList, r.Kubernetes.Resource.Spec.Autolinks)
	if err != nil {
		return err
	}

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

	autolinksToAdd := make(map[string]*v1alpha1.Autolink)
	for _, desired := range desiredAutolinks {
		autolink := desired.Spec
		h := mapper.HashAutolink(autolink.KeyPrefix, autolink.URLTemplate, autolink.IsAlphanumeric)
		if _, exists := existingAutolinks[h]; exists {
			log.V(1).Info("Autolink exists", "source", desired.Source)
			delete(autolinksToRemove, h)
			continue
		}
		autolinksToAdd[h] = &autolink
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

// collectDesiredAutolinks returns the union of the autolinks referenced via presetRefs and the inline autolinks.
//
// Autolinks are identified by their key prefix (GitHub enforces unique key prefixes per repository):
//   - identical definitions: a warning is logged and the autolink is returned only once.
//   - differing definitions: an AutolinkConflictError is returned for every conflicting key prefix (joined via errors.Join).
func collectDesiredAutolinks(ctx context.Context, k8sClient client.Client, namespace string,
	presetRefs []corev1.LocalObjectReference, inline []v1alpha1.Autolink) ([]DesiredAutolink, error) {
	log := logPkg.FromContext(ctx)

	candidates := make([]DesiredAutolink, 0, len(inline))
	for _, ref := range presetRefs {
		var preset v1alpha1.AutolinksPreset
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: namespace}, &preset); err != nil {
			log.Error(err, "unable to get autolinks preset", "autolinksPreset", ref.Name)
			return nil, fmt.Errorf("failed to get autolinks preset %s: %w", ref.Name, err)
		}
		for i, autolink := range preset.Spec.AutolinkList {
			candidates = append(candidates, DesiredAutolink{Source: AutolinksPresetSource(ref.Name, i), Spec: autolink})
		}
	}
	for i, autolink := range inline {
		candidates = append(candidates, DesiredAutolink{Source: merge.InlineSource(i), Spec: autolink})
	}

	result, conflicts := merge.Dedupe(candidates,
		func(d DesiredAutolink) string { return d.Spec.KeyPrefix },
		func(a, b DesiredAutolink) bool { return equality.Semantic.DeepEqual(a.Spec, b.Spec) },
		func(d DesiredAutolink) string { return d.Source },
		func(kept, dup DesiredAutolink) {
			log.Info("WARNING: duplicate autolink definition with identical content, applying once",
				"keyPrefix", dup.Spec.KeyPrefix, "sources", []string{kept.Source, dup.Source})
		})
	if len(conflicts) > 0 {
		errs := make([]error, 0, len(conflicts))
		for _, c := range conflicts {
			conflictErr := &AutolinkConflictError{KeyPrefix: c.Key, Sources: c.Sources}
			log.Error(conflictErr, "conflicting autolink definitions", "keyPrefix", c.Key, "sources", c.Sources)
			errs = append(errs, conflictErr)
		}
		return nil, errors.Join(errs...)
	}
	return result, nil
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

func (r *GitHubRepoReconciler) createMissingAutolinks(ctx context.Context, autolinksToAdd map[string]*v1alpha1.Autolink) error {
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
