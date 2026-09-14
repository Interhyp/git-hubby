package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WebhookPresetSpec defines the desired state of WebhookPreset.
// Webhooks allow external services to be notified when certain events occur in a repository.
// See: https://docs.github.com/en/rest/webhooks/repos
type WebhookPresetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// PayloadURL is the URL that will receive the webhook POST requests.
	// Must be a publicly accessible HTTP or HTTPS endpoint.
	// GitHub will send HTTP POST requests to this URL when subscribed events occur.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:Pattern=`^https?://[a-zA-Z0-9.-]+(:[0-9]+)?(/.*)?$`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type=string
	PayloadURL string `json:"payloadUrl,omitempty"`

	// Secret is a reference to a Kubernetes Secret containing the webhook secret.
	// The webhook secret is used by GitHub to sign webhook payloads.
	// Your service can verify this signature to ensure the request came from GitHub.
	// This field takes precedence over SecretValue if both are provided.
	// See: https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries
	Secret *WebhookPresetSecretSpec `json:"secret,omitempty"`

	// SecretValue is the plaintext value of the webhook secret.
	// Use this for simple cases, but Secret (referencing a Kubernetes Secret) is more secure.
	// If both Secret and SecretValue are provided, Secret takes precedence.
	// +kubebuilder:validation:Type=string
	SecretValue *string `json:"secretValue,omitempty"`

	// ContentType specifies the format of the webhook payload.
	// - "json": Send payload as application/json (recommended)
	// - "form": Send payload as application/x-www-form-urlencoded
	// See: https://docs.github.com/en/webhooks/webhook-events-and-payloads
	// +kubebuilder:validation:Enum=json;form
	// +kubebuilder:validation:Default=form
	// +kubebuilder:validation:Type=string
	ContentType string `json:"contentType,omitempty"`

	// Active determines whether the webhook is active and will send events.
	// Set to false to temporarily disable the webhook without deleting it.
	// +default:value=true
	Active *bool `json:"active,omitempty"`

	// Events is a list of GitHub event types that trigger this webhook.
	// If empty, the webhook subscribes to all events ("*").
	// Common events include "push", "pull_request", "issues", "release".
	// See: https://docs.github.com/en/webhooks/webhook-events-and-payloads
	// +kubebuilder:validation:Type=array
	// +kubebuilder:validation:MinItems=0
	// +kubebuilder:validation:MaxItems=100
	// +kubebuilder:validation:Items=string
	// +kubebuilder:validation:items:Enum=branch_protection_rule;check_run;check_suite;code_scanning_alert;commit_comment;create;delete;dependabot_alert;deploy_key;deployment;deployment_status;discussion;discussion_comment;fork;github_app_authorization;gollum;installation;installation_repositories;issue_comment;issues;label;marketplace_purchase;member;membership;merge_group;meta;milestone;organization;org_block;package;page_build;ping;project;project_card;project_column;public;pull_request;pull_request_review;pull_request_review_comment;pull_request_review_thread;push;registry_package;release;repository;repository_dispatch;repository_import;repository_vulnerability_alert;secret_scanning_alert;security_advisory;sponsorship;star;status;team;team_add;watch;workflow_dispatch;workflow_job;workflow_run
	Events []string `json:"events,omitempty"`

	// SSLVerify enables SSL certificate verification for the webhook endpoint.
	// When true, GitHub verifies the SSL certificate of the PayloadURL.
	// Disable only for testing with self-signed certificates; always enable in production.
	// +default:value=true
	SSLVerify *bool `json:"sslVerify,omitempty"`
}

// WebhookPresetSecretSpec references a Kubernetes Secret containing the webhook secret.
type WebhookPresetSecretSpec struct {
	// Name is the name of the Kubernetes Secret containing the webhook secret.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=250
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9.-]+$`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type=string
	Name *string `json:"name,omitempty"`

	// Key is the key within the Secret that contains the webhook secret value.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=250
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9.-]+$`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type=string
	Key *string `json:"key,omitempty"`

	// Namespace is the namespace of the Secret.
	// If not specified, the namespace of the WebhookPreset is used.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Type=string
	Namespace *string `json:"namespace,omitempty"`
}

// WebhookPresetStatus defines the observed state of WebhookPreset.
type WebhookPresetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the WebhookPreset resource.
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

// WebhookPreset is the Schema for the webhookpresets API
type WebhookPreset struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of WebhookPreset
	// +required
	Spec WebhookPresetSpec `json:"spec"`

	// status defines the observed state of WebhookPreset
	// +optional
	Status WebhookPresetStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// WebhookPresetList contains a list of WebhookPreset
type WebhookPresetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []WebhookPreset `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &WebhookPreset{}, &WebhookPresetList{})
		return nil
	})
}
