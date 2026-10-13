package conditions

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ToApplyConfig", func() {
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

		result := ToApplyConfig(c)

		Expect(result).NotTo(BeNil())
		Expect(*result.Type).To(Equal("Ready"))
		Expect(*result.Status).To(Equal(metav1.ConditionTrue))
		Expect(*result.ObservedGeneration).To(Equal(int64(7)))
		Expect(*result.LastTransitionTime).To(Equal(t))
		Expect(*result.Reason).To(Equal("Synced"))
		Expect(*result.Message).To(Equal("all good"))
	})
})

var _ = Describe("ToApplyConfigs", func() {
	It("returns an empty slice for an empty input", func() {
		Expect(ToApplyConfigs(nil)).To(BeEmpty())
	})

	It("converts every condition in the slice", func() {
		conditions := []metav1.Condition{
			{Type: "Ready", Status: metav1.ConditionTrue, Reason: "OK", Message: "fine"},
			{Type: "GitHubSynced", Status: metav1.ConditionFalse, Reason: "Error", Message: "boom"},
		}

		result := ToApplyConfigs(conditions)

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
		result := ToApplyConfigs(conditions)
		Expect(*result[0].Type).To(Equal("A"))
		Expect(*result[1].Type).To(Equal("B"))
		Expect(*result[2].Type).To(Equal("C"))
	})
})
