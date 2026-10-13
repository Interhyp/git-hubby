package reconciler

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/conditions"
	"github.com/go-logr/logr/funcr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("CollectDesiredRulesets", func() {
	const namespace = "test-ns"

	var (
		ctx        context.Context
		k8sClient  client.Client
		presets    []client.Object
		presetRefs []corev1.LocalObjectReference
		inline     []v1alpha1.RulesetSpec
		result     []DesiredRuleset
		err        error
		logLines   []string
		logMu      sync.Mutex
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

	BeforeEach(func() {
		logLines = nil
		logger := funcr.New(func(prefix, args string) {
			logMu.Lock()
			defer logMu.Unlock()
			logLines = append(logLines, args)
		}, funcr.Options{Verbosity: 1})
		ctx = logPkg.IntoContext(context.Background(), logger)
		presets = nil
		presetRefs = nil
		inline = nil
	})

	JustBeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())
		k8sClient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(presets...).Build()
		result, err = CollectDesiredRulesets(ctx, k8sClient, namespace, presetRefs, inline)
	})

	logContains := func(s string) bool {
		logMu.Lock()
		defer logMu.Unlock()
		for _, l := range logLines {
			if strings.Contains(l, s) {
				return true
			}
		}
		return false
	}

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

		It("returns a RulesetConflictError naming both sources", func() {
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
			var conflictErr *RulesetConflictError
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

		It("returns a RulesetConflictError naming both presets", func() {
			var conflictErr *RulesetConflictError
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

var _ = Describe("RulesetConflictError in status conditions", func() {
	It("sets RulesetsSynced to False with the conflict error in the message", func() {
		conflictErr := &RulesetConflictError{Name: "shared", Sources: []string{"rulesetPreset/preset-a", "inline[0]"}}
		condition := conditionFromResult(reconciliationResult{
			Condition:  conditions.TypeRulesetsSynced,
			Error:      errors.Join(conflictErr),
			FinishedAt: time.Now(),
		})
		Expect(condition.Type).To(Equal(string(conditions.TypeRulesetsSynced)))
		Expect(condition.Status).To(Equal(metav1.ConditionFalse))
		Expect(condition.Reason).To(Equal(conditions.ReasonSyncFailed))
		Expect(condition.Message).To(ContainSubstring(`conflicting ruleset definitions for ruleset "shared"`))
		Expect(condition.Message).To(ContainSubstring("rulesetPreset/preset-a"))
		Expect(condition.Message).To(ContainSubstring("inline[0]"))
	})
})
