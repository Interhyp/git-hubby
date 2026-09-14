package ratelimit

import (
	"net/http"
	"strings"
)

// Category represents a GitHub API rate limit category as returned by GET /rate_limit.
type Category string

const (
	// CategoryCore covers all general REST API calls. This is the primary category for git-hubby.
	CategoryCore Category = "core"
	// CategoryGraphQL covers GraphQL API calls (/graphql). Planned for future use.
	CategoryGraphQL Category = "graphql"
	// CategorySearch covers general Search API calls (/search/*).
	CategorySearch Category = "search"
	// CategoryCodeSearch covers Code Search API calls (/search/code).
	CategoryCodeSearch Category = "code_search"
)

// MonitoredCategories lists all categories that the OrgRateLimitRegistry tracks.
// To add a new category in the future, define a new constant and append it here.
var MonitoredCategories = []Category{
	CategoryCore,
	CategoryGraphQL,
	CategorySearch,
	CategoryCodeSearch,
}

// ClassifyRequest determines the rate limit category for an HTTP request/response pair.
// It prefers the explicit X-RateLimit-Resource response header (set by GitHub) over URL-path heuristics.
func ClassifyRequest(req *http.Request, resourceHeader string) Category {
	if resourceHeader != "" {
		return Category(resourceHeader)
	}
	return ClassifyByPath(req.URL.Path)
}

// ClassifyByPath determines the rate limit category based on the request URL path.
// Used as a fallback when the X-RateLimit-Resource header is absent.
func ClassifyByPath(path string) Category {
	switch {
	case path == "/graphql":
		return CategoryGraphQL
	case strings.HasPrefix(path, "/search/code"):
		return CategoryCodeSearch
	case strings.HasPrefix(path, "/search/"):
		return CategorySearch
	default:
		return CategoryCore
	}
}
