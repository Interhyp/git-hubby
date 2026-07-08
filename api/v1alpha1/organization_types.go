/*
Copyright 2026.

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

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GitHubAppCredentials defines the GitHub App credentials configuration for authenticating with the GitHub API.
//
// Deprecated: Use GitHubAppConfig instead, which allows per-organization credential secrets.
type GitHubAppCredentials struct {
	// SecretRef is a reference to a Kubernetes Secret containing GitHub App credentials.
	// The secret must contain the following keys:
	// - app-id: The GitHub App ID
	// - private-key: The GitHub App private key in PEM format
	SecretRef v1.LocalObjectReference `json:"secretRef"`
}

// GitHubAppConfig defines the GitHub App configuration for an organization, referencing the
// Kubernetes Secret that holds the app credentials by name. The secret must reside in the
// namespace configured via the APP_CREDENTIALS_SECRET_NAMESPACE environment variable.
type GitHubAppConfig struct {
	// InstallationId is the numeric ID of the GitHub App installation for this organization.
	// You can find this ID in your GitHub App's installation settings or via the GitHub API.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	InstallationId int64 `json:"installationId"`

	// CredentialsSecretName is the name of the Kubernetes Secret containing the GitHub App credentials.
	// The secret must contain the keys `app-id` and `private-key` and must reside in the namespace
	// configured via the APP_CREDENTIALS_SECRET_NAMESPACE environment variable.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	CredentialsSecretName string `json:"credentialsSecretName"`
}

// OrgCustomPropertyDefaultValue defines the default value for an organization custom property.
// Either Value (for single values) or Values (for multi-select) must be set, but not both.
// +kubebuilder:validation:ExactlyOneOf=value;values
type OrgCustomPropertyDefaultValue struct {
	// Value is the default value for properties with ValueType "string", "single_select", or "true_false".
	// For "true_false", it must be either "true" or "false".
	// For "single_select", it must be one of the AllowedValues defined in the property.
	Value *string `json:"value,omitempty"`
	// Values is the default value for properties with ValueType "multi_select".
	// Each value must be one of the AllowedValues defined in the property.
	Values []string `json:"values,omitempty"`
}

// OrgCustomProperty defines a custom property for an organization.
// Custom properties allow you to add metadata to repositories in your organization.
// This is a kubebuilder annotated copy of github.CustomProperty without the source_type (as it is fixed to "organization").
// For the logic to work the json field names must match the ones in github.CustomProperty.
// See: https://docs.github.com/en/rest/orgs/custom-properties
type OrgCustomProperty struct {
	// PropertyName is the unique name of the custom property.
	// Must start with a letter, number, _, $, or # and can only contain letters, numbers, _, $, #, and -.
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_\$#\-]+$`
	PropertyName string `json:"propertyName"`
	// ValueType specifies the type of value this property accepts.
	// - "string": A free-form text value
	// - "single_select": A single value from a predefined list (requires AllowedValues)
	// - "multi_select": Multiple values from a predefined list (requires AllowedValues)
	// - "true_false": A boolean value represented as "true" or "false"
	// +kubebuilder:validation:Enum=string;single_select;multi_select;true_false
	ValueType string `json:"valueType"`
	// Required indicates whether this property must be set on all repositories.
	// If true, a DefaultValue must be provided.
	// +default:value=false
	Required *bool `json:"required,omitempty"`
	// DefaultValue is the default value for the property.
	// This property must be set if Required is true. It must be empty if Required is false.
	// The allowed format depends on the ValueType.
	// For ValueType "string" or "single_select", it must be a string. For "single_select", it must be one of the AllowedValues.
	// For ValueType "multi_select", it must be a JSON array of strings only containing elements of AllowedValues.
	// For ValueType "true_false", it must be a string that is either "true" or "false".
	DefaultValue *OrgCustomPropertyDefaultValue `json:"defaultValue,omitempty"`
	// Description provides additional information about the purpose and usage of this custom property.
	Description *string `json:"description,omitempty"`
	// AllowedValues is a list of allowed values for the property.
	// This property is required for ValueType "single_select" and "multi_select".
	// For the other ValueTypes, it must be empty.
	// +kubebuilder:validation:MaxItems=200
	AllowedValues []string `json:"allowedValues,omitempty"`
	// ValuesEditableBy determines who can edit the property values on repositories.
	// - "org_actors": Only organization members can edit values
	// - "org_and_repo_actors": Both organization and repository members can edit values
	// +kubebuilder:validation:Enum=org_actors;org_and_repo_actors
	// +default:value="org_actors"
	ValuesEditableBy string `json:"valuesEditableBy,omitempty"`
}

// DependencyGraphAutosubmitActionOptions configures options for automatic dependency submission actions.
// See: https://docs.github.com/en/rest/code-security/configurations
type DependencyGraphAutosubmitActionOptions struct {
	// LabeledRunners indicates whether to use labeled runners for dependency submission actions.
	// If true, actions will run on runners with specific labels instead of GitHub-hosted runners.
	LabeledRunners *bool `json:"labeledRunners,omitempty"`
}

// CodeScanningOptions configures code scanning feature options for a security configuration.
// See: https://docs.github.com/en/rest/code-security/configurations
type CodeScanningOptions struct {
	// AllowAdvanced determines whether users can enable advanced code scanning features.
	// When true, repository administrators can configure advanced code scanning settings beyond the default setup.
	AllowAdvanced *bool `json:"allowAdvanced,omitempty"`
}

// CodeScanningDefaultSetupOptions configures the default setup options for code scanning.
// See: https://docs.github.com/en/rest/code-security/configurations
type CodeScanningDefaultSetupOptions struct {
	// RunnerType specifies which type of runners to use for code scanning.
	// - "standard": Use GitHub-hosted standard runners
	// - "labeled": Use self-hosted runners with specific labels (requires RunnerLabel)
	// - "not_set": No runner type is configured
	// +kubebuilder:validation:Enum=standard;labeled;not_set
	RunnerType string `json:"runnerType"`
	// RunnerLabel specifies the label of self-hosted runners to use.
	// This field is required when RunnerType is "labeled" and ignored otherwise.
	RunnerLabel *string `json:"runnerLabel,omitempty"`
}

// SecretScanningDelegatedBypassOptions configures reviewers who can approve secret scanning bypass requests.
// When delegated bypass is enabled, contributors can request to bypass secret scanning push protection,
// and the specified reviewers can approve or deny these requests.
// See: https://docs.github.com/en/rest/code-security/configurations
type SecretScanningDelegatedBypassOptions struct {
	// Reviewers is a list of teams or organization roles that can review bypass requests.
	Reviewers []*BypassReviewer `json:"reviewers"`
}

// BypassReviewer represents a team or role that can review secret scanning bypass requests.
// Either ReviewerId (for direct ID specification) or ReviewerName (for name-based resolution) must be set.
// See: https://docs.github.com/en/rest/code-security/configurations
// +kubebuilder:validation:ExactlyOneOf=reviewerId;reviewerName
type BypassReviewer struct {
	// ReviewerId is the numeric ID of the reviewer (team ID or role ID).
	// This field is mutually exclusive with ReviewerName.
	ReviewerId *int64 `json:"reviewerId,omitempty"`
	// ReviewerName is the name of the reviewer (team slug or role name) which will be resolved to an ID based on the ReviewerType.
	// This field is mutually exclusive with ReviewerId.
	// For TEAM type, this should be the team slug.
	// For ROLE type, this should be the role name.
	ReviewerName *string `json:"reviewerName,omitempty"`
	// ReviewerType specifies the type of reviewer.
	// - "TEAM": A team within the organization (use team slug for ReviewerName)
	// - "ROLE": An organization role (use role name for ReviewerName)
	// +kubebuilder:validation:Enum=TEAM;ROLE
	ReviewerType string `json:"reviewerType"`
}

// SelectedAllowedActions defines which specific actions are allowed when AllowedActions is set to "selected".
// At least one setting must be configured to allow some actions.
// See: https://docs.github.com/en/rest/actions/permissions
type SelectedAllowedActions struct {
	// GitHubOwnedAllowed determines whether actions created by GitHub are allowed to run.
	// This includes actions in the "actions" and "github" organizations.
	// +default:value=false
	GitHubOwnedAllowed *bool `json:"githubOwnedAllowed,omitempty"`
	// VerifiedAllowed determines whether actions from verified creators are allowed to run.
	// Verified creators are trusted partners and organizations with verified domains.
	// +default:value=false
	VerifiedAllowed *bool `json:"verifiedAllowed,omitempty"`
	// PatternsAllowed is a list of glob patterns specifying allowed actions.
	// Each pattern can match action repositories using wildcards, e.g., "my-org/*" or "*/action-name@*".
	// +default:value=[]
	PatternsAllowed []string `json:"patternsAllowed,omitempty"`
}

