package ghclient

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

// testAppSecret builds a Kubernetes secret containing a freshly generated 2048-bit RSA key,
// suitable for driving client creation in the factory without touching the network.
func testAppSecret() *corev1.Secret {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	der := x509.MarshalPKCS1PrivateKey(key)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})
	return &corev1.Secret{
		Data: map[string][]byte{
			"app-id":      []byte("12345"),
			"private-key": pemBytes,
		},
	}
}

var _ = Describe("GraphQL client factory", func() {
	const cacheKey = "my-org"
	app := AppConfig{InstallationID: 67890, CredentialsSecretName: "creds"}

	It("creates, caches and returns a GraphQL client", func() {
		callCount := 0
		provider := func(_ context.Context, _ string) (*corev1.Secret, error) {
			callCount++
			return testAppSecret(), nil
		}
		factory, err := NewGitHubCachingClientFactory(DefaultClientConfig(), provider, "legacy", nil)
		Expect(err).NotTo(HaveOccurred())

		gql, err := factory.GetGraphQLClient(context.Background(), cacheKey, app)
		Expect(err).NotTo(HaveOccurred())
		Expect(gql).NotTo(BeNil())

		// Second call returns the same cached instance without re-fetching the secret.
		gql2, err := factory.GetGraphQLClient(context.Background(), cacheKey, app)
		Expect(err).NotTo(HaveOccurred())
		Expect(gql2).To(BeIdenticalTo(gql))
		Expect(callCount).To(Equal(1), "secret should be fetched only once and reused")
	})

	It("shares the same cached ClientInfo between REST and GraphQL clients", func() {
		provider := func(_ context.Context, _ string) (*corev1.Secret, error) {
			return testAppSecret(), nil
		}
		factory, err := NewGitHubCachingClientFactory(DefaultClientConfig(), provider, "legacy", nil)
		Expect(err).NotTo(HaveOccurred())

		restClient, err := factory.GetClient(context.Background(), cacheKey, app)
		Expect(err).NotTo(HaveOccurred())
		Expect(restClient).NotTo(BeNil())

		// GraphQL client must have been created alongside the REST client.
		gql := factory.getCachedGraphQLClient(cacheKey, "creds")
		Expect(gql).NotTo(BeNil())
	})

	It("propagates secret provider errors", func() {
		provider := func(_ context.Context, _ string) (*corev1.Secret, error) {
			return nil, errors.New("boom")
		}
		factory, err := NewGitHubCachingClientFactory(DefaultClientConfig(), provider, "legacy", nil)
		Expect(err).NotTo(HaveOccurred())

		_, err = factory.GetGraphQLClient(context.Background(), cacheKey, app)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("boom"))
	})

	It("falls back to the legacy secret name when none is configured", func() {
		var requestedSecret string
		provider := func(_ context.Context, name string) (*corev1.Secret, error) {
			requestedSecret = name
			return testAppSecret(), nil
		}
		factory, err := NewGitHubCachingClientFactory(DefaultClientConfig(), provider, "legacy-secret", nil)
		Expect(err).NotTo(HaveOccurred())

		_, err = factory.GetGraphQLClient(context.Background(), cacheKey, AppConfig{InstallationID: 1})
		Expect(err).NotTo(HaveOccurred())
		Expect(requestedSecret).To(Equal("legacy-secret"))
	})
})
