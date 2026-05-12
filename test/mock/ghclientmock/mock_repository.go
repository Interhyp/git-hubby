package ghclientmock

import (
	"context"
	"fmt"

	"github.com/google/go-github/v86/github"
)

// Repository operations

func (m *MockGitHubClientWrapper) GetRepository(ctx context.Context, owner, repo string) (*github.Repository, error) {
	m.recordRepoCall(RepoCall{Method: "GetRepository", Owner: owner, Repo: repo})

	if m.GetRepositoryFunc != nil {
		return m.GetRepositoryFunc(ctx, owner, repo)
	}

	// Default implementation
	return &github.Repository{
		ID:       github.Ptr(int64(12345)),
		Name:     github.Ptr(repo),
		FullName: github.Ptr(fmt.Sprintf("%s/%s", owner, repo)),
		Owner: &github.User{
			Login: github.Ptr(owner),
		},
		Archived: github.Ptr(false),
	}, nil
}

func (m *MockGitHubClientWrapper) CreateRepository(ctx context.Context, org string, repo *github.Repository) (*github.Repository, error) {
	m.recordRepoCall(RepoCall{Method: "CreateRepository", Owner: org, Repo: repo.GetName()})

	if m.CreateRepositoryFunc != nil {
		return m.CreateRepositoryFunc(ctx, org, repo)
	}

	// Default implementation - return the created repository
	createdRepo := *repo
	createdRepo.ID = github.Ptr(int64(12345))
	createdRepo.FullName = github.Ptr(fmt.Sprintf("%s/%s", org, repo.GetName()))
	createdRepo.Owner = &github.User{Login: github.Ptr(org)}
	if createdRepo.Archived == nil {
		createdRepo.Archived = github.Ptr(false)
	}

	return &createdRepo, nil
}

func (m *MockGitHubClientWrapper) EditRepository(ctx context.Context, owner, repo string, repository *github.Repository) (*github.Repository, error) {
	m.recordRepoCall(RepoCall{Method: "EditRepository", Owner: owner, Repo: repo})

	if m.EditRepositoryFunc != nil {
		return m.EditRepositoryFunc(ctx, owner, repo, repository)
	}

	// Default implementation - return the updated repository
	updatedRepo := *repository
	updatedRepo.FullName = github.Ptr(fmt.Sprintf("%s/%s", owner, repo))
	updatedRepo.Owner = &github.User{Login: github.Ptr(owner)}
	// Ensure ID is set if not already present
	if updatedRepo.ID == nil {
		updatedRepo.ID = github.Ptr(int64(12345))
	}

	return &updatedRepo, nil
}

func (m *MockGitHubClientWrapper) DeleteRepository(ctx context.Context, owner, repo string) error {
	m.recordRepoCall(RepoCall{Method: "DeleteRepository"})

	if m.DeleteRepositoryFunc != nil {
		return m.DeleteRepositoryFunc(ctx, owner, repo)
	}

	return nil
}

func (m *MockGitHubClientWrapper) GetAllCustomPropertyValues(ctx context.Context, owner string, repo string) ([]*github.CustomPropertyValue, error) {
	m.recordRepoCall(RepoCall{Method: "GetAllCustomPropertyValues", Owner: owner, Repo: repo})

	if m.GetAllCustomPropertyValuesFunc != nil {
		return m.GetAllCustomPropertyValuesFunc(ctx, owner, repo)
	}

	// Default implementation - return empty list
	return make([]*github.CustomPropertyValue, 0), nil
}

func (m *MockGitHubClientWrapper) CreateOrUpdateRepositoryCustomProperties(ctx context.Context, owner string, name string, values []*github.CustomPropertyValue) error {
	m.recordRepoCall(RepoCall{Method: "CreateOrUpdateRepositoryCustomProperties", Owner: owner, Repo: name})

	if m.CreateOrUpdateRepositoryCustomPropertiesFunc != nil {
		return m.CreateOrUpdateRepositoryCustomPropertiesFunc(ctx, owner, name, values)
	}

	// Default implementation - return success
	return nil
}

