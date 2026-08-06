package ghclient

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/google/go-github/v90/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakeClient implements GitHubClient (via embedding a nil) with only the methods
// we need for testing. It counts calls to verify caching behavior.
type fakeClient struct {
	GitHubClient // embed nil — panics on any unimplemented method (tests only call what we override)

	teamCallCount  atomic.Int64
	appsCallCount  atomic.Int64
	rolesCallCount atomic.Int64

	teams []*github.Team
	apps  []*github.Installation
	roles []*github.CustomOrgRole
}

func (f *fakeClient) GetAllTeamsForOrg(_ context.Context, _ string) ([]*github.Team, error) {
	f.teamCallCount.Add(1)
	return f.teams, nil
}

func (f *fakeClient) GetGitHubAppsInstallations(_ context.Context, _ string) ([]*github.Installation, error) {
	f.appsCallCount.Add(1)
	return f.apps, nil
}

func (f *fakeClient) GetAllOrgRoles(_ context.Context, _ string) ([]*github.CustomOrgRole, error) {
	f.rolesCallCount.Add(1)
	return f.roles, nil
}

var _ = Describe("CachingClient", func() {
	var (
		inner  *fakeClient
		cached *CachingClient
	)

	BeforeEach(func() {
		inner = &fakeClient{
			teams: []*github.Team{
				{ID: ptr(int64(1)), Slug: ptr("team-a")},
				{ID: ptr(int64(2)), Slug: ptr("team-b")},
			},
			apps: []*github.Installation{
				{ID: ptr(int64(10)), AppSlug: ptr("github-actions")},
			},
			roles: []*github.CustomOrgRole{
				{ID: ptr(int64(99)), Name: ptr("maintain")},
			},
		}
		cached = NewCachingClient(inner, 5*time.Minute)
	})

	Describe("WithCache context", func() {
		It("isCacheEnabled returns false for plain context", func() {
			Expect(isCacheEnabled(context.Background())).To(BeFalse())
		})

		It("isCacheEnabled returns true for WithCache context", func() {
			ctx := WithCache(context.Background())
			Expect(isCacheEnabled(ctx)).To(BeTrue())
		})
	})

	Describe("GetAllTeamsForOrg", func() {
		Context("without cache context", func() {
			It("always calls the inner client", func() {
				ctx := context.Background()

				result1, err := cached.GetAllTeamsForOrg(ctx, "my-org")
				Expect(err).NotTo(HaveOccurred())
				Expect(result1).To(HaveLen(2))

				result2, err := cached.GetAllTeamsForOrg(ctx, "my-org")
				Expect(err).NotTo(HaveOccurred())
				Expect(result2).To(HaveLen(2))

				Expect(inner.teamCallCount.Load()).To(Equal(int64(2)))
			})
		})

		Context("with cache context", func() {
			It("caches the result on first call and returns cached on second", func() {
				ctx := WithCache(context.Background())

				result1, err := cached.GetAllTeamsForOrg(ctx, "my-org")
				Expect(err).NotTo(HaveOccurred())
				Expect(result1).To(HaveLen(2))

				result2, err := cached.GetAllTeamsForOrg(ctx, "my-org")
				Expect(err).NotTo(HaveOccurred())
				Expect(result2).To(HaveLen(2))

				Expect(inner.teamCallCount.Load()).To(Equal(int64(1)))
			})

			It("returns fresh data after TTL expires", func() {
				shortTTL := NewCachingClient(inner, 1*time.Millisecond)
				ctx := WithCache(context.Background())

				_, err := shortTTL.GetAllTeamsForOrg(ctx, "my-org")
				Expect(err).NotTo(HaveOccurred())

				time.Sleep(5 * time.Millisecond)

				_, err = shortTTL.GetAllTeamsForOrg(ctx, "my-org")
				Expect(err).NotTo(HaveOccurred())

				Expect(inner.teamCallCount.Load()).To(Equal(int64(2)))
			})
		})

		Context("mixed cache and non-cache calls", func() {
			It("non-cache call does not invalidate existing cache", func() {
				cacheCtx := WithCache(context.Background())
				plainCtx := context.Background()

				// Populate cache
				_, err := cached.GetAllTeamsForOrg(cacheCtx, "my-org")
				Expect(err).NotTo(HaveOccurred())
				Expect(inner.teamCallCount.Load()).To(Equal(int64(1)))

				// Non-cache call goes to API directly
				_, err = cached.GetAllTeamsForOrg(plainCtx, "my-org")
				Expect(err).NotTo(HaveOccurred())
				Expect(inner.teamCallCount.Load()).To(Equal(int64(2)))

				// Cache still valid for cache-enabled calls
				_, err = cached.GetAllTeamsForOrg(cacheCtx, "my-org")
				Expect(err).NotTo(HaveOccurred())
				Expect(inner.teamCallCount.Load()).To(Equal(int64(2)))
			})
		})
	})

	Describe("GetGitHubAppsInstallations", func() {
		Context("with cache context", func() {
			It("caches the result", func() {
				ctx := WithCache(context.Background())

				result, err := cached.GetGitHubAppsInstallations(ctx, "my-org")
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].GetAppSlug()).To(Equal("github-actions"))

				_, err = cached.GetGitHubAppsInstallations(ctx, "my-org")
				Expect(err).NotTo(HaveOccurred())

				Expect(inner.appsCallCount.Load()).To(Equal(int64(1)))
			})
		})

		Context("without cache context", func() {
			It("does not cache", func() {
				ctx := context.Background()

				_, _ = cached.GetGitHubAppsInstallations(ctx, "my-org")
				_, _ = cached.GetGitHubAppsInstallations(ctx, "my-org")

				Expect(inner.appsCallCount.Load()).To(Equal(int64(2)))
			})
		})
	})

	Describe("GetAllOrgRoles", func() {
		Context("with cache context", func() {
			It("caches the result", func() {
				ctx := WithCache(context.Background())

				result, err := cached.GetAllOrgRoles(ctx, "my-org")
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].GetName()).To(Equal("maintain"))

				_, err = cached.GetAllOrgRoles(ctx, "my-org")
				Expect(err).NotTo(HaveOccurred())

				Expect(inner.rolesCallCount.Load()).To(Equal(int64(1)))
			})
		})

		Context("without cache context", func() {
			It("does not cache", func() {
				ctx := context.Background()

				_, _ = cached.GetAllOrgRoles(ctx, "my-org")
				_, _ = cached.GetAllOrgRoles(ctx, "my-org")

				Expect(inner.rolesCallCount.Load()).To(Equal(int64(2)))
			})
		})
	})
})

func ptr[T any](v T) *T {
	return &v
}
