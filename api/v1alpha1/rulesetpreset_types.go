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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RulesetPresetSpec defines the desired state of RulesetPreset.
// A ruleset preset defines reusable repository rules that can be applied to multiple repositories
// or organizations. Rulesets enforce policies like branch protection, required reviews, and more.
// See: https://docs.github.com/en/rest/repos/rules
type RulesetPresetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// Name is the display name of the ruleset shown in the GitHub UI.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9][a-zA-Z0-9\s\[\]*/'_.,~-]*[a-zA-Z0-9\[\]]$`
	Name string `json:"name"`

	// Target defines which ref types this ruleset applies to.
	// The Target 'repository' is only supported by Organization-level RulesetPresets. Repository-level
	// RulesetPresets with Target 'repository' are filtered out (i.e. are not checked nor applied).
	// +kubebuilder:validation:Enum=branch;tag;push;repository
	// +default:value="branch"
	// +optional
	Target string `json:"target,omitempty"`

	// Conditions defines which refs are included or excluded in the list of targets for this Ruleset.
	// They also define which Repositories are targeted by Organization-level Rulesets.
	// +optional
	Conditions *RulesetConditions `json:"conditions,omitempty"`

	// Enforcement determines whether the ruleset is enforced.
	// - "disabled": Ruleset is not enforced
	// - "active": Ruleset is actively enforced; violations block operations
	// - "evaluate": Ruleset is evaluated but violations only generate warnings
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=disabled;active;evaluate
	Enforcement RulesetEnforcement `json:"enforcement"`

	// BypassActors defines actors (users, teams, apps) who can bypass this ruleset.
	// Bypass actors can perform operations that would otherwise be blocked by the ruleset.
	// See: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/about-rulesets#about-bypass-mode-for-rulesets
	// +optional
	// +kubebuilder:validation:MaxItems=100
	BypassActors []RulesetBypassActor `json:"bypassActors,omitempty"`

	// Rules defines the specific rules to enforce in this ruleset.
	// +kubebuilder:validation:Required
	Rules RulesetRules `json:"rules"`
}

// RulesetConditions define which refs are targeted by the Ruleset. For Organization-level rules they additionally define
// which Repositories are targeted by the Ruleset via the fields RepositoryName and RepositoryProperty. If neither
// RepositoryName nor RepositoryProperty are set for an Organization-level ruleset, the ruleset will target all repositories.
// +kubebuilder:validation:AtMostOneOf=repositoryName;repositoryProperty
type RulesetConditions struct {
	// RefName defines which git refs (branches or tags) a ruleset applies to.
	// +optional
	RefName *RefNameCondition `json:"refName,omitempty"`

	// RepositoryName targets repositories for Organization-level rulesets by their name.
	// The field is ignored for Repository-level rulesets.
	// +optional
	RepositoryName *RepositoryNameCondition `json:"repositoryName,omitempty"`

	// RepositoryProperty targets repositories for Organization-level rulesets by matching against custom properties.
	// The field is ignored for Repository-level rulesets.
	// +optional
	RepositoryProperty *RepositoryPropertyCondition `json:"repositoryProperty,omitempty"`
}

// RefNameCondition defines which refs a ruleset applies to.
// At least one pattern must be specified.
// +kubebuilder:validation:MinProperties=1
type RefNameCondition struct {
	// Include defines ref patterns that the ruleset applies to.
	// Patterns can use wildcards (*) and must start with refs/heads/ (branches) or refs/tags/ (tags).
	// Use "~DEFAULT_BRANCH" to target the default branch.
	// Use "~ALL" to target all branches.
	// Examples: "refs/heads/main", "refs/heads/feature/*", "refs/tags/v*", "~DEFAULT_BRANCH", "~ALL"
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=50
	// +kubebuilder:validation:items:Pattern=`^(~DEFAULT_BRANCH|~ALL|refs/(heads|tags)(/?[*a-zA-Z0-9][a-zA-Z0-9*_.-]*)*)$`
	Include []string `json:"include,omitempty"`

	// Exclude defines ref patterns to exempt from the ruleset.
	// Refs matching exclude patterns will not be subject to the ruleset rules.
	// Useful for exempting release branches or other special refs.
	// +optional
	// +kubebuilder:validation:MaxItems=50
	// +kubebuilder:validation:items:Pattern=`^(~DEFAULT_BRANCH|~ALL|refs/(heads|tags)(/?[*a-zA-Z0-9][a-zA-Z0-9*_.-]*)*)$`
	Exclude []string `json:"exclude,omitempty"`
}

