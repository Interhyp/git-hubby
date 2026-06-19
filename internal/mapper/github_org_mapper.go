package mapper

import (
	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/google/go-github/v86/github"
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

	return false
}
