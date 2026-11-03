package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	ac "github.com/Interhyp/git-hubby/api/v1alpha1/applyconfiguration/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler"
)

// These tests run against the envtest API server (see suite_test.go). They verify that the status map
// observedSubResourceGenerations is atomic: a status apply must replace it entirely, so that keys of sub-resources
// that are no longer referenced do not survive. Stale keys make startup spreading treat the resource as changed on
// every restart, which leads to periodic reconciliation spikes.
var _ = Describe("Repository status observedSubResourceGenerations", func() {
	const namespace = "default"

	var (
		ctx  context.Context
		repo *githubv1alpha1.Repository
	)

	BeforeEach(func() {
		ctx = context.Background()
		repo = &githubv1alpha1.Repository{
			GenerateName: "subresource-gens-", Namespace: namespace,
			Spec: githubv1alpha1.RepositorySpec{
				Name:            "subresource-gens",
				OrganizationRef: githubv1alpha1.OrganizationRef{Name: "some-org"},
			},
		}
		Expect(k8sClient.Create(ctx, repo)).To(Succeed())
		DeferCleanup(func() { Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, repo))).To(Succeed()) })
	})

	applyGenerations := func(owner client.FieldOwner, gens map[string]int64) {
		// Like the reconcilers, always apply conditions; an empty status would be serialized as null.
		status := ac.RepositoryStatus().WithConditions(metav1ac.Condition().
			WithType("Ready").WithStatus(metav1.ConditionTrue).WithReason("Test").WithMessage("test").
			WithLastTransitionTime(metav1.Now()))
		if gens != nil {
			status.WithObservedSubResourceGenerations(gens)
		}
		cfg := ac.Repository(repo.Name, repo.Namespace).WithStatus(status)
		Expect(k8sClient.Status().Apply(ctx, cfg, client.ForceOwnership, owner)).To(Succeed())
	}

	observedGenerations := func() map[string]int64 {
		var current githubv1alpha1.Repository
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: repo.Name, Namespace: repo.Namespace}, &current)).To(Succeed())
		return current.Status.ObservedSubResourceGenerations
	}

	It("removes keys of sub-resources that are no longer referenced", func() {
		applyGenerations(reconciler.FieldOwner, map[string]int64{"WebhookPreset/default/jenkins": 1, "RulesetPreset/default/default": 2})
		applyGenerations(reconciler.FieldOwner, map[string]int64{"RulesetPreset/default/default": 2})

		Expect(observedGenerations()).To(Equal(map[string]int64{"RulesetPreset/default/default": 2}))
	})

	It("removes keys written by another field manager", func() {
		applyGenerations("previous-manager", map[string]int64{"WebhookPreset/default/jenkins": 1, "RulesetPreset/default/default": 2})
		applyGenerations(reconciler.FieldOwner, map[string]int64{"RulesetPreset/default/default": 2})

		Expect(observedGenerations()).To(Equal(map[string]int64{"RulesetPreset/default/default": 2}))
	})

	It("removes the map once no sub-resources are referenced anymore", func() {
		applyGenerations(reconciler.FieldOwner, map[string]int64{"WebhookPreset/default/jenkins": 1})
		applyGenerations(reconciler.FieldOwner, nil)

		Expect(observedGenerations()).To(BeEmpty())
	})
})
