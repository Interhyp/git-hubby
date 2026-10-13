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
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/google/go-github/v92/github"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *GitHubRepoReconciler) reconcileWebhooks(ctx context.Context) error {
	log := logf.FromContext(ctx,
		"function", "applyWebhookPresets",
	)
	log.V(1).Info("Reconciling repository webhooks on GitHub")

	// Collect ignore rules and desired webhooks first, so that misconfigurations and conflicts are detected before
	// any webhook is changed on GitHub.
	ignoreRegexes, err := reconciler.CollectWebhookIgnoreRegexes(ctx, r.Kubernetes.Client, r.Kubernetes.Resource.Namespace,
		r.Kubernetes.Resource.Spec.WebhookIgnorePresetsList, r.Kubernetes.Resource.Spec.WebhookIgnores)
	if err != nil {
		log.Error(err, "Failed to collect webhook ignore rules")
		return err
	}

	desired, err := r.collectDesiredWebhooks(ctx)
	if err != nil {
		return err
	}

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

	// existingHookMap is the total set of webhooks currently present in the repository
	existingHookMap := make(map[string]*github.Hook)
	// hooksToRemove includes webhooks that are present in the repository but not in the desired state
	hooksToRemove := make(map[string]*github.Hook)
	for _, hook := range hooks {
		log := log.WithValues(
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
		if isIgnoredHook(hook, ignoreRegexes) {
			log.V(1).Info("Ignoring webhook matching an ignore rule")
			continue
		}
		h := mapper.HashWebhookConfig(*hook.Config.URL, *hook.Config.ContentType, hook.Events)
		existingHookMap[h] = hook
		hooksToRemove[h] = hook
	}

	// desiredWebhooks is the total set of webhooks we want to have
	desiredWebhooks := make(map[string]*reconciler.DesiredWebhook, len(desired))
	// hooksToAdd will contain the webhooks that need to be created or updated
	hooksToAdd := make(map[string]*reconciler.DesiredWebhook)

	for i := range desired {
		d := &desired[i]
		h := d.Hash
		desiredWebhooks[h] = d
		wantSecretHash := d.Spec.GetSecretValueHash()
		_, exists := existingHookMap[h]
		haveSecretHash := ""
		existingStatus, ok := r.Kubernetes.Resource.Status.Webhooks[h]
		if ok {
			haveSecretHash = existingStatus.SecretHash
		}

		if exists && wantSecretHash == haveSecretHash {
			log.V(1).Info("Webhook exists and secret is correct", "source", d.Source)
			delete(hooksToRemove, h)
			continue
		}
		hooksToAdd[h] = d
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

// collectDesiredWebhooks returns the union of the webhooks referenced via WebhookPresetList and the inline webhooks.
// All specs are deep copies with templated payload URLs and loaded secrets; they are never written back to Kubernetes.
// Duplicates are resolved via reconciler.DedupeDesiredWebhooks before secrets are loaded.
func (r *GitHubRepoReconciler) collectDesiredWebhooks(ctx context.Context) ([]reconciler.DesiredWebhook, error) {
	log := logf.FromContext(ctx).WithValues(
		"function", "collectDesiredWebhooks",
	)
	repo := r.Kubernetes.Resource

	candidates := make([]reconciler.DesiredWebhook, 0, len(repo.Spec.WebhookPresetList)+len(repo.Spec.Webhooks))
	for _, wp := range repo.Spec.WebhookPresetList {
		webhookPreset := &githubv1alpha1.WebhookPreset{}
		if err := r.Kubernetes.Client.Get(ctx, client.ObjectKey{Name: wp.Name, Namespace: repo.Namespace}, webhookPreset); err != nil {
			log.Error(err, "unable to get webhook preset", "preset", wp.Name)
			return nil, fmt.Errorf("failed to get webhook preset %s: %w", wp.Name, err)
		}
		candidates = append(candidates, reconciler.DesiredWebhook{
			Source: reconciler.WebhookPresetSource(wp.Name),
			Spec:   *webhookPreset.Spec.DeepCopy(),
		})
	}
	for i := range repo.Spec.Webhooks {
		candidates = append(candidates, reconciler.DesiredWebhook{
			Source: reconciler.InlineSource(i),
			Inline: true,
			Spec:   *repo.Spec.Webhooks[i].DeepCopy(),
		})
	}

	for i := range candidates {
		c := &candidates[i]
		if err := r.templatePayloadURL(ctx, &c.Spec); err != nil {
			log.Error(err, "failed to template webhook payload URL", "source", c.Source)
			return nil, fmt.Errorf("failed to template webhook payload URL of %s: %w", c.Source, err)
		}
		c.Hash = mapper.HashWebhookConfig(c.Spec.PayloadURL, c.Spec.ContentType, c.Spec.Events)
	}

	desired, err := reconciler.DedupeDesiredWebhooks(ctx, candidates)
	if err != nil {
		return nil, err
	}

	for i := range desired {
		if err := r.loadWebhookSecret(ctx, &desired[i]); err != nil {
			return nil, err
		}
	}
	return desired, nil
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

func (r *GitHubRepoReconciler) createMissingWebhooks(ctx context.Context, hooksToAdd map[string]*reconciler.DesiredWebhook) error {
	log := logf.FromContext(ctx).WithValues(
		"function", "createMissingWebhooks",
	)

	for _, desired := range hooksToAdd {
		if desired == nil {
			log.Info("WARNING: Skipping nil webhook")
			continue
		}
		log := log.WithValues("source", desired.Source)

		log.V(1).Info("Creating new webhook")

		hook := mapper.WebhookSpecToGithubHook(desired.Spec)
		if _, err := r.GitHub.Client.CreateHook(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, hook); err != nil {
			log.Error(err, "failed to create webhook")
			return fmt.Errorf("failed to create webhook via GitHub API: %w", err)
		}
	}
	return nil
}

// templatePayloadURL renders Go template placeholders in the payload URL of the given spec against the GitHub
// repository. The spec is modified in place, so callers must pass a copy.
func (r *GitHubRepoReconciler) templatePayloadURL(ctx context.Context, spec *githubv1alpha1.WebhookSpec) error {
	log := logf.FromContext(ctx).WithValues(
		"function", "templatePayloadURL",
		"payloadUrl", spec.PayloadURL,
	)
	templateChecker := regexp.MustCompile(`.*\{\{.+}}.*`)
	if !templateChecker.MatchString(spec.PayloadURL) {
		return nil
	}
	tmpl, err := template.New("payloadURL").Parse(spec.PayloadURL)
	if err != nil {
		return err
	}
	repo, err := r.GitHub.Client.GetRepository(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name)
	if err != nil {
		log.Error(err, "failed to get repository for webhook creation")
		return fmt.Errorf("failed to get repository for webhook creation: %w", err)
	}
	var buf strings.Builder

	err = tmpl.Execute(&buf, repo)
	if err != nil {
		log.Error(err, "failed to template webhook url")
		return fmt.Errorf("failed to template webhook url: %w", err)
	}
	spec.PayloadURL = buf.String()
	log.V(1).Info("Successfully templated webhook payload URL", "payloadUrl", spec.PayloadURL)
	return nil
}

func (r *GitHubRepoReconciler) updateWebhooksStatus(ctx context.Context, allWebhooks map[string]*reconciler.DesiredWebhook) error {
	log := logf.FromContext(ctx).WithValues(
		"function", "updateWebhooksStatus",
	)

	r.Kubernetes.Resource.Status.Webhooks = make(map[string]githubv1alpha1.WebhookStatus)
	for hash, desired := range allWebhooks {
		if desired == nil {
			log.Info("WARNING: Skipping nil webhook", "hash", hash)
			continue
		}
		r.Kubernetes.Resource.Status.Webhooks[hash] = githubv1alpha1.WebhookStatus{
			SecretHash: desired.Spec.GetSecretValueHash(),
		}
	}

	return nil
}

// isIgnoredHook returns true if the URL of the given hook matches one of the ignore regexes.
func isIgnoredHook(hook *github.Hook, ignoreRegexes []*regexp.Regexp) bool {
	if hook == nil || hook.Config == nil || hook.Config.URL == nil {
		return false
	}
	for _, re := range ignoreRegexes {
		if re.MatchString(*hook.Config.URL) {
			return true
		}
	}
	return false
}

// loadWebhookSecret loads the secret value referenced by desired.Spec.Secret into desired.Spec.SecretValue.
// Inline webhooks may only reference secrets in the namespace of the Repository.
func (r *GitHubRepoReconciler) loadWebhookSecret(ctx context.Context, desired *reconciler.DesiredWebhook) error {
	log := logf.FromContext(ctx).WithValues(
		"function", "loadWebhookSecret",
		"source", desired.Source,
	)

	secretRef := desired.Spec.Secret
	if secretRef == nil {
		return nil
	}
	if secretRef.Name == nil || secretRef.Key == nil {
		return fmt.Errorf("webhook secret reference of %s must define name and key", desired.Source)
	}
	repoNamespace := r.Kubernetes.Resource.Namespace
	namespace := repoNamespace
	if secretRef.Namespace != nil && *secretRef.Namespace != "" {
		namespace = *secretRef.Namespace
	}
	if desired.Inline && namespace != repoNamespace {
		err := fmt.Errorf("inline webhook %s references secret %s/%s outside of the repository namespace %s",
			desired.Source, namespace, *secretRef.Name, repoNamespace)
		log.Error(err, "rejecting cross-namespace secret reference of inline webhook")
		return err
	}

	var secret corev1.Secret
	if err := r.Kubernetes.Client.Get(ctx, client.ObjectKey{Name: *secretRef.Name, Namespace: namespace}, &secret); err != nil {
		log.Error(err, "unable to get secret", "secret", *secretRef.Name)
		return fmt.Errorf("failed to get secret %s/%s: %w", namespace, *secretRef.Name, err)
	}
	tokenData, tokenExists := secret.Data[*secretRef.Key]
	if !tokenExists {
		log.Info("WARNING: Unable to find key in secret", "secret", *secretRef.Name, "key", *secretRef.Key)
		return fmt.Errorf("failed to find key %s in secret %s/%s", *secretRef.Key, namespace, *secretRef.Name)
	}
	desired.Spec.SetSecretValue(string(tokenData))
	return nil
}