// RepositoryNameCondition defines repository name patterns for organization-level ruleset targeting.
// Only effective for organization-level rulesets; ignored when applied at repository level.
// Use "~ALL" to target all repositories.
// See: https://docs.github.com/en/rest/orgs/rules#create-an-organization-repository-ruleset
type RepositoryNameCondition struct {
	// Include defines repository name patterns that the ruleset applies to.
	// Use "~ALL" to target all repositories. Supports wildcards (*).
	// Examples: "~ALL", "my-repo-*", "backend-*"
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=50
	Include []string `json:"include"`

	// Exclude defines repository name patterns to exempt from the ruleset.
	// +optional
	// +kubebuilder:validation:MaxItems=50
	Exclude []string `json:"exclude,omitempty"`

	// Protected determines whether renaming a targeted repository is prevented.
	// +optional
	// +default:value=false
	Protected *bool `json:"protected,omitempty"`
}

// RepositoryPropertyCondition defines repository property-based conditions for organization-level ruleset targeting.
// Only effective for organization-level rulesets; ignored when applied at repository level.
// Repositories matching the included property conditions (and not matching excluded ones) are targeted.
// See: https://docs.github.com/en/rest/orgs/rules#create-an-organization-repository-ruleset
type RepositoryPropertyCondition struct {
	// Include defines repository property conditions that must match for the ruleset to apply.
	// A repository must match all included property conditions. The names of the properties in the slice are
	// validated to be unique.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=50
	// +kubebuilder:validation:XValidation:rule="self.all(x, self.filter(y, y.name == x.name).size() <= 1)",message="property names must be unique"
	Include []RepositoryPropertyTarget `json:"include"`

	// Exclude defines repository property conditions that exempt repositories from the ruleset.
	// A repository matching any of the conditions is excluded from the rule.
	// The names of the properties in the slice are validated to be unique.
	// +optional
	// +kubebuilder:validation:MaxItems=50
	// +kubebuilder:validation:XValidation:rule="self.all(x, self.filter(y, y.name == x.name).size() <= 1)",message="property names must be unique"
	Exclude []RepositoryPropertyTarget `json:"exclude,omitempty"`
}

// RepositoryPropertyTarget defines a single repository property condition for ruleset targeting.
// The repository must have the specified property set to one of the given values.
type RepositoryPropertyTarget struct {
	// Name is the name of the repository custom property to match against.
	// Must match a custom property defined at the organization level.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// Note: restrict name length to be able to validate within budget
	// +kubebuilder:validation:MaxLength=100
	Name string `json:"name"`

	// PropertyValues is the list of values to match against the custom property.
	// The repository's property value must be one of these values for the condition to match.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	PropertyValues []string `json:"propertyValues"`

	// Source defines where the property is defined. Defaults to "custom" for organization-defined properties.
	// +optional
	Source *string `json:"source,omitempty"`
}

// RulesetEnforcement defines the enforcement level
type RulesetEnforcement string

const (
	// RulesetEnforcementDisabled means the ruleset is disabled
	RulesetEnforcementDisabled RulesetEnforcement = "disabled"
	// RulesetEnforcementActive means the ruleset is actively enforced
	RulesetEnforcementActive RulesetEnforcement = "active"
	// RulesetEnforcementEvaluate means the ruleset is evaluated but not enforced
	RulesetEnforcementEvaluate RulesetEnforcement = "evaluate"
)

