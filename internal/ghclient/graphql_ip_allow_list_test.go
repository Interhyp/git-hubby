package ghclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/shurcooL/githubv4"
)

var _ = Describe("GraphQLClientWrapper IP allow list", func() {
	var server *httptest.Server

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
	})

	// newWrapper wires a GraphQLClientWrapper to a test server using the given handler.
	newWrapper := func(handler http.HandlerFunc) *GraphQLClientWrapper {
		server = httptest.NewServer(handler)
		v4 := githubv4.NewEnterpriseClient(server.URL, server.Client())
		return NewGraphQLClientWrapper(v4)
	}

	Describe("GetOrgIPAllowList", func() {
		It("reads settings and entries, paginating over all pages", func() {
			page := 0
			client := newWrapper(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if page == 0 {
					page++
					_, _ = w.Write([]byte(`{"data":{"organization":{
						"id":"org-1",
						"ipAllowListEnabledSetting":"ENABLED",
						"ipAllowListForInstalledAppsEnabledSetting":"DISABLED",
						"ipAllowListEntries":{
							"nodes":[{"id":"e1","name":"Office","allowListValue":"192.0.2.0/24","isActive":true,"createdAt":"2024-01-01T00:00:00Z","updatedAt":"2024-01-01T00:00:00Z"}],
							"pageInfo":{"hasNextPage":true,"endCursor":"CURSOR1"}
						}
					}}}`))
					return
				}
				_, _ = w.Write([]byte(`{"data":{"organization":{
					"id":"org-1",
					"ipAllowListEnabledSetting":"ENABLED",
					"ipAllowListForInstalledAppsEnabledSetting":"DISABLED",
					"ipAllowListEntries":{
						"nodes":[{"id":"e2","name":"VPN","allowListValue":"203.0.113.0/24","isActive":false,"createdAt":"2024-01-02T00:00:00Z","updatedAt":"2024-01-02T00:00:00Z"}],
						"pageInfo":{"hasNextPage":false,"endCursor":"CURSOR2"}
					}
				}}}`))
			})

			cfg, err := client.GetOrgIPAllowList(context.Background(), "my-org")
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.OwnerID).To(Equal("org-1"))
			Expect(cfg.EnabledSetting).To(Equal(IPAllowListEnabled))
			Expect(cfg.InstalledAppsEnabledSetting).To(Equal(IPAllowListDisabled))
			Expect(cfg.Entries).To(HaveLen(2))
			Expect(cfg.Entries[0].ID).To(Equal("e1"))
			Expect(cfg.Entries[0].AllowListValue).To(Equal("192.0.2.0/24"))
			Expect(cfg.Entries[0].IsActive).To(BeTrue())
			Expect(cfg.Entries[1].ID).To(Equal("e2"))
			Expect(cfg.Entries[1].IsActive).To(BeFalse())
		})

		It("propagates GraphQL errors", func() {
			client := newWrapper(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"errors":[{"message":"IP allow list is not available for this organization"}]}`))
			})
			_, err := client.GetOrgIPAllowList(context.Background(), "my-org")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not available"))
		})
	})

	Describe("mutations", func() {
		It("CreateOrgIPAllowListEntry sends the expected input", func() {
			var body map[string]any
			client := newWrapper(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(raw, &body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"createIpAllowListEntry":{"ipAllowListEntry":{"id":"new-1"}}}}`))
			})
			err := client.CreateOrgIPAllowListEntry(context.Background(), "org-1", "192.0.2.1", "Office", true)
			Expect(err).NotTo(HaveOccurred())
			vars, _ := body["variables"].(map[string]any)
			input, _ := vars["input"].(map[string]any)
			Expect(input["ownerId"]).To(Equal("org-1"))
			Expect(input["allowListValue"]).To(Equal("192.0.2.1"))
			Expect(input["isActive"]).To(BeTrue())
			Expect(input["name"]).To(Equal("Office"))
		})

		It("SetOrgIPAllowListEnabled maps true to ENABLED", func() {
			var body map[string]any
			client := newWrapper(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(raw, &body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"updateIpAllowListEnabledSetting":{"owner":{"id":"org-1"}}}}`))
			})
			err := client.SetOrgIPAllowListEnabled(context.Background(), "org-1", true)
			Expect(err).NotTo(HaveOccurred())
			vars, _ := body["variables"].(map[string]any)
			input, _ := vars["input"].(map[string]any)
			Expect(input["settingValue"]).To(Equal("ENABLED"))
		})

		It("SetOrgIPAllowListForInstalledAppsEnabled maps false to DISABLED", func() {
			var body map[string]any
			client := newWrapper(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(raw, &body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"updateIpAllowListForInstalledAppsEnabledSetting":{"owner":{"id":"org-1"}}}}`))
			})
			err := client.SetOrgIPAllowListForInstalledAppsEnabled(context.Background(), "org-1", false)
			Expect(err).NotTo(HaveOccurred())
			vars, _ := body["variables"].(map[string]any)
			input, _ := vars["input"].(map[string]any)
			Expect(input["settingValue"]).To(Equal("DISABLED"))
		})

		It("DeleteOrgIPAllowListEntry sends the entry id", func() {
			var body map[string]any
			client := newWrapper(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(raw, &body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"deleteIpAllowListEntry":{"ipAllowListEntry":{"id":"e1"}}}}`))
			})
			err := client.DeleteOrgIPAllowListEntry(context.Background(), "e1")
			Expect(err).NotTo(HaveOccurred())
			vars, _ := body["variables"].(map[string]any)
			input, _ := vars["input"].(map[string]any)
			Expect(input["ipAllowListEntryId"]).To(Equal("e1"))
		})
	})
})
