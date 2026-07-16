package config

import (
	"github.com/caarlos0/env/v11"
)

// Features holds all operator feature flags. Fields are populated from environment
// variables at startup via config.Parse and are immutable after construction.
type Features struct {
	// EnableRequiredReviewersRules enables reconciliation of requiredReviewers in
	// pull-request ruleset rules. Disabled by default; the GitHub API is in beta.
	EnableRequiredReviewersRules bool `env:"ENABLE_REQUIRED_REVIEWERS_RULES" envDefault:"false"`

	// EnableWebhooks enables registration of the admission webhook server.
	// Disable locally with ENABLE_WEBHOOKS=false to run without cert-manager.
	EnableWebhooks bool `env:"ENABLE_WEBHOOKS" envDefault:"true"`

	// EnableStartupSpreading enables the startup spreading mechanism that
	// distributes warm-start reconciliations over time to avoid API rate-limit exhaustion.
	EnableStartupSpreading bool `env:"ENABLE_STARTUP_SPREADING" envDefault:"true"`
}

// Config holds all operator configuration loaded from environment variables at startup.
type Config struct {
	Features Features

	// Kubernetes scope
	WatchNamespace                string `env:"WATCH_NAMESPACE,notEmpty"`
	AppCredentialsSecretNamespace string `env:"APP_CREDENTIALS_SECRET_NAMESPACE,notEmpty"`

	// Repository reconciliation
	RepositoryFinalizerMode string `env:"REPOSITORY_FINALIZER_MODE"`

	// Team reconciliation
	GitHubMemberSuffix string `env:"GITHUB_MEMBER_SUFFIX"`

	// Startup spreading — numeric tuning; defaults match spreading.DefaultSpreadPeriodMinutes
	// and spreading.DefaultSpreadIntervalMinutes.
	SpreadPeriodMinutes   int `env:"STARTUP_SPREAD_PERIOD_MINUTES" envDefault:"5"`
	SpreadIntervalMinutes int `env:"SPREAD_INTERVAL_MINUTES"       envDefault:"180"`

	// Logging — consumed early in main.go before the structured logger is fully initialised.
	LogLevel  string `env:"LOG_LEVEL"`
	LogFormat string `env:"LOG_FORMAT"`
}

func Parse() (Config, error) {
	c := Config{}
	return c, env.Parse(&c)
}
