package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OrganizationRef is a reference to an Organization CRD.
type OrganizationRef struct {
	// Name is the name of the referenced Organization CRD.
	// +kubebuilder:validation:Optional
	Name string `json:"name"`
}

// CustomPropertyValue defines a custom property value for a repository.
// Custom properties are defined at the organization level and applied to repositories.
// If both Value and Values are empty, the value for the property is considered nil (removes the property).
// For custom properties of value type "multi_select", use Values to specify multiple selections.
// For all other value types ("string", "single_select", "true_false"), use Value.
// See: https://docs.github.com/en/rest/repos/custom-properties
// +kubebuilder:validation:ExactlyOneOf=value;values
type CustomPropertyValue struct {
	// Value is the property value for types "string", "single_select", and "true_false".
	// For "true_false", must be "true" or "false".
	// For "single_select", must be one of the allowed values defined in the organization's custom property.
	Value *string `json:"value,omitempty"`
	// Values is the list of selected values for "multi_select" type properties.
	// Each value must be one of the allowed values defined in the organization's custom property.
	Values []string `json:"values,omitempty"`
	// PropertyName is the name of the custom property as defined in the organization.
	PropertyName string `json:"propertyName"`
}

// RepositorySpec defines the desired state of Repository.
// A Repository represents a GitHub repository and its configuration including settings, webhooks,
// rulesets, custom properties, and more.
// See: https://docs.github.com/en/rest/repos/repos
type RepositorySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// TODO: validate validations. Compare with bit-brother.
	// Name is the GitHub repository name.
	// Repository names can contain alphanumeric characters, hyphens, underscores, and periods.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=100
	// +kubebuilder:validation:Pattern=`^[.a-zA-Z0-9][a-zA-Z0-9_.-]{0,99}$`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type=string
	Name string `json:"name"`

	// CustomProperties is a list of custom property values to apply to this repository.
	// These properties must be defined in the parent organization's custom properties.
	// If a property is not present in this list, it will be unset (removed) from the repository.
	// See: https://docs.github.com/en/rest/repos/custom-properties
	CustomProperties []CustomPropertyValue `json:"customProperties,omitempty"`

	// DefaultBranch is the name of the default branch for the repository.
	// This is the base branch for pull requests and where the repository opens by default.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=100
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,99}$`
	// +kubebuilder:validation:Type=string
	// +default:value="main"
	DefaultBranch string `json:"defaultBranch,omitempty"`

	// Visibility controls who can see the repository.
	// - "public": Anyone can see the repository
	// - "private": Only people with explicit access can see the repository
	// - "internal": Only members of the organization can see the repository (Enterprise only)
	// See: https://docs.github.com/en/rest/repos/repos#create-an-organization-repository
	// +kubebuilder:validation:Enum=public;private;internal
	// +default:value="private"
	// +kubebuilder:validation:Type=string
	Visibility string `json:"visibility,omitempty"`

	// HasIssues enables or disables the GitHub Issues feature for the repository.
	// When enabled, users can create and track issues.
	// +default:value=true
	// +kubebuilder:validation:Type=boolean
	// +default:value=true
	HasIssues *bool `json:"hasIssues,omitempty"`

	// HasProjects enables or disables the GitHub Projects (classic) feature for the repository.
	// Note: This refers to classic projects, not the newer Projects feature.
	// +default:value=false
	// +kubebuilder:validation:Type=boolean
	// +default:value=false
	HasProjects *bool `json:"hasProjects,omitempty"`

	// HasWiki enables or disables the GitHub Wiki feature for the repository.
	// When enabled, users can create wiki pages for documentation.
	// +default:value=false
	// +kubebuilder:validation:Type=boolean
	// +default:value=false
	HasWiki *bool `json:"hasWiki,omitempty"`

	// HasDownloads enables or disables the Downloads feature for the repository.
	// This feature is deprecated and has been replaced by Releases.
	// +default:value=false
	// +kubebuilder:validation:Type=boolean
	// +default:value=false
	HasDownloads *bool `json:"hasDownloads,omitempty"`

	// IsTemplate marks the repository as a template repository.
	// Template repositories can be used as a starting point for new repositories.
	// See: https://docs.github.com/en/repositories/creating-and-managing-repositories/creating-a-template-repository
	// +default:value=false
	// +kubebuilder:validation:Type=boolean
	IsTemplate *bool `json:"isTemplate,omitempty"`

	// MergeCommitTitle determines the default title for merge commits.
	// - "PR_TITLE": Use the pull request title
	// - "MERGE_MESSAGE": Use the default merge message format
	// See: https://docs.github.com/en/rest/repos/repos#update-a-repository
	// +kubebuilder:validation:Enum=PR_TITLE;MERGE_MESSAGE
	// +default:value="MERGE_MESSAGE"
	// +kubebuilder:validation:Type=string
	MergeCommitTitle string `json:"mergeCommitTitle,omitempty"`

	// MergeCommitMessage determines the default message for merge commits.
	// - "PR_BODY": Use the pull request body
	// - "PR_TITLE": Use the pull request title
	// - "BLANK": Use a blank message
	// See: https://docs.github.com/en/rest/repos/repos#update-a-repository
	// +kubebuilder:validation:Enum=PR_BODY;PR_TITLE;BLANK
	// +default:value="PR_TITLE"
	// +kubebuilder:validation:Type=string
	MergeCommitMessage string `json:"mergeCommitMessage,omitempty"`

	// AllowedMergeStrategies lists the merge strategies allowed for pull requests.
	// See: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges
	// +default:value=[{"type":"merge"},{"type":"rebase"}]
	AllowedMergeStrategies []MergeStrategy `json:"allowedMergeStrategies,omitempty"`

	// DeleteBranchOnMerge automatically deletes head branches after pull requests are merged.
	// This helps keep the repository clean by removing merged feature branches.
	// See: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-the-automatic-deletion-of-branches
	// +default:value=true
	DeleteBranchOnMerge *bool `json:"deleteBranchOnMerge,omitempty"`

	// About contains descriptive information about the repository.
	About About `json:"about,omitempty"`

	// Archived marks the repository as archived (read-only).
	// Archived repositories cannot receive new issues, pull requests, or commits.
	// See: https://docs.github.com/en/repositories/archiving-a-github-repository/archiving-repositories
	// +default:value=false
	Archived *bool `json:"archived,omitempty"`

	// ActionsEnabled determines whether this repository can use GitHub Actions.
	// This must be enabled at the organization level for this setting to take effect.
	// See: https://docs.github.com/en/rest/actions/permissions
	// +default:value=true
	ActionsEnabled *bool `json:"actionsEnabled,omitempty"`

	// AccessLevelForExternalWorkflows controls access to workflows outside the repository.
	// - "none": Only workflows in this repository can access actions and reusable workflows
	// - "user": Workflows in user-owned private repositories can access them
	// - "organization": Workflows across the organization can access them
	// - "enterprise": Workflows across the enterprise can access them
	// See: https://docs.github.com/en/rest/actions/permissions
	// +kubebuilder:validation:Enum=none;user;organization;enterprise
	// +default:value="none"
	AccessLevelForExternalWorkflows *string `json:"accessLevelForExternalWorkflows,omitempty"`

	// AvailableActionsRunnerGroups lists runner group names that this repository can use.
	// This is only relevant when the organization's runner groups have "selected" visibility.
	// See: https://docs.github.com/en/rest/actions/self-hosted-runner-groups
	AvailableActionsRunnerGroups []string `json:"availableActionsRunnerGroups,omitempty"`

	// OrganizationRef references the Organization CRD this repository belongs to.
	// +kubebuilder:validation:Required
	OrganizationRef OrganizationRef `json:"organizationRef,omitempty"`

	// RulesetPresetList references RulesetPreset CRDs to apply to this repository.
	// These define branch protection rules, required status checks, and other policies.
	// See: https://docs.github.com/en/rest/repos/rules
	RulesetPresetList []v1.LocalObjectReference `json:"rulesetPresets,omitempty"`

	// WebhookPresetList references WebhookPreset CRDs to create webhooks for this repository.
	// Webhooks send HTTP POST payloads to external services when specific events occur.
	// See: https://docs.github.com/en/rest/webhooks/repos
	WebhookPresetList []v1.LocalObjectReference `json:"webhookPresets,omitempty"`

	// WebhookIgnorePresetsList references WebhookIgnorePreset CRDs that define webhooks to ignore.
	// Webhooks matching these patterns will not be created even if they are in WebhookPresetList.
	WebhookIgnorePresetsList []v1.LocalObjectReference `json:"webhookIgnorePresets,omitempty"`

	// AutolinksPresetList references AutolinksPreset CRDs to create autolinks for this repository.
	// Autolinks automatically convert references (like "JIRA-123") into clickable links.
	// See: https://docs.github.com/en/rest/repos/autolinks
	AutolinksPresetList []v1.LocalObjectReference `json:"autolinksPresets,omitempty"`

	// DeployKeyList defines deploy keys to create for this repository.
	// Deploy keys are SSH keys that grant access to a single repository.
	// See: https://docs.github.com/en/rest/deploy-keys/deploy-keys
	DeployKeyList []DeployKey `json:"deployKeys,omitempty"`

	// AttachedCodeSecurityConfiguration references a CodeSecurityConfiguration to attach to this repository.
	// This is only used when the organization's configuration has "selected" attachment scope.
	// See: https://docs.github.com/en/rest/code-security/configurations
	AttachedCodeSecurityConfiguration *CodeSecurityConfigurationRef `json:"attachedCodeSecurityConfiguration,omitempty"`
}

// CodeSecurityConfigurationRef references a CodeSecurityConfiguration CRD.
type CodeSecurityConfigurationRef struct {
	// Name is the name of the referenced CodeSecurityConfiguration CRD.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type=string
	Name string `json:"name"`
}

// MergeStrategy defines an allowed merge strategy for pull requests.
// See: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/about-merge-methods-on-github
type MergeStrategy struct {
	// Type specifies the merge strategy type.
	// - "merge": Create a merge commit (preserves all commits from the feature branch)
	// - "rebase": Rebase and merge (rebases commits onto base branch)
	// - "squash": Squash and merge (combines all commits into a single commit)
	// +kubebuilder:validation:Enum=merge;rebase;squash
	Type string `json:"type,omitempty"`
}

// About contains descriptive information about a repository.
type About struct {
	// Description is a short description of the repository displayed on the repository page.
	// +kubebuilder:validation:MaxLength=1000
	// +kubebuilder:validation:Type=string
	Description string `json:"description,omitempty"`

	// Website is the URL of the repository's homepage or documentation.
	// Must be a valid HTTP or HTTPS URL.
	// +kubebuilder:validation:MaxLength=200
	// +kubebuilder:validation:Pattern=`^https?://[^\s]+$`
	// +kubebuilder:validation:Type=string
	Website string `json:"website,omitempty"`

	// Topics is a list of topics (tags) that categorize and help discover the repository.
	// Topics appear on the repository page and in GitHub's topic explorer.
	// See: https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/classifying-your-repository-with-topics
	Topics []Topic `json:"topics,omitempty"`
}