// ActionsSettings configures GitHub Actions permissions and behavior for an organization.
// See: https://docs.github.com/en/rest/actions/permissions
type ActionsSettings struct {
	// EnabledRepositories determines which repositories can use GitHub Actions.
	// - "all": Actions enabled for all repositories
	// - "none": Actions disabled for all repositories
	// - "selected": Actions enabled for specific repositories (requires additional configuration)
	// +kubebuilder:validation:Enum=all;none;selected
	// +default:value="none"
	EnabledRepositories *string `json:"enabledRepositories,omitempty"`
	// AllowedActions configures which actions and workflows are allowed to run.
	// Must be nil if EnabledRepositories is "none".
	// - "all": All actions and reusable workflows are allowed
	// - "local_only": Only actions and workflows defined in the same repository or organization are allowed
	// - "selected": Only specific actions are allowed (requires SelectedAllowedActions)
	// +kubebuilder:validation:Enum=all;local_only;selected
	AllowedActions *string `json:"allowedActions,omitempty"`
	// SelectedAllowedActions specifies which actions are allowed when AllowedActions is "selected".
	// This field is required when AllowedActions is "selected" and ignored otherwise.
	SelectedAllowedActions *SelectedAllowedActions `json:"selectedAllowedActions,omitempty"`
	// ShaPinningRequired determines whether workflows must reference actions using the commit SHA instead of tags or branches.
	// When true, improves security by preventing tag manipulation attacks.
	// +default:value=false
	ShaPinningRequired *bool `json:"shaPinningRequired,omitempty"`
	// DefaultWorkflowPermissions sets the default GITHUB_TOKEN permissions for workflows.
	// - "read": Token has read-only access to repository contents
	// - "write": Token has read and write access to repository contents
	// +kubebuilder:validation:Enum=read;write
	// +default:value="read"
	DefaultWorkflowPermissions *string `json:"defaultWorkflowPermissions,omitempty"`
	// CanApprovePullRequestReviews determines whether the GITHUB_TOKEN can approve pull requests.
	// When false, prevents workflows from approving pull requests automatically.
	// +default:value=false
	CanApprovePullRequestReviews *bool `json:"canApprovePullRequestReviews,omitempty"`
	// ArtifactAndLogRetentionDays specifies how many days workflow artifacts and logs are retained.
	// Must be between 1 and 400 days. Shorter retention periods reduce storage costs.
	// +default:value=400
	ArtifactAndLogRetentionDays *int `json:"artifactAndLogRetentionDays,omitempty"`

	// RunnerGroups configures self-hosted runner groups for the organization.
	// Each group can have different visibility and workflow restrictions.
	RunnerGroups []RunnerGroup `json:"runnerGroups,omitempty"`
}

