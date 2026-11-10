package mapper

import (
	"github.com/google/go-github/v93/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CodeSecurityConfigurationsDiffer", func() {

	Context("when configurations are identical", func() {
		It("should return false for identical configurations", func() {
			first := &github.CodeSecurityConfiguration{
				Name:             "test-config",
				Description:      "Test description",
				AdvancedSecurity: new("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:             "test-config",
				Description:      "Test description",
				AdvancedSecurity: new("enabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeFalse())
		})

		It("should return false for complex identical configurations", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                          "test-config",
				Description:                   "Test description",
				AdvancedSecurity:              new("enabled"),
				DependencyGraph:               new("enabled"),
				DependabotAlerts:              new("enabled"),
				SecretScanning:                new("enabled"),
				SecretScanningPushProtection:  new("enabled"),
				PrivateVulnerabilityReporting: new("enabled"),
				Enforcement:                   new("active"),
				CodeSecurity:                  new("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                          "test-config",
				Description:                   "Test description",
				AdvancedSecurity:              new("enabled"),
				DependencyGraph:               new("enabled"),
				DependabotAlerts:              new("enabled"),
				SecretScanning:                new("enabled"),
				SecretScanningPushProtection:  new("enabled"),
				PrivateVulnerabilityReporting: new("enabled"),
				Enforcement:                   new("active"),
				CodeSecurity:                  new("enabled"),
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
				AdvancedSecurity: new("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:             "test-config",
				AdvancedSecurity: new("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different dependency graph", func() {
			first := &github.CodeSecurityConfiguration{
				Name:            "test-config",
				DependencyGraph: new("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:            "test-config",
				DependencyGraph: new("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different dependency graph autosubmit action", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                            "test-config",
				DependencyGraphAutosubmitAction: new("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                            "test-config",
				DependencyGraphAutosubmitAction: new("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different dependency graph autosubmit action options", func() {
			first := &github.CodeSecurityConfiguration{
				Name: "test-config",
				DependencyGraphAutosubmitActionOptions: &github.DependencyGraphAutosubmitActionOptions{
					LabeledRunners: new(true),
				},
			}
			second := &github.CodeSecurityConfiguration{
				Name: "test-config",
				DependencyGraphAutosubmitActionOptions: &github.DependencyGraphAutosubmitActionOptions{
					LabeledRunners: new(false),
				},
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true when one has dependency graph autosubmit action options and other is nil", func() {
			first := &github.CodeSecurityConfiguration{
				Name: "test-config",
				DependencyGraphAutosubmitActionOptions: &github.DependencyGraphAutosubmitActionOptions{
					LabeledRunners: new(true),
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
				DependabotAlerts: new("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:             "test-config",
				DependabotAlerts: new("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different dependabot security updates", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                      "test-config",
				DependabotSecurityUpdates: new("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                      "test-config",
				DependabotSecurityUpdates: new("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different code scanning default setup", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                     "test-config",
				CodeScanningDefaultSetup: new("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                     "test-config",
				CodeScanningDefaultSetup: new("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different code scanning default setup options", func() {
			first := &github.CodeSecurityConfiguration{
				Name: "test-config",
				CodeScanningDefaultSetupOptions: &github.CodeScanningDefaultSetupOptions{
					RunnerType:  "labeled",
					RunnerLabel: new("runner-1"),
				},
			}
			second := &github.CodeSecurityConfiguration{
				Name: "test-config",
				CodeScanningDefaultSetupOptions: &github.CodeScanningDefaultSetupOptions{
					RunnerType:  "labeled",
					RunnerLabel: new("runner-2"),
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
				CodeScanningDelegatedAlertDismissal: new("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                                "test-config",
				CodeScanningDelegatedAlertDismissal: new("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different code scanning options", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                     "test-config",
				CodeScanningDefaultSetup: new("enabled"),
				CodeScanningOptions: &github.CodeScanningOptions{
					AllowAdvanced: new(true),
				},
			}
			second := &github.CodeSecurityConfiguration{
				Name:                     "test-config",
				CodeScanningDefaultSetup: new("enabled"),
				CodeScanningOptions: &github.CodeScanningOptions{
					AllowAdvanced: new(false),
				},
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true when one has code scanning options and other is nil", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                     "test-config",
				CodeScanningDefaultSetup: new("enabled"),
				CodeScanningOptions: &github.CodeScanningOptions{
					AllowAdvanced: new(true),
				},
			}
			second := &github.CodeSecurityConfiguration{
				Name:                     "test-config",
				CodeScanningDefaultSetup: new("enabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should compare code scanning options if default setup is only enabled on GitHub", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                "test-config",
				CodeScanningOptions: &github.CodeScanningOptions{AllowAdvanced: new(true)},
			}
			second := &github.CodeSecurityConfiguration{
				Name:                     "test-config",
				CodeScanningDefaultSetup: new("enabled"),
			}

			Expect(CodeSecurityConfigurationsDiffer(first, second)).To(BeTrue())
		})

		DescribeTable("should ignore code scanning options if default setup is not enabled",
			func(desiredDefaultSetup *string, currentDefaultSetup string) {
				first := &github.CodeSecurityConfiguration{
					Name:                     "test-config",
					CodeScanningDefaultSetup: desiredDefaultSetup,
					CodeScanningOptions:      &github.CodeScanningOptions{AllowAdvanced: new(true)},
				}
				second := &github.CodeSecurityConfiguration{
					Name:                     "test-config",
					CodeScanningDefaultSetup: new(currentDefaultSetup),
				}

				Expect(CodeSecurityConfigurationsDiffer(first, second)).To(BeFalse())
			},
			Entry("disabled", new("disabled"), "disabled"),
			Entry("not set", new("not_set"), "not_set"),
			Entry("unmanaged and disabled on GitHub", nil, "disabled"),
		)

		It("should return true for different code security", func() {
			first := &github.CodeSecurityConfiguration{
				Name:         "test-config",
				CodeSecurity: new("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:         "test-config",
				CodeSecurity: new("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different secret scanning", func() {
			first := &github.CodeSecurityConfiguration{
				Name:           "test-config",
				SecretScanning: new("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:           "test-config",
				SecretScanning: new("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different secret scanning push protection", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                         "test-config",
				SecretScanningPushProtection: new("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                         "test-config",
				SecretScanningPushProtection: new("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different secret scanning validity checks", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                         "test-config",
				SecretScanningValidityChecks: new("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                         "test-config",
				SecretScanningValidityChecks: new("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different secret scanning non provider patterns", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                              "test-config",
				SecretScanningNonProviderPatterns: new("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                              "test-config",
				SecretScanningNonProviderPatterns: new("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different secret scanning generic secrets", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                         "test-config",
				SecretScanningGenericSecrets: new("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                         "test-config",
				SecretScanningGenericSecrets: new("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different secret scanning delegated alert dismissal", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                                  "test-config",
				SecretScanningDelegatedAlertDismissal: new("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                                  "test-config",
				SecretScanningDelegatedAlertDismissal: new("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different secret protection", func() {
			first := &github.CodeSecurityConfiguration{
				Name:             "test-config",
				SecretProtection: new("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:             "test-config",
				SecretProtection: new("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different private vulnerability reporting", func() {
			first := &github.CodeSecurityConfiguration{
				Name:                          "test-config",
				PrivateVulnerabilityReporting: new("enabled"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:                          "test-config",
				PrivateVulnerabilityReporting: new("disabled"),
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})

		It("should return true for different enforcement", func() {
			first := &github.CodeSecurityConfiguration{
				Name:        "test-config",
				Enforcement: new("active"),
			}
			second := &github.CodeSecurityConfiguration{
				Name:        "test-config",
				Enforcement: new("inactive"),
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
					RunnerLabel: new("my-runner"),
				},
			}
			second := &github.CodeSecurityConfiguration{
				Name: "test-config",
				CodeScanningDefaultSetupOptions: &github.CodeScanningDefaultSetupOptions{
					RunnerType:  "labeled",
					RunnerLabel: new("my-runner"),
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
					RunnerLabel: new("my-runner"),
				},
			}
			second := &github.CodeSecurityConfiguration{
				Name: "test-config",
				CodeScanningDefaultSetupOptions: &github.CodeScanningDefaultSetupOptions{
					RunnerType:  "unlabeled",
					RunnerLabel: new("my-runner"),
				},
			}

			result := CodeSecurityConfigurationsDiffer(first, second)
			Expect(result).To(BeTrue())
		})
	})

	Context("when fields are not set in the desired configuration", func() {
		// Response of GitHub for an existing configuration: all features are returned, secret_protection is omitted.
		current := func() *github.CodeSecurityConfiguration {
			return &github.CodeSecurityConfiguration{
				Name:                            "disabled-config",
				Description:                     "Disables secret scanning",
				AdvancedSecurity:                new("disabled"),
				DependencyGraph:                 new("enabled"),
				DependencyGraphAutosubmitAction: new("disabled"),
				DependencyGraphAutosubmitActionOptions: &github.DependencyGraphAutosubmitActionOptions{
					LabeledRunners: new(false),
				},
				DependabotAlerts:                      new("enabled"),
				DependabotSecurityUpdates:             new("disabled"),
				CodeScanningDefaultSetup:              new("disabled"),
				CodeScanningDefaultSetupOptions:       &github.CodeScanningDefaultSetupOptions{RunnerType: "not_set"},
				CodeScanningDelegatedAlertDismissal:   new("disabled"),
				SecretScanning:                        new("disabled"),
				SecretScanningPushProtection:          new("disabled"),
				SecretScanningValidityChecks:          new("disabled"),
				SecretScanningNonProviderPatterns:     new("disabled"),
				SecretScanningGenericSecrets:          new("disabled"),
				SecretScanningDelegatedAlertDismissal: new("disabled"),
				PrivateVulnerabilityReporting:         new("disabled"),
				Enforcement:                           new("enforced"),
			}
		}
		desired := func() *github.CodeSecurityConfiguration {
			return &github.CodeSecurityConfiguration{
				Name:                                  "disabled-config",
				Description:                           "Disables secret scanning",
				SecretProtection:                      new("disabled"),
				SecretScanning:                        new("disabled"),
				SecretScanningDelegatedAlertDismissal: new("disabled"),
				SecretScanningGenericSecrets:          new("disabled"),
				SecretScanningNonProviderPatterns:     new("disabled"),
				SecretScanningPushProtection:          new("disabled"),
				SecretScanningValidityChecks:          new("disabled"),
				DependencyGraph:                       new("enabled"),
				DependabotAlerts:                      new("enabled"),
				Enforcement:                           new("enforced"),
			}
		}

		It("should return false if all set fields match", func() {
			Expect(CodeSecurityConfigurationsDiffer(desired(), current())).To(BeFalse())
		})

		It("should return true if a set field differs", func() {
			d := desired()
			d.SecretScanning = new("enabled")
			Expect(CodeSecurityConfigurationsDiffer(d, current())).To(BeTrue())
		})

		It("should return true if a set field is enabled but omitted by GitHub", func() {
			d := desired()
			d.SecretProtection = new("enabled")
			Expect(CodeSecurityConfigurationsDiffer(d, current())).To(BeTrue())
		})

		It("should return true if set options are missing on GitHub", func() {
			d := desired()
			d.CodeScanningDefaultSetup = new("enabled")
			d.CodeScanningOptions = &github.CodeScanningOptions{AllowAdvanced: new(true)}
			Expect(CodeSecurityConfigurationsDiffer(d, current())).To(BeTrue())
		})
	})

	// GitHub never returns code_security and secret_protection but reports both combined in advanced_security,
	// see the API behaviour verified against GitHub (API version 2026-03-10).
	DescribeTable("when comparing code security and secret protection with advanced security",
		func(advancedSecurity, codeSecurity, secretProtection *string, currentAdvancedSecurity string, expected bool) {
			desired := &github.CodeSecurityConfiguration{
				Name:             "test-config",
				AdvancedSecurity: advancedSecurity,
				CodeSecurity:     codeSecurity,
				SecretProtection: secretProtection,
			}
			current := &github.CodeSecurityConfiguration{
				Name:             "test-config",
				AdvancedSecurity: new(currentAdvancedSecurity),
			}
			Expect(CodeSecurityConfigurationsDiffer(desired, current)).To(Equal(expected))
		},
		Entry("both enabled", nil, new("enabled"), new("enabled"), "enabled", false),
		Entry("only code security enabled", nil, new("enabled"), new("disabled"), "code_security", false),
		Entry("only code security enabled, secret protection not set", nil, new("enabled"), new("not_set"), "code_security", false),
		Entry("only secret protection enabled", nil, new("disabled"), new("enabled"), "secret_protection", false),
		Entry("only secret protection enabled, code security not set", nil, new("not_set"), new("enabled"), "secret_protection", false),
		Entry("both disabled", nil, new("disabled"), new("disabled"), "disabled", false),
		Entry("both not set", nil, new("not_set"), new("not_set"), "disabled", false),
		Entry("code security enabled on GitHub only", nil, new("disabled"), new("enabled"), "enabled", true),
		Entry("secret protection disabled on GitHub", nil, new("enabled"), new("enabled"), "code_security", true),
		Entry("only code security managed, matching", nil, new("enabled"), nil, "enabled", false),
		Entry("only code security managed, differing", nil, new("disabled"), nil, "code_security", true),
		Entry("only secret protection managed, matching", nil, nil, new("disabled"), "code_security", false),
		Entry("advanced security enabled", new("enabled"), nil, nil, "enabled", false),
		Entry("advanced security enabled, differing", new("enabled"), nil, nil, "secret_protection", true),
		Entry("deprecated advanced security value", new("code_security"), nil, nil, "code_security", false),
		Entry("nothing managed", nil, nil, nil, "enabled", false),
	)

	Context("when configurations are nil", func() {
		It("should return false if both are nil", func() {
			Expect(CodeSecurityConfigurationsDiffer(nil, nil)).To(BeFalse())
		})

		It("should return true if only one is nil", func() {
			Expect(CodeSecurityConfigurationsDiffer(&github.CodeSecurityConfiguration{}, nil)).To(BeTrue())
			Expect(CodeSecurityConfigurationsDiffer(nil, &github.CodeSecurityConfiguration{})).To(BeTrue())
		})
	})
})
