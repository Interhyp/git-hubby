package mapper

import (
	"crypto/sha256"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
)

var _ = Describe("HashAutolink", func() {
	Context("when hashing autolink configuration", func() {
		It("should produce expected hash for alphanumeric autolink", func() {
			keyPrefix := "ABC"
			urlTemplate := "https://example.com/ABC/<num>"
			isAlphanumeric := true

			expected := fmt.Sprintf("%x", sha256.Sum256(fmt.Appendf(nil, "%s|%s|%t", keyPrefix, urlTemplate, isAlphanumeric)))
			actual := HashAutolink(keyPrefix, urlTemplate, isAlphanumeric)

			Expect(actual).To(Equal(expected))
		})

		It("should produce expected hash for non-alphanumeric autolink", func() {
			keyPrefix := "XYZ"
			urlTemplate := "https://example.com/XYZ/<num>"
			isAlphanumeric := false

			expected := fmt.Sprintf("%x", sha256.Sum256(fmt.Appendf(nil, "%s|%s|%t", keyPrefix, urlTemplate, isAlphanumeric)))
			actual := HashAutolink(keyPrefix, urlTemplate, isAlphanumeric)

			Expect(actual).To(Equal(expected))
		})

		It("should produce different hash for different isAlphanumeric values", func() {
			keyPrefix := "TEST"
			urlTemplate := "https://example.com/TEST/<num>"

			hashTrue := HashAutolink(keyPrefix, urlTemplate, true)
			hashFalse := HashAutolink(keyPrefix, urlTemplate, false)

			Expect(hashTrue).NotTo(Equal(hashFalse))
		})

		It("should produce consistent hash for same inputs", func() {
			keyPrefix := "ABC"
			urlTemplate := "https://example.com/ABC/<num>"
			isAlphanumeric := true

			hash1 := HashAutolink(keyPrefix, urlTemplate, isAlphanumeric)
			hash2 := HashAutolink(keyPrefix, urlTemplate, isAlphanumeric)

			Expect(hash1).To(Equal(hash2))
		})
	})
})

var _ = Describe("KubernetesAutolinkToGitHubAutolink", func() {
	Context("when converting autolink preset to GitHub autolink", func() {
		It("should convert all fields correctly", func() {
			preset := v1alpha1.Autolink{
				KeyPrefix:      "JIRA",
				URLTemplate:    "https://jira.example.com/browse/<num>",
				IsAlphanumeric: true,
			}

			result := KubernetesAutolinkToGitHubAutolink(preset)

			Expect(result).NotTo(BeNil())
			Expect(result.GetKeyPrefix()).To(Equal(preset.KeyPrefix))
			Expect(result.GetURLTemplate()).To(Equal(preset.URLTemplate))
			Expect(result.GetIsAlphanumeric()).To(Equal(preset.IsAlphanumeric))
		})

		It("should handle non-alphanumeric autolink", func() {
			preset := v1alpha1.Autolink{
				KeyPrefix:      "GH",
				URLTemplate:    "https://github.com/issues/<num>",
				IsAlphanumeric: false,
			}

			result := KubernetesAutolinkToGitHubAutolink(preset)

			Expect(result).NotTo(BeNil())
			Expect(result.GetKeyPrefix()).To(Equal("GH"))
			Expect(result.GetURLTemplate()).To(Equal("https://github.com/issues/<num>"))
			Expect(result.GetIsAlphanumeric()).To(BeFalse())
		})

		It("should preserve empty values", func() {
			preset := v1alpha1.Autolink{
				KeyPrefix:      "",
				URLTemplate:    "",
				IsAlphanumeric: false,
			}

			result := KubernetesAutolinkToGitHubAutolink(preset)

			Expect(result).NotTo(BeNil())
			Expect(result.GetKeyPrefix()).To(BeEmpty())
			Expect(result.GetURLTemplate()).To(BeEmpty())
		})
	})
})