// Topic represents a repository topic (tag) for categorization.
// See: https://docs.github.com/en/rest/repos/repos#replace-all-repository-topics
type Topic struct {
	// Name is the topic name.
	// Topics must be lowercase and can contain letters, numbers, and hyphens.
	// They must start with a letter or number.
	// +kubebuilder:validation:MaxLength=50
	// +kubebuilder:validation:Pattern=`^[a-z0-9][a-z0-9-]{0,49}$`
	// +kubebuilder:validation:Type=string
	Name string `json:"name,omitempty"`
}

// DeployKey defines an SSH key for read-only or read-write access to a single repository.
// Deploy keys are commonly used for CI/CD systems and automated deployments.
// See: https://docs.github.com/en/rest/deploy-keys/deploy-keys
type DeployKey struct {
	// Key is the public SSH key in OpenSSH format.
	// Supported key types are RSA and Ed25519.
	// Example: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC..." or "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5..."
	// +kubebuilder:validation:Pattern=`^ssh-(rsa|ed25519) [A-Za-z0-9+/]+={0,3}( [^\s]+)?$`
	// +kubebuilder:validation:Type=string
	Key string `json:"key"`

	// Title is a descriptive name for the deploy key shown in the repository settings.
	// Examples: "CI/CD Key", "Read-Only Deploy Key", "Production Server"
	// +kubebuilder:validation:Type=string
	Title string `json:"title"`

	// ReadOnly determines the access level for this deploy key.
	// - true: Key can only read from the repository (cannot push)
	// - false: Key can read and write to the repository (can push commits)
	// +default:value=true
	// +kubebuilder:validation:Type=boolean
	ReadOnly *bool `json:"readOnly,omitempty"`
}

