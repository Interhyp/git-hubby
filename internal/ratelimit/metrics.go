package ratelimit

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	rateLimitLabels    = []string{"organization", "category"}
	rateLimitRemaining = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "git_hubby_github_rate_limit_remaining",
		Help: "Last observed remaining primary rate limit budget",
	}, rateLimitLabels)
	rateLimitBudget = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "git_hubby_github_rate_limit_budget",
		Help: "Last observed primary rate limit budget",
	}, rateLimitLabels)
	rateLimitReset = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "git_hubby_github_rate_limit_reset_timestamp_seconds",
		Help: "Last observed primary rate limit reset time as a Unix timestamp",
	}, rateLimitLabels)
	rateLimitObserved = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "git_hubby_github_rate_limit_observed_timestamp_seconds",
		Help: "Time of the last primary rate limit observation as a Unix timestamp",
	}, rateLimitLabels)
	rateLimitThreshold = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "git_hubby_github_rate_limit_stall_threshold",
		Help: "Configured primary rate limit stall threshold, or zero when disabled",
	}, rateLimitLabels)
	rateLimitStallUntil = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "git_hubby_github_rate_limit_stall_until_timestamp_seconds",
		Help: "End of the primary rate limit stall as a Unix timestamp, or zero when the observed budget does not require stalling",
	}, rateLimitLabels)
)

func init() {
	ctrlmetrics.Registry.MustRegister(
		rateLimitRemaining, rateLimitBudget, rateLimitReset,
		rateLimitObserved, rateLimitThreshold, rateLimitStallUntil,
	)
}

// observeRateLimit updates the gauges for an organization and category.
func (r *OrgRateLimitRegistry) observeRateLimit(org string, category Category, state *CategoryState) {
	labels := []string{org, string(category)}
	threshold, configured := r.config.CategoryThresholds[category]
	rateLimitRemaining.WithLabelValues(labels...).Set(float64(state.Remaining))
	rateLimitBudget.WithLabelValues(labels...).Set(float64(state.Limit))
	rateLimitReset.WithLabelValues(labels...).Set(float64(state.ResetTime.Unix()))
	rateLimitObserved.WithLabelValues(labels...).Set(float64(state.LastUpdated.Unix()))
	rateLimitThreshold.WithLabelValues(labels...).Set(float64(threshold))
	stallUntil := 0.0
	if configured && state.Remaining < threshold {
		deadline := state.ResetTime.Add(r.config.ResetGracePeriod)
		stallUntil = float64(deadline.Unix()) + float64(deadline.Nanosecond())/1e9
	}
	rateLimitStallUntil.WithLabelValues(labels...).Set(stallUntil)
}
