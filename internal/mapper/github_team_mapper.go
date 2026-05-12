package mapper

import (
	"strings"
	"unicode"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/google/go-github/v86/github"
	"golang.org/x/text/unicode/norm"
)

const (
	DefaultTeamPrivacy             = "closed"
	DefaultTeamPermission          = "pull"
	DefaultTeamNotificationSetting = "notifications_disabled"
)

func teamPrivacy(team *v1alpha1.Team) string {
	if team.Spec.Privacy == "" {
		return DefaultTeamPrivacy
	}
	return team.Spec.Privacy
}

func teamPermission(team *v1alpha1.Team) string {
	if team.Spec.Permission == "" {
		return DefaultTeamPermission
	}
	return team.Spec.Permission
}

func teamNotificationSetting(team *v1alpha1.Team) string {
	if team.Spec.NotificationSetting == "" {
		return DefaultTeamNotificationSetting
	}
	return team.Spec.NotificationSetting
}

func TeamToNewGitHubTeam(team *v1alpha1.Team) *github.NewTeam {
	return &github.NewTeam{
		Name:                team.Spec.Name,
		Description:         github.Ptr(team.Spec.Description),
		Privacy:             github.Ptr(teamPrivacy(team)),
		Permission:          github.Ptr(teamPermission(team)),
		NotificationSetting: github.Ptr(teamNotificationSetting(team)),
	}
}

func TeamDiffers(team *v1alpha1.Team, githubTeam *github.Team, org string) bool {
	if team == nil {
		return true
	}
	if githubTeam == nil {
		return true
	}
	if team.Spec.Name != *githubTeam.Name {
		return true
	}
	if githubTeam.Description == nil || *githubTeam.Description != team.Spec.Description {
		return true
	}
	if githubTeam.Privacy == nil || *githubTeam.Privacy != teamPrivacy(team) {
		return true
	}
	if githubTeam.Permission == nil || *githubTeam.Permission != teamPermission(team) {
		return true
	}
	if githubTeam.NotificationSetting == nil || *githubTeam.NotificationSetting != teamNotificationSetting(team) {
		return true
	}

	return false
}

func TeamNameToSlug(name string) *string {
	// To create the slug, GitHub Enterprise Cloud replaces special characters in the name string,
	// changes all words to lowercase, and replaces spaces with a - separator.
	// For example, "My TEam Näme" would become my-team-name.

	slug := strings.ToLower(name)

	// Replace spaces with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")

	// Remove or replace special characters (keep only alphanumeric and hyphens)
	var result strings.Builder
	for _, r := range slug {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			// Normalize accented characters to their base form
			normalized := norm.NFD.String(string(r))
			for _, nr := range normalized {
				if unicode.Is(unicode.Mn, nr) {
					continue // Skip combining marks
				}
				if (nr >= 'a' && nr <= 'z') || (nr >= '0' && nr <= '9') || nr == '-' {
					result.WriteRune(nr)
				}
			}
		}
	}

	// Clean up multiple consecutive hyphens
	cleanedSlug := result.String()
	for strings.Contains(cleanedSlug, "--") {
		cleanedSlug = strings.ReplaceAll(cleanedSlug, "--", "-")
	}

	// Trim leading/trailing hyphens
	cleanedSlug = strings.Trim(cleanedSlug, "-")

	return github.Ptr(cleanedSlug)
}
