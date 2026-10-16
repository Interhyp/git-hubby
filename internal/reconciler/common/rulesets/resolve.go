package rulesets

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// MaxExtendsDepth is the maximum number of RulesetPresets a ruleset may (transitively) extend.
	MaxExtendsDepth = 5
	// DefaultTarget is applied to resolved rulesets without a target.
	DefaultTarget = "branch"
)

// UnresolvedReason describes why the extends chain of a ruleset could not be resolved.
type UnresolvedReason string

const (
	ReasonParentNotFound    UnresolvedReason = "ParentPresetNotFound"
	ReasonParentUnavailable UnresolvedReason = "ParentPresetUnavailable"
	ReasonExtendsCycle      UnresolvedReason = "ExtendsCycle"
	ReasonExtendsTooDeep    UnresolvedReason = "ExtendsTooDeep"
	ReasonNoEnforcement     UnresolvedReason = "EnforcementMissing"
)

// UnresolvedError is returned when the extends chain of a ruleset cannot be resolved. Rulesets with an
// UnresolvedError are neither created, updated nor deleted on GitHub (fail closed).
type UnresolvedError struct {
	Ruleset string
	// Chain lists the RulesetPresets walked so far, nearest parent first.
	Chain  []string
	Reason UnresolvedReason
	Err    error
}

func (e *UnresolvedError) Error() string {
	msg := fmt.Sprintf("ruleset %q could not be resolved (%s), extends chain [%s]", e.Ruleset, e.Reason, strings.Join(e.Chain, " -> "))
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return msg
}

func (e *UnresolvedError) Unwrap() error {
	return e.Err
}

// AwaitsPresetChange reports whether err (including all wrapped and joined errors) consists only of
// UnresolvedErrors that can only be fixed by changing a RulesetPreset or the resource itself (missing parent, cycle,
// too deep, missing enforcement). Retrying such errors with backoff is pointless: the RulesetPreset watches reconcile
// all affected resources once a preset in the chain is created or changed. Transient read errors
// (ReasonParentUnavailable) and any other error return false.
func AwaitsPresetChange(err error) bool {
	if unresolved, ok := err.(*UnresolvedError); ok { //nolint:errorlint // the error tree is walked explicitly
		return unresolved.Reason != ReasonParentUnavailable
	}
	switch e := err.(type) { //nolint:errorlint // the error tree is walked explicitly
	case interface{ Unwrap() []error }:
		errs := e.Unwrap()
		for _, inner := range errs {
			if !AwaitsPresetChange(inner) {
				return false
			}
		}
		return len(errs) > 0
	case interface{ Unwrap() error }:
		return AwaitsPresetChange(e.Unwrap())
	default:
		return false
	}
}

// ChainResolver resolves the extends chains of rulesets. Fetched RulesetPresets are memoized, so a ChainResolver
// should be used for a single reconciliation only (presets sharing a base fetch it once, and changes are picked up
// by the next reconciliation). It is not safe for concurrent use.
type ChainResolver struct {
	reader    client.Reader
	namespace string
	presets   map[string]*v1alpha1.RulesetPreset
}

// NewChainResolver creates a ChainResolver reading RulesetPresets from the given namespace. The reader should be
// the manager's cached client, so resolution does not call the API server.
func NewChainResolver(reader client.Reader, namespace string) *ChainResolver {
	return &ChainResolver{reader: reader, namespace: namespace, presets: make(map[string]*v1alpha1.RulesetPreset)}
}

// GetPreset returns the RulesetPreset with the given name. The returned object is shared and must not be modified.
func (r *ChainResolver) GetPreset(ctx context.Context, name string) (*v1alpha1.RulesetPreset, error) {
	if p, ok := r.presets[name]; ok {
		return p, nil
	}
	var preset v1alpha1.RulesetPreset
	if err := r.reader.Get(ctx, client.ObjectKey{Name: name, Namespace: r.namespace}, &preset); err != nil {
		return nil, err
	}
	r.presets[name] = &preset
	return &preset, nil
}

// Resolve returns the effective ruleset of spec: the fully resolved parent chain with spec merged on top. self is
// the name of the RulesetPreset spec belongs to (empty for inline rulesets) and is used for cycle detection.
// The returned spec is an independent deep copy with Extends cleared and Target defaulted. Specs without Extends
// are returned as (defaulted) deep copy.
func (r *ChainResolver) Resolve(ctx context.Context, spec v1alpha1.RulesetSpec, self string) (v1alpha1.RulesetSpec, error) {
	specs := []*v1alpha1.RulesetSpec{&spec}
	chain := make([]string, 0, MaxExtendsDepth)
	seen := map[string]bool{}
	if self != "" {
		seen[self] = true
	}
	unresolved := func(reason UnresolvedReason, err error) (v1alpha1.RulesetSpec, error) {
		return v1alpha1.RulesetSpec{}, &UnresolvedError{Ruleset: spec.Name, Chain: chain, Reason: reason, Err: err}
	}
	for next := spec.Extends; next != ""; {
		if seen[next] {
			chain = append(chain, next)
			return unresolved(ReasonExtendsCycle, nil)
		}
		if len(chain) == MaxExtendsDepth {
			chain = append(chain, next)
			return unresolved(ReasonExtendsTooDeep, fmt.Errorf("at most %d levels are allowed", MaxExtendsDepth))
		}
		seen[next] = true
		chain = append(chain, next)
		preset, err := r.GetPreset(ctx, next)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return unresolved(ReasonParentNotFound, nil)
			}
			return unresolved(ReasonParentUnavailable, err)
		}
		specs = append(specs, &preset.Spec)
		next = preset.Spec.Extends
	}

	result := *specs[len(specs)-1].DeepCopy()
	for i := len(specs) - 2; i >= 0; i-- {
		result = mergeSpec(result, *specs[i].DeepCopy())
	}
	result.Extends = ""
	if result.Target == "" {
		result.Target = DefaultTarget
	}
	if result.Enforcement == "" {
		return unresolved(ReasonNoEnforcement, errors.New("no ruleset in the chain sets enforcement"))
	}
	return result, nil
}

// Ancestors returns the RulesetPresets the given extends chain consists of, nearest parent first. It is meant for
// best-effort change tracking: walking stops silently at a missing preset, a cycle or MaxExtendsDepth. Only
// unexpected read errors are returned.
func (r *ChainResolver) Ancestors(ctx context.Context, extends, self string) ([]*v1alpha1.RulesetPreset, error) {
	result := make([]*v1alpha1.RulesetPreset, 0, MaxExtendsDepth)
	seen := map[string]bool{self: self != ""}
	for next := extends; next != "" && !seen[next] && len(result) < MaxExtendsDepth; {
		seen[next] = true
		preset, err := r.GetPreset(ctx, next)
		if err != nil {
			if apierrors.IsNotFound(err) {
				break
			}
			return nil, err
		}
		result = append(result, preset)
		next = preset.Spec.Extends
	}
	return result, nil
}
