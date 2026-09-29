package ghclient

import (
	"context"
	"fmt"

	"github.com/shurcooL/githubv4"
)

// ipAllowListEntriesPageSize is the number of IP allow list entries fetched per GraphQL page.
const ipAllowListEntriesPageSize = 100

// ipAllowListEntryNode is the shared GraphQL selection for a single IP allow list entry.
type ipAllowListEntryNode struct {
	ID             githubv4.ID
	Name           githubv4.String
	AllowListValue githubv4.String
	IsActive       githubv4.Boolean
	CreatedAt      githubv4.DateTime
	UpdatedAt      githubv4.DateTime
}

func (n ipAllowListEntryNode) toDomain() IPAllowListEntry {
	return IPAllowListEntry{
		ID:             fmt.Sprintf("%v", n.ID),
		Name:           string(n.Name),
		AllowListValue: string(n.AllowListValue),
		IsActive:       bool(n.IsActive),
		CreatedAt:      n.CreatedAt.Time,
		UpdatedAt:      n.UpdatedAt.Time,
	}
}

// GetOrgIPAllowList reads the organization's IP allow list configuration, paginating over all entries.
func (g *GraphQLClientWrapper) GetOrgIPAllowList(ctx context.Context, orgLogin string) (*OrgIPAllowListConfig, error) {
	var query struct {
		Organization struct {
			ID                                        githubv4.ID
			IPAllowListEnabledSetting                 githubv4.String
			IPAllowListForInstalledAppsEnabledSetting githubv4.String
			IPAllowListEntries                        struct {
				Nodes    []ipAllowListEntryNode
				PageInfo struct {
					HasNextPage githubv4.Boolean
					EndCursor   githubv4.String
				}
			} `graphql:"ipAllowListEntries(first: $first, after: $after)"`
		} `graphql:"organization(login: $login)"`
	}

	variables := map[string]any{
		"login": githubv4.String(orgLogin),
		"first": githubv4.Int(ipAllowListEntriesPageSize),
		"after": (*githubv4.String)(nil),
	}

	config := &OrgIPAllowListConfig{}
	entries := make([]IPAllowListEntry, 0)
	for {
		if err := g.client.Query(ctx, &query, variables); err != nil {
			return nil, err
		}

		// Owner-level fields are identical on every page; capture them from the first response.
		config.OwnerID = fmt.Sprintf("%v", query.Organization.ID)
		config.EnabledSetting = IPAllowListEnabledSetting(query.Organization.IPAllowListEnabledSetting)
		config.InstalledAppsEnabledSetting = IPAllowListEnabledSetting(query.Organization.IPAllowListForInstalledAppsEnabledSetting)

		for _, node := range query.Organization.IPAllowListEntries.Nodes {
			entries = append(entries, node.toDomain())
		}

		if !bool(query.Organization.IPAllowListEntries.PageInfo.HasNextPage) {
			break
		}
		variables["after"] = new(query.Organization.IPAllowListEntries.PageInfo.EndCursor)
	}

	config.Entries = entries
	return config, nil
}

// CreateOrgIPAllowListEntry adds a new organization-owned entry.
func (g *GraphQLClientWrapper) CreateOrgIPAllowListEntry(ctx context.Context, ownerID string, value string, name string, isActive bool) error {
	var mutation struct {
		CreateIPAllowListEntry struct {
			IPAllowListEntry struct {
				ID githubv4.ID
			}
		} `graphql:"createIpAllowListEntry(input: $input)"`
	}
	input := githubv4.CreateIpAllowListEntryInput{
		OwnerID:        githubv4.ID(ownerID),
		AllowListValue: githubv4.String(value),
		IsActive:       githubv4.Boolean(isActive),
		Name:           new(githubv4.String(name)),
	}
	return g.client.Mutate(ctx, &mutation, input, nil)
}

// UpdateOrgIPAllowListEntry updates an existing entry by its GraphQL node ID.
func (g *GraphQLClientWrapper) UpdateOrgIPAllowListEntry(ctx context.Context, entryID string, value string, name string, isActive bool) error {
	var mutation struct {
		UpdateIPAllowListEntry struct {
			IPAllowListEntry struct {
				ID githubv4.ID
			}
		} `graphql:"updateIpAllowListEntry(input: $input)"`
	}
	input := githubv4.UpdateIpAllowListEntryInput{
		IPAllowListEntryID: githubv4.ID(entryID),
		AllowListValue:     githubv4.String(value),
		IsActive:           githubv4.Boolean(isActive),
		Name:               new(githubv4.String(name)),
	}
	return g.client.Mutate(ctx, &mutation, input, nil)
}

// DeleteOrgIPAllowListEntry deletes an entry by its GraphQL node ID.
func (g *GraphQLClientWrapper) DeleteOrgIPAllowListEntry(ctx context.Context, entryID string) error {
	var mutation struct {
		DeleteIPAllowListEntry struct {
			IPAllowListEntry struct {
				ID githubv4.ID
			}
		} `graphql:"deleteIpAllowListEntry(input: $input)"`
	}
	input := githubv4.DeleteIpAllowListEntryInput{
		IPAllowListEntryID: githubv4.ID(entryID),
	}
	return g.client.Mutate(ctx, &mutation, input, nil)
}

// SetOrgIPAllowListEnabled sets the enforcement setting for the organization.
func (g *GraphQLClientWrapper) SetOrgIPAllowListEnabled(ctx context.Context, ownerID string, enabled bool) error {
	var mutation struct {
		UpdateIPAllowListEnabledSetting struct {
			Owner struct {
				ID githubv4.ID `graphql:"id"`
			} `graphql:"owner"`
		} `graphql:"updateIpAllowListEnabledSetting(input: $input)"`
	}
	input := githubv4.UpdateIpAllowListEnabledSettingInput{
		OwnerID:      githubv4.ID(ownerID),
		SettingValue: boolToEnabledSetting(enabled),
	}
	return g.client.Mutate(ctx, &mutation, input, nil)
}

// SetOrgIPAllowListForInstalledAppsEnabled sets the "configure for installed GitHub Apps" setting.
func (g *GraphQLClientWrapper) SetOrgIPAllowListForInstalledAppsEnabled(ctx context.Context, ownerID string, enabled bool) error {
	var mutation struct {
		UpdateIPAllowListForInstalledAppsEnabledSetting struct {
			Owner struct {
				ID githubv4.ID `graphql:"id"`
			} `graphql:"owner"`
		} `graphql:"updateIpAllowListForInstalledAppsEnabledSetting(input: $input)"`
	}
	input := githubv4.UpdateIpAllowListForInstalledAppsEnabledSettingInput{
		OwnerID:      githubv4.ID(ownerID),
		SettingValue: boolToInstalledAppsSetting(enabled),
	}
	return g.client.Mutate(ctx, &mutation, input, nil)
}

func boolToEnabledSetting(enabled bool) githubv4.IpAllowListEnabledSettingValue {
	if enabled {
		return githubv4.IpAllowListEnabledSettingValueEnabled
	}
	return githubv4.IpAllowListEnabledSettingValueDisabled
}

func boolToInstalledAppsSetting(enabled bool) githubv4.IpAllowListForInstalledAppsEnabledSettingValue {
	if enabled {
		return githubv4.IpAllowListForInstalledAppsEnabledSettingValueEnabled
	}
	return githubv4.IpAllowListForInstalledAppsEnabledSettingValueDisabled
}
