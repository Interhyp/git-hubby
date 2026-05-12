package mapper

import (
	"crypto/sha256"
	"fmt"
	"slices"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/utils"
	"github.com/google/go-github/v86/github"
)

const (
	// defaultContentType is the default content type for webhooks
	defaultContentType = "application/json"
	// defaultSSLVerify is the default SSL verification setting
	defaultSSLVerify = "1"
	// disabledSSLVerify is the value for disabled SSL verification
	disabledSSLVerify = "0"
)

func WebhookPresetToGithubHook(preset v1alpha1.WebhookPreset) *github.Hook {
	conf := &github.HookConfig{
		URL: &preset.Spec.PayloadURL,
	}

	conf.Secret = preset.Spec.SecretValue

	// Set content type
	contentType := defaultContentType
	if preset.Spec.ContentType != "" {
		contentType = preset.Spec.ContentType
	}
	conf.ContentType = &contentType

	// Set SSL verification
	sslVerify := defaultSSLVerify
	if !utils.WithDefault(preset.Spec.SSLVerify, true) {
		sslVerify = disabledSSLVerify
	}
	conf.InsecureSSL = &sslVerify
	hook := &github.Hook{
		Name:   github.Ptr("web"),
		Active: utils.WithDefaultAsPtr(preset.Spec.Active, true),
		Config: conf,
	}

	// Set events
	if len(preset.Spec.Events) > 0 {
		hook.Events = preset.Spec.Events
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
