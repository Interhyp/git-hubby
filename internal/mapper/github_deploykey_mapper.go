package mapper

import (
	"crypto/sha256"
	"fmt"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/utils"
	"github.com/google/go-github/v86/github"
)

func HashDeployKey(key, title string, readonly bool) string {
	h := sha256.Sum256(fmt.Appendf(nil, "%s|%s|%t", key, title, readonly))
	return fmt.Sprintf("%x", h)
}

func DeployKeyPresetToGitHubDeployKey(preset v1alpha1.DeployKey) *github.Key {
	return &github.Key{
		Key:      github.Ptr(preset.Key),
		ReadOnly: utils.WithDefaultAsPtr(preset.ReadOnly, true),
		Title:    github.Ptr(preset.Title),
	}
}
