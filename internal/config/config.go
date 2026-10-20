package config

import (
	"time"

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

// RateLimitConfig holds configuration for the per-org rate limit registry.
type RateLimitConfig struct {
	// StallThresholdCore is the minimum core API calls remaining before stalling an org.
	StallThresholdCore int `env:"RATE_LIMIT_STALL_THRESHOLD_CORE" envDefault:"100"`
	// ResetGracePeriodSeconds is added to the GitHub reset time before allowing reconciliation to resume.
	// Use ResetGracePeriod to get it as a duration.
	ResetGracePeriodSeconds int `env:"RATE_LIMIT_RESET_GRACE_PERIOD_SECONDS" envDefault:"10"`
	// StalenessThresholdMinutes is how many minutes old registry data can be before refreshing.
	// Use StalenessThreshold to get it as a duration.
	StalenessThresholdMinutes int `env:"RATE_LIMIT_STALENESS_THRESHOLD_MINUTES" envDefault:"5"`
}

// ResetGracePeriod returns ResetGracePeriodSeconds as a duration.
func (c RateLimitConfig) ResetGracePeriod() time.Duration {
	return time.Duration(c.ResetGracePeriodSeconds) * time.Second
}

// StalenessThreshold returns StalenessThresholdMinutes as a duration.
func (c RateLimitConfig) StalenessThreshold() time.Duration {
	return time.Duration(c.StalenessThresholdMinutes) * time.Minute
}

// Config holds all operator configuration loaded from environment variables at startup.
type Config struct {
	Features        Features
	RateLimitConfig RateLimitConfig

	// Kubernetes scope
	WatchNamespace                string `env:"WATCH_NAMESPACE,notEmpty"`
	AppCredentialsSecretNamespace string `env:"APP_CREDENTIALS_SECRET_NAMESPACE,notEmpty"`

	// Repository reconciliation
	RepositoryFinalizerMode string `env:"REPOSITORY_FINALIZER_MODE"`

	// Team reconciliation
	GitHubMemberSuffix string `env:"GITHUB_MEMBER_SUFFIX"`

	// Startup spreading, see spreading.NewManager.
	SpreadPeriodMinutes   int `env:"STARTUP_SPREAD_PERIOD_MINUTES" envDefault:"5"`
	SpreadIntervalMinutes int `env:"SPREAD_INTERVAL_MINUTES"       envDefault:"180"`

	// GitHubStateRequeueIntervalMinutes is the requeue interval for resources whose reconciliation requires user
	// action on GitHub (e.g. a referenced team or GitHub App does not exist, a default branch is missing, GitHub
	// rejected the request with 422). No watch event signals such changes, so they are retried more often than
	// the regular success requeue interval. Capped at SPREAD_INTERVAL_MINUTES.
	GitHubStateRequeueIntervalMinutes int `env:"GITHUB_STATE_REQUEUE_INTERVAL_MINUTES" envDefault:"15"`

	// Logging — consumed early in main.go before the structured logger is fully initialised.
	LogLevel  string `env:"LOG_LEVEL"`
	LogFormat string `env:"LOG_FORMAT"`
}

func Parse() (Config, error) {
	c := Config{}
	return c, env.Parse(&c)
}

// SpreadPeriod returns SpreadPeriodMinutes as a duration.
func (c Config) SpreadPeriod() time.Duration {
	return time.Duration(c.SpreadPeriodMinutes) * time.Minute
}

// SpreadInterval returns SpreadIntervalMinutes as a duration.
func (c Config) SpreadInterval() time.Duration {
	return time.Duration(c.SpreadIntervalMinutes) * time.Minute
}

// SuccessRequeueInterval returns the requeue interval after successful reconciliations (drift detection). It equals
// the spread interval, so that requeued resources are distributed like warm-start reconciliations.
func (c Config) SuccessRequeueInterval() time.Duration {
	return c.SpreadInterval()
}

// GitHubStateRequeueInterval returns GitHubStateRequeueIntervalMinutes as a duration.
func (c Config) GitHubStateRequeueInterval() time.Duration {
	return time.Duration(c.GitHubStateRequeueIntervalMinutes) * time.Minute
}
