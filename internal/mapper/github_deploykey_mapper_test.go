package mapper

import (
	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/google/go-github/v89/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HashDeployKey", func() {
	const keyPlaceholder = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC..."
	Context("when hashing deploy key configuration", func() {
		It("should produce consistent hash for same configuration", func() {
			key := keyPlaceholder
			title := "test-deploy-key"
			readonly := true

			hash1 := HashDeployKey(key, title, readonly)
			hash2 := HashDeployKey(key, title, readonly)

			Expect(hash1).To(Equal(hash2))
		})

		It("should produce different hash for different keys", func() {
			title := "deploy-key"
			readonly := true

			hash1 := HashDeployKey(keyPlaceholder, title, readonly)
			hash2 := HashDeployKey("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQD...", title, readonly)

			Expect(hash1).NotTo(Equal(hash2))
		})

		It("should produce different hash for different titles", func() {
			key := keyPlaceholder
			readonly := true

			hash1 := HashDeployKey(key, "deploy-key-1", readonly)
			hash2 := HashDeployKey(key, "deploy-key-2", readonly)

			Expect(hash1).NotTo(Equal(hash2))
		})

		It("should produce different hash for different readonly values", func() {
			key := keyPlaceholder
			title := "deploy-key"

			hash1 := HashDeployKey(key, title, true)
			hash2 := HashDeployKey(key, title, false)

			Expect(hash1).NotTo(Equal(hash2))
		})

		It("should handle empty key", func() {
			hash := HashDeployKey("", "title", true)

			Expect(hash).NotTo(BeEmpty())
			Expect(hash).To(HaveLen(64)) // SHA256 hex length
		})

		It("should handle empty title", func() {
			hash := HashDeployKey("ssh-rsa AAAAB3...", "", false)

			Expect(hash).NotTo(BeEmpty())
			Expect(hash).To(HaveLen(64))
		})

		It("should handle all empty values", func() {
			hash := HashDeployKey("", "", false)

			Expect(hash).NotTo(BeEmpty())
			Expect(hash).To(HaveLen(64))
		})

		It("should produce valid SHA256 hex string", func() {
			hash := HashDeployKey("test-key", "test-title", true)

			Expect(hash).To(MatchRegexp("^[0-9a-f]{64}$"))
		})

		It("should handle very long key", func() {
			longKey := "ssh-rsa " + string(make([]byte, 10000))
			hash := HashDeployKey(longKey, "title", true)

			Expect(hash).NotTo(BeEmpty())
			Expect(hash).To(HaveLen(64))
		})

		It("should handle special characters in title", func() {
			key := "ssh-rsa AAAAB3..."
			title := "deploy-key-!@#$%^&*()_+-=[]{}|;:',.<>?/~`"
			readonly := true

			hash1 := HashDeployKey(key, title, readonly)
			hash2 := HashDeployKey(key, title, readonly)

			Expect(hash1).To(Equal(hash2))
		})

		It("should handle unicode characters in title", func() {
			key := "ssh-rsa AAAAB3..."
			title := "deploy-key-日本語-中文-한글"
			readonly := true

			hash := HashDeployKey(key, title, readonly)

			Expect(hash).NotTo(BeEmpty())
			Expect(hash).To(HaveLen(64))
		})

		It("should handle newlines in key", func() {
			key := "ssh-rsa AAAAB3...\nwith\nnewlines"
			title := "deploy-key"
			readonly := true

			hash := HashDeployKey(key, title, readonly)

			Expect(hash).NotTo(BeEmpty())
			Expect(hash).To(HaveLen(64))
		})
	})

	Context("when comparing hashes", func() {
		It("should treat readonly=true and readonly=false differently", func() {
			key := keyPlaceholder
			title := "deploy-key"

			hashTrue := HashDeployKey(key, title, true)
			hashFalse := HashDeployKey(key, title, false)

			Expect(hashTrue).NotTo(Equal(hashFalse))
		})

		It("should be case-sensitive for key", func() {
			title := "deploy-key"
			readonly := true

			hash1 := HashDeployKey("SSH-RSA AAAAB3...", title, readonly)
			hash2 := HashDeployKey("ssh-rsa AAAAB3...", title, readonly)

			Expect(hash1).NotTo(Equal(hash2))
		})

		It("should be case-sensitive for title", func() {
			key := "ssh-rsa AAAAB3..."
			readonly := true

			hash1 := HashDeployKey(key, "Deploy-Key", readonly)
			hash2 := HashDeployKey(key, "deploy-key", readonly)

			Expect(hash1).NotTo(Equal(hash2))
		})
	})
})