// RulesetBypassActor defines an actor (user, team, or integration) who can bypass ruleset enforcement.
// Either ActorID (for direct specification) or ActorSlug (for name-based resolution) must be provided for
// ActorTypes "Integration" and "Team". ActorID must be provided for ActorType "RepositoryRole".
// Both must be empty for ActorType "DeployKey".
// See: https://docs.github.com/en/rest/repos/rules#create-an-organization-repository-ruleset
// +kubebuilder:validation:AtMostOneOf=actorId;actorSlug
type RulesetBypassActor struct {
	// ActorID is the numeric ID of the bypass actor.
	// This field is mutually exclusive with ActorSlug.
	ActorID *int64 `json:"actorId,omitempty"`

	// ActorSlug is the slug or name of the actor, which will be resolved to an ID.
	// This field is mutually exclusive with ActorID.
	// Only supported for ActorType "Integration" (GitHub Apps) and "Team" (organization teams).
	// For Integration, use the app slug (e.g., "my-github-app").
	// For Team, use the team slug (e.g., "platform-engineers").
	ActorSlug *string `json:"actorSlug,omitempty"`

	// ActorType specifies the type of actor that can bypass the ruleset.
	// - "Integration": A GitHub App
	// - "OrganizationAdmin": Organization administrators
	// - "RepositoryRole": Users with a specific repository role
	// - "Team": An organization team
	// - "DeployKey": A deploy key
	// - "EnterpriseOwner": Enterprise owners (GitHub Enterprise only)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Integration;OrganizationAdmin;RepositoryRole;Team;DeployKey;EnterpriseOwner
	ActorType string `json:"actorType"`

	// BypassMode determines when and how the actor can bypass the ruleset.
	// - "always": Actor can always bypass the ruleset
	// - "pull_request": Actor can bypass only when submitting via pull request
	// +optional
	// +kubebuilder:validation:Enum=always;pull_request
	BypassMode string `json:"bypassMode,omitempty"`
}

// RulesetRules defines the specific rules to enforce in a ruleset.
// Each rule is optional and can be combined to create comprehensive protection policies.
// See: https://docs.github.com/en/rest/repos/rules#available-rules
type RulesetRules struct {
	// Creation prevents the creation of matching refs.
	// When enabled, users cannot create branches or tags matching the ruleset target.
	// +optional
	// +default:value=false
	Creation *bool `json:"creation,omitempty"`

	// Update prevents updates to matching refs.
	// When enabled, users cannot push commits to matching branches.
	// +optional
	// +default:value=false
	Update *bool `json:"update,omitempty"`

	// Deletion prevents deletion of matching refs.
	// When enabled, users cannot delete matching branches or tags.
	// +optional
	// +default:value=false
	Deletion *bool `json:"deletion,omitempty"`

	// RequiredLinearHistory requires branches to have a linear commit history.
	// When enabled, merge commits are not allowed; only rebasing and fast-forward merges are permitted.
	// +optional
	// +default:value=false
	RequiredLinearHistory *bool `json:"requiredLinearHistory,omitempty"`

	// RequiredSignatures requires commits to be signed with a verified signature.
	// When enabled, only commits signed with GPG, SSH, or S/MIME are allowed.
	// See: https://docs.github.com/en/authentication/managing-commit-signature-verification
	// +optional
	// +default:value=false
	RequiredSignatures *bool `json:"requiredSignatures,omitempty"`

	// PullRequest defines pull request requirements for merging.
	// See: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches#require-pull-request-reviews-before-merging
	// +optional
	PullRequest *PullRequestRule `json:"pullRequest,omitempty"`

	// RequiredStatusChecks defines status checks that must pass before merging.
	// See: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches#require-status-checks-before-merging
	// +optional
	RequiredStatusChecks *RequiredStatusChecks `json:"requiredStatusChecks,omitempty"`

	// NonFastForward prevents non-fast-forward updates.
	// When enabled, only fast-forward pushes are allowed, preventing force pushes.
	// +optional
	// +default:value=false
	NonFastForward *bool `json:"nonFastForward,omitempty"`

	// CommitMessagePattern enforces a pattern for commit messages.
	// Use this to enforce commit message conventions like Conventional Commits.
	// +optional
	CommitMessagePattern *PatternRule `json:"commitMessagePattern,omitempty"`

	// CommitAuthorEmailPattern enforces a pattern for commit author email addresses.
	// Use this to ensure commits come from verified email domains.
	// +optional
	CommitAuthorEmailPattern *PatternRule `json:"commitAuthorEmailPattern,omitempty"`

	// CommitterEmailPattern enforces a pattern for committer email addresses.
	// +optional
	CommitterEmailPattern *PatternRule `json:"committerEmailPattern,omitempty"`

	// BranchNamePattern enforces a pattern for branch names.
	// Use this to enforce branch naming conventions like "feature/*" or "hotfix/*".
	// +optional
	BranchNamePattern *PatternRule `json:"branchNamePattern,omitempty"`

	// TagNamePattern enforces a pattern for tag names.
	// Use this to enforce semantic versioning or other tag naming conventions.
	// +optional
	TagNamePattern *PatternRule `json:"tagNamePattern,omitempty"`

	// CopilotReview automatically requests a GitHub Copilot pull request review
	// if the author has access to Copilot code review and their premium requests quota has not reached the limit.
	// +optional
	CopilotReview *CopilotCodeReviewRule `json:"copilotReview,omitempty"`

	// Workflows defines required workflow rules that must pass before merging.
	// This rule type is only effective for organization-level rulesets and is ignored
	// when the preset is applied at the repository level.
	// See: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets#require-workflows-to-pass-before-merging
	// +optional
	Workflows *WorkflowsRule `json:"workflows,omitempty"`

	// missing branch rules, see github.com/google/go-github/v85/github/rules.go:
	// MergeQueue
	// RequiredDeployments
	// CodeScanning
}

