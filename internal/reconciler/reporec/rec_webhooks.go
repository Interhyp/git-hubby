package reporec

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"text/template"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/mapper"
	"github.com/Interhyp/git-hubby/internal/reconciler/common/merge"
	"github.com/google/go-github/v93/github"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

// DesiredWebhook is a webhook that should exist on GitHub, together with the source it was defined in.
type DesiredWebhook struct {
	// Source describes where the webhook was defined, e.g. "webhookPreset/<name>" or "inline[<idx>]".
	Source string
	// Inline is true if the webhook was defined inline in the Repository spec.
	Inline bool
	// Spec is a deep copy of the webhook definition (payload URL templated, secret value loaded).
	// It must never be written back to a Kubernetes object.
	Spec v1alpha1.WebhookSpec
	// Hash is the identity of the webhook, see mapper.HashWebhookConfig.
	Hash string
}

// WebhookConflictError is returned when multiple sources define a webhook with the same identity
// (payload URL, content type and events) but different content.
type WebhookConflictError struct {
	URL     string
	Hash    string
	Sources []string
}

func (e *WebhookConflictError) Error() string {
	return fmt.Sprintf("conflicting webhook definitions for payload URL %q in sources [%s]", e.URL, strings.Join(e.Sources, ", "))
}

// WebhookPresetSource returns the source description of a webhook defined by a WebhookPreset.
func WebhookPresetSource(name string) string {
	return "webhookPreset/" + name
}

