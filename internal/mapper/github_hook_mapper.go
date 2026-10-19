package mapper

import (
	"crypto/sha256"
	"fmt"
	"slices"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/utils"
	"github.com/google/go-github/v93/github"
)

const (
	// defaultContentType is the default content type for webhooks
	defaultContentType = "application/json"
	// defaultSSLVerify is the default SSL verification setting
	defaultSSLVerify = "1"
	// disabledSSLVerify is the value for disabled SSL verification
	disabledSSLVerify = "0"
)

// WebhookSpecToGithubHook maps a webhook spec (from a WebhookPreset or an inline webhook) to a GitHub hook.
func WebhookSpecToGithubHook(spec v1alpha1.WebhookSpec) *github.Hook {
	conf := &github.HookConfig{
		URL: &spec.PayloadURL,
	}

	conf.Secret = spec.SecretValue

	// Set content type
	contentType := defaultContentType
	if spec.ContentType != "" {
		contentType = spec.ContentType
	}
	conf.ContentType = &contentType

	// Set SSL verification
	sslVerify := defaultSSLVerify
	if !utils.WithDefault(spec.SSLVerify, true) {
		sslVerify = disabledSSLVerify
	}
	conf.InsecureSSL = &sslVerify
	hook := &github.Hook{
		Name:   new("web"),
		Active: utils.WithDefaultAsPtr(spec.Active, true),
		Config: conf,
	}

	// Set events
	if len(spec.Events) > 0 {
		hook.Events = spec.Events
	} else {
		hook.Events = []string{"push"}
	}
	return hook
}

func HashWebhookConfig(url, contentType string, events []string) string {
	sortedEvents := slices.Sorted(slices.Values(events))
	h := sha256.Sum256(fmt.Appendf(nil, "%s|%s|%v", url, contentType, sortedEvents))
	return fmt.Sprintf("%x", h)
}
