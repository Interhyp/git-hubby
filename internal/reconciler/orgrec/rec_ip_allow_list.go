package orgrec

import (
	"context"
	"strings"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/ghclient"
	"github.com/Interhyp/git-hubby/internal/mapper"
	"github.com/Interhyp/git-hubby/internal/utils"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

// ipAllowListUnavailableMarkers are lowercase substrings that identify GitHub GraphQL errors
// indicating that organization IP allow list management is unavailable for this owner. This
// happens on non-Enterprise-Cloud organizations and when the enterprise delegates its allow list
// to an identity provider (Enterprise Managed Users with Entra ID and OIDC), which deactivates the
// organization IP allow list GraphQL APIs. In those cases the feature genuinely cannot be managed,
// so the reconciler surfaces the situation and stops without treating it as a transient failure.
var ipAllowListUnavailableMarkers = []string{
	"ip allow list is not available",
	"ip allow list is disabled",
	"ip allow list management",
	"not available for this",
	"ip allow lists are not available",
}

// reconcileIpAllowList reconciles the organization-level IP allow list (settings and
// organization-owned entries) via the GitHub GraphQL API.
//
// It is a no-op when the spec does not configure an IP allow list (spec.IpAllowList == nil),
// leaving all existing settings and entries untouched.
//
// Only the organization-owned portion of the list is managed. Entries inherited from the
// enterprise account are not returned by the organization GraphQL connection and are therefore
// never touched. Entries automatically managed by installed GitHub Apps are read-only and are
// filtered out of the diff.
//
// Entries are always reconciled before the enabled setting, so enabling enforcement can never
// lock out the currently configured (active) addresses.
func (o *GitHubOrgReconciler) reconcileIpAllowList(ctx context.Context) error {
	log := logPkg.FromContext(ctx)

	spec := o.Kubernetes.Resource.Spec.IpAllowList
	if spec == nil {
		log.V(1).Info("No IP allow list configuration in spec, skipping reconciliation")
		return nil
	}

	log.V(1).Info("Reconciling organization IP allow list on GitHub")

	current, err := o.GitHub.GraphQLClient.GetOrgIPAllowList(ctx, o.GitHub.Resource)
	if err != nil {
		if isIpAllowListUnavailable(err) {
			// Terminal, non-transient: the feature cannot be managed for this owner. Log and
			// stop without returning an error so the reconciliation is not retried in a loop.
			log.Info("IP allow list management is unavailable for this organization, skipping reconciliation. "+
				"This is expected on non-Enterprise-Cloud orgs or when the enterprise delegates its allow list to an identity provider",
				"organization", o.GitHub.Resource, "reason", err.Error())
			return nil
		}
		return err
	}

	// 1. Reconcile entries first (create/update/delete) so enforcement never precedes the addresses.
	if spec.Entries != nil {
		if err := o.reconcileIpAllowListEntries(ctx, current, spec.Entries); err != nil {
			return err
		}
	}

	// 2. Reconcile the "configure for installed GitHub Apps" setting.
	desiredInstalledApps := utils.WithDefault(spec.EnabledForInstalledApps, false)
	if mapper.IPAllowListEnabledSettingDiffers(desiredInstalledApps, current.InstalledAppsEnabledSetting) {
		if err := o.GitHub.GraphQLClient.SetOrgIPAllowListForInstalledAppsEnabled(ctx, current.OwnerID, desiredInstalledApps); err != nil {
			return err
		}
	}

	// 3. Reconcile the enforcement setting last.
	desiredEnabled := utils.WithDefault(spec.Enabled, false)
	if mapper.IPAllowListEnabledSettingDiffers(desiredEnabled, current.EnabledSetting) {
		if err := o.GitHub.GraphQLClient.SetOrgIPAllowListEnabled(ctx, current.OwnerID, desiredEnabled); err != nil {
			return err
		}
	}

	log.V(1).Info("Successfully reconciled organization IP allow list on GitHub")
	return nil
}

// reconcileIpAllowListEntries applies the create/update/delete plan for organization-owned entries.
// Enterprise-inherited entries are never returned by the organization GraphQL connection, and
// App-managed entries are filtered out by the mapper, so only organization-owned entries are
// affected.
func (o *GitHubOrgReconciler) reconcileIpAllowListEntries(ctx context.Context, current *ghclient.OrgIPAllowListConfig, desired []v1alpha1.IpAllowListEntry) error {
	log := logPkg.FromContext(ctx)
	plan := mapper.DiffIPAllowListEntries(desired, current.Entries)
	if plan.IsEmpty() {
		return nil
	}

	// Create and update desired entries before deleting obsolete ones, so that a currently active
	// address is never briefly absent from the list while enforcement is (or may be) enabled.
	for _, create := range plan.Create {
		log.V(1).Info("Creating IP allow list entry", "value", create.AllowListValue, "isActive", create.IsActive)
		if err := o.GitHub.GraphQLClient.CreateOrgIPAllowListEntry(ctx, current.OwnerID, create.AllowListValue, create.Name, create.IsActive); err != nil {
			return err
		}
	}
	for _, update := range plan.Update {
		log.V(1).Info("Updating IP allow list entry", "value", update.AllowListValue, "isActive", update.IsActive)
		if err := o.GitHub.GraphQLClient.UpdateOrgIPAllowListEntry(ctx, update.EntryID, update.AllowListValue, update.Name, update.IsActive); err != nil {
			return err
		}
	}
	for _, del := range plan.Delete {
		log.V(1).Info("Deleting IP allow list entry", "value", del.AllowListValue)
		if err := o.GitHub.GraphQLClient.DeleteOrgIPAllowListEntry(ctx, del.EntryID); err != nil {
			return err
		}
	}

	return nil
}

// isIpAllowListUnavailable reports whether the given error indicates the IP allow list feature is
// unavailable for the organization (as opposed to a transient error worth retrying).
func isIpAllowListUnavailable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range ipAllowListUnavailableMarkers {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}
