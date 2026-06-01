package orgrec

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"time"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/ghclient"
	"github.com/Interhyp/git-hubby/internal/mapper"
	"github.com/google/go-github/v86/github"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

const targetTypeOrganization = "organization"

func (o *GitHubOrgReconciler) reconcileCodeSecurityConfigurations(ctx context.Context) error {
	// GitHub returns 409 on code-security configuration endpoints when an enablement
	// event is in progress. This is transient and should be retried at the transport level.
	ctx = ghclient.WithRetryableStatusCodes(ctx, http.StatusConflict)

	log := logPkg.FromContext(ctx)
	log.V(1).Info("Reconciling organization code security configurations on GitHub")

	if err := o.unsetObsoleteDefaults(ctx); err != nil {
		return err
	}

	ghOrgConfsByName, err := o.getOrgLevelCodeSecurityConfigurationsByName(ctx)
	if err != nil {
		return err
	}

	type attachable struct {
		k8sName  string
		scope    *string
		githubId int64
	}
	seen := make(map[string]attachable)
	for _, csc := range o.Kubernetes.Resource.Spec.CodeSecurityConfigurations {
		expectedCsc := &githubv1alpha1.CodeSecurityConfiguration{}
		if err := o.Kubernetes.Client.Get(ctx, client.ObjectKey{Name: csc.Name, Namespace: o.Kubernetes.Resource.Namespace}, expectedCsc); err != nil {
			log.Error(err, "failed to get code security configuration", "csc", csc.Name)
			return err
		}
		k8sName := expectedCsc.Name
		expectedCsc, err = o.resolveBypassReviewerNames(ctx, expectedCsc)
		if err != nil {
			return err
		}
		match, found := ghOrgConfsByName[k8sName]
		var currentCsc *github.CodeSecurityConfiguration
		if found {
			if currentCsc, err = o.updateCsc(ctx, expectedCsc, &match); err != nil {
				return err
			}
		} else {
			if currentCsc, err = o.createCsc(ctx, expectedCsc); err != nil {
				return err
			}
		}
		seen[k8sName] = attachable{
			k8sName:  k8sName,
			scope:    csc.AttachmentScope,
			githubId: currentCsc.GetID(),
		}
	}
	for ghName, ghConf := range ghOrgConfsByName {
		if _, found := seen[ghName]; !found {
			if err = o.deleteCsc(ctx, ghConf); err != nil {
				return err
			}
		}
	}
	for _, candidate := range seen {
		if candidate.scope != nil {
			// attaching must happen last as this is an async process that will block other operations on the csc
			err = o.attachCSC(ctx, *candidate.scope, candidate.k8sName, candidate.githubId)
			if err != nil {
				return err
			}
		}
	}

	log.V(1).Info("Successfully reconciled organization code security configurations on GitHub")
	return nil
}