// RepositoryStatus defines the observed state of Repository.
type RepositoryStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// Webhooks is a list of webhooks configured for this repository
	// the key is the hash of the configuration
	Webhooks map[string]WebhookStatus `json:"webhooks,omitempty"`

	// conditions represent the current state of the Repository resource.
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

	// ID is the repository ID as created by GitHub.
	ID *int64 `json:"id,omitempty"`

	// ObservedSubResourceGenerations is a map of sub-resource names to their observed generations.
	// Keys are in the format "<kind>/<namespace/<name>".
	// SubResources are kubernetes resources that are referenced by this Repository and are not managed
	// by their own controllers like WebhookPresets, RuleSetPresets and the attached CodeSecurityConfiguration
	ObservedSubResourceGenerations map[string]int64 `json:"observedSubResourceGenerations,omitempty"`
}

// WebhookStatus defines the status of a webhook configured for a repository
type WebhookStatus struct {
	// Secret is a hash of the secret used for the webhook
	SecretHash string `json:"secretHash,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource

// Repository is the Schema for the repositories API
type Repository struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Repository
	// +required
	Spec RepositorySpec `json:"spec"`

	// status defines the observed state of Repository
	// +optional
	Status RepositoryStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// RepositoryList contains a list of Repository
type RepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Repository `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Repository{}, &RepositoryList{})
		return nil
	})
}
