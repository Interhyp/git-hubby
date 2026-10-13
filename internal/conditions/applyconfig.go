package conditions

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
)

// ToApplyConfig converts a metav1.Condition to a ConditionApplyConfiguration for server-side apply.
func ToApplyConfig(c metav1.Condition) *metav1ac.ConditionApplyConfiguration {
	return metav1ac.Condition().
		WithType(c.Type).
		WithStatus(c.Status).
		WithObservedGeneration(c.ObservedGeneration).
		WithLastTransitionTime(c.LastTransitionTime).
		WithReason(c.Reason).
		WithMessage(c.Message)
}

// ToApplyConfigs converts a slice of metav1.Condition to apply configurations.
func ToApplyConfigs(conditions []metav1.Condition) []*metav1ac.ConditionApplyConfiguration {
	result := make([]*metav1ac.ConditionApplyConfiguration, len(conditions))
	for i, c := range conditions {
		result[i] = ToApplyConfig(c)
	}
	return result
}