// unsetObsoleteDefaults unsets code security configurations as default in GitHub that are no longer marked as default in the Kubernetes resource.
func (o *GitHubOrgReconciler) unsetObsoleteDefaults(ctx context.Context) error {
	defaultK8sConfs := make(map[string]bool)

	// Fetch CodeSecurityConfiguration objects referenced by this organization
	for _, ref := range o.Kubernetes.Resource.Spec.CodeSecurityConfigurations {
		csc := &githubv1alpha1.CodeSecurityConfiguration{}
		if err := o.Kubernetes.Client.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: o.Kubernetes.Resource.Namespace}, csc); err != nil {
			log := logPkg.FromContext(ctx)
			log.Error(err, "failed to get code security configuration", "csc", ref.Name)
			return err
		}
		if csc != nil && csc.Spec.DefaultForNewRepos != nil && *csc.Spec.DefaultForNewRepos != "none" {
			defaultK8sConfs[csc.Spec.Name] = true
		}
	}

	defaultGhConfs, err := o.GitHub.Client.GetDefaultCodeSecurityConfigurationsForOrg(ctx, o.GitHub.Resource)
	if err != nil {
		return err
	}
	defaultGhOrgConfsByName := byName(defaultGhConfs,
		func(c github.CodeSecurityConfigurationWithDefaultForNewRepos) string {
			return c.GetConfiguration().Name
		},
		func(c github.CodeSecurityConfigurationWithDefaultForNewRepos) bool {
			return c.GetConfiguration().GetTargetType() != targetTypeOrganization
		},
	)

	for ghName, ghConf := range defaultGhOrgConfsByName {
		if _, isStillDefault := defaultK8sConfs[ghName]; !isStillDefault {
			err = o.GitHub.Client.SetCodeSecurityConfigurationAsDefaultForOrg(ctx, o.GitHub.Resource, ghConf.Configuration.GetID(), "none")
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// getOrgLevelCodeSecurityConfigurationsByName fetches all code security configurations for the organization from GitHub and returns them as a map keyed by name.
func (o *GitHubOrgReconciler) getOrgLevelCodeSecurityConfigurationsByName(ctx context.Context) (map[string]github.CodeSecurityConfiguration, error) {
	ghConfs, err := o.GitHub.Client.GetCodeSecurityConfigurationsForOrg(ctx, o.GitHub.Resource)
	if err != nil {
		return nil, err
	}

	ghOrgConfsByName := byName(ghConfs,
		func(c github.CodeSecurityConfiguration) string { return c.Name },
		func(c github.CodeSecurityConfiguration) bool { return c.GetTargetType() != targetTypeOrganization },
	)
	return ghOrgConfsByName, nil
}

// resolveBypassReviewerNames resolves the ReviewerName fields in the BypassReviewers to ReviewerId fields with respect
// to their type (TEAM or ROLE).
func (o *GitHubOrgReconciler) resolveBypassReviewerNames(ctx context.Context, csc *githubv1alpha1.CodeSecurityConfiguration) (*githubv1alpha1.CodeSecurityConfiguration, error) {
	if csc.Spec.SecretScanningDelegatedBypassOptions != nil {
		updated := make([]*githubv1alpha1.BypassReviewer, len(csc.Spec.SecretScanningDelegatedBypassOptions.Reviewers))
		for i, reviewer := range csc.Spec.SecretScanningDelegatedBypassOptions.Reviewers {
			if reviewer.ReviewerName != nil {
				switch reviewer.ReviewerType {
				case "TEAM":
					team, err := o.GitHub.Client.GetTeamBySlug(ctx, o.GitHub.Resource, *reviewer.ReviewerName)
					if err != nil {
						return nil, err
					}
					id := team.GetID()
					reviewer.ReviewerId = &id
				case "ROLE":
					role, err := o.GitHub.Client.GetRoleByName(ctx, o.GitHub.Resource, *reviewer.ReviewerName)
					if err != nil {
						return nil, err
					}
					id := role.GetID()
					reviewer.ReviewerId = &id
				}
			}
			updated[i] = reviewer
		}
		csc.Spec.SecretScanningDelegatedBypassOptions.Reviewers = updated
	}
	return csc, nil
}

func (o *GitHubOrgReconciler) updateCsc(ctx context.Context, desired *githubv1alpha1.CodeSecurityConfiguration, current *github.CodeSecurityConfiguration) (*github.CodeSecurityConfiguration, error) {
	log := logPkg.FromContext(ctx).WithValues("CodeSecurityConfiguration", current.Name)
	log.V(1).Info("Updating code security configurations on GitHub")

	desiredAsGH := mapper.ToGithubCodeSecurityConfiguration(desired)
	cscID := current.GetID()
	if mapper.CodeSecurityConfigurationsDiffer(&desiredAsGH, current) {
		current, err := o.GitHub.Client.UpdateCodeSecurityConfigurationForOrg(ctx, o.GitHub.Resource, cscID, desiredAsGH)
		if err != nil {
			log.Error(err, "Failed to update code security configuration on GitHub")
			return nil, err
		}
		cscID = current.GetID()
		if desired.Spec.DefaultForNewRepos != nil {
			err = o.GitHub.Client.SetCodeSecurityConfigurationAsDefaultForOrg(ctx, o.GitHub.Resource, cscID, *desired.Spec.DefaultForNewRepos)
			if err != nil {
				log.Error(err, "Failed to set code security configuration as default on GitHub")
				return nil, err
			}
		}
	}

	log.V(1).Info("Updated code security configurations on GitHub")
	return current, nil
}

// attachCSC attaches the code security configuration with the given cscGitHubID to repositories according to the specified scope if the
// list of currently attached repositories differs from the expectation. the expectation is based on the given scope.
// If the scope is "all_without_configurations", it attaches the code security configuration to all repositories without comparing the current attachments.
// If the scope is "selected", it expects exactly all repositories that are explicitly referencing it in Kubernetes by the v1alpha1.RepositorySpec field AttachedCodeSecurityConfiguration.
// If the scope is "all", it expects all repositories of the organization to be attached.
// If the scope is "public", it expects all public repositories of the organization to be attached.
// If the scope is "private_or_internal", it expects all private and internal repositories of the organization to be attached.
// We only count repositories as attached if their attachment status is "attached", "removed_by_enterprise" or "enforced".
// If any repository has a failed attachment status like "detached", "removed" or "failed", we consider the attachments as not matching the expectation and try to re-attach according to the scope.
func (o *GitHubOrgReconciler) attachCSC(ctx context.Context, attachmentScope string, cscK8sName string, cscGitHubID int64) error {
	log := logPkg.FromContext(ctx).WithValues("CodeSecurityConfiguration", cscK8sName)
	if attachmentScope == "all_without_configurations" {
		return o.attachToRepos(ctx, attachmentScope, cscGitHubID, nil)
	}
	currentAttachments, err := o.getAndWaitForFinishedAttachments(ctx, cscGitHubID, 5*time.Second, 1*time.Minute)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to get repositories attached to code security configuration %s", cscK8sName))
		return err
	}
	desiredAttachments, err := o.getDesiredAttachmentsForScope(ctx, attachmentScope, cscK8sName)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to get repositories attached to code security configuration %s", cscK8sName))
		return err
	}
	if attachmentsDiffer(currentAttachments, desiredAttachments) {
		var ids []int64
		//nolint:goconst
		if attachmentScope == "selected" {
			ids = slices.Collect(maps.Values(desiredAttachments))
		}
		if err := o.attachToRepos(ctx, attachmentScope, cscGitHubID, ids); err != nil {
			log.Error(err, "Failed to attach code security configuration to repositories with scope selected")
			return err
		}
	}
	// handle attachment scope?

	return nil
}