var _ = Describe("DeployKeyPresetToGitHubDeployKey", func() {
	Context("when converting deploy key preset", func() {
		It("should convert all fields correctly", func() {
			preset := v1alpha1.DeployKey{
				Key:      "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC...",
				ReadOnly: new(true),
				Title:    "production-deploy-key",
			}

			result := DeployKeyPresetToGitHubDeployKey(preset)

			Expect(result).NotTo(BeNil())
			Expect(result.Key).NotTo(BeNil())
			Expect(*result.Key).To(Equal(preset.Key))
			Expect(result.ReadOnly).NotTo(BeNil())
			Expect(*result.ReadOnly).To(Equal(*preset.ReadOnly))
			Expect(result.Title).NotTo(BeNil())
			Expect(*result.Title).To(Equal(preset.Title))
		})

		It("should handle read-write deploy key", func() {
			preset := v1alpha1.DeployKey{
				Key:      "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI...",
				ReadOnly: new(false),
				Title:    "write-access-key",
			}

			result := DeployKeyPresetToGitHubDeployKey(preset)

			Expect(result).NotTo(BeNil())
			Expect(result.GetReadOnly()).To(BeFalse())
		})

		It("should handle empty key", func() {
			preset := v1alpha1.DeployKey{
				Key:      "",
				ReadOnly: new(true),
				Title:    "empty-key",
			}

			result := DeployKeyPresetToGitHubDeployKey(preset)

			Expect(result).NotTo(BeNil())
			Expect(result.GetKey()).To(BeEmpty())
		})

		It("should handle empty title", func() {
			preset := v1alpha1.DeployKey{
				Key:      "ssh-rsa AAAAB3...",
				ReadOnly: new(true),
				Title:    "",
			}

			result := DeployKeyPresetToGitHubDeployKey(preset)

			Expect(result).NotTo(BeNil())
			Expect(result.GetTitle()).To(BeEmpty())
		})

		It("should create proper GitHub Key type", func() {
			preset := v1alpha1.DeployKey{
				Key:      "test-key",
				ReadOnly: new(false),
				Title:    "test-title",
			}

			result := DeployKeyPresetToGitHubDeployKey(preset)

			Expect(result).To(BeAssignableToTypeOf(&github.Key{}))
		})

		It("should handle all fields as pointers", func() {
			preset := v1alpha1.DeployKey{
				Key:      "ssh-rsa AAAAB3...",
				ReadOnly: new(true),
				Title:    "deploy-key",
			}

			result := DeployKeyPresetToGitHubDeployKey(preset)

			Expect(result.Key).NotTo(BeNil())
			Expect(result.ReadOnly).NotTo(BeNil())
			Expect(result.Title).NotTo(BeNil())
		})

		It("should handle special characters in key", func() {
			preset := v1alpha1.DeployKey{
				Key:      "ssh-rsa AAAAB3...+/=",
				ReadOnly: new(true),
				Title:    "special-key",
			}

			result := DeployKeyPresetToGitHubDeployKey(preset)

			Expect(result.GetKey()).To(Equal(preset.Key))
		})

		It("should handle unicode in title", func() {
			preset := v1alpha1.DeployKey{
				Key:      "ssh-rsa AAAAB3...",
				ReadOnly: new(false),
				Title:    "部署密钥-デプロイキー",
			}

			result := DeployKeyPresetToGitHubDeployKey(preset)

			Expect(result.GetTitle()).To(Equal(preset.Title))
		})

		It("should preserve readonly flag exactly", func() {
			presetTrue := v1alpha1.DeployKey{
				Key:      "key1",
				ReadOnly: new(true),
				Title:    "readonly-key",
			}
			presetFalse := v1alpha1.DeployKey{
				Key:      "key2",
				ReadOnly: new(false),
				Title:    "readwrite-key",
			}

			resultTrue := DeployKeyPresetToGitHubDeployKey(presetTrue)
			resultFalse := DeployKeyPresetToGitHubDeployKey(presetFalse)

			Expect(resultTrue.GetReadOnly()).To(BeTrue())
			Expect(resultFalse.GetReadOnly()).To(BeFalse())
		})
	})

	Context("when handling different key types", func() {
		It("should handle RSA keys", func() {
			preset := v1alpha1.DeployKey{
				Key:      "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC...",
				ReadOnly: new(true),
				Title:    "rsa-key",
			}

			result := DeployKeyPresetToGitHubDeployKey(preset)

			Expect(result.GetKey()).To(ContainSubstring("ssh-rsa"))
		})

		It("should handle Ed25519 keys", func() {
			preset := v1alpha1.DeployKey{
				Key:      "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI...",
				ReadOnly: new(true),
				Title:    "ed25519-key",
			}

			result := DeployKeyPresetToGitHubDeployKey(preset)

			Expect(result.GetKey()).To(ContainSubstring("ssh-ed25519"))
		})

		It("should handle ECDSA keys", func() {
			preset := v1alpha1.DeployKey{
				Key:      "ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTY...",
				ReadOnly: new(false),
				Title:    "ecdsa-key",
			}

			result := DeployKeyPresetToGitHubDeployKey(preset)

			Expect(result.GetKey()).To(ContainSubstring("ecdsa-sha2"))
		})

		It("should handle keys with comments", func() {
			preset := v1alpha1.DeployKey{
				Key:      "ssh-rsa AAAAB3... user@host",
				ReadOnly: new(true),
				Title:    "key-with-comment",
			}

			result := DeployKeyPresetToGitHubDeployKey(preset)

			Expect(result.GetKey()).To(ContainSubstring("user@host"))
		})
	})

	Context("when handling edge cases", func() {
		It("should handle very long titles", func() {
			longTitle := string(make([]byte, 1000))
			preset := v1alpha1.DeployKey{
				Key:      "ssh-rsa AAAAB3...",
				ReadOnly: new(true),
				Title:    longTitle,
			}

			result := DeployKeyPresetToGitHubDeployKey(preset)

			Expect(result.GetTitle()).To(Equal(longTitle))
		})

		It("should handle whitespace in key", func() {
			preset := v1alpha1.DeployKey{
				Key:      "  ssh-rsa AAAAB3...  ",
				ReadOnly: new(false),
				Title:    "whitespace-key",
			}

			result := DeployKeyPresetToGitHubDeployKey(preset)

			Expect(result.GetKey()).To(Equal(preset.Key))
		})

		It("should handle whitespace in title", func() {
			preset := v1alpha1.DeployKey{
				Key:      "ssh-rsa AAAAB3...",
				ReadOnly: new(true),
				Title:    "  deploy key  ",
			}

			result := DeployKeyPresetToGitHubDeployKey(preset)

			Expect(result.GetTitle()).To(Equal(preset.Title))
		})
	})

	Context("when ReadOnly field is nil (testing defaults)", func() {
		It("should fall back to true", func() {
			preset := v1alpha1.DeployKey{
				Key:      "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC...",
				ReadOnly: nil,
				Title:    "nil-readonly-key",
			}

			result := DeployKeyPresetToGitHubDeployKey(preset)

			Expect(result).NotTo(BeNil())
			Expect(result.Key).NotTo(BeNil())
			Expect(*result.Key).To(Equal(preset.Key))
			Expect(result.Title).NotTo(BeNil())
			Expect(*result.Title).To(Equal(preset.Title))
			Expect(result.ReadOnly).NotTo(BeNil())
			Expect(*result.ReadOnly).To(BeTrue())
		})

	})
})
