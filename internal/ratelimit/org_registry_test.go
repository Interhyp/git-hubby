package ratelimit_test

import (
	"net/http"
	"time"

	"github.com/Interhyp/git-hubby/internal/config"
	"github.com/Interhyp/git-hubby/internal/ratelimit"
	"github.com/google/go-github/v90/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("OrgRateLimitRegistry", func() {
	var (
		registry *ratelimit.OrgRateLimitRegistry
		config   ratelimit.OrgRegistryConfig
	)

	BeforeEach(func() {
		config = ratelimit.DefaultOrgRegistryConfig()
		registry = ratelimit.NewOrgRateLimitRegistry(config)
	})

	Describe("Update", func() {
		It("stores state for a monitored category", func() {
			reset := time.Now().Add(time.Hour)
			registry.Update("my-org", ratelimit.CategoryCore, 50, 5000, reset, 1)

			state := registry.GetCategoryState("my-org", ratelimit.CategoryCore)
			Expect(state).NotTo(BeNil())
			Expect(state.Remaining).To(Equal(50))
			Expect(state.Limit).To(Equal(5000))
			Expect(state.ResetTime).To(BeTemporally("~", reset, time.Second))
		})

		It("ignores unmonitored categories", func() {
			reset := time.Now().Add(time.Hour)
			registry.Update("my-org", ratelimit.Category("audit_log"), 10, 1750, reset, 1)

			state := registry.GetCategoryState("my-org", ratelimit.Category("audit_log"))
			Expect(state).To(BeNil())
		})

		It("updates existing state for the same category", func() {
			reset1 := time.Now().Add(time.Hour)
			reset2 := time.Now().Add(2 * time.Hour)
			registry.Update("my-org", ratelimit.CategoryCore, 500, 5000, reset1, 1)
			registry.Update("my-org", ratelimit.CategoryCore, 200, 5000, reset2, 1)

			state := registry.GetCategoryState("my-org", ratelimit.CategoryCore)
			Expect(state.Remaining).To(Equal(200))
		})

		It("tracks multiple orgs independently", func() {
			reset := time.Now().Add(time.Hour)
			registry.Update("org-a", ratelimit.CategoryCore, 100, 5000, reset, 1)
			registry.Update("org-b", ratelimit.CategoryCore, 500, 5000, reset, 2)

			stateA := registry.GetCategoryState("org-a", ratelimit.CategoryCore)
			stateB := registry.GetCategoryState("org-b", ratelimit.CategoryCore)
			Expect(stateA.Remaining).To(Equal(100))
			Expect(stateB.Remaining).To(Equal(500))
		})

		It("still stores state for monitored categories without a threshold", func() {
			// Categories like search/graphql are monitored (tracked) but may not have
			// a stall threshold configured. Update should still record their state.
			reset := time.Now().Add(time.Hour)
			registry.Update("my-org", ratelimit.CategorySearch, 20, 30, reset, 1)

			state := registry.GetCategoryState("my-org", ratelimit.CategorySearch)
			Expect(state).NotTo(BeNil())
			Expect(state.Remaining).To(Equal(20))
		})
	})

	Describe("UpdateFromRateLimitResponse", func() {
		It("populates all known categories from the response", func() {
			resetTime := github.Timestamp{Time: time.Now().Add(time.Hour)}
			rateLimits := &github.RateLimits{
				Core:       &github.Rate{Remaining: 4000, Limit: 5000, Reset: resetTime},
				GraphQL:    &github.Rate{Remaining: 3000, Limit: 5000, Reset: resetTime},
				Search:     &github.Rate{Remaining: 25, Limit: 30, Reset: resetTime},
				CodeSearch: &github.Rate{Remaining: 8, Limit: 10, Reset: resetTime},
			}
			registry.UpdateFromRateLimitResponse("my-org", rateLimits, 42)

			Expect(registry.GetCategoryState("my-org", ratelimit.CategoryCore).Remaining).To(Equal(4000))
			Expect(registry.GetCategoryState("my-org", ratelimit.CategoryGraphQL).Remaining).To(Equal(3000))
			Expect(registry.GetCategoryState("my-org", ratelimit.CategorySearch).Remaining).To(Equal(25))
			Expect(registry.GetCategoryState("my-org", ratelimit.CategoryCodeSearch).Remaining).To(Equal(8))
		})

		It("is a no-op for nil input", func() {
			registry.UpdateFromRateLimitResponse("my-org", nil, 1)
			Expect(registry.GetState("my-org")).To(BeNil())
		})

		It("skips nil sub-fields gracefully", func() {
			rateLimits := &github.RateLimits{
				Core: &github.Rate{Remaining: 1000, Limit: 5000, Reset: github.Timestamp{Time: time.Now().Add(time.Hour)}},
			}
			registry.UpdateFromRateLimitResponse("my-org", rateLimits, 1)

			Expect(registry.GetCategoryState("my-org", ratelimit.CategoryCore)).NotTo(BeNil())
			Expect(registry.GetCategoryState("my-org", ratelimit.CategoryGraphQL)).To(BeNil())
		})
	})

	Describe("ShouldStall", func() {
		Context("when remaining is below the threshold", func() {
			It("returns stalled=true and a positive delay", func() {
				future := time.Now().Add(30 * time.Minute)
				registry.Update("my-org", ratelimit.CategoryCore, 50, 5000, future, 1) // threshold is 100

				stalled, delay := registry.ShouldStall("my-org", ratelimit.CategoryCore)
				Expect(stalled).To(BeTrue())
				Expect(delay).To(BeNumerically(">", 0))
			})
		})

		Context("when remaining is at or above the threshold", func() {
			It("returns stalled=false", func() {
				registry.Update("my-org", ratelimit.CategoryCore, 200, 5000, time.Now().Add(time.Hour), 1)

				stalled, delay := registry.ShouldStall("my-org", ratelimit.CategoryCore)
				Expect(stalled).To(BeFalse())
				Expect(delay).To(Equal(time.Duration(0)))
			})
		})

		Context("when reset time is already in the past", func() {
			It("returns stalled=false because the rate limit window has renewed", func() {
				past := time.Now().Add(-5 * time.Minute)
				registry.Update("my-org", ratelimit.CategoryCore, 5, 5000, past, 1)

				stalled, delay := registry.ShouldStall("my-org", ratelimit.CategoryCore)
				Expect(stalled).To(BeFalse())
				Expect(delay).To(Equal(time.Duration(0)))
			})
		})

		Context("when the org has no recorded state", func() {
			It("returns stalled=false (unknown = allow)", func() {
				stalled, delay := registry.ShouldStall("unknown-org", ratelimit.CategoryCore)
				Expect(stalled).To(BeFalse())
				Expect(delay).To(Equal(time.Duration(0)))
			})
		})

		Context("when checking multiple categories", func() {
			var multiCatRegistry *ratelimit.OrgRateLimitRegistry

			BeforeEach(func() {
				// Use a custom config with thresholds for search and code_search
				// to test multi-category stalling behavior.
				multiCatConfig := ratelimit.OrgRegistryConfig{
					CategoryThresholds: map[ratelimit.Category]int{
						ratelimit.CategoryCore:       100,
						ratelimit.CategorySearch:     3,
						ratelimit.CategoryCodeSearch: 2,
					},
					ResetGracePeriod:   config.ResetGracePeriod,
					StalenessThreshold: config.StalenessThreshold,
				}
				multiCatRegistry = ratelimit.NewOrgRateLimitRegistry(multiCatConfig)
			})

			It("returns stalled=true when any category is below threshold", func() {
				reset := time.Now().Add(time.Hour)
				multiCatRegistry.Update("my-org", ratelimit.CategoryCore, 500, 5000, reset, 1) // above threshold
				multiCatRegistry.Update("my-org", ratelimit.CategorySearch, 1, 30, reset, 1)   // below threshold=3

				stalled, _ := multiCatRegistry.ShouldStall("my-org", ratelimit.CategoryCore, ratelimit.CategorySearch)
				Expect(stalled).To(BeTrue())
			})

			It("returns maximum delay across stalled categories", func() {
				reset1 := time.Now().Add(10 * time.Minute)
				reset2 := time.Now().Add(30 * time.Minute)
				multiCatRegistry.Update("my-org", ratelimit.CategorySearch, 1, 30, reset1, 1)
				multiCatRegistry.Update("my-org", ratelimit.CategoryCodeSearch, 0, 10, reset2, 1)

				_, delay := multiCatRegistry.ShouldStall("my-org", ratelimit.CategorySearch, ratelimit.CategoryCodeSearch)
				// delay should be approximately 30 minutes + grace period
				Expect(delay).To(BeNumerically(">", 25*time.Minute))
			})
		})

		Context("when a category has no threshold configured", func() {
			It("never stalls for that category", func() {
				// graphql has no threshold in DefaultOrgRegistryConfig
				registry.Update("my-org", ratelimit.CategoryGraphQL, 0, 5000, time.Now().Add(time.Hour), 1)

				stalled, _ := registry.ShouldStall("my-org", ratelimit.CategoryGraphQL)
				Expect(stalled).To(BeFalse())
			})
		})
	})

	Describe("IsStale", func() {
		It("returns true for unknown orgs", func() {
			Expect(registry.IsStale("unknown")).To(BeTrue())
		})

		It("returns false for recently updated orgs", func() {
			registry.Update("my-org", ratelimit.CategoryCore, 500, 5000, time.Now().Add(time.Hour), 1)
			Expect(registry.IsStale("my-org")).To(BeFalse())
		})

		It("returns true when data is older than the staleness threshold", func() {
			cfg := ratelimit.OrgRegistryConfig{
				CategoryThresholds: map[ratelimit.Category]int{ratelimit.CategoryCore: 100},
				ResetGracePeriod:   0,
				StalenessThreshold: 1 * time.Millisecond,
			}
			r := ratelimit.NewOrgRateLimitRegistry(cfg)
			r.Update("my-org", ratelimit.CategoryCore, 500, 5000, time.Now().Add(time.Hour), 1)
			time.Sleep(5 * time.Millisecond)
			Expect(r.IsStale("my-org")).To(BeTrue())
		})
	})

	Describe("GetState", func() {
		It("returns nil for unknown orgs", func() {
			Expect(registry.GetState("nobody")).To(BeNil())
		})

		It("returns the org state after updates", func() {
			registry.Update("my-org", ratelimit.CategoryCore, 100, 5000, time.Now().Add(time.Hour), 42)
			state := registry.GetState("my-org")
			Expect(state).NotTo(BeNil())
			Expect(state.OrgLogin).To(Equal("my-org"))
			Expect(state.AppID).To(Equal(int64(42)))
			Expect(state.Categories).To(HaveKey(ratelimit.CategoryCore))
		})
	})
})

