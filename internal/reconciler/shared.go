package reconciler

import (
	"github.com/Interhyp/git-hubby/api/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v2 "k8s.io/client-go/applyconfigurations/meta/v1"
)

func IsActionsDisabledForOrgSpec(org *v1alpha1.Organization) bool {
	return org.Spec.ActionsSettings.EnabledRepositories == nil ||
		*org.Spec.ActionsSettings.EnabledRepositories == "none"
}

// ConditionToApplyConfig converts a metav1.Condition to a ConditionApplyConfiguration for SSA.
func ConditionToApplyConfig(c v1.Condition) *v2.ConditionApplyConfiguration {
	return v2.Condition().
		WithType(c.Type).
		WithStatus(c.Status).
		WithObservedGeneration(c.ObservedGeneration).
		WithLastTransitionTime(c.LastTransitionTime).
		WithReason(c.Reason).
		WithMessage(c.Message)
}

// ConditionsToApplyConfigs converts a slice of metav1.Condition to apply configurations.
func ConditionsToApplyConfigs(conditions []v1.Condition) []*v2.ConditionApplyConfiguration {
	result := make([]*v2.ConditionApplyConfiguration, len(conditions))
	for i, c := range conditions {
		result[i] = ConditionToApplyConfig(c)
	}
	return result
}
