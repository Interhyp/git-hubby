package conditions

type ConditionType string

// Common condition types for both Organization and Repository resources
const (
	// Ready indicates that the resource is ready and all components are synced
	TypeReady ConditionType = "Ready"

	// GitHubSynced indicates that the core resource metadata is synced with GitHub
	TypeBaseSettingsSynced ConditionType = "GitHubSynced"

	// RulesetsSynced indicates that rulesets are synced successfully
	TypeRulesetsSynced ConditionType = "RulesetsSynced"

	// ActionsConfigurationSynced indicates that rulesets are synced successfully
	TypeActionsConfigurationSynced ConditionType = "ActionsConfigurationSynced"
)

// Organization-specific condition types
const (
	// CustomPropertyDefinitionsSynced indicates that organization custom properties are synced
	TypeCustomPropertyDefinitionsSynced ConditionType = "CustomPropertyDefinitionsSynced"
	// CustomPropertyDefinitionsSynced indicates that organization custom properties are synced
	TypeCodeSecurityConfigurationsSynced ConditionType = "CodeSecurityConfigurationsSynced"
)

// Repository-specific condition types
const (
	// WebhooksSynced indicates that repository webhooks are configured successfully
	TypeWebhooksSynced ConditionType = "WebhooksSynced"
	// TeamsSynced indicates that repository team permissions are configured successfully
	TypeTeamsSynced ConditionType = "TeamsSynced"
	// CollaboratorsSynced indicates that repository collaborator permissions are configured successfully
	TypeCollaboratorsSynced ConditionType = "CollaboratorsSynced"
	// CustomPropertiesValuesSynced indicates that repository custom properties are configured successfully
	TypeCustomPropertiesValuesSynced ConditionType = "CustomPropertiesValuesSynced"
	// TopicsSynced indicates that repository topics are synced successfully
	TypeTopicsSynced ConditionType = "TopicsSynced"
	// AutolinksSynced indicates that repository autolinks are synced successfully
	TypeAutolinksSynced ConditionType = "AutolinksSynced"
	// DeployKeysSynced indicates that repository deploy keys are synced successfully
	TypeDeployKeysSynced ConditionType = "DeployKeysSynced"
)

// Team-specific condition types
const (
	// TypeTeamMembersSynced indicates that team members are synced successfully
	TypeTeamMembersSynced ConditionType = "TeamMembersSynced"
	// TypeOutdatedOrganizationRefsSynced indicates that the teams has been removed from organization that are no longer referenced
	TypeOutdatedOrganizationRefsSynced ConditionType = "OutdatedOrganizationRefsSynced"
	// TypeTeamRoleAssignmentsSynced indicates that all-repo-write role has been assigned to all repos successfully
	TypeTeamRoleAssignmentsSynced ConditionType = "TeamRoleAssignmentsSynced"
)

// IDPTeam-specific condition types
const (
	// TypeIDPTeamMembersSynced indicates that IDP team members are synced successfully
	TypeIDPTeamGroupSettingsSynced   ConditionType = "TypeIDPTeamGroupSettingsSynced"
	TypeIDPTeamRoleAssignmentsSynced ConditionType = "TypeIDPTeamRoleAssignmentsSynced"
)

// Common condition reasons
const (
	ReasonReconcileStarted   = "ReconcileStarted"
	ReasonReconcileCompleted = "ReconcileCompleted"
	ReasonReconcileFailed    = "ReconcileFailed"

	// Sync-related reasons
	ReasonSyncCompleted  = "SyncCompleted"
	ReasonSyncFailed     = "SyncFailed"
	ReasonSyncInProgress = "SyncInProgress"
)
