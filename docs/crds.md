# API Reference

## Packages
- [github.interhyp.de/v1alpha1](#githubinterhypdev1alpha1)


## github.interhyp.de/v1alpha1

Package v1alpha1 contains API Schema definitions for the github v1alpha1 API group.


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

### Resource Types
- [AutolinksPreset](#autolinkspreset)
- [AutolinksPresetList](#autolinkspresetlist)
- [CodeSecurityConfiguration](#codesecurityconfiguration)
- [CodeSecurityConfigurationList](#codesecurityconfigurationlist)
- [Organization](#organization)
- [OrganizationList](#organizationlist)
- [Repository](#repository)
- [RepositoryList](#repositorylist)
- [RulesetPreset](#rulesetpreset)
- [RulesetPresetList](#rulesetpresetlist)
- [Team](#team)
- [TeamList](#teamlist)
- [WebhookIgnorePreset](#webhookignorepreset)
- [WebhookIgnorePresetList](#webhookignorepresetlist)
- [WebhookPreset](#webhookpreset)
- [WebhookPresetList](#webhookpresetlist)



#### About



About contains descriptive information about a repository.



_Appears in:_
- [RepositorySpec](#repositoryspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `description` _string_ | Description is a short description of the repository displayed on the repository page. |  | MaxLength: 1000 <br />Type: string <br /> |
| `website` _string_ | Website is the URL of the repository's homepage or documentation.<br />Must be a valid HTTP or HTTPS URL. |  | MaxLength: 200 <br />Pattern: `^https?://[^\s]+$` <br />Type: string <br /> |
| `topics` _[Topic](#topic) array_ | Topics is a list of topics (tags) that categorize and help discover the repository.<br />Topics appear on the repository page and in GitHub's topic explorer.<br />See: https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/classifying-your-repository-with-topics |  |  |


#### ActionsSettings



ActionsSettings configures GitHub Actions permissions and behavior for an organization.
See: https://docs.github.com/en/rest/actions/permissions



_Appears in:_
- [OrganizationSpec](#organizationspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabledRepositories` _string_ | EnabledRepositories determines which repositories can use GitHub Actions.<br />- "all": Actions enabled for all repositories<br />- "none": Actions disabled for all repositories<br />- "selected": Actions enabled for specific repositories (requires additional configuration) | none | Enum: [all none selected] <br /> |
| `allowedActions` _string_ | AllowedActions configures which actions and workflows are allowed to run.<br />Must be nil if EnabledRepositories is "none".<br />- "all": All actions and reusable workflows are allowed<br />- "local_only": Only actions and workflows defined in the same repository or organization are allowed<br />- "selected": Only specific actions are allowed (requires SelectedAllowedActions) |  | Enum: [all local_only selected] <br /> |
| `selectedAllowedActions` _[SelectedAllowedActions](#selectedallowedactions)_ | SelectedAllowedActions specifies which actions are allowed when AllowedActions is "selected".<br />This field is required when AllowedActions is "selected" and ignored otherwise. |  |  |
| `shaPinningRequired` _boolean_ | ShaPinningRequired determines whether workflows must reference actions using the commit SHA instead of tags or branches.<br />When true, improves security by preventing tag manipulation attacks. | false |  |
| `defaultWorkflowPermissions` _string_ | DefaultWorkflowPermissions sets the default GITHUB_TOKEN permissions for workflows.<br />- "read": Token has read-only access to repository contents<br />- "write": Token has read and write access to repository contents | read | Enum: [read write] <br /> |
| `canApprovePullRequestReviews` _boolean_ | CanApprovePullRequestReviews determines whether the GITHUB_TOKEN can approve pull requests.<br />When false, prevents workflows from approving pull requests automatically. | false |  |
| `artifactAndLogRetentionDays` _integer_ | ArtifactAndLogRetentionDays specifies how many days workflow artifacts and logs are retained.<br />Must be between 1 and 400 days. Shorter retention periods reduce storage costs. | 400 |  |
| `runnerGroups` _[RunnerGroup](#runnergroup) array_ | RunnerGroups configures self-hosted runner groups for the organization.<br />Each group can have different visibility and workflow restrictions. |  |  |


#### AttachableCodeSecurityConfigurationRef



AttachableCodeSecurityConfigurationRef references a CodeSecurityConfiguration CRD and specifies its attachment scope.
Code security configurations define security settings like dependency scanning, secret scanning, and code scanning.
See: https://docs.github.com/en/rest/code-security/configurations



_Appears in:_
- [OrganizationSpec](#organizationspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the referenced CodeSecurityConfiguration CRD. |  | Required: \{\} <br />Type: string <br /> |
| `attachmentScope` _string_ | AttachmentScope defines which repositories the code security configuration applies to.<br />- "all": Apply to all repositories in the organization<br />- "all_without_configurations": Apply to repositories without an existing configuration<br />- "public": Apply only to public repositories<br />- "private_or_internal": Apply only to private and internal repositories<br />- "selected": Apply only to repositories that explicitly reference this configuration in their AttachedCodeSecurityConfiguration field<br />If not set, the configuration is created but not attached to any repositories.<br />Note: GitHub's API does not provide a way to retrieve the current attachment scope type.<br />The reconciler ensures functional correctness by comparing the actual list of attached repositories<br />to the desired state, not the scope label itself. This means GitHub's UI may display "selected repositories"<br />even when the scope is set to "all" (if all repositories happen to be selected), which is a cosmetic<br />discrepancy that does not affect the actual security configuration. The reconciler will only re-attach<br />if the actual repository attachments differ from what the scope implies.<br />For scope "all_without_configurations", the attachment is performed unconditionally without<br />comparing repository lists, as there is no reliable way to determine which repositories should<br />be included (repositories without configurations at the time of attachment may have since<br />been configured). The reconciler will re-attach on every reconciliation for this scope. |  | Enum: [all all_without_configurations public private_or_internal selected] <br />Optional: \{\} <br /> |


#### Autolink



Autolink defines an automatic link reference for external resources.
When a reference matching KeyPrefix is found in issues, pull requests, or commit messages,
GitHub automatically converts it to a clickable link using the URLTemplate.
See: https://docs.github.com/en/rest/repos/autolinks



_Appears in:_
- [AutolinksPresetSpec](#autolinkspresetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `keyPrefix` _string_ | KeyPrefix is the text prefix that triggers autolink creation.<br />When text starts with this prefix followed by a reference, it becomes a link.<br />Examples: "JIRA-", "TICKET-", "BUG-" |  | MaxLength: 20 <br />Pattern: `^[a-zA-Z0-9][a-zA-Z0-9-]\{0,19\}$` <br />Type: string <br /> |
| `urlTemplate` _string_ | URLTemplate is the URL pattern used to generate links.<br />Use <num> as a placeholder for the reference number/ID.<br />Example: "https://jira.example.com/browse/<num>" converts "JIRA-123" to "https://jira.example.com/browse/123" |  | MaxLength: 200 <br />Type: string <br /> |
| `isAlphanumeric` _boolean_ | IsAlphanumeric determines whether the reference must be alphanumeric.<br />- true: the <num> parameter of the url_template matches alphanumeric characters `A-Z` (case insensitive), `0-9`, and `-`<br />- false: reference only matches numeric characters. | false | Type: boolean <br /> |


#### AutolinksPreset



AutolinksPreset is the Schema for the autolinkspresets API



_Appears in:_
- [AutolinksPresetList](#autolinkspresetlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `github.interhyp.de/v1alpha1` | | |
| `kind` _string_ | `AutolinksPreset` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[AutolinksPresetSpec](#autolinkspresetspec)_ | spec defines the desired state of AutolinksPreset |  | Required: \{\} <br /> |
| `status` _[AutolinksPresetStatus](#autolinkspresetstatus)_ | status defines the observed state of AutolinksPreset |  | Optional: \{\} <br /> |


#### AutolinksPresetList



AutolinksPresetList contains a list of AutolinksPreset





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `github.interhyp.de/v1alpha1` | | |
| `kind` _string_ | `AutolinksPresetList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[AutolinksPreset](#autolinkspreset) array_ |  |  |  |


#### AutolinksPresetSpec



AutolinksPresetSpec defines the desired state of AutolinksPreset.
Autolinks automatically convert references to external resources (like issue trackers) into clickable links.
See: https://docs.github.com/en/rest/repos/autolinks



_Appears in:_
- [AutolinksPreset](#autolinkspreset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `autolinks` _[Autolink](#autolink) array_ | AutolinkList is a list of autolink configurations to create in repositories.<br />Each autolink defines a prefix that triggers link generation and a URL template. |  |  |


#### AutolinksPresetStatus



AutolinksPresetStatus defines the observed state of AutolinksPreset.



_Appears in:_
- [AutolinksPreset](#autolinkspreset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | conditions represent the current state of the AutolinksPreset resource.<br />Each condition has a unique type and reflects the status of a specific aspect of the resource.<br />Standard condition types include:<br />- "Available": the resource is fully functional<br />- "Progressing": the resource is being created or updated<br />- "Degraded": the resource failed to reach or maintain its desired state<br />The status of each condition is one of True, False, or Unknown. |  | Optional: \{\} <br /> |


#### BypassReviewer



BypassReviewer represents a team or role that can review secret scanning bypass requests.
Either ReviewerId (for direct ID specification) or ReviewerName (for name-based resolution) must be set.
See: https://docs.github.com/en/rest/code-security/configurations

_Validation:_
- ExactlyOneOf: [reviewerId reviewerName]

_Appears in:_
- [SecretScanningDelegatedBypassOptions](#secretscanningdelegatedbypassoptions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `reviewerId` _integer_ | ReviewerId is the numeric ID of the reviewer (team ID or role ID).<br />This field is mutually exclusive with ReviewerName. |  |  |
| `reviewerName` _string_ | ReviewerName is the name of the reviewer (team slug or role name) which will be resolved to an ID based on the ReviewerType.<br />This field is mutually exclusive with ReviewerId.<br />For TEAM type, this should be the team slug.<br />For ROLE type, this should be the role name. |  |  |
| `reviewerType` _string_ | ReviewerType specifies the type of reviewer.<br />- "TEAM": A team within the organization (use team slug for ReviewerName)<br />- "ROLE": An organization role (use role name for ReviewerName) |  | Enum: [TEAM ROLE] <br /> |


#### CodeScanningDefaultSetupOptions



CodeScanningDefaultSetupOptions configures the default setup options for code scanning.
See: https://docs.github.com/en/rest/code-security/configurations



_Appears in:_
- [CodeSecurityConfigurationSpec](#codesecurityconfigurationspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `runnerType` _string_ | RunnerType specifies which type of runners to use for code scanning.<br />- "standard": Use GitHub-hosted standard runners<br />- "labeled": Use self-hosted runners with specific labels (requires RunnerLabel)<br />- "not_set": No runner type is configured |  | Enum: [standard labeled not_set] <br /> |
| `runnerLabel` _string_ | RunnerLabel specifies the label of self-hosted runners to use.<br />This field is required when RunnerType is "labeled" and ignored otherwise. |  |  |


#### CodeScanningOptions



CodeScanningOptions configures code scanning feature options for a security configuration.
See: https://docs.github.com/en/rest/code-security/configurations



_Appears in:_
- [CodeSecurityConfigurationSpec](#codesecurityconfigurationspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `allowAdvanced` _boolean_ | AllowAdvanced determines whether users can enable advanced code scanning features.<br />When true, repository administrators can configure advanced code scanning settings beyond the default setup. |  |  |


#### CodeSecurityConfiguration



CodeSecurityConfiguration is the Schema for the codesecurityconfigurations API



_Appears in:_
- [CodeSecurityConfigurationList](#codesecurityconfigurationlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `github.interhyp.de/v1alpha1` | | |
| `kind` _string_ | `CodeSecurityConfiguration` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[CodeSecurityConfigurationSpec](#codesecurityconfigurationspec)_ | spec defines the desired state of CodeSecurityConfiguration |  | Required: \{\} <br /> |
| `status` _[CodeSecurityConfigurationStatus](#codesecurityconfigurationstatus)_ | status defines the observed state of CodeSecurityConfiguration |  | Optional: \{\} <br /> |


#### CodeSecurityConfigurationList



CodeSecurityConfigurationList contains a list of CodeSecurityConfiguration





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `github.interhyp.de/v1alpha1` | | |
| `kind` _string_ | `CodeSecurityConfigurationList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[CodeSecurityConfiguration](#codesecurityconfiguration) array_ |  |  |  |


#### CodeSecurityConfigurationRef



CodeSecurityConfigurationRef references a CodeSecurityConfiguration CRD.



_Appears in:_
- [RepositorySpec](#repositoryspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the referenced CodeSecurityConfiguration CRD. |  | Required: \{\} <br />Type: string <br /> |


#### CodeSecurityConfigurationSpec



CodeSecurityConfigurationSpec defines the desired state of CodeSecurityConfiguration.
A code security configuration defines a set of security features and settings that can be applied
to repositories in an organization. This is a configuration-only CRD with no dedicated controller;
it is reconciled by the Organization controller.
Please note that activating features may cause additional costs as the code security features are billed additionally.
See: https://docs.github.com/en/rest/code-security/configurations



_Appears in:_
- [CodeSecurityConfiguration](#codesecurityconfiguration)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the display name of the code security configuration. |  |  |
| `description` _string_ | Description provides additional information about the configuration's purpose and settings. |  |  |
| `advancedSecurity` _string_ | AdvancedSecurity enables or disables GitHub Advanced Security features.<br />- "enabled": Enable Advanced Security (required for code scanning, secret scanning, and dependency review)<br />- "disabled": Disable Advanced Security<br />- "code_security": Enable code security features only<br />- "secret_protection": Enable secret protection features only<br />Warning: code_security and secret_protection are deprecated values for this field.<br />Prefer the individual code_security and secret_protection fields to set the status of these features.<br />See: https://docs.github.com/en/get-started/learning-about-github/about-github-advanced-security |  | Enum: [enabled disabled code_security secret_protection] <br /> |
| `dependencyGraph` _string_ | DependencyGraph enables or disables the dependency graph.<br />The dependency graph identifies all dependencies in your repository.<br />- "enabled": Enable dependency graph<br />- "disabled": Disable dependency graph<br />- "not_set": Use default organization or repository setting<br />See: https://docs.github.com/en/code-security/supply-chain-security/understanding-your-software-supply-chain/about-the-dependency-graph |  | Enum: [enabled disabled not_set] <br /> |
| `dependencyGraphAutosubmitAction` _string_ | DependencyGraphAutosubmitAction enables automatic submission of dependency information.<br />When enabled, dependency information is automatically submitted from Actions workflows. |  | Enum: [enabled disabled not_set] <br /> |
| `dependencyGraphAutosubmitActionOptions` _[DependencyGraphAutosubmitActionOptions](#dependencygraphautosubmitactionoptions)_ | DependencyGraphAutosubmitActionOptions configures options for automatic dependency submission. |  |  |
| `dependabotAlerts` _string_ | DependabotAlerts enables or disables Dependabot alerts for vulnerable dependencies.<br />Requires DependencyGraph to be enabled.<br />See: https://docs.github.com/en/code-security/dependabot/dependabot-alerts/about-dependabot-alerts |  | Enum: [enabled disabled not_set] <br /> |
| `dependabotSecurityUpdates` _string_ | DependabotSecurityUpdates enables or disables Dependabot security updates.<br />When enabled, Dependabot automatically creates pull requests to update vulnerable dependencies.<br />Requires DependabotAlerts to be enabled.<br />See: https://docs.github.com/en/code-security/dependabot/dependabot-security-updates/about-dependabot-security-updates |  | Enum: [enabled disabled not_set] <br /> |
| `codeScanningDefaultSetup` _string_ | CodeScanningDefaultSetup enables or disables default code scanning setup.<br />Default setup automatically configures code scanning with recommended settings.<br />See: https://docs.github.com/en/code-security/code-scanning/enabling-code-scanning/configuring-default-setup-for-code-scanning |  | Enum: [enabled disabled not_set] <br /> |
| `codeScanningDefaultSetupOptions` _[CodeScanningDefaultSetupOptions](#codescanningdefaultsetupoptions)_ | CodeScanningDefaultSetupOptions configures runner options for default code scanning setup. |  |  |
| `code_scanning_delegated_alert_dismissal` _string_ | CodeScanningDelegatedAlertDismissal enables users to dismiss code scanning alerts.<br />When enabled, users with appropriate permissions can dismiss alerts that don't require action. |  | Enum: [enabled disabled not_set] <br /> |
| `code_scanning_options` _[CodeScanningOptions](#codescanningoptions)_ | CodeScanningOptions configures advanced code scanning options. |  |  |
| `codeSecurity` _string_ | CodeSecurity is a meta-setting that enables multiple code security features. |  | Enum: [enabled disabled not_set] <br /> |
| `secretScanning` _string_ | SecretScanning enables or disables secret scanning.<br />Secret scanning detects secrets (like API keys and tokens) in your code.<br />Requires AdvancedSecurity to be enabled.<br />See: https://docs.github.com/en/code-security/secret-scanning/about-secret-scanning |  | Enum: [enabled disabled not_set] <br /> |
| `secretScanningPushProtection` _string_ | SecretScanningPushProtection enables or disables push protection for secret scanning.<br />When enabled, pushes containing detected secrets are blocked.<br />Requires SecretScanning to be enabled.<br />See: https://docs.github.com/en/code-security/secret-scanning/push-protection-for-repositories-and-organizations |  | Enum: [enabled disabled not_set] <br /> |
| `secretScanningValidityChecks` _string_ | SecretScanningValidityChecks enables validation of detected secrets.<br />When enabled, GitHub validates whether detected secrets are still active. |  | Enum: [enabled disabled not_set] <br /> |
| `secretScanningNonProviderPatterns` _string_ | SecretScanningNonProviderPatterns enables detection of non-provider secret patterns.<br />This expands secret scanning beyond known service provider patterns. |  | Enum: [enabled disabled not_set] <br /> |
| `secretScanningGenericSecrets` _string_ | SecretScanningGenericSecrets enables detection of generic secrets.<br />This uses AI to detect potential secrets that don't match specific patterns. |  | Enum: [enabled disabled not_set] <br /> |
| `secretScanningDelegatedAlertDismissal` _string_ | SecretScanningDelegatedAlertDismissal enables users to dismiss secret scanning alerts.<br />When enabled, users with appropriate permissions can dismiss false-positive alerts. |  | Enum: [enabled disabled not_set] <br /> |
| `secretProtection` _string_ | SecretProtection is a meta-setting that enables multiple secret protection features. |  | Enum: [enabled disabled not_set] <br /> |
| `privateVulnerabilityReporting` _string_ | PrivateVulnerabilityReporting enables or disables private vulnerability reporting.<br />When enabled, security researchers can privately report vulnerabilities.<br />See: https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability |  | Enum: [enabled disabled not_set] <br /> |
| `enforcement` _string_ | Enforcement determines how strictly this configuration is applied.<br />- "enforced": Configuration settings are strictly enforced and cannot be overridden<br />- "unenforced": Configuration settings are recommended but can be overridden at the repository level |  | Enum: [enforced unenforced] <br /> |
| `secretScanningDelegatedBypass` _string_ | SecretScanningDelegatedBypass enables delegated bypass for secret scanning push protection.<br />When enabled, contributors can request bypass approval from designated reviewers. |  | Enum: [enabled disabled not_set] <br /> |
| `secretScanningDelegatedBypassOptions` _[SecretScanningDelegatedBypassOptions](#secretscanningdelegatedbypassoptions)_ | SecretScanningDelegatedBypassOptions configures reviewers who can approve bypass requests. |  |  |
| `defaultForNewRepos` _string_ | DefaultForNewRepos determines whether this configuration is automatically applied to new repositories.<br />- "all": Apply to all new repositories<br />- "private_and_internal": Apply only to new private and internal repositories<br />- "public": Apply only to new public repositories |  | Enum: [all private_and_internal public] <br /> |


#### CodeSecurityConfigurationStatus



CodeSecurityConfigurationStatus defines the observed state of CodeSecurityConfiguration.



_Appears in:_
- [CodeSecurityConfiguration](#codesecurityconfiguration)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | conditions represent the current state of the CodeSecurityConfiguration resource.<br />Each condition has a unique type and reflects the status of a specific aspect of the resource.<br />Standard condition types include:<br />- "Available": the resource is fully functional<br />- "Progressing": the resource is being created or updated<br />- "Degraded": the resource failed to reach or maintain its desired state<br />The status of each condition is one of True, False, or Unknown. |  | Optional: \{\} <br /> |


#### CopilotCodeReviewRule



CopilotCodeReviewRule defines the automatic pull request review by GitHub Copilot.



_Appears in:_
- [RulesetRules](#rulesetrules)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `reviewOnPush` _boolean_ | ReviewOnPush configures Copilot to automatically review each new push to the pull request. | true |  |
| `reviewDraftPullRequests` _boolean_ | ReviewDraftPullRequests configures Copilot to automatically review draft pull requests before they are marked as ready for review. | true |  |


#### CustomPropertyValue



CustomPropertyValue defines a custom property value for a repository.
Custom properties are defined at the organization level and applied to repositories.
If both Value and Values are empty, the value for the property is considered nil (removes the property).
For custom properties of value type "multi_select", use Values to specify multiple selections.
For all other value types ("string", "single_select", "true_false"), use Value.
See: https://docs.github.com/en/rest/repos/custom-properties

_Validation:_
- ExactlyOneOf: [value values]

_Appears in:_
- [RepositorySpec](#repositoryspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `value` _string_ | Value is the property value for types "string", "single_select", and "true_false".<br />For "true_false", must be "true" or "false".<br />For "single_select", must be one of the allowed values defined in the organization's custom property. |  |  |
| `values` _string array_ | Values is the list of selected values for "multi_select" type properties.<br />Each value must be one of the allowed values defined in the organization's custom property. |  |  |
| `propertyName` _string_ | PropertyName is the name of the custom property as defined in the organization. |  |  |


#### DependencyGraphAutosubmitActionOptions



DependencyGraphAutosubmitActionOptions configures options for automatic dependency submission actions.
See: https://docs.github.com/en/rest/code-security/configurations



_Appears in:_
- [CodeSecurityConfigurationSpec](#codesecurityconfigurationspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `labeledRunners` _boolean_ | LabeledRunners indicates whether to use labeled runners for dependency submission actions.<br />If true, actions will run on runners with specific labels instead of GitHub-hosted runners. |  |  |


#### DeployKey



DeployKey defines an SSH key for read-only or read-write access to a single repository.
Deploy keys are commonly used for CI/CD systems and automated deployments.
See: https://docs.github.com/en/rest/deploy-keys/deploy-keys



_Appears in:_
- [RepositorySpec](#repositoryspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `key` _string_ | Key is the public SSH key in OpenSSH format.<br />Supported key types are RSA and Ed25519.<br />Example: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC..." or "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5..." |  | Pattern: `^ssh-(rsa\|ed25519) [A-Za-z0-9+/]+=\{0,3\}( [^\s]+)?$` <br />Type: string <br /> |
| `title` _string_ | Title is a descriptive name for the deploy key shown in the repository settings.<br />Examples: "CI/CD Key", "Read-Only Deploy Key", "Production Server" |  | Type: string <br /> |
| `readOnly` _boolean_ | ReadOnly determines the access level for this deploy key.<br />- true: Key can only read from the repository (cannot push)<br />- false: Key can read and write to the repository (can push commits) | true | Type: boolean <br /> |


#### GitHubAppConfig



GitHubAppConfig defines the GitHub App configuration for an organization, referencing the
Kubernetes Secret that holds the app credentials by name. The secret must reside in the
namespace configured via the APP_CREDENTIALS_SECRET_NAMESPACE environment variable.



_Appears in:_
- [OrganizationSpec](#organizationspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `installationId` _integer_ | InstallationId is the numeric ID of the GitHub App installation for this organization.<br />You can find this ID in your GitHub App's installation settings or via the GitHub API. |  | Minimum: 1 <br />Required: \{\} <br /> |
| `credentialsSecretName` _string_ | CredentialsSecretName is the name of the Kubernetes Secret containing the GitHub App credentials.<br />The secret must contain the keys `app-id` and `private-key` and must reside in the namespace<br />configured via the APP_CREDENTIALS_SECRET_NAMESPACE environment variable. |  | MinLength: 1 <br />Required: \{\} <br /> |




#### MergeStrategy



MergeStrategy defines an allowed merge strategy for pull requests.
See: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/about-merge-methods-on-github



_Appears in:_
- [RepositorySpec](#repositoryspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _string_ | Type specifies the merge strategy type.<br />- "merge": Create a merge commit (preserves all commits from the feature branch)<br />- "rebase": Rebase and merge (rebases commits onto base branch)<br />- "squash": Squash and merge (combines all commits into a single commit) |  | Enum: [merge rebase squash] <br /> |


#### OrgCustomProperty



OrgCustomProperty defines a custom property for an organization.
Custom properties allow you to add metadata to repositories in your organization.
This is a kubebuilder annotated copy of github.CustomProperty without the source_type (as it is fixed to "organization").
For the logic to work the json field names must match the ones in github.CustomProperty.
See: https://docs.github.com/en/rest/orgs/custom-properties



_Appears in:_
- [OrganizationSpec](#organizationspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `propertyName` _string_ | PropertyName is the unique name of the custom property.<br />Must start with a letter, number, _, $, or # and can only contain letters, numbers, _, $, #, and -. |  | Pattern: `^[a-zA-Z0-9_\$#\-]+$` <br /> |
| `valueType` _string_ | ValueType specifies the type of value this property accepts.<br />- "string": A free-form text value<br />- "single_select": A single value from a predefined list (requires AllowedValues)<br />- "multi_select": Multiple values from a predefined list (requires AllowedValues)<br />- "true_false": A boolean value represented as "true" or "false" |  | Enum: [string single_select multi_select true_false] <br /> |
| `required` _boolean_ | Required indicates whether this property must be set on all repositories.<br />If true, a DefaultValue must be provided. | false |  |
| `defaultValue` _[OrgCustomPropertyDefaultValue](#orgcustompropertydefaultvalue)_ | DefaultValue is the default value for the property.<br />This property must be set if Required is true. It must be empty if Required is false.<br />The allowed format depends on the ValueType.<br />For ValueType "string" or "single_select", it must be a string. For "single_select", it must be one of the AllowedValues.<br />For ValueType "multi_select", it must be a JSON array of strings only containing elements of AllowedValues.<br />For ValueType "true_false", it must be a string that is either "true" or "false". |  | ExactlyOneOf: [value values] <br /> |
| `description` _string_ | Description provides additional information about the purpose and usage of this custom property. |  |  |
| `allowedValues` _string array_ | AllowedValues is a list of allowed values for the property.<br />This property is required for ValueType "single_select" and "multi_select".<br />For the other ValueTypes, it must be empty. |  | MaxItems: 200 <br /> |
| `valuesEditableBy` _string_ | ValuesEditableBy determines who can edit the property values on repositories.<br />- "org_actors": Only organization members can edit values<br />- "org_and_repo_actors": Both organization and repository members can edit values | org_actors | Enum: [org_actors org_and_repo_actors] <br /> |


#### OrgCustomPropertyDefaultValue



OrgCustomPropertyDefaultValue defines the default value for an organization custom property.
Either Value (for single values) or Values (for multi-select) must be set, but not both.

_Validation:_
- ExactlyOneOf: [value values]

_Appears in:_
- [OrgCustomProperty](#orgcustomproperty)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `value` _string_ | Value is the default value for properties with ValueType "string", "single_select", or "true_false".<br />For "true_false", it must be either "true" or "false".<br />For "single_select", it must be one of the AllowedValues defined in the property. |  |  |
| `values` _string array_ | Values is the default value for properties with ValueType "multi_select".<br />Each value must be one of the AllowedValues defined in the property. |  |  |


#### Organization



Organization is the Schema for the organizations API



_Appears in:_
- [OrganizationList](#organizationlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `github.interhyp.de/v1alpha1` | | |
| `kind` _string_ | `Organization` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[OrganizationSpec](#organizationspec)_ | spec defines the desired state of Organization |  | Required: \{\} <br /> |
| `status` _[OrganizationStatus](#organizationstatus)_ | status defines the observed state of Organization |  | Optional: \{\} <br /> |


#### OrganizationList



OrganizationList contains a list of Organization





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `github.interhyp.de/v1alpha1` | | |
| `kind` _string_ | `OrganizationList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Organization](#organization) array_ |  |  |  |


#### OrganizationRef



OrganizationRef is a reference to an Organization CRD.



_Appears in:_
- [RepositorySpec](#repositoryspec)
- [TeamSpec](#teamspec)
- [TeamStatus](#teamstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the referenced Organization CRD. |  | Optional: \{\} <br /> |


#### OrganizationSpec



OrganizationSpec defines the desired state of Organization.
An Organization represents a GitHub organization and its configuration including custom properties,
rulesets, code security settings, and Actions permissions.
See: https://docs.github.com/en/rest/orgs/orgs



_Appears in:_
- [Organization](#organization)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `login` _string_ | Login is the GitHub organization login (the unique, immutable identifier on GitHub).<br />This field is optional for backwards compatibility. If not specified, the Name field<br />will be used as both login and display name.<br />It is recommended to explicitly set this field to clearly separate login from display name. |  | MaxLength: 39 <br />MinLength: 1 <br />Optional: \{\} <br /> |
| `name` _string_ | Name is the organization's display name shown on the GitHub profile.<br />If Login is not specified, this field will also be used as the organization login<br />for backwards compatibility.<br />At least one of Login or Name must be specified. |  | MaxLength: 255 <br />MinLength: 1 <br />Optional: \{\} <br /> |
| `githubAppInstallationId` _integer_ | GitHubAppInstallationId is the numeric ID of the GitHub App installation for this organization.<br />This field is deprecated. Use GitHubAppConfig instead, which also allows specifying which<br />credential secret to use. When only this field is set, the operator falls back to the<br />secret name configured via --app-credentials-secret-name.<br />At least one of GitHubAppInstallationId or GitHubAppConfig must be set.<br />If both are set, GitHubAppConfig takes precedence. |  | Minimum: 1 <br />Optional: \{\} <br /> |
| `githubAppConfig` _[GitHubAppConfig](#githubappconfig)_ | GitHubAppConfig specifies the GitHub App installation and credentials secret to use for<br />authenticating API requests on behalf of this organization.<br />At least one of GitHubAppConfig or GitHubAppInstallationId must be set.<br />If both are set, GitHubAppConfig takes precedence. |  | Optional: \{\} <br /> |
| `customProperties` _[OrgCustomProperty](#orgcustomproperty) array_ | CustomProperties defines custom metadata properties that can be assigned to repositories in the organization.<br />These properties allow you to categorize and add structured metadata to your repositories.<br />See: https://docs.github.com/en/rest/orgs/custom-properties |  | MaxItems: 100 <br /> |
| `actionsSettings` _[ActionsSettings](#actionssettings)_ | ActionsSettings configures GitHub Actions permissions and behavior for the organization.<br />This includes which repositories can use Actions, which actions are allowed, and runner group configurations.<br />See: https://docs.github.com/en/rest/actions/permissions |  |  |
| `codeSecurityConfigurations` _[AttachableCodeSecurityConfigurationRef](#attachablecodesecurityconfigurationref) array_ | CodeSecurityConfigurations lists code security configurations to create and optionally attach to repositories.<br />Each configuration defines security features like dependency scanning, secret scanning, and code scanning.<br />See: https://docs.github.com/en/rest/code-security/configurations |  |  |
| `rulesetPresets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core) array_ | RulesetPresetList references RulesetPreset CRDs that define repository rulesets for this organization.<br />Rulesets enforce policies like branch protection, required reviews, and status checks.<br />See: https://docs.github.com/en/rest/orgs/rules |  |  |
| `description` _string_ | Description is a human-readable description of the organization.<br />This appears on the organization's GitHub profile page. |  |  |
| `location` _string_ | Location is the organization's location (e.g., "Munich, Germany").<br />This appears on the organization's GitHub profile page. |  | MaxLength: 100 <br />Optional: \{\} <br /> |
| `website` _string_ | Website is the organization's website URL.<br />This appears on the organization's GitHub profile page as a clickable link. |  | MaxLength: 255 <br />Optional: \{\} <br /> |
| `plan` _string_ | Plan indicates the GitHub plan tier for this organization (enterprise, team, or free).<br />Determines whether Enterprise-only features (e.g., custom properties, runner groups) are reconciled or skipped. | enterprise | Enum: [enterprise team free] <br />Optional: \{\} <br /> |


#### OrganizationStatus



OrganizationStatus defines the observed state of Organization.



_Appears in:_
- [Organization](#organization)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | conditions represent the current state of the Organization resource.<br />Each condition has a unique type and reflects the status of a specific aspect of the resource.<br />Standard condition types include:<br />- "Available": the resource is fully functional<br />- "Progressing": the resource is being created or updated<br />- "Degraded": the resource failed to reach or maintain its desired state<br />The status of each condition is one of True, False, or Unknown. |  | Optional: \{\} <br /> |
| `observedSubResourceGenerations` _object (keys:string, values:integer)_ | ObservedSubResourceGenerations is a map of sub-resource names to their observed generations.<br />Keys are in the format "<kind>/<namespace/<name>".<br />SubResources are kubernetes resources that are referenced by this Organization and are not managed<br />by their own controllers like RuleSetPresets and CodeSecurityConfigurations |  |  |


#### PatternRule



PatternRule defines a pattern-based rule for enforcing naming conventions or content requirements.
Patterns are evaluated using the specified operator and can be negated if needed.
See: https://docs.github.com/en/rest/repos/rules#metadata-restrictions



_Appears in:_
- [RulesetRules](#rulesetrules)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pattern` _string_ | Pattern is the pattern to match against.<br />For regex operator, this is a regular expression.<br />For other operators, this is a literal string or substring. |  | MaxLength: 1024 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `operator` _string_ | Operator defines how the pattern is evaluated.<br />- "starts_with": String must start with the pattern<br />- "ends_with": String must end with the pattern<br />- "contains": String must contain the pattern<br />- "regex": String must match the pattern as a regular expression |  | Enum: [starts_with ends_with contains regex] <br />Required: \{\} <br /> |
| `negate` _boolean_ | Negate inverts the pattern matching logic.<br />When true, the rule passes if the pattern does NOT match.<br />Example: Use with "contains" to prevent certain words in commit messages. | false | Optional: \{\} <br /> |


#### PullRequestRule



PullRequestRule defines pull request requirements that must be met before merging.
See: https://docs.github.com/en/rest/repos/rules#pull-request



_Appears in:_
- [RulesetRules](#rulesetrules)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `allowedMergeMethods` _string array_ | AllowedMergeMethods specifies which merge methods are allowed for pull requests.<br />- "squash": Squash all commits into a single commit<br />- "merge": Create a merge commit (preserves all commits)<br />- "rebase": Rebase commits onto the base branch |  | Required: \{\} <br />items:Enum: [squash merge rebase] <br /> |
| `dismissStaleReviewsOnPush` _boolean_ | DismissStaleReviewsOnPush automatically dismisses approved reviews when new commits are pushed.<br />This ensures reviewers see the latest changes before approval. | false | Optional: \{\} <br /> |
| `requireCodeOwnerReviews` _boolean_ | RequireCodeOwnerReviews requires approval from code owners before merging.<br />Code owners are defined in a CODEOWNERS file in the repository.<br />See: https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners | false | Optional: \{\} <br /> |
| `requireLastPushApproval` _boolean_ | RequireLastPushApproval requires that the most recent push be approved.<br />This prevents merging if new commits are pushed after the last approval. | false | Optional: \{\} <br /> |
| `requiredApprovingReviewCount` _integer_ | RequiredApprovingReviewCount specifies the minimum number of approving reviews required.<br />Must be between 1 and 10. |  | Maximum: 10 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `requiredReviewThreadResolution` _boolean_ | RequiredReviewThreadResolution requires all review comment threads to be resolved before merging.<br />This ensures all feedback is addressed. | false | Optional: \{\} <br /> |


#### RefNameCondition



RefNameCondition defines which refs a ruleset applies to.
At least one pattern must be specified.

_Validation:_
- MinProperties: 1

_Appears in:_
- [RulesetConditions](#rulesetconditions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `include` _string array_ | Include defines ref patterns that the ruleset applies to.<br />Patterns can use wildcards (*) and must start with refs/heads/ (branches) or refs/tags/ (tags).<br />Use "~DEFAULT_BRANCH" to target the default branch.<br />Use "~ALL" to target all branches.<br />Examples: "refs/heads/main", "refs/heads/feature/*", "refs/tags/v*", "~DEFAULT_BRANCH", "~ALL" |  | MaxItems: 50 <br />MinItems: 1 <br />items:Pattern: `^(~DEFAULT_BRANCH\|~ALL\|refs/(heads\|tags)(/?[*a-zA-Z0-9][a-zA-Z0-9*_.-]*)*)$` <br />Optional: \{\} <br /> |
| `exclude` _string array_ | Exclude defines ref patterns to exempt from the ruleset.<br />Refs matching exclude patterns will not be subject to the ruleset rules.<br />Useful for exempting release branches or other special refs. |  | MaxItems: 50 <br />items:Pattern: `^(~DEFAULT_BRANCH\|~ALL\|refs/(heads\|tags)(/?[*a-zA-Z0-9][a-zA-Z0-9*_.-]*)*)$` <br />Optional: \{\} <br /> |


#### Repository



Repository is the Schema for the repositories API



_Appears in:_
- [RepositoryList](#repositorylist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `github.interhyp.de/v1alpha1` | | |
| `kind` _string_ | `Repository` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[RepositorySpec](#repositoryspec)_ | spec defines the desired state of Repository |  | Required: \{\} <br /> |
| `status` _[RepositoryStatus](#repositorystatus)_ | status defines the observed state of Repository |  | Optional: \{\} <br /> |


#### RepositoryList



RepositoryList contains a list of Repository





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `github.interhyp.de/v1alpha1` | | |
| `kind` _string_ | `RepositoryList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Repository](#repository) array_ |  |  |  |


#### RepositoryNameCondition



RepositoryNameCondition defines repository name patterns for organization-level ruleset targeting.
Only effective for organization-level rulesets; ignored when applied at repository level.
Use "~ALL" to target all repositories.
See: https://docs.github.com/en/rest/orgs/rules#create-an-organization-repository-ruleset



_Appears in:_
- [RulesetConditions](#rulesetconditions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `include` _string array_ | Include defines repository name patterns that the ruleset applies to.<br />Use "~ALL" to target all repositories. Supports wildcards (*).<br />Examples: "~ALL", "my-repo-*", "backend-*" |  | MaxItems: 50 <br />MinItems: 1 <br /> |
| `exclude` _string array_ | Exclude defines repository name patterns to exempt from the ruleset. |  | MaxItems: 50 <br />Optional: \{\} <br /> |
| `protected` _boolean_ | Protected determines whether renaming a targeted repository is prevented. | false | Optional: \{\} <br /> |


#### RepositoryPropertyCondition



RepositoryPropertyCondition defines repository property-based conditions for organization-level ruleset targeting.
Only effective for organization-level rulesets; ignored when applied at repository level.
Repositories matching the included property conditions (and not matching excluded ones) are targeted.
See: https://docs.github.com/en/rest/orgs/rules#create-an-organization-repository-ruleset



_Appears in:_
- [RulesetConditions](#rulesetconditions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `include` _[RepositoryPropertyTarget](#repositorypropertytarget) array_ | Include defines repository property conditions that must match for the ruleset to apply.<br />A repository must match all included property conditions. The names of the properties in the slice are<br />validated to be unique. |  | MaxItems: 50 <br />MinItems: 1 <br /> |
| `exclude` _[RepositoryPropertyTarget](#repositorypropertytarget) array_ | Exclude defines repository property conditions that exempt repositories from the ruleset.<br />A repository matching any of the conditions is excluded from the rule.<br />The names of the properties in the slice are validated to be unique. |  | MaxItems: 50 <br />Optional: \{\} <br /> |


#### RepositoryPropertyTarget



RepositoryPropertyTarget defines a single repository property condition for ruleset targeting.
The repository must have the specified property set to one of the given values.



_Appears in:_
- [RepositoryPropertyCondition](#repositorypropertycondition)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the repository custom property to match against.<br />Must match a custom property defined at the organization level.<br />Note: restrict name length to be able to validate within budget |  | MaxLength: 100 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `propertyValues` _string array_ | PropertyValues is the list of values to match against the custom property.<br />The repository's property value must be one of these values for the condition to match. |  | MinItems: 1 <br />Required: \{\} <br /> |
| `source` _string_ | Source defines where the property is defined. Defaults to "custom" for organization-defined properties. |  | Optional: \{\} <br /> |


#### RepositorySpec



RepositorySpec defines the desired state of Repository.
A Repository represents a GitHub repository and its configuration including settings, webhooks,
rulesets, custom properties, and more.
See: https://docs.github.com/en/rest/repos/repos



_Appears in:_
- [Repository](#repository)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the GitHub repository name.<br />Repository names can contain alphanumeric characters, hyphens, underscores, and periods. |  | MaxLength: 100 <br />MinLength: 1 <br />Pattern: `^[.a-zA-Z0-9][a-zA-Z0-9_.-]\{0,99\}$` <br />Required: \{\} <br />Type: string <br /> |
| `customProperties` _[CustomPropertyValue](#custompropertyvalue) array_ | CustomProperties is a list of custom property values to apply to this repository.<br />These properties must be defined in the parent organization's custom properties.<br />If a property is not present in this list, it will be unset (removed) from the repository.<br />See: https://docs.github.com/en/rest/repos/custom-properties |  | ExactlyOneOf: [value values] <br /> |
| `defaultBranch` _string_ | DefaultBranch is the name of the default branch for the repository.<br />This is the base branch for pull requests and where the repository opens by default. | main | MaxLength: 100 <br />MinLength: 1 <br />Pattern: `^[a-zA-Z0-9][a-zA-Z0-9_.-]\{0,99\}$` <br />Type: string <br /> |
| `visibility` _string_ | Visibility controls who can see the repository.<br />- "public": Anyone can see the repository<br />- "private": Only people with explicit access can see the repository<br />- "internal": Only members of the organization can see the repository (Enterprise only)<br />See: https://docs.github.com/en/rest/repos/repos#create-an-organization-repository | private | Enum: [public private internal] <br />Type: string <br /> |
| `hasIssues` _boolean_ | HasIssues enables or disables the GitHub Issues feature for the repository.<br />When enabled, users can create and track issues. | true | Type: boolean <br /> |
| `hasProjects` _boolean_ | HasProjects enables or disables the GitHub Projects (classic) feature for the repository.<br />Note: This refers to classic projects, not the newer Projects feature. | false | Type: boolean <br /> |
| `hasWiki` _boolean_ | HasWiki enables or disables the GitHub Wiki feature for the repository.<br />When enabled, users can create wiki pages for documentation. | false | Type: boolean <br /> |
| `hasDownloads` _boolean_ | HasDownloads enables or disables the Downloads feature for the repository.<br />This feature is deprecated and has been replaced by Releases. | false | Type: boolean <br /> |
| `isTemplate` _boolean_ | IsTemplate marks the repository as a template repository.<br />Template repositories can be used as a starting point for new repositories.<br />See: https://docs.github.com/en/repositories/creating-and-managing-repositories/creating-a-template-repository | false | Type: boolean <br /> |
| `mergeCommitTitle` _string_ | MergeCommitTitle determines the default title for merge commits.<br />- "PR_TITLE": Use the pull request title<br />- "MERGE_MESSAGE": Use the default merge message format<br />See: https://docs.github.com/en/rest/repos/repos#update-a-repository | MERGE_MESSAGE | Enum: [PR_TITLE MERGE_MESSAGE] <br />Type: string <br /> |
| `mergeCommitMessage` _string_ | MergeCommitMessage determines the default message for merge commits.<br />- "PR_BODY": Use the pull request body<br />- "PR_TITLE": Use the pull request title<br />- "BLANK": Use a blank message<br />See: https://docs.github.com/en/rest/repos/repos#update-a-repository | PR_TITLE | Enum: [PR_BODY PR_TITLE BLANK] <br />Type: string <br /> |
| `allowedMergeStrategies` _[MergeStrategy](#mergestrategy) array_ | AllowedMergeStrategies lists the merge strategies allowed for pull requests.<br />See: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges | [map[type:merge] map[type:rebase]] |  |
| `deleteBranchOnMerge` _boolean_ | DeleteBranchOnMerge automatically deletes head branches after pull requests are merged.<br />This helps keep the repository clean by removing merged feature branches.<br />See: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-the-automatic-deletion-of-branches | true |  |
| `about` _[About](#about)_ | About contains descriptive information about the repository. |  |  |
| `archived` _boolean_ | Archived marks the repository as archived (read-only).<br />Archived repositories cannot receive new issues, pull requests, or commits.<br />See: https://docs.github.com/en/repositories/archiving-a-github-repository/archiving-repositories | false |  |
| `actionsEnabled` _boolean_ | ActionsEnabled determines whether this repository can use GitHub Actions.<br />This must be enabled at the organization level for this setting to take effect.<br />See: https://docs.github.com/en/rest/actions/permissions | true |  |
| `accessLevelForExternalWorkflows` _string_ | AccessLevelForExternalWorkflows controls access to workflows outside the repository.<br />- "none": Only workflows in this repository can access actions and reusable workflows<br />- "user": Workflows in user-owned private repositories can access them<br />- "organization": Workflows across the organization can access them<br />- "enterprise": Workflows across the enterprise can access them<br />See: https://docs.github.com/en/rest/actions/permissions | none | Enum: [none user organization enterprise] <br /> |
| `availableActionsRunnerGroups` _string array_ | AvailableActionsRunnerGroups lists runner group names that this repository can use.<br />This is only relevant when the organization's runner groups have "selected" visibility.<br />See: https://docs.github.com/en/rest/actions/self-hosted-runner-groups |  |  |
| `organizationRef` _[OrganizationRef](#organizationref)_ | OrganizationRef references the Organization CRD this repository belongs to. |  | Required: \{\} <br /> |
| `rulesetPresets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core) array_ | RulesetPresetList references RulesetPreset CRDs to apply to this repository.<br />These define branch protection rules, required status checks, and other policies.<br />See: https://docs.github.com/en/rest/repos/rules |  |  |
| `webhookPresets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core) array_ | WebhookPresetList references WebhookPreset CRDs to create webhooks for this repository.<br />Webhooks send HTTP POST payloads to external services when specific events occur.<br />See: https://docs.github.com/en/rest/webhooks/repos |  |  |
| `webhookIgnorePresets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core) array_ | WebhookIgnorePresetsList references WebhookIgnorePreset CRDs that define webhooks to ignore.<br />Webhooks matching these patterns will not be created even if they are in WebhookPresetList. |  |  |
| `autolinksPresets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core) array_ | AutolinksPresetList references AutolinksPreset CRDs to create autolinks for this repository.<br />Autolinks automatically convert references (like "JIRA-123") into clickable links.<br />See: https://docs.github.com/en/rest/repos/autolinks |  |  |
| `deployKeys` _[DeployKey](#deploykey) array_ | DeployKeyList defines deploy keys to create for this repository.<br />Deploy keys are SSH keys that grant access to a single repository.<br />See: https://docs.github.com/en/rest/deploy-keys/deploy-keys |  |  |
| `attachedCodeSecurityConfiguration` _[CodeSecurityConfigurationRef](#codesecurityconfigurationref)_ | AttachedCodeSecurityConfiguration references a CodeSecurityConfiguration to attach to this repository.<br />This is only used when the organization's configuration has "selected" attachment scope.<br />See: https://docs.github.com/en/rest/code-security/configurations |  |  |


#### RepositoryStatus



RepositoryStatus defines the observed state of Repository.



_Appears in:_
- [Repository](#repository)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `webhooks` _object (keys:string, values:[WebhookStatus](#webhookstatus))_ | Webhooks is a list of webhooks configured for this repository<br />the key is the hash of the configuration |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | conditions represent the current state of the Repository resource.<br />Each condition has a unique type and reflects the status of a specific aspect of the resource.<br />Standard condition types include:<br />- "Available": the resource is fully functional<br />- "Progressing": the resource is being created or updated<br />- "Degraded": the resource failed to reach or maintain its desired state<br />The status of each condition is one of True, False, or Unknown. |  | Optional: \{\} <br /> |
| `id` _integer_ | ID is the repository ID as created by GitHub. |  |  |
| `observedSubResourceGenerations` _object (keys:string, values:integer)_ | ObservedSubResourceGenerations is a map of sub-resource names to their observed generations.<br />Keys are in the format "<kind>/<namespace/<name>".<br />SubResources are kubernetes resources that are referenced by this Repository and are not managed<br />by their own controllers like WebhookPresets, RuleSetPresets and the attached CodeSecurityConfiguration |  |  |


#### RequiredStatusChecks



RequiredStatusChecks defines status check requirements that must pass before merging.
Status checks are CI/CD jobs, security scans, or other automated checks.
See: https://docs.github.com/en/rest/repos/rules#required-status-checks



_Appears in:_
- [RulesetRules](#rulesetrules)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `checks` _[StatusCheck](#statuscheck) array_ | Checks lists the required status checks that must pass. |  | AtMostOneOf: [integrationId appSlug] <br />MaxItems: 100 <br />MinItems: 1 <br />Required: \{\} <br /> |
| `strictPolicy` _boolean_ | StrictPolicy requires branches to be up to date with the base branch before merging.<br />When enabled, branches must include the latest changes from the base branch.<br />This prevents merge conflicts but may require additional merges/rebases. | false | Optional: \{\} <br /> |


#### RuleWorkflow



RuleWorkflow defines a single required workflow for the workflows rule.
The workflow is referenced by its path in a repository. The repository is identified by name
(resolved to a numeric ID at reconciliation time via the GitHub API).



_Appears in:_
- [WorkflowsRule](#workflowsrule)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `path` _string_ | Path is the path to the workflow file relative to the repository root.<br />Example: ".github/workflows/ci.yaml" |  | MaxLength: 500 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `repositoryName` _string_ | RepositoryName is the name of the repository containing the workflow.<br />Must be a repository within the same organization. The name will be resolved<br />to a numeric repository ID at reconciliation time via the GitHub API. |  | MaxLength: 100 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `ref` _string_ | Ref is the git ref (branch, tag, or SHA) to use for the workflow file.<br />Example: "refs/heads/main" |  | Optional: \{\} <br /> |


#### RulesetBypassActor



RulesetBypassActor defines an actor (user, team, or integration) who can bypass ruleset enforcement.
Either ActorID (for direct specification) or ActorSlug (for name-based resolution) must be provided for
ActorTypes "Integration" and "Team". ActorID must be provided for ActorType "RepositoryRole".
Both must be empty for ActorType "DeployKey".
See: https://docs.github.com/en/rest/repos/rules#create-an-organization-repository-ruleset

_Validation:_
- AtMostOneOf: [actorId actorSlug]

_Appears in:_
- [RulesetPresetSpec](#rulesetpresetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `actorId` _integer_ | ActorID is the numeric ID of the bypass actor.<br />This field is mutually exclusive with ActorSlug. |  |  |
| `actorSlug` _string_ | ActorSlug is the slug or name of the actor, which will be resolved to an ID.<br />This field is mutually exclusive with ActorID.<br />Only supported for ActorType "Integration" (GitHub Apps) and "Team" (organization teams).<br />For Integration, use the app slug (e.g., "my-github-app").<br />For Team, use the team slug (e.g., "platform-engineers"). |  |  |
| `actorType` _string_ | ActorType specifies the type of actor that can bypass the ruleset.<br />- "Integration": A GitHub App<br />- "OrganizationAdmin": Organization administrators<br />- "RepositoryRole": Users with a specific repository role<br />- "Team": An organization team<br />- "DeployKey": A deploy key<br />- "EnterpriseOwner": Enterprise owners (GitHub Enterprise only) |  | Enum: [Integration OrganizationAdmin RepositoryRole Team DeployKey EnterpriseOwner] <br />Required: \{\} <br /> |
| `bypassMode` _string_ | BypassMode determines when and how the actor can bypass the ruleset.<br />- "always": Actor can always bypass the ruleset<br />- "pull_request": Actor can bypass only when submitting via pull request |  | Enum: [always pull_request] <br />Optional: \{\} <br /> |


#### RulesetConditions



RulesetConditions define which refs are targeted by the Ruleset. For Organization-level rules they additionally define
which Repositories are targeted by the Ruleset via the fields RepositoryName and RepositoryProperty. If neither
RepositoryName nor RepositoryProperty are set for an Organization-level ruleset, the ruleset will target all repositories.

_Validation:_
- AtMostOneOf: [repositoryName repositoryProperty]

_Appears in:_
- [RulesetPresetSpec](#rulesetpresetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `refName` _[RefNameCondition](#refnamecondition)_ | RefName defines which git refs (branches or tags) a ruleset applies to. |  | MinProperties: 1 <br />Optional: \{\} <br /> |
| `repositoryName` _[RepositoryNameCondition](#repositorynamecondition)_ | RepositoryName targets repositories for Organization-level rulesets by their name.<br />The field is ignored for Repository-level rulesets. |  | Optional: \{\} <br /> |
| `repositoryProperty` _[RepositoryPropertyCondition](#repositorypropertycondition)_ | RepositoryProperty targets repositories for Organization-level rulesets by matching against custom properties.<br />The field is ignored for Repository-level rulesets. |  | Optional: \{\} <br /> |


#### RulesetEnforcement

_Underlying type:_ _string_

RulesetEnforcement defines the enforcement level



_Appears in:_
- [RulesetPresetSpec](#rulesetpresetspec)

| Field | Description |
| --- | --- |
| `disabled` | RulesetEnforcementDisabled means the ruleset is disabled<br /> |
| `active` | RulesetEnforcementActive means the ruleset is actively enforced<br /> |
| `evaluate` | RulesetEnforcementEvaluate means the ruleset is evaluated but not enforced<br /> |


#### RulesetPreset



RulesetPreset is the Schema for the rulesetpresets API



_Appears in:_
- [RulesetPresetList](#rulesetpresetlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `github.interhyp.de/v1alpha1` | | |
| `kind` _string_ | `RulesetPreset` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[RulesetPresetSpec](#rulesetpresetspec)_ | spec defines the desired state of RulesetPreset |  | Required: \{\} <br /> |
| `status` _[RulesetPresetStatus](#rulesetpresetstatus)_ | status defines the observed state of RulesetPreset |  | Optional: \{\} <br /> |


#### RulesetPresetList



RulesetPresetList contains a list of RulesetPreset





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `github.interhyp.de/v1alpha1` | | |
| `kind` _string_ | `RulesetPresetList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[RulesetPreset](#rulesetpreset) array_ |  |  |  |


#### RulesetPresetSpec



RulesetPresetSpec defines the desired state of RulesetPreset.
A ruleset preset defines reusable repository rules that can be applied to multiple repositories
or organizations. Rulesets enforce policies like branch protection, required reviews, and more.
See: https://docs.github.com/en/rest/repos/rules



_Appears in:_
- [RulesetPreset](#rulesetpreset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the display name of the ruleset shown in the GitHub UI. |  | MaxLength: 255 <br />MinLength: 1 <br />Pattern: `^[a-zA-Z0-9][a-zA-Z0-9\s\[\]*/'_.,~-]*[a-zA-Z0-9\[\]]$` <br />Required: \{\} <br /> |
| `target` _string_ | Target defines which ref types this ruleset applies to.<br />The Target 'repository' is only supported by Organization-level RulesetPresets. Repository-level<br />RulesetPresets with Target 'repository' are filtered out (i.e. are not checked nor applied). | branch | Enum: [branch tag push repository] <br />Optional: \{\} <br /> |
| `conditions` _[RulesetConditions](#rulesetconditions)_ | Conditions defines which refs are included or excluded in the list of targets for this Ruleset.<br />They also define which Repositories are targeted by Organization-level Rulesets. |  | AtMostOneOf: [repositoryName repositoryProperty] <br />Optional: \{\} <br /> |
| `enforcement` _[RulesetEnforcement](#rulesetenforcement)_ | Enforcement determines whether the ruleset is enforced.<br />- "disabled": Ruleset is not enforced<br />- "active": Ruleset is actively enforced; violations block operations<br />- "evaluate": Ruleset is evaluated but violations only generate warnings |  | Enum: [disabled active evaluate] <br />Required: \{\} <br /> |
| `bypassActors` _[RulesetBypassActor](#rulesetbypassactor) array_ | BypassActors defines actors (users, teams, apps) who can bypass this ruleset.<br />Bypass actors can perform operations that would otherwise be blocked by the ruleset.<br />See: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/about-rulesets#about-bypass-mode-for-rulesets |  | AtMostOneOf: [actorId actorSlug] <br />MaxItems: 100 <br />Optional: \{\} <br /> |
| `rules` _[RulesetRules](#rulesetrules)_ | Rules defines the specific rules to enforce in this ruleset. |  | Required: \{\} <br /> |


#### RulesetPresetStatus



RulesetPresetStatus defines the observed state of RulesetPreset.



_Appears in:_
- [RulesetPreset](#rulesetpreset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | conditions represent the current state of the RulesetPreset resource.<br />Each condition has a unique type and reflects the status of a specific aspect of the resource.<br />Standard condition types include:<br />- "Available": the resource is fully functional<br />- "Progressing": the resource is being created or updated<br />- "Degraded": the resource failed to reach or maintain its desired state<br />The status of each condition is one of True, False, or Unknown. |  | Optional: \{\} <br /> |


#### RulesetRules



RulesetRules defines the specific rules to enforce in a ruleset.
Each rule is optional and can be combined to create comprehensive protection policies.
See: https://docs.github.com/en/rest/repos/rules#available-rules



_Appears in:_
- [RulesetPresetSpec](#rulesetpresetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `creation` _boolean_ | Creation prevents the creation of matching refs.<br />When enabled, users cannot create branches or tags matching the ruleset target. | false | Optional: \{\} <br /> |
| `update` _boolean_ | Update prevents updates to matching refs.<br />When enabled, users cannot push commits to matching branches. | false | Optional: \{\} <br /> |
| `deletion` _boolean_ | Deletion prevents deletion of matching refs.<br />When enabled, users cannot delete matching branches or tags. | false | Optional: \{\} <br /> |
| `requiredLinearHistory` _boolean_ | RequiredLinearHistory requires branches to have a linear commit history.<br />When enabled, merge commits are not allowed; only rebasing and fast-forward merges are permitted. | false | Optional: \{\} <br /> |
| `requiredSignatures` _boolean_ | RequiredSignatures requires commits to be signed with a verified signature.<br />When enabled, only commits signed with GPG, SSH, or S/MIME are allowed.<br />See: https://docs.github.com/en/authentication/managing-commit-signature-verification | false | Optional: \{\} <br /> |
| `pullRequest` _[PullRequestRule](#pullrequestrule)_ | PullRequest defines pull request requirements for merging.<br />See: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches#require-pull-request-reviews-before-merging |  | Optional: \{\} <br /> |
| `requiredStatusChecks` _[RequiredStatusChecks](#requiredstatuschecks)_ | RequiredStatusChecks defines status checks that must pass before merging.<br />See: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches#require-status-checks-before-merging |  | Optional: \{\} <br /> |
| `nonFastForward` _boolean_ | NonFastForward prevents non-fast-forward updates.<br />When enabled, only fast-forward pushes are allowed, preventing force pushes. | false | Optional: \{\} <br /> |
| `commitMessagePattern` _[PatternRule](#patternrule)_ | CommitMessagePattern enforces a pattern for commit messages.<br />Use this to enforce commit message conventions like Conventional Commits. |  | Optional: \{\} <br /> |
| `commitAuthorEmailPattern` _[PatternRule](#patternrule)_ | CommitAuthorEmailPattern enforces a pattern for commit author email addresses.<br />Use this to ensure commits come from verified email domains. |  | Optional: \{\} <br /> |
| `committerEmailPattern` _[PatternRule](#patternrule)_ | CommitterEmailPattern enforces a pattern for committer email addresses. |  | Optional: \{\} <br /> |
| `branchNamePattern` _[PatternRule](#patternrule)_ | BranchNamePattern enforces a pattern for branch names.<br />Use this to enforce branch naming conventions like "feature/*" or "hotfix/*". |  | Optional: \{\} <br /> |
| `tagNamePattern` _[PatternRule](#patternrule)_ | TagNamePattern enforces a pattern for tag names.<br />Use this to enforce semantic versioning or other tag naming conventions. |  | Optional: \{\} <br /> |
| `copilotReview` _[CopilotCodeReviewRule](#copilotcodereviewrule)_ | CopilotReview automatically requests a GitHub Copilot pull request review<br />if the author has access to Copilot code review and their premium requests quota has not reached the limit. |  | Optional: \{\} <br /> |
| `workflows` _[WorkflowsRule](#workflowsrule)_ | Workflows defines required workflow rules that must pass before merging.<br />This rule type is only effective for organization-level rulesets and is ignored<br />when the preset is applied at the repository level.<br />See: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets#require-workflows-to-pass-before-merging |  | Optional: \{\} <br /> |


#### RunnerGroup



RunnerGroup configures a self-hosted runner group for GitHub Actions in an organization.
Runner groups allow you to control which repositories can use specific sets of self-hosted runners.
See: https://docs.github.com/en/rest/actions/self-hosted-runner-groups



_Appears in:_
- [ActionsSettings](#actionssettings)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the unique name of the runner group within the organization. |  | Required: \{\} <br /> |
| `visibility` _string_ | Visibility determines which repositories can access runners in this group.<br />- "all": All repositories in the organization can use these runners<br />- "private": Only private repositories can use these runners<br />- "selected": Only specific repositories can use these runners (selected via AvailableActionsRunnerGroups in RepositorySpec) | all | Enum: [all private selected] <br /> |
| `restrictedToWorkflows` _boolean_ | RestrictedToWorkflows determines whether this runner group can only run specific workflows.<br />If true, only workflows listed in SelectedWorkflows can use runners in this group.<br />This provides additional security by limiting which workflows can execute on sensitive runners. | false |  |
| `selectedWorkflows` _string array_ | SelectedWorkflows lists the workflows that can use runners in this group.<br />This field is only used when RestrictedToWorkflows is true.<br />Each entry must be a full workflow path with a reference (branch, tag, or SHA).<br />Example: "octo-org/octo-repo/.github/workflows/deploy.yaml@refs/heads/main" |  |  |


#### SecretScanningDelegatedBypassOptions



SecretScanningDelegatedBypassOptions configures reviewers who can approve secret scanning bypass requests.
When delegated bypass is enabled, contributors can request to bypass secret scanning push protection,
and the specified reviewers can approve or deny these requests.
See: https://docs.github.com/en/rest/code-security/configurations



_Appears in:_
- [CodeSecurityConfigurationSpec](#codesecurityconfigurationspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `reviewers` _[BypassReviewer](#bypassreviewer) array_ | Reviewers is a list of teams or organization roles that can review bypass requests. |  | ExactlyOneOf: [reviewerId reviewerName] <br /> |


#### SelectedAllowedActions



SelectedAllowedActions defines which specific actions are allowed when AllowedActions is set to "selected".
At least one setting must be configured to allow some actions.
See: https://docs.github.com/en/rest/actions/permissions



_Appears in:_
- [ActionsSettings](#actionssettings)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `githubOwnedAllowed` _boolean_ | GitHubOwnedAllowed determines whether actions created by GitHub are allowed to run.<br />This includes actions in the "actions" and "github" organizations. | false |  |
| `verifiedAllowed` _boolean_ | VerifiedAllowed determines whether actions from verified creators are allowed to run.<br />Verified creators are trusted partners and organizations with verified domains. | false |  |
| `patternsAllowed` _string array_ | PatternsAllowed is a list of glob patterns specifying allowed actions.<br />Each pattern can match action repositories using wildcards, e.g., "my-org/*" or "*/action-name@*". | [] |  |


#### StatusCheck



StatusCheck defines a required status check that must pass before merging.
A status check can be provided by a GitHub App or CI/CD integration.
See: https://docs.github.com/en/rest/repos/rules#required-status-checks

_Validation:_
- AtMostOneOf: [integrationId appSlug]

_Appears in:_
- [RequiredStatusChecks](#requiredstatuschecks)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `context` _string_ | Context is the name of the status check as reported by the CI/CD system or app.<br />Examples: "ci/circleci: build", "Security Scan", "Unit Tests" |  | MaxLength: 255 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `integrationId` _integer_ | IntegrationID is the numeric ID of the GitHub App integration providing the status check.<br />This field is mutually exclusive with AppSlug. |  | Minimum: 1 <br />Optional: \{\} <br /> |
| `appSlug` _string_ | AppSlug is the slug of the GitHub App integration providing the status check.<br />This field is mutually exclusive with IntegrationID.<br />The slug will be resolved to the corresponding integration ID.<br />Only supported for GitHub App integrations.<br />Example: "my-ci-app" |  | Optional: \{\} <br /> |


#### Team







_Appears in:_
- [TeamList](#teamlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `github.interhyp.de/v1alpha1` | | |
| `kind` _string_ | `Team` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[TeamSpec](#teamspec)_ | spec defines the desired state of Team |  | ExactlyOneOf: [idpGroup members] <br />Required: \{\} <br /> |
| `status` _[TeamStatus](#teamstatus)_ | status defines the observed state of Team |  | Optional: \{\} <br /> |


#### TeamList



TeamList contains a list of Teams





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `github.interhyp.de/v1alpha1` | | |
| `kind` _string_ | `TeamList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Team](#team) array_ |  |  |  |


#### TeamSpec



TeamSpec defines the desired state of Team within one or more Organizations.
Teams group organization members and can be assigned permissions to repositories.
A Team can exist in multiple organizations simultaneously.
See: https://docs.github.com/en/rest/teams/teams

_Validation:_
- ExactlyOneOf: [idpGroup members]

_Appears in:_
- [Team](#team)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the display name of the team in GitHub.<br />GitHub automatically generates a "slug" from this name for use in URLs and APIs. |  | MaxLength: 100 <br />MinLength: 1 <br />Pattern: `^[a-zA-Z0-9][a-zA-Z0-9_.-]\{0,99\}$` <br />Required: \{\} <br />Type: string <br /> |
| `members` _string array_ | Members is a list of GitHub usernames to add to the team.<br />This field is mutually exclusive with IDPGroup.<br />When set, team membership is managed manually through this list.<br />Members not in this list will be removed from the team. |  | MaxItems: 100 <br /> |
| `idpGroup` _string_ | IDPGroup is the name of the Identity Provider group to synchronize with this team.<br />This field is mutually exclusive with Members.<br />When set, team membership is automatically synchronized from the IDP group.<br />See: https://docs.github.com/en/organizations/organizing-members-into-teams/synchronizing-a-team-with-an-identity-provider-group |  | MaxLength: 100 <br />Pattern: `^[a-zA-Z0-9][a-zA-Z0-9_.-]\{0,99\}$` <br />Type: string <br /> |
| `description` _string_ | Description provides additional information about the team's purpose.<br />This appears on the team's page in GitHub. |  | MaxLength: 1000 <br />Optional: \{\} <br />Type: string <br /> |
| `privacy` _string_ | Privacy controls the visibility of the team within the organization.<br />- "closed": The team is visible to all members of the organization, but only team members can see team discussions and manage team membership.<br />- "secret": The team is only visible to organization owners and team members.<br />See: https://docs.github.com/en/rest/teams/teams#create-a-team | closed | Enum: [closed secret] <br />Optional: \{\} <br /> |
| `permission` _string_ | Permission specifies the default permission granted to team members for organization repositories.<br />- "pull": Team members can pull (read) from organization repositories.<br />- "push": Team members can pull and push (read and write) to organization repositories.<br />Note: This is a legacy field. Use organization roles for more fine-grained permissions.<br />See: https://docs.github.com/en/rest/teams/teams#create-a-team | pull | Enum: [pull push] <br />Optional: \{\} <br /> |
| `notificationSetting` _string_ | NotificationSetting controls whether team members receive notifications for the team.<br />- "notifications_disabled": No one receives notifications.<br />- "notifications_enabled": Everyone receives notifications when the team is @mentioned.<br />See: https://docs.github.com/en/rest/teams/teams#create-a-team | notifications_disabled | Enum: [notifications_disabled notifications_enabled] <br />Optional: \{\} <br /> |
| `organizationRoles` _string array_ | OrganizationRoles is a list of organization role names to assign to this team.<br />Organization roles define the permissions the team has within the organization.<br />If not specified, defaults to empty list.<br />Set to an empty list to remove all role assignments.<br />See: https://docs.github.com/en/rest/orgs/organization-roles |  | Optional: \{\} <br /> |
| `organizationRefs` _[OrganizationRef](#organizationref) array_ | OrganizationRefs is a list of Organization CRDs that this team belongs to.<br />The team will be created or updated in all referenced organizations.<br />Removing an organization from this list will delete the team from that organization<br />while preserving it in other organizations. |  | MinItems: 1 <br />Required: \{\} <br /> |


#### TeamStatus



TeamStatus defines the observed state of Team.



_Appears in:_
- [Team](#team)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | conditions represent the current state of the Team resource.<br />Each condition has a unique type and reflects the status of a specific aspect of the resource.<br />Standard condition types include:<br />- "Available": the resource is fully functional<br />- "Progressing": the resource is being created or updated<br />- "Degraded": the resource failed to reach or maintain its desired state<br />The status of each condition is one of True, False, or Unknown. |  | Optional: \{\} <br /> |
| `previousOrganizationRefs` _[OrganizationRef](#organizationref) array_ | PreviousOrganizationRefs tracks the organization references from the last successful reconciliation.<br />This allows the reconciler to detect when organizations are removed from the spec<br />and clean up teams from those organizations while preserving them in remaining organizations. |  | Optional: \{\} <br /> |
| `slug` _string_ | Slug is the URL-friendly version of the team name as assigned by GitHub.<br />This slug is used in URLs and API calls. GitHub generates it automatically from the Name field.<br />Example: A team named "Platform Engineers" might have the slug "platform-engineers". |  |  |


#### Topic



Topic represents a repository topic (tag) for categorization.
See: https://docs.github.com/en/rest/repos/repos#replace-all-repository-topics



_Appears in:_
- [About](#about)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the topic name.<br />Topics must be lowercase and can contain letters, numbers, and hyphens.<br />They must start with a letter or number. |  | MaxLength: 50 <br />Pattern: `^[a-z0-9][a-z0-9-]\{0,49\}$` <br />Type: string <br /> |


#### WebhookIgnorePreset



WebhookIgnorePreset is the Schema for the webhookignorepresets API



_Appears in:_
- [WebhookIgnorePresetList](#webhookignorepresetlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `github.interhyp.de/v1alpha1` | | |
| `kind` _string_ | `WebhookIgnorePreset` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[WebhookIgnorePresetSpec](#webhookignorepresetspec)_ | spec defines the desired state of WebhookIgnorePreset |  | Required: \{\} <br /> |
| `status` _[WebhookIgnorePresetStatus](#webhookignorepresetstatus)_ | status defines the observed state of WebhookIgnorePreset |  | Optional: \{\} <br /> |


#### WebhookIgnorePresetList



WebhookIgnorePresetList contains a list of WebhookIgnorePreset





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `github.interhyp.de/v1alpha1` | | |
| `kind` _string_ | `WebhookIgnorePresetList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[WebhookIgnorePreset](#webhookignorepreset) array_ |  |  |  |


#### WebhookIgnorePresetSpec



WebhookIgnorePresetSpec defines the desired state of WebhookIgnorePreset.
WebhookIgnorePresets allow you to exclude certain webhooks from being created,
even if they are referenced in a repository's WebhookPresetList.
This is useful for globally excluding webhooks based on URL patterns.



_Appears in:_
- [WebhookIgnorePreset](#webhookignorepreset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ignoreURLRegex` _string_ | IgnoreURLRegex is a regular expression pattern to match against webhook payload URLs.<br />Webhooks with URLs matching this pattern will not be created, even if they are<br />referenced in a repository's WebhookPresetList.<br />Example: "^https://deprecated\\.example\\.com/.*" to ignore all webhooks to deprecated.example.com |  | Optional: \{\} <br /> |


#### WebhookIgnorePresetStatus



WebhookIgnorePresetStatus defines the observed state of WebhookIgnorePreset.



_Appears in:_
- [WebhookIgnorePreset](#webhookignorepreset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | conditions represent the current state of the WebhookIgnorePreset resource.<br />Each condition has a unique type and reflects the status of a specific aspect of the resource.<br />Standard condition types include:<br />- "Available": the resource is fully functional<br />- "Progressing": the resource is being created or updated<br />- "Degraded": the resource failed to reach or maintain its desired state<br />The status of each condition is one of True, False, or Unknown. |  | Optional: \{\} <br /> |


#### WebhookPreset



WebhookPreset is the Schema for the webhookpresets API



_Appears in:_
- [WebhookPresetList](#webhookpresetlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `github.interhyp.de/v1alpha1` | | |
| `kind` _string_ | `WebhookPreset` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[WebhookPresetSpec](#webhookpresetspec)_ | spec defines the desired state of WebhookPreset |  | Required: \{\} <br /> |
| `status` _[WebhookPresetStatus](#webhookpresetstatus)_ | status defines the observed state of WebhookPreset |  | Optional: \{\} <br /> |


#### WebhookPresetList



WebhookPresetList contains a list of WebhookPreset





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `github.interhyp.de/v1alpha1` | | |
| `kind` _string_ | `WebhookPresetList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[WebhookPreset](#webhookpreset) array_ |  |  |  |


#### WebhookPresetSecretSpec



WebhookPresetSecretSpec references a Kubernetes Secret containing the webhook secret.



_Appears in:_
- [WebhookPresetSpec](#webhookpresetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the Kubernetes Secret containing the webhook secret. |  | MaxLength: 250 <br />MinLength: 1 <br />Pattern: `^[a-zA-Z0-9.-]+$` <br />Required: \{\} <br />Type: string <br /> |
| `key` _string_ | Key is the key within the Secret that contains the webhook secret value. |  | MaxLength: 250 <br />MinLength: 1 <br />Pattern: `^[a-zA-Z0-9.-]+$` <br />Required: \{\} <br />Type: string <br /> |
| `namespace` _string_ | Namespace is the namespace of the Secret.<br />If not specified, the namespace of the WebhookPreset is used. |  | Optional: \{\} <br />Type: string <br /> |


#### WebhookPresetSpec



WebhookPresetSpec defines the desired state of WebhookPreset.
Webhooks allow external services to be notified when certain events occur in a repository.
See: https://docs.github.com/en/rest/webhooks/repos



_Appears in:_
- [WebhookPreset](#webhookpreset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `payloadUrl` _string_ | PayloadURL is the URL that will receive the webhook POST requests.<br />Must be a publicly accessible HTTP or HTTPS endpoint.<br />GitHub will send HTTP POST requests to this URL when subscribed events occur. |  | MaxLength: 2048 <br />MinLength: 1 <br />Pattern: `^https?://[a-zA-Z0-9.-]+(:[0-9]+)?(/.*)?$` <br />Required: \{\} <br />Type: string <br /> |
| `secret` _[WebhookPresetSecretSpec](#webhookpresetsecretspec)_ | Secret is a reference to a Kubernetes Secret containing the webhook secret.<br />The webhook secret is used by GitHub to sign webhook payloads.<br />Your service can verify this signature to ensure the request came from GitHub.<br />This field takes precedence over SecretValue if both are provided.<br />See: https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries |  |  |
| `secretValue` _string_ | SecretValue is the plaintext value of the webhook secret.<br />Use this for simple cases, but Secret (referencing a Kubernetes Secret) is more secure.<br />If both Secret and SecretValue are provided, Secret takes precedence. |  | Type: string <br /> |
| `contentType` _string_ | ContentType specifies the format of the webhook payload.<br />- "json": Send payload as application/json (recommended)<br />- "form": Send payload as application/x-www-form-urlencoded<br />See: https://docs.github.com/en/webhooks/webhook-events-and-payloads |  | Enum: [json form] <br />Type: string <br /> |
| `active` _boolean_ | Active determines whether the webhook is active and will send events.<br />Set to false to temporarily disable the webhook without deleting it. | true |  |
| `events` _string array_ | Events is a list of GitHub event types that trigger this webhook.<br />If empty, the webhook subscribes to all events ("*").<br />Common events include "push", "pull_request", "issues", "release".<br />See: https://docs.github.com/en/webhooks/webhook-events-and-payloads |  | MaxItems: 100 <br />MinItems: 0 <br />Type: array <br />items:Enum: [branch_protection_rule check_run check_suite code_scanning_alert commit_comment create delete dependabot_alert deploy_key deployment deployment_status discussion discussion_comment fork github_app_authorization gollum installation installation_repositories issue_comment issues label marketplace_purchase member membership merge_group meta milestone organization org_block package page_build ping project project_card project_column public pull_request pull_request_review pull_request_review_comment pull_request_review_thread push registry_package release repository repository_dispatch repository_import repository_vulnerability_alert secret_scanning_alert security_advisory sponsorship star status team team_add watch workflow_dispatch workflow_job workflow_run] <br /> |
| `sslVerify` _boolean_ | SSLVerify enables SSL certificate verification for the webhook endpoint.<br />When true, GitHub verifies the SSL certificate of the PayloadURL.<br />Disable only for testing with self-signed certificates; always enable in production. | true |  |


#### WebhookPresetStatus



WebhookPresetStatus defines the observed state of WebhookPreset.



_Appears in:_
- [WebhookPreset](#webhookpreset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | conditions represent the current state of the WebhookPreset resource.<br />Each condition has a unique type and reflects the status of a specific aspect of the resource.<br />Standard condition types include:<br />- "Available": the resource is fully functional<br />- "Progressing": the resource is being created or updated<br />- "Degraded": the resource failed to reach or maintain its desired state<br />The status of each condition is one of True, False, or Unknown. |  | Optional: \{\} <br /> |


#### WebhookStatus



WebhookStatus defines the status of a webhook configured for a repository



_Appears in:_
- [RepositoryStatus](#repositorystatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretHash` _string_ | Secret is a hash of the secret used for the webhook |  |  |


#### WorkflowsRule



WorkflowsRule defines required workflow rules that must pass before merging.
Workflows are GitHub Actions workflows that are required to run and pass.
This rule type is only effective for organization-level rulesets and is ignored
when the preset is applied at the repository level.
See: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets#require-workflows-to-pass-before-merging



_Appears in:_
- [RulesetRules](#rulesetrules)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `doNotEnforceOnCreate` _boolean_ | DoNotEnforceOnCreate disables enforcement of this rule for newly created refs.<br />When true, the workflow requirement is not enforced on the first push creating the ref. | false | Optional: \{\} <br /> |
| `workflows` _[RuleWorkflow](#ruleworkflow) array_ | Workflows lists the required workflows that must pass. |  | MaxItems: 100 <br />MinItems: 1 <br />Required: \{\} <br /> |


