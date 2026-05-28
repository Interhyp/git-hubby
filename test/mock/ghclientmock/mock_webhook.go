package ghclientmock

import (
	"context"

	"github.com/google/go-github/v86/github"
)

// Webhook operations

func (m *MockGitHubClientWrapper) ListHooks(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.Hook, error) {
	m.recordWebhookCall(WebhookCall{Method: "ListHooks", Owner: owner, Repo: repo})

	if m.ListHooksFunc != nil {
		return m.ListHooksFunc(ctx, owner, repo, opts)
	}

	// Default implementation - return empty list
	return []*github.Hook{}, nil
}

func (m *MockGitHubClientWrapper) CreateHook(ctx context.Context, owner, repo string, hook *github.Hook) (*github.Hook, error) {
	m.recordWebhookCall(WebhookCall{Method: "CreateHook", Owner: owner, Repo: repo})

	if m.CreateHookFunc != nil {
		return m.CreateHookFunc(ctx, owner, repo, hook)
	}

	// Default implementation - return the created hook with an ID
	createdHook := *hook
	createdHook.ID = new(int64(123))

	return &createdHook, nil
}

func (m *MockGitHubClientWrapper) DeleteHook(ctx context.Context, owner, repo string, id int64) error {
	m.recordWebhookCall(WebhookCall{Method: "DeleteHook", Owner: owner, Repo: repo, ID: id})

	if m.DeleteHookFunc != nil {
		return m.DeleteHookFunc(ctx, owner, repo, id)
	}

	// Default implementation
	return nil
}
