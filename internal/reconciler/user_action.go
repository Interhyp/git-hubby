package reconciler

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/google/go-github/v93/github"
)

// UserActionRequiredError marks errors that cannot be fixed by retrying: the spec or the state on GitHub has to be
// changed by a user (e.g. conflicting definitions, missing references, a missing default branch).
//
// UserActionRequired reports whether the concrete error instance requires user action. Returning a bool (instead of
// a pure marker method) allows error types to classify single instances as transient, e.g. a ruleset whose parent
// preset could not be read because of a temporary API error.
//
// Packages do not need to import this package to mark their errors; implementing the method is sufficient.
type UserActionRequiredError interface {
	error
	UserActionRequired() bool
}

// RequiresUserAction reports whether err (including all wrapped and joined errors) consists only of errors that
// require user action. These are errors implementing UserActionRequiredError (returning true) and GitHub
// "422 Unprocessable Entity" responses. nil and any other error return false.
func RequiresUserAction(err error) bool {
	if err == nil {
		return false
	}
	if marked, ok := err.(UserActionRequiredError); ok { //nolint:errorlint // the error tree is walked explicitly
		return marked.UserActionRequired()
	}
	if ghErr, ok := err.(*github.ErrorResponse); ok { //nolint:errorlint // the error tree is walked explicitly
		return isUnprocessableEntity(ghErr)
	}
	switch e := err.(type) { //nolint:errorlint // the error tree is walked explicitly
	case interface{ Unwrap() []error }:
		errs := e.Unwrap()
		for _, inner := range errs {
			if !RequiresUserAction(inner) {
				return false
			}
		}
		return len(errs) > 0
	case interface{ Unwrap() error }:
		return RequiresUserAction(e.Unwrap())
	default:
		return false
	}
}

func isUnprocessableEntity(ghErr *github.ErrorResponse) bool {
	return ghErr != nil && ghErr.Response != nil && ghErr.Response.StatusCode == http.StatusUnprocessableEntity
}

// GitHubStateError can additionally be implemented by UserActionRequiredErrors which are fixed by a change on GitHub
// (e.g. creating a team, installing a GitHub App, pushing a branch) instead of a change of Kubernetes resources.
// No watch event signals such changes, so the controllers requeue these errors after a shorter interval.
type GitHubStateError interface {
	error
	AwaitsGitHubChange() bool
}

// AwaitsGitHubChange reports whether err (including all wrapped and joined errors) contains at least one error that
// requires user action on GitHub: errors implementing GitHubStateError (returning true) and GitHub
// "422 Unprocessable Entity" responses. It is meant to be used together with RequiresUserAction to choose the
// requeue interval.
func AwaitsGitHubChange(err error) bool {
	if err == nil {
		return false
	}
	if marked, ok := err.(GitHubStateError); ok && marked.AwaitsGitHubChange() { //nolint:errorlint // the error tree is walked explicitly
		return true
	}
	if ghErr, ok := err.(*github.ErrorResponse); ok { //nolint:errorlint // the error tree is walked explicitly
		return isUnprocessableEntity(ghErr)
	}
	switch e := err.(type) { //nolint:errorlint // the error tree is walked explicitly
	case interface{ Unwrap() []error }:
		return slices.ContainsFunc(e.Unwrap(), AwaitsGitHubChange)
	case interface{ Unwrap() error }:
		return AwaitsGitHubChange(e.Unwrap())
	default:
		return false
	}
}

// ErrorMessage returns a human-readable message for err. The raw message of GitHub "422 Unprocessable Entity"
// responses (method, URL and a Go struct dump of the validation errors) is replaced by GitHub's response message,
// e.g. "Validation Failed: default_branch: Cannot update default branch for an empty repository".
func ErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	for _, ghErr := range unprocessableEntityErrors(err) {
		msg = strings.ReplaceAll(msg, ghErr.Error(), formatGitHubErrorResponse(ghErr))
	}
	return msg
}

// unprocessableEntityErrors returns all GitHub 422 responses contained in the error tree of err.
func unprocessableEntityErrors(err error) []*github.ErrorResponse {
	if err == nil {
		return nil
	}
	if ghErr, ok := err.(*github.ErrorResponse); ok { //nolint:errorlint // the error tree is walked explicitly
		if isUnprocessableEntity(ghErr) {
			return []*github.ErrorResponse{ghErr}
		}
		return nil
	}
	switch e := err.(type) { //nolint:errorlint // the error tree is walked explicitly
	case interface{ Unwrap() []error }:
		var result []*github.ErrorResponse
		for _, inner := range e.Unwrap() {
			result = append(result, unprocessableEntityErrors(inner)...)
		}
		return result
	case interface{ Unwrap() error }:
		return unprocessableEntityErrors(e.Unwrap())
	default:
		return nil
	}
}

// formatGitHubErrorResponse formats GitHub's response message and its validation errors.
func formatGitHubErrorResponse(ghErr *github.ErrorResponse) string {
	details := make([]string, 0, len(ghErr.Errors))
	for _, e := range ghErr.Errors {
		detail := e.Message
		if detail == "" {
			detail = e.Code
		}
		if e.Field != "" {
			detail = fmt.Sprintf("%s: %s", e.Field, detail)
		}
		if detail != "" {
			details = append(details, detail)
		}
	}
	msg := ghErr.Message
	if msg == "" {
		msg = http.StatusText(http.StatusUnprocessableEntity)
	}
	if len(details) > 0 {
		msg = fmt.Sprintf("%s: %s", msg, strings.Join(details, "; "))
	}
	return msg
}

// readableError wraps an error so that its message is the one returned by ErrorMessage while keeping the original
// error tree accessible via errors.Is/As and RequiresUserAction.
type readableError struct {
	err error
}

func (e *readableError) Error() string {
	return ErrorMessage(e.err)
}

func (e *readableError) Unwrap() error {
	return e.err
}

// withReadableMessage wraps err in a readableError if its message differs from the readable message.
func withReadableMessage(err error) error {
	if err == nil || len(unprocessableEntityErrors(err)) == 0 {
		return err
	}
	return &readableError{err: err}
}
