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

// AutolinksPresetSpec defines the desired state of AutolinksPreset.
// Autolinks automatically convert references to external resources (like issue trackers) into clickable links.
// See: https://docs.github.com/en/rest/repos/autolinks
type AutolinksPresetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// AutolinkList is a list of autolink configurations to create in repositories.
	// Each autolink defines a prefix that triggers link generation and a URL template.
	AutolinkList []Autolink `json:"autolinks,omitempty"`
}

// Autolink defines an automatic link reference for external resources.
// When a reference matching KeyPrefix is found in issues, pull requests, or commit messages,
// GitHub automatically converts it to a clickable link using the URLTemplate.
// See: https://docs.github.com/en/rest/repos/autolinks
type Autolink struct {
	// KeyPrefix is the text prefix that triggers autolink creation.
	// When text starts with this prefix followed by a reference, it becomes a link.
	// Examples: "JIRA-", "TICKET-", "BUG-"
	// +kubebuilder:validation:MaxLength=20
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9][a-zA-Z0-9-]{0,19}$`
	// +kubebuilder:validation:Type=string
	KeyPrefix string `json:"keyPrefix"`

	// URLTemplate is the URL pattern used to generate links.
	// Use <num> as a placeholder for the reference number/ID.
	// Example: "https://jira.example.com/browse/<num>" converts "JIRA-123" to "https://jira.example.com/browse/123"
	// +kubebuilder:validation:MaxLength=200
	// +kubebuilder:validation:Type=string
	URLTemplate string `json:"urlTemplate"`

	// IsAlphanumeric determines whether the reference must be alphanumeric.
	// - true: the <num> parameter of the url_template matches alphanumeric characters `A-Z` (case insensitive), `0-9`, and `-`
	// - false: reference only matches numeric characters.
	// +default:value=false
	// +kubebuilder:validation:Type=boolean
	IsAlphanumeric bool `json:"isAlphanumeric"`
}

// AutolinksPresetStatus defines the observed state of AutolinksPreset.
type AutolinksPresetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the AutolinksPreset resource.
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

// AutolinksPreset is the Schema for the autolinkspresets API
type AutolinksPreset struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of AutolinksPreset
	// +required
	Spec AutolinksPresetSpec `json:"spec"`

	// status defines the observed state of AutolinksPreset
	// +optional
	Status AutolinksPresetStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// AutolinksPresetList contains a list of AutolinksPreset
type AutolinksPresetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []AutolinksPreset `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &AutolinksPreset{}, &AutolinksPresetList{})
		return nil
	})
}
