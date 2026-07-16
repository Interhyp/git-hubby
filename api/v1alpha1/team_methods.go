package v1alpha1

import (
	"github.com/Interhyp/git-hubby/internal/conditions"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (t *Team) GetConditions() *[]metav1.Condition {
	if t == nil {
		return nil
	}
	return &t.Status.Conditions
}

func (t *Team) GetTypeRepresentation() string {
	return "Team"
}

func (t *Team) IsIDPTeam() bool {
	if t == nil {
		return false
	}
	return t.Spec.IDPGroup != nil
}

func (t *Team) IsHealthy() bool {
	if t == nil {
		return false
	}
	readyCondition := meta.FindStatusCondition(t.Status.Conditions, "Ready")
	return readyCondition != nil && readyCondition.Status == metav1.ConditionTrue
}

func (t *Team) GetObservedGeneration() int64 {
	if t == nil {
		return 0
	}
	readyCondition := meta.FindStatusCondition(t.Status.Conditions, string(conditions.TypeReady))
	if readyCondition == nil {
		return 0
	}
	return readyCondition.ObservedGeneration
}

func (t *Team) GetObservedSubResourceGenerations() map[string]int64 {
	return nil
}

func (t *Team) SetObservedSubResourceGeneration(_ map[string]int64) {}
