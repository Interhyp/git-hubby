package ghclient

import (
	"crypto/rsa"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
)

// newInstallationTransport builds the shared GitHub App installation auth transport.
//
// It is the single source of truth for turning an App ID + installation ID + RSA private
// key into an http.RoundTripper that signs requests with an installation access token.
// Both the REST client and the GraphQL client build their auth on top of this so that the
// App/installation authentication logic lives in exactly one place.
//
// The returned *ghinstallation.Transport caches installation tokens internally and refreshes
// them before expiry, so it is intended to be constructed once per client and reused across
// requests (unlike wrapping it fresh on every RoundTrip).
func newInstallationTransport(rt http.RoundTripper, appID int64, appInstallationID int64, privateKey *rsa.PrivateKey) *ghinstallation.Transport {
	appsTransport := ghinstallation.NewAppsTransportFromPrivateKey(rt, appID, privateKey)
	return ghinstallation.NewFromAppsTransport(appsTransport, appInstallationID)
}

// AuthorizeGitHubAccessOptions is an http.RoundTripper that authenticates requests as a
// GitHub App installation. It delegates to the shared newInstallationTransport builder.
type AuthorizeGitHubAccessOptions struct {
	http.RoundTripper

	appID             int64
	appInstallationID int64
	privateKey        *rsa.PrivateKey
}

func AuthorizeGitHubAccess(rt http.RoundTripper, appID int64, appInstallationID int64, privateKey *rsa.PrivateKey) *AuthorizeGitHubAccessOptions {
	return &AuthorizeGitHubAccessOptions{
		RoundTripper:      rt,
		appID:             appID,
		appInstallationID: appInstallationID,
		privateKey:        privateKey,
	}
}

// RoundTrip implements http.RoundTripper interface.
func (t *AuthorizeGitHubAccessOptions) RoundTrip(req *http.Request) (*http.Response, error) {
	rt := newInstallationTransport(t.RoundTripper, t.appID, t.appInstallationID, t.privateKey)
	return rt.RoundTrip(req)
}
