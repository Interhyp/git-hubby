package mapper

import (
	"github.com/google/go-github/v86/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CodeSecurityConfigurationsDiffer", func() {

	Context("when configurations are identical", func() {
		It("should return false for identical configurations", func() {
			first := &github.CodeSecurityConfiguration{
				Name:             "test-config",
				Description:      "Test description",
				AdvancedSecurity: github.Ptr("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:             "test-config",
				Description:      "Test description",
				AdvancedSecurity: github.Ptr("enabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeFalse())
		})

		It("should return false for complex identical configurations", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                          "test-config",
				Description:                   "Test description",
				AdvancedSecurity:              github.Ptr("enabled"),
				DependencyGraph:               github.Ptr("enabled"),
				DependabotAlerts:              github.Ptr("enabled"),
				SecretScanning:                github.Ptr("enabled"),
				SecretScanningPushProtection:  github.Ptr("enabled"),
				PrivateVulnerabilityReporting: github.Ptr("enabled"),
				Enforcement:                   github.Ptr("active"),
				CodeSecurity:                  github.Ptr("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                          "test-config",
				Description:                   "Test description",
				AdvancedSecurity:              github.Ptr("enabled"),
				DependencyGraph:               github.Ptr("enabled"),
				DependabotAlerts:              github.Ptr("enabled"),
				SecretScanning:                github.Ptr("enabled"),
				SecretScanningPushProtection:  github.Ptr("enabled"),
				PrivateVulnerabilityReporting: github.Ptr("enabled"),
				Enforcement:                   github.Ptr("active"),
				CodeSecurity:                  github.Ptr("enabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeFalse())
		})
	})

	Context("when configurations differ", func() {
		It("should return true for different names", func() {
			first := &github.CodeSecurityConfiguration{
				Name:        "test-config-1",
				Description: "Test description",
			}
			second := &github.CodeSecurityConfiguration{
				Name:        "test-config-2",
				Description: "Test description",
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different descriptions", func() {
			first := &github.CodeSecurityConfiguration{
				Name:        "test-config",
				Description: "Description 1",
			}
			second := &github.CodeSecurityConfiguration{
				Name:        "test-config",
				Description: "Description 2",
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different advanced security", func() {
			first := &github.CodeSecurityConfiguration{
				Name:             "test-config",
				AdvancedSecurity: github.Ptr("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:             "test-config",
				AdvancedSecurity: github.Ptr("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different dependency graph", func() {
			first := &github.CodeSecurityConfiguration{
				Name:            "test-config",
				DependencyGraph: github.Ptr("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:            "test-config",
				DependencyGraph: github.Ptr("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different dependency graph autosubmit action", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                            "test-config",
				DependencyGraphAutosubmitAction: github.Ptr("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                            "test-config",
				DependencyGraphAutosubmitAction: github.Ptr("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different dependency graph autosubmit action options", func() {
			first := &github.CodeSecurityConfiguration{
				Name: "test-config",
				DependencyGraphAutosubmitActionOptions: &github.DependencyGraphAutosubmitActionOptions{
					LabeledRunners: github.Ptr(true),
				},
			}
			second := &github.CodeSecurityConfiguration{
				Name: "test-config",
				DependencyGraphAutosubmitActionOptions: &github.DependencyGraphAutosubmitActionOptions{
					LabeledRunners: github.Ptr(false),
				},
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true when one has dependency graph autosubmit action options and other is nil", func() {
			first := &github.CodeSecurityConfiguration{
				Name: "test-config",
				DependencyGraphAutosubmitActionOptions: &github.DependencyGraphAutosubmitActionOptions{
					LabeledRunners: github.Ptr(true),
				},
			}
			second := &github.CodeSecurityConfiguration{
				Name: "test-config",
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different dependabot alerts", func() {
			first := &github.CodeSecurityConfiguration{
				Name:             "test-config",
				DependabotAlerts: github.Ptr("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:             "test-config",
				DependabotAlerts: github.Ptr("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different dependabot security updates", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                      "test-config",
				DependabotSecurityUpdates: github.Ptr("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                      "test-config",
				DependabotSecurityUpdates: github.Ptr("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different code scanning default setup", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                     "test-config",
				CodeScanningDefaultSetup: github.Ptr("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                     "test-config",
				CodeScanningDefaultSetup: github.Ptr("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different code scanning default setup options", func() {
			first := &github.CodeSecurityConfiguration{
				Name: "test-config",
				CodeScanningDefaultSetupOptions: &github.CodeScanningDefaultSetupOptions{
					RunnerType:  "labeled",
					RunnerLabel: github.Ptr("runner-1"),
				},
			}
			second := &github.CodeSecurityConfiguration{
				Name: "test-config",
				CodeScanningDefaultSetupOptions: &github.CodeScanningDefaultSetupOptions{
					RunnerType:  "labeled",
					RunnerLabel: github.Ptr("runner-2"),
				},
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true when one has code scanning default setup options and other is nil", func() {
			first := &github.CodeSecurityConfiguration{
				Name: "test-config",
				CodeScanningDefaultSetupOptions: &github.CodeScanningDefaultSetupOptions{
					RunnerType: "labeled",
				},
			}
			second := &github.CodeSecurityConfiguration{
				Name: "test-config",
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different code scanning delegated alert dismissal", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                                "test-config",
				CodeScanningDelegatedAlertDismissal: github.Ptr("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                                "test-config",
				CodeScanningDelegatedAlertDismissal: github.Ptr("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different code scanning options", func() {
			first := &github.CodeSecurityConfiguration{
				Name: "test-config",
				CodeScanningOptions: &github.CodeScanningOptions{
					AllowAdvanced: github.Ptr(true),
				},
			}
			second := &github.CodeSecurityConfiguration{
				Name: "test-config",
				CodeScanningOptions: &github.CodeScanningOptions{
					AllowAdvanced: github.Ptr(false),
				},
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true when one has code scanning options and other is nil", func() {
			first := &github.CodeSecurityConfiguration{
				Name: "test-config",
				CodeScanningOptions: &github.CodeScanningOptions{
					AllowAdvanced: github.Ptr(true),
				},
			}
			second := &github.CodeSecurityConfiguration{
				Name: "test-config",
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different code security", func() {
			first := &github.CodeSecurityConfiguration{
				Name:         "test-config",
				CodeSecurity: github.Ptr("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:         "test-config",
				CodeSecurity: github.Ptr("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different secret scanning", func() {
			first := &github.CodeSecurityConfiguration{
				Name:           "test-config",
				SecretScanning: github.Ptr("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:           "test-config",
				SecretScanning: github.Ptr("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different secret scanning push protection", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                         "test-config",
				SecretScanningPushProtection: github.Ptr("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                         "test-config",
				SecretScanningPushProtection: github.Ptr("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different secret scanning validity checks", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                         "test-config",
				SecretScanningValidityChecks: github.Ptr("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                         "test-config",
				SecretScanningValidityChecks: github.Ptr("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different secret scanning non provider patterns", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                              "test-config",
				SecretScanningNonProviderPatterns: github.Ptr("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                              "test-config",
				SecretScanningNonProviderPatterns: github.Ptr("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different secret scanning generic secrets", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                         "test-config",
				SecretScanningGenericSecrets: github.Ptr("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                         "test-config",
				SecretScanningGenericSecrets: github.Ptr("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different secret scanning delegated alert dismissal", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                                  "test-config",
				SecretScanningDelegatedAlertDismissal: github.Ptr("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                                  "test-config",
				SecretScanningDelegatedAlertDismissal: github.Ptr("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different secret protection", func() {
			first := &github.CodeSecurityConfiguration{
				Name:             "test-config",
				SecretProtection: github.Ptr("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:             "test-config",
				SecretProtection: github.Ptr("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different private vulnerability reporting", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                          "test-config",
				PrivateVulnerabilityReporting: github.Ptr("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                          "test-config",
				PrivateVulnerabilityReporting: github.Ptr("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different enforcement", func() {
			first := &github.CodeSecurityConfiguration{
				Name:        "test-config",
				Enforcement: github.Ptr("active"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:        "test-config",
				Enforcement: github.Ptr("inactive"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})
	})

	Context("when comparing complex option objects", func() {
		It("should return false when both dependency graph autosubmit action options are nil", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                                   "test-config",
				DependencyGraphAutosubmitActionOptions: nil,
			}
			second := &github.CodeSecurityConfiguration{
				Name:                                   "test-config",
				DependencyGraphAutosubmitActionOptions: nil,
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeFalse())
		})

		It("should return false when both code scanning default setup options are nil", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                            "test-config",
				CodeScanningDefaultSetupOptions: nil,
			}
			second := &github.CodeSecurityConfiguration{
				Name:                            "test-config",
				CodeScanningDefaultSetupOptions: nil,
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeFalse())
		})

		It("should return false when both code scanning options are nil", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                "test-config",
				CodeScanningOptions: nil,
			}
			second := &github.CodeSecurityConfiguration{
				Name:                "test-config",
				CodeScanningOptions: nil,
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeFalse())
		})

		It("should return false when code scanning default setup options have same runner type and label", func() {
			first := &github.CodeSecurityConfiguration{
				Name: "test-config",
				CodeScanningDefaultSetupOptions: &github.CodeScanningDefaultSetupOptions{
					RunnerType:  "labeled",
					RunnerLabel: github.Ptr("my-runner"),
				},
			}
			second := &github.CodeSecurityConfiguration{
				Name: "test-config",
				CodeScanningDefaultSetupOptions: &github.CodeScanningDefaultSetupOptions{
					RunnerType:  "labeled",
					RunnerLabel: github.Ptr("my-runner"),
				},
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeFalse())
		})

		It("should return true for different runner types in code scanning default setup options", func() {
			first := &github.CodeSecurityConfiguration{
				Name: "test-config",
				CodeScanningDefaultSetupOptions: &github.CodeScanningDefaultSetupOptions{
					RunnerType:  "labeled",
					RunnerLabel: github.Ptr("my-runner"),
				},
			}
			second := &github.CodeSecurityConfiguration{
				Name: "test-config",
				CodeScanningDefaultSetupOptions: &github.CodeScanningDefaultSetupOptions{
					RunnerType:  "unlabeled",
					RunnerLabel: github.Ptr("my-runner"),
				},
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})
	})
})
