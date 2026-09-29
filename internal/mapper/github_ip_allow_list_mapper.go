package mapper

import (
	"strings"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/ghclient"
	"github.com/Interhyp/git-hubby/internal/utils"
)

// appManagedDescriptionPrefix is the prefix GitHub uses for the description of IP allow list
// entries that are automatically managed by an installed GitHub App
// ("Managed by the <name> GitHub App."). Such entries are read-only and must never be modified
// or deleted by git-hubby.
const appManagedDescriptionPrefix = "Managed by the "

// appManagedDescriptionSuffix together with the prefix identifies App-managed entries.
const appManagedDescriptionSuffix = " GitHub App."

// IsAppManagedIPAllowListEntry reports whether the given entry is automatically managed by an
// installed GitHub App and is therefore read-only. These entries are surfaced on the organization
// IP allow list but cannot be edited, deleted, or disabled by an organization owner.
func IsAppManagedIPAllowListEntry(entry ghclient.IPAllowListEntry) bool {
	name := entry.Name
	return strings.HasPrefix(name, appManagedDescriptionPrefix) && strings.HasSuffix(name, appManagedDescriptionSuffix)
}

// IPAllowListEntryCreate describes an organization-owned entry that must be created.
type IPAllowListEntryCreate struct {
	AllowListValue string
	Name           string
	IsActive       bool
}

// IPAllowListEntryUpdate describes an existing organization-owned entry that must be updated.
type IPAllowListEntryUpdate struct {
	EntryID        string
	AllowListValue string
	Name           string
	IsActive       bool
}

// IPAllowListEntryDelete describes an organization-owned entry that must be deleted.
type IPAllowListEntryDelete struct {
	EntryID        string
	AllowListValue string
}

// IPAllowListEntryPlan is the set of mutations required to bring the organization-owned portion of
// the IP allow list into the desired state. Read-only entries (App-managed and enterprise-inherited)
// are never included.
type IPAllowListEntryPlan struct {
	Create []IPAllowListEntryCreate
	Update []IPAllowListEntryUpdate
	Delete []IPAllowListEntryDelete
}

// IsEmpty reports whether the plan requires no mutations.
func (p IPAllowListEntryPlan) IsEmpty() bool {
	return len(p.Create) == 0 && len(p.Update) == 0 && len(p.Delete) == 0
}

// DiffIPAllowListEntries computes the create/update/delete plan for the organization-owned IP allow
// list entries, matching entries by their AllowListValue (CIDR). App-managed entries in the current
// state are ignored entirely so they are neither updated nor deleted.
//
// desired is the list from the Organization spec. current is the full list read from GitHub
// (which may include App-managed entries). Matching is keyed on the normalized allow list value.
func DiffIPAllowListEntries(desired []v1alpha1.IpAllowListEntry, current []ghclient.IPAllowListEntry) IPAllowListEntryPlan {
	plan := IPAllowListEntryPlan{}

	currentByValue := make(map[string]ghclient.IPAllowListEntry, len(current))
	for _, entry := range current {
		if IsAppManagedIPAllowListEntry(entry) {
			// Read-only: never manage App-managed entries.
			continue
		}
		currentByValue[entry.AllowListValue] = entry
	}

	desiredSeen := make(map[string]struct{}, len(desired))
	for _, want := range desired {
		value := want.AllowListValue
		desiredSeen[value] = struct{}{}
		wantActive := utils.WithDefault(want.IsActive, true)

		existing, found := currentByValue[value]
		if !found {
			plan.Create = append(plan.Create, IPAllowListEntryCreate{
				AllowListValue: value,
				Name:           want.Name,
				IsActive:       wantActive,
			})
			continue
		}
		if existing.Name != want.Name || existing.IsActive != wantActive {
			plan.Update = append(plan.Update, IPAllowListEntryUpdate{
				EntryID:        existing.ID,
				AllowListValue: value,
				Name:           want.Name,
				IsActive:       wantActive,
			})
		}
	}

	for value, existing := range currentByValue {
		if _, keep := desiredSeen[value]; !keep {
			plan.Delete = append(plan.Delete, IPAllowListEntryDelete{
				EntryID:        existing.ID,
				AllowListValue: value,
			})
		}
	}

	return plan
}

// IPAllowListEnabledSettingDiffers reports whether the desired enabled state differs from the
// current GitHub setting. A current value other than ENABLED is treated as "not enabled".
func IPAllowListEnabledSettingDiffers(desiredEnabled bool, current ghclient.IPAllowListEnabledSetting) bool {
	return desiredEnabled != (current == ghclient.IPAllowListEnabled)
}