func (o *GitHubOrgReconciler) getDesiredAttachmentsForScope(ctx context.Context, attachmentScope string, cscK8sName string) (map[string]int64, error) {
	var desiredAttachments map[string]int64
	switch attachmentScope {
	//nolint:goconst
	case "selected":
		return o.getSelectedAttachedRepoNamesToIDs(ctx, cscK8sName)
	default:
		repos, err := o.GitHub.Client.GetOrgRepositories(ctx, o.GitHub.Resource)
		if err != nil {
			return nil, err
		}
		desiredAttachments = make(map[string]int64)
		for _, repo := range repos {
			if repo.ID == nil || repo.Name == nil {
				continue
			}
			doInclude := false
			switch attachmentScope {
			case "public":
				if repo.GetVisibility() == "public" {
					doInclude = true
				}
			case "private_or_internal":
				if repo.GetVisibility() == "private" || repo.GetVisibility() == "internal" {
					doInclude = true
				}
			case "all":
				doInclude = true
			}
			if doInclude {
				desiredAttachments[*repo.Name] = *repo.ID
			}
		}

	}
	return desiredAttachments, nil
}

func (o *GitHubOrgReconciler) getAndWaitForFinishedAttachments(ctx context.Context, cscGitHubID int64, queryInterval, timeoutAfter time.Duration) ([]*github.RepositoryAttachment, error) {
	attachments, err := o.getAttachmentsIfAllAttached(ctx, cscGitHubID, nil)
	if err != nil {
		return nil, err
	}
	if attachments != nil {
		return attachments, nil
	}

	// not all attachments are finished, start waiting and polling
	ticker := time.NewTicker(queryInterval)
	defer ticker.Stop()
	timeout := time.After(timeoutAfter)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for code security configuration attachment to complete")
		case <-ticker.C:
			attachments, err = o.getAttachmentsIfAllAttached(ctx, cscGitHubID, &GetAttachmentsOpts{returnErrorOnFailedStatus: true})
			if err != nil {
				return nil, err
			}
			if attachments != nil {
				return attachments, nil
			}
		}
	}
}

