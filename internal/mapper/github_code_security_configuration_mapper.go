package mapper

import (
	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/google/go-github/v86/github"
)

func ToGithubCodeSecurityConfiguration(csc *v1alpha1.CodeSecurityConfiguration) github.CodeSecurityConfiguration {
	if csc == nil {
		return github.CodeSecurityConfiguration{}
	}
	spec := csc.Spec
	return github.CodeSecurityConfiguration{
		Name:                                   spec.Name,
		Description:                            spec.Description,
		AdvancedSecurity:                       spec.AdvancedSecurity,
		DependencyGraph:                        spec.DependencyGraph,
		DependencyGraphAutosubmitAction:        spec.DependencyGraphAutosubmitAction,
		DependencyGraphAutosubmitActionOptions: toDependencyGraphAutosubmitActionOptions(spec.DependencyGraphAutosubmitActionOptions),
		DependabotAlerts:                       spec.DependabotAlerts,
		DependabotSecurityUpdates:              spec.DependabotSecurityUpdates,
		CodeScanningDefaultSetup:               spec.CodeScanningDefaultSetup,
		CodeScanningDefaultSetupOptions:        toCodeScanningDefaultSetupOption(spec.CodeScanningDefaultSetupOptions),
		CodeScanningDelegatedAlertDismissal:    spec.CodeScanningDelegatedAlertDismissal,
		CodeScanningOptions:                    toCodeScanningOptions(spec.CodeScanningOptions),
		CodeSecurity:                           spec.CodeSecurity,
		SecretScanning:                         spec.SecretScanning,
		SecretScanningPushProtection:           spec.SecretScanningPushProtection,
		SecretScanningValidityChecks:           spec.SecretScanningValidityChecks,
		SecretScanningNonProviderPatterns:      spec.SecretScanningNonProviderPatterns,
		SecretScanningGenericSecrets:           spec.SecretScanningGenericSecrets,
		SecretScanningDelegatedAlertDismissal:  spec.SecretScanningDelegatedAlertDismissal,
		SecretProtection:                       spec.SecretProtection,
		PrivateVulnerabilityReporting:          spec.PrivateVulnerabilityReporting,
		SecretScanningDelegatedBypass:          spec.SecretScanningDelegatedBypass,
		SecretScanningDelegatedBypassOptions:   toSecretScanningDelegatedBypassOptions(spec.SecretScanningDelegatedBypassOptions),
		Enforcement:                            spec.Enforcement,
	}
}

func toDependencyGraphAutosubmitActionOptions(spec *v1alpha1.DependencyGraphAutosubmitActionOptions) *github.DependencyGraphAutosubmitActionOptions {
	if spec == nil {
		return nil
	}
	return &github.DependencyGraphAutosubmitActionOptions{
		LabeledRunners: spec.LabeledRunners,
	}
}

func toSecretScanningDelegatedBypassOptions(spec *v1alpha1.SecretScanningDelegatedBypassOptions) *github.SecretScanningDelegatedBypassOptions {
	if spec == nil {
		return nil
	}
	reviewers := make([]*github.BypassReviewer, len(spec.Reviewers))
	for i := range spec.Reviewers {
		reviewers[i] = &github.BypassReviewer{
			ReviewerID:   *spec.Reviewers[i].ReviewerId,
			ReviewerType: spec.Reviewers[i].ReviewerType,
		}
	}
	return &github.SecretScanningDelegatedBypassOptions{
		Reviewers: reviewers,
	}
}

func toCodeScanningDefaultSetupOption(spec *v1alpha1.CodeScanningDefaultSetupOptions) *github.CodeScanningDefaultSetupOptions {
	if spec == nil {
		return nil
	}
	return &github.CodeScanningDefaultSetupOptions{
		RunnerType:  spec.RunnerType,
		RunnerLabel: spec.RunnerLabel,
	}
}

