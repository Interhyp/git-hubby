package mapper

import (
	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/google/go-github/v86/github"
)

func OrgToGithubOrg(organization *v1alpha1.Organization) *github.Organization {
	return &github.Organization{
		Name:        &organization.Spec.Name,
		Description: &organization.Spec.Description,
	}
}

func OrgDiffers(org *v1alpha1.Organization, githubOrg github.Organization) bool {
	if org.Spec.Name != githubOrg.GetName() {
		return true
	}
	if org.Spec.Description != githubOrg.GetDescription() {
		return true
	}

	return false
}
