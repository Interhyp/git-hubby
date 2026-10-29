package ghclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"syscall"

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
	return slices.Contains(codes, att.Response.StatusCode) //nolint:govet
}

// transportRetryMaxRetriesFn limits retries of transport-level errors.
var transportRetryMaxRetriesFn = rehttp.RetryMaxRetries(transportRetryMaxRetries)

// retryOnTransportError is a rehttp.RetryFn that retries requests which failed without a
// response because of a transient transport error (HTTP/2 stream errors, GOAWAY,
// connection resets, unexpected EOF). To stay safe it only retries requests that are
// idempotent (GET, HEAD, OPTIONS) without a body, or are the installation token request.
func retryOnTransportError(att rehttp.Attempt) bool {
	if att.Error == nil || att.Request == nil || !transportRetryMaxRetriesFn(att) {
		return false
	}
	if !isTransientTransportError(att.Error) {
		return false
	}
	return isSafeToRetry(att.Request)
}

func isSafeToRetry(req *http.Request) bool {
	// The installation token request is idempotent and carries a small body (rehttp buffers it).
	if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/access_tokens") {
		return true
	}
	if req.Body != nil && req.Body != http.NoBody {
		return false
	}
	switch req.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func isTransientTransportError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) {
		return true
	}
	msg := err.Error()
	for _, s := range []string{"PROTOCOL_ERROR", "GOAWAY", "REFUSED_STREAM", "connection reset by peer", "http2: server sent", "http2: client conn"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}