// CopilotCodeReviewRule defines the automatic pull request review by GitHub Copilot.
type CopilotCodeReviewRule struct {
	// ReviewOnPush configures Copilot to automatically review each new push to the pull request.
	// +default:value=true
	ReviewOnPush *bool `json:"reviewOnPush,omitempty"`
	// ReviewDraftPullRequests configures Copilot to automatically review draft pull requests before they are marked as ready for review.
	// +default:value=true
	ReviewDraftPullRequests *bool `json:"reviewDraftPullRequests,omitempty"`
}

// WorkflowsRule defines required workflow rules that must pass before merging.
// Workflows are GitHub Actions workflows that are required to run and pass.
// This rule type is only effective for organization-level rulesets and is ignored
// when the preset is applied at the repository level.
// See: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets#require-workflows-to-pass-before-merging
type WorkflowsRule struct {
	// DoNotEnforceOnCreate disables enforcement of this rule for newly created refs.
	// When true, the workflow requirement is not enforced on the first push creating the ref.
	// +optional
	// +default:value=false
	DoNotEnforceOnCreate *bool `json:"doNotEnforceOnCreate,omitempty"`

	// Workflows lists the required workflows that must pass.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	Workflows []RuleWorkflow `json:"workflows"`
}

// RuleWorkflow defines a single required workflow for the workflows rule.
// The workflow is referenced by its path in a repository. The repository is identified by name
// (resolved to a numeric ID at reconciliation time via the GitHub API).
type RuleWorkflow struct {
	// Path is the path to the workflow file relative to the repository root.
	// Example: ".github/workflows/ci.yaml"
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=500
	Path string `json:"path"`

	// RepositoryName is the name of the repository containing the workflow.
	// Must be a repository within the same organization. The name will be resolved
	// to a numeric repository ID at reconciliation time via the GitHub API.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=100
	RepositoryName string `json:"repositoryName"`

	// Ref is the git ref (branch, tag, or SHA) to use for the workflow file.
	// Example: "refs/heads/main"
	// +optional
	Ref *string `json:"ref,omitempty"`

	// ResolvedRepositoryID is the numeric ID of the repository, resolved from RepositoryName at reconciliation time.
	// This field is transient and intentionally excluded from JSON serialization (json:"-").
	// It is populated in-memory by ResolveNamesToIDsInRuleset() during each reconciliation and passed to the mapper
	// to construct the GitHub API request. It is not persisted in the CRD because:
	// - Repository IDs can change (e.g., repo deleted and recreated with the same name)
	// - Re-resolving on each reconciliation ensures correctness
	// - This follows the same pattern as bypass actor slug → ID resolution
	ResolvedRepositoryID *int64 `json:"-"`
}

