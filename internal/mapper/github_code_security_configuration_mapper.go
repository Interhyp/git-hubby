package mapper

import (
	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/google/go-github/v93/github"
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

// featureStateDisabled is assumed for feature states that GitHub omits in its response.
const featureStateDisabled = "disabled"

const featureStateEnabled = "enabled"

// advancedSecurity values returned by GitHub. The code_security and secret_protection values are deprecated for
// requests, but GitHub still uses them in responses to report that only one of the features is enabled.
const (
	advancedSecurityCodeSecurity     = "code_security"
	advancedSecuritySecretProtection = "secret_protection"
)

// advancedSecurityComponents splits an advanced_security value into the states of Code Security and Secret Protection.
func advancedSecurityComponents(advancedSecurity string) (codeSecurity, secretProtection bool) {
	switch advancedSecurity {
	case featureStateEnabled:
		return true, true
	case advancedSecurityCodeSecurity:
		return true, false
	case advancedSecuritySecretProtection:
		return false, true
	default:
		return false, false
	}
}

// advancedSecurityDiffers reports whether the desired Code Security and Secret Protection states differ from the
// current ones. GitHub never returns the code_security and secret_protection fields; it reports both features
// combined in advanced_security instead (enabled, code_security, secret_protection or disabled). not_set is reported
// as disabled. advanced_security must not be set together with the individual fields (rejected by GitHub and the CRD
// validation); features which are set by neither are not managed and never differ.
func advancedSecurityDiffers(desired, current *github.CodeSecurityConfiguration) bool {
	currentCS, currentSP := advancedSecurityComponents(current.GetAdvancedSecurity())
	var desiredCS, desiredSP *bool
	if desired.AdvancedSecurity != nil {
		cs, sp := advancedSecurityComponents(*desired.AdvancedSecurity)
		desiredCS, desiredSP = &cs, &sp
	}
	if desired.CodeSecurity != nil {
		cs := *desired.CodeSecurity == featureStateEnabled
		desiredCS = &cs
	}
	if desired.SecretProtection != nil {
		sp := *desired.SecretProtection == featureStateEnabled
		desiredSP = &sp
	}
	return (desiredCS != nil && *desiredCS != currentCS) || (desiredSP != nil && *desiredSP != currentSP)
}

// stateDiffers reports whether a desired feature state differs from the current one. Unset desired states are not
// managed and never differ; states missing on GitHub are treated as disabled.
func stateDiffers(desired, current *string) bool {
	if desired == nil {
		return false
	}
	cur := featureStateDisabled
	if current != nil {
		cur = *current
	}
	return *desired != cur
}

// dependencyGraphAutosubmitActionOptionsDiffer reports whether the desired options differ from the current ones.
// Unset desired options are not managed and never differ.
func dependencyGraphAutosubmitActionOptionsDiffer(desired, current *github.DependencyGraphAutosubmitActionOptions) bool {
	if desired == nil {
		return false
	}
	return desired.GetLabeledRunners() != current.GetLabeledRunners()
}

// codeScanningDefaultSetupOptionsDiffer reports whether the desired options differ from the current ones.
// Unset desired options are not managed and never differ.
func codeScanningDefaultSetupOptionsDiffer(desired, current *github.CodeScanningDefaultSetupOptions) bool {
	if desired == nil {
		return false
	}
	if current == nil {
		return true
	}
	return desired.RunnerType != current.RunnerType || desired.GetRunnerLabel() != current.GetRunnerLabel()
}

// codeScanningOptionsDiffer reports whether the desired options differ from the current ones.
// Unset desired options are not managed and never differ. The options are a variant of code scanning default setup
// ("Enabled with advanced setup allowed" in the GitHub UI) and are not returned by GitHub unless default setup is
// enabled, so they are only compared if the effective default setup state is enabled.
func codeScanningOptionsDiffer(desired, current *github.CodeSecurityConfiguration) bool {
	if desired.CodeScanningOptions == nil {
		return false
	}
	defaultSetup := current.GetCodeScanningDefaultSetup()
	if desired.CodeScanningDefaultSetup != nil {
		defaultSetup = *desired.CodeScanningDefaultSetup
	}
	if defaultSetup != featureStateEnabled {
		return false
	}
	return desired.CodeScanningOptions.GetAllowAdvanced() != current.GetCodeScanningOptions().GetAllowAdvanced()
}

// CodeSecurityConfigurationsDiffer returns true if the desired configuration differs from the current one on GitHub.
// Compares all fields in organization_types.go#CodeSecurityConfiguration except DefaultForNewRepos (handled by different API endpoint).
// Only fields set in desired are compared: GitHub returns values for all fields (e.g. "disabled" for unset features),
// which would otherwise cause an update on every reconciliation. Every update of an attached configuration starts
// an enablement on all attached repositories, blocking further updates with 409 until it is finished.
func CodeSecurityConfigurationsDiffer(desired, current *github.CodeSecurityConfiguration) bool {
	if desired == nil && current == nil {
		return false
	}
	if desired == nil || current == nil {
		return true
	}
	return desired.Name != current.Name ||
		desired.Description != current.Description ||
		advancedSecurityDiffers(desired, current) ||
		stateDiffers(desired.DependencyGraph, current.DependencyGraph) ||
		stateDiffers(desired.DependencyGraphAutosubmitAction, current.DependencyGraphAutosubmitAction) ||
		dependencyGraphAutosubmitActionOptionsDiffer(desired.DependencyGraphAutosubmitActionOptions, current.DependencyGraphAutosubmitActionOptions) ||
		stateDiffers(desired.DependabotAlerts, current.DependabotAlerts) ||
		stateDiffers(desired.DependabotSecurityUpdates, current.DependabotSecurityUpdates) ||
		stateDiffers(desired.CodeScanningDefaultSetup, current.CodeScanningDefaultSetup) ||
		codeScanningDefaultSetupOptionsDiffer(desired.CodeScanningDefaultSetupOptions, current.CodeScanningDefaultSetupOptions) ||
		stateDiffers(desired.CodeScanningDelegatedAlertDismissal, current.CodeScanningDelegatedAlertDismissal) ||
		codeScanningOptionsDiffer(desired, current) ||
		stateDiffers(desired.SecretScanning, current.SecretScanning) ||
		stateDiffers(desired.SecretScanningPushProtection, current.SecretScanningPushProtection) ||
		stateDiffers(desired.SecretScanningValidityChecks, current.SecretScanningValidityChecks) ||
		stateDiffers(desired.SecretScanningNonProviderPatterns, current.SecretScanningNonProviderPatterns) ||
		stateDiffers(desired.SecretScanningGenericSecrets, current.SecretScanningGenericSecrets) ||
		stateDiffers(desired.SecretScanningDelegatedAlertDismissal, current.SecretScanningDelegatedAlertDismissal) ||
		stateDiffers(desired.PrivateVulnerabilityReporting, current.PrivateVulnerabilityReporting) ||
		stateDiffers(desired.Enforcement, current.Enforcement)
}
