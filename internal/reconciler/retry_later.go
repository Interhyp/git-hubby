package reconciler

import (
	"time"
)

// RetryLaterError marks a transient state on GitHub that blocks the reconciliation for a while and is expected to
// resolve on its own (e.g. a running code security configuration enablement). The controllers requeue the resource
// after RequeueAfter without logging an error and without exponential backoff.
type RetryLaterError struct {
	// Reason describes why the reconciliation has to be retried later.
	Reason string
	// RequeueAfter is the delay before the resource is reconciled again.
	RequeueAfter time.Duration
	// Err is the optional underlying error.
	Err error
}

func (e *RetryLaterError) Error() string {
	if e.Err != nil {
		return e.Reason + ": " + e.Err.Error()
	}
	return e.Reason
}

func (e *RetryLaterError) Unwrap() error {
	return e.Err
}

// RetryLater reports whether err (including all wrapped and joined errors) consists only of RetryLaterErrors and
// returns the longest RequeueAfter of them. nil and any other error return false, so that other errors joined with
// a RetryLaterError are still handled as failures.
func RetryLater(err error) (time.Duration, bool) {
	if err == nil {
		return 0, false
	}
	if retry, ok := err.(*RetryLaterError); ok { //nolint:errorlint // the error tree is walked explicitly
		return retry.RequeueAfter, true
	}
	switch e := err.(type) { //nolint:errorlint // the error tree is walked explicitly
	case interface{ Unwrap() []error }:
		errs := e.Unwrap()
		var longest time.Duration
		for _, inner := range errs {
			after, ok := RetryLater(inner)
			if !ok {
				return 0, false
			}
			longest = max(longest, after)
		}
		return longest, len(errs) > 0
	case interface{ Unwrap() error }:
		return RetryLater(e.Unwrap())
	default:
		return 0, false
	}
}