func toCodeScanningOptions(spec *v1alpha1.CodeScanningOptions) *github.CodeScanningOptions {
	if spec == nil {
		return nil
	}
	return &github.CodeScanningOptions{
		AllowAdvanced: spec.AllowAdvanced,
	}
}

// dependencyGraphAutosubmitActionOptionsEqual checks if two DependencyGraphAutosubmitActionOptions are equal
func dependencyGraphAutosubmitActionOptionsEqual(first, second *github.DependencyGraphAutosubmitActionOptions) bool {
	if first == nil && second == nil {
		return true
	}
	if first == nil || second == nil {
		return false
	}
	return first.GetLabeledRunners() == second.GetLabeledRunners()
}

// codeScanningDefaultSetupOptionsEqual checks if two CodeScanningDefaultSetupOptions are equal
func codeScanningDefaultSetupOptionsEqual(first, second *github.CodeScanningDefaultSetupOptions) bool {
	if first == nil && second == nil {
		return true
	}
	if first == nil || second == nil {
		return false
	}
	return first.RunnerType == second.RunnerType && first.GetRunnerLabel() == second.GetRunnerLabel()
}

// codeScanningOptionsEqual checks if two CodeScanningOptions are equal
func codeScanningOptionsEqual(first, second *github.CodeScanningOptions) bool {
	if first == nil && second == nil {
		return true
	}
	if first == nil || second == nil {
		return false
	}
	return first.GetAllowAdvanced() == second.GetAllowAdvanced()
}

// CodeSecurityConfigurationsDiffer returns true if the configurations differ, false if they are identical
// Compares all fields in organization_types.go#CodeSecurityConfiguration except DefaultForNewRepos (handled by different API endpoint)
func CodeSecurityConfigurationsDiffer(first, second *github.CodeSecurityConfiguration) bool {
	if first == nil && second == nil {
		return true
	}
	if first == nil || second == nil {
		return false
	}
	return first.Name != second.Name ||
		first.Description != second.Description ||
		first.GetAdvancedSecurity() != second.GetAdvancedSecurity() ||
		first.GetDependencyGraph() != second.GetDependencyGraph() ||
		first.GetDependencyGraphAutosubmitAction() != second.GetDependencyGraphAutosubmitAction() ||
		!dependencyGraphAutosubmitActionOptionsEqual(first.GetDependencyGraphAutosubmitActionOptions(), second.GetDependencyGraphAutosubmitActionOptions()) ||
		first.GetDependabotAlerts() != second.GetDependabotAlerts() ||
		first.GetDependabotSecurityUpdates() != second.GetDependabotSecurityUpdates() ||
		first.GetCodeScanningDefaultSetup() != second.GetCodeScanningDefaultSetup() ||
		!codeScanningDefaultSetupOptionsEqual(first.GetCodeScanningDefaultSetupOptions(), second.GetCodeScanningDefaultSetupOptions()) ||
		first.GetCodeScanningDelegatedAlertDismissal() != second.GetCodeScanningDelegatedAlertDismissal() ||
		!codeScanningOptionsEqual(first.GetCodeScanningOptions(), second.GetCodeScanningOptions()) ||
		first.GetCodeSecurity() != second.GetCodeSecurity() ||
		first.GetSecretScanning() != second.GetSecretScanning() ||
		first.GetSecretScanningPushProtection() != second.GetSecretScanningPushProtection() ||
		first.GetSecretScanningValidityChecks() != second.GetSecretScanningValidityChecks() ||
		first.GetSecretScanningNonProviderPatterns() != second.GetSecretScanningNonProviderPatterns() ||
		first.GetSecretScanningGenericSecrets() != second.GetSecretScanningGenericSecrets() ||
		first.GetSecretScanningDelegatedAlertDismissal() != second.GetSecretScanningDelegatedAlertDismissal() ||
		first.GetSecretProtection() != second.GetSecretProtection() ||
		first.GetPrivateVulnerabilityReporting() != second.GetPrivateVulnerabilityReporting() ||
		first.GetEnforcement() != second.GetEnforcement()
}
