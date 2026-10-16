package rulesets

import (
	"context"
	"errors"
	"fmt"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler/common/merge"
	"github.com/Interhyp/git-hubby/test/testutil"
	"github.com/google/go-github/v92/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// countingReader counts Get calls per object name and can inject errors.
type countingReader struct {
	client.Reader
	gets   map[string]int
	errFor map[string]error
}

func (c *countingReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	c.gets[key.Name]++
	if err, ok := c.errFor[key.Name]; ok {
		return err
	}
	return c.Reader.Get(ctx, key, obj, opts...)
}

var _ = Describe("Extends resolution", func() {
	const namespace = "test-ns"

	var (
		ctx      context.Context
		presets  []client.Object
		reader   *countingReader
		resolver *ChainResolver
	)

	preset := func(name string, spec v1alpha1.RulesetSpec) *v1alpha1.RulesetPreset {
		return &v1alpha1.RulesetPreset{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}, Spec: spec}
	}

	base := v1alpha1.RulesetSpec{
		Name:        "base",
		Target:      "tag",
		Enforcement: v1alpha1.RulesetEnforcementActive,
		Conditions:  &v1alpha1.RulesetConditions{RefName: &v1alpha1.RefNameCondition{Include: []string{"~DEFAULT_BRANCH"}}},
		BypassActors: []v1alpha1.RulesetBypassActor{
			{ActorType: "Team", ActorSlug: new("incidentmanager"), BypassMode: "always"},
		},
		Rules: v1alpha1.RulesetRules{
			RequiredSignatures: new(true),
			PullRequest:        &v1alpha1.PullRequestRule{RequiredApprovingReviewCount: 1},
		},
	}

	BeforeEach(func() {
		ctx, _ = testutil.CapturingContext()
		presets = []client.Object{preset("base", base)}
	})

	JustBeforeEach(func() {
		reader = &countingReader{Reader: testutil.NewFakeClient(presets...), gets: map[string]int{}, errFor: map[string]error{}}
		resolver = NewChainResolver(reader, namespace)
	})

	Describe("Resolve", func() {
		It("returns a defaulted deep copy of a spec without extends", func() {
			spec := v1alpha1.RulesetSpec{Name: "plain", Enforcement: v1alpha1.RulesetEnforcementActive}
			resolved, err := resolver.Resolve(ctx, spec, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved.Target).To(Equal(DefaultTarget))
			Expect(reader.gets).To(BeEmpty())
		})

		Context("with a chain of presets", func() {
			BeforeEach(func() {
				presets = append(presets,
					preset("middle", v1alpha1.RulesetSpec{
						Name:    "middle",
						Extends: "base",
						Rules: v1alpha1.RulesetRules{RequiredStatusChecks: &v1alpha1.RequiredStatusChecks{
							Checks: []v1alpha1.StatusCheck{{Context: "ci/build"}},
						}},
					}),
				)
			})

			It("merges the whole chain, keeps the own name and inherits the target", func() {
				child := v1alpha1.RulesetSpec{
					Name:    "child",
					Extends: "middle",
					BypassActors: []v1alpha1.RulesetBypassActor{
						{ActorType: "Integration", ActorSlug: new("release-bot"), BypassMode: "always"},
					},
				}
				resolved, err := resolver.Resolve(ctx, child, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(resolved.Name).To(Equal("child"))
				Expect(resolved.Extends).To(BeEmpty())
				Expect(resolved.Target).To(Equal("tag"))
				Expect(resolved.Enforcement).To(Equal(v1alpha1.RulesetEnforcementActive))
				Expect(resolved.Conditions).To(Equal(base.Conditions))
				Expect(resolved.BypassActors).To(HaveLen(2))
				Expect(resolved.Rules.RequiredSignatures).To(Equal(new(true)))
				Expect(resolved.Rules.PullRequest.RequiredApprovingReviewCount).To(Equal(1))
				Expect(resolved.Rules.RequiredStatusChecks.Checks).To(Equal([]v1alpha1.StatusCheck{{Context: "ci/build"}}))
			})

			It("fetches every preset only once per resolver", func() {
				for _, name := range []string{"a", "b", "c"} {
					_, err := resolver.Resolve(ctx, v1alpha1.RulesetSpec{Name: name, Extends: "middle"}, "")
					Expect(err).NotTo(HaveOccurred())
				}
				Expect(reader.gets).To(Equal(map[string]int{"middle": 1, "base": 1}))
			})

			It("returns results that do not share memory with the cached presets", func() {
				first, err := resolver.Resolve(ctx, v1alpha1.RulesetSpec{Name: "a", Extends: "middle"}, "")
				Expect(err).NotTo(HaveOccurred())
				first.Rules.PullRequest.RequiredApprovingReviewCount = 99
				first.Rules.RequiredStatusChecks.Checks[0].Context = "changed"
				first.BypassActors[0].BypassMode = "changed"

				second, err := resolver.Resolve(ctx, v1alpha1.RulesetSpec{Name: "b", Extends: "middle"}, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(second.Rules.PullRequest.RequiredApprovingReviewCount).To(Equal(1))
				Expect(second.Rules.RequiredStatusChecks.Checks[0].Context).To(Equal("ci/build"))
				Expect(second.BypassActors[0].BypassMode).To(Equal("always"))
			})
		})

		expectUnresolved := func(err error, reason UnresolvedReason, chain ...string) *UnresolvedError {
			unresolved, ok := errors.AsType[*UnresolvedError](err)
			ExpectWithOffset(1, ok).To(BeTrue(), "expected UnresolvedError, got %v", err)
			ExpectWithOffset(1, unresolved.Reason).To(Equal(reason))
			ExpectWithOffset(1, unresolved.Chain).To(Equal(chain))
			return unresolved
		}

		It("reports a missing parent", func() {
			presets = append(presets, preset("middle", v1alpha1.RulesetSpec{Name: "middle", Extends: "missing"}))
			reader = &countingReader{Reader: testutil.NewFakeClient(presets...), gets: map[string]int{}, errFor: map[string]error{}}
			resolver = NewChainResolver(reader, namespace)
			_, err := resolver.Resolve(ctx, v1alpha1.RulesetSpec{Name: "child", Extends: "middle"}, "")
			unresolved := expectUnresolved(err, ReasonParentNotFound, "middle", "missing")
			Expect(unresolved.Ruleset).To(Equal("child"))
			Expect(err.Error()).To(ContainSubstring(`ruleset "child" could not be resolved (ParentPresetNotFound), extends chain [middle -> missing]`))
		})

		It("reports read errors as unavailable parent", func() {
			reader.errFor["base"] = errors.New("boom")
			_, err := resolver.Resolve(ctx, v1alpha1.RulesetSpec{Name: "child", Extends: "base"}, "")
			expectUnresolved(err, ReasonParentUnavailable, "base")
		})

		It("detects a preset extending itself", func() {
			self := v1alpha1.RulesetSpec{Name: "self", Extends: "self"}
			_, err := resolver.Resolve(ctx, self, "self")
			expectUnresolved(err, ReasonExtendsCycle, "self")
		})

		It("detects cycles across presets", func() {
			presets = []client.Object{
				preset("a", v1alpha1.RulesetSpec{Name: "a", Extends: "b"}),
				preset("b", v1alpha1.RulesetSpec{Name: "b", Extends: "a"}),
			}
			reader = &countingReader{Reader: testutil.NewFakeClient(presets...), gets: map[string]int{}, errFor: map[string]error{}}
			resolver = NewChainResolver(reader, namespace)
			_, err := resolver.Resolve(ctx, v1alpha1.RulesetSpec{Name: "child", Extends: "a"}, "")
			expectUnresolved(err, ReasonExtendsCycle, "a", "b", "a")
		})

		It("limits the chain depth", func() {
			names := []string{"l1", "l2", "l3", "l4", "l5", "l6"}
			presets = []client.Object{}
			for i, name := range names {
				extends := ""
				if i+1 < len(names) {
					extends = names[i+1]
				}
				presets = append(presets, preset(name, v1alpha1.RulesetSpec{Name: name, Extends: extends, Enforcement: "active"}))
			}
			reader = &countingReader{Reader: testutil.NewFakeClient(presets...), gets: map[string]int{}, errFor: map[string]error{}}
			resolver = NewChainResolver(reader, namespace)

			_, err := resolver.Resolve(ctx, v1alpha1.RulesetSpec{Name: "ok", Extends: "l2"}, "")
			Expect(err).NotTo(HaveOccurred())

			_, err = resolver.Resolve(ctx, v1alpha1.RulesetSpec{Name: "too-deep", Extends: "l1"}, "")
			expectUnresolved(err, ReasonExtendsTooDeep, names...)
		})

		It("fails if no ruleset in the chain sets enforcement", func() {
			presets = []client.Object{preset("no-enforcement", v1alpha1.RulesetSpec{Name: "no-enforcement"})}
			reader = &countingReader{Reader: testutil.NewFakeClient(presets...), gets: map[string]int{}, errFor: map[string]error{}}
			resolver = NewChainResolver(reader, namespace)
			_, err := resolver.Resolve(ctx, v1alpha1.RulesetSpec{Name: "child", Extends: "no-enforcement"}, "")
			expectUnresolved(err, ReasonNoEnforcement, "no-enforcement")
		})
	})

	Describe("Ancestors", func() {
		BeforeEach(func() {
			presets = append(presets,
				preset("middle", v1alpha1.RulesetSpec{Name: "middle", Extends: "base"}),
				preset("dangling", v1alpha1.RulesetSpec{Name: "dangling", Extends: "missing"}),
			)
		})

		names := func(ps []*v1alpha1.RulesetPreset) []string {
			result := make([]string, 0, len(ps))
			for _, p := range ps {
				result = append(result, p.Name)
			}
			return result
		}

		It("returns the chain nearest parent first", func() {
			ancestors, err := resolver.Ancestors(ctx, "middle", "child")
			Expect(err).NotTo(HaveOccurred())
			Expect(names(ancestors)).To(Equal([]string{"middle", "base"}))
		})

		It("stops silently at missing presets and at self references", func() {
			ancestors, err := resolver.Ancestors(ctx, "dangling", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(names(ancestors)).To(Equal([]string{"dangling"}))

			ancestors, err = resolver.Ancestors(ctx, "middle", "middle")
			Expect(err).NotTo(HaveOccurred())
			Expect(ancestors).To(BeEmpty())
		})

		It("returns unexpected read errors", func() {
			reader.errFor["base"] = apierrors.NewServiceUnavailable("down")
			_, err := resolver.Ancestors(ctx, "middle", "")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Collect and Sync with extends", func() {
		var target *fakeRulesetTarget

		BeforeEach(func() {
			presets = append(presets,
				preset("child", v1alpha1.RulesetSpec{Name: "child", Extends: "base"}),
				preset("broken", v1alpha1.RulesetSpec{Name: "broken", Extends: "missing"}),
			)
			target = newFakeRulesetTarget()
		})

		It("resolves preset and inline rulesets", func() {
			desired, err := Collect(ctx, reader, namespace,
				[]corev1.LocalObjectReference{{Name: "child"}},
				[]v1alpha1.RulesetSpec{{Name: "inline", Extends: "base", Enforcement: v1alpha1.RulesetEnforcementEvaluate}})
			Expect(err).NotTo(HaveOccurred())
			Expect(desired).To(HaveLen(2))
			Expect(desired[0].Source).To(Equal(PresetSource("child")))
			Expect(desired[0].Spec.Rules.RequiredSignatures).To(Equal(new(true)))
			Expect(desired[1].Source).To(Equal(merge.InlineSource(0)))
			Expect(desired[1].Spec.Enforcement).To(Equal(v1alpha1.RulesetEnforcementEvaluate))
			Expect(desired[1].Spec.Target).To(Equal("tag"))
			Expect(desired[0].Unresolved).ToNot(HaveOccurred())
		})

		It("still fails completely if a directly referenced preset is missing", func() {
			_, err := Collect(ctx, reader, namespace, []corev1.LocalObjectReference{{Name: "missing"}}, nil)
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("keeps an unresolvable ruleset on GitHub while syncing all others", func() {
			desired, err := Collect(ctx, reader, namespace,
				[]corev1.LocalObjectReference{{Name: "child"}, {Name: "broken"}}, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(desired).To(HaveLen(2))
			Expect(desired[1].Unresolved).To(HaveOccurred())
			Expect(desired[1].Spec.Name).To(Equal("broken"))

			target.existing = []*github.RepositoryRuleset{
				{ID: new(int64(1)), Name: "broken"},
				{ID: new(int64(2)), Name: "orphan"},
			}
			err = Sync(ctx, target.target(), desired, SyncOptions{Resolver: &fakeRulesetResolver{}})

			unresolved, ok := errors.AsType[*UnresolvedError](err)
			Expect(ok).To(BeTrue(), "expected UnresolvedError, got %v", err)
			Expect(unresolved.Reason).To(Equal(ReasonParentNotFound))
			Expect(target.createdNames()).To(Equal([]string{"child"}))
			Expect(target.deleted).To(Equal(map[int64]string{2: "orphan"}))
			Expect(target.updated).To(BeEmpty())
			Expect(target.getCalls).To(BeEmpty())
		})

		It("treats an unresolved and a resolved ruleset with the same name as conflict", func() {
			_, err := Collect(ctx, reader, namespace,
				[]corev1.LocalObjectReference{{Name: "broken"}},
				[]v1alpha1.RulesetSpec{{Name: "broken", Extends: "base"}})
			_, ok := errors.AsType[*ConflictError](err)
			Expect(ok).To(BeTrue(), "expected ConflictError, got %v", err)
		})
	})
})

var _ = Describe("AwaitsPresetChange", func() {
	unresolved := func(reason UnresolvedReason) error {
		return &UnresolvedError{Ruleset: "r", Reason: reason}
	}

	DescribeTable("classifies errors",
		func(err error, expected bool) {
			Expect(AwaitsPresetChange(err)).To(Equal(expected))
		},
		Entry("nil", nil, false),
		Entry("generic error", errors.New("boom"), false),
		Entry("missing parent", unresolved(ReasonParentNotFound), true),
		Entry("cycle", unresolved(ReasonExtendsCycle), true),
		Entry("too deep", unresolved(ReasonExtendsTooDeep), true),
		Entry("missing enforcement", unresolved(ReasonNoEnforcement), true),
		Entry("transient read error", unresolved(ReasonParentUnavailable), false),
		Entry("wrapped and joined unresolved errors",
			fmt.Errorf("sync: %w", errors.Join(unresolved(ReasonParentNotFound), errors.Join(unresolved(ReasonExtendsCycle)))), true),
		Entry("unresolved joined with a transient read error",
			errors.Join(unresolved(ReasonParentNotFound), unresolved(ReasonParentUnavailable)), false),
		Entry("unresolved joined with another error",
			errors.Join(unresolved(ReasonParentNotFound), errors.New("webhook failed")), false),
		Entry("empty join wrapper", fmt.Errorf("x: %w", errors.Join()), false),
	)
})