// RunnerGroup configures a self-hosted runner group for GitHub Actions in an organization.
// Runner groups allow you to control which repositories can use specific sets of self-hosted runners.
// See: https://docs.github.com/en/rest/actions/self-hosted-runner-groups
type RunnerGroup struct {
	// Name is the unique name of the runner group within the organization.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Visibility determines which repositories can access runners in this group.
	// - "all": All repositories in the organization can use these runners
	// - "private": Only private repositories can use these runners
	// - "selected": Only specific repositories can use these runners (selected via AvailableActionsRunnerGroups in RepositorySpec)
	// +default:value="all"
	// +kubebuilder:validation:Enum=all;private;selected
	Visibility *string `json:"visibility,omitempty"`

	// RestrictedToWorkflows determines whether this runner group can only run specific workflows.
	// If true, only workflows listed in SelectedWorkflows can use runners in this group.
	// This provides additional security by limiting which workflows can execute on sensitive runners.
	// +default:value=false
	RestrictedToWorkflows *bool `json:"restrictedToWorkflows,omitempty"`

	// SelectedWorkflows lists the workflows that can use runners in this group.
	// This field is only used when RestrictedToWorkflows is true.
	// Each entry must be a full workflow path with a reference (branch, tag, or SHA).
	// Example: "octo-org/octo-repo/.github/workflows/deploy.yaml@refs/heads/main"
	SelectedWorkflows []string `json:"selectedWorkflows,omitempty"`
}

