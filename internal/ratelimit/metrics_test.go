package ratelimit_test

import (
	"fmt"
	"strings"
	"time"

	"github.com/Interhyp/git-hubby/internal/ratelimit"
	"github.com/google/go-github/v93/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var _ = Describe("Rate limit metrics", func() {
	var (
		registry  *ratelimit.OrgRateLimitRegistry
		resetTime time.Time
		orgPrefix string
		sequence  int
	)

	BeforeEach(func() {
		sequence++
		orgPrefix = fmt.Sprintf("rate-limit-metrics-test-%d-", sequence)
		registry = ratelimit.NewOrgRateLimitRegistry(ratelimit.DefaultOrgRegistryConfig())
		resetTime = time.Now().Add(time.Hour).Truncate(time.Second)
	})

	It("emits no metrics before the first observation", func() {
		Expect(gatherRateLimitMetrics(orgPrefix)).To(BeEmpty())
	})

	It("exports gauges for header and rate limit response updates", func() {
		before := time.Now().Unix()
		registry.Update(orgPrefix+"a", ratelimit.CategoryCore, 42, 5000, resetTime, 1)
		registry.UpdateFromRateLimitResponse(orgPrefix+"b", &github.RateLimits{
			Search: &github.Rate{Remaining: 25, Limit: 30, Reset: github.Timestamp{Time: resetTime}},
		}, 2)
		after := time.Now().Unix()

		values := gatherRateLimitMetrics(orgPrefix)
		Expect(values).To(HaveLen(12))
		Expect(values).To(HaveKeyWithValue("git_hubby_github_rate_limit_remaining/a/core", float64(42)))
		Expect(values).To(HaveKeyWithValue("git_hubby_github_rate_limit_budget/a/core", float64(5000)))
		Expect(values).To(HaveKeyWithValue("git_hubby_github_rate_limit_reset_timestamp_seconds/a/core", float64(resetTime.Unix())))
		Expect(values["git_hubby_github_rate_limit_observed_timestamp_seconds/a/core"]).To(BeNumerically(">=", before))
		Expect(values["git_hubby_github_rate_limit_observed_timestamp_seconds/a/core"]).To(BeNumerically("<=", after))
		Expect(values).To(HaveKeyWithValue("git_hubby_github_rate_limit_stall_threshold/a/core", float64(100)))
		Expect(values).To(HaveKeyWithValue("git_hubby_github_rate_limit_stall_until_timestamp_seconds/a/core",
			float64(resetTime.Add(ratelimit.DefaultResetGracePeriod).Unix())))
		Expect(values).To(HaveKeyWithValue("git_hubby_github_rate_limit_remaining/b/search", float64(25)))
		Expect(values).To(HaveKeyWithValue("git_hubby_github_rate_limit_budget/b/search", float64(30)))
		Expect(values).To(HaveKeyWithValue("git_hubby_github_rate_limit_reset_timestamp_seconds/b/search", float64(resetTime.Unix())))
		Expect(values["git_hubby_github_rate_limit_observed_timestamp_seconds/b/search"]).To(BeNumerically(">=", before))
		Expect(values["git_hubby_github_rate_limit_observed_timestamp_seconds/b/search"]).To(BeNumerically("<=", after))
		Expect(values).To(HaveKeyWithValue("git_hubby_github_rate_limit_stall_threshold/b/search", float64(0)))
		Expect(values).To(HaveKeyWithValue("git_hubby_github_rate_limit_stall_until_timestamp_seconds/b/search", float64(0)))
	})

	It("follows the strict threshold and reset grace period", func() {
		reset := time.Now().Add(-5 * time.Second).Truncate(time.Second)
		registry.Update(orgPrefix+"a", ratelimit.CategoryCore, 99, 5000, reset, 1)
		Expect(gatherRateLimitMetrics(orgPrefix)["git_hubby_github_rate_limit_stall_until_timestamp_seconds/a/core"]).
			To(Equal(float64(reset.Add(ratelimit.DefaultResetGracePeriod).Unix())))

		reset = time.Now().Add(-time.Minute).Truncate(time.Second)
		registry.Update(orgPrefix+"a", ratelimit.CategoryCore, 99, 5000, reset, 1)
		values := gatherRateLimitMetrics(orgPrefix)
		Expect(values["git_hubby_github_rate_limit_remaining/a/core"]).To(Equal(float64(99)))
		Expect(values["git_hubby_github_rate_limit_stall_until_timestamp_seconds/a/core"]).
			To(Equal(float64(reset.Add(ratelimit.DefaultResetGracePeriod).Unix())))
		Expect(values["git_hubby_github_rate_limit_stall_until_timestamp_seconds/a/core"]).To(BeNumerically("<", time.Now().Unix()))

		registry.Update(orgPrefix+"a", ratelimit.CategoryCore, 100, 5000, resetTime, 1)
		Expect(gatherRateLimitMetrics(orgPrefix)["git_hubby_github_rate_limit_stall_until_timestamp_seconds/a/core"]).To(Equal(float64(0)))

		registry.Update(orgPrefix+"a", ratelimit.CategorySearch, -1, 30, resetTime, 1)
		Expect(gatherRateLimitMetrics(orgPrefix)["git_hubby_github_rate_limit_stall_until_timestamp_seconds/a/search"]).To(Equal(float64(0)))
	})

	It("preserves fractional seconds in the stall deadline", func() {
		cfg := ratelimit.DefaultOrgRegistryConfig()
		cfg.ResetGracePeriod = 1500 * time.Millisecond
		registry = ratelimit.NewOrgRateLimitRegistry(cfg)
		registry.Update(orgPrefix+"a", ratelimit.CategoryCore, 1, 5000, resetTime.Add(250*time.Millisecond), 1)

		deadline := gatherRateLimitMetrics(orgPrefix)["git_hubby_github_rate_limit_stall_until_timestamp_seconds/a/core"]
		Expect(deadline).To(Equal(float64(resetTime.Unix()) + 1.75))
	})

	It("updates the stall deadline when a newer observation arrives", func() {
		cfg := ratelimit.DefaultOrgRegistryConfig()
		cfg.ResetGracePeriod = 0
		registry = ratelimit.NewOrgRateLimitRegistry(cfg)
		registry.Update(orgPrefix+"a", ratelimit.CategoryCore, 1, 5000, resetTime.Add(-time.Minute), 1)
		registry.Update(orgPrefix+"a", ratelimit.CategoryCore, 1, 5000, resetTime, 1)
		Expect(gatherRateLimitMetrics(orgPrefix)["git_hubby_github_rate_limit_stall_until_timestamp_seconds/a/core"]).
			To(Equal(float64(resetTime.Unix())))
	})

	It("collects safely while observations are updated", func() {
		done := make(chan struct{})
		go func() {
			defer close(done)
			for i := range 500 {
				registry.Update(orgPrefix+"a", ratelimit.CategoryCore, i, 5000, resetTime, 1)
			}
		}()
		for range 100 {
			_, err := ctrlmetrics.Registry.Gather()
			Expect(err).NotTo(HaveOccurred())
		}
		<-done
		Expect(gatherRateLimitMetrics(orgPrefix)["git_hubby_github_rate_limit_remaining/a/core"]).To(Equal(float64(499)))
	})
})

func gatherRateLimitMetrics(orgPrefix string) map[string]float64 {
	families, err := ctrlmetrics.Registry.Gather()
	Expect(err).NotTo(HaveOccurred())
	values := make(map[string]float64)
	for _, family := range families {
		if !strings.HasPrefix(family.GetName(), "git_hubby_github_rate_limit_") {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := make(map[string]string)
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			if !strings.HasPrefix(labels["organization"], orgPrefix) {
				continue
			}
			Expect(labels).To(HaveLen(2))
			Expect(labels).To(HaveKey("category"))
			Expect(metric.GetGauge()).NotTo(BeNil())
			org := strings.TrimPrefix(labels["organization"], orgPrefix)
			key := fmt.Sprintf("%s/%s/%s", family.GetName(), org, labels["category"])
			values[key] = metric.GetGauge().GetValue()
		}
	}
	return values
}
