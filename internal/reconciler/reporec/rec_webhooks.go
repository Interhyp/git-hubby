package reporec

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"text/template"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/mapper"
	"github.com/google/go-github/v86/github"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *GitHubRepoReconciler) reconcileWebhooks(ctx context.Context) error {
	log := logf.FromContext(ctx,
		"function", "applyWebhookPresets",
	)
	log.V(1).Info("Reconciling repository webhooks on GitHub")

	// List existing webhooks
	hooks, err := r.GitHub.Client.ListHooks(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, &github.ListOptions{})
	if err != nil {
		var ghErr *github.ErrorResponse
		log.Error(err, "Failed to list webhooks for repository")
		if errors.As(err, &ghErr) && ghErr.Response.StatusCode == http.StatusNotFound {
			return fmt.Errorf("repository %s/%s not found: %w", r.GitHub.Resource.Owner, r.GitHub.Resource.Name, err)
		}
		return fmt.Errorf("failed to list webhooks: %w", err)
	}

	hooks, err = r.filterIgnoredHooks(ctx, hooks)
	if err != nil {
		log.Error(err, "Failed to filter webhooks")
		return err
	}

	// existingHookMap is the total set of webhooks currently present in the repository
	existingHookMap := make(map[string]*github.Hook)
	// hooksToRemove includes webhooks that are present in the repository but not in the desired state
	hooksToRemove := make(map[string]*github.Hook)
	for _, hook := range hooks {
		log = log.WithValues(
			"hookID", hook.GetID(),
		)
		if hook.Config == nil || hook.ID == nil {
			log.Info("WARNING: Skipping webhook with missing config or ID")
			continue
		}
		if hook.Config.URL == nil {
			log.Info("WARNING: Skipping webhook with missing URL")
			continue
		}
		if hook.Config.ContentType == nil {
			log.Info("WARNING: Skipping webhook with missing ContentType")
			continue
		}
		h := mapper.HashWebhookConfig(*hook.Config.URL, *hook.Config.ContentType, hook.Events)
		existingHookMap[h] = hook
		hooksToRemove[h] = hook
	}

	// desiredWebhooks is the total set of webhooks we want to have based on the presets
	desiredWebhooks := make(map[string]*githubv1alpha1.WebhookPreset)
	// hooksToAdd will contain the webhooks that need to be created or updated
	hooksToAdd := make(map[string]*githubv1alpha1.WebhookPreset)

	for _, wp := range r.Kubernetes.Resource.Spec.WebhookPresetList {
		webhookPreset := &githubv1alpha1.WebhookPreset{}
		if err := r.Kubernetes.Client.Get(ctx, client.ObjectKey{Name: wp.Name, Namespace: r.Kubernetes.Resource.Namespace}, webhookPreset); err != nil {
			log.Error(err, "unable to get webhook preset", "preset", wp.Name)
			return fmt.Errorf("failed to get webhook preset %s: %w", wp.Name, err)
		}
		webhookPreset, err := r.templatePayloadURL(ctx, webhookPreset)
		if err != nil {
			log.Error(err, "failed to template webhook payload URL")
			return fmt.Errorf("failed to template webhook payload URL: %w", err)
		}
		h := mapper.HashWebhookConfig(webhookPreset.Spec.PayloadURL, webhookPreset.Spec.ContentType, webhookPreset.Spec.Events)

		if err := r.loadWebhookPresetSecret(ctx, wp, webhookPreset); err != nil {
			return err
		}
		desiredWebhooks[h] = webhookPreset
		wantSecretHash := webhookPreset.GetSecretValueHash()
		_, exists := existingHookMap[h]
		haveSecretHash := ""
		existingStatus, ok := r.Kubernetes.Resource.Status.Webhooks[h]
		if ok {
			haveSecretHash = existingStatus.SecretHash
		}

		if exists && wantSecretHash == haveSecretHash {
			log.V(1).Info("Webhook exists and secret is correct")
			delete(hooksToRemove, h)
			continue
		}
		hooksToAdd[h] = webhookPreset
	}

	if err := r.cleanupUnusedWebhooks(ctx, hooksToRemove); err != nil {
		log.Error(err, "failed to cleanup unused webhooks")
		return err
	}

	if err := r.createMissingWebhooks(ctx, hooksToAdd); err != nil {
		log.Error(err, "failed to create missing webhooks")
		return err
	}

	if err := r.updateWebhooksStatus(ctx, desiredWebhooks); err != nil {
		log.Error(err, "failed to update webhook status")
		return err
	}

	log.V(1).Info("Successfully reconciled repository webhooks on GitHub")
	return nil
}

