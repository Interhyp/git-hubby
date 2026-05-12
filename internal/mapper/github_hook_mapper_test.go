package mapper

import (
	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/google/go-github/v86/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GitHub Hook Mapper", func() {

	const contentTypeApplicationJSON = "application/json"
	Describe("WebhookPresetToGithubHook", func() {
		Context("when converting a webhook preset with all fields set", func() {
			It("should successfully convert to GitHub hook", func() {
				preset := v1alpha1.WebhookPreset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-webhook",
					},
					Spec: v1alpha1.WebhookPresetSpec{
						PayloadURL:  "https://example.com/webhook",
						ContentType: contentTypeApplicationJSON,
						SecretValue: github.Ptr("secret123"),
						SSLVerify:   github.Ptr(true),
						Active:      github.Ptr(true),
						Events:      []string{"push", "pull_request"},
					},
				}

				hook := WebhookPresetToGithubHook(preset)

				Expect(hook).NotTo(BeNil())
				Expect(hook.Name).To(Equal(github.Ptr("web")))
				Expect(hook.Active).To(Equal(github.Ptr(true)))
				Expect(hook.Config).NotTo(BeNil())
				Expect(hook.Config.URL).To(Equal(github.Ptr("https://example.com/webhook")))
				Expect(hook.Config.ContentType).To(Equal(github.Ptr(contentTypeApplicationJSON)))
				Expect(hook.Config.Secret).To(Equal(github.Ptr("secret123")))
				Expect(hook.Config.InsecureSSL).To(Equal(github.Ptr("1")))
				Expect(hook.Events).To(ConsistOf("push", "pull_request"))
			})
		})

		Context("when converting a webhook preset with SSL verification disabled", func() {
			It("should set InsecureSSL to 0", func() {
				preset := v1alpha1.WebhookPreset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "insecure-webhook",
					},
					Spec: v1alpha1.WebhookPresetSpec{
						PayloadURL: "https://example.com/webhook",
						SSLVerify:  github.Ptr(false),
						Active:     github.Ptr(true),
					},
				}

				hook := WebhookPresetToGithubHook(preset)

				Expect(hook).NotTo(BeNil())
				Expect(hook.Config.InsecureSSL).To(Equal(github.Ptr("0")))
			})
		})

		Context("when converting a webhook preset without content type", func() {
			It("should use default content type", func() {
				preset := v1alpha1.WebhookPreset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "default-content-type",
					},
					Spec: v1alpha1.WebhookPresetSpec{
						PayloadURL: "https://example.com/webhook",
						Active:     github.Ptr(true),
					},
				}

				hook := WebhookPresetToGithubHook(preset)

				Expect(hook).NotTo(BeNil())
				Expect(hook.Config.ContentType).To(Equal(github.Ptr(contentTypeApplicationJSON)))
			})
		})

		Context("when converting a webhook preset with custom content type", func() {
			It("should use the custom content type", func() {
				preset := v1alpha1.WebhookPreset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "custom-content-type",
					},
					Spec: v1alpha1.WebhookPresetSpec{
						PayloadURL:  "https://example.com/webhook",
						ContentType: "application/x-www-form-urlencoded",
						Active:      github.Ptr(true),
					},
				}

				hook := WebhookPresetToGithubHook(preset)

				Expect(hook).NotTo(BeNil())
				Expect(hook.Config.ContentType).To(Equal(github.Ptr("application/x-www-form-urlencoded")))
			})
		})

		Context("when converting a webhook preset without events", func() {
			It("should default to push event", func() {
				preset := v1alpha1.WebhookPreset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "default-events",
					},
					Spec: v1alpha1.WebhookPresetSpec{
						PayloadURL: "https://example.com/webhook",
						Active:     github.Ptr(true),
					},
				}

				hook := WebhookPresetToGithubHook(preset)

				Expect(hook).NotTo(BeNil())
				Expect(hook.Events).To(ConsistOf("push"))
			})
		})

		Context("when converting an inactive webhook preset", func() {
			It("should set Active to false", func() {
				preset := v1alpha1.WebhookPreset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "inactive-webhook",
					},
					Spec: v1alpha1.WebhookPresetSpec{
						PayloadURL: "https://example.com/webhook",
						Active:     github.Ptr(false),
					},
				}

				hook := WebhookPresetToGithubHook(preset)

				Expect(hook).NotTo(BeNil())
				Expect(hook.Active).To(Equal(github.Ptr(false)))
			})
		})

		Context("when converting a webhook preset with multiple events", func() {
			It("should include all events", func() {
				preset := v1alpha1.WebhookPreset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "multi-event-webhook",
					},
					Spec: v1alpha1.WebhookPresetSpec{
						PayloadURL: "https://example.com/webhook",
						Active:     github.Ptr(true),
						Events:     []string{"push", "pull_request", "issues", "release"},
					},
				}

				hook := WebhookPresetToGithubHook(preset)

				Expect(hook).NotTo(BeNil())
				Expect(hook.Events).To(ConsistOf("push", "pull_request", "issues", "release"))
			})
		})

		Context("when converting a webhook preset with empty secret", func() {
			It("should set empty secret", func() {
				preset := v1alpha1.WebhookPreset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "no-secret",
					},
					Spec: v1alpha1.WebhookPresetSpec{
						PayloadURL: "https://example.com/webhook",
						Active:     github.Ptr(true),
					},
				}

				hook := WebhookPresetToGithubHook(preset)

				Expect(hook).NotTo(BeNil())
				Expect(hook.Config.Secret).To(BeNil())
			})
		})
	})

	Describe("HashWebhookConfig", func() {
		Context("when hashing webhook configuration", func() {
			It("should produce consistent hash for same configuration", func() {
				url := "https://example.com/webhook"
				contentType := contentTypeApplicationJSON
				events := []string{"push", "pull_request"}

				hash1 := HashWebhookConfig(url, contentType, events)
				hash2 := HashWebhookConfig(url, contentType, events)

				Expect(hash1).To(Equal(hash2))
			})

			It("should produce different hash for different URLs", func() {
				contentType := contentTypeApplicationJSON
				events := []string{"push"}

				hash1 := HashWebhookConfig("https://example.com/webhook1", contentType, events)
				hash2 := HashWebhookConfig("https://example.com/webhook2", contentType, events)

				Expect(hash1).NotTo(Equal(hash2))
			})

			It("should produce different hash for different content types", func() {
				url := "https://example.com/webhook"
				events := []string{"push"}

				hash1 := HashWebhookConfig(url, contentTypeApplicationJSON, events)
				hash2 := HashWebhookConfig(url, "application/x-www-form-urlencoded", events)

				Expect(hash1).NotTo(Equal(hash2))
			})

			It("should produce different hash for different events", func() {
				url := "https://example.com/webhook"
				contentType := contentTypeApplicationJSON

				hash1 := HashWebhookConfig(url, contentType, []string{"push"})
				hash2 := HashWebhookConfig(url, contentType, []string{"pull_request"})

				Expect(hash1).NotTo(Equal(hash2))
			})

			It("should produce same hash regardless of event order", func() {
				url := "https://example.com/webhook"
				contentType := contentTypeApplicationJSON

				hash1 := HashWebhookConfig(url, contentType, []string{"push", "pull_request", "issues"})
				hash2 := HashWebhookConfig(url, contentType, []string{"issues", "pull_request", "push"})

				Expect(hash1).To(Equal(hash2))
			})

			It("should handle empty events list", func() {
				url := "https://example.com/webhook"
				contentType := contentTypeApplicationJSON

				hash := HashWebhookConfig(url, contentType, []string{})

				Expect(hash).NotTo(BeEmpty())
				Expect(hash).To(HaveLen(64)) // SHA256 hex length
			})

			It("should handle single event", func() {
				url := "https://example.com/webhook"
				contentType := contentTypeApplicationJSON

				hash := HashWebhookConfig(url, contentType, []string{"push"})

				Expect(hash).NotTo(BeEmpty())
				Expect(hash).To(HaveLen(64))
			})
		})
	})
})
