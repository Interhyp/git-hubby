package config

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Describe("duration getters", func() {
		It("converts the configured values", func() {
			c := Config{
				SpreadPeriodMinutes:               5,
				SpreadIntervalMinutes:             180,
				GitHubStateRequeueIntervalMinutes: 15,
				RateLimitConfig: RateLimitConfig{
					ResetGracePeriodSeconds:   10,
					StalenessThresholdMinutes: 3,
				},
			}

			Expect(c.SpreadPeriod()).To(Equal(5 * time.Minute))
			Expect(c.SpreadInterval()).To(Equal(180 * time.Minute))
			Expect(c.SuccessRequeueInterval()).To(Equal(180 * time.Minute))
			Expect(c.GitHubStateRequeueInterval()).To(Equal(15 * time.Minute))
			Expect(c.RateLimitConfig.ResetGracePeriod()).To(Equal(10 * time.Second))
			Expect(c.RateLimitConfig.StalenessThreshold()).To(Equal(3 * time.Minute))
		})

		It("returns zero durations for an empty config", func() {
			c := Config{}

			Expect(c.SpreadPeriod()).To(BeZero())
			Expect(c.SpreadInterval()).To(BeZero())
			Expect(c.GitHubStateRequeueInterval()).To(BeZero())
			Expect(c.RateLimitConfig.ResetGracePeriod()).To(BeZero())
			Expect(c.RateLimitConfig.StalenessThreshold()).To(BeZero())
		})
	})

	Describe("Parse", func() {
		It("applies the defaults", func() {
			GinkgoT().Setenv("WATCH_NAMESPACE", "ns")
			GinkgoT().Setenv("APP_CREDENTIALS_SECRET_NAMESPACE", "ns")

			c, err := Parse()

			Expect(err).NotTo(HaveOccurred())
			Expect(c.SpreadPeriod()).To(Equal(5 * time.Minute))
			Expect(c.SpreadInterval()).To(Equal(180 * time.Minute))
			Expect(c.GitHubStateRequeueInterval()).To(Equal(15 * time.Minute))
			Expect(c.RateLimitConfig.ResetGracePeriod()).To(Equal(10 * time.Second))
			Expect(c.RateLimitConfig.StalenessThreshold()).To(Equal(5 * time.Minute))
		})
	})
})
