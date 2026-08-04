package mapper

import (
	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v90/github"
)

func OrgToGithubOrg(organization *v1alpha1.Organization) *github.Organization {
	displayName := organization.GetDisplayName()
	login := organization.GetLogin()
	ghOrg := &github.Organization{
		Login:       &login,
		Name:        &displayName,
		Description: &organization.Spec.Description,
	}

	if organization.Spec.Location != "" {
		ghOrg.Location = &organization.Spec.Location
	}
	if organization.Spec.Website != "" {
		ghOrg.Blog = &organization.Spec.Website
	}
	if organization.Spec.MemberPrivileges != nil {
		mp := *organization.Spec.MemberPrivileges
		ghOrg.DefaultRepoPermission = mp.DefaultRepositoryPermission
		ghOrg.MembersCanCreatePages = mp.MembersCanCreatePages
		ghOrg.MembersCanCreatePublicPages = mp.MembersCanCreatePublicPages
		ghOrg.MembersCanCreatePrivatePages = mp.MembersCanCreatePrivatePages
		ghOrg.MembersCanCreatePublicRepos = mp.MembersCanCreatePublicRepositories
		ghOrg.MembersCanCreatePrivateRepos = mp.MembersCanCreatePrivateRepositories
		ghOrg.MembersCanForkPrivateRepos = mp.MembersCanForkPrivateRepositories
		if organization.HasEnterpriseFeatures() {
			ghOrg.MembersCanCreateInternalRepos = mp.MembersCanCreateInternalRepositories
		}
	}

	return ghOrg
}

func OrgDiffers(org *v1alpha1.Organization, githubOrg github.Organization) bool {
	expectedLogin := org.GetLogin()
	if expectedLogin != githubOrg.GetLogin() {
		return true
	}

	expectedDisplayName := org.GetDisplayName()
	if expectedDisplayName != githubOrg.GetName() {
		return true
	}

	if org.Spec.Description != githubOrg.GetDescription() {
		return true
	}

	if org.Spec.Location != githubOrg.GetLocation() {
		return true
	}

	if org.Spec.Website != githubOrg.GetBlog() {
		return true
	}

	if org.Spec.MemberPrivileges != nil {
		mp := *org.Spec.MemberPrivileges

		if privilegeDiffers(githubOrg.DefaultRepoPermission, mp.DefaultRepositoryPermission) {
			return true
		}
		if privilegeDiffers(githubOrg.MembersCanCreatePages, mp.MembersCanCreatePages) {
			return true
		}
		if privilegeDiffers(githubOrg.MembersCanCreatePublicPages, mp.MembersCanCreatePublicPages) {
			return true
		}
		if privilegeDiffers(githubOrg.MembersCanCreatePrivatePages, mp.MembersCanCreatePrivatePages) {
			return true
		}
		if privilegeDiffers(githubOrg.MembersCanCreatePublicRepos, mp.MembersCanCreatePublicRepositories) {
			return true
		}
		if privilegeDiffers(githubOrg.MembersCanCreatePrivateRepos, mp.MembersCanCreatePrivateRepositories) {
			return true
		}
		if privilegeDiffers(githubOrg.MembersCanForkPrivateRepos, mp.MembersCanForkPrivateRepositories) {
			return true
		}
		if org.HasEnterpriseFeatures() && privilegeDiffers(githubOrg.MembersCanCreateInternalRepos, mp.MembersCanCreateInternalRepositories) {
			return true
		}
	}
	return false
}

func privilegeDiffers[A any](ghValue, specValue *A) bool {
	// always compare mp value to nil: if privilege setting is nil, the GitHub setting is authoritative
	// and no reconciliation should be triggered
	return specValue != nil && !cmp.Equal(ghValue, specValue)
}
