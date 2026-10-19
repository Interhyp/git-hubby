package reporec

import (
	"fmt"
)

// ArchivedMessage is used for the conditions of archived repositories (see reconcileRepository).
const ArchivedMessage = "repository is archived (read-only); settings are not reconciled until it is unarchived"

// DefaultBranchNotFoundError is returned when spec.defaultBranch references a branch that does not exist in the
// GitHub repository (e.g. right after creation, when auto_init only created the organization's default branch).
// GitHub would reject the repository update, so no update is sent at all. Retrying with backoff is pointless: the
// error can only be fixed by pushing the branch or changing the spec. The controller therefore does not requeue
// with backoff but waits for the next spec change or the regular success requeue interval.
type DefaultBranchNotFoundError struct {
	RepositoryOwner string
	RepositoryName  string
	Branch          string
}

func (e *DefaultBranchNotFoundError) Error() string {
	return fmt.Sprintf("default branch %q does not exist in repository %s/%s; push the branch or change spec.defaultBranch",
		e.Branch, e.RepositoryOwner, e.RepositoryName)
}

// UserActionRequired marks the missing default branch as not retryable (see reconciler.RequiresUserAction).
func (e *DefaultBranchNotFoundError) UserActionRequired() bool {
	return true
}

// AwaitsGitHubChange marks the error as fixable on GitHub by pushing the branch (see reconciler.AwaitsGitHubChange).
func (e *DefaultBranchNotFoundError) AwaitsGitHubChange() bool {
	return true
}
