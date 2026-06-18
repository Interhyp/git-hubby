package ghclient

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/PuerkitoBio/rehttp"
	"github.com/gofri/go-github-pagination/githubpagination"
	"github.com/gofri/go-github-ratelimit/v2/github_ratelimit"
	"github.com/gofri/go-github-ratelimit/v2/github_ratelimit/github_primary_ratelimit"
	"github.com/google/go-github/v86/github"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	v1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// ClientConfig holds configuration for GitHub client creation
type ClientConfig struct {
	// Client timeout for HTTP requests
	Timeout time.Duration
	// Whether to enable retry middleware
	EnableRetry bool
	// Whether to enable metrics collection
	EnableMetrics bool
	// Whether to enable request logging
	EnableLogging bool
	// Maximum number of retries for failed requests
	MaxRetries int
	// Base delay for exponential backoff
	RetryBaseDelay time.Duration
	// Maximum delay between retries
	RetryMaxDelay time.Duration
}

type RateLimitedError struct {
	ResetTime time.Time
}

func (e RateLimitedError) Error() string {
	return fmt.Sprintf("GitHub rate limit exceeded, reset time: %v", e.ResetTime)
}

func (e RateLimitedError) Is(err error) bool {
	var rateLimitedError *RateLimitedError
	return errors.As(err, &rateLimitedError)
}

// DefaultClientConfig returns a configuration with sensible defaults
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		Timeout:        2 * time.Minute,
		EnableRetry:    true,
		EnableMetrics:  true,
		EnableLogging:  false, // Disabled by default to avoid log spam
		MaxRetries:     3,
		RetryBaseDelay: 1 * time.Second,
		RetryMaxDelay:  10 * time.Second,
	}
}

// ClientInfo holds metadata about a cached client
type ClientInfo struct {
	Client         *GitHubClientWrapper
	InstallationID int64
	CacheKey       string
	SecretName     string
}

// CachingGitHubClientFactory creates and caches GitHub clients with proper lifecycle and thread safety.
// Credentials are cached per secret name and clients are cached per organization (cacheKey).
// Rate limit state is shared per GitHub App ID so all installations of the same App share a quota bucket.
type CachingGitHubClientFactory struct {
	mu              sync.RWMutex
	clients         map[string]*ClientInfo
	config          *ClientConfig
	secretProvider  SecretProviderFunc
	credentials     map[string]*AppCredentials
	rateLimitStates map[int64]*github_primary_ratelimit.RateLimitState
}

// AppCredentials holds parsed GitHub App credentials
type AppCredentials struct {
	AppID      int64
	PrivateKey *rsa.PrivateKey
}

// SecretProviderFunc fetches a Kubernetes Secret by name. The factory calls this function
// lazily on first client creation for a given credential secret.
type SecretProviderFunc = func(ctx context.Context, secretName string) (*v1.Secret, error)

// NewGitHubCachingClientFactory creates a new client cache with the given configuration. The necessary GitHub App
// credentials are fetched lazily via the given SecretProviderFunc upon first client creation for each secret.
func NewGitHubCachingClientFactory(config *ClientConfig, providerFunc SecretProviderFunc) (*CachingGitHubClientFactory, error) {
	if config == nil {
		config = DefaultClientConfig()
	}

	manager := &CachingGitHubClientFactory{
		clients:         make(map[string]*ClientInfo),
		credentials:     make(map[string]*AppCredentials),
		rateLimitStates: make(map[int64]*github_primary_ratelimit.RateLimitState),
		config:          config,
		secretProvider:  providerFunc,
	}
	return manager, nil
}

