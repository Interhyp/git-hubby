package ghclientmock

import (
	"context"

	"github.com/google/go-github/v89/github"
)

// Repository ruleset operations

func (m *MockGitHubClientWrapper) GetRepositoryRuleset(ctx context.Context, owner, repo string, rulesetID int64, includesParents bool) (*github.RepositoryRuleset, error) {
	m.recordRulesetCall(RulesetCall{Method: "GetRepositoryRuleset", Owner: owner, Repo: repo, RulesetID: rulesetID})

	if m.GetRepositoryRulesetFunc != nil {
		return m.GetRepositoryRulesetFunc(ctx, owner, repo, rulesetID, includesParents)
	}

	// Default implementation - return a ruleset with the given ID
	return &github.RepositoryRuleset{
		ID:   new(rulesetID),
		Name: "test-ruleset",
	}, nil
}

func (m *MockGitHubClientWrapper) GetAllRepositoryRulesets(ctx context.Context, owner, repo string, includesParents bool) ([]*github.RepositoryRuleset, error) {
	m.recordRulesetCall(RulesetCall{Method: "GetAllRepositoryRulesets", Owner: owner, Repo: repo})

	if m.GetAllRepositoryRulesetsFunc != nil {
		return m.GetAllRepositoryRulesetsFunc(ctx, owner, repo, includesParents)
	}

	// Default implementation - return empty list
	return []*github.RepositoryRuleset{}, nil
}

func (m *MockGitHubClientWrapper) CreateRepositoryRuleset(ctx context.Context, owner, repo string, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
	m.recordRulesetCall(RulesetCall{Method: "CreateRepositoryRuleset", Owner: owner, Repo: repo})

	if m.CreateRepositoryRulesetFunc != nil {
		return m.CreateRepositoryRulesetFunc(ctx, owner, repo, ruleset)
	}

	// Default implementation - return the created ruleset with an ID
	createdRuleset := *ruleset
	createdRuleset.ID = new(int64(456))

	return &createdRuleset, nil
}

func (m *MockGitHubClientWrapper) UpdateRepositoryRuleset(ctx context.Context, owner, repo string, rulesetID int64, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
	m.recordRulesetCall(RulesetCall{Method: "UpdateRepositoryRuleset", Owner: owner, Repo: repo, RulesetID: rulesetID})

	if m.UpdateRepositoryRulesetFunc != nil {
		return m.UpdateRepositoryRulesetFunc(ctx, owner, repo, rulesetID, ruleset)
	}

	// Default implementation - return the updated ruleset
	updatedRuleset := *ruleset
	updatedRuleset.ID = new(rulesetID)

	return &updatedRuleset, nil
}

func (m *MockGitHubClientWrapper) DeleteRepositoryRuleset(ctx context.Context, owner, repo string, rulesetID int64) error {
	m.recordRulesetCall(RulesetCall{Method: "DeleteRepositoryRuleset", Owner: owner, Repo: repo, RulesetID: rulesetID})

	if m.DeleteRepositoryRulesetFunc != nil {
		return m.DeleteRepositoryRulesetFunc(ctx, owner, repo, rulesetID)
	}

	// Default implementation
	return nil
}

// Organization ruleset operations

func (m *MockGitHubClientWrapper) GetOrganizationRuleset(ctx context.Context, org string, rulesetID int64) (*github.RepositoryRuleset, error) {
	m.recordOrgRulesetCall(OrgRulesetCall{Method: "GetOrganizationRuleset", Org: org, RulesetID: rulesetID})

	if m.GetOrganizationRulesetFunc != nil {
		return m.GetOrganizationRulesetFunc(ctx, org, rulesetID)
	}

	// Default implementation - return a ruleset with the given ID
	return &github.RepositoryRuleset{
		ID:   new(rulesetID),
		Name: "org-test-ruleset",
	}, nil
}

func (m *MockGitHubClientWrapper) GetAllOrganizationRulesets(ctx context.Context, org string, includesParents bool) ([]*github.RepositoryRuleset, error) {
	m.recordOrgRulesetCall(OrgRulesetCall{Method: "GetAllOrganizationRulesets", Org: org})

	if m.GetAllOrganizationRulesetsFunc != nil {
		return m.GetAllOrganizationRulesetsFunc(ctx, org, includesParents)
	}

	// Default implementation - return empty list
	return []*github.RepositoryRuleset{}, nil
}

func (m *MockGitHubClientWrapper) CreateOrganizationRuleset(ctx context.Context, org string, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
	m.recordOrgRulesetCall(OrgRulesetCall{Method: "CreateOrganizationRuleset", Org: org})

	if m.CreateOrganizationRulesetFunc != nil {
		return m.CreateOrganizationRulesetFunc(ctx, org, ruleset)
	}

	// Default implementation - return the created ruleset with an ID
	createdRuleset := *ruleset
	createdRuleset.ID = new(int64(789))

	return &createdRuleset, nil
}

func (m *MockGitHubClientWrapper) UpdateOrganizationRuleset(ctx context.Context, org string, rulesetID int64, ruleset *github.RepositoryRuleset) (*github.RepositoryRuleset, error) {
	m.recordOrgRulesetCall(OrgRulesetCall{Method: "UpdateOrganizationRuleset", Org: org, RulesetID: rulesetID})

	if m.UpdateOrganizationRulesetFunc != nil {
		return m.UpdateOrganizationRulesetFunc(ctx, org, rulesetID, ruleset)
	}

	// Default implementation - return the updated ruleset
	updatedRuleset := *ruleset
	updatedRuleset.ID = new(rulesetID)

	return &updatedRuleset, nil
}

func (m *MockGitHubClientWrapper) DeleteOrganizationRuleset(ctx context.Context, org string, rulesetID int64) error {
	m.recordOrgRulesetCall(OrgRulesetCall{Method: "DeleteOrganizationRuleset", Org: org, RulesetID: rulesetID})

	if m.DeleteOrganizationRulesetFunc != nil {
		return m.DeleteOrganizationRulesetFunc(ctx, org, rulesetID)
	}

	// Default implementation
	return nil
}
