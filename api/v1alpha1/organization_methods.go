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

func (in *Organization) GetTypeRepresentation() string {
	return "Organization"
}

func (in *Organization) GetConditions() *[]metav1.Condition {
	if in == nil {
		return nil
	}
	return &in.Status.Conditions
}

func (in *Organization) IsHealthy() bool {
	if in == nil {
		return false
	}
	readyCondition := meta.FindStatusCondition(in.Status.Conditions, string(conditions.TypeReady))
	return readyCondition != nil && readyCondition.Status == metav1.ConditionTrue
}

func (in *Organization) GetObservedGeneration() int64 {
	if in == nil {
		return 0
	}
	readyCondition := meta.FindStatusCondition(in.Status.Conditions, string(conditions.TypeReady))
	if readyCondition == nil {
		return 0
	}
	return readyCondition.ObservedGeneration
}

func (in *Organization) GetObservedSubResourceGenerations() map[string]int64 {
	if in == nil {
		return nil
	}
	return in.Status.ObservedSubResourceGenerations
}

func (in *Organization) GetPlan() string {
	if in == nil {
		return ""
	}
	if in.Spec.Plan == "" {
		return PlanEnterprise
	}
	return in.Spec.Plan
}

// HasEnterpriseFeatures returns true if the organization has enterprise-level features.
// Returns false for free plan, true for enterprise and other plans.
func (in *Organization) HasEnterpriseFeatures() bool {
	if in == nil {
		return false
	}
	return in.GetPlan() != PlanFree
}

func (in *Organization) SetObservedSubResourceGeneration(new map[string]int64) {
	if in == nil {
		return
	}
	in.Status.ObservedSubResourceGenerations = new
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

// ResolveGitHubAppConfig resolves the effective GitHubAppConfig for this Organization.
// If GitHubAppConfig is set, it is returned directly (it takes precedence).
// Otherwise, if the deprecated GitHubAppInstallationId field is set, a GitHubAppConfig is
// synthesised using the provided legacySecretName as the credentials secret.
// Returns an error if neither field is set.
func (in *Organization) ResolveGitHubAppConfig(legacySecretName string) (*GitHubAppConfig, error) {
	if in == nil {
		return nil, fmt.Errorf("organization is nil")
	}
	if in.Spec.GitHubAppConfig != nil {
		return in.Spec.GitHubAppConfig, nil
	}
	if in.Spec.GitHubAppInstallationId != nil {
		return &GitHubAppConfig{
			InstallationId:        *in.Spec.GitHubAppInstallationId,
			CredentialsSecretName: legacySecretName,
		}, nil
	}
	return nil, fmt.Errorf("organization %s/%s has neither githubAppConfig nor githubAppInstallationId set", in.Namespace, in.Name)
}
