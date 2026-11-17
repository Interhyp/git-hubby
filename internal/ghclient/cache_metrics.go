package ghclient

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// responseCacheLookups counts the lookups of the opt-in response cache of the CachingClient. Calls without
	// WithCache bypass the cache and are not counted. Concurrent misses may fetch in parallel, so misses can exceed
	// the number of refreshes.
	responseCacheLookups = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "git_hubby_github_response_cache_lookups_total",
		Help: "Number of lookups in the GitHub response cache per cache and result (hit, miss)",
	}, []string{"cache", "result"})

	// responseCacheInvalidations counts invalidations, e.g. after a lookup failed because of stale data.
	responseCacheInvalidations = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "git_hubby_github_response_cache_invalidations_total",
		Help: "Number of invalidations of the GitHub response cache per cache",
	}, []string{"cache"})
)

func init() {
	ctrlmetrics.Registry.MustRegister(responseCacheLookups, responseCacheInvalidations)
}
