package v1alpha1

import (
	"github.com/Interhyp/git-hubby/internal/conditions"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (in *Repository) GetConditions() *[]metav1.Condition {
	if in == nil {
		return nil
	}
	return &in.Status.Conditions
}

func (in *Repository) GetTypeRepresentation() string {
	return "Repository"
}

func (in *Repository) IsHealthy() bool {
	if in == nil {
		return false
	}
	readyCondition := meta.FindStatusCondition(in.Status.Conditions, "Ready")
	return readyCondition != nil && readyCondition.Status == metav1.ConditionTrue
}

func (in *Repository) GetObservedGeneration() int64 {
	if in == nil {
		return 0
	}
	readyCondition := meta.FindStatusCondition(in.Status.Conditions, string(conditions.TypeReady))
	if readyCondition == nil {
		return 0
	}
	return readyCondition.ObservedGeneration
}

func (in *Repository) GetObservedSubResourceGenerations() map[string]int64 {
	if in == nil {
		return nil
	}
	return in.Status.ObservedSubResourceGenerations
}

func (in *Repository) SetObservedSubResourceGeneration(new map[string]int64) {
	if in == nil {
		return
	}
	in.Status.ObservedSubResourceGenerations = new
}

func (c *CustomPropertyValue) GetValue() *string {
	if c == nil {
		return nil
	}
	return c.Value
}

func (c *CustomPropertyValue) GetValues() []string {
	if c == nil {
		return nil
	}
	return c.Values
}
