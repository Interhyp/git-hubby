package mapper

import (
	"slices"
	"strings"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/go-github/v86/github"
)

func EqualActionsPermissions(configuration *github.ActionsPermissions, current *github.ActionsPermissions) bool {
	if configuration == nil && current == nil {
		return true
	}
	if configuration == nil || current == nil {
		return false
	}
	return cmp.Equal(configuration.SHAPinningRequired, current.SHAPinningRequired) &&
		cmp.Equal(configuration.AllowedActions, current.AllowedActions) &&
		cmp.Equal(configuration.EnabledRepositories, current.EnabledRepositories)
}

func EqualActionsAllowed(actions *github.ActionsAllowed, current *github.ActionsAllowed) bool {
	if actions == nil && current == nil {
		return true
	}
	if actions == nil || current == nil {
		return false
	}
	return cmp.Equal(actions.GithubOwnedAllowed, current.GithubOwnedAllowed) &&
		cmp.Equal(actions.VerifiedAllowed, current.VerifiedAllowed) &&
		cmp.Equal(actions.PatternsAllowed, current.PatternsAllowed, cmpopts.SortSlices(strings.Compare))
}

func EqualDefaultWorkflowPermissions(expected *github.DefaultWorkflowPermissionOrganization, current *github.DefaultWorkflowPermissionOrganization) bool {
	if expected == nil && current == nil {
		return true
	}
	if expected == nil || current == nil {
		return false
	}
	return cmp.Equal(expected.DefaultWorkflowPermissions, current.DefaultWorkflowPermissions) &&
		cmp.Equal(expected.CanApprovePullRequestReviews, current.CanApprovePullRequestReviews)
}

func EqualRunnerGroup(k8sGroup v1alpha1.RunnerGroup, ghGroup *github.RunnerGroup) bool {
	equalRestrictedToWorkflows := cmp.Equal(k8sGroup.RestrictedToWorkflows, ghGroup.RestrictedToWorkflows)
	equalSelectedWorkflows := true
	if equalRestrictedToWorkflows && k8sGroup.RestrictedToWorkflows != nil && *k8sGroup.RestrictedToWorkflows {
		// only compare selected workflows if the runner group is restricted to workflows, otherwise the selected workflows are not relevant for the equality of the runner group
		equalSelectedWorkflows = cmp.Equal(k8sGroup.SelectedWorkflows, ghGroup.SelectedWorkflows, cmpopts.SortSlices(strings.Compare))
	}

	return k8sGroup.Name == ghGroup.GetName() &&
		equalRestrictedToWorkflows &&
		cmp.Equal(k8sGroup.Visibility, ghGroup.Visibility) &&
		equalSelectedWorkflows
}

func MapRunnerGroupToCreateRequest(group v1alpha1.RunnerGroup, repos []v1alpha1.Repository) github.CreateRunnerGroupRequest {
	selectedRepositoryIDs := GetSelectedRepositoryIDsForRunnerGroup(group, repos)
	return github.CreateRunnerGroupRequest{
		Name:                  new(group.Name),
		Visibility:            group.Visibility,
		SelectedRepositoryIDs: selectedRepositoryIDs,
		RestrictedToWorkflows: group.RestrictedToWorkflows,
		SelectedWorkflows:     group.SelectedWorkflows,
	}
}

func GetSelectedRepositoryIDsForRunnerGroup(group v1alpha1.RunnerGroup, repos []v1alpha1.Repository) []int64 {
	var selectedRepositoryIDs []int64
	if group.Visibility != nil && *group.Visibility == "selected" {
		selectedRepositoryIDs = make([]int64, 0)
		for _, repo := range repos {
			if repo.Status.ID == nil {
				// no id stored, thus the repo has not been reconciled. Therefore, we can not guarantee its existence
				continue
			}
			if slices.Contains(repo.Spec.AvailableActionsRunnerGroups, group.Name) {
				selectedRepositoryIDs = append(selectedRepositoryIDs, *repo.Status.ID)
			}
		}
	}
	return selectedRepositoryIDs
}
func MapRunnerGroupToUpdateRequest(group v1alpha1.RunnerGroup) github.UpdateRunnerGroupRequest {

	return github.UpdateRunnerGroupRequest{
		Name:                  new(group.Name),
		Visibility:            group.Visibility,
		RestrictedToWorkflows: group.RestrictedToWorkflows,
		SelectedWorkflows:     group.SelectedWorkflows,
	}
}

func EqualRepositoryIDs(expectedSelectedRepositoryIDs []int64, currentSelectedRepositories []*github.Repository) bool {
	if len(expectedSelectedRepositoryIDs) != len(currentSelectedRepositories) {
		return false
	}
	for _, repo := range currentSelectedRepositories {
		if !slices.Contains(expectedSelectedRepositoryIDs, repo.GetID()) {
			return false
		}
	}

	return true
}