type GetAttachmentsOpts struct {
	returnErrorOnFailedStatus bool
}

// getAttachmentsIfAllAttached returns the RepositoryAttachments for a Code Security Configuration with the given ID,
// but only if all of them are finished, i.e. the attachment status is "attached", "removed_by_enterprise", "enforced".
// otherwise it returns nil, which can be used to indicate that the attachments are still in progress.
// If any attachment has a failed status like "detached", "removed" or "failed", it returns an error.
// The attachment states can be found in the response schema at
// https://docs.github.com/en/enterprise-cloud@latest/rest/code-security/configurations?apiVersion=2022-11-28#get-repositories-associated-with-a-code-security-configuration
func (o *GitHubOrgReconciler) getAttachmentsIfAllAttached(ctx context.Context, cscGitHubID int64, opts *GetAttachmentsOpts) ([]*github.RepositoryAttachment, error) {
	options := GetAttachmentsOpts{}
	if opts != nil {
		options = *opts
	}
	attachedRepos, err := o.GitHub.Client.GetRepositoriesAttachedToCodeSecurityConfiguration(ctx, o.GitHub.Resource, cscGitHubID)
	if err != nil {
		return nil, err
	}
	allAttached := true
	for _, attachment := range attachedRepos {
		status := attachment.GetStatus()
		if status == "attaching" || status == "updating" {
			allAttached = false
			break
		} else if status == "detached" || status == "removed" || status == "failed" {
			if options.returnErrorOnFailedStatus {
				return nil, fmt.Errorf("encountered failed attachment status: %s", attachment.GetStatus())
			}
			allAttached = false
			break
		}
	}
	if allAttached {
		return attachedRepos, nil
	}
	return nil, nil
}

func (o *GitHubOrgReconciler) attachToRepos(ctx context.Context, attachmentScope string, cscGitHubID int64, repoIds []int64) error {
	log := logPkg.FromContext(ctx)
	err := o.GitHub.Client.AttachCodeSecurityConfigurations(ctx, o.GitHub.Resource, cscGitHubID, attachmentScope, repoIds)
	if err != nil {
		var acceptedErr *github.AcceptedError
		if errors.As(err, &acceptedErr) {
			// Attachment was accepted and is processing asynchronously - wait for it to finish
			log.V(1).Info("Attachment accepted, waiting for completion", "scope", attachmentScope)
			if _, waitErr := o.getAndWaitForFinishedAttachments(ctx, cscGitHubID, 5*time.Second, 2*time.Minute); waitErr != nil {
				log.Error(waitErr, "Attaching code security configuration did not finish in time", "scope", attachmentScope)
				return waitErr
			}
			return nil
		}
		log.Error(err, "Failed to attach code security configuration to repositories", "scope", attachmentScope)
		return err
	}
	return nil
}

