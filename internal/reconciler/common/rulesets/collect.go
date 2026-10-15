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
	// Spec is a deep copy of the ruleset definition. It may be modified freely by the caller.
	Spec v1alpha1.RulesetSpec
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

// Collect returns the union of the rulesets referenced via presetRefs and the inline rulesets.
// All returned specs are deep copies and can be mutated by the caller without affecting cached objects.
//
// Rulesets sharing the same name are handled as follows:
//   - identical definitions: a warning is logged and the ruleset is returned only once.
//   - differing definitions: a ConflictError is returned for every conflicting name (joined via errors.Join).
func Collect(ctx context.Context, k8sClient client.Client, namespace string,
	presetRefs []corev1.LocalObjectReference, inline []v1alpha1.RulesetSpec) ([]Desired, error) {
	log := logPkg.FromContext(ctx)

	candidates := make([]Desired, 0, len(presetRefs)+len(inline))
	for _, ref := range presetRefs {
		var preset v1alpha1.RulesetPreset
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: namespace}, &preset); err != nil {
			log.Error(err, "unable to get ruleset preset", "rulesetPreset", ref.Name)
			return nil, fmt.Errorf("failed to get ruleset preset %s: %w", ref.Name, err)
		}
		candidates = append(candidates, Desired{
			Source: PresetSource(ref.Name),
			Spec:   *preset.Spec.DeepCopy(),
		})
	}
	for i := range inline {
		candidates = append(candidates, Desired{
			Source: merge.InlineSource(i),
			Spec:   *inline[i].DeepCopy(),
		})
	}

	result, conflicts := merge.Dedupe(candidates,
		func(d Desired) string { return d.Spec.Name },
		func(a, b Desired) bool { return equality.Semantic.DeepEqual(a.Spec, b.Spec) },
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
	return result, nil
}
