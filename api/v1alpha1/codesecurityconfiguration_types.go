package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CodeSecurityConfigurationSpec defines the desired state of CodeSecurityConfiguration.
// A code security configuration defines a set of security features and settings that can be applied
// to repositories in an organization. This is a configuration-only CRD with no dedicated controller;
// it is reconciled by the Organization controller.
// Please note that activating features may cause additional costs as the code security features are billed additionally.
// See: https://docs.github.com/en/rest/code-security/configurations
type CodeSecurityConfigurationSpec struct {
	// Name is the display name of the code security configuration.
	Name string `json:"name"`
	// Description provides additional information about the configuration's purpose and settings.
	Description string `json:"description"`

	// AdvancedSecurity enables or disables GitHub Advanced Security features.
	// - "enabled": Enable Advanced Security (required for code scanning, secret scanning, and dependency review)
	// - "disabled": Disable Advanced Security
	// - "code_security": Enable code security features only
	// - "secret_protection": Enable secret protection features only
	// Warning: code_security and secret_protection are deprecated values for this field.
	// Prefer the individual code_security and secret_protection fields to set the status of these features.
	// See: https://docs.github.com/en/get-started/learning-about-github/about-github-advanced-security
	// +kubebuilder:validation:Enum=enabled;disabled;code_security;secret_protection
	AdvancedSecurity *string `json:"advancedSecurity,omitempty"`

	// DependencyGraph enables or disables the dependency graph.
	// The dependency graph identifies all dependencies in your repository.
	// - "enabled": Enable dependency graph
	// - "disabled": Disable dependency graph
	// - "not_set": Use default organization or repository setting
	// See: https://docs.github.com/en/code-security/supply-chain-security/understanding-your-software-supply-chain/about-the-dependency-graph
	// +kubebuilder:validation:Enum=enabled;disabled;not_set
	DependencyGraph *string `json:"dependencyGraph,omitempty"`

	// DependencyGraphAutosubmitAction enables automatic submission of dependency information.
	// When enabled, dependency information is automatically submitted from Actions workflows.
	// +kubebuilder:validation:Enum=enabled;disabled;not_set
	DependencyGraphAutosubmitAction *string `json:"dependencyGraphAutosubmitAction,omitempty"`
	// DependencyGraphAutosubmitActionOptions configures options for automatic dependency submission.
	DependencyGraphAutosubmitActionOptions *DependencyGraphAutosubmitActionOptions `json:"dependencyGraphAutosubmitActionOptions,omitempty"`

	// DependabotAlerts enables or disables Dependabot alerts for vulnerable dependencies.
	// Requires DependencyGraph to be enabled.
	// See: https://docs.github.com/en/code-security/dependabot/dependabot-alerts/about-dependabot-alerts
	// +kubebuilder:validation:Enum=enabled;disabled;not_set
	DependabotAlerts *string `json:"dependabotAlerts,omitempty"`

	// DependabotSecurityUpdates enables or disables Dependabot security updates.
	// When enabled, Dependabot automatically creates pull requests to update vulnerable dependencies.
	// Requires DependabotAlerts to be enabled.
	// See: https://docs.github.com/en/code-security/dependabot/dependabot-security-updates/about-dependabot-security-updates
	// +kubebuilder:validation:Enum=enabled;disabled;not_set
	DependabotSecurityUpdates *string `json:"dependabotSecurityUpdates,omitempty"`

	// CodeScanningDefaultSetup enables or disables default code scanning setup.
	// Default setup automatically configures code scanning with recommended settings.
	// See: https://docs.github.com/en/code-security/code-scanning/enabling-code-scanning/configuring-default-setup-for-code-scanning
	// +kubebuilder:validation:Enum=enabled;disabled;not_set
	CodeScanningDefaultSetup *string `json:"codeScanningDefaultSetup,omitempty"`
	// CodeScanningDefaultSetupOptions configures runner options for default code scanning setup.
	CodeScanningDefaultSetupOptions *CodeScanningDefaultSetupOptions `json:"codeScanningDefaultSetupOptions,omitempty"`

	// CodeScanningDelegatedAlertDismissal enables users to dismiss code scanning alerts.
	// When enabled, users with appropriate permissions can dismiss alerts that don't require action.
	// +kubebuilder:validation:Enum=enabled;disabled;not_set
	CodeScanningDelegatedAlertDismissal *string `json:"codeScanningDelegatedAlertDismissal,omitempty"`
	// CodeScanningOptions configures advanced code scanning options.
	CodeScanningOptions *CodeScanningOptions `json:"codeScanningOptions,omitempty"`

	// CodeSecurity is a meta-setting that enables multiple code security features.
	// +kubebuilder:validation:Enum=enabled;disabled;not_set
	CodeSecurity *string `json:"codeSecurity,omitempty"`

	// SecretScanning enables or disables secret scanning.
	// Secret scanning detects secrets (like API keys and tokens) in your code.
	// Requires AdvancedSecurity to be enabled.
	// See: https://docs.github.com/en/code-security/secret-scanning/about-secret-scanning
	// +kubebuilder:validation:Enum=enabled;disabled;not_set
	SecretScanning *string `json:"secretScanning,omitempty"`

	// SecretScanningPushProtection enables or disables push protection for secret scanning.
	// When enabled, pushes containing detected secrets are blocked.
	// Requires SecretScanning to be enabled.
	// See: https://docs.github.com/en/code-security/secret-scanning/push-protection-for-repositories-and-organizations
	// +kubebuilder:validation:Enum=enabled;disabled;not_set
	SecretScanningPushProtection *string `json:"secretScanningPushProtection,omitempty"`

	// SecretScanningValidityChecks enables validation of detected secrets.
	// When enabled, GitHub validates whether detected secrets are still active.
	// +kubebuilder:validation:Enum=enabled;disabled;not_set
	SecretScanningValidityChecks *string `json:"secretScanningValidityChecks,omitempty"`

	// SecretScanningNonProviderPatterns enables detection of non-provider secret patterns.
	// This expands secret scanning beyond known service provider patterns.
	// +kubebuilder:validation:Enum=enabled;disabled;not_set
	SecretScanningNonProviderPatterns *string `json:"secretScanningNonProviderPatterns,omitempty"`

	// SecretScanningGenericSecrets enables detection of generic secrets.
	// This uses AI to detect potential secrets that don't match specific patterns.
	// +kubebuilder:validation:Enum=enabled;disabled;not_set
	SecretScanningGenericSecrets *string `json:"secretScanningGenericSecrets,omitempty"`

	// SecretScanningDelegatedAlertDismissal enables users to dismiss secret scanning alerts.
	// When enabled, users with appropriate permissions can dismiss false-positive alerts.
	// +kubebuilder:validation:Enum=enabled;disabled;not_set
	SecretScanningDelegatedAlertDismissal *string `json:"secretScanningDelegatedAlertDismissal,omitempty"`

	// SecretProtection is a meta-setting that enables multiple secret protection features.
	// +kubebuilder:validation:Enum=enabled;disabled;not_set
	SecretProtection *string `json:"secretProtection,omitempty"`

	// PrivateVulnerabilityReporting enables or disables private vulnerability reporting.
	// When enabled, security researchers can privately report vulnerabilities.
	// See: https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability
	// +kubebuilder:validation:Enum=enabled;disabled;not_set
	PrivateVulnerabilityReporting *string `json:"privateVulnerabilityReporting,omitempty"`

	// Enforcement determines how strictly this configuration is applied.
	// - "enforced": Configuration settings are strictly enforced and cannot be overridden
	// - "unenforced": Configuration settings are recommended but can be overridden at the repository level
	// +kubebuilder:validation:Enum=enforced;unenforced
	Enforcement *string `json:"enforcement,omitempty"`

	// SecretScanningDelegatedBypass enables delegated bypass for secret scanning push protection.
	// When enabled, contributors can request bypass approval from designated reviewers.
	// +kubebuilder:validation:Enum=enabled;disabled;not_set
	SecretScanningDelegatedBypass *string `json:"secretScanningDelegatedBypass,omitempty"`
	// SecretScanningDelegatedBypassOptions configures reviewers who can approve bypass requests.
	SecretScanningDelegatedBypassOptions *SecretScanningDelegatedBypassOptions `json:"secretScanningDelegatedBypassOptions,omitempty"`

	// DefaultForNewRepos determines whether this configuration is automatically applied to new repositories.
	// - "all": Apply to all new repositories
	// - "private_and_internal": Apply only to new private and internal repositories
	// - "public": Apply only to new public repositories
	// +kubebuilder:validation:Enum=all;private_and_internal;public
	DefaultForNewRepos *string `json:"defaultForNewRepos,omitempty"`
}

// CodeSecurityConfigurationStatus defines the observed state of CodeSecurityConfiguration.
type CodeSecurityConfigurationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the CodeSecurityConfiguration resource.
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

// CodeSecurityConfiguration is the Schema for the codesecurityconfigurations API
type CodeSecurityConfiguration struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of CodeSecurityConfiguration
	// +required
	Spec CodeSecurityConfigurationSpec `json:"spec"`

	// status defines the observed state of CodeSecurityConfiguration
	// +optional
	Status CodeSecurityConfigurationStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// CodeSecurityConfigurationList contains a list of CodeSecurityConfiguration
type CodeSecurityConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []CodeSecurityConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &CodeSecurityConfiguration{}, &CodeSecurityConfigurationList{})
		return nil
	})
}