func (o *GitHubOrgReconciler) getSelectedAttachedRepoNamesToIDs(ctx context.Context, cscK8sName string) (map[string]int64, error) {
	log := logPkg.FromContext(ctx)
	desiredAttachments := make(map[string]int64)
	repoList := &githubv1alpha1.RepositoryList{}
	err := o.Kubernetes.Client.List(ctx, repoList,
		client.InNamespace(o.Kubernetes.Resource.Namespace),
		client.MatchingFields{
			"spec.attachedCodeSecurityConfiguration.name": cscK8sName,
		},
	)
	if err != nil {
		log.Error(err, "Failed to list repositories referencing code security configuration in Kubernetes")
		return nil, err
	}
	for _, repo := range repoList.Items {
		id := repo.Status.ID
		if id == nil {
			ghRepo, err := o.GitHub.Client.GetRepository(ctx, o.GitHub.Resource, repo.Spec.Name)
			if err != nil {
				log.Error(err, fmt.Sprintf("Failed to get repository %s from GitHub to resolve its ID for code security configuration attachment", repo.Spec.Name))
				return nil, err
			}
			if ghRepo.ID == nil {
				log.Error(err, fmt.Sprintf("Repository %s has no ID in GitHub, cannot attach code security configuration", repo.Spec.Name))
				return nil, fmt.Errorf("repository %s has no ID in GitHub, cannot attach code security configuration", repo.Spec.Name)
			}
			id = ghRepo.ID
		}
		desiredAttachments[repo.GetName()] = *id
	}
	return desiredAttachments, nil
}

func attachmentsDiffer(currentAttachments []*github.RepositoryAttachment, desiredAttachments map[string]int64) bool {
	if len(currentAttachments) != len(desiredAttachments) {
		return true
	}
	for _, current := range currentAttachments {
		// same size, so equality is ensured if one collection is wholely contained in the other
		_, found := desiredAttachments[current.GetRepository().GetName()]
		if !found {
			// one not found = not fully contained -> different
			return true
		}
	}
	return false
}

func (o *GitHubOrgReconciler) createCsc(ctx context.Context, conf *githubv1alpha1.CodeSecurityConfiguration) (*github.CodeSecurityConfiguration, error) {
	log := logPkg.FromContext(ctx).WithValues("CodeSecurityConfiguration", conf.Name)
	log.V(1).Info("Creating code security configurations on GitHub")

	ghConf := mapper.ToGithubCodeSecurityConfiguration(conf)
	result, err := o.GitHub.Client.CreateCodeSecurityConfigurationForOrg(ctx, o.GitHub.Resource, ghConf)
	if err != nil {
		log.Error(err, "Failed to create code security configuration on GitHub")
		return nil, err
	}
	if conf.Spec.DefaultForNewRepos != nil {
		err = o.GitHub.Client.SetCodeSecurityConfigurationAsDefaultForOrg(ctx, o.GitHub.Resource, result.GetID(), *conf.Spec.DefaultForNewRepos)
		if err != nil {
			log.Error(err, "Failed to set code security configuration as default on GitHub")
			return nil, err
		}
	}
	log.V(1).Info("Created code security configurations on GitHub")
	return result, nil
}

func (o *GitHubOrgReconciler) deleteCsc(ctx context.Context, conf github.CodeSecurityConfiguration) error {
	log := logPkg.FromContext(ctx).WithValues("CodeSecurityConfiguration", conf.Name)
	log.V(1).Info("Deleting code security configurations on GitHub")

	if err := o.GitHub.Client.DeleteCodeSecurityConfigurationForOrg(ctx, o.GitHub.Resource, conf.GetID()); err != nil {
		log.Error(err, "Failed to delete code security configuration on GitHub")
		return err
	}
	return nil
}

// byName creates a map from name to item for the given list, applying optional filters to exclude items.
// If at least one filter returns true the item is not added to the resulting map.
func byName[T any](list []*T, getName func(T) string, filters ...func(T) bool) map[string]T {
	result := make(map[string]T)
	for _, item := range list {
		if item != nil {
			doAdd := true
			for _, filter := range filters {
				if filter(*item) {
					doAdd = false
				}
			}
			if doAdd {
				result[getName(*item)] = *item
			}
		}
	}
	return result
}