// GetClient retrieves or creates a GitHub client for the given organization (cacheKey).
// If a cached client exists for the cacheKey with matching credentials, it is returned directly.
// If the credentials secret changed, the old client is evicted and a new one is created.
func (m *CachingGitHubClientFactory) GetClient(ctx context.Context, cacheKey string, app v1alpha1.GitHubAppConfig) (GitHubClient, error) {
	log := logf.FromContext(ctx,
		"function", "GetClient",
	)

	if c := m.getCachedClient(cacheKey, app.CredentialsSecretName); c != nil {
		return c, nil
	}

	// Create new client with write lock
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if info, exists := m.clients[cacheKey]; exists {
		if info.SecretName == app.CredentialsSecretName {
			return info.Client, nil
		}
		// Credentials secret changed – evict the stale client
		delete(m.clients, cacheKey)
		log.Info("Evicted stale GitHub client due to credential change", "cacheKey", cacheKey)
	}

	log.Info("Creating new GitHub client")

	ghClient, err := m.createClient(ctx, app)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client for key %s: %w", cacheKey, err)
	}
	wrappedClient := &GitHubClientWrapper{
		client: ghClient,
	}

	m.clients[cacheKey] = &ClientInfo{
		Client:         wrappedClient,
		InstallationID: app.InstallationId,
		CacheKey:       cacheKey,
		SecretName:     app.CredentialsSecretName,
	}

	log.Info("Successfully created and cached GitHub client", "installationID", app.InstallationId)
	return wrappedClient, nil
}

// GetGitHubClientAndCheckRateLimit retrieves a GitHub client and verifies the remaining rate limit.
func (m *CachingGitHubClientFactory) GetGitHubClientAndCheckRateLimit(ctx context.Context, cacheKey string, app v1alpha1.GitHubAppConfig, rateLimitMinimum int) (GitHubClient, error) {
	ghClient, err := m.GetClient(ctx, cacheKey, app)
	if err != nil {
		return nil, err
	}
	rl, err := ghClient.GetRateLimit(ctx)
	if err != nil {
		return nil, err
	}
	if rl.Core != nil && rl.Core.Remaining < rateLimitMinimum {
		logf.FromContext(ctx).V(1).Info("Encountered Rate limit", "remaining", rl.Core.Remaining, "reset", rl.Core.Reset.Time)
		return nil, &RateLimitedError{
			ResetTime: rl.Core.Reset.Time,
		}
	}
	return ghClient, nil
}

// getCachedClient returns an existing client without creating a new one, only if the cached
// client was created with the same credential secret.
func (m *CachingGitHubClientFactory) getCachedClient(cacheKey string, secretName string) *GitHubClientWrapper {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if info, exists := m.clients[cacheKey]; exists && info.SecretName == secretName {
		return info.Client
	}

	return nil
}

// createClient creates a new GitHub client with proper middleware setup
func (m *CachingGitHubClientFactory) createClient(ctx context.Context, app v1alpha1.GitHubAppConfig) (*github.Client, error) {
	log := logf.FromContext(ctx)
	log.Info("Creating GitHub client with middleware stack")

	creds, ok := m.credentials[app.CredentialsSecretName]
	if !ok {
		// Fetch and parse the secret on first use
		secret, err := m.secretProvider(ctx, app.CredentialsSecretName)
		if err != nil {
			log.Error(err, "failed to get GitHub app credentials secret", "secretName", app.CredentialsSecretName)
			return nil, err
		}
		if secret == nil {
			return nil, errors.New("GitHub app credentials secret cannot be nil")
		}
		parsedCreds, err := parseCredentials(*secret)
		if err != nil {
			log.Error(err, "failed to prepare GitHub app credentials")
			return nil, err
		}
		m.credentials[app.CredentialsSecretName] = parsedCreds
		creds = parsedCreds
	}

	ghClient := m.buildClientWithMiddleware(app.InstallationId, creds)

	return ghClient, nil
}

