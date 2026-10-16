package controller

import (
	"context"
	"slices"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler/common/rulesets"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// rulesetPresetExtendsIndex indexes RulesetPresets by the preset they extend.
	rulesetPresetExtendsIndex = "spec.extends"
	// rulesetPresetsNameIndex indexes Organizations and Repositories by the RulesetPresets they reference directly
	// or via the extends field of an inline ruleset.
	rulesetPresetsNameIndex = "spec.rulesetPresets.name"
)

// RulesetPresetIndex maps RulesetPreset changes to the Organizations and Repositories using the preset directly,
// via an inline ruleset extending it or via a (transitively) extending preset. It is set up once per manager and
// shared by the Organization and the Repository controller.
type RulesetPresetIndex struct {
	reader client.Reader
}

// SetupRulesetPresetIndex registers the field indexes on RulesetPresets, Organizations and Repositories needed to
// map RulesetPreset changes and returns the index operating on the manager's cached client.
func SetupRulesetPresetIndex(ctx context.Context, mgr ctrl.Manager) (*RulesetPresetIndex, error) {
	indexer := mgr.GetFieldIndexer()
	if err := indexer.IndexField(ctx, &githubv1alpha1.RulesetPreset{}, rulesetPresetExtendsIndex, indexRulesetPresetExtends); err != nil {
		return nil, err
	}
	if err := indexer.IndexField(ctx, &githubv1alpha1.Organization{}, rulesetPresetsNameIndex, indexOrganizationRulesetPresets); err != nil {
		return nil, err
	}
	if err := indexer.IndexField(ctx, &githubv1alpha1.Repository{}, rulesetPresetsNameIndex, indexRepositoryRulesetPresets); err != nil {
		return nil, err
	}
	return &RulesetPresetIndex{reader: mgr.GetClient()}, nil
}

// OrganizationRequests returns reconcile requests for all Organizations affected by the given RulesetPreset.
func (i *RulesetPresetIndex) OrganizationRequests(ctx context.Context, obj client.Object) []reconcile.Request {
	return i.requests(ctx, obj.(*githubv1alpha1.RulesetPreset),
		func() client.ObjectList { return &githubv1alpha1.OrganizationList{} },
		func(list client.ObjectList) []client.Object {
			items := list.(*githubv1alpha1.OrganizationList).Items
			objs := make([]client.Object, 0, len(items))
			for idx := range items {
				objs = append(objs, &items[idx])
			}
			return objs
		})
}

// RepositoryRequests returns reconcile requests for all Repositories affected by the given RulesetPreset.
func (i *RulesetPresetIndex) RepositoryRequests(ctx context.Context, obj client.Object) []reconcile.Request {
	return i.requests(ctx, obj.(*githubv1alpha1.RulesetPreset),
		func() client.ObjectList { return &githubv1alpha1.RepositoryList{} },
		func(list client.ObjectList) []client.Object {
			items := list.(*githubv1alpha1.RepositoryList).Items
			objs := make([]client.Object, 0, len(items))
			for idx := range items {
				objs = append(objs, &items[idx])
			}
			return objs
		})
}

func indexRulesetPresetExtends(obj client.Object) []string {
	preset := obj.(*githubv1alpha1.RulesetPreset)
	if preset.Spec.Extends == "" {
		return nil
	}
	return []string{preset.Spec.Extends}
}

// referencedRulesetPresetNames returns the names of all RulesetPresets referenced directly or as extends target of
// an inline ruleset.
func referencedRulesetPresetNames(refs []corev1.LocalObjectReference, inline []githubv1alpha1.RulesetSpec) []string {
	names := make([]string, 0, len(refs)+len(inline))
	for _, ref := range refs {
		names = append(names, ref.Name)
	}
	for _, rs := range inline {
		if rs.Extends != "" {
			names = append(names, rs.Extends)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func indexOrganizationRulesetPresets(obj client.Object) []string {
	org := obj.(*githubv1alpha1.Organization)
	return referencedRulesetPresetNames(org.Spec.RulesetPresetList, org.Spec.Rulesets)
}

func indexRepositoryRulesetPresets(obj client.Object) []string {
	repo := obj.(*githubv1alpha1.Repository)
	return referencedRulesetPresetNames(repo.Spec.RulesetPresetList, repo.Spec.Rulesets)
}

// presetWithDescendants returns the name of the given preset and of all presets (transitively) extending it.
// The walk is bounded by rulesets.MaxExtendsDepth and tolerates cycles; on list errors the names found so far are
// returned.
func (i *RulesetPresetIndex) presetWithDescendants(ctx context.Context, preset *githubv1alpha1.RulesetPreset) []string {
	names := []string{preset.Name}
	seen := map[string]bool{preset.Name: true}
	level := []string{preset.Name}
	for depth := 0; depth < rulesets.MaxExtendsDepth && len(level) > 0; depth++ {
		var next []string
		for _, parent := range level {
			var children githubv1alpha1.RulesetPresetList
			if err := i.reader.List(ctx, &children, client.InNamespace(preset.Namespace),
				client.MatchingFields{rulesetPresetExtendsIndex: parent}); err != nil {
				logf.FromContext(ctx).Error(err, "Failed to list RulesetPresets extending RulesetPreset", "rulesetPreset", parent)
				return names
			}
			for _, child := range children.Items {
				if seen[child.Name] {
					continue
				}
				seen[child.Name] = true
				names = append(names, child.Name)
				next = append(next, child.Name)
			}
		}
		level = next
	}
	return names
}

// requests returns reconcile requests for all objects of the given list type that reference the preset or any of
// its descendants.
func (i *RulesetPresetIndex) requests(ctx context.Context, preset *githubv1alpha1.RulesetPreset,
	newList func() client.ObjectList, items func(client.ObjectList) []client.Object) []reconcile.Request {
	var requests []reconcile.Request
	seen := map[types.NamespacedName]bool{}
	for _, name := range i.presetWithDescendants(ctx, preset) {
		list := newList()
		if err := i.reader.List(ctx, list, client.InNamespace(preset.Namespace), client.MatchingFields{rulesetPresetsNameIndex: name}); err != nil {
			logf.FromContext(ctx).Error(err, "Failed to list resources referencing RulesetPreset", "rulesetPreset", name)
			continue
		}
		for _, obj := range items(list) {
			key := types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}
			if seen[key] {
				continue
			}
			seen[key] = true
			requests = append(requests, reconcile.Request{NamespacedName: key})
		}
	}
	return requests
}
