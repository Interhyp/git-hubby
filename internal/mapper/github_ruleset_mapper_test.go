package mapper

import (
	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/google/go-github/v89/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GitHub Ruleset Mapper", func() {
	const TargetTypeBranch = "branch"
	const TargetTypeTag = "tag"
	Context("RulesetPresetToGithubRuleset", func() {
		var rulesetPreset githubv1alpha1.RulesetPreset

		BeforeEach(func() {
			rulesetPreset = githubv1alpha1.RulesetPreset{
				Spec: githubv1alpha1.RulesetPresetSpec{
					Name:        "main-protection",
					Enforcement: githubv1alpha1.RulesetEnforcementActive,
					Conditions: &githubv1alpha1.RulesetConditions{
						RefName: &githubv1alpha1.RefNameCondition{
							Include: []string{"main", "develop"},
							Exclude: []string{"feature/*"},
						},
					},
					BypassActors: []githubv1alpha1.RulesetBypassActor{
						{
							ActorID:    new(int64(123)),
							ActorType:  "Team",
							BypassMode: "always",
						},
					},
					Rules: githubv1alpha1.RulesetRules{
						Creation:              new(true),
						Update:                new(false),
						Deletion:              new(true),
						RequiredLinearHistory: new(true),
						RequiredSignatures:    new(false),
						NonFastForward:        new(true),
						PullRequest: &githubv1alpha1.PullRequestRule{
							DismissStaleReviewsOnPush:      new(true),
							RequireCodeOwnerReviews:        new(true),
							RequireLastPushApproval:        new(false),
							RequiredApprovingReviewCount:   2,
							RequiredReviewThreadResolution: new(true),
						},
						RequiredStatusChecks: &githubv1alpha1.RequiredStatusChecks{
							Checks: []githubv1alpha1.StatusCheck{
								{
									Context:       "ci/build",
									IntegrationID: new(int64(456)),
								},
								{
									Context: "ci/test",
								},
							},
							StrictPolicy: new(true),
						},
						CopilotReview: &githubv1alpha1.CopilotCodeReviewRule{
							ReviewOnPush:            new(true),
							ReviewDraftPullRequests: new(false),
						},
						CommitMessagePattern: &githubv1alpha1.PatternRule{
							Pattern:  "^(feat|fix|docs):",
							Operator: "regex",
							Negate:   new(false),
						},
						BranchNamePattern: &githubv1alpha1.PatternRule{
							Pattern:  "hotfix/",
							Operator: "starts_with",
							Negate:   new(true),
						},
					},
				},
			}
		})

		It("should successfully convert a complete ruleset preset", func() {
			githubRuleset, err := RulesetPresetToGithubRuleset(rulesetPreset)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying basic properties")
			Expect(githubRuleset.Name).To(Equal("main-protection"))
			Expect(githubRuleset.Enforcement).To(Equal(github.RulesetEnforcementActive))

			By("Verifying conditions")
			Expect(githubRuleset.Conditions).NotTo(BeNil())
			Expect(githubRuleset.Conditions.RefName).NotTo(BeNil())
			Expect(githubRuleset.Conditions.RefName.Include).To(Equal([]string{"main", "develop"}))
			Expect(githubRuleset.Conditions.RefName.Exclude).To(Equal([]string{"feature/*"}))

			By("Verifying bypass actors")
			Expect(githubRuleset.BypassActors).To(HaveLen(1))
			Expect(*githubRuleset.BypassActors[0].ActorID).To(Equal(int64(123)))
			Expect(*githubRuleset.BypassActors[0].ActorType).To(Equal(github.BypassActorType("Team")))
			Expect(*githubRuleset.BypassActors[0].BypassMode).To(Equal(github.BypassMode("always")))

			By("Verifying rules")
			Expect(githubRuleset.Rules).NotTo(BeNil())

			// Check for creation rule
			Expect(hasRule(githubRuleset.Rules, "creation")).To(BeTrue())

			// Check for deletion rule
			Expect(hasRule(githubRuleset.Rules, "deletion")).To(BeTrue())

			// Check for required linear history rule
			Expect(hasRule(githubRuleset.Rules, "required_linear_history")).To(BeTrue())

			// Check for non fast forward rule
			Expect(hasRule(githubRuleset.Rules, "non_fast_forward")).To(BeTrue())

			// Check for copilot code review rule
			Expect(hasRule(githubRuleset.Rules, "copilot_code_review")).To(BeTrue())
			Expect(githubRuleset.Rules.CopilotCodeReview.ReviewOnPush).To(BeTrue())
			Expect(githubRuleset.Rules.CopilotCodeReview.ReviewDraftPullRequests).To(BeFalse())

			// Check for pull request rule
			Expect(hasRule(githubRuleset.Rules, "pull_request")).To(BeTrue())
			Expect(githubRuleset.Rules.PullRequest.RequiredApprovingReviewCount).To(Equal(2))

			// Check for required status checks rule
			Expect(hasRule(githubRuleset.Rules, "required_status_checks")).To(BeTrue())
			Expect(githubRuleset.Rules.RequiredStatusChecks.RequiredStatusChecks).To(HaveLen(2))

			// Check for pattern rules
			Expect(hasRule(githubRuleset.Rules, "commit_message_pattern")).To(BeTrue())
			Expect(githubRuleset.Rules.CommitMessagePattern.Pattern).To(Equal("^(feat|fix|docs):"))

			Expect(hasRule(githubRuleset.Rules, "branch_name_pattern")).To(BeTrue())
			Expect(githubRuleset.Rules.BranchNamePattern.Negate).To(Equal(new(true)))
		})

		It("should convert CopilotCodeReview rule", func() {
			copilotPreset := githubv1alpha1.RulesetPreset{
				Spec: githubv1alpha1.RulesetPresetSpec{
					Name:        "copilot-review-ruleset",
					Enforcement: githubv1alpha1.RulesetEnforcementActive,
					Conditions: &githubv1alpha1.RulesetConditions{
						RefName: &githubv1alpha1.RefNameCondition{
							Include: []string{"main"},
						},
					},
					Rules: githubv1alpha1.RulesetRules{
						CopilotReview: &githubv1alpha1.CopilotCodeReviewRule{
							ReviewOnPush:            new(true),
							ReviewDraftPullRequests: new(false),
						},
					},
				},
			}

			githubRuleset, err := RulesetPresetToGithubRuleset(copilotPreset)
			Expect(err).NotTo(HaveOccurred())

			Expect(hasRule(githubRuleset.Rules, "copilot_code_review")).To(BeTrue())
			Expect(githubRuleset.Rules.CopilotCodeReview).NotTo(BeNil())
			Expect(githubRuleset.Rules.CopilotCodeReview.ReviewOnPush).To(BeTrue())
			Expect(githubRuleset.Rules.CopilotCodeReview.ReviewDraftPullRequests).To(BeFalse())
		})

		It("should convert CopilotCodeReview rule with both options enabled", func() {
			copilotPreset := githubv1alpha1.RulesetPreset{
				Spec: githubv1alpha1.RulesetPresetSpec{
					Name:        "copilot-review-full",
					Enforcement: githubv1alpha1.RulesetEnforcementActive,
					Conditions: &githubv1alpha1.RulesetConditions{
						RefName: &githubv1alpha1.RefNameCondition{
							Include: []string{"main"},
						},
					},
					Rules: githubv1alpha1.RulesetRules{
						CopilotReview: &githubv1alpha1.CopilotCodeReviewRule{
							ReviewOnPush:            new(true),
							ReviewDraftPullRequests: new(true),
						},
					},
				},
			}

			githubRuleset, err := RulesetPresetToGithubRuleset(copilotPreset)
			Expect(err).NotTo(HaveOccurred())

			Expect(githubRuleset.Rules.CopilotCodeReview).NotTo(BeNil())
			Expect(githubRuleset.Rules.CopilotCodeReview.ReviewOnPush).To(BeTrue())
			Expect(githubRuleset.Rules.CopilotCodeReview.ReviewDraftPullRequests).To(BeTrue())
		})

		It("should handle nil CopilotCodeReview rule", func() {
			noCopilotPreset := githubv1alpha1.RulesetPreset{
				Spec: githubv1alpha1.RulesetPresetSpec{
					Name:        "no-copilot-ruleset",
					Enforcement: githubv1alpha1.RulesetEnforcementActive,
					Conditions: &githubv1alpha1.RulesetConditions{
						RefName: &githubv1alpha1.RefNameCondition{
							Include: []string{"main"},
						},
					},
					Rules: githubv1alpha1.RulesetRules{
						RequiredLinearHistory: new(true),
					},
				},
			}

			githubRuleset, err := RulesetPresetToGithubRuleset(noCopilotPreset)
			Expect(err).NotTo(HaveOccurred())

			Expect(githubRuleset.Rules.CopilotCodeReview).To(BeNil())
		})

		It("should handle minimal ruleset preset", func() {
			minimalPreset := githubv1alpha1.RulesetPreset{
				Spec: githubv1alpha1.RulesetPresetSpec{
					Name:        "minimal-ruleset",
					Enforcement: githubv1alpha1.RulesetEnforcementEvaluate,
					Conditions: &githubv1alpha1.RulesetConditions{
						RefName: &githubv1alpha1.RefNameCondition{
							Include: []string{"main"},
						},
					},
					Rules: githubv1alpha1.RulesetRules{
						RequiredLinearHistory: new(true),
					},
				},
			}

			githubRuleset, err := RulesetPresetToGithubRuleset(minimalPreset)
			Expect(err).NotTo(HaveOccurred())

			Expect(githubRuleset.Name).To(Equal("minimal-ruleset"))
			Expect(githubRuleset.Enforcement).To(Equal(github.RulesetEnforcementEvaluate))
			Expect(githubRuleset.Rules).NotTo(BeNil())

			Expect(hasRule(githubRuleset.Rules, "required_linear_history")).To(BeTrue())
		})

		It("should return error when name is empty", func() {
			emptyNamePreset := githubv1alpha1.RulesetPreset{
				Spec: githubv1alpha1.RulesetPresetSpec{
					Name:        "",
					Enforcement: githubv1alpha1.RulesetEnforcementActive,
					Conditions: &githubv1alpha1.RulesetConditions{
						RefName: &githubv1alpha1.RefNameCondition{
							Include: []string{"main"},
						},
					},
					Rules: githubv1alpha1.RulesetRules{},
				},
			}

			_, err := RulesetPresetToGithubRuleset(emptyNamePreset)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ruleset name cannot be empty"))
		})

		Context("Conditions mapping", func() {
			It("should map RepositoryName conditions", func() {
				preset := githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "org-ruleset-with-repo-name",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Target:      "repository",
						Conditions: &githubv1alpha1.RulesetConditions{
							RefName: &githubv1alpha1.RefNameCondition{
								Include: []string{"~ALL"},
							},
							RepositoryName: &githubv1alpha1.RepositoryNameCondition{
								Include:   []string{"backend-*", "frontend-*"},
								Exclude:   []string{"legacy-*"},
								Protected: new(true),
							},
						},
						Rules: githubv1alpha1.RulesetRules{
							RequiredLinearHistory: new(true),
						},
					},
				}

				githubRuleset, err := RulesetPresetToGithubRuleset(preset)
				Expect(err).NotTo(HaveOccurred())
				Expect(githubRuleset.Conditions).NotTo(BeNil())
				Expect(githubRuleset.Conditions.RefName).NotTo(BeNil())
				Expect(githubRuleset.Conditions.RefName.Include).To(Equal([]string{"~ALL"}))
				Expect(githubRuleset.Conditions.RepositoryName).NotTo(BeNil())
				Expect(githubRuleset.Conditions.RepositoryName.Include).To(Equal([]string{"backend-*", "frontend-*"}))
				Expect(githubRuleset.Conditions.RepositoryName.Exclude).To(Equal([]string{"legacy-*"}))
				Expect(githubRuleset.Conditions.RepositoryName.Protected).To(Equal(new(true)))
				Expect(githubRuleset.Conditions.RepositoryProperty).To(BeNil())
			})

			It("should map RepositoryProperty conditions", func() {
				preset := githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "org-ruleset-with-repo-property",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Target:      "repository",
						Conditions: &githubv1alpha1.RulesetConditions{
							RefName: &githubv1alpha1.RefNameCondition{
								Include: []string{"~ALL"},
							},
							RepositoryProperty: &githubv1alpha1.RepositoryPropertyCondition{
								Include: []githubv1alpha1.RepositoryPropertyTarget{
									{Name: "environment", PropertyValues: []string{"production", "staging"}, Source: new("custom")},
								},
								Exclude: []githubv1alpha1.RepositoryPropertyTarget{
									{Name: "archived", PropertyValues: []string{"true"}},
								},
							},
						},
						Rules: githubv1alpha1.RulesetRules{
							RequiredLinearHistory: new(true),
						},
					},
				}

				githubRuleset, err := RulesetPresetToGithubRuleset(preset)
				Expect(err).NotTo(HaveOccurred())
				Expect(githubRuleset.Conditions).NotTo(BeNil())
				Expect(githubRuleset.Conditions.RefName).NotTo(BeNil())
				Expect(githubRuleset.Conditions.RepositoryName).To(BeNil())
				Expect(githubRuleset.Conditions.RepositoryProperty).NotTo(BeNil())
				Expect(githubRuleset.Conditions.RepositoryProperty.Include).To(HaveLen(1))
				Expect(githubRuleset.Conditions.RepositoryProperty.Include[0].Name).To(Equal("environment"))
				Expect(githubRuleset.Conditions.RepositoryProperty.Include[0].PropertyValues).To(Equal([]string{"production", "staging"}))
				Expect(githubRuleset.Conditions.RepositoryProperty.Include[0].Source).To(Equal(new("custom")))
				Expect(githubRuleset.Conditions.RepositoryProperty.Exclude).To(HaveLen(1))
				Expect(githubRuleset.Conditions.RepositoryProperty.Exclude[0].Name).To(Equal("archived"))
				Expect(githubRuleset.Conditions.RepositoryProperty.Exclude[0].PropertyValues).To(Equal([]string{"true"}))
			})

			It("should map only RefName when no RepositoryName or RepositoryProperty is set", func() {
				preset := githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "simple-conditions",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Conditions: &githubv1alpha1.RulesetConditions{
							RefName: &githubv1alpha1.RefNameCondition{
								Include: []string{"main"},
								Exclude: []string{"temp/*"},
							},
						},
						Rules: githubv1alpha1.RulesetRules{
							RequiredLinearHistory: new(true),
						},
					},
				}

				githubRuleset, err := RulesetPresetToGithubRuleset(preset)
				Expect(err).NotTo(HaveOccurred())
				Expect(githubRuleset.Conditions).NotTo(BeNil())
				Expect(githubRuleset.Conditions.RefName).NotTo(BeNil())
				Expect(githubRuleset.Conditions.RepositoryName).To(BeNil())
				Expect(githubRuleset.Conditions.RepositoryProperty).To(BeNil())
			})

			It("should not map RepositoryProperty when RepositoryName is set (mutually exclusive)", func() {
				preset := githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "repo-name-takes-precedence",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Target:      "repository",
						Conditions: &githubv1alpha1.RulesetConditions{
							RefName: &githubv1alpha1.RefNameCondition{
								Include: []string{"~ALL"},
							},
							RepositoryName: &githubv1alpha1.RepositoryNameCondition{
								Include: []string{"~ALL"},
							},
							RepositoryProperty: &githubv1alpha1.RepositoryPropertyCondition{
								Include: []githubv1alpha1.RepositoryPropertyTarget{
									{Name: "team", PropertyValues: []string{"platform"}},
								},
							},
						},
						Rules: githubv1alpha1.RulesetRules{
							RequiredLinearHistory: new(true),
						},
					},
				}

				githubRuleset, err := RulesetPresetToGithubRuleset(preset)
				Expect(err).NotTo(HaveOccurred())
				// mapConditions uses else-if: RepositoryName takes precedence over RepositoryProperty
				Expect(githubRuleset.Conditions.RepositoryName).NotTo(BeNil())
				Expect(githubRuleset.Conditions.RepositoryProperty).To(BeNil())
			})

			It("should map RepositoryName with nil Protected field", func() {
				preset := githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "repo-name-no-protected",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Target:      "repository",
						Conditions: &githubv1alpha1.RulesetConditions{
							RefName: &githubv1alpha1.RefNameCondition{
								Include: []string{"~ALL"},
							},
							RepositoryName: &githubv1alpha1.RepositoryNameCondition{
								Include: []string{"my-repo"},
							},
						},
						Rules: githubv1alpha1.RulesetRules{
							RequiredLinearHistory: new(true),
						},
					},
				}

				githubRuleset, err := RulesetPresetToGithubRuleset(preset)
				Expect(err).NotTo(HaveOccurred())
				Expect(githubRuleset.Conditions.RepositoryName).NotTo(BeNil())
				Expect(githubRuleset.Conditions.RepositoryName.Include).To(Equal([]string{"my-repo"}))
				Expect(githubRuleset.Conditions.RepositoryName.Exclude).To(BeEmpty())
				Expect(githubRuleset.Conditions.RepositoryName.Protected).To(BeNil())
			})

			It("should map RepositoryProperty with multiple include and exclude targets", func() {
				preset := githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "multi-property-targets",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Target:      "repository",
						Conditions: &githubv1alpha1.RulesetConditions{
							RefName: &githubv1alpha1.RefNameCondition{
								Include: []string{"~ALL"},
							},
							RepositoryProperty: &githubv1alpha1.RepositoryPropertyCondition{
								Include: []githubv1alpha1.RepositoryPropertyTarget{
									{Name: "team", PropertyValues: []string{"platform", "backend"}},
									{Name: "tier", PropertyValues: []string{"critical"}},
								},
								Exclude: []githubv1alpha1.RepositoryPropertyTarget{
									{Name: "deprecated", PropertyValues: []string{"true"}},
								},
							},
						},
						Rules: githubv1alpha1.RulesetRules{
							RequiredLinearHistory: new(true),
						},
					},
				}

				githubRuleset, err := RulesetPresetToGithubRuleset(preset)
				Expect(err).NotTo(HaveOccurred())
				Expect(githubRuleset.Conditions.RepositoryProperty).NotTo(BeNil())
				Expect(githubRuleset.Conditions.RepositoryProperty.Include).To(HaveLen(2))
				Expect(githubRuleset.Conditions.RepositoryProperty.Include[0].Name).To(Equal("team"))
				Expect(githubRuleset.Conditions.RepositoryProperty.Include[0].PropertyValues).To(Equal([]string{"platform", "backend"}))
				Expect(githubRuleset.Conditions.RepositoryProperty.Include[1].Name).To(Equal("tier"))
				Expect(githubRuleset.Conditions.RepositoryProperty.Exclude).To(HaveLen(1))
				Expect(githubRuleset.Conditions.RepositoryProperty.Exclude[0].Name).To(Equal("deprecated"))
			})
		})

		Context("Target Type handling", func() {
			It("should set target type to branch when explicitly specified", func() {
				branchPreset := githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "branch-ruleset",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Target:      TargetTypeBranch,
						Conditions: &githubv1alpha1.RulesetConditions{
							RefName: &githubv1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: githubv1alpha1.RulesetRules{
							RequiredLinearHistory: new(true),
						},
					},
				}

				githubRuleset, err := RulesetPresetToGithubRuleset(branchPreset)
				Expect(err).NotTo(HaveOccurred())
				Expect(githubRuleset.Target).NotTo(BeNil())
				Expect(*githubRuleset.Target).To(Equal(github.RulesetTargetBranch))
			})

			It("should set target type to tag when specified", func() {
				tagPreset := githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "tag-ruleset",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Target:      TargetTypeTag,
						Conditions: &githubv1alpha1.RulesetConditions{
							RefName: &githubv1alpha1.RefNameCondition{
								Include: []string{"v*"},
							},
						},
						Rules: githubv1alpha1.RulesetRules{
							RequiredSignatures: new(true),
						},
					},
				}

				githubRuleset, err := RulesetPresetToGithubRuleset(tagPreset)
				Expect(err).NotTo(HaveOccurred())
				Expect(githubRuleset.Target).NotTo(BeNil())
				Expect(*githubRuleset.Target).To(Equal(github.RulesetTargetTag))
			})

			It("should set target type to push when specified", func() {
				pushPreset := githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "push-ruleset",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Target:      "push",
						Conditions: &githubv1alpha1.RulesetConditions{
							RefName: &githubv1alpha1.RefNameCondition{
								Include: []string{"~ALL"},
							},
						},
						Rules: githubv1alpha1.RulesetRules{
							Creation: new(true),
						},
					},
				}

				githubRuleset, err := RulesetPresetToGithubRuleset(pushPreset)
				Expect(err).NotTo(HaveOccurred())
				Expect(githubRuleset.Target).NotTo(BeNil())
				Expect(*githubRuleset.Target).To(Equal(github.RulesetTargetPush))
			})

			It("should handle empty target type gracefully", func() {
				emptyTargetTypePreset := githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "default-target-ruleset",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Target:      "", // Empty target type
						Conditions: &githubv1alpha1.RulesetConditions{
							RefName: &githubv1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: githubv1alpha1.RulesetRules{
							RequiredLinearHistory: new(true),
						},
					},
				}

				githubRuleset, err := RulesetPresetToGithubRuleset(emptyTargetTypePreset)
				Expect(err).NotTo(HaveOccurred())
				Expect(githubRuleset.Target).NotTo(BeNil())
				// Empty string converts to empty RulesetTarget, which GitHub treats as branch by default
			})

			It("should work with tag target type and tag name patterns", func() {
				tagWithPatternPreset := githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "tag-protection",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Target:      TargetTypeTag,
						Conditions: &githubv1alpha1.RulesetConditions{
							RefName: &githubv1alpha1.RefNameCondition{
								Include: []string{"v*", "release-*"},
								Exclude: []string{"*-beta"},
							},
						},
						Rules: githubv1alpha1.RulesetRules{
							Creation:           new(true),
							Deletion:           new(true),
							RequiredSignatures: new(true),
							TagNamePattern: &githubv1alpha1.PatternRule{
								Pattern:  "^v[0-9]+\\.[0-9]+\\.[0-9]+$",
								Operator: "regex",
								Negate:   new(false),
							},
						},
					},
				}

				githubRuleset, err := RulesetPresetToGithubRuleset(tagWithPatternPreset)
				Expect(err).NotTo(HaveOccurred())
				Expect(githubRuleset.Target).NotTo(BeNil())
				Expect(*githubRuleset.Target).To(Equal(github.RulesetTargetTag))
				Expect(githubRuleset.Conditions.RefName.Include).To(ConsistOf("v*", "release-*"))
				Expect(githubRuleset.Conditions.RefName.Exclude).To(ConsistOf("*-beta"))
				Expect(hasRule(githubRuleset.Rules, "tag_name_pattern")).To(BeTrue())
			})
		})
	})

	Context("RulesetsDiffer", func() {
		var rulesetPreset githubv1alpha1.RulesetPreset
		var githubRuleset github.RepositoryRuleset

		BeforeEach(func() {
			rulesetPreset = githubv1alpha1.RulesetPreset{
				Spec: githubv1alpha1.RulesetPresetSpec{
					Name:        "test-ruleset",
					Enforcement: githubv1alpha1.RulesetEnforcementActive,
					Target:      TargetTypeBranch, // Set default target type
					BypassActors: []githubv1alpha1.RulesetBypassActor{
						{ActorID: new(int64(123)), ActorType: "Team"},
					},
					Conditions: &githubv1alpha1.RulesetConditions{
						RefName: &githubv1alpha1.RefNameCondition{
							Include: []string{"main"},
						},
					},
					Rules: githubv1alpha1.RulesetRules{
						Creation: new(true),
						PullRequest: &githubv1alpha1.PullRequestRule{
							RequiredApprovingReviewCount: 2,
						},
						RequiredStatusChecks: &githubv1alpha1.RequiredStatusChecks{
							Checks: []githubv1alpha1.StatusCheck{
								{Context: "ci/test"},
							},
						},
						CommitMessagePattern: &githubv1alpha1.PatternRule{
							Pattern:  "^(feat|fix):",
							Operator: "starts_with",
						},
					},
				},
			}

			actorType := github.BypassActorType("Team")
			githubRuleset = github.RepositoryRuleset{
				Name:        "test-ruleset",
				Enforcement: github.RulesetEnforcementActive,
				Target:      github.Ptr(github.RulesetTargetBranch), // Set target type
				BypassActors: []*github.BypassActor{
					{ActorID: new(int64(123)), ActorType: &actorType},
				},
				Conditions: &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"main"},
					},
				},
				Rules: &github.RepositoryRulesetRules{
					Creation: &github.EmptyRuleParameters{},
					PullRequest: &github.PullRequestRuleParameters{
						RequiredApprovingReviewCount: 2,
					},
					RequiredStatusChecks: &github.RequiredStatusChecksRuleParameters{
						RequiredStatusChecks: []*github.RuleStatusCheck{
							{Context: "ci/test"},
						},
					},
					CommitMessagePattern: &github.PatternRuleParameters{
						Pattern:  "^(feat|fix):",
						Operator: github.PatternRuleOperatorStartsWith,
					},
				},
			}
		})

		It("should return false when rulesets match", func() {
			differs := RulesetsDiffer(rulesetPreset, githubRuleset)
			Expect(differs).To(BeFalse())
		})

		It("should return true when names differ", func() {
			githubRuleset.Name = "different-name"
			differs := RulesetsDiffer(rulesetPreset, githubRuleset)
			Expect(differs).To(BeTrue())
		})

		It("should return true when enforcement differs", func() {
			githubRuleset.Enforcement = github.RulesetEnforcementEvaluate
			differs := RulesetsDiffer(rulesetPreset, githubRuleset)
			Expect(differs).To(BeTrue())
		})

		It("should return true when bypass actors count differs", func() {
			githubRuleset.BypassActors = []*github.BypassActor{}
			differs := RulesetsDiffer(rulesetPreset, githubRuleset)
			Expect(differs).To(BeTrue())
		})

		It("should return true when bypass actor properties differ", func() {
			githubRuleset.BypassActors[0].ActorType = github.Ptr(github.BypassActorTypeIntegration)
			differs := RulesetsDiffer(rulesetPreset, githubRuleset)
			Expect(differs).To(BeTrue())
		})

		It("should return true when target conditions differ", func() {
			githubRuleset.Conditions.RefName.Include = []string{"different-branch"}
			differs := RulesetsDiffer(rulesetPreset, githubRuleset)
			Expect(differs).To(BeTrue())
		})

		It("should return true when boolean rules differ", func() {
			githubRuleset.Rules.Creation = nil // Remove creation rule
			differs := RulesetsDiffer(rulesetPreset, githubRuleset)
			Expect(differs).To(BeTrue())
		})

		It("should return true when pull request rules differ", func() {
			githubRuleset.Rules.PullRequest.RequiredApprovingReviewCount = 5 // Different count
			differs := RulesetsDiffer(rulesetPreset, githubRuleset)
			Expect(differs).To(BeTrue())
		})

		It("should return true when status checks differ", func() {
			githubRuleset.Rules.RequiredStatusChecks.RequiredStatusChecks[0].Context = "different-context"
			differs := RulesetsDiffer(rulesetPreset, githubRuleset)
			Expect(differs).To(BeTrue())
		})

		It("should return true when pattern rules differ", func() {
			githubRuleset.Rules.CommitMessagePattern.Pattern = "different-pattern"
			differs := RulesetsDiffer(rulesetPreset, githubRuleset)
			Expect(differs).To(BeTrue())
		})

		It("should return false when CopilotCodeReview rules match", func() {
			rulesetPreset.Spec.Rules.CopilotReview = &githubv1alpha1.CopilotCodeReviewRule{
				ReviewOnPush:            new(true),
				ReviewDraftPullRequests: new(false),
			}
			githubRuleset.Rules.CopilotCodeReview = &github.CopilotCodeReviewRuleParameters{
				ReviewOnPush:            true,
				ReviewDraftPullRequests: false,
			}

			differs := RulesetsDiffer(rulesetPreset, githubRuleset)
			Expect(differs).To(BeFalse())
		})

		It("should return true when CopilotCodeReview ReviewOnPush differs", func() {
			rulesetPreset.Spec.Rules.CopilotReview = &githubv1alpha1.CopilotCodeReviewRule{
				ReviewOnPush:            new(true),
				ReviewDraftPullRequests: new(false),
			}
			githubRuleset.Rules.CopilotCodeReview = &github.CopilotCodeReviewRuleParameters{
				ReviewOnPush:            false, // Different
				ReviewDraftPullRequests: false,
			}

			differs := RulesetsDiffer(rulesetPreset, githubRuleset)
			Expect(differs).To(BeTrue())
		})

		It("should return true when CopilotCodeReview ReviewDraftPullRequests differs", func() {
			rulesetPreset.Spec.Rules.CopilotReview = &githubv1alpha1.CopilotCodeReviewRule{
				ReviewOnPush:            new(true),
				ReviewDraftPullRequests: new(false),
			}
			githubRuleset.Rules.CopilotCodeReview = &github.CopilotCodeReviewRuleParameters{
				ReviewOnPush:            true,
				ReviewDraftPullRequests: true, // Different
			}

			differs := RulesetsDiffer(rulesetPreset, githubRuleset)
			Expect(differs).To(BeTrue())
		})

		It("should return true when CopilotCodeReview is present in preset but missing in GitHub", func() {
			rulesetPreset.Spec.Rules.CopilotReview = &githubv1alpha1.CopilotCodeReviewRule{
				ReviewOnPush:            new(true),
				ReviewDraftPullRequests: new(false),
			}
			githubRuleset.Rules.CopilotCodeReview = nil

			differs := RulesetsDiffer(rulesetPreset, githubRuleset)
			Expect(differs).To(BeTrue())
		})

		It("should return true when CopilotCodeReview is missing in preset but present in GitHub", func() {
			rulesetPreset.Spec.Rules.CopilotReview = nil
			githubRuleset.Rules.CopilotCodeReview = &github.CopilotCodeReviewRuleParameters{
				ReviewOnPush:            true,
				ReviewDraftPullRequests: false,
			}

			differs := RulesetsDiffer(rulesetPreset, githubRuleset)
			Expect(differs).To(BeTrue())
		})

		It("should return false when CopilotCodeReview is nil in both", func() {
			rulesetPreset.Spec.Rules.CopilotReview = nil
			githubRuleset.Rules.CopilotCodeReview = nil

			differs := RulesetsDiffer(rulesetPreset, githubRuleset)
			Expect(differs).To(BeFalse())
		})

		It("should handle nil GitHub conditions", func() {
			githubRuleset.Conditions = nil
			differs := RulesetsDiffer(rulesetPreset, githubRuleset)
			Expect(differs).To(BeTrue()) // Should differ since preset has target conditions
		})

		It("should handle nil GitHub rules", func() {
			githubRuleset.Rules = nil
			differs := RulesetsDiffer(rulesetPreset, githubRuleset)
			Expect(differs).To(BeTrue()) // Should differ since preset has rules
		})

		It("should handle empty target conditions", func() {
			// Create preset with no conditions
			emptyTargetPreset := rulesetPreset
			emptyTargetPreset.Spec.Conditions = nil

			githubRuleset.Conditions = nil
			differs := RulesetsDiffer(emptyTargetPreset, githubRuleset)
			Expect(differs).To(BeFalse()) // Should not differ since both have no conditions
		})

		Context("when comparing RepositoryName conditions", func() {
			It("should return false when RepositoryName conditions match", func() {
				rulesetPreset.Spec.Conditions = &githubv1alpha1.RulesetConditions{
					RefName: &githubv1alpha1.RefNameCondition{
						Include: []string{"~ALL"},
					},
					RepositoryName: &githubv1alpha1.RepositoryNameCondition{
						Include: []string{"backend-*", "frontend-*"},
						Exclude: []string{"legacy-*"},
					},
				}
				githubRuleset.Conditions = &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"~ALL"},
					},
					RepositoryName: &github.RepositoryRulesetRepositoryNamesConditionParameters{
						Include: []string{"backend-*", "frontend-*"},
						Exclude: []string{"legacy-*"},
					},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeFalse())
			})

			It("should return true when RepositoryName include differs", func() {
				rulesetPreset.Spec.Conditions = &githubv1alpha1.RulesetConditions{
					RefName: &githubv1alpha1.RefNameCondition{
						Include: []string{"~ALL"},
					},
					RepositoryName: &githubv1alpha1.RepositoryNameCondition{
						Include: []string{"backend-*"},
					},
				}
				githubRuleset.Conditions = &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"~ALL"},
					},
					RepositoryName: &github.RepositoryRulesetRepositoryNamesConditionParameters{
						Include: []string{"frontend-*"},
					},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})

			It("should return true when RepositoryName exclude differs", func() {
				rulesetPreset.Spec.Conditions = &githubv1alpha1.RulesetConditions{
					RefName: &githubv1alpha1.RefNameCondition{
						Include: []string{"~ALL"},
					},
					RepositoryName: &githubv1alpha1.RepositoryNameCondition{
						Include: []string{"~ALL"},
						Exclude: []string{"legacy-*"},
					},
				}
				githubRuleset.Conditions = &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"~ALL"},
					},
					RepositoryName: &github.RepositoryRulesetRepositoryNamesConditionParameters{
						Include: []string{"~ALL"},
						Exclude: []string{"archive-*"},
					},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})

			It("should return true when preset has RepositoryName but GitHub does not", func() {
				rulesetPreset.Spec.Conditions = &githubv1alpha1.RulesetConditions{
					RefName: &githubv1alpha1.RefNameCondition{
						Include: []string{"~ALL"},
					},
					RepositoryName: &githubv1alpha1.RepositoryNameCondition{
						Include: []string{"~ALL"},
					},
				}
				githubRuleset.Conditions = &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"~ALL"},
					},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})

			It("should return true when GitHub has RepositoryName but preset does not", func() {
				rulesetPreset.Spec.Conditions = &githubv1alpha1.RulesetConditions{
					RefName: &githubv1alpha1.RefNameCondition{
						Include: []string{"~ALL"},
					},
				}
				githubRuleset.Conditions = &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"~ALL"},
					},
					RepositoryName: &github.RepositoryRulesetRepositoryNamesConditionParameters{
						Include: []string{"~ALL"},
					},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})

			It("should return false when both have nil RepositoryName", func() {
				rulesetPreset.Spec.Conditions = &githubv1alpha1.RulesetConditions{
					RefName: &githubv1alpha1.RefNameCondition{
						Include: []string{"main"},
					},
				}
				githubRuleset.Conditions = &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"main"},
					},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeFalse())
			})
		})

		Context("when comparing RepositoryProperty conditions", func() {
			It("should return false when RepositoryProperty conditions match", func() {
				rulesetPreset.Spec.Conditions = &githubv1alpha1.RulesetConditions{
					RefName: &githubv1alpha1.RefNameCondition{
						Include: []string{"~ALL"},
					},
					RepositoryProperty: &githubv1alpha1.RepositoryPropertyCondition{
						Include: []githubv1alpha1.RepositoryPropertyTarget{
							{Name: "environment", PropertyValues: []string{"production"}},
						},
						Exclude: []githubv1alpha1.RepositoryPropertyTarget{
							{Name: "archived", PropertyValues: []string{"true"}},
						},
					},
				}
				githubRuleset.Conditions = &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"~ALL"},
					},
					RepositoryProperty: &github.RepositoryRulesetRepositoryPropertyConditionParameters{
						Include: []*github.RepositoryRulesetRepositoryPropertyTargetParameters{
							{Name: "environment", PropertyValues: []string{"production"}},
						},
						Exclude: []*github.RepositoryRulesetRepositoryPropertyTargetParameters{
							{Name: "archived", PropertyValues: []string{"true"}},
						},
					},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeFalse())
			})

			It("should return true when RepositoryProperty include count differs", func() {
				rulesetPreset.Spec.Conditions = &githubv1alpha1.RulesetConditions{
					RefName: &githubv1alpha1.RefNameCondition{
						Include: []string{"~ALL"},
					},
					RepositoryProperty: &githubv1alpha1.RepositoryPropertyCondition{
						Include: []githubv1alpha1.RepositoryPropertyTarget{
							{Name: "environment", PropertyValues: []string{"production"}},
							{Name: "tier", PropertyValues: []string{"critical"}},
						},
					},
				}
				githubRuleset.Conditions = &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"~ALL"},
					},
					RepositoryProperty: &github.RepositoryRulesetRepositoryPropertyConditionParameters{
						Include: []*github.RepositoryRulesetRepositoryPropertyTargetParameters{
							{Name: "environment", PropertyValues: []string{"production"}},
						},
					},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})

			It("should return true when RepositoryProperty property values differ", func() {
				rulesetPreset.Spec.Conditions = &githubv1alpha1.RulesetConditions{
					RefName: &githubv1alpha1.RefNameCondition{
						Include: []string{"~ALL"},
					},
					RepositoryProperty: &githubv1alpha1.RepositoryPropertyCondition{
						Include: []githubv1alpha1.RepositoryPropertyTarget{
							{Name: "environment", PropertyValues: []string{"production"}},
						},
					},
				}
				githubRuleset.Conditions = &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"~ALL"},
					},
					RepositoryProperty: &github.RepositoryRulesetRepositoryPropertyConditionParameters{
						Include: []*github.RepositoryRulesetRepositoryPropertyTargetParameters{
							{Name: "environment", PropertyValues: []string{"staging"}},
						},
					},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})

			It("should return true when RepositoryProperty property names differ", func() {
				rulesetPreset.Spec.Conditions = &githubv1alpha1.RulesetConditions{
					RefName: &githubv1alpha1.RefNameCondition{
						Include: []string{"~ALL"},
					},
					RepositoryProperty: &githubv1alpha1.RepositoryPropertyCondition{
						Include: []githubv1alpha1.RepositoryPropertyTarget{
							{Name: "environment", PropertyValues: []string{"production"}},
						},
					},
				}
				githubRuleset.Conditions = &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"~ALL"},
					},
					RepositoryProperty: &github.RepositoryRulesetRepositoryPropertyConditionParameters{
						Include: []*github.RepositoryRulesetRepositoryPropertyTargetParameters{
							{Name: "team", PropertyValues: []string{"production"}},
						},
					},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})

			It("should return true when preset has RepositoryProperty but GitHub does not", func() {
				rulesetPreset.Spec.Conditions = &githubv1alpha1.RulesetConditions{
					RefName: &githubv1alpha1.RefNameCondition{
						Include: []string{"~ALL"},
					},
					RepositoryProperty: &githubv1alpha1.RepositoryPropertyCondition{
						Include: []githubv1alpha1.RepositoryPropertyTarget{
							{Name: "environment", PropertyValues: []string{"production"}},
						},
					},
				}
				githubRuleset.Conditions = &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"~ALL"},
					},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})

			It("should return true when GitHub has RepositoryProperty but preset does not", func() {
				rulesetPreset.Spec.Conditions = &githubv1alpha1.RulesetConditions{
					RefName: &githubv1alpha1.RefNameCondition{
						Include: []string{"~ALL"},
					},
				}
				githubRuleset.Conditions = &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"~ALL"},
					},
					RepositoryProperty: &github.RepositoryRulesetRepositoryPropertyConditionParameters{
						Include: []*github.RepositoryRulesetRepositoryPropertyTargetParameters{
							{Name: "environment", PropertyValues: []string{"production"}},
						},
					},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})

			It("should return true when RepositoryProperty exclude differs", func() {
				rulesetPreset.Spec.Conditions = &githubv1alpha1.RulesetConditions{
					RefName: &githubv1alpha1.RefNameCondition{
						Include: []string{"~ALL"},
					},
					RepositoryProperty: &githubv1alpha1.RepositoryPropertyCondition{
						Include: []githubv1alpha1.RepositoryPropertyTarget{
							{Name: "environment", PropertyValues: []string{"production"}},
						},
						Exclude: []githubv1alpha1.RepositoryPropertyTarget{
							{Name: "archived", PropertyValues: []string{"true"}},
						},
					},
				}
				githubRuleset.Conditions = &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"~ALL"},
					},
					RepositoryProperty: &github.RepositoryRulesetRepositoryPropertyConditionParameters{
						Include: []*github.RepositoryRulesetRepositoryPropertyTargetParameters{
							{Name: "environment", PropertyValues: []string{"production"}},
						},
						Exclude: []*github.RepositoryRulesetRepositoryPropertyTargetParameters{
							{Name: "deprecated", PropertyValues: []string{"true"}},
						},
					},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})

			It("should return false when both have nil RepositoryProperty", func() {
				rulesetPreset.Spec.Conditions = &githubv1alpha1.RulesetConditions{
					RefName: &githubv1alpha1.RefNameCondition{
						Include: []string{"main"},
					},
				}
				githubRuleset.Conditions = &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"main"},
					},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeFalse())
			})

			It("should return true when RepositoryProperty source differs", func() {
				rulesetPreset.Spec.Conditions = &githubv1alpha1.RulesetConditions{
					RefName: &githubv1alpha1.RefNameCondition{
						Include: []string{"~ALL"},
					},
					RepositoryProperty: &githubv1alpha1.RepositoryPropertyCondition{
						Include: []githubv1alpha1.RepositoryPropertyTarget{
							{Name: "environment", PropertyValues: []string{"production"}, Source: new("custom")},
						},
					},
				}
				githubRuleset.Conditions = &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"~ALL"},
					},
					RepositoryProperty: &github.RepositoryRulesetRepositoryPropertyConditionParameters{
						Include: []*github.RepositoryRulesetRepositoryPropertyTargetParameters{
							{Name: "environment", PropertyValues: []string{"production"}, Source: new("system")},
						},
					},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})

			It("should return false when RepositoryProperty source matches explicitly", func() {
				rulesetPreset.Spec.Conditions = &githubv1alpha1.RulesetConditions{
					RefName: &githubv1alpha1.RefNameCondition{
						Include: []string{"~ALL"},
					},
					RepositoryProperty: &githubv1alpha1.RepositoryPropertyCondition{
						Include: []githubv1alpha1.RepositoryPropertyTarget{
							{Name: "environment", PropertyValues: []string{"production"}, Source: new("custom")},
						},
					},
				}
				githubRuleset.Conditions = &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"~ALL"},
					},
					RepositoryProperty: &github.RepositoryRulesetRepositoryPropertyConditionParameters{
						Include: []*github.RepositoryRulesetRepositoryPropertyTargetParameters{
							{Name: "environment", PropertyValues: []string{"production"}, Source: new("custom")},
						},
					},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeFalse())
			})

			It("should return false when RepositoryProperty source is nil vs custom (default)", func() {
				rulesetPreset.Spec.Conditions = &githubv1alpha1.RulesetConditions{
					RefName: &githubv1alpha1.RefNameCondition{
						Include: []string{"~ALL"},
					},
					RepositoryProperty: &githubv1alpha1.RepositoryPropertyCondition{
						Include: []githubv1alpha1.RepositoryPropertyTarget{
							{Name: "environment", PropertyValues: []string{"production"}, Source: nil},
						},
					},
				}
				githubRuleset.Conditions = &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"~ALL"},
					},
					RepositoryProperty: &github.RepositoryRulesetRepositoryPropertyConditionParameters{
						Include: []*github.RepositoryRulesetRepositoryPropertyTargetParameters{
							{Name: "environment", PropertyValues: []string{"production"}, Source: new("custom")},
						},
					},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeFalse())
			})

			It("should return true when RepositoryProperty source is nil vs system", func() {
				rulesetPreset.Spec.Conditions = &githubv1alpha1.RulesetConditions{
					RefName: &githubv1alpha1.RefNameCondition{
						Include: []string{"~ALL"},
					},
					RepositoryProperty: &githubv1alpha1.RepositoryPropertyCondition{
						Include: []githubv1alpha1.RepositoryPropertyTarget{
							{Name: "environment", PropertyValues: []string{"production"}, Source: nil},
						},
					},
				}
				githubRuleset.Conditions = &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{
						Include: []string{"~ALL"},
					},
					RepositoryProperty: &github.RepositoryRulesetRepositoryPropertyConditionParameters{
						Include: []*github.RepositoryRulesetRepositoryPropertyTargetParameters{
							{Name: "environment", PropertyValues: []string{"production"}, Source: new("system")},
						},
					},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})
		})

		Context("when comparing bypass actors", func() {
			It("should return false when DeployKey actors match (both nil ActorID)", func() {
				deployKeyType := github.BypassActorTypeDeployKey
				rulesetPreset.Spec.BypassActors = []githubv1alpha1.RulesetBypassActor{
					{
						ActorType:  "DeployKey",
						BypassMode: "always",
						ActorID:    nil,
					},
				}
				githubRuleset.BypassActors = []*github.BypassActor{
					{
						ActorType:  &deployKeyType,
						BypassMode: github.Ptr(github.BypassMode("always")),
						ActorID:    nil,
					},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeFalse())
			})

			It("should return false when multiple DeployKey actors match", func() {
				deployKeyType := github.BypassActorTypeDeployKey
				rulesetPreset.Spec.BypassActors = []githubv1alpha1.RulesetBypassActor{
					{ActorType: "DeployKey", BypassMode: "always"},
					{ActorType: "DeployKey", BypassMode: "pull_request"},
				}
				githubRuleset.BypassActors = []*github.BypassActor{
					{ActorType: &deployKeyType, BypassMode: github.Ptr(github.BypassMode("always"))},
					{ActorType: &deployKeyType, BypassMode: github.Ptr(github.BypassMode("pull_request"))},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeFalse())
			})

			It("should return true when DeployKey bypass modes differ", func() {
				deployKeyType := github.BypassActorTypeDeployKey
				rulesetPreset.Spec.BypassActors = []githubv1alpha1.RulesetBypassActor{
					{ActorType: "DeployKey", BypassMode: "always"},
				}
				githubRuleset.BypassActors = []*github.BypassActor{
					{ActorType: &deployKeyType, BypassMode: github.Ptr(github.BypassMode("pull_request"))},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})

			It("should return false when OrganizationAdmin actors match (both nil ActorID)", func() {
				orgAdminType := github.BypassActorTypeOrganizationAdmin
				rulesetPreset.Spec.BypassActors = []githubv1alpha1.RulesetBypassActor{
					{ActorType: "OrganizationAdmin", BypassMode: "always"},
				}
				githubRuleset.BypassActors = []*github.BypassActor{
					{ActorType: &orgAdminType, BypassMode: github.Ptr(github.BypassMode("always"))},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeFalse())
			})

			It("should return false when EnterpriseOwner actors match (both nil ActorID)", func() {
				enterpriseOwnerType := github.BypassActorType("EnterpriseOwner")
				rulesetPreset.Spec.BypassActors = []githubv1alpha1.RulesetBypassActor{
					{ActorType: "EnterpriseOwner", BypassMode: "always"},
				}
				githubRuleset.BypassActors = []*github.BypassActor{
					{ActorType: &enterpriseOwnerType, BypassMode: github.Ptr(github.BypassMode("always"))},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeFalse())
			})

			It("should return false when mixing Team (with ActorID) and DeployKey (without ActorID)", func() {
				teamType := github.BypassActorTypeTeam
				deployKeyType := github.BypassActorTypeDeployKey
				rulesetPreset.Spec.BypassActors = []githubv1alpha1.RulesetBypassActor{
					{ActorID: new(int64(123)), ActorType: "Team", BypassMode: "always"},
					{ActorType: "DeployKey", BypassMode: "always"},
				}
				githubRuleset.BypassActors = []*github.BypassActor{
					{ActorID: new(int64(123)), ActorType: &teamType, BypassMode: github.Ptr(github.BypassMode("always"))},
					{ActorType: &deployKeyType, BypassMode: github.Ptr(github.BypassMode("always"))},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeFalse())
			})

			It("should return false when all actor types with nil ActorID are present", func() {
				deployKeyType := github.BypassActorTypeDeployKey
				orgAdminType := github.BypassActorTypeOrganizationAdmin
				enterpriseOwnerType := github.BypassActorType("EnterpriseOwner")
				teamType := github.BypassActorTypeTeam

				rulesetPreset.Spec.BypassActors = []githubv1alpha1.RulesetBypassActor{
					{ActorType: "DeployKey", BypassMode: "always"},
					{ActorType: "OrganizationAdmin", BypassMode: "pull_request"},
					{ActorType: "EnterpriseOwner", BypassMode: "always"},
					{ActorID: new(int64(456)), ActorType: "Team", BypassMode: "always"},
				}
				githubRuleset.BypassActors = []*github.BypassActor{
					{ActorType: &deployKeyType, BypassMode: github.Ptr(github.BypassMode("always"))},
					{ActorType: &orgAdminType, BypassMode: github.Ptr(github.BypassMode("pull_request"))},
					{ActorType: &enterpriseOwnerType, BypassMode: github.Ptr(github.BypassMode("always"))},
					{ActorID: new(int64(456)), ActorType: &teamType, BypassMode: github.Ptr(github.BypassMode("always"))},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeFalse())
			})

			It("should return true when DeployKey count differs", func() {
				deployKeyType := github.BypassActorTypeDeployKey
				rulesetPreset.Spec.BypassActors = []githubv1alpha1.RulesetBypassActor{
					{ActorType: "DeployKey", BypassMode: "always"},
					{ActorType: "DeployKey", BypassMode: "pull_request"},
				}
				githubRuleset.BypassActors = []*github.BypassActor{
					{ActorType: &deployKeyType, BypassMode: github.Ptr(github.BypassMode("always"))},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})

			It("should return true when actors with same type but different ActorIDs", func() {
				teamType := github.BypassActorTypeTeam
				rulesetPreset.Spec.BypassActors = []githubv1alpha1.RulesetBypassActor{
					{ActorID: new(int64(123)), ActorType: "Team", BypassMode: "always"},
				}
				githubRuleset.BypassActors = []*github.BypassActor{
					{ActorID: new(int64(456)), ActorType: &teamType, BypassMode: github.Ptr(github.BypassMode("always"))},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})

			It("should return false when Integration actors match with same ActorID", func() {
				integrationType := github.BypassActorTypeIntegration
				rulesetPreset.Spec.BypassActors = []githubv1alpha1.RulesetBypassActor{
					{ActorID: new(int64(789)), ActorType: "Integration", BypassMode: "always"},
				}
				githubRuleset.BypassActors = []*github.BypassActor{
					{ActorID: new(int64(789)), ActorType: &integrationType, BypassMode: github.Ptr(github.BypassMode("always"))},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeFalse())
			})

			It("should return false when RepositoryRole actors match with same ActorID", func() {
				roleType := github.BypassActorTypeRepositoryRole
				rulesetPreset.Spec.BypassActors = []githubv1alpha1.RulesetBypassActor{
					{ActorID: new(int64(321)), ActorType: "RepositoryRole", BypassMode: "pull_request"},
				}
				githubRuleset.BypassActors = []*github.BypassActor{
					{ActorID: new(int64(321)), ActorType: &roleType, BypassMode: github.Ptr(github.BypassMode("pull_request"))},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeFalse())
			})

			It("should handle empty bypass mode correctly", func() {
				teamType := github.BypassActorTypeTeam
				rulesetPreset.Spec.BypassActors = []githubv1alpha1.RulesetBypassActor{
					{ActorID: new(int64(123)), ActorType: "Team", BypassMode: ""},
				}
				githubRuleset.BypassActors = []*github.BypassActor{
					{ActorID: new(int64(123)), ActorType: &teamType, BypassMode: nil},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeFalse())
			})

			It("should return true when bypass mode differs between empty and set", func() {
				teamType := github.BypassActorTypeTeam
				rulesetPreset.Spec.BypassActors = []githubv1alpha1.RulesetBypassActor{
					{ActorID: new(int64(123)), ActorType: "Team", BypassMode: "always"},
				}
				githubRuleset.BypassActors = []*github.BypassActor{
					{ActorID: new(int64(123)), ActorType: &teamType, BypassMode: nil},
				}

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})
		})

		Context("Target Type comparison", func() {
			It("should return false when target types match (branch)", func() {
				rulesetPreset.Spec.Target = TargetTypeBranch
				githubRuleset.Target = github.Ptr(github.RulesetTargetBranch)

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeFalse())
			})

			It("should return false when target types match (tag)", func() {
				rulesetPreset.Spec.Target = TargetTypeTag
				githubRuleset.Target = github.Ptr(github.RulesetTargetTag)

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeFalse())
			})

			It("should return false when target types match (push)", func() {
				rulesetPreset.Spec.Target = "push"
				githubRuleset.Target = github.Ptr(github.RulesetTargetPush)

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeFalse())
			})

			It("should return true when target types differ (branch vs tag)", func() {
				rulesetPreset.Spec.Target = TargetTypeBranch
				githubRuleset.Target = github.Ptr(github.RulesetTargetTag)

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})

			It("should return true when target types differ (tag vs push)", func() {
				rulesetPreset.Spec.Target = TargetTypeTag
				githubRuleset.Target = github.Ptr(github.RulesetTargetPush)

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})

			It("should return true when target types differ (branch vs push)", func() {
				rulesetPreset.Spec.Target = TargetTypeBranch
				githubRuleset.Target = github.Ptr(github.RulesetTargetPush)

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})

			It("should return true when preset has empty target type but GitHub has tag", func() {
				rulesetPreset.Spec.Target = ""
				githubRuleset.Target = github.Ptr(github.RulesetTargetTag)

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})

			It("should handle nil GitHub target (defaults to branch)", func() {
				rulesetPreset.Spec.Target = TargetTypeBranch
				githubRuleset.Target = nil

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeFalse())
			})

			It("should return true when preset has tag but GitHub target is nil", func() {
				rulesetPreset.Spec.Target = TargetTypeTag
				githubRuleset.Target = nil

				differs := RulesetsDiffer(rulesetPreset, githubRuleset)
				Expect(differs).To(BeTrue())
			})
		})

		Context("when comparing empty rulesets", func() {
			var emptyPreset githubv1alpha1.RulesetPreset
			var emptyGithubRuleset github.RepositoryRuleset

			BeforeEach(func() {
				emptyPreset = githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "empty-ruleset",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Target:      TargetTypeBranch, // Set default target type
						Conditions:  nil,
						Rules:       githubv1alpha1.RulesetRules{},
					},
				}
				emptyGithubRuleset = github.RepositoryRuleset{
					Name:        "empty-ruleset",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch), // Set target type
				}
			})

			It("should return false when both are empty", func() {
				differs := RulesetsDiffer(emptyPreset, emptyGithubRuleset)
				Expect(differs).To(BeFalse())
			})
		})

		Context("when workflow rules differ", func() {
			It("should return true when preset has workflows but GitHub does not", func() {
				preset := githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "test",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Target:      TargetTypeBranch,
						Conditions: &githubv1alpha1.RulesetConditions{
							RefName: &githubv1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: githubv1alpha1.RulesetRules{
							Workflows: &githubv1alpha1.WorkflowsRule{
								Workflows: []githubv1alpha1.RuleWorkflow{
									{Path: ".github/workflows/ci.yaml", RepositoryName: "my-repo", ResolvedRepositoryID: new(int64(42))},
								},
							},
						},
					},
				}
				ghRuleset := github.RepositoryRuleset{
					Name:        "test",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{Include: []string{"main"}},
					},
					Rules: &github.RepositoryRulesetRules{},
				}
				Expect(RulesetsDiffer(preset, ghRuleset)).To(BeTrue())
			})

			It("should return false when workflow rules match", func() {
				preset := githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "test",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Target:      TargetTypeBranch,
						Conditions: &githubv1alpha1.RulesetConditions{
							RefName: &githubv1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: githubv1alpha1.RulesetRules{
							Workflows: &githubv1alpha1.WorkflowsRule{
								Workflows: []githubv1alpha1.RuleWorkflow{
									{Path: ".github/workflows/ci.yaml", RepositoryName: "my-repo", ResolvedRepositoryID: new(int64(42)), Ref: new("refs/heads/main")},
								},
							},
						},
					},
				}
				ghRuleset := github.RepositoryRuleset{
					Name:        "test",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{Include: []string{"main"}},
					},
					Rules: &github.RepositoryRulesetRules{
						Workflows: &github.WorkflowsRuleParameters{
							Workflows: []*github.RuleWorkflow{
								{Path: ".github/workflows/ci.yaml", RepositoryID: new(int64(42)), Ref: new("refs/heads/main")},
							},
						},
					},
				}
				Expect(RulesetsDiffer(preset, ghRuleset)).To(BeFalse())
			})

			It("should return true when workflow repository IDs differ", func() {
				preset := githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "test",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Target:      TargetTypeBranch,
						Conditions: &githubv1alpha1.RulesetConditions{
							RefName: &githubv1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: githubv1alpha1.RulesetRules{
							Workflows: &githubv1alpha1.WorkflowsRule{
								Workflows: []githubv1alpha1.RuleWorkflow{
									{Path: ".github/workflows/ci.yaml", RepositoryName: "my-repo", ResolvedRepositoryID: new(int64(42))},
								},
							},
						},
					},
				}
				ghRuleset := github.RepositoryRuleset{
					Name:        "test",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{Include: []string{"main"}},
					},
					Rules: &github.RepositoryRulesetRules{
						Workflows: &github.WorkflowsRuleParameters{
							Workflows: []*github.RuleWorkflow{
								{Path: ".github/workflows/ci.yaml", RepositoryID: new(int64(99))},
							},
						},
					},
				}
				Expect(RulesetsDiffer(preset, ghRuleset)).To(BeTrue())
			})

			It("should return true when workflow refs differ", func() {
				preset := githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "test",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Target:      TargetTypeBranch,
						Conditions: &githubv1alpha1.RulesetConditions{
							RefName: &githubv1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: githubv1alpha1.RulesetRules{
							Workflows: &githubv1alpha1.WorkflowsRule{
								Workflows: []githubv1alpha1.RuleWorkflow{
									{Path: ".github/workflows/ci.yaml", RepositoryName: "my-repo", ResolvedRepositoryID: new(int64(42)), Ref: new("refs/heads/main")},
								},
							},
						},
					},
				}
				ghRuleset := github.RepositoryRuleset{
					Name:        "test",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{Include: []string{"main"}},
					},
					Rules: &github.RepositoryRulesetRules{
						Workflows: &github.WorkflowsRuleParameters{
							Workflows: []*github.RuleWorkflow{
								{Path: ".github/workflows/ci.yaml", RepositoryID: new(int64(42)), Ref: new("refs/heads/develop")},
							},
						},
					},
				}
				Expect(RulesetsDiffer(preset, ghRuleset)).To(BeTrue())
			})

			It("should return true when DoNotEnforceOnCreate differs", func() {
				preset := githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "test",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Target:      TargetTypeBranch,
						Conditions: &githubv1alpha1.RulesetConditions{
							RefName: &githubv1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: githubv1alpha1.RulesetRules{
							Workflows: &githubv1alpha1.WorkflowsRule{
								DoNotEnforceOnCreate: new(true),
								Workflows: []githubv1alpha1.RuleWorkflow{
									{Path: ".github/workflows/ci.yaml", RepositoryName: "my-repo", ResolvedRepositoryID: new(int64(42))},
								},
							},
						},
					},
				}
				ghRuleset := github.RepositoryRuleset{
					Name:        "test",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{Include: []string{"main"}},
					},
					Rules: &github.RepositoryRulesetRules{
						Workflows: &github.WorkflowsRuleParameters{
							DoNotEnforceOnCreate: new(false),
							Workflows: []*github.RuleWorkflow{
								{Path: ".github/workflows/ci.yaml", RepositoryID: new(int64(42))},
							},
						},
					},
				}
				Expect(RulesetsDiffer(preset, ghRuleset)).To(BeTrue())
			})

			It("should return false when DoNotEnforceOnCreate matches (both true)", func() {
				preset := githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "test",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Target:      TargetTypeBranch,
						Conditions: &githubv1alpha1.RulesetConditions{
							RefName: &githubv1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: githubv1alpha1.RulesetRules{
							Workflows: &githubv1alpha1.WorkflowsRule{
								DoNotEnforceOnCreate: new(true),
								Workflows: []githubv1alpha1.RuleWorkflow{
									{Path: ".github/workflows/ci.yaml", RepositoryName: "my-repo", ResolvedRepositoryID: new(int64(42))},
								},
							},
						},
					},
				}
				ghRuleset := github.RepositoryRuleset{
					Name:        "test",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{Include: []string{"main"}},
					},
					Rules: &github.RepositoryRulesetRules{
						Workflows: &github.WorkflowsRuleParameters{
							DoNotEnforceOnCreate: new(true),
							Workflows: []*github.RuleWorkflow{
								{Path: ".github/workflows/ci.yaml", RepositoryID: new(int64(42))},
							},
						},
					},
				}
				Expect(RulesetsDiffer(preset, ghRuleset)).To(BeFalse())
			})

			It("should return false when DoNotEnforceOnCreate is nil vs false (defaults match)", func() {
				preset := githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "test",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Target:      TargetTypeBranch,
						Conditions: &githubv1alpha1.RulesetConditions{
							RefName: &githubv1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: githubv1alpha1.RulesetRules{
							Workflows: &githubv1alpha1.WorkflowsRule{
								DoNotEnforceOnCreate: nil,
								Workflows: []githubv1alpha1.RuleWorkflow{
									{Path: ".github/workflows/ci.yaml", RepositoryName: "my-repo", ResolvedRepositoryID: new(int64(42))},
								},
							},
						},
					},
				}
				ghRuleset := github.RepositoryRuleset{
					Name:        "test",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{Include: []string{"main"}},
					},
					Rules: &github.RepositoryRulesetRules{
						Workflows: &github.WorkflowsRuleParameters{
							DoNotEnforceOnCreate: new(false),
							Workflows: []*github.RuleWorkflow{
								{Path: ".github/workflows/ci.yaml", RepositoryID: new(int64(42))},
							},
						},
					},
				}
				Expect(RulesetsDiffer(preset, ghRuleset)).To(BeFalse())
			})

			It("should return true when same path exists in different repositories", func() {
				preset := githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "test",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Target:      TargetTypeBranch,
						Conditions: &githubv1alpha1.RulesetConditions{
							RefName: &githubv1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: githubv1alpha1.RulesetRules{
							Workflows: &githubv1alpha1.WorkflowsRule{
								Workflows: []githubv1alpha1.RuleWorkflow{
									{Path: ".github/workflows/ci.yaml", RepositoryName: "repo-a", ResolvedRepositoryID: new(int64(42))},
								},
							},
						},
					},
				}
				ghRuleset := github.RepositoryRuleset{
					Name:        "test",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{Include: []string{"main"}},
					},
					Rules: &github.RepositoryRulesetRules{
						Workflows: &github.WorkflowsRuleParameters{
							Workflows: []*github.RuleWorkflow{
								{Path: ".github/workflows/ci.yaml", RepositoryID: new(int64(99))},
							},
						},
					},
				}
				Expect(RulesetsDiffer(preset, ghRuleset)).To(BeTrue())
			})

			It("should return false when multiple workflows from different repos match", func() {
				preset := githubv1alpha1.RulesetPreset{
					Spec: githubv1alpha1.RulesetPresetSpec{
						Name:        "test",
						Enforcement: githubv1alpha1.RulesetEnforcementActive,
						Target:      TargetTypeBranch,
						Conditions: &githubv1alpha1.RulesetConditions{
							RefName: &githubv1alpha1.RefNameCondition{
								Include: []string{"main"},
							},
						},
						Rules: githubv1alpha1.RulesetRules{
							Workflows: &githubv1alpha1.WorkflowsRule{
								Workflows: []githubv1alpha1.RuleWorkflow{
									{Path: ".github/workflows/ci.yaml", RepositoryName: "repo-a", ResolvedRepositoryID: new(int64(42))},
									{Path: ".github/workflows/ci.yaml", RepositoryName: "repo-b", ResolvedRepositoryID: new(int64(99))},
								},
							},
						},
					},
				}
				ghRuleset := github.RepositoryRuleset{
					Name:        "test",
					Enforcement: github.RulesetEnforcementActive,
					Target:      github.Ptr(github.RulesetTargetBranch),
					Conditions: &github.RepositoryRulesetConditions{
						RefName: &github.RepositoryRulesetRefConditionParameters{Include: []string{"main"}},
					},
					Rules: &github.RepositoryRulesetRules{
						Workflows: &github.WorkflowsRuleParameters{
							Workflows: []*github.RuleWorkflow{
								{Path: ".github/workflows/ci.yaml", RepositoryID: new(int64(42))},
								{Path: ".github/workflows/ci.yaml", RepositoryID: new(int64(99))},
							},
						},
					},
				}
				Expect(RulesetsDiffer(preset, ghRuleset)).To(BeFalse())
			})
		})
	})

	Context("RulesetPresetToGithubRuleset with required reviewers", func() {
		basePreset := func(reviewers []githubv1alpha1.RequiredPullRequestReviewer) githubv1alpha1.RulesetPreset {
			return githubv1alpha1.RulesetPreset{
				Spec: githubv1alpha1.RulesetPresetSpec{
					Name:        "pr-reviewers-ruleset",
					Enforcement: githubv1alpha1.RulesetEnforcementActive,
					Target:      TargetTypeBranch,
					Conditions: &githubv1alpha1.RulesetConditions{
						RefName: &githubv1alpha1.RefNameCondition{
							Include: []string{"main"},
						},
					},
					Rules: githubv1alpha1.RulesetRules{
						PullRequest: &githubv1alpha1.PullRequestRule{
							RequiredReviewers: reviewers,
						},
					},
				},
			}
		}

		It("should map nil required reviewers to empty slice", func() {
			preset := basePreset(nil)
			ruleset, err := RulesetPresetToGithubRuleset(preset)
			Expect(err).NotTo(HaveOccurred())
			Expect(ruleset.Rules.PullRequest).NotTo(BeNil())
			Expect(ruleset.Rules.PullRequest.RequiredReviewers).To(BeEmpty())
		})

		It("should map a single required reviewer with ID", func() {
			teamID := int64(42)
			preset := basePreset([]githubv1alpha1.RequiredPullRequestReviewer{
				{
					MinimumApprovals: 1,
					FilePatterns:     []string{"*.go", "*.yaml"},
					Reviewer: githubv1alpha1.PullRequestReviewerEntity{
						ID:   &teamID,
						Type: "Team",
					},
				},
			})
			ruleset, err := RulesetPresetToGithubRuleset(preset)
			Expect(err).NotTo(HaveOccurred())
			Expect(ruleset.Rules.PullRequest.RequiredReviewers).To(HaveLen(1))

			r := ruleset.Rules.PullRequest.RequiredReviewers[0]
			Expect(r.MinimumApprovals).To(Equal(new(1)))
			Expect(r.FilePatterns).To(ConsistOf("*.go", "*.yaml"))
			Expect(r.Reviewer).NotTo(BeNil())
			Expect(*r.Reviewer.ID).To(Equal(int64(42)))
			Expect(string(*r.Reviewer.Type)).To(Equal("Team"))
		})

		It("should map minimum approvals of zero", func() {
			teamID := int64(10)
			preset := basePreset([]githubv1alpha1.RequiredPullRequestReviewer{
				{
					MinimumApprovals: 0,
					Reviewer: githubv1alpha1.PullRequestReviewerEntity{
						ID:   &teamID,
						Type: "Team",
					},
				},
			})
			ruleset, err := RulesetPresetToGithubRuleset(preset)
			Expect(err).NotTo(HaveOccurred())
			r := ruleset.Rules.PullRequest.RequiredReviewers[0]
			Expect(r.MinimumApprovals).To(Equal(new(0)))
		})

		It("should map nil file patterns to nil slice", func() {
			teamID := int64(10)
			preset := basePreset([]githubv1alpha1.RequiredPullRequestReviewer{
				{
					FilePatterns: nil,
					Reviewer: githubv1alpha1.PullRequestReviewerEntity{
						ID:   &teamID,
						Type: "Team",
					},
				},
			})
			ruleset, err := RulesetPresetToGithubRuleset(preset)
			Expect(err).NotTo(HaveOccurred())
			r := ruleset.Rules.PullRequest.RequiredReviewers[0]
			Expect(r.FilePatterns).To(BeNil())
		})

		It("should map multiple required reviewers", func() {
			teamID1 := int64(10)
			teamID2 := int64(20)
			preset := basePreset([]githubv1alpha1.RequiredPullRequestReviewer{
				{
					MinimumApprovals: 2,
					FilePatterns:     []string{"src/**"},
					Reviewer:         githubv1alpha1.PullRequestReviewerEntity{ID: &teamID1, Type: "Team"},
				},
				{
					MinimumApprovals: 1,
					FilePatterns:     []string{"docs/**"},
					Reviewer:         githubv1alpha1.PullRequestReviewerEntity{ID: &teamID2, Type: "Team"},
				},
			})
			ruleset, err := RulesetPresetToGithubRuleset(preset)
			Expect(err).NotTo(HaveOccurred())
			Expect(ruleset.Rules.PullRequest.RequiredReviewers).To(HaveLen(2))

			ids := []int64{
				*ruleset.Rules.PullRequest.RequiredReviewers[0].Reviewer.ID,
				*ruleset.Rules.PullRequest.RequiredReviewers[1].Reviewer.ID,
			}
			Expect(ids).To(ConsistOf(int64(10), int64(20)))
		})

		It("should use resolved ID (not slug) for the reviewer", func() {
			resolvedID := int64(99)
			preset := basePreset([]githubv1alpha1.RequiredPullRequestReviewer{
				{
					Reviewer: githubv1alpha1.PullRequestReviewerEntity{
						ID:   &resolvedID,
						Type: "Team",
					},
				},
			})
			ruleset, err := RulesetPresetToGithubRuleset(preset)
			Expect(err).NotTo(HaveOccurred())
			Expect(*ruleset.Rules.PullRequest.RequiredReviewers[0].Reviewer.ID).To(Equal(int64(99)))
		})
	})

	Context("RulesetsDiffer with required reviewers", func() {
		teamReviewerType := github.RulesetReviewerType("Team")

		basePresetWithReviewers := func(reviewers []githubv1alpha1.RequiredPullRequestReviewer) githubv1alpha1.RulesetPreset {
			return githubv1alpha1.RulesetPreset{
				Spec: githubv1alpha1.RulesetPresetSpec{
					Name:        "test",
					Enforcement: githubv1alpha1.RulesetEnforcementActive,
					Target:      TargetTypeBranch,
					Conditions: &githubv1alpha1.RulesetConditions{
						RefName: &githubv1alpha1.RefNameCondition{Include: []string{"main"}},
					},
					Rules: githubv1alpha1.RulesetRules{
						PullRequest: &githubv1alpha1.PullRequestRule{
							RequiredReviewers: reviewers,
						},
					},
				},
			}
		}

		baseGithubWithReviewers := func(reviewers []*github.RulesetRequiredReviewer) github.RepositoryRuleset {
			return github.RepositoryRuleset{
				Name:        "test",
				Enforcement: github.RulesetEnforcementActive,
				Target:      github.Ptr(github.RulesetTargetBranch),
				Conditions: &github.RepositoryRulesetConditions{
					RefName: &github.RepositoryRulesetRefConditionParameters{Include: []string{"main"}},
				},
				Rules: &github.RepositoryRulesetRules{
					PullRequest: &github.PullRequestRuleParameters{
						RequiredReviewers: reviewers,
					},
				},
			}
		}

		It("should return false when both have no required reviewers", func() {
			preset := basePresetWithReviewers(nil)
			ghRuleset := baseGithubWithReviewers(nil)
			Expect(RulesetsDiffer(preset, ghRuleset)).To(BeFalse())
		})

		It("should return false when required reviewers match", func() {
			teamID := int64(42)
			minApprovals := 1
			preset := basePresetWithReviewers([]githubv1alpha1.RequiredPullRequestReviewer{
				{
					MinimumApprovals: 1,
					FilePatterns:     []string{"*.go"},
					Reviewer:         githubv1alpha1.PullRequestReviewerEntity{ID: &teamID, Type: "Team"},
				},
			})
			ghRuleset := baseGithubWithReviewers([]*github.RulesetRequiredReviewer{
				{
					MinimumApprovals: &minApprovals,
					FilePatterns:     []string{"*.go"},
					Reviewer:         &github.RulesetReviewer{ID: &teamID, Type: &teamReviewerType},
				},
			})
			Expect(RulesetsDiffer(preset, ghRuleset)).To(BeFalse())
		})

		It("should return true when reviewer count differs", func() {
			teamID := int64(42)
			minApprovals := 1
			preset := basePresetWithReviewers([]githubv1alpha1.RequiredPullRequestReviewer{
				{Reviewer: githubv1alpha1.PullRequestReviewerEntity{ID: &teamID, Type: "Team"}},
			})
			ghRuleset := baseGithubWithReviewers([]*github.RulesetRequiredReviewer{
				{MinimumApprovals: &minApprovals, Reviewer: &github.RulesetReviewer{ID: &teamID, Type: &teamReviewerType}},
				{MinimumApprovals: &minApprovals, Reviewer: &github.RulesetReviewer{ID: github.Ptr(int64(99)), Type: &teamReviewerType}},
			})
			Expect(RulesetsDiffer(preset, ghRuleset)).To(BeTrue())
		})

		It("should return true when reviewer ID differs", func() {
			teamID := int64(42)
			differentID := int64(99)
			minApprovals := 1
			preset := basePresetWithReviewers([]githubv1alpha1.RequiredPullRequestReviewer{
				{Reviewer: githubv1alpha1.PullRequestReviewerEntity{ID: &teamID, Type: "Team"}},
			})
			ghRuleset := baseGithubWithReviewers([]*github.RulesetRequiredReviewer{
				{MinimumApprovals: &minApprovals, Reviewer: &github.RulesetReviewer{ID: &differentID, Type: &teamReviewerType}},
			})
			Expect(RulesetsDiffer(preset, ghRuleset)).To(BeTrue())
		})

		It("should return true when minimum approvals differ", func() {
			teamID := int64(42)
			minApprovals := 2
			preset := basePresetWithReviewers([]githubv1alpha1.RequiredPullRequestReviewer{
				{MinimumApprovals: 1, Reviewer: githubv1alpha1.PullRequestReviewerEntity{ID: &teamID, Type: "Team"}},
			})
			ghRuleset := baseGithubWithReviewers([]*github.RulesetRequiredReviewer{
				{MinimumApprovals: &minApprovals, Reviewer: &github.RulesetReviewer{ID: &teamID, Type: &teamReviewerType}},
			})
			Expect(RulesetsDiffer(preset, ghRuleset)).To(BeTrue())
		})

		It("should return true when file patterns differ", func() {
			teamID := int64(42)
			minApprovals := 0
			preset := basePresetWithReviewers([]githubv1alpha1.RequiredPullRequestReviewer{
				{FilePatterns: []string{"*.go"}, Reviewer: githubv1alpha1.PullRequestReviewerEntity{ID: &teamID, Type: "Team"}},
			})
			ghRuleset := baseGithubWithReviewers([]*github.RulesetRequiredReviewer{
				{MinimumApprovals: &minApprovals, FilePatterns: []string{"*.yaml"}, Reviewer: &github.RulesetReviewer{ID: &teamID, Type: &teamReviewerType}},
			})
			Expect(RulesetsDiffer(preset, ghRuleset)).To(BeTrue())
		})

		It("should return false when minimum approvals is nil on GitHub side and 0 in preset", func() {
			teamID := int64(42)
			preset := basePresetWithReviewers([]githubv1alpha1.RequiredPullRequestReviewer{
				{MinimumApprovals: 0, Reviewer: githubv1alpha1.PullRequestReviewerEntity{ID: &teamID, Type: "Team"}},
			})
			ghRuleset := baseGithubWithReviewers([]*github.RulesetRequiredReviewer{
				{MinimumApprovals: nil, Reviewer: &github.RulesetReviewer{ID: &teamID, Type: &teamReviewerType}},
			})
			Expect(RulesetsDiffer(preset, ghRuleset)).To(BeFalse())
		})

		It("should return true when preset has reviewers but GitHub has none", func() {
			teamID := int64(42)
			preset := basePresetWithReviewers([]githubv1alpha1.RequiredPullRequestReviewer{
				{Reviewer: githubv1alpha1.PullRequestReviewerEntity{ID: &teamID, Type: "Team"}},
			})
			ghRuleset := baseGithubWithReviewers(nil)
			Expect(RulesetsDiffer(preset, ghRuleset)).To(BeTrue())
		})

		It("should return true when GitHub has reviewers but preset has none", func() {
			teamID := int64(42)
			minApprovals := 0
			preset := basePresetWithReviewers(nil)
			ghRuleset := baseGithubWithReviewers([]*github.RulesetRequiredReviewer{
				{MinimumApprovals: &minApprovals, Reviewer: &github.RulesetReviewer{ID: &teamID, Type: &teamReviewerType}},
			})
			Expect(RulesetsDiffer(preset, ghRuleset)).To(BeTrue())
		})
	})

	Context("RulesetPresetToGithubRuleset with workflows", func() {
		It("should map workflows rule correctly", func() {
			preset := githubv1alpha1.RulesetPreset{
				Spec: githubv1alpha1.RulesetPresetSpec{
					Name:        "workflow-ruleset",
					Enforcement: githubv1alpha1.RulesetEnforcementActive,
					Target:      TargetTypeBranch,
					Conditions: &githubv1alpha1.RulesetConditions{
						RefName: &githubv1alpha1.RefNameCondition{
							Include: []string{"main"},
						},
					},
					Rules: githubv1alpha1.RulesetRules{
						Workflows: &githubv1alpha1.WorkflowsRule{
							DoNotEnforceOnCreate: new(true),
							Workflows: []githubv1alpha1.RuleWorkflow{
								{
									Path:                 ".github/workflows/ci.yaml",
									RepositoryName:       "my-repo",
									ResolvedRepositoryID: new(int64(42)),
									Ref:                  new("refs/heads/main"),
								},
							},
						},
					},
				},
			}

			ruleset, err := RulesetPresetToGithubRuleset(preset)
			Expect(err).NotTo(HaveOccurred())
			Expect(ruleset.Rules.Workflows).NotTo(BeNil())
			Expect(ruleset.Rules.Workflows.DoNotEnforceOnCreate).To(Equal(new(true)))
			Expect(ruleset.Rules.Workflows.Workflows).To(HaveLen(1))
			Expect(ruleset.Rules.Workflows.Workflows[0].Path).To(Equal(".github/workflows/ci.yaml"))
			Expect(ruleset.Rules.Workflows.Workflows[0].RepositoryID).To(Equal(new(int64(42))))
			Expect(ruleset.Rules.Workflows.Workflows[0].Ref).To(Equal(new("refs/heads/main")))
		})
	})

})

// Helper function to check if a rule is enabled in the ruleset rules
func hasRule(rules *github.RepositoryRulesetRules, ruleType string) bool {
	switch ruleType {
	case "creation":
		return rules.Creation != nil
	case "deletion":
		return rules.Deletion != nil
	case "required_linear_history":
		return rules.RequiredLinearHistory != nil
	case "non_fast_forward":
		return rules.NonFastForward != nil
	case "required_deployments":
		return rules.RequiredDeployments != nil
	case "pull_request":
		return rules.PullRequest != nil
	case "required_status_checks":
		return rules.RequiredStatusChecks != nil
	case "commit_message_pattern":
		return rules.CommitMessagePattern != nil
	case "branch_name_pattern":
		return rules.BranchNamePattern != nil
	case "tag_name_pattern":
		return rules.TagNamePattern != nil
	case "copilot_code_review":
		return rules.CopilotCodeReview != nil
	case "workflows":
		return rules.Workflows != nil
	}
	return false
}
