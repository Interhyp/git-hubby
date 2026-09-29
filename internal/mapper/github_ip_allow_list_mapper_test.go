package mapper

import (
	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/ghclient"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GitHub IP Allow List Mapper", func() {
	Describe("IsAppManagedIPAllowListEntry", func() {
		It("recognizes App-managed entries by their description", func() {
			Expect(IsAppManagedIPAllowListEntry(ghclient.IPAllowListEntry{
				Name: "Managed by the Acme GitHub App.",
			})).To(BeTrue())
		})

		It("does not flag organization-owned entries", func() {
			Expect(IsAppManagedIPAllowListEntry(ghclient.IPAllowListEntry{
				Name: "Office network",
			})).To(BeFalse())
			Expect(IsAppManagedIPAllowListEntry(ghclient.IPAllowListEntry{
				Name: "",
			})).To(BeFalse())
		})
	})

	Describe("IPAllowListEnabledSettingDiffers", func() {
		It("treats ENABLED as enabled", func() {
			Expect(IPAllowListEnabledSettingDiffers(true, ghclient.IPAllowListEnabled)).To(BeFalse())
			Expect(IPAllowListEnabledSettingDiffers(false, ghclient.IPAllowListEnabled)).To(BeTrue())
		})

		It("treats DISABLED and unknown values as not enabled", func() {
			Expect(IPAllowListEnabledSettingDiffers(false, ghclient.IPAllowListDisabled)).To(BeFalse())
			Expect(IPAllowListEnabledSettingDiffers(true, ghclient.IPAllowListDisabled)).To(BeTrue())
			Expect(IPAllowListEnabledSettingDiffers(false, ghclient.IPAllowListEnabledSetting("UNKNOWN"))).To(BeFalse())
		})
	})

	Describe("DiffIPAllowListEntries", func() {
		It("creates entries that are desired but absent", func() {
			desired := []v1alpha1.IpAllowListEntry{
				{AllowListValue: "192.0.2.0/24", Name: "Office", IsActive: new(true)},
			}
			plan := DiffIPAllowListEntries(desired, nil)

			Expect(plan.Update).To(BeEmpty())
			Expect(plan.Delete).To(BeEmpty())
			Expect(plan.Create).To(HaveLen(1))
			Expect(plan.Create[0].AllowListValue).To(Equal("192.0.2.0/24"))
			Expect(plan.Create[0].Name).To(Equal("Office"))
			Expect(plan.Create[0].IsActive).To(BeTrue())
		})

		It("defaults IsActive to true when unset", func() {
			desired := []v1alpha1.IpAllowListEntry{
				{AllowListValue: "192.0.2.1"},
			}
			plan := DiffIPAllowListEntries(desired, nil)
			Expect(plan.Create).To(HaveLen(1))
			Expect(plan.Create[0].IsActive).To(BeTrue())
		})

		It("updates entries whose name or active state changed", func() {
			desired := []v1alpha1.IpAllowListEntry{
				{AllowListValue: "192.0.2.0/24", Name: "New name", IsActive: new(false)},
			}
			current := []ghclient.IPAllowListEntry{
				{ID: "id-1", AllowListValue: "192.0.2.0/24", Name: "Old name", IsActive: true},
			}
			plan := DiffIPAllowListEntries(desired, current)

			Expect(plan.Create).To(BeEmpty())
			Expect(plan.Delete).To(BeEmpty())
			Expect(plan.Update).To(HaveLen(1))
			Expect(plan.Update[0].EntryID).To(Equal("id-1"))
			Expect(plan.Update[0].Name).To(Equal("New name"))
			Expect(plan.Update[0].IsActive).To(BeFalse())
		})

		It("does not update entries that already match", func() {
			desired := []v1alpha1.IpAllowListEntry{
				{AllowListValue: "192.0.2.0/24", Name: "Office", IsActive: new(true)},
			}
			current := []ghclient.IPAllowListEntry{
				{ID: "id-1", AllowListValue: "192.0.2.0/24", Name: "Office", IsActive: true},
			}
			plan := DiffIPAllowListEntries(desired, current)
			Expect(plan.IsEmpty()).To(BeTrue())
		})

		It("deletes organization-owned entries that are no longer desired", func() {
			current := []ghclient.IPAllowListEntry{
				{ID: "id-1", AllowListValue: "192.0.2.0/24", Name: "Office", IsActive: true},
			}
			plan := DiffIPAllowListEntries(nil, current)

			Expect(plan.Create).To(BeEmpty())
			Expect(plan.Update).To(BeEmpty())
			Expect(plan.Delete).To(HaveLen(1))
			Expect(plan.Delete[0].EntryID).To(Equal("id-1"))
		})

		It("never updates or deletes App-managed entries", func() {
			desired := []v1alpha1.IpAllowListEntry{
				{AllowListValue: "192.0.2.0/24", Name: "Office", IsActive: new(true)},
			}
			current := []ghclient.IPAllowListEntry{
				{ID: "app-1", AllowListValue: "203.0.113.5", Name: "Managed by the Acme GitHub App.", IsActive: true},
				{ID: "app-2", AllowListValue: "192.0.2.0/24", Name: "Managed by the Beta GitHub App.", IsActive: true},
			}
			plan := DiffIPAllowListEntries(desired, current)

			// The App-managed entry sharing the desired value must NOT be treated as the current
			// entry: it is ignored, so the desired value is created fresh.
			Expect(plan.Delete).To(BeEmpty())
			Expect(plan.Update).To(BeEmpty())
			Expect(plan.Create).To(HaveLen(1))
			Expect(plan.Create[0].AllowListValue).To(Equal("192.0.2.0/24"))
		})
	})
})
