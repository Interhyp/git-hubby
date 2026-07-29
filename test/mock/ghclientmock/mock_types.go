package ghclientmock

// Call tracking types
type OrgCall struct {
	Method string
	Org    string
}

type RoleAssignmentCall struct {
	Method string
	Org    string
}

type RepoCall struct {
	Method string
	Owner  string
	Repo   string
}

type WebhookCall struct {
	Method string
	Owner  string
	Repo   string
	ID     int64
}

type RateLimitCall struct {
	Method string
}

type CustomPropCall struct {
	Method string
	Org    string
}

type RulesetCall struct {
	Method    string
	Owner     string
	Repo      string
	RulesetID int64
}

type OrgRulesetCall struct {
	Method    string
	Org       string
	RulesetID int64
}

type CodeSecurityConfigurationCall struct {
	Method   string
	Org      string
	ConfigID int64
}

type TeamCall struct {
	Method      string
	Org         string
	Slug        string
	Description string
	Owner       string
	Repo        string
	Permission  string
}

type CollaboratorCall struct {
	Method     string
	Owner      string
	Repo       string
	Username   string
	Permission string
}

type TeamMemberCall struct {
	Method   string
	Org      string
	Slug     string
	Username string
}

type ExternalGroupCall struct {
	Method  string
	Org     string
	Slug    string
	GroupID int64
}

type RoleCall struct {
	Method   string
	Org      string
	RoleName string
}

type ActionsCall struct {
	Method string
	Org    string
	Owner  string
	Repo   string
	RepoID int64
}

type AppsCall struct {
	Method     string
	Enterprise string
	Org        string
}

// Thread-safe helper methods for recording calls
func (m *MockGitHubClientWrapper) recordOrgCall(call OrgCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.OrganizationCalls = append(m.OrganizationCalls, call)
}

func (m *MockGitHubClientWrapper) recordRoleAssignmentCall(call RoleAssignmentCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RoleAssignmentCalls = append(m.RoleAssignmentCalls, call)
}

func (m *MockGitHubClientWrapper) recordCustomPropCall(call CustomPropCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CustomPropertiesCalls = append(m.CustomPropertiesCalls, call)
}

func (m *MockGitHubClientWrapper) recordRepoCall(call RepoCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RepositoryCalls = append(m.RepositoryCalls, call)
}

func (m *MockGitHubClientWrapper) recordWebhookCall(call WebhookCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.WebhookCalls = append(m.WebhookCalls, call)
}

func (m *MockGitHubClientWrapper) recordRulesetCall(call RulesetCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RulesetCalls = append(m.RulesetCalls, call)
}

func (m *MockGitHubClientWrapper) recordOrgRulesetCall(call OrgRulesetCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.OrganizationRulesetCalls = append(m.OrganizationRulesetCalls, call)
}

func (m *MockGitHubClientWrapper) recordRateLimitCall(call RateLimitCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RateLimitCalls = append(m.RateLimitCalls, call)
}

func (m *MockGitHubClientWrapper) recordCodeSecurityConfigurationCall(call CodeSecurityConfigurationCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CodeSecurityConfigurationCalls = append(m.CodeSecurityConfigurationCalls, call)
}

func (m *MockGitHubClientWrapper) recordTeamCall(call TeamCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TeamCalls = append(m.TeamCalls, call)
}

func (m *MockGitHubClientWrapper) recordTeamMemberCall(call TeamMemberCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TeamMemberCalls = append(m.TeamMemberCalls, call)
}

func (m *MockGitHubClientWrapper) recordExternalGroupCall(call ExternalGroupCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ExternalGroupCalls = append(m.ExternalGroupCalls, call)
}

func (m *MockGitHubClientWrapper) recordRoleCall(call RoleCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RoleCalls = append(m.RoleCalls, call)
}

func (m *MockGitHubClientWrapper) recordActionsCall(call ActionsCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ActionsCalls = append(m.ActionsCalls, call)
}

func (m *MockGitHubClientWrapper) recordAppsCall(call AppsCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.EnterpriseAppsCalls = append(m.EnterpriseAppsCalls, call)
}

// Thread-safe getter methods for reading calls (returns copies to prevent external modification)
func (m *MockGitHubClientWrapper) GetOrganizationCalls() []OrgCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]OrgCall, len(m.OrganizationCalls))
	copy(calls, m.OrganizationCalls)
	return calls
}
func (m *MockGitHubClientWrapper) GetRoleAssignmentCalls() []RoleAssignmentCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]RoleAssignmentCall, len(m.RoleAssignmentCalls))
	copy(calls, m.RoleAssignmentCalls)
	return calls
}

func (m *MockGitHubClientWrapper) GetCustomPropertiesCalls() []CustomPropCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]CustomPropCall, len(m.CustomPropertiesCalls))
	copy(calls, m.CustomPropertiesCalls)
	return calls
}

func (m *MockGitHubClientWrapper) GetRepositoryCalls() []RepoCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]RepoCall, len(m.RepositoryCalls))
	copy(calls, m.RepositoryCalls)
	return calls
}

func (m *MockGitHubClientWrapper) GetWebhookCalls() []WebhookCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]WebhookCall, len(m.WebhookCalls))
	copy(calls, m.WebhookCalls)
	return calls
}

func (m *MockGitHubClientWrapper) GetRulesetCalls() []RulesetCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]RulesetCall, len(m.RulesetCalls))
	copy(calls, m.RulesetCalls)
	return calls
}

func (m *MockGitHubClientWrapper) GetOrganizationRulesetCalls() []OrgRulesetCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]OrgRulesetCall, len(m.OrganizationRulesetCalls))
	copy(calls, m.OrganizationRulesetCalls)
	return calls
}

func (m *MockGitHubClientWrapper) GetRateLimitCalls() []RateLimitCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]RateLimitCall, len(m.RateLimitCalls))
	copy(calls, m.RateLimitCalls)
	return calls
}

func (m *MockGitHubClientWrapper) GetCodeSecurityConfigurationCalls() []CodeSecurityConfigurationCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]CodeSecurityConfigurationCall, len(m.CodeSecurityConfigurationCalls))
	copy(calls, m.CodeSecurityConfigurationCalls)
	return calls
}

func (m *MockGitHubClientWrapper) GetTeamCalls() []TeamCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]TeamCall, len(m.TeamCalls))
	copy(calls, m.TeamCalls)
	return calls
}

func (m *MockGitHubClientWrapper) GetTeamMemberCalls() []TeamMemberCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]TeamMemberCall, len(m.TeamMemberCalls))
	copy(calls, m.TeamMemberCalls)
	return calls
}

func (m *MockGitHubClientWrapper) GetExternalGroupCalls() []ExternalGroupCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]ExternalGroupCall, len(m.ExternalGroupCalls))
	copy(calls, m.ExternalGroupCalls)
	return calls
}

func (m *MockGitHubClientWrapper) GetRoleCalls() []RoleCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]RoleCall, len(m.RoleCalls))
	copy(calls, m.RoleCalls)
	return calls
}

func (m *MockGitHubClientWrapper) GetActionsCalls() []ActionsCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]ActionsCall, len(m.ActionsCalls))
	copy(calls, m.ActionsCalls)
	return calls
}

func (m *MockGitHubClientWrapper) GetEnterpriseAppsCalls() []AppsCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]AppsCall, len(m.EnterpriseAppsCalls))
	copy(calls, m.EnterpriseAppsCalls)
	return calls
}
