package rulesets

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// jsonName returns the JSON field name of a struct field, or "" if it is not serialized.
func jsonName(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	if name == "-" || !f.IsExported() {
		return ""
	}
	return name
}

// specFieldPaths returns the JSON paths of all RulesetSpec fields relevant for merging: the fields of the spec and,
// for every field declared as MergeRecursive, the fields of the nested object.
func specFieldPaths(t reflect.Type, prefix string) []string {
	var paths []string
	for field := range t.Fields() {
		name := jsonName(field)
		if name == "" {
			continue
		}
		path := prefix + name
		paths = append(paths, path)
		if MergeRules[path] == MergeRecursive {
			ft := field.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			paths = append(paths, specFieldPaths(ft, path+".")...)
		}
	}
	return paths
}

// fieldByPath returns the (addressable) field at the given JSON path, allocating nil pointers on the way.
func fieldByPath(v reflect.Value, path string) reflect.Value {
	for segment := range strings.SplitSeq(path, ".") {
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		found := false
		for i := range v.NumField() {
			if jsonName(v.Type().Field(i)) == segment {
				v, found = v.Field(i), true
				break
			}
		}
		Expect(found).To(BeTrue(), "unknown path segment %q in %q", segment, path)
	}
	return v
}

// fill sets v to a non-zero value derived from seed. Different seeds lead to different values (and keys).
func fill(v reflect.Value, seed string) {
	switch v.Kind() {
	case reflect.String:
		v.SetString(seed)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int64:
		v.SetInt(int64(len(seed)))
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		if v.Elem().Kind() == reflect.Bool {
			// distinguish parent and child for boolean pointers
			v.Elem().SetBool(strings.HasPrefix(seed, "child"))
			return
		}
		fill(v.Elem(), seed)
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		fill(v.Index(0), seed)
	case reflect.Struct:
		for i := range v.NumField() {
			if jsonName(v.Type().Field(i)) != "" {
				fill(v.Field(i), fmt.Sprintf("%s-%d", seed, i))
			}
		}
	default:
		Fail(fmt.Sprintf("fill: unsupported kind %s", v.Kind()))
	}
}

