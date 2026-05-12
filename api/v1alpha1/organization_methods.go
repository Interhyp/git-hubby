package v1alpha1

import (
	"github.com/Interhyp/git-hubby/internal/conditions"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (in *Organization) GetTypeRepresentation() string {
	return "Organization"
}

func (in *Organization) GetConditions() *[]metav1.Condition {
	if in == nil {
		return nil
	}
	return &in.Status.Conditions
}

func (in *Organization) IsHealthy() bool {
	if in == nil {
		return false
	}
	readyCondition := meta.FindStatusCondition(in.Status.Conditions, string(conditions.TypeReady))
	return readyCondition != nil && readyCondition.Status == metav1.ConditionTrue
}

func (in *Organization) GetObservedGeneration() int64 {
	if in == nil {
		return 0
	}
	readyCondition := meta.FindStatusCondition(in.Status.Conditions, string(conditions.TypeReady))
	if readyCondition == nil {
		return 0
	}
	return readyCondition.ObservedGeneration
}

func (in *Organization) GetObservedSubResourceGenerations() map[string]int64 {
	if in == nil {
		return nil
	}
	return in.Status.ObservedSubResourceGenerations
}

func (in *Organization) SetObservedSubResourceGeneration(new map[string]int64) {
	if in == nil {
		return
	}
	in.Status.ObservedSubResourceGenerations = new
}

func (o *OrgCustomPropertyDefaultValue) GetValue() *string {
	if o == nil {
		return nil
	}
	return o.Value
}

func (o *OrgCustomPropertyDefaultValue) GetValues() []string {
	if o == nil {
		return nil
	}
	return o.Values
}
