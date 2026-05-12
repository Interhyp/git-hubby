package ghclient

import (
	"crypto/rsa"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
)

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
	rt1 := ghinstallation.NewAppsTransportFromPrivateKey(t.RoundTripper, t.appID, t.privateKey)
	rt2 := ghinstallation.NewFromAppsTransport(rt1, t.appInstallationID)
	return rt2.RoundTrip(req)
}
