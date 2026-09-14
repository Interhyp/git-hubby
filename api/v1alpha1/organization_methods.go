package v1alpha1

import (
	"fmt"

	"github.com/Interhyp/git-hubby/internal/conditions"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// PlanFree represents the GitHub free plan
	PlanFree = "free"
	// PlanTeam represents the GitHub team plan
	PlanTeam = "team"
	// PlanEnterprise represents the GitHub enterprise plan
	PlanEnterprise = "enterprise"

	// VisibilityPublic represents a public repository visible to everyone
	VisibilityPublic = "public"
	// VisibilityPrivate represents a private repository visible only to explicit collaborators
	VisibilityPrivate = "private"
	// VisibilityInternal represents an internal repository visible to organization members (Enterprise only)
	VisibilityInternal = "internal"
)

func (o *Organization) GetTypeRepresentation() string {
	return "Organization"
}

func (o *Organization) GetConditions() *[]metav1.Condition {
	if o == nil {
		return nil
	}
	return &o.Status.Conditions
}

func (o *Organization) IsHealthy() bool {
	if o == nil {
		return false
	}
	readyCondition := meta.FindStatusCondition(o.Status.Conditions, string(conditions.TypeReady))
	return readyCondition != nil && readyCondition.Status == metav1.ConditionTrue
}

func (o *Organization) GetObservedGeneration() int64 {
	if o == nil {
		return 0
	}
	readyCondition := meta.FindStatusCondition(o.Status.Conditions, string(conditions.TypeReady))
	if readyCondition == nil {
		return 0
	}
	return readyCondition.ObservedGeneration
}

func (o *Organization) GetObservedSubResourceGenerations() map[string]int64 {
	if o == nil {
		return nil
	}
	return o.Status.ObservedSubResourceGenerations
}

func (o *Organization) GetPlan() string {
	if o == nil {
		return ""
	}
	if o.Spec.Plan == "" {
		return PlanEnterprise
	}
	return o.Spec.Plan
}

// HasEnterpriseFeatures returns true if the organization has enterprise-level features.
// Returns false for free plan, true for enterprise and other plans.
func (o *Organization) HasEnterpriseFeatures() bool {
	if o == nil {
		return false
	}
	return o.GetPlan() != PlanFree
}

func (o *Organization) SetObservedSubResourceGeneration(new map[string]int64) {
	if o == nil {
		return
	}
	o.Status.ObservedSubResourceGenerations = new
}

func (o *OrgCustomPropertyDefaultValue) GetValue() *string {
	if o == nil {
		return nil
	}
	return o.Value
}

func (o *OrgCustomPropertyDefaultValue) GetValues() []string {
	if o == nil {
		return nil
	}
	return o.Values
}

// GetLogin returns the effective login for this organization.
// If Login is explicitly set, it returns Login. Otherwise, it returns Name.
func (o *Organization) GetLogin() string {
	if o == nil {
		return ""
	}
	if o.Spec.Login != "" {
		return o.Spec.Login
	}
	return o.Spec.Name
}

// GetDisplayName returns the effective display name for this organization.
// If both Login and Name are set, Name is the display name.
// If only Login is set, Login is used as display name.
// If only Name is set, Name is used as both login and display name.
func (o *Organization) GetDisplayName() string {
	if o == nil {
		return ""
	}
	if o.Spec.Login != "" && o.Spec.Name != "" {
		return o.Spec.Name
	}
	if o.Spec.Login != "" && o.Spec.Name == "" {
		return o.Spec.Login
	}
	return o.Spec.Name
}

// IsUsingLegacyNameField returns true if only the Name field is set (backwards compatibility mode).
// This is used to generate deprecation warnings.
func (o *Organization) IsUsingLegacyNameField() bool {
	if o == nil {
		return false
	}
	return o.Spec.Login == "" && o.Spec.Name != ""
}

// GetGitHubAppInstallationID returns the effective GitHub App installation ID for this Organization.
// spec.githubAppConfig takes precedence over the deprecated spec.githubAppInstallationId field.
// Returns an error if neither field is set.
func (o *Organization) GetGitHubAppInstallationID() (int64, error) {
	if o == nil {
		return 0, fmt.Errorf("organization is nil")
	}
	if o.Spec.GitHubAppConfig != nil {
		return o.Spec.GitHubAppConfig.InstallationId, nil
	}
	if o.Spec.GitHubAppInstallationId != nil {
		return *o.Spec.GitHubAppInstallationId, nil
	}
	return 0, fmt.Errorf("organization %s/%s has neither githubAppConfig nor githubAppInstallationId set", o.Namespace, o.Name)
}

// GetGitHubAppCredentialsSecretName returns the credentials secret name for this Organization,
// or an empty string when using the deprecated spec.githubAppInstallationId field.
// An empty return value means the GitHub client manager should fall back to its configured
// legacy secret name.
func (o *Organization) GetGitHubAppCredentialsSecretName() string {
	if o == nil {
		return ""
	}
	if o.Spec.GitHubAppConfig != nil {
		return o.Spec.GitHubAppConfig.CredentialsSecretName
	}
	return ""
}
