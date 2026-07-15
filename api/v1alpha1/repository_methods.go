package v1alpha1

import (
	"github.com/Interhyp/git-hubby/internal/conditions"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *Repository) GetConditions() *[]metav1.Condition {
	if r == nil {
		return nil
	}
	return &r.Status.Conditions
}

func (r *Repository) GetTypeRepresentation() string {
	return "Repository"
}

func (r *Repository) IsHealthy() bool {
	if r == nil {
		return false
	}
	readyCondition := meta.FindStatusCondition(r.Status.Conditions, "Ready")
	return readyCondition != nil && readyCondition.Status == metav1.ConditionTrue
}

func (r *Repository) GetObservedGeneration() int64 {
	if r == nil {
		return 0
	}
	readyCondition := meta.FindStatusCondition(r.Status.Conditions, string(conditions.TypeReady))
	if readyCondition == nil {
		return 0
	}
	return readyCondition.ObservedGeneration
}

func (r *Repository) GetObservedSubResourceGenerations() map[string]int64 {
	if r == nil {
		return nil
	}
	return r.Status.ObservedSubResourceGenerations
}

func (r *Repository) SetObservedSubResourceGeneration(new map[string]int64) {
	if r == nil {
		return
	}
	r.Status.ObservedSubResourceGenerations = new
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