// PullRequestRule defines pull request requirements that must be met before merging.
// See: https://docs.github.com/en/rest/repos/rules#pull-request
type PullRequestRule struct {

	// AllowedMergeMethods specifies which merge methods are allowed for pull requests.
	// - "squash": Squash all commits into a single commit
	// - "merge": Create a merge commit (preserves all commits)
	// - "rebase": Rebase commits onto the base branch
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:items:Enum=squash;merge;rebase
	AllowedMergeMethods []string `json:"allowedMergeMethods,omitempty"`

	// DismissStaleReviewsOnPush automatically dismisses approved reviews when new commits are pushed.
	// This ensures reviewers see the latest changes before approval.
	// +optional
	// +default:value=false
	DismissStaleReviewsOnPush *bool `json:"dismissStaleReviewsOnPush,omitempty"`

	// RequireCodeOwnerReviews requires approval from code owners before merging.
	// Code owners are defined in a CODEOWNERS file in the repository.
	// See: https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners
	// +optional
	// +default:value=false
	RequireCodeOwnerReviews *bool `json:"requireCodeOwnerReviews,omitempty"`

	// RequireLastPushApproval requires that the most recent push be approved.
	// This prevents merging if new commits are pushed after the last approval.
	// +optional
	// +default:value=false
	RequireLastPushApproval *bool `json:"requireLastPushApproval,omitempty"`

	// RequiredApprovingReviewCount specifies the minimum number of approving reviews required.
	// Must be between 1 and 10.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	RequiredApprovingReviewCount int `json:"requiredApprovingReviewCount,omitempty"`

	// RequiredReviewThreadResolution requires all review comment threads to be resolved before merging.
	// This ensures all feedback is addressed.
	// +optional
	// +default:value=false
	RequiredReviewThreadResolution *bool `json:"requiredReviewThreadResolution,omitempty"`
}

// RequiredStatusChecks defines status check requirements that must pass before merging.
// Status checks are CI/CD jobs, security scans, or other automated checks.
// See: https://docs.github.com/en/rest/repos/rules#required-status-checks
type RequiredStatusChecks struct {
	// Checks lists the required status checks that must pass.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	Checks []StatusCheck `json:"checks"`

	// StrictPolicy requires branches to be up to date with the base branch before merging.
	// When enabled, branches must include the latest changes from the base branch.
	// This prevents merge conflicts but may require additional merges/rebases.
	// +optional
	// +default:value=false
	StrictPolicy *bool `json:"strictPolicy,omitempty"`
}

// StatusCheck defines a required status check that must pass before merging.
// A status check can be provided by a GitHub App or CI/CD integration.
// See: https://docs.github.com/en/rest/repos/rules#required-status-checks
// +kubebuilder:validation:AtMostOneOf=integrationId;appSlug
type StatusCheck struct {
	// Context is the name of the status check as reported by the CI/CD system or app.
	// Examples: "ci/circleci: build", "Security Scan", "Unit Tests"
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	Context string `json:"context"`

	// IntegrationID is the numeric ID of the GitHub App integration providing the status check.
	// This field is mutually exclusive with AppSlug.
	// +optional
	// +kubebuilder:validation:Minimum=1
	IntegrationID *int64 `json:"integrationId,omitempty"`

	// AppSlug is the slug of the GitHub App integration providing the status check.
	// This field is mutually exclusive with IntegrationID.
	// The slug will be resolved to the corresponding integration ID.
	// Only supported for GitHub App integrations.
	// Example: "my-ci-app"
	// +optional
	AppSlug *string `json:"appSlug,omitempty"`
}

// PatternRule defines a pattern-based rule for enforcing naming conventions or content requirements.
// Patterns are evaluated using the specified operator and can be negated if needed.
// See: https://docs.github.com/en/rest/repos/rules#metadata-restrictions
type PatternRule struct {
	// Pattern is the pattern to match against.
	// For regex operator, this is a regular expression.
	// For other operators, this is a literal string or substring.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	Pattern string `json:"pattern"`

	// Operator defines how the pattern is evaluated.
	// - "starts_with": String must start with the pattern
	// - "ends_with": String must end with the pattern
	// - "contains": String must contain the pattern
	// - "regex": String must match the pattern as a regular expression
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=starts_with;ends_with;contains;regex
	Operator string `json:"operator"`

	// Negate inverts the pattern matching logic.
	// When true, the rule passes if the pattern does NOT match.
	// Example: Use with "contains" to prevent certain words in commit messages.
	// +optional
	// +default:value=false
	Negate *bool `json:"negate,omitempty"`
}

// RulesetPresetStatus defines the observed state of RulesetPreset.
type RulesetPresetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the RulesetPreset resource.
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

// RulesetPreset is the Schema for the rulesetpresets API
type RulesetPreset struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of RulesetPreset
	// +required
	Spec RulesetPresetSpec `json:"spec"`

	// status defines the observed state of RulesetPreset
	// +optional
	Status RulesetPresetStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// RulesetPresetList contains a list of RulesetPreset
type RulesetPresetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []RulesetPreset `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RulesetPreset{}, &RulesetPresetList{})
}