func (r *GitHubRepoReconciler) reconcileWebhooks(ctx context.Context) error {
	log := logPkg.FromContext(ctx,
		"function", "applyWebhookPresets",
	)
	log.V(1).Info("Reconciling repository webhooks on GitHub")

	// Collect ignore rules and desired webhooks first, so that misconfigurations and conflicts are detected before
	// any webhook is changed on GitHub.
	ignoreRegexes, err := collectWebhookIgnoreRegexes(ctx, r.Kubernetes.Client, r.Kubernetes.Resource.Namespace,
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
	desiredWebhooks := make(map[string]*DesiredWebhook, len(desired))
	// hooksToAdd will contain the webhooks that need to be created or updated
	hooksToAdd := make(map[string]*DesiredWebhook)

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
// Duplicates are resolved via dedupeDesiredWebhooks before secrets are loaded.
func (r *GitHubRepoReconciler) collectDesiredWebhooks(ctx context.Context) ([]DesiredWebhook, error) {
	log := logPkg.FromContext(ctx).WithValues(
		"function", "collectDesiredWebhooks",
	)
	repo := r.Kubernetes.Resource

	candidates := make([]DesiredWebhook, 0, len(repo.Spec.WebhookPresetList)+len(repo.Spec.Webhooks))
	for _, wp := range repo.Spec.WebhookPresetList {
		webhookPreset := &v1alpha1.WebhookPreset{}
		if err := r.Kubernetes.Client.Get(ctx, client.ObjectKey{Name: wp.Name, Namespace: repo.Namespace}, webhookPreset); err != nil {
			log.Error(err, "unable to get webhook preset", "preset", wp.Name)
			return nil, fmt.Errorf("failed to get webhook preset %s: %w", wp.Name, err)
		}
		candidates = append(candidates, DesiredWebhook{
			Source: WebhookPresetSource(wp.Name),
			Spec:   *webhookPreset.Spec.DeepCopy(),
		})
	}
	for i := range repo.Spec.Webhooks {
		candidates = append(candidates, DesiredWebhook{
			Source: merge.InlineSource(i),
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

	desired, err := dedupeDesiredWebhooks(ctx, candidates)
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

// dedupeDesiredWebhooks de-duplicates webhooks by their Hash. The Hash must already be set and the payload URL
// templated. It must be called before secrets are loaded so that plaintext secrets loaded from Kubernetes Secrets
// are never compared or logged.
//
//   - identical definitions: a warning is logged and the webhook is returned only once.
//   - differing definitions: a WebhookConflictError is returned for every conflicting webhook (joined via errors.Join).
func dedupeDesiredWebhooks(ctx context.Context, candidates []DesiredWebhook) ([]DesiredWebhook, error) {
	log := logPkg.FromContext(ctx)

	urls := make(map[string]string, len(candidates))
	for _, c := range candidates {
		urls[c.Hash] = c.Spec.PayloadURL
	}

	result, conflicts := merge.Dedupe(candidates,
		func(d DesiredWebhook) string { return d.Hash },
		func(a, b DesiredWebhook) bool { return equality.Semantic.DeepEqual(a.Spec, b.Spec) },
		func(d DesiredWebhook) string { return d.Source },
		func(kept, dup DesiredWebhook) {
			log.Info("WARNING: duplicate webhook definition with identical content, applying once",
				"payloadUrl", dup.Spec.PayloadURL, "sources", []string{kept.Source, dup.Source})
		})
	if len(conflicts) > 0 {
		errs := make([]error, 0, len(conflicts))
		for _, c := range conflicts {
			conflictErr := &WebhookConflictError{URL: urls[c.Key], Hash: c.Key, Sources: c.Sources}
			log.Error(conflictErr, "conflicting webhook definitions", "payloadUrl", conflictErr.URL, "sources", c.Sources)
			errs = append(errs, conflictErr)
		}
		return nil, errors.Join(errs...)
	}
	return result, nil
}

// cleanupUnusedWebhooks removes webhooks that are no longer in the desired state
func (r *GitHubRepoReconciler) cleanupUnusedWebhooks(ctx context.Context, hooksToRemove map[string]*github.Hook) error {
	log := logPkg.FromContext(ctx).WithValues(
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

func (r *GitHubRepoReconciler) createMissingWebhooks(ctx context.Context, hooksToAdd map[string]*DesiredWebhook) error {
	log := logPkg.FromContext(ctx).WithValues(
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
func (r *GitHubRepoReconciler) templatePayloadURL(ctx context.Context, spec *v1alpha1.WebhookSpec) error {
	log := logPkg.FromContext(ctx).WithValues(
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

func (r *GitHubRepoReconciler) updateWebhooksStatus(ctx context.Context, allWebhooks map[string]*DesiredWebhook) error {
	log := logPkg.FromContext(ctx).WithValues(
		"function", "updateWebhooksStatus",
	)

	r.Kubernetes.Resource.Status.Webhooks = make(map[string]v1alpha1.WebhookStatus)
	for hash, desired := range allWebhooks {
		if desired == nil {
			log.Info("WARNING: Skipping nil webhook", "hash", hash)
			continue
		}
		r.Kubernetes.Resource.Status.Webhooks[hash] = v1alpha1.WebhookStatus{
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
func (r *GitHubRepoReconciler) loadWebhookSecret(ctx context.Context, desired *DesiredWebhook) error {
	log := logPkg.FromContext(ctx).WithValues(
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

// collectWebhookIgnoreRegexes returns the compiled union of the ignore regexes referenced via presetRefs and the
// inline ignore rules. Entries without a regex are skipped, duplicate regexes are compiled only once.
func collectWebhookIgnoreRegexes(ctx context.Context, k8sClient client.Client, namespace string,
	presetRefs []corev1.LocalObjectReference, inline []v1alpha1.WebhookIgnoreSpec) ([]*regexp.Regexp, error) {
	log := logPkg.FromContext(ctx)

	type entry struct {
		source string
		spec   v1alpha1.WebhookIgnoreSpec
	}
	entries := make([]entry, 0, len(presetRefs)+len(inline))
	for _, ref := range presetRefs {
		var preset v1alpha1.WebhookIgnorePreset
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: namespace}, &preset); err != nil {
			log.Error(err, "unable to get webhook ignore preset", "preset", ref.Name)
			return nil, fmt.Errorf("failed to get webhook ignore preset %s: %w", ref.Name, err)
		}
		entries = append(entries, entry{source: "webhookIgnorePreset/" + ref.Name, spec: preset.Spec})
	}
	for i, spec := range inline {
		entries = append(entries, entry{source: merge.InlineSource(i), spec: spec})
	}

	seen := make(map[string]struct{}, len(entries))
	result := make([]*regexp.Regexp, 0, len(entries))
	for _, e := range entries {
		if e.spec.IgnoreURLRegex == nil || *e.spec.IgnoreURLRegex == "" {
			log.V(1).Info("Skipping webhook ignore rule without regex", "source", e.source)
			continue
		}
		expr := *e.spec.IgnoreURLRegex
		if _, ok := seen[expr]; ok {
			continue
		}
		seen[expr] = struct{}{}
		re, err := regexp.Compile(expr)
		if err != nil {
			log.Error(err, "unable to compile webhook ignore regex", "source", e.source)
			return nil, fmt.Errorf("failed to compile webhook ignore regex of %s: %w", e.source, err)
		}
		result = append(result, re)
	}
	return result, nil
}