// AttachableCodeSecurityConfigurationRef references a CodeSecurityConfiguration CRD and specifies its attachment scope.
// Code security configurations define security settings like dependency scanning, secret scanning, and code scanning.
// See: https://docs.github.com/en/rest/code-security/configurations
type AttachableCodeSecurityConfigurationRef struct {
	// Name is the name of the referenced CodeSecurityConfiguration CRD.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type=string
	Name string `json:"name"`

	// AttachmentScope defines which repositories the code security configuration applies to.
	// - "all": Apply to all repositories in the organization
	// - "all_without_configurations": Apply to repositories without an existing configuration
	// - "public": Apply only to public repositories
	// - "private_or_internal": Apply only to private and internal repositories
	// - "selected": Apply only to repositories that explicitly reference this configuration in their AttachedCodeSecurityConfiguration field
	// If not set, the configuration is created but not attached to any repositories.
	//
	// Note: GitHub's API does not provide a way to retrieve the current attachment scope type.
	// The reconciler ensures functional correctness by comparing the actual list of attached repositories
	// to the desired state, not the scope label itself. This means GitHub's UI may display "selected repositories"
	// even when the scope is set to "all" (if all repositories happen to be selected), which is a cosmetic
	// discrepancy that does not affect the actual security configuration. The reconciler will only re-attach
	// if the actual repository attachments differ from what the scope implies.
	//
	// For scope "all_without_configurations", the attachment is performed unconditionally without
	// comparing repository lists, as there is no reliable way to determine which repositories should
	// be included (repositories without configurations at the time of attachment may have since
	// been configured). The reconciler will re-attach on every reconciliation for this scope.
	//
	// +kubebuilder:validation:Enum=all;all_without_configurations;public;private_or_internal;selected
	// +optional
	AttachmentScope *string `json:"attachmentScope,omitempty"`
}