// cleanupUnusedWebhooks removes webhooks that are no longer in the desired state
func (r *GitHubRepoReconciler) cleanupUnusedWebhooks(ctx context.Context, hooksToRemove map[string]*github.Hook) error {
	log := logf.FromContext(ctx).WithValues(
		"function", "cleanupUnusedWebhooks",
	)

	for hash, hook := range hooksToRemove {
		if hook == nil || hook.ID == nil {
			log.Info("WARNING: Skipping webhook with missing ID or nil hook")
			continue
		}
		log := log.WithValues("hookID", *hook.ID, "hash", hash)
		log.V(1).Info("Removing unused webhook")
		if err := r.GitHub.Client.DeleteHook(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, *hook.ID); err != nil {
			return fmt.Errorf("failed to delete unused webhook %d: %w", *hook.ID, err)
		}
		log.V(1).Info("Successfully removed unused webhook")
	}
	return nil
}

func (r *GitHubRepoReconciler) createMissingWebhooks(ctx context.Context, hooksToAdd map[string]*githubv1alpha1.WebhookPreset) error {
	log := logf.FromContext(ctx).WithValues(
		"function", "createMissingWebhooks",
	)

	for _, preset := range hooksToAdd {
		if preset == nil {
			log.Info("WARNING: Skipping nil webhook preset")
			continue
		}
		log := log.WithValues("preset", preset.Name)

		log.V(1).Info("Creating new webhook")

		hook := mapper.WebhookPresetToGithubHook(*preset)
		if _, err := r.GitHub.Client.CreateHook(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, hook); err != nil {
			log.Error(err, "failed to create webhook")
			return fmt.Errorf("failed to create webhook via GitHub API: %w", err)
		}
	}
	return nil
}

func (r *GitHubRepoReconciler) templatePayloadURL(ctx context.Context, preset *githubv1alpha1.WebhookPreset) (*githubv1alpha1.WebhookPreset, error) {
	log := logf.FromContext(ctx).WithValues(
		"function", "templatePayloadURL",
		"payloadUrl", preset.Spec.PayloadURL,
	)
	templateChecker := regexp.MustCompile(`.*\{\{.+}}.*`)
	if !templateChecker.MatchString(preset.Spec.PayloadURL) {
		return preset, nil
	}
	tmpl, err := template.New("payloadURL").Parse(preset.Spec.PayloadURL)
	if err != nil {
		return nil, err
	}
	repo, err := r.GitHub.Client.GetRepository(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name)
	if err != nil {
		log.Error(err, "failed to get repository for webhook creation")
		return nil, fmt.Errorf("failed to get repository for webhook creation: %w", err)
	}
	var buf strings.Builder

	err = tmpl.Execute(&buf, repo)
	if err != nil {
		log.Error(err, "failed to template webhook url")
		return nil, fmt.Errorf("failed to template webhook url: %w", err)
	}
	preset.Spec.PayloadURL = buf.String()
	log.V(1).Info("Successfully templated webhook payload URL", "payloadUrl", preset.Spec.PayloadURL)
	return preset, nil
}