var _ = Describe("ClassifyRequest", func() {
	DescribeTable("classifies by X-RateLimit-Resource header when present",
		func(headerValue string, expected ratelimit.Category) {
			req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/orgs/test", nil)
			result := ratelimit.ClassifyRequest(req, headerValue)
			Expect(result).To(Equal(expected))
		},
		Entry("core header", "core", ratelimit.CategoryCore),
		Entry("graphql header", "graphql", ratelimit.CategoryGraphQL),
		Entry("search header", "search", ratelimit.CategorySearch),
		Entry("code_search header", "code_search", ratelimit.CategoryCodeSearch),
	)

	DescribeTable("falls back to URL path classification when header is absent",
		func(path string, expected ratelimit.Category) {
			req, _ := http.NewRequest(http.MethodGet, "https://api.github.com"+path, nil)
			result := ratelimit.ClassifyRequest(req, "")
			Expect(result).To(Equal(expected))
		},
		Entry("graphql endpoint", "/graphql", ratelimit.CategoryGraphQL),
		Entry("code search endpoint", "/search/code", ratelimit.CategoryCodeSearch),
		Entry("search endpoint", "/search/repositories", ratelimit.CategorySearch),
		Entry("REST endpoint", "/orgs/my-org", ratelimit.CategoryCore),
		Entry("root", "/", ratelimit.CategoryCore),
	)
})