func (m *MockGitHubClientWrapper) GetAllTopics(ctx context.Context, owner, repo string) ([]string, error) {
	m.recordRepoCall(RepoCall{Method: "GetAllTopics", Owner: owner, Repo: repo})

	if m.GetAllTopicsFunc != nil {
		return m.GetAllTopicsFunc(ctx, owner, repo)
	}

	// Default implementation - return empty list
	return make([]string, 0), nil
}

func (m *MockGitHubClientWrapper) ReplaceAllTopics(ctx context.Context, owner, repo string, topics []string) error {
	m.recordRepoCall(RepoCall{Method: "ReplaceAllTopics", Owner: owner, Repo: repo})

	if m.ReplaceAllTopicsFunc != nil {
		return m.ReplaceAllTopicsFunc(ctx, owner, repo, topics)
	}

	// Default implementation - return success
	return nil
}

func (m *MockGitHubClientWrapper) ListAllAutolinks(ctx context.Context, owner, repo string) ([]*github.Autolink, error) {
	m.recordRepoCall(RepoCall{Method: "ListAllAutolinks", Owner: owner, Repo: repo})

	if m.ListAllAutolinksFunc != nil {
		return m.ListAllAutolinksFunc(ctx, owner, repo)
	}

	// Default implementation - return empty list
	return make([]*github.Autolink, 0), nil
}

func (m *MockGitHubClientWrapper) DeleteAutolink(ctx context.Context, owner, repo string, id int64) error {
	m.recordRepoCall(RepoCall{Method: "DeleteAutolink", Owner: owner, Repo: repo})

	if m.DeleteAutolinkFunc != nil {
		return m.DeleteAutolinkFunc(ctx, owner, repo, id)
	}

	// Default implementation - return success
	return nil
}

func (m *MockGitHubClientWrapper) CreateAutolink(ctx context.Context, owner, repo string, autolink *github.AutolinkOptions) error {
	m.recordRepoCall(RepoCall{Method: "CreateAutolink", Owner: owner, Repo: repo})

	if m.CreateAutolinkFunc != nil {
		return m.CreateAutolinkFunc(ctx, owner, repo, autolink)
	}

	// Default implementation - return success
	return nil
}

func (m *MockGitHubClientWrapper) ListAllDeployKeys(ctx context.Context, owner, repo string) ([]*github.Key, error) {
	m.recordRepoCall(RepoCall{Method: "ListAllDeployKeys", Owner: owner, Repo: repo})

	if m.ListAllDeployKeysFunc != nil {
		return m.ListAllDeployKeysFunc(ctx, owner, repo)
	}

	// Default implementation - return empty list
	return make([]*github.Key, 0), nil
}

func (m *MockGitHubClientWrapper) DeleteDeployKey(ctx context.Context, owner, repo string, id int64) error {
	m.recordRepoCall(RepoCall{Method: "DeleteDeployKey", Owner: owner, Repo: repo})

	if m.DeleteDeployKeyFunc != nil {
		return m.DeleteDeployKeyFunc(ctx, owner, repo, id)
	}

	// Default implementation - return success
	return nil
}

func (m *MockGitHubClientWrapper) CreateDeployKey(ctx context.Context, owner, repo string, key *github.Key) error {
	m.recordRepoCall(RepoCall{Method: "CreateDeployKey", Owner: owner, Repo: repo})

	if m.CreateDeployKeyFunc != nil {
		return m.CreateDeployKeyFunc(ctx, owner, repo, key)
	}

	// Default implementation - return success
	return nil
}

func (m *MockGitHubClientWrapper) GetOrgRepositories(ctx context.Context, org string) ([]*github.Repository, error) {
	m.recordRepoCall(RepoCall{Method: "GetOrgRepositories", Owner: org})

	if m.GetOrgRepositoriesFunc != nil {
		return m.GetOrgRepositoriesFunc(ctx, org)
	}

	// Default implementation - return empty list
	return make([]*github.Repository, 0), nil
}