var _ = Describe("Extends merge rules", func() {
	It("declares a merge rule for every RulesetSpec field and no unknown fields", func() {
		paths := specFieldPaths(reflect.TypeFor[v1alpha1.RulesetSpec](), "")
		var missing []string
		for _, p := range paths {
			if _, ok := MergeRules[p]; !ok {
				missing = append(missing, p)
			}
		}
		Expect(missing).To(BeEmpty(), "add a merge rule to MergeRules and mergeSpec for these fields")

		var unknown []string
		for p := range MergeRules {
			if !slices.Contains(paths, p) {
				unknown = append(unknown, p)
			}
		}
		slices.Sort(unknown)
		Expect(unknown).To(BeEmpty(), "MergeRules contains paths that do not exist in RulesetSpec")
	})

	// For every non-recursive field, the declared rule is verified against mergeSpec. This catches fields that are
	// declared in MergeRules but not handled (or handled differently) in mergeSpec.
	// iterate in a stable order, Ginkgo requires a deterministic spec tree
	for _, path := range slices.Sorted(maps.Keys(MergeRules)) {
		kind := MergeRules[path]
		if kind == MergeRecursive {
			continue
		}
		Context(fmt.Sprintf("field %s (%s)", path, kind), func() {
			var parent, child v1alpha1.RulesetSpec

			BeforeEach(func() {
				parent = v1alpha1.RulesetSpec{Name: "parent"}
				child = v1alpha1.RulesetSpec{Name: "child"}
				fill(fieldByPath(reflect.ValueOf(&parent).Elem(), path), "parent")
			})

			valueOf := func(spec *v1alpha1.RulesetSpec) any {
				return fieldByPath(reflect.ValueOf(spec).Elem(), path).Interface()
			}

			It("handles an unset child value", func() {
				expected := valueOf(parent.DeepCopy())
				merged := mergeSpec(*parent.DeepCopy(), *child.DeepCopy())
				switch kind {
				case MergeOwn:
					Expect(valueOf(&merged)).To(Equal(valueOf(child.DeepCopy())))
				default:
					Expect(valueOf(&merged)).To(Equal(expected))
				}
			})

			It("handles a set child value", func() {
				fill(fieldByPath(reflect.ValueOf(&child).Elem(), path), "child")
				parentValue := valueOf(parent.DeepCopy())
				childValue := valueOf(child.DeepCopy())
				merged := mergeSpec(*parent.DeepCopy(), *child.DeepCopy())
				switch kind {
				case MergeKeyed:
					mergedList := reflect.ValueOf(valueOf(&merged))
					Expect(mergedList.Len()).To(Equal(2))
					Expect(mergedList.Index(0).Interface()).To(Equal(reflect.ValueOf(parentValue).Index(0).Interface()))
					Expect(mergedList.Index(1).Interface()).To(Equal(reflect.ValueOf(childValue).Index(0).Interface()))
				default:
					Expect(valueOf(&merged)).To(Equal(childValue))
				}
			})
		})
	}

	Describe("keyed lists", func() {
		It("replaces a bypass actor with the same type and slug in place and appends new ones", func() {
			parent := v1alpha1.RulesetSpec{BypassActors: []v1alpha1.RulesetBypassActor{
				{ActorType: "Team", ActorSlug: new("incidentmanager"), BypassMode: "always"},
				{ActorType: "OrganizationAdmin", BypassMode: "always"},
			}}
			child := v1alpha1.RulesetSpec{BypassActors: []v1alpha1.RulesetBypassActor{
				{ActorType: "Integration", ActorSlug: new("release-bot"), BypassMode: "always"},
				{ActorType: "Team", ActorSlug: new("incidentmanager"), BypassMode: "pull_request"},
				// same slug, different type: a different actor
				{ActorType: "Integration", ActorSlug: new("incidentmanager"), BypassMode: "always"},
			}}
			merged := mergeSpec(parent, child)
			Expect(merged.BypassActors).To(Equal([]v1alpha1.RulesetBypassActor{
				{ActorType: "Team", ActorSlug: new("incidentmanager"), BypassMode: "pull_request"},
				{ActorType: "OrganizationAdmin", BypassMode: "always"},
				{ActorType: "Integration", ActorSlug: new("release-bot"), BypassMode: "always"},
				{ActorType: "Integration", ActorSlug: new("incidentmanager"), BypassMode: "always"},
			}))
		})

		It("distinguishes bypass actors given by slug and by ID", func() {
			parent := v1alpha1.RulesetSpec{BypassActors: []v1alpha1.RulesetBypassActor{{ActorType: "RepositoryRole", ActorID: new(int64(5))}}}
			child := v1alpha1.RulesetSpec{BypassActors: []v1alpha1.RulesetBypassActor{{ActorType: "RepositoryRole", ActorID: new(int64(5)), BypassMode: "pull_request"}}}
			Expect(mergeSpec(parent, child).BypassActors).To(Equal(child.BypassActors))
		})

		It("merges status checks by context and app and keeps the strict policy of the parent", func() {
			parent := v1alpha1.RulesetSpec{Rules: v1alpha1.RulesetRules{RequiredStatusChecks: &v1alpha1.RequiredStatusChecks{
				StrictPolicy: new(true),
				Checks: []v1alpha1.StatusCheck{
					{Context: "build", AppSlug: new("github-actions")},
					{Context: "build", AppSlug: new("jenkins")},
					{Context: "snyk", IntegrationID: new(int64(1))},
					{Context: "ci/build"},
				},
			}}}
			child := v1alpha1.RulesetSpec{Rules: v1alpha1.RulesetRules{RequiredStatusChecks: &v1alpha1.RequiredStatusChecks{
				Checks: []v1alpha1.StatusCheck{
					{Context: "build", AppSlug: new("other")},
					{Context: "snyk", IntegrationID: new(int64(2))},
					{Context: "ci/build"},
					{Context: "build / build"},
				},
			}}}
			merged := mergeSpec(parent, child)
			Expect(*merged.Rules.RequiredStatusChecks).To(Equal(v1alpha1.RequiredStatusChecks{
				StrictPolicy: new(true),
				Checks: []v1alpha1.StatusCheck{
					{Context: "build", AppSlug: new("github-actions")},
					{Context: "build", AppSlug: new("jenkins")},
					{Context: "snyk", IntegrationID: new(int64(1))},
					{Context: "ci/build"},
					{Context: "build", AppSlug: new("other")},
					{Context: "snyk", IntegrationID: new(int64(2))},
					{Context: "build / build"},
				},
			}))
		})
	})

	Describe("mergeKeyed", func() {
		type entry struct{ key, value string }
		key := func(e entry) string { return e.key }

		It("replaces all parent entries sharing a key with the child entry, once and in place", func() {
			parent := []entry{{"a", "p1"}, {"b", "p"}, {"a", "p2"}, {"c", "p"}}
			child := []entry{{"d", "c"}, {"a", "c"}}
			Expect(mergeKeyed(parent, child, key)).To(Equal([]entry{{"a", "c"}, {"b", "p"}, {"c", "p"}, {"d", "c"}}))
		})

		It("keeps parent entries with duplicate keys not overridden by the child", func() {
			parent := []entry{{"a", "p1"}, {"a", "p2"}}
			Expect(mergeKeyed(parent, []entry{{"b", "c"}}, key)).To(Equal([]entry{{"a", "p1"}, {"a", "p2"}, {"b", "c"}}))
		})

		It("uses the last child entry for duplicate child keys", func() {
			Expect(mergeKeyed(nil, []entry{{"a", "c1"}, {"a", "c2"}}, key)).To(Equal([]entry{{"a", "c2"}}))
		})
	})

	It("merges nested rules recursively", func() {
		parent := v1alpha1.RulesetSpec{Rules: v1alpha1.RulesetRules{
			RequiredSignatures: new(true),
			PullRequest: &v1alpha1.PullRequestRule{
				AllowedMergeMethods:          []string{"rebase", "merge"},
				RequireCodeOwnerReviews:      new(true),
				RequiredApprovingReviewCount: 1,
			},
		}}
		child := v1alpha1.RulesetSpec{Rules: v1alpha1.RulesetRules{
			RequiredSignatures: new(false),
			PullRequest:        &v1alpha1.PullRequestRule{RequiredApprovingReviewCount: 2},
		}}
		merged := mergeSpec(parent, child)
		Expect(merged.Rules.RequiredSignatures).To(Equal(new(false)))
		Expect(*merged.Rules.PullRequest).To(Equal(v1alpha1.PullRequestRule{
			AllowedMergeMethods:          []string{"rebase", "merge"},
			RequireCodeOwnerReviews:      new(true),
			RequiredApprovingReviewCount: 2,
		}))
	})
})
