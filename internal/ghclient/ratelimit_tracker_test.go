package ghclient

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Interhyp/git-hubby/internal/ratelimit"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// stubTransport returns a pre-built response without hitting the network.
type stubTransport struct {
	resp *http.Response
	err  error
}

func (s *stubTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	return s.resp, s.err
}

func buildStubResponse(statusCode int, headers map[string]string) *http.Response {
	resp := &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
	}
	for k, v := range headers {
		resp.Header.Set(k, v)
	}
	return resp
}

var _ = Describe("rateLimitTrackerTransport", func() {
	var (
		registry *ratelimit.OrgRateLimitRegistry
		orgLogin string
		appID    int64
		resetAt  time.Time
	)

	BeforeEach(func() {
		registry = ratelimit.NewOrgRateLimitRegistry(ratelimit.DefaultOrgRegistryConfig())
		orgLogin = "my-org"
		appID = int64(42)
		resetAt = time.Now().Add(time.Hour).Truncate(time.Second)
	})

	newTracker := func(inner http.RoundTripper) http.RoundTripper {
		return newRateLimitTrackerTransport(inner, orgLogin, appID, registry)
	}

	makeRequest := func(path string) *http.Request {
		req, err := http.NewRequest(http.MethodGet, "https://api.github.com"+path, nil)
		Expect(err).NotTo(HaveOccurred())
		return req
	}

	Describe("RoundTrip", func() {
		Context("with complete rate limit headers", func() {
			It("updates the registry for the core category", func() {
				stub := &stubTransport{resp: buildStubResponse(200, map[string]string{
					"X-RateLimit-Remaining": "4500",
					"X-RateLimit-Limit":     "5000",
					"X-RateLimit-Reset":     fmt.Sprintf("%d", resetAt.Unix()),
				})}

				tracker := newTracker(stub)
				resp, err := tracker.RoundTrip(makeRequest("/orgs/my-org"))
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))

				state := registry.GetCategoryState(orgLogin, ratelimit.CategoryCore)
				Expect(state).NotTo(BeNil())
				Expect(state.Remaining).To(Equal(4500))
				Expect(state.Limit).To(Equal(5000))
				Expect(state.ResetTime.Unix()).To(Equal(resetAt.Unix()))
			})

			It("uses X-RateLimit-Resource header to determine category when present", func() {
				stub := &stubTransport{resp: buildStubResponse(200, map[string]string{
					"X-RateLimit-Resource":  "search",
					"X-RateLimit-Remaining": "20",
					"X-RateLimit-Limit":     "30",
					"X-RateLimit-Reset":     fmt.Sprintf("%d", resetAt.Unix()),
				})}

				tracker := newTracker(stub)
				_, err := tracker.RoundTrip(makeRequest("/orgs/my-org")) // path would classify as core
				Expect(err).NotTo(HaveOccurred())

				// Should be stored as search (from header), not core (from path)
				Expect(registry.GetCategoryState(orgLogin, ratelimit.CategorySearch)).NotTo(BeNil())
				Expect(registry.GetCategoryState(orgLogin, ratelimit.CategoryCore)).To(BeNil())
			})

			It("classifies graphql by path when header is absent", func() {
				stub := &stubTransport{resp: buildStubResponse(200, map[string]string{
					"X-RateLimit-Remaining": "3000",
					"X-RateLimit-Limit":     "5000",
					"X-RateLimit-Reset":     fmt.Sprintf("%d", resetAt.Unix()),
				})}

				tracker := newTracker(stub)
				_, err := tracker.RoundTrip(makeRequest("/graphql"))
				Expect(err).NotTo(HaveOccurred())

				Expect(registry.GetCategoryState(orgLogin, ratelimit.CategoryGraphQL)).NotTo(BeNil())
				Expect(registry.GetCategoryState(orgLogin, ratelimit.CategoryCore)).To(BeNil())
			})
		})

		Context("with missing or malformed headers", func() {
			It("does not update registry when all headers are missing", func() {
				stub := &stubTransport{resp: buildStubResponse(200, nil)}
				tracker := newTracker(stub)
				_, err := tracker.RoundTrip(makeRequest("/orgs/my-org"))
				Expect(err).NotTo(HaveOccurred())
				Expect(registry.GetState(orgLogin)).To(BeNil())
			})

			It("does not update registry when limit header is zero", func() {
				stub := &stubTransport{resp: buildStubResponse(200, map[string]string{
					"X-RateLimit-Remaining": "100",
					"X-RateLimit-Limit":     "0",
					"X-RateLimit-Reset":     fmt.Sprintf("%d", resetAt.Unix()),
				})}
				tracker := newTracker(stub)
				_, err := tracker.RoundTrip(makeRequest("/orgs/my-org"))
				Expect(err).NotTo(HaveOccurred())
				Expect(registry.GetState(orgLogin)).To(BeNil())
			})

			It("does not update registry when reset header is missing", func() {
				stub := &stubTransport{resp: buildStubResponse(200, map[string]string{
					"X-RateLimit-Remaining": "100",
					"X-RateLimit-Limit":     "5000",
				})}
				tracker := newTracker(stub)
				_, err := tracker.RoundTrip(makeRequest("/orgs/my-org"))
				Expect(err).NotTo(HaveOccurred())
				Expect(registry.GetState(orgLogin)).To(BeNil())
			})
		})

		Context("when the inner transport returns an error", func() {
			It("propagates the error and does not update the registry", func() {
				stub := &stubTransport{err: fmt.Errorf("network error")}
				tracker := newTracker(stub)
				resp, err := tracker.RoundTrip(makeRequest("/orgs/my-org"))
				Expect(err).To(HaveOccurred())
				Expect(resp).To(BeNil())
				Expect(registry.GetState(orgLogin)).To(BeNil())
			})
		})

		Context("with non-200 responses", func() {
			It("still updates the registry when headers are present on a 403", func() {
				stub := &stubTransport{resp: buildStubResponse(403, map[string]string{
					"X-RateLimit-Remaining": "0",
					"X-RateLimit-Limit":     "5000",
					"X-RateLimit-Reset":     fmt.Sprintf("%d", resetAt.Unix()),
				})}
				tracker := newTracker(stub)
				resp, err := tracker.RoundTrip(makeRequest("/orgs/my-org"))
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(403))

				state := registry.GetCategoryState(orgLogin, ratelimit.CategoryCore)
				Expect(state).NotTo(BeNil())
				Expect(state.Remaining).To(Equal(0))
			})
		})
	})
})