// OrganizationSpec defines the desired state of Organization.
// An Organization represents a GitHub organization and its configuration including custom properties,
// rulesets, code security settings, and Actions permissions.
// See: https://docs.github.com/en/rest/orgs/orgs
type OrganizationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// Login is the GitHub organization login (the unique, immutable identifier on GitHub).
	// This field is optional for backwards compatibility. If not specified, the Name field
	// will be used as both login and display name.
	// It is recommended to explicitly set this field to clearly separate login from display name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=39
	// +optional
	Login string `json:"login,omitempty"`

	// Name is the organization's display name shown on the GitHub profile.
	// If Login is not specified, this field will also be used as the organization login
	// for backwards compatibility.
	// At least one of Login or Name must be specified.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	// +optional
	Name string `json:"name,omitempty"`

	// GitHubAppInstallationId is the numeric ID of the GitHub App installation for this organization.
	// This field is deprecated. Use GitHubAppConfig instead, which also allows specifying which
	// credential secret to use. When only this field is set, the operator falls back to the
	// secret name configured via --app-credentials-secret-name.
	// At least one of GitHubAppInstallationId or GitHubAppConfig must be set.
	// If both are set, GitHubAppConfig takes precedence.
	// +kubebuilder:validation:Minimum=1
	// +optional
	GitHubAppInstallationId *int64 `json:"githubAppInstallationId,omitempty"`

	// GitHubAppConfig specifies the GitHub App installation and credentials secret to use for
	// authenticating API requests on behalf of this organization.
	// At least one of GitHubAppConfig or GitHubAppInstallationId must be set.
	// If both are set, GitHubAppConfig takes precedence.
	// +optional
	GitHubAppConfig *GitHubAppConfig `json:"githubAppConfig,omitempty"`

	// CustomProperties defines custom metadata properties that can be assigned to repositories in the organization.
	// These properties allow you to categorize and add structured metadata to your repositories.
	// See: https://docs.github.com/en/rest/orgs/custom-properties
	// +kubebuilder:validation:MaxItems=100
	CustomProperties []OrgCustomProperty `json:"customProperties,omitempty"`

	// ActionsSettings configures GitHub Actions permissions and behavior for the organization.
	// This includes which repositories can use Actions, which actions are allowed, and runner group configurations.
	// See: https://docs.github.com/en/rest/actions/permissions
	ActionsSettings ActionsSettings `json:"actionsSettings"`

	// CodeSecurityConfigurations lists code security configurations to create and optionally attach to repositories.
	// Each configuration defines security features like dependency scanning, secret scanning, and code scanning.
	// See: https://docs.github.com/en/rest/code-security/configurations
	CodeSecurityConfigurations []AttachableCodeSecurityConfigurationRef `json:"codeSecurityConfigurations"`

	// RulesetPresetList references RulesetPreset CRDs that define repository rulesets for this organization.
	// Rulesets enforce policies like branch protection, required reviews, and status checks.
	// See: https://docs.github.com/en/rest/orgs/rules
	RulesetPresetList []v1.LocalObjectReference `json:"rulesetPresets,omitempty"`

	// Description is a human-readable description of the organization.
	// This appears on the organization's GitHub profile page.
	Description string `json:"description"`

	// Location is the organization's location (e.g., "Munich, Germany").
	// This appears on the organization's GitHub profile page.
	// +kubebuilder:validation:MaxLength=100
	// +optional
	Location string `json:"location,omitempty"`

	// MemberSuffix defines a suffix appended to each team member username before matching/adding them on GitHub.
	// Useful when GitHub usernames follow a naming convention (e.g. enterprise suffix).
	// Is ignored if environment variable GITHUB_MEMBER_SUFFIX is set.
	// +kubebuilder:validation:MaxLength=100
	// +optional
	// +kubebuilder:default=""
	MemberSuffix string `json:"memberSuffix,omitempty"`

	// Website is the organization's website URL.
	// This appears on the organization's GitHub profile page as a clickable link.
	// +kubebuilder:validation:MaxLength=255
	// +optional
	Website string `json:"website,omitempty"`

	// Plan indicates the GitHub plan tier for this organization (enterprise, team, or free).
	// Determines whether Enterprise-only features (e.g., custom properties, runner groups) are reconciled or skipped.
	// +kubebuilder:validation:Enum=enterprise;team;free
	// +kubebuilder:default=enterprise
	// +optional
	Plan string `json:"plan,omitempty"`
}

// OrganizationStatus defines the observed state of Organization.
type OrganizationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the Organization resource.
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

	// ObservedSubResourceGenerations is a map of sub-resource names to their observed generations.
	// Keys are in the format "<kind>/<namespace/<name>".
	// SubResources are kubernetes resources that are referenced by this Organization and are not managed
	// by their own controllers like RuleSetPresets and CodeSecurityConfigurations
	ObservedSubResourceGenerations map[string]int64 `json:"observedSubResourceGenerations,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource

// Organization is the Schema for the organizations API
type Organization struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Organization
	// +required
	Spec OrganizationSpec `json:"spec"`

	// status defines the observed state of Organization
	// +optional
	Status OrganizationStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// OrganizationList contains a list of Organization
type OrganizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Organization `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Organization{}, &OrganizationList{})
}
