package mapper

import (
	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/utils"
	"github.com/google/go-github/v86/github"
)

func RepoToGithubRepo(repo *v1alpha1.Repository) *github.Repository {
	return &github.Repository{
		Name: &repo.Spec.Name,
		// don't set "private: true" to avoid errors or mismatched configurations
		Visibility:          &repo.Spec.Visibility,
		Archived:            repo.Spec.Archived,
		HasIssues:           repo.Spec.HasIssues,
		HasProjects:         repo.Spec.HasProjects,
		HasWiki:             repo.Spec.HasWiki,
		HasDownloads:        repo.Spec.HasDownloads,
		IsTemplate:          repo.Spec.IsTemplate,
		AllowSquashMerge:    getMergeStrategy(repo, "squash"),
		AllowRebaseMerge:    getMergeStrategy(repo, "rebase"),
		AllowMergeCommit:    getMergeStrategy(repo, "merge"),
		DeleteBranchOnMerge: repo.Spec.DeleteBranchOnMerge,
		MergeCommitTitle:    &repo.Spec.MergeCommitTitle,
		MergeCommitMessage:  &repo.Spec.MergeCommitMessage,
		Homepage:            utils.WithDefaultAsPtr(github.Ptr(repo.Spec.About.Website), ""),
		Description:         utils.WithDefaultAsPtr(github.Ptr(repo.Spec.About.Description), ""),
		DefaultBranch:       github.Ptr(repo.Spec.DefaultBranch),
	}
}

func RepoStaysArchived(repo *v1alpha1.Repository, githubRepo github.Repository) bool {
	wantArchived := false
	if repo.Spec.Archived != nil {
		wantArchived = *repo.Spec.Archived
	}
	return githubRepo.GetArchived() && githubRepo.GetArchived() == wantArchived
}

//nolint:gocyclo // complexity is manageable because it is just a sequence of simple comparisons
func RepoDiffers(repo *v1alpha1.Repository, githubRepo github.Repository) bool {
	if githubRepo.Name == nil {
		return true
	}
	if repo.Spec.Name != *githubRepo.Name {
		return true
	}
	specArchived := utils.WithDefault(repo.Spec.Archived, false)
	if githubRepo.Archived == nil {
		if specArchived {
			return true
		}
	} else {
		if specArchived != *githubRepo.Archived {
			return true
		}
	}
	if githubRepo.Visibility == nil || *githubRepo.Visibility != repo.Spec.Visibility {
		return true
	}
	// Compare bool fields, applying defaults to K8s spec nil values
	specHasIssues := utils.WithDefault(repo.Spec.HasIssues, true)
	if githubRepo.HasIssues == nil || *githubRepo.HasIssues != specHasIssues {
		return true
	}
	specHasProjects := utils.WithDefault(repo.Spec.HasProjects, false)
	if githubRepo.HasProjects == nil || *githubRepo.HasProjects != specHasProjects {
		return true
	}
	specHasWiki := utils.WithDefault(repo.Spec.HasWiki, false)
	if githubRepo.HasWiki == nil || *githubRepo.HasWiki != specHasWiki {
		return true
	}
	specHasDownloads := utils.WithDefault(repo.Spec.HasDownloads, false)
	if githubRepo.HasDownloads == nil || *githubRepo.HasDownloads != specHasDownloads {
		return true
	}
	specIsTemplate := utils.WithDefault(repo.Spec.IsTemplate, false)
	if githubRepo.IsTemplate == nil || *githubRepo.IsTemplate != specIsTemplate {
		return true
	}
	if githubRepo.AllowSquashMerge == nil || *githubRepo.AllowSquashMerge != *getMergeStrategy(repo, "squash") {
		return true
	}
	if githubRepo.AllowRebaseMerge == nil || *githubRepo.AllowRebaseMerge != *getMergeStrategy(repo, "rebase") {
		return true
	}
	if githubRepo.AllowMergeCommit == nil || *githubRepo.AllowMergeCommit != *getMergeStrategy(repo, "merge") {
		return true
	}
	specDeleteBranchOnMerge := utils.WithDefault(repo.Spec.DeleteBranchOnMerge, true)
	if githubRepo.DeleteBranchOnMerge == nil || *githubRepo.DeleteBranchOnMerge != specDeleteBranchOnMerge {
		return true
	}
	if githubRepo.MergeCommitTitle == nil || *githubRepo.MergeCommitTitle != repo.Spec.MergeCommitTitle {
		return true
	}
	if githubRepo.MergeCommitMessage == nil || *githubRepo.MergeCommitMessage != repo.Spec.MergeCommitMessage {
		return true
	}
	if githubRepo.Homepage == nil || *githubRepo.Homepage != repo.Spec.About.Website {
		return true
	}
	if *utils.WithDefaultAsPtr(githubRepo.Description, "") != repo.Spec.About.Description {
		return true
	}
	if githubRepo.DefaultBranch == nil || *githubRepo.DefaultBranch != repo.Spec.DefaultBranch {
		return true
	}

	return false
}

func getMergeStrategy(repo *v1alpha1.Repository, strategy string) *bool {
	for _, s := range repo.Spec.AllowedMergeStrategies {
		if s.Type == strategy {
			return github.Ptr(true)
		}
	}
	return github.Ptr(false)
}
