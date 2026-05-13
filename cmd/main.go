/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/Interhyp/git-hubby/internal/logging"
	"github.com/Interhyp/git-hubby/internal/reconciler/spreading"
	"github.com/joho/godotenv"
	"go.elastic.co/ecszap"
	uberzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"github.com/Interhyp/git-hubby/internal/reconciler/reconcilerfactory"

	"github.com/Interhyp/git-hubby/internal/ghclient"
	"github.com/Interhyp/git-hubby/internal/ratelimit"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/controller"
	webhookv1alpha1 "github.com/Interhyp/git-hubby/internal/webhook/v1alpha1"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(githubv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// getWatchNamespace returns the namespace(s) the manager should watch for changes.
// It reads the value from the WATCH_NAMESPACE environment variable.
func getWatchNamespace() (string, error) {
	watchNamespaceEnvVar := "WATCH_NAMESPACE"
	ns, found := os.LookupEnv(watchNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s must be set", watchNamespaceEnvVar)
	}
	ns = strings.TrimSpace(ns)
	if ns == "" {
		return "", fmt.Errorf("%s must not be empty", watchNamespaceEnvVar)
	}
	return ns, nil
}

// setupCacheNamespaces configures the cache to watch specific namespace(s).
// It returns an error if no valid namespaces remain after parsing.
func setupCacheNamespaces(namespaces string) (cache.Options, error) {
	defaultNamespaces := make(map[string]cache.Config)
	for ns := range strings.SplitSeq(namespaces, ",") {
		cleaned := strings.TrimSpace(ns)
		if cleaned == "" {
			continue
		}
		defaultNamespaces[cleaned] = cache.Config{}
	}
	if len(defaultNamespaces) == 0 {
		return cache.Options{}, fmt.Errorf("WATCH_NAMESPACE resolved to zero valid namespaces (input: %q)", namespaces)
	}
	return cache.Options{
		DefaultNamespaces: defaultNamespaces,
	}, nil
}

// parseLogLevel parses a log level string (e.g. "debug", "info", "warn", "error")
// into a zapcore.Level. It returns the level and true if parsing succeeded,
// or the zero value and false if the input is empty or invalid.
func parseLogLevel(value string) (zapcore.Level, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return zapcore.InfoLevel, false
	}
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(value)); err != nil {
		return zapcore.InfoLevel, false
	}
	return level, true
}

// logFormat represents the supported log output formats.
type logFormat string

const (
	// logFormatJSON is the default structured JSON format.
	logFormatJSON logFormat = "json"
	// logFormatECS is the Elastic Common Schema JSON format.
	logFormatECS logFormat = "ecs"
	// logFormatConsole is a human-readable console format for local development.
	logFormatConsole logFormat = "console"
)

// parseLogFormat normalises the LOG_FORMAT env value into a known logFormat.
// The comparison is case-insensitive and surrounding whitespace is trimmed.
// Unknown or empty values fall back to logFormatJSON.
func parseLogFormat(value string) logFormat {
	switch logFormat(strings.ToLower(strings.TrimSpace(value))) {
	case logFormatECS:
		return logFormatECS
	case logFormatConsole:
		return logFormatConsole
	default:
		return logFormatJSON
	}
}

// buildLoggerOpts returns the zap.Opts slice for configuring the controller-runtime logger.
//   - logFormatECS: ECS-compatible JSON encoding with ecszap core wrapping.
//   - logFormatConsole: human-readable console encoder (ideal for local development).
//   - logFormatJSON (default): standard kubebuilder JSON encoder.
func buildLoggerOpts(flagOpts *zap.Options, format logFormat) []zap.Opts {
	logOpts := []zap.Opts{
		zap.UseFlagOptions(flagOpts),
		zap.RawZapOpts(
			uberzap.WrapCore(func(core zapcore.Core) zapcore.Core {
				return logging.NewLogMapper(core, nil)
			}),
		),
	}

	switch format {
	case logFormatECS:
		ecsConfigOpt := func(config *zapcore.EncoderConfig) {
			if config != nil {
				*config = ecszap.ECSCompatibleEncoderConfig(*config)
			}
		}
		logOpts = append(logOpts,
			zap.JSONEncoder(ecsConfigOpt),
			zap.RawZapOpts(ecszap.WrapCoreOption()),
		)
	case logFormatConsole:
		logOpts = append(logOpts, zap.ConsoleEncoder())
	default:
		logOpts = append(logOpts, zap.JSONEncoder())
	}

	return logOpts
}

