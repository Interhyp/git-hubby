package ghclient

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Interhyp/git-hubby/internal/ratelimit"
)

// rateLimitTrackerTransport is an HTTP RoundTripper that reads GitHub rate limit response headers
// and updates the OrgRateLimitRegistry for the owning organization.
// It is constructed with the org login baked in at client creation time, so no per-request
// context propagation is needed.
type rateLimitTrackerTransport struct {
	inner    http.RoundTripper
	orgLogin string
	appID    int64
	registry *ratelimit.OrgRateLimitRegistry
}

// newRateLimitTrackerTransport wraps the given transport with rate limit header tracking.
func newRateLimitTrackerTransport(
	inner http.RoundTripper,
	orgLogin string,
	appID int64,
	registry *ratelimit.OrgRateLimitRegistry,
) http.RoundTripper {
	return &rateLimitTrackerTransport{
		inner:    inner,
		orgLogin: orgLogin,
		appID:    appID,
		registry: registry,
	}
}

// RoundTrip executes the request and extracts rate limit headers from every response.
// It uses the X-RateLimit-Resource header to identify the category (preferred) and falls
// back to URL-path classification when the header is absent.
func (t *rateLimitTrackerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.inner.RoundTrip(req)
	if err != nil || resp == nil {
		return resp, err
	}

	remaining := parseRateLimitIntHeader(resp.Header.Get("X-RateLimit-Remaining"))
	limit := parseRateLimitIntHeader(resp.Header.Get("X-RateLimit-Limit"))
	resetEpoch := parseRateLimitInt64Header(resp.Header.Get("X-RateLimit-Reset"))

	// Only record if all three headers are present and sensible
	if remaining >= 0 && limit > 0 && resetEpoch > 0 {
		resetTime := time.Unix(resetEpoch, 0)
		resourceHeader := resp.Header.Get("X-RateLimit-Resource")
		category := ratelimit.ClassifyRequest(req, resourceHeader)
		t.registry.Update(t.orgLogin, category, remaining, limit, resetTime, t.appID)
	}

	return resp, nil
}

func parseRateLimitIntHeader(s string) int {
	if s == "" {
		return -1
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return v
}

func parseRateLimitInt64Header(s string) int64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}
