package ghclient

import (
	"context"

	"github.com/PuerkitoBio/rehttp"
	"golang.org/x/exp/slices"
)

type retryableStatusCodesKey struct{}

// WithRetryableStatusCodes returns a new context that instructs the HTTP transport
// retry middleware to retry requests that receive any of the given HTTP status codes.
//
// Usage at call site:
//
//	ctx = ghclient.WithRetryableStatusCodes(ctx, http.StatusConflict)
//	client.UpdateCodeSecurityConfigurationForOrg(ctx, ...)
func WithRetryableStatusCodes(ctx context.Context, codes ...int) context.Context {
	return context.WithValue(ctx, retryableStatusCodesKey{}, codes)
}

// retryableStatusCodesFromContext extracts the retryable status codes from the context, if any.
func retryableStatusCodesFromContext(ctx context.Context) []int {
	codes, _ := ctx.Value(retryableStatusCodesKey{}).([]int)
	return codes
}

// retryByContextCodes is a rehttp.RetryFn that retries a request only if its context
// has been annotated with WithRetryableStatusCodes and the response status code matches
// one of those codes. This makes retry intent explicit and visible at the call site.
func retryByContextCodes(att rehttp.Attempt) bool {
	if att.Response == nil || att.Request == nil {
		return false
	}
	codes := retryableStatusCodesFromContext(att.Request.Context())
	return slices.Contains(codes, att.Response.StatusCode)
}
