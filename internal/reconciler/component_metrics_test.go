package reconciler

import (
	"context"
	"errors"
	"time"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/conditions"
	"github.com/Interhyp/git-hubby/internal/reconciler/common/rulesets"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var _ = Describe("Component metrics", func() {
	observations := func(component conditions.ConditionType, outcome string) float64 {
		return gatheredValue("git_hubby_component_reconcile_duration_seconds",
			map[string]string{"kind": "Organization", "component": string(component), "outcome": outcome})
	}
	skippedCount := func(component conditions.ConditionType) float64 {
		return gatheredValue("git_hubby_component_reconcile_skipped_total",
			map[string]string{"kind": "Organization", "component": string(component)})
	}

	DescribeTable("classifies the outcome of a component",
		func(err error, outcome string) {
			Expect(componentOutcome(err)).To(Equal(outcome))
		},
		Entry("success", nil, componentOutcomeSuccess),
		Entry("error", errors.New("boom"), componentOutcomeError),
		Entry("user action", &rulesets.ConflictError{Name: "a"}, componentOutcomeUserAction),
		Entry("retry later", &RetryLaterError{Reason: "in progress", RequeueAfter: time.Minute}, componentOutcomeRetryLater),
		Entry("stop", &StopReconciliation{Message: "archived"}, componentOutcomeStopped),
	)

	It("observes every component and counts the skipped ones", func() {
		const (
			succeeding conditions.ConditionType = "MetricsTestSucceeding"
			stopping   conditions.ConditionType = "MetricsTestStopping"
			skipped    conditions.ConditionType = "MetricsTestSkipped"
		)
		successBefore := observations(succeeding, componentOutcomeSuccess)
		stoppedBefore := observations(stopping, componentOutcomeStopped)
		skippedBefore := skippedCount(skipped)

		rec := &ReconciliationExecutor[*githubv1alpha1.Organization]{
			Reconciler: &MockReconciler{
				k8s: Kubernetes[*githubv1alpha1.Organization]{Resource: &githubv1alpha1.Organization{}},
				reconciliations: []ParallelReconciliationGroup{
					{
						{Condition: succeeding, Function: func(context.Context) error { return nil }},
						{Condition: stopping, Function: func(context.Context) error { return &StopReconciliation{Message: "archived"} }},
					},
					{
						{Condition: skipped, Function: func(context.Context) error { return nil }},
					},
				},
			},
		}

		_, err := rec.runReconciliations(context.Background())
		Expect(err).NotTo(HaveOccurred())

		Expect(observations(succeeding, componentOutcomeSuccess)).To(Equal(successBefore + 1))
		Expect(observations(stopping, componentOutcomeStopped)).To(Equal(stoppedBefore + 1))
		Expect(observations(skipped, componentOutcomeSuccess)).To(BeZero())
		Expect(skippedCount(skipped)).To(Equal(skippedBefore + 1))
	})
})

// gatheredValue returns the value of the counter or the sample count of the histogram with exactly the given labels
// from the controller-runtime registry, or 0 if it was not observed yet.
func gatheredValue(name string, labels map[string]string) float64 {
	families, err := ctrlmetrics.Registry.Gather()
	Expect(err).NotTo(HaveOccurred())
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
	metrics:
		for _, metric := range family.GetMetric() {
			if len(metric.GetLabel()) != len(labels) {
				continue
			}
			for _, label := range metric.GetLabel() {
				if labels[label.GetName()] != label.GetValue() {
					continue metrics
				}
			}
			if metric.GetHistogram() != nil {
				return float64(metric.GetHistogram().GetSampleCount())
			}
			return metric.GetCounter().GetValue()
		}
	}
	return 0
}
