package reconciler

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IsActionsDisabledForOrgSpec", func() {
	It("returns true when EnabledRepositories is nil", func() {
		org := &v1alpha1.Organization{}
		Expect(IsActionsDisabledForOrgSpec(org)).To(BeTrue())
	})

	It("returns true when EnabledRepositories is \"none\"", func() {
		none := "none"
		org := &v1alpha1.Organization{
			Spec: v1alpha1.OrganizationSpec{
				ActionsSettings: v1alpha1.ActionsSettings{EnabledRepositories: &none},
			},
		}
		Expect(IsActionsDisabledForOrgSpec(org)).To(BeTrue())
	})

	It("returns false when EnabledRepositories is \"all\"", func() {
		all := "all"
		org := &v1alpha1.Organization{
			Spec: v1alpha1.OrganizationSpec{
				ActionsSettings: v1alpha1.ActionsSettings{EnabledRepositories: &all},
			},
		}
		Expect(IsActionsDisabledForOrgSpec(org)).To(BeFalse())
	})

	It("returns false when EnabledRepositories is \"selected\"", func() {
		selected := "selected"
		org := &v1alpha1.Organization{
			Spec: v1alpha1.OrganizationSpec{
				ActionsSettings: v1alpha1.ActionsSettings{EnabledRepositories: &selected},
			},
		}
		Expect(IsActionsDisabledForOrgSpec(org)).To(BeFalse())
	})
})

var _ = Describe("ConditionToApplyConfig", func() {
	It("copies all fields from the source condition", func() {
		t := metav1.Now()
		c := metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: 7,
			LastTransitionTime: t,
			Reason:             "Synced",
			Message:            "all good",
		}

		result := ConditionToApplyConfig(c)

		Expect(result).NotTo(BeNil())
		Expect(*result.Type).To(Equal("Ready"))
		Expect(*result.Status).To(Equal(metav1.ConditionTrue))
		Expect(*result.ObservedGeneration).To(Equal(int64(7)))
		Expect(*result.LastTransitionTime).To(Equal(t))
		Expect(*result.Reason).To(Equal("Synced"))
		Expect(*result.Message).To(Equal("all good"))
	})
})

var _ = Describe("ConditionsToApplyConfigs", func() {
	It("returns an empty slice for an empty input", func() {
		result := ConditionsToApplyConfigs(nil)
		Expect(result).To(BeEmpty())
	})

	It("converts every condition in the slice", func() {
		conditions := []metav1.Condition{
			{Type: "Ready", Status: metav1.ConditionTrue, Reason: "OK", Message: "fine"},
			{Type: "GitHubSynced", Status: metav1.ConditionFalse, Reason: "Error", Message: "boom"},
		}

		result := ConditionsToApplyConfigs(conditions)

		Expect(result).To(HaveLen(2))
		Expect(*result[0].Type).To(Equal("Ready"))
		Expect(*result[0].Status).To(Equal(metav1.ConditionTrue))
		Expect(*result[1].Type).To(Equal("GitHubSynced"))
		Expect(*result[1].Status).To(Equal(metav1.ConditionFalse))
		Expect(*result[1].Reason).To(Equal("Error"))
	})

	It("preserves order", func() {
		conditions := []metav1.Condition{
			{Type: "A", Status: metav1.ConditionTrue, Reason: "r"},
			{Type: "B", Status: metav1.ConditionFalse, Reason: "r"},
			{Type: "C", Status: metav1.ConditionUnknown, Reason: "r"},
		}
		result := ConditionsToApplyConfigs(conditions)
		Expect(*result[0].Type).To(Equal("A"))
		Expect(*result[1].Type).To(Equal("B"))
		Expect(*result[2].Type).To(Equal("C"))
	})
})
