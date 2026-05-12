package mapper

import (
	"crypto/sha256"
	"fmt"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/google/go-github/v86/github"
)

func HashAutolink(keyPrefix, urlTemplate string, isAlphanumeric bool) string {
	h := sha256.Sum256(fmt.Appendf(nil, "%s|%s|%t", keyPrefix, urlTemplate, isAlphanumeric))
	return fmt.Sprintf("%x", h)
}

func KubernetesAutolinkToGitHubAutolink(preset v1alpha1.Autolink) *github.AutolinkOptions {
	return &github.AutolinkOptions{
		KeyPrefix:      github.Ptr(preset.KeyPrefix),
		URLTemplate:    github.Ptr(preset.URLTemplate),
		IsAlphanumeric: github.Ptr(preset.IsAlphanumeric),
	}
}
