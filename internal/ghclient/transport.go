package ghclient

import (
	"crypto/rsa"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/PuerkitoBio/rehttp"
	"github.com/bradleyfalzon/ghinstallation/v2"
)

const (
	transportRetryMaxRetries = 3
	transportRetryBaseDelay  = 500 * time.Millisecond
	transportRetryMaxDelay   = 5 * time.Second
)

// AuthorizeGitHubAccessOptions authenticates requests as a GitHub App installation.
//
// In the default (legacy) mode a new ghinstallation transport is created for every
// request, so every request fetches its own installation token.
//
// In resilient mode (see AuthOptions.Resilient) the ghinstallation transport is
// reused, which caches the installation token until shortly before it expires.
// Transient transport errors are retried and a 401 response discards the cached token.
type AuthorizeGitHubAccessOptions struct {
	http.RoundTripper

	appID             int64
	appInstallationID int64
	privateKey        *rsa.PrivateKey

	resilient bool
	base      http.RoundTripper // transport used for GitHub requests in resilient mode

	mu           sync.Mutex
	installation http.RoundTripper
}

// AuthOptions configures AuthorizeGitHubAccess.
type AuthOptions struct {
	// Resilient enables the resilient authentication mode:
	//
	//   - The installation token is cached until shortly before it expires.
	//   - Transient transport errors (e.g. HTTP/2 PROTOCOL_ERROR/GOAWAY, connection resets) are
	//     retried below the authentication layer. This also covers the access_tokens request.
	//   - If GitHub answers 401, the cached token is discarded and the request is replayed once
	//     with a freshly minted token (if its body can be replayed).
	//
	// When false, every request fetches its own installation token.
	Resilient bool
}

// AuthorizeGitHubAccess authenticates requests as the given GitHub App installation.
func AuthorizeGitHubAccess(rt http.RoundTripper, appID int64, appInstallationID int64, privateKey *rsa.PrivateKey, opts AuthOptions) *AuthorizeGitHubAccessOptions {
	t := &AuthorizeGitHubAccessOptions{
		RoundTripper:      rt,
		appID:             appID,
		appInstallationID: appInstallationID,
		privateKey:        privateKey,
		resilient:         opts.Resilient,
	}
	if opts.Resilient {
		t.base = rehttp.NewTransport(
			rt,
			retryOnTransportError,
			rehttp.ExpJitterDelay(transportRetryBaseDelay, transportRetryMaxDelay),
		)
	}
	return t
}

// RoundTrip implements http.RoundTripper interface.
func (t *AuthorizeGitHubAccessOptions) RoundTrip(req *http.Request) (*http.Response, error) {
	if !t.resilient {
		return t.newInstallationTransport(t.RoundTripper).RoundTrip(req)
	}

	inst := t.currentInstallation()
	resp, err := inst.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		return resp, err
	}

	// GitHub rejected the token: never reuse it, regardless of whether we can replay.
	t.invalidate(inst)

	replay, ok := replayableRequest(req)
	if !ok {
		return resp, nil
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return t.currentInstallation().RoundTrip(replay)
}

func (t *AuthorizeGitHubAccessOptions) newInstallationTransport(base http.RoundTripper) http.RoundTripper {
	apps := ghinstallation.NewAppsTransportFromPrivateKey(base, t.appID, t.privateKey)
	return ghinstallation.NewFromAppsTransport(apps, t.appInstallationID)
}

func (t *AuthorizeGitHubAccessOptions) currentInstallation() http.RoundTripper {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.installation == nil {
		t.installation = t.newInstallationTransport(t.base)
	}
	return t.installation
}

// invalidate drops the cached transport (and thereby its token), unless it has already
// been replaced by a concurrent request that also received a 401.
func (t *AuthorizeGitHubAccessOptions) invalidate(rejected http.RoundTripper) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.installation == rejected {
		t.installation = nil
	}
}

// replayableRequest returns a copy of req with a fresh body, if the body can be replayed.
// Replaying is safe for every method because a 401 means GitHub did not process the request.
func replayableRequest(req *http.Request) (*http.Request, bool) {
	replay := req.Clone(req.Context())
	if req.Body == nil || req.Body == http.NoBody {
		return replay, true
	}
	if req.GetBody == nil {
		return nil, false
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, false
	}
	replay.Body = body
	return replay, true
}
