package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
)

var _ = Describe("RulesetPreset watch mapping", func() {
	const namespace = "mapping-ns"

	var (
		ctx     context.Context
		objects []client.Object
		c       client.Client
	)

	preset := func(name, extends string) *githubv1alpha1.RulesetPreset {
		return &githubv1alpha1.RulesetPreset{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec:       githubv1alpha1.RulesetSpec{Name: name, Extends: extends},
		}
	}
	refs := func(names ...string) []corev1.LocalObjectReference {
		result := make([]corev1.LocalObjectReference, 0, len(names))
		for _, name := range names {
			result = append(result, corev1.LocalObjectReference{Name: name})
		}
		return result
	}
	repo := func(name string, presets []corev1.LocalObjectReference, inline ...githubv1alpha1.RulesetSpec) *githubv1alpha1.Repository {
		return &githubv1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec:       githubv1alpha1.RepositorySpec{RulesetPresetList: presets, Rulesets: inline},
		}
	}
	request := func(name string) reconcile.Request {
		return reconcile.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: namespace}}
	}

	BeforeEach(func() {
		ctx = context.Background()
		objects = []client.Object{
			preset("base", ""),
			preset("middle", "base"),
			preset("leaf-a", "middle"),
			preset("leaf-b", "middle"),
			preset("unrelated", ""),
			preset("cycle-a", "cycle-b"),
			preset("cycle-b", "cycle-a"),
			&githubv1alpha1.RulesetPreset{
				ObjectMeta: metav1.ObjectMeta{Name: "middle-other-ns", Namespace: "other"},
				Spec:       githubv1alpha1.RulesetSpec{Name: "x", Extends: "base"},
			},
		}
	})

	JustBeforeEach(func() {
		c = fake.NewClientBuilder().
			WithScheme(scheme.Scheme).
			WithObjects(objects...).
			WithIndex(&githubv1alpha1.RulesetPreset{}, rulesetPresetExtendsIndex, indexRulesetPresetExtends).
			WithIndex(&githubv1alpha1.Repository{}, rulesetPresetsNameIndex, indexRepositoryRulesetPresets).
			WithIndex(&githubv1alpha1.Organization{}, rulesetPresetsNameIndex, indexOrganizationRulesetPresets).
			Build()
	})

	Describe("index functions", func() {
		It("indexes extends and all referenced presets incl. inline extends", func() {
			Expect(indexRulesetPresetExtends(preset("x", ""))).To(BeEmpty())
			Expect(indexRulesetPresetExtends(preset("x", "base"))).To(Equal([]string{"base"}))
			Expect(indexRepositoryRulesetPresets(repo("r", refs("a", "b"),
				githubv1alpha1.RulesetSpec{Name: "i1", Extends: "c"},
				githubv1alpha1.RulesetSpec{Name: "i2", Extends: "a"},
				githubv1alpha1.RulesetSpec{Name: "i3"},
			))).To(ConsistOf("a", "b", "c"))
		})
	})

	Describe("presetWithDescendants", func() {
		It("returns the preset and all transitive descendants in the same namespace", func() {
			Expect((&RulesetPresetIndex{reader: c}).presetWithDescendants(ctx, preset("base", ""))).
				To(ConsistOf("base", "middle", "leaf-a", "leaf-b"))
			Expect((&RulesetPresetIndex{reader: c}).presetWithDescendants(ctx, preset("leaf-a", "middle"))).To(Equal([]string{"leaf-a"}))
		})

		It("terminates on cycles", func() {
			Expect((&RulesetPresetIndex{reader: c}).presetWithDescendants(ctx, preset("cycle-a", "cycle-b"))).To(ConsistOf("cycle-a", "cycle-b"))
		})
	})

	Describe("RepositoryRequests / OrganizationRequests", func() {
		BeforeEach(func() {
			objects = append(objects,
				repo("uses-leaf", refs("leaf-a")),
				repo("uses-base-and-leaf", refs("base", "leaf-b")),
				repo("inline-extends-middle", nil, githubv1alpha1.RulesetSpec{Name: "i", Extends: "middle"}),
				repo("uses-unrelated", refs("unrelated")),
				&githubv1alpha1.Organization{
					ObjectMeta: metav1.ObjectMeta{Name: "org", Namespace: namespace},
					Spec:       githubv1alpha1.OrganizationSpec{RulesetPresetList: refs("leaf-b")},
				},
			)
		})

		It("enqueues every resource using the changed preset directly, via extends or inline, once", func() {
			index := &RulesetPresetIndex{reader: c}

			Expect(index.RepositoryRequests(ctx, preset("base", ""))).To(ConsistOf(
				request("uses-leaf"), request("uses-base-and-leaf"), request("inline-extends-middle"),
			))
			Expect(index.RepositoryRequests(ctx, preset("leaf-a", "middle"))).To(ConsistOf(
				request("uses-leaf"),
			))
			Expect(index.OrganizationRequests(ctx, preset("base", ""))).To(ConsistOf(request("org")))
			Expect(index.OrganizationRequests(ctx, preset("unrelated", ""))).To(BeEmpty())
		})
	})
})