// nolint:gocyclo
func main() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var appCredentialsSecretName string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var envFile string
	var tlsOpts []func(*tls.Config)
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.StringVar(&appCredentialsSecretName, "app-credentials-secret-name", "git-hubby-app-credentials",
		"The name of the secret containing the GitHub app credentials.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&envFile, "env-file", ".env",
		"Comma-separated list of paths to env-files to load environment based configuration from. Defaults to .env.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	if err := godotenv.Load(strings.Split(envFile, ",")...); err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "Error loading .env file: %v\n", err)
		os.Exit(1)
	}
	// Support LOG_LEVEL env var (overrides --zap-log-level flag)
	if level, ok := parseLogLevel(os.Getenv("LOG_LEVEL")); ok {
		opts.Level = level
	}

	ctrl.SetLogger(zap.New(buildLoggerOpts(&opts, parseLogFormat(os.Getenv("LOG_FORMAT")))...))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts
	webhookServerOptions := webhook.Options{
		TLSOpts: webhookTLSOpts,
	}

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		webhookServerOptions.CertDir = webhookCertPath
		webhookServerOptions.CertName = webhookCertName
		webhookServerOptions.KeyName = webhookCertKey
	}

	webhookServer := webhook.NewServer(webhookServerOptions)

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		metricsServerOptions.CertDir = metricsCertPath
		metricsServerOptions.CertName = metricsCertName
		metricsServerOptions.KeyName = metricsCertKey
	}

	// Get the namespace(s) for namespace-scoped mode from WATCH_NAMESPACE environment variable.
	watchNamespace, err := getWatchNamespace()
	if err != nil {
		setupLog.Error(err, "Unable to get WATCH_NAMESPACE")
		os.Exit(1)
	}

	// APP_CREDENTIALS_SECRET_NAMESPACE specifies the namespace containing the GitHub App credentials secret.
	// In the Helm chart this defaults to the release namespace (where the controller is deployed).
	appCredentialsSecretNamespace := os.Getenv("APP_CREDENTIALS_SECRET_NAMESPACE")
	if strings.TrimSpace(appCredentialsSecretNamespace) == "" {
		setupLog.Error(fmt.Errorf("APP_CREDENTIALS_SECRET_NAMESPACE must be set"), "Unable to determine secret namespace")
		os.Exit(1)
	}
	setupLog.Info("App credentials secret namespace configured", "namespace", appCredentialsSecretNamespace)

	mgrOptions := ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "6cee1c41.interhyp.de",

		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		LeaderElectionReleaseOnCancel: true,
	}

	// Configure cache to watch namespace(s) specified in WATCH_NAMESPACE
	cacheOpts, err := setupCacheNamespaces(watchNamespace)
	if err != nil {
		setupLog.Error(err, "Invalid WATCH_NAMESPACE configuration")
		os.Exit(1)
	}
	mgrOptions.Cache = cacheOpts
	setupLog.Info("Watching namespace(s)", "namespaces", watchNamespace)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOptions)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Create a direct (non-cached) client for reading secrets outside the watched namespaces.
	// The manager's cached client only sees resources in WATCH_NAMESPACE, but the credentials
	// secret may live in a different namespace (APP_CREDENTIALS_SECRET_NAMESPACE).
	directClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "unable to create direct API client")
		os.Exit(1)
	}

	fetchGitHubAppSecret := func(ctx context.Context) (*v1.Secret, error) {
		log := ctrl.LoggerFrom(ctx)
		var appCredentialsSecret v1.Secret
		secretName := client.ObjectKey{
			Name:      appCredentialsSecretName,
			Namespace: appCredentialsSecretNamespace,
		}
		if fetchErr := directClient.Get(ctx, secretName, &appCredentialsSecret); fetchErr != nil {
			log.Error(fetchErr, "Failed to fetch GitHub App credentials secret")
			return nil, fetchErr
		}
		return &appCredentialsSecret, nil
	}
	clientManager, err := ghclient.NewGitHubCachingClientFactory(ghclient.DefaultClientConfig(), fetchGitHubAppSecret)
	if err != nil {
		setupLog.Error(err, "failed to create GitHub client factory")
		os.Exit(1)
	}
	spreadingManager := spreading.NewDefaultManager()
	setupLog.Info("Startup spreading configured",
		"spreadPeriod", spreadingManager.Config.SpreadPeriod,
		"spreadInterval", spreadingManager.Config.SpreadInterval,
		"startTime", spreadingManager.Config.StartTime,
		"enabled", spreadingManager.Config.Enabled)

	reconcilerFactory := &reconcilerfactory.Factory{
		ClientManager:    clientManager,
		SpreadingManager: spreadingManager,
		K8sClient:        mgr.GetClient(),
	}

	globalLimiter := ratelimit.NewGitHubRateLimiter(ratelimit.GitHubRateLimiterConfig{
		RequestsPerHour: 15000, // GitHub's rate limit
		BurstSize:       500,   // Allow some burst
		EnableBlocking:  true,  // Enable blocking behavior
	})

	if err := (&controller.OrganizationCtl{
		Scheme:                 mgr.GetScheme(),
		ReconcilerFactory:      reconcilerFactory,
		GithubRateLimiter:      globalLimiter,
		SuccessRequeueInterval: spreadingManager.GetSpreadInterval(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Organization")
		os.Exit(1)
	}
	if err := (&controller.RepositoryCtl{
		Scheme:                 mgr.GetScheme(),
		ReconcilerFactory:      reconcilerFactory,
		GithubRateLimiter:      globalLimiter,
		SuccessRequeueInterval: spreadingManager.GetSpreadInterval(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Repository")
		os.Exit(1)
	}
	if err := (&controller.TeamCtl{
		Scheme:                 mgr.GetScheme(),
		ReconcilerFactory:      reconcilerFactory,
		GithubRateLimiter:      globalLimiter,
		SuccessRequeueInterval: spreadingManager.GetSpreadInterval(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Team")
		os.Exit(1)
	}

	webhooksEnabled := os.Getenv("ENABLE_WEBHOOKS") != "false"
	if webhooksEnabled {
		setupLog.Info("Webhooks enabled")
		if err := webhookv1alpha1.SetupOrganizationWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Organization")
			os.Exit(1)
		}
		if err := webhookv1alpha1.SetupRepositoryWebhookWithManager(mgr, clientManager); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Repository")
			os.Exit(1)
		}
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
	if webhooksEnabled {
		if err := mgr.AddReadyzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
			setupLog.Error(err, "unable to set up webhook ready check")
			os.Exit(1)
		}
	}

	setupLog.Info("Starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "unable to run manager")
		os.Exit(1)
	}

}
