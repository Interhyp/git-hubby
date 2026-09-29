package orgrec

import (
	"context"
	"errors"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/ghclient"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// callsOfMethod returns the recorded GraphQL calls with the given method name.
func callsOfMethod(calls []ghclientmock.GraphQLCall, method string) []ghclientmock.GraphQLCall {
	var out []ghclientmock.GraphQLCall
	for _, c := range calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

var _ = Describe("ReconcileIpAllowList", func() {
	var (
		ctx        context.Context
		gqlClient  *ghclientmock.MockGraphQLClient
		k8sClient  client.Client
		rec        *GitHubOrgReconciler
		scheme     *runtime.Scheme
		org        *v1alpha1.Organization
		ipSpec     *v1alpha1.IpAllowListSettings
		current    *ghclient.OrgIPAllowListConfig
		currentErr error
		err        error
	)

	BeforeEach(func() {
		ctx = context.Background()
		gqlClient = ghclientmock.NewMockGraphQLClient()

		scheme = runtime.NewScheme()
		Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())

		// Default current GitHub state: enterprise-cloud org, list disabled, no entries.
		current = &ghclient.OrgIPAllowListConfig{
			OwnerID:                     "org-node-id",
			EnabledSetting:              ghclient.IPAllowListDisabled,
			InstalledAppsEnabledSetting: ghclient.IPAllowListDisabled,
			Entries:                     nil,
		}
		currentErr = nil
		ipSpec = nil
	})

	JustBeforeEach(func() {
		gqlClient.GetOrgIPAllowListFunc = func(_ context.Context, _ string) (*ghclient.OrgIPAllowListConfig, error) {
			return current, currentErr
		}

		org = &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{Name: "test-org", Namespace: "default"},
			Spec: v1alpha1.OrganizationSpec{
				Name:                    "test-org",
				GitHubAppInstallationId: new(int64(12345)),
				IpAllowList:             ipSpec,
			},
		}

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(org).
			WithStatusSubresource(org).
			Build()

		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				GraphQLClient: gqlClient,
				Resource:      "test-org",
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Client:   k8sClient,
				Resource: org,
			},
		}

		err = rec.reconcileIpAllowList(ctx)
	})

	Context("when the spec has no IP allow list configuration", func() {
		BeforeEach(func() { ipSpec = nil })

		It("skips reconciliation entirely without querying GitHub", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(gqlClient.GraphQLCalls).To(BeEmpty())
		})
	})

	Context("when IP allow list management is unavailable (enterprise IdP mode / non-GHEC)", func() {
		BeforeEach(func() {
			ipSpec = &v1alpha1.IpAllowListSettings{Enabled: new(true)}
			currentErr = errors.New("IP allow list is not available for this organization")
		})

		It("does not return an error and performs no mutations", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(callsOfMethod(gqlClient.GraphQLCalls, "SetOrgIPAllowListEnabled")).To(BeEmpty())
			Expect(callsOfMethod(gqlClient.GraphQLCalls, "CreateOrgIPAllowListEntry")).To(BeEmpty())
		})
	})

	Context("when the read fails with a transient error", func() {
		BeforeEach(func() {
			ipSpec = &v1alpha1.IpAllowListSettings{Enabled: new(true)}
			currentErr = errors.New("something transient went wrong")
		})

		It("propagates the error", func() {
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when enabling enforcement with new entries", func() {
		BeforeEach(func() {
			ipSpec = &v1alpha1.IpAllowListSettings{
				Enabled: new(true),
				Entries: []v1alpha1.IpAllowListEntry{
					{AllowListValue: "192.0.2.0/24", Name: "Office", IsActive: new(true)},
				},
			}
		})

		It("creates the entry before enabling the setting", func() {
			Expect(err).NotTo(HaveOccurred())

			creates := callsOfMethod(gqlClient.GraphQLCalls, "CreateOrgIPAllowListEntry")
			enables := callsOfMethod(gqlClient.GraphQLCalls, "SetOrgIPAllowListEnabled")
			Expect(creates).To(HaveLen(1))
			Expect(creates[0].Value).To(Equal("192.0.2.0/24"))
			Expect(creates[0].OwnerID).To(Equal("org-node-id"))
			Expect(enables).To(HaveLen(1))
			Expect(enables[0].Enabled).To(BeTrue())

			// Ordering: the create call must be recorded before the enable call.
			var createIdx, enableIdx int
			for i, c := range gqlClient.GraphQLCalls {
				switch c.Method {
				case "CreateOrgIPAllowListEntry":
					createIdx = i
				case "SetOrgIPAllowListEnabled":
					enableIdx = i
				}
			}
			Expect(createIdx).To(BeNumerically("<", enableIdx))
		})
	})

	Context("when the settings already match", func() {
		BeforeEach(func() {
			current.EnabledSetting = ghclient.IPAllowListEnabled
			current.InstalledAppsEnabledSetting = ghclient.IPAllowListDisabled
			ipSpec = &v1alpha1.IpAllowListSettings{
				Enabled:                 new(true),
				EnabledForInstalledApps: new(false),
			}
		})

		It("does not call any setting mutations", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(callsOfMethod(gqlClient.GraphQLCalls, "SetOrgIPAllowListEnabled")).To(BeEmpty())
			Expect(callsOfMethod(gqlClient.GraphQLCalls, "SetOrgIPAllowListForInstalledAppsEnabled")).To(BeEmpty())
		})
	})

	Context("when entries is nil", func() {
		BeforeEach(func() {
			current.Entries = []ghclient.IPAllowListEntry{
				{ID: "id-1", AllowListValue: "203.0.113.1", Name: "Existing", IsActive: true},
			}
			ipSpec = &v1alpha1.IpAllowListSettings{Enabled: new(true)} // Entries omitted
		})

		It("leaves existing entries untouched", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(callsOfMethod(gqlClient.GraphQLCalls, "CreateOrgIPAllowListEntry")).To(BeEmpty())
			Expect(callsOfMethod(gqlClient.GraphQLCalls, "UpdateOrgIPAllowListEntry")).To(BeEmpty())
			Expect(callsOfMethod(gqlClient.GraphQLCalls, "DeleteOrgIPAllowListEntry")).To(BeEmpty())
		})
	})

	Context("when an empty entries list is given", func() {
		BeforeEach(func() {
			current.Entries = []ghclient.IPAllowListEntry{
				{ID: "id-1", AllowListValue: "203.0.113.1", Name: "Existing", IsActive: true},
				{ID: "app-1", AllowListValue: "203.0.113.9", Name: "Managed by the Acme GitHub App.", IsActive: true},
			}
			ipSpec = &v1alpha1.IpAllowListSettings{Entries: []v1alpha1.IpAllowListEntry{}}
		})

		It("deletes organization-owned entries but preserves App-managed ones", func() {
			Expect(err).NotTo(HaveOccurred())
			deletes := callsOfMethod(gqlClient.GraphQLCalls, "DeleteOrgIPAllowListEntry")
			Expect(deletes).To(HaveLen(1))
			Expect(deletes[0].EntryID).To(Equal("id-1"))
		})
	})
})
