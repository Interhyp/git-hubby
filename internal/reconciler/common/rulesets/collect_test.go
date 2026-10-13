package rulesets

import (
	"context"
	"errors"
	"strings"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/test/testutil"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Collect", func() {
	const namespace = "test-ns"

	var (
		ctx        context.Context
		logLines   func() []string
		presets    []client.Object
		presetRefs []corev1.LocalObjectReference
		inline     []v1alpha1.RulesetSpec
		result     []Desired
		err        error
	)

	newSpec := func(name string, enforcement v1alpha1.RulesetEnforcement) v1alpha1.RulesetSpec {
		return v1alpha1.RulesetSpec{
			Name:        name,
			Target:      "branch",
			Enforcement: enforcement,
			Conditions: &v1alpha1.RulesetConditions{
				RefName: &v1alpha1.RefNameCondition{Include: []string{"~DEFAULT_BRANCH"}},
			},
			Rules: v1alpha1.RulesetRules{
				Deletion: new(true),
				PullRequest: &v1alpha1.PullRequestRule{
					RequiredReviewers: []v1alpha1.RequiredPullRequestReviewer{
						{Reviewer: v1alpha1.PullRequestReviewerEntity{Slug: new("team-a"), Type: "Team"}},
					},
				},
			},
		}
	}

	newPreset := func(objName string, spec v1alpha1.RulesetSpec) *v1alpha1.RulesetPreset {
		return &v1alpha1.RulesetPreset{
			ObjectMeta: metav1.ObjectMeta{Name: objName, Namespace: namespace},
			Spec:       spec,
		}
	}

	logContains := func(s string) bool {
		for _, l := range logLines() {
			if strings.Contains(l, s) {
				return true
			}
		}
		return false
	}

	BeforeEach(func() {
		ctx, logLines = testutil.CapturingContext()
		presets = nil
		presetRefs = nil
		inline = nil
	})

	JustBeforeEach(func() {
		result, err = Collect(ctx, testutil.NewFakeClient(presets...), namespace, presetRefs, inline)
	})

	Context("with presets only", func() {
		BeforeEach(func() {
			presets = []client.Object{
				newPreset("preset-a", newSpec("ruleset-a", "active")),
				newPreset("preset-b", newSpec("ruleset-b", "evaluate")),
			}
			presetRefs = []corev1.LocalObjectReference{{Name: "preset-a"}, {Name: "preset-b"}}
		})

		It("returns all preset rulesets with their sources", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(2))
			Expect(result[0].Source).To(Equal("rulesetPreset/preset-a"))
			Expect(result[0].Spec.Name).To(Equal("ruleset-a"))
			Expect(result[1].Source).To(Equal("rulesetPreset/preset-b"))
			Expect(result[1].Spec.Name).To(Equal("ruleset-b"))
		})
	})

	Context("with inline rulesets only", func() {
		BeforeEach(func() {
			inline = []v1alpha1.RulesetSpec{newSpec("inline-a", "active"), newSpec("inline-b", "disabled")}
		})

		It("returns all inline rulesets with their sources", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(2))
			Expect(result[0].Source).To(Equal("inline[0]"))
			Expect(result[0].Spec.Name).To(Equal("inline-a"))
			Expect(result[1].Source).To(Equal("inline[1]"))
			Expect(result[1].Spec.Name).To(Equal("inline-b"))
		})
	})

	Context("with neither presets nor inline rulesets", func() {
		It("returns an empty result", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeEmpty())
		})
	})

	Context("with both presets and inline rulesets", func() {
		BeforeEach(func() {
			presets = []client.Object{newPreset("preset-a", newSpec("ruleset-a", "active"))}
			presetRefs = []corev1.LocalObjectReference{{Name: "preset-a"}}
			inline = []v1alpha1.RulesetSpec{newSpec("inline-a", "active")}
		})

		It("returns the union", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(2))
			Expect(result[0].Source).To(Equal("rulesetPreset/preset-a"))
			Expect(result[1].Source).To(Equal("inline[0]"))
		})
	})

	Context("with a name collision and identical content", func() {
		BeforeEach(func() {
			presets = []client.Object{newPreset("preset-a", newSpec("shared", "active"))}
			presetRefs = []corev1.LocalObjectReference{{Name: "preset-a"}}
			inline = []v1alpha1.RulesetSpec{newSpec("shared", "active")}
		})

		It("returns the ruleset once without error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0].Source).To(Equal("rulesetPreset/preset-a"))
			Expect(result[0].Spec.Name).To(Equal("shared"))
		})

		It("logs a warning", func() {
			Expect(logContains("WARNING: duplicate ruleset definition with identical content")).To(BeTrue())
		})
	})

	Context("with a name collision between an inline ruleset and a preset with different content", func() {
		BeforeEach(func() {
			presets = []client.Object{newPreset("preset-a", newSpec("shared", "active"))}
			presetRefs = []corev1.LocalObjectReference{{Name: "preset-a"}}
			inline = []v1alpha1.RulesetSpec{newSpec("shared", "evaluate")}
		})

		It("returns a ConflictError naming both sources", func() {
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
			var conflictErr *ConflictError
			Expect(errors.As(err, &conflictErr)).To(BeTrue())
			Expect(conflictErr.Name).To(Equal("shared"))
			Expect(conflictErr.Sources).To(ConsistOf("rulesetPreset/preset-a", "inline[0]"))
			Expect(err.Error()).To(ContainSubstring("rulesetPreset/preset-a"))
			Expect(err.Error()).To(ContainSubstring("inline[0]"))
		})

		It("logs an error", func() {
			Expect(logContains("conflicting ruleset definitions")).To(BeTrue())
		})
	})

	Context("with a name collision between two presets with different content", func() {
		BeforeEach(func() {
			presets = []client.Object{
				newPreset("preset-a", newSpec("shared", "active")),
				newPreset("preset-b", newSpec("shared", "disabled")),
			}
			presetRefs = []corev1.LocalObjectReference{{Name: "preset-a"}, {Name: "preset-b"}}
		})

		It("returns a ConflictError naming both presets", func() {
			var conflictErr *ConflictError
			Expect(errors.As(err, &conflictErr)).To(BeTrue())
			Expect(conflictErr.Name).To(Equal("shared"))
			Expect(conflictErr.Sources).To(ConsistOf("rulesetPreset/preset-a", "rulesetPreset/preset-b"))
		})
	})

	Context("with multiple conflicting names", func() {
		BeforeEach(func() {
			presets = []client.Object{
				newPreset("preset-a", newSpec("one", "active")),
				newPreset("preset-b", newSpec("two", "active")),
			}
			presetRefs = []corev1.LocalObjectReference{{Name: "preset-a"}, {Name: "preset-b"}}
			inline = []v1alpha1.RulesetSpec{newSpec("one", "evaluate"), newSpec("two", "evaluate")}
		})

		It("reports every conflict", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`"one"`))
			Expect(err.Error()).To(ContainSubstring(`"two"`))
		})
	})

	Context("when a referenced preset does not exist", func() {
		BeforeEach(func() {
			presetRefs = []corev1.LocalObjectReference{{Name: "missing"}}
			inline = []v1alpha1.RulesetSpec{newSpec("inline-a", "active")}
		})

		It("returns a not found error", func() {
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("failed to get ruleset preset missing"))
			Expect(result).To(BeNil())
		})
	})

	Context("when the returned specs are modified", func() {
		BeforeEach(func() {
			inline = []v1alpha1.RulesetSpec{newSpec("inline-a", "active")}
		})

		It("does not modify the original inline spec", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
			result[0].Spec.Rules.PullRequest.RequiredReviewers = nil
			result[0].Spec.Conditions.RefName.Include[0] = "changed"
			Expect(inline[0].Rules.PullRequest.RequiredReviewers).To(HaveLen(1))
			Expect(inline[0].Conditions.RefName.Include[0]).To(Equal("~DEFAULT_BRANCH"))
		})
	})
})
