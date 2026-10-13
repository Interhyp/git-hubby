package reconciler

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

// InlineSource returns the source description of an inline entry (ruleset, webhook, autolink, ...) at the given index.
func InlineSource(idx int) string {
	return fmt.Sprintf("inline[%d]", idx)
}

// ---------------------------------------------------------------------------------------------------------------------
// Autolinks
// ---------------------------------------------------------------------------------------------------------------------

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

// CollectDesiredAutolinks returns the union of the autolinks referenced via presetRefs and the inline autolinks.
//
// Autolinks are identified by their key prefix (GitHub enforces unique key prefixes per repository):
//   - identical definitions: a warning is logged and the autolink is returned only once.
//   - differing definitions: an AutolinkConflictError is returned for every conflicting key prefix (joined via errors.Join).
func CollectDesiredAutolinks(ctx context.Context, k8sClient client.Client, namespace string,
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
		candidates = append(candidates, DesiredAutolink{Source: InlineSource(i), Spec: autolink})
	}

	result, conflicts := dedupe(candidates,
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
			conflictErr := &AutolinkConflictError{KeyPrefix: c.key, Sources: c.sources}
			log.Error(conflictErr, "conflicting autolink definitions", "keyPrefix", c.key, "sources", c.sources)
			errs = append(errs, conflictErr)
		}
		return nil, errors.Join(errs...)
	}
	return result, nil
}

// ---------------------------------------------------------------------------------------------------------------------
// Webhooks
// ---------------------------------------------------------------------------------------------------------------------

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

// DedupeDesiredWebhooks de-duplicates webhooks by their Hash. The Hash must already be set and the payload URL
// templated. It must be called before secrets are loaded so that plaintext secrets loaded from Kubernetes Secrets
// are never compared or logged.
//
//   - identical definitions: a warning is logged and the webhook is returned only once.
//   - differing definitions: a WebhookConflictError is returned for every conflicting webhook (joined via errors.Join).
func DedupeDesiredWebhooks(ctx context.Context, candidates []DesiredWebhook) ([]DesiredWebhook, error) {
	log := logPkg.FromContext(ctx)

	urls := make(map[string]string, len(candidates))
	for _, c := range candidates {
		urls[c.Hash] = c.Spec.PayloadURL
	}

	result, conflicts := dedupe(candidates,
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
			conflictErr := &WebhookConflictError{URL: urls[c.key], Hash: c.key, Sources: c.sources}
			log.Error(conflictErr, "conflicting webhook definitions", "payloadUrl", conflictErr.URL, "sources", c.sources)
			errs = append(errs, conflictErr)
		}
		return nil, errors.Join(errs...)
	}
	return result, nil
}

// ---------------------------------------------------------------------------------------------------------------------
// Webhook ignores
// ---------------------------------------------------------------------------------------------------------------------

// CollectWebhookIgnoreRegexes returns the compiled union of the ignore regexes referenced via presetRefs and the
// inline ignore rules. Entries without a regex are skipped, duplicate regexes are compiled only once.
func CollectWebhookIgnoreRegexes(ctx context.Context, k8sClient client.Client, namespace string,
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
		entries = append(entries, entry{source: InlineSource(i), spec: spec})
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

// ---------------------------------------------------------------------------------------------------------------------
// Generic helpers
// ---------------------------------------------------------------------------------------------------------------------

type dedupeConflict struct {
	key     string
	sources []string
}

// dedupe keeps the first candidate per key. Later candidates with equal content are dropped (onDuplicate is called),
// later candidates with different content result in a conflict for that key. Conflicts are returned in the order of
// their first occurrence and list all sources defining the key.
func dedupe[T any](candidates []T, keyFn func(T) string, equalFn func(a, b T) bool, sourceFn func(T) string,
	onDuplicate func(kept, dup T)) ([]T, []dedupeConflict) {
	type info struct {
		keptIdx  int
		sources  []string
		conflict bool
	}
	result := make([]T, 0, len(candidates))
	byKey := make(map[string]*info, len(candidates))
	conflictOrder := make([]string, 0)

	for _, c := range candidates {
		key := keyFn(c)
		i, seen := byKey[key]
		if !seen {
			byKey[key] = &info{keptIdx: len(result), sources: []string{sourceFn(c)}}
			result = append(result, c)
			continue
		}
		kept := result[i.keptIdx]
		i.sources = append(i.sources, sourceFn(c))
		if equalFn(kept, c) {
			onDuplicate(kept, c)
			continue
		}
		if !i.conflict {
			i.conflict = true
			conflictOrder = append(conflictOrder, key)
		}
	}

	conflicts := make([]dedupeConflict, 0, len(conflictOrder))
	for _, key := range conflictOrder {
		conflicts = append(conflicts, dedupeConflict{key: key, sources: byKey[key].sources})
	}
	return result, conflicts
}
