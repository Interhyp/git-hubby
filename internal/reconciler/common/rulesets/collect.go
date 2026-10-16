// Package rulesets implements the ruleset reconciliation shared by the organization and repository reconcilers:
// collecting the desired rulesets from presets and inline definitions (Collect) and applying them to a ruleset
// owner on GitHub (Sync).
package rulesets

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler/common/merge"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

// Desired is a ruleset that should exist on GitHub, together with the source it was defined in.
type Desired struct {
	// Source describes where the ruleset was defined, e.g. "rulesetPreset/<name>" or "inline[<idx>]".
	Source string
	// Spec is a deep copy of the resolved ruleset definition (extends chain merged). It may be modified freely by
	// the caller. If Unresolved is set, Spec is the unresolved definition and only Spec.Name is meaningful.
	Spec v1alpha1.RulesetSpec
	// Unresolved is set if the extends chain of the ruleset could not be resolved. Such a ruleset must be neither
	// created, updated nor deleted.
	Unresolved *UnresolvedError
	// raw is the definition before resolving the extends chain.
	raw v1alpha1.RulesetSpec
}

// ConflictError is returned when multiple sources define a ruleset with the same name but different content.
type ConflictError struct {
	Name    string
	Sources []string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflicting ruleset definitions for ruleset %q in sources [%s]", e.Name, strings.Join(e.Sources, ", "))
}

// PresetSource returns the source description of a ruleset defined by a RulesetPreset.
func PresetSource(name string) string {
	return "rulesetPreset/" + name
}

// Collect returns the union of the rulesets referenced via presetRefs and the inline rulesets, with their extends
// chains resolved. All returned specs are deep copies and can be mutated by the caller without affecting cached objects.
//
// A referenced preset that does not exist fails the whole collection. A ruleset whose extends chain cannot be
// resolved is returned with Unresolved set, so only that ruleset is skipped (see Sync).
//
// Rulesets sharing the same name are handled as follows:
//   - identical definitions: a warning is logged and the ruleset is returned only once. Definitions are compared
//     after resolving their extends chains; if one of them could not be resolved, the unresolved definitions are
//     compared and the returned ruleset is marked as unresolved.
//   - differing definitions: a ConflictError is returned for every conflicting name (joined via errors.Join).
func Collect(ctx context.Context, k8sClient client.Reader, namespace string,
	presetRefs []corev1.LocalObjectReference, inline []v1alpha1.RulesetSpec) ([]Desired, error) {
	log := logPkg.FromContext(ctx)
	resolver := NewChainResolver(k8sClient, namespace)

	candidates := make([]Desired, 0, len(presetRefs)+len(inline))
	for _, ref := range presetRefs {
		preset, err := resolver.GetPreset(ctx, ref.Name)
		if err != nil {
			log.Error(err, "unable to get ruleset preset", "rulesetPreset", ref.Name)
			return nil, fmt.Errorf("failed to get ruleset preset %s: %w", ref.Name, err)
		}
		candidates = append(candidates, resolveDesired(ctx, resolver, PresetSource(ref.Name), preset.Spec, ref.Name))
	}
	for i := range inline {
		candidates = append(candidates, resolveDesired(ctx, resolver, merge.InlineSource(i), inline[i], ""))
	}

	result, conflicts := merge.Dedupe(candidates,
		func(d Desired) string { return d.Spec.Name },
		func(a, b Desired) bool {
			if a.Unresolved == nil && b.Unresolved == nil {
				return equality.Semantic.DeepEqual(a.Spec, b.Spec)
			}
			return equality.Semantic.DeepEqual(a.raw, b.raw)
		},
		func(d Desired) string { return d.Source },
		func(kept, dup Desired) {
			log.Info("WARNING: duplicate ruleset definition with identical content, applying once",
				"ruleset", dup.Spec.Name, "sources", []string{kept.Source, dup.Source})
		})
	if len(conflicts) > 0 {
		errs := make([]error, 0, len(conflicts))
		for _, c := range conflicts {
			conflictErr := &ConflictError{Name: c.Key, Sources: c.Sources}
			log.Error(conflictErr, "conflicting ruleset definitions", "ruleset", c.Key, "sources", c.Sources)
			errs = append(errs, conflictErr)
		}
		return nil, errors.Join(errs...)
	}
	markUnresolvedDuplicates(candidates, result)
	return result, nil
}

// markUnresolvedDuplicates marks a deduplicated ruleset as unresolved if any of its identical definitions could not
// be resolved, so it is left untouched on GitHub.
func markUnresolvedDuplicates(candidates, result []Desired) {
	unresolved := map[string]Desired{}
	for _, c := range candidates {
		if c.Unresolved != nil {
			unresolved[c.Spec.Name] = c
		}
	}
	for i := range result {
		if u, ok := unresolved[result[i].Spec.Name]; ok && result[i].Unresolved == nil {
			result[i].Spec = u.Spec
			result[i].Unresolved = u.Unresolved
		}
	}
}

func resolveDesired(ctx context.Context, resolver *ChainResolver, source string, spec v1alpha1.RulesetSpec, self string) Desired {
	resolved, err := resolver.Resolve(ctx, spec, self)
	if err == nil {
		return Desired{Source: source, Spec: resolved, raw: spec}
	}
	unresolved, ok := errors.AsType[*UnresolvedError](err)
	if !ok {
		unresolved = &UnresolvedError{Ruleset: spec.Name, Reason: ReasonParentUnavailable, Err: err}
	}
	logPkg.FromContext(ctx).Error(unresolved, "Could not resolve extends chain of ruleset, ruleset will not be changed on GitHub",
		"ruleset", spec.Name, "rulesetSource", source, "reason", unresolved.Reason, "chain", unresolved.Chain)
	return Desired{Source: source, Spec: *spec.DeepCopy(), Unresolved: unresolved, raw: spec}
}
