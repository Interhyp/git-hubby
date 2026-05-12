package v1alpha1

import (
	"github.com/Interhyp/git-hubby/internal/conditions"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (in *Team) GetConditions() *[]metav1.Condition {
	if in == nil {
		return nil
	}
	return &in.Status.Conditions
}

func (in *Team) GetTypeRepresentation() string {
	return "Team"
}

func (in *Team) IsIDPTeam() bool {
	if in == nil {
		return false
	}
	return in.Spec.IDPGroup != nil
}

func (in *Team) IsHealthy() bool {
	if in == nil {
		return false
	}
	readyCondition := meta.FindStatusCondition(in.Status.Conditions, "Ready")
	return readyCondition != nil && readyCondition.Status == metav1.ConditionTrue
}

func (in *Team) GetObservedGeneration() int64 {
	if in == nil {
		return 0
	}
	readyCondition := meta.FindStatusCondition(in.Status.Conditions, string(conditions.TypeReady))
	if readyCondition == nil {
		return 0
	}
	return readyCondition.ObservedGeneration
}

func (in *Team) GetObservedSubResourceGenerations() map[string]int64 {
	return nil
}

func (in *Team) SetObservedSubResourceGeneration(_ map[string]int64) {}