func (r *GitHubRepoReconciler) updateWebhooksStatus(ctx context.Context, allWebhooks map[string]*githubv1alpha1.WebhookPreset) error {
	log := logf.FromContext(ctx).WithValues(
		"function", "updateWebhooksStatus",
	)

	r.Kubernetes.Resource.Status.Webhooks = make(map[string]githubv1alpha1.WebhookStatus)
	for hash, preset := range allWebhooks {
		if preset == nil {
			log.Info("WARNING: Skipping nil webhook preset", "hash", hash)
			continue
		}
		r.Kubernetes.Resource.Status.Webhooks[hash] = githubv1alpha1.WebhookStatus{
			SecretHash: preset.GetSecretValueHash(),
		}
	}

	return nil
}

func (r *GitHubRepoReconciler) filterIgnoredHooks(ctx context.Context, hooks []*github.Hook) ([]*github.Hook, error) {
	log := logf.FromContext(ctx).WithValues(
		"function", "filterIgnoredHooks",
	)

	if len(r.Kubernetes.Resource.Spec.WebhookIgnorePresetsList) == 0 {
		return hooks, nil
	}

	filteredHooks := make(map[string]*github.Hook)
	for _, hook := range hooks {
		h := mapper.HashWebhookConfig(*hook.Config.URL, *hook.Config.ContentType, hook.Events)
		filteredHooks[h] = hook
	}

	for _, ip := range r.Kubernetes.Resource.Spec.WebhookIgnorePresetsList {
		ignoreWebhookPresets := &githubv1alpha1.WebhookIgnorePreset{}
		if err := r.Kubernetes.Client.Get(ctx, client.ObjectKey{Name: ip.Name, Namespace: r.Kubernetes.Resource.Namespace}, ignoreWebhookPresets); err != nil {
			log.Error(err, "unable to get webhook ignore preset", "preset", ip.Name)
			return hooks, fmt.Errorf("failed to get webhook ignore preset %s: %w", ip.Name, err)
		}

		regex, err := regexp.Compile(*ignoreWebhookPresets.Spec.IgnoreURLRegex)
		if err != nil {
			log.Error(err, "unable to compile webhook ignore preset regex", "preset", ip.Name)
			return hooks, err
		}

		for _, hook := range filteredHooks {
			if regex.MatchString(*hook.Config.URL) {
				h := mapper.HashWebhookConfig(*hook.Config.URL, *hook.Config.ContentType, hook.Events)
				delete(filteredHooks, h)
			}
		}
	}

	var result []*github.Hook
	for _, hook := range filteredHooks {
		result = append(result, hook)
	}
	return result, nil
}

func (r *GitHubRepoReconciler) loadWebhookPresetSecret(ctx context.Context, wp corev1.LocalObjectReference, webhookPreset *githubv1alpha1.WebhookPreset) error {
	log := logf.FromContext(ctx).WithValues(
		"function", "loadWebhookPresetSecret",
	)

	if webhookPreset.Spec.Secret != nil {
		secretPreset := webhookPreset.Spec.Secret
		namespace := r.Kubernetes.Resource.Namespace
		if secretPreset.Namespace != nil {
			namespace = *secretPreset.Namespace
		}
		var secret corev1.Secret
		if err := r.Kubernetes.Client.Get(ctx, client.ObjectKey{Name: *secretPreset.Name, Namespace: namespace}, &secret); err != nil {
			log.Error(err, "unable to get secret", "secret", wp.Name)
			return fmt.Errorf("failed to get secret %s/%s: %w", namespace, *secretPreset.Name, err)
		}
		if tokenData, tokenExists := secret.Data[*secretPreset.Key]; tokenExists {
			token := string(tokenData)
			webhookPreset.SetSecretValue(token)
		} else {
			log.Info("WARNING: Unable to find key in secret", "secret", wp.Name, "key", *secretPreset.Key)
			return fmt.Errorf("failed to find key %s in secret %s/%s", *secretPreset.Key, namespace, *secretPreset.Name)
		}
	}
	return nil
}
