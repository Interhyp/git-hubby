<<<<<<< HEAD
=======
/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

>>>>>>> tmp-original-30-06-26-04-09
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WebhookIgnorePresetSpec defines the desired state of WebhookIgnorePreset.
// WebhookIgnorePresets allow you to exclude certain webhooks from being created,
// even if they are referenced in a repository's WebhookPresetList.
// This is useful for globally excluding webhooks based on URL patterns.
type WebhookIgnorePresetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// IgnoreURLRegex is a regular expression pattern to match against webhook payload URLs.
	// Webhooks with URLs matching this pattern will not be created, even if they are
	// referenced in a repository's WebhookPresetList.
	// Example: "^https://deprecated\\.example\\.com/.*" to ignore all webhooks to deprecated.example.com
	// +optional
	IgnoreURLRegex *string `json:"ignoreURLRegex,omitempty"`
}

// WebhookIgnorePresetStatus defines the observed state of WebhookIgnorePreset.
type WebhookIgnorePresetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the WebhookIgnorePreset resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource

// WebhookIgnorePreset is the Schema for the webhookignorepresets API
type WebhookIgnorePreset struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of WebhookIgnorePreset
	// +required
	Spec WebhookIgnorePresetSpec `json:"spec"`

	// status defines the observed state of WebhookIgnorePreset
	// +optional
	Status WebhookIgnorePresetStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// WebhookIgnorePresetList contains a list of WebhookIgnorePreset
type WebhookIgnorePresetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []WebhookIgnorePreset `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &WebhookIgnorePreset{}, &WebhookIgnorePresetList{})
		return nil
	})
}