var _ = Describe("ClassifyByPath", func() {
	DescribeTable("classifies paths correctly",
		func(path string, expected ratelimit.Category) {
			Expect(ratelimit.ClassifyByPath(path)).To(Equal(expected))
		},
		Entry("/graphql", "/graphql", ratelimit.CategoryGraphQL),
		Entry("/search/code", "/search/code", ratelimit.CategoryCodeSearch),
		Entry("/search/code?q=test", "/search/code?q=test", ratelimit.CategoryCodeSearch),
		Entry("/search/repositories", "/search/repositories", ratelimit.CategorySearch),
		Entry("/search/", "/search/", ratelimit.CategorySearch),
		Entry("/repos/owner/repo", "/repos/owner/repo", ratelimit.CategoryCore),
		Entry("/orgs/test", "/orgs/test", ratelimit.CategoryCore),
		Entry("empty string", "", ratelimit.CategoryCore),
	)
})

// This test ensures that every entry in MonitoredCategories has a corresponding
// env-configurable threshold wired in ConfiguredThresholds(). If a developer adds
// a new category to MonitoredCategories but forgets to add the config field and
// wiring, this test fails — catching the issue at CI time, long before any Helm
// chart user is affected.
var _ = Describe("ConfiguredThresholds coverage", func() {
	It("covers every MonitoredCategory with a config-driven threshold entry", func() {
		configured := ratelimit.ConfiguredThresholds(config.Config{})
		for _, cat := range ratelimit.MonitoredCategories {
			_, exists := configured[cat]
			Expect(exists).To(BeTrue(),
				"MonitoredCategory %q has no entry in ConfiguredThresholds — "+
					"add a StallThreshold* field to config.RateLimitConfig and wire it in "+
					"ratelimit.ConfiguredThresholds()", cat)
		}
	})
})
