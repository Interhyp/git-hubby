package reconciler

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

// DesiredRuleset is a ruleset that should exist on GitHub, together with the source it was defined in.
type DesiredRuleset struct {
	// Source describes where the ruleset was defined, e.g. "rulesetPreset/<name>" or "inline[<idx>]".
	Source string
	// Spec is a deep copy of the ruleset definition. It may be modified freely by the caller.
	Spec v1alpha1.RulesetSpec
}

// RulesetConflictError is returned when multiple sources define a ruleset with the same name but different content.
type RulesetConflictError struct {
	Name    string
	Sources []string
}

func (e *RulesetConflictError) Error() string {
	return fmt.Sprintf("conflicting ruleset definitions for ruleset %q in sources [%s]", e.Name, strings.Join(e.Sources, ", "))
}

// RulesetPresetSource returns the source description of a ruleset defined by a RulesetPreset.
func RulesetPresetSource(name string) string {
	return "rulesetPreset/" + name
}

// InlineRulesetSource returns the source description of an inline ruleset at the given index.
func InlineRulesetSource(idx int) string {
	return fmt.Sprintf("inline[%d]", idx)
}

// CollectDesiredRulesets returns the union of the rulesets referenced via presetRefs and the inline rulesets.
// All returned specs are deep copies and can be mutated by the caller without affecting cached objects.
//
// Rulesets sharing the same name are handled as follows:
//   - identical definitions: a warning is logged and the ruleset is returned only once.
//   - differing definitions: a RulesetConflictError is returned for every conflicting name (joined via errors.Join).
func CollectDesiredRulesets(ctx context.Context, k8sClient client.Client, namespace string,
	presetRefs []corev1.LocalObjectReference, inline []v1alpha1.RulesetSpec) ([]DesiredRuleset, error) {
	log := logPkg.FromContext(ctx)

	candidates := make([]DesiredRuleset, 0, len(presetRefs)+len(inline))
	for _, ref := range presetRefs {
		var preset v1alpha1.RulesetPreset
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: namespace}, &preset); err != nil {
			log.Error(err, "unable to get ruleset preset", "rulesetPreset", ref.Name)
			return nil, fmt.Errorf("failed to get ruleset preset %s: %w", ref.Name, err)
		}
		candidates = append(candidates, DesiredRuleset{
			Source: RulesetPresetSource(ref.Name),
			Spec:   *preset.Spec.DeepCopy(),
		})
	}
	for i := range inline {
		candidates = append(candidates, DesiredRuleset{
			Source: InlineRulesetSource(i),
			Spec:   *inline[i].DeepCopy(),
		})
	}

	type nameInfo struct {
		keptIdx  int
		sources  []string
		conflict bool
	}
	result := make([]DesiredRuleset, 0, len(candidates))
	byName := make(map[string]*nameInfo, len(candidates))
	conflictOrder := make([]string, 0)

	for _, c := range candidates {
		info, seen := byName[c.Spec.Name]
		if !seen {
			byName[c.Spec.Name] = &nameInfo{keptIdx: len(result), sources: []string{c.Source}}
			result = append(result, c)
			continue
		}
		kept := result[info.keptIdx]
		info.sources = append(info.sources, c.Source)
		if equality.Semantic.DeepEqual(kept.Spec, c.Spec) {
			log.Info("WARNING: duplicate ruleset definition with identical content, applying once",
				"ruleset", c.Spec.Name, "sources", []string{kept.Source, c.Source})
			continue
		}
		if !info.conflict {
			info.conflict = true
			conflictOrder = append(conflictOrder, c.Spec.Name)
		}
	}

	if len(conflictOrder) > 0 {
		errs := make([]error, 0, len(conflictOrder))
		for _, name := range conflictOrder {
			conflictErr := &RulesetConflictError{Name: name, Sources: byName[name].sources}
			log.Error(conflictErr, "conflicting ruleset definitions", "ruleset", name, "sources", conflictErr.Sources)
			errs = append(errs, conflictErr)
		}
		return nil, errors.Join(errs...)
	}

	return result, nil
}
