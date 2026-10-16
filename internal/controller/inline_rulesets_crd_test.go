package controller

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
)

// These tests run against the envtest API server (see suite_test.go). The CRDs being installed successfully
// in BeforeSuite also verifies that the inline rulesets stay within the CEL cost budget.
var _ = Describe("Inline rulesets CRD validation", func() {
	const namespace = "default"

	inlineRuleset := func(name string) githubv1alpha1.RulesetSpec {
		return githubv1alpha1.RulesetSpec{
			Name:        "ruleset-" + name,
			Target:      "branch",
			Enforcement: githubv1alpha1.RulesetEnforcement("active"),
			Rules:       githubv1alpha1.RulesetRules{Deletion: new(true)},
		}
	}

	inlineRulesets := func(names ...string) []githubv1alpha1.RulesetSpec {
		result := make([]githubv1alpha1.RulesetSpec, 0, len(names))
		for _, n := range names {
			result = append(result, inlineRuleset(n))
		}
		return result
	}

	newOrg := func(name string, rulesets []githubv1alpha1.RulesetSpec) *githubv1alpha1.Organization {
		return &githubv1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: githubv1alpha1.OrganizationSpec{
				Name:                       name,
				Description:                "test",
				GitHubAppInstallationId:    new(int64(1)),
				CodeSecurityConfigurations: []githubv1alpha1.AttachableCodeSecurityConfigurationRef{},
				Rulesets:                   rulesets,
			},
		}
	}

	newRepo := func(name string, rulesets []githubv1alpha1.RulesetSpec) *githubv1alpha1.Repository {
		return &githubv1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: githubv1alpha1.RepositorySpec{
				Name:            name,
				OrganizationRef: githubv1alpha1.OrganizationRef{Name: "some-org"},
				Rulesets:        rulesets,
			},
		}
	}

	expectInvalid := func(err error, msg string) {
		ExpectWithOffset(1, err).To(HaveOccurred())
		ExpectWithOffset(1, apierrors.IsInvalid(err)).To(BeTrue(), fmt.Sprintf("expected invalid error, got: %v", err))
		ExpectWithOffset(1, err.Error()).To(ContainSubstring(msg))
	}

	Context("Organization", func() {
		It("accepts up to 10 inline rulesets with unique names", func() {
			org := newOrg("inline-ok", inlineRulesets("a", "b", "c", "d", "e", "f", "g", "h", "i", "j"))
			Expect(k8sClient.Create(ctx, org)).To(Succeed())
			DeferCleanup(func() { Expect(k8sClient.Delete(ctx, org)).To(Succeed()) })
		})

		It("rejects duplicate inline ruleset names", func() {
			err := k8sClient.Create(ctx, newOrg("inline-dup", inlineRulesets("a", "a")))
			expectInvalid(err, "ruleset names must be unique")
		})

		It("rejects more than 10 inline rulesets", func() {
			err := k8sClient.Create(ctx, newOrg("inline-too-many", inlineRulesets("a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k")))
			expectInvalid(err, "spec.rulesets")
		})
	})

	Context("Repository", func() {
		It("accepts up to 10 inline rulesets with unique names", func() {
			repo := newRepo("inline-ok", inlineRulesets("a", "b", "c", "d", "e", "f", "g", "h", "i", "j"))
			Expect(k8sClient.Create(ctx, repo)).To(Succeed())
			DeferCleanup(func() { Expect(k8sClient.Delete(ctx, repo)).To(Succeed()) })
		})

		It("rejects duplicate inline ruleset names", func() {
			err := k8sClient.Create(ctx, newRepo("inline-dup", inlineRulesets("a", "a")))
			expectInvalid(err, "ruleset names must be unique")
		})

		It("rejects more than 10 inline rulesets", func() {
			err := k8sClient.Create(ctx, newRepo("inline-too-many", inlineRulesets("a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k")))
			expectInvalid(err, "spec.rulesets")
		})
	})

	Context("RulesetPreset extends", func() {
		newPreset := func(name string, spec githubv1alpha1.RulesetSpec) *githubv1alpha1.RulesetPreset {
			return &githubv1alpha1.RulesetPreset{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}, Spec: spec}
		}

		It("accepts a preset that only sets extends and name", func() {
			preset := newPreset("extends-only", githubv1alpha1.RulesetSpec{Name: "child", Extends: "base"})
			Expect(k8sClient.Create(ctx, preset)).To(Succeed())
			DeferCleanup(func() { Expect(k8sClient.Delete(ctx, preset)).To(Succeed()) })
			Expect(preset.Spec.Target).To(BeEmpty(), "target must not be defaulted, it is inherited")
		})

		It("requires enforcement and rules without extends", func() {
			err := k8sClient.Create(ctx, newPreset("no-enforcement", githubv1alpha1.RulesetSpec{Name: "x"}))
			expectInvalid(err, "enforcement is required unless extends is set")
			expectInvalid(err, "rules is required unless extends is set")
		})

		It("rejects a preset extending itself", func() {
			err := k8sClient.Create(ctx, newPreset("self", githubv1alpha1.RulesetSpec{Name: "x", Extends: "self"}))
			expectInvalid(err, "a RulesetPreset cannot extend itself")
		})

		It("accepts inline rulesets with extends", func() {
			repo := newRepo("inline-extends", []githubv1alpha1.RulesetSpec{{Name: "child", Extends: "base"}})
			Expect(k8sClient.Create(ctx, repo)).To(Succeed())
			DeferCleanup(func() { Expect(k8sClient.Delete(ctx, repo)).To(Succeed()) })
		})
	})
})