// buildClientWithMiddleware creates a GitHub client with the full middleware stack
func (m *CachingGitHubClientFactory) buildClientWithMiddleware(appInstallationID int64, creds *AppCredentials) *github.Client {
	clientName := fmt.Sprintf("github-%d", appInstallationID)

	client := github.NewClient(&http.Client{
		Transport: m.buildMiddlewareStack(clientName, creds, appInstallationID),
		Timeout:   m.config.Timeout,
	})
	client.DisableRateLimitCheck = true
	return client
}

// buildMiddlewareStack constructs the HTTP transport middleware stack.
// Rate limit state is shared per GitHub App ID so installations of the same App share a quota bucket.
func (m *CachingGitHubClientFactory) buildMiddlewareStack(clientName string, creds *AppCredentials, appInstallationID int64) http.RoundTripper {
	// Start with the base transport
	rt := http.DefaultTransport

	// Rate limiting (bottom layer) – shared state per App ID so all installations are counted together
	if state, exists := m.rateLimitStates[creds.AppID]; !exists {
		primary := github_ratelimit.NewPrimaryLimiter(rt)
		m.rateLimitStates[creds.AppID] = primary.GetState()
		rt = github_ratelimit.NewSecondaryLimiter(primary)
	} else {
		rt = github_ratelimit.New(rt, github_primary_ratelimit.WithSharedState(state))
	}

	// Authentication
	rt = AuthorizeGitHubAccess(rt, creds.AppID, appInstallationID, creds.PrivateKey)

	// Request logging (if enabled) - TODO: Implement when logging package is available
	// retry
	retryFn := rehttp.RetryAll(
		rehttp.RetryAny(
			rehttp.RetryStatusInterval(500, 600), // 5xx server errors
			retryByContextCodes,                  // per-request retryable codes via context
		),
		rehttp.RetryMaxRetries(5),
	)
	delayFn := rehttp.ExpJitterDelay(5*time.Second, 30*time.Second)
	rt = rehttp.NewTransport(rt, retryFn, delayFn)

	// Pagination handling
	rt = githubpagination.New(rt, githubpagination.WithPerPage(30))
	// OpenTelemetry instrumentation (top layer)
	rt = otelhttp.NewTransport(rt, otelhttp.WithServerName(clientName))

	return rt
}

// parseCredentials sets the GitHub App credentials for the manager after parsing them from a Kubernetes secret containing them.
func parseCredentials(secret v1.Secret) (*AppCredentials, error) {
	appIDStr := string(secret.Data["app-id"])
	privateKeyData := string(secret.Data["private-key"])

	if appIDStr == "" || privateKeyData == "" {
		return nil, errors.New("GitHub App secret is missing required fields (app-id, app-installation-id, private-key)")
	}

	appID, err := parseAppID(appIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid app-id in secret: %w", err)
	}

	privateKey, err := parseGithubPrivateKey(privateKeyData)
	if err != nil {
		return nil, fmt.Errorf("invalid private-key in secret: %w", err)
	}
	return &AppCredentials{
		AppID:      appID,
		PrivateKey: privateKey,
	}, nil
}

// parseGithubPrivateKey safely parses an RSA private key from PEM format
func parseGithubPrivateKey(value string) (*rsa.PrivateKey, error) {
	if value == "" {
		return nil, errors.New("private key cannot be empty")
	}

	block, _ := pem.Decode([]byte(value))
	if block == nil {
		return nil, errors.New("failed to decode PEM block - invalid format")
	}

	if block.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("expected RSA PRIVATE KEY, got %s", block.Type)
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSA private key: %w", err)
	}

	// Validate key size for security
	if privateKey.Size() < 256 { // 2048 bits minimum
		return nil, errors.New("RSA private key too small, minimum 2048 bits required")
	}

	return privateKey, nil
}

// parseAppID safely parses an app ID from string
func parseAppID(value string) (int64, error) {
	if value == "" {
		return 0, errors.New("app ID cannot be empty")
	}

	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid app ID format - must be a number: %sw", err)
	}

	if id <= 0 {
		return 0, errors.New("app ID must be positive")
	}

	return id, nil
}
