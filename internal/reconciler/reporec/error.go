package reporec

import (
	"errors"
	"fmt"
)

type RepoArchivedError struct {
	RepositoryName  string
	RepositoryOwner string
}

func (e RepoArchivedError) Error() string {
	return fmt.Sprintf("Repository %s/%s is archived and is therefore read-only. No further reconciliation possible.", e.RepositoryOwner, e.RepositoryName)
}

func (e RepoArchivedError) Is(err error) bool {
	var repoArchivedError *RepoArchivedError
	return errors.As(err, &repoArchivedError)
}
