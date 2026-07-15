package mapper

import (
	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/google/go-github/v89/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GitHub Team Mapper", func() {

	Describe("TeamToNewGitHubTeam", func() {
		Context("when converting a team with manual members", func() {
			It("should successfully convert to GitHub new team", func() {
				team := &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-team",
					},
					Spec: v1alpha1.TeamSpec{
						Name:        "my-team",
						Description: "This is a test team",
						Members:     []string{"user1", "user2"},
					},
				}

				newTeam := TeamToNewGitHubTeam(team)

				Expect(newTeam).NotTo(BeNil())
				Expect(newTeam.Name).To(Equal("my-team"))
				Expect(newTeam.Description).To(Equal(new("This is a test team")))
				Expect(newTeam.Privacy).To(Equal(new("closed")))
				Expect(newTeam.Permission).To(Equal(new("pull"))) //nolint:staticcheck
				Expect(newTeam.NotificationSetting).To(Equal(new("notifications_disabled")))
			})
		})

		Context("when converting a team with IDP group", func() {
			It("should use the team description", func() {
				idpGroup := "engineers"
				team := &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-team",
					},
					Spec: v1alpha1.TeamSpec{
						Name:        "idp-team",
						Description: "IDP synchronized team",
						IDPGroup:    &idpGroup,
					},
				}

				newTeam := TeamToNewGitHubTeam(team)

				Expect(newTeam).NotTo(BeNil())
				Expect(newTeam.Description).To(Equal(new("IDP synchronized team")))
			})
		})

		Context("when converting a team without description", func() {
			It("should use empty description", func() {
				team := &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-team",
					},
					Spec: v1alpha1.TeamSpec{
						Name:    "my-team",
						Members: []string{"user1"},
					},
				}

				newTeam := TeamToNewGitHubTeam(team)

				Expect(newTeam).NotTo(BeNil())
				Expect(*newTeam.Description).To(Equal(""))
			})
		})

		Context("when converting a team with empty description", func() {
			It("should use empty description", func() {
				team := &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-team",
					},
					Spec: v1alpha1.TeamSpec{
						Name:        "my-team",
						Description: "",
						Members:     []string{"user1"},
					},
				}

				newTeam := TeamToNewGitHubTeam(team)

				Expect(newTeam).NotTo(BeNil())
				Expect(*newTeam.Description).To(Equal(""))
			})
		})

		Context("when converting teams for different organizations", func() {
			It("should use the same description for both organizations", func() {
				team := &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-team",
					},
					Spec: v1alpha1.TeamSpec{
						Name:        "my-team",
						Description: "Shared team description",
						Members:     []string{"user1"},
					},
				}

				newTeam1 := TeamToNewGitHubTeam(team)
				newTeam2 := TeamToNewGitHubTeam(team)

				Expect(*newTeam1.Description).To(Equal("Shared team description"))
				Expect(*newTeam2.Description).To(Equal("Shared team description"))
			})
		})

		Context("when custom privacy is specified", func() {
			It("should use the specified privacy", func() {
				team := &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-team",
					},
					Spec: v1alpha1.TeamSpec{
						Name:    "my-team",
						Privacy: "secret",
						Members: []string{"user1"},
					},
				}

				newTeam := TeamToNewGitHubTeam(team)

				Expect(newTeam.Privacy).To(Equal(new("secret")))
			})
		})

		Context("when custom permission is specified", func() {
			It("should use the specified permission", func() {
				team := &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-team",
					},
					Spec: v1alpha1.TeamSpec{
						Name:       "my-team",
						Permission: "push",
						Members:    []string{"user1"},
					},
				}

				newTeam := TeamToNewGitHubTeam(team)

				Expect(newTeam.Permission).To(Equal(new("push"))) //nolint:staticcheck
			})
		})

		Context("when custom notification setting is specified", func() {
			It("should use the specified notification setting", func() {
				team := &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-team",
					},
					Spec: v1alpha1.TeamSpec{
						Name:                "my-team",
						NotificationSetting: "notifications_enabled",
						Members:             []string{"user1"},
					},
				}

				newTeam := TeamToNewGitHubTeam(team)

				Expect(newTeam.NotificationSetting).To(Equal(new("notifications_enabled")))
			})
		})

		Context("when all team settings are customized", func() {
			It("should use all specified values", func() {
				team := &v1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-team",
					},
					Spec: v1alpha1.TeamSpec{
						Name:                "my-team",
						Description:         "Custom team",
						Privacy:             "secret",
						Permission:          "push",
						NotificationSetting: "notifications_enabled",
						Members:             []string{"user1"},
					},
				}

				newTeam := TeamToNewGitHubTeam(team)

				Expect(newTeam.Name).To(Equal("my-team"))
				Expect(newTeam.Description).To(Equal(new("Custom team")))
				Expect(newTeam.Privacy).To(Equal(new("secret")))
				Expect(newTeam.Permission).To(Equal(new("push"))) //nolint:staticcheck
				Expect(newTeam.NotificationSetting).To(Equal(new("notifications_enabled")))
			})
		})
	})

	Describe("TeamDiffers", func() {
		var team *v1alpha1.Team

		BeforeEach(func() {
			team = &v1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-team",
				},
				Spec: v1alpha1.TeamSpec{
					Name:        "my-team",
					Description: "Test team description",
					Members:     []string{"user1"},
				},
			}
		})

		Context("when teams match exactly", func() {
			It("should return false", func() {
				githubTeam := &github.Team{
					Name:                new("my-team"),
					Description:         new("Test team description"),
					Privacy:             new("closed"),
					Permission:          new("pull"),
					NotificationSetting: new("notifications_disabled"),
				}

				differs := TeamDiffers(team, githubTeam, "my-org")

				Expect(differs).To(BeFalse())
			})
		})

		Context("when K8s team is nil", func() {
			It("should return true", func() {
				githubTeam := &github.Team{
					Name: new("my-team"),
				}

				differs := TeamDiffers(nil, githubTeam, "my-org")

				Expect(differs).To(BeTrue())
			})
		})

		Context("when GitHub team is nil", func() {
			It("should return true", func() {
				differs := TeamDiffers(team, nil, "my-org")

				Expect(differs).To(BeTrue())
			})
		})

		Context("when name differs", func() {
			It("should return true", func() {
				githubTeam := &github.Team{
					Name:                new("different-team"),
					Description:         new("Test team description"),
					Privacy:             new("closed"),
					Permission:          new("pull"),
					NotificationSetting: new("notifications_disabled"),
				}

				differs := TeamDiffers(team, githubTeam, "my-org")

				Expect(differs).To(BeTrue())
			})
		})

		Context("when description differs", func() {
			It("should return true", func() {
				githubTeam := &github.Team{
					Name:                new("my-team"),
					Description:         new("Different description"),
					Privacy:             new("closed"),
					Permission:          new("pull"),
					NotificationSetting: new("notifications_disabled"),
				}

				differs := TeamDiffers(team, githubTeam, "my-org")

				Expect(differs).To(BeTrue())
			})
		})

		Context("when description is nil on GitHub", func() {
			It("should return true", func() {
				githubTeam := &github.Team{
					Name:                new("my-team"),
					Description:         nil,
					Privacy:             new("closed"),
					Permission:          new("pull"),
					NotificationSetting: new("notifications_disabled"),
				}

				differs := TeamDiffers(team, githubTeam, "my-org")

				Expect(differs).To(BeTrue())
			})
		})

		Context("when privacy differs", func() {
			It("should return true for secret privacy", func() {
				githubTeam := &github.Team{
					Name:                new("my-team"),
					Description:         new("Test team description"),
					Privacy:             new("secret"),
					Permission:          new("pull"),
					NotificationSetting: new("notifications_disabled"),
				}

				differs := TeamDiffers(team, githubTeam, "my-org")

				Expect(differs).To(BeTrue())
			})

			It("should return true when privacy is nil", func() {
				githubTeam := &github.Team{
					Name:                new("my-team"),
					Description:         new("Test team description"),
					Privacy:             nil,
					Permission:          new("pull"),
					NotificationSetting: new("notifications_disabled"),
				}

				differs := TeamDiffers(team, githubTeam, "my-org")

				Expect(differs).To(BeTrue())
			})
		})

		Context("when permission differs", func() {
			It("should return true for push permission", func() {
				githubTeam := &github.Team{
					Name:                new("my-team"),
					Description:         new("Test team description"),
					Privacy:             new("closed"),
					Permission:          new("push"),
					NotificationSetting: new("notifications_disabled"),
				}

				differs := TeamDiffers(team, githubTeam, "my-org")

				Expect(differs).To(BeTrue())
			})

			It("should return true when permission is nil", func() {
				githubTeam := &github.Team{
					Name:                new("my-team"),
					Description:         new("Test team description"),
					Privacy:             new("closed"),
					Permission:          nil,
					NotificationSetting: new("notifications_disabled"),
				}

				differs := TeamDiffers(team, githubTeam, "my-org")

				Expect(differs).To(BeTrue())
			})
		})

		Context("when notification setting differs", func() {
			It("should return true for notifications enabled", func() {
				githubTeam := &github.Team{
					Name:                new("my-team"),
					Description:         new("Test team description"),
					Privacy:             new("closed"),
					Permission:          new("pull"),
					NotificationSetting: new("notifications_enabled"),
				}

				differs := TeamDiffers(team, githubTeam, "my-org")

				Expect(differs).To(BeTrue())
			})

			It("should return true when notification setting is nil", func() {
				githubTeam := &github.Team{
					Name:                new("my-team"),
					Description:         new("Test team description"),
					Privacy:             new("closed"),
					Permission:          new("pull"),
					NotificationSetting: nil,
				}

				differs := TeamDiffers(team, githubTeam, "my-org")

				Expect(differs).To(BeTrue())
			})
		})

		Context("when team has no description", func() {
			It("should compare against empty description", func() {
				team.Spec.Description = ""
				githubTeam := &github.Team{
					Name:                new("my-team"),
					Description:         new(""),
					Privacy:             new("closed"),
					Permission:          new("pull"),
					NotificationSetting: new("notifications_disabled"),
				}

				differs := TeamDiffers(team, githubTeam, "my-org")

				Expect(differs).To(BeFalse())
			})
		})

		Context("when multiple fields differ", func() {
			It("should return true", func() {
				githubTeam := &github.Team{
					Name:                new("different-team"),
					Description:         new("Different description"),
					Privacy:             new("secret"),
					Permission:          new("admin"),
					NotificationSetting: new("notifications_enabled"),
				}

				differs := TeamDiffers(team, githubTeam, "my-org")

				Expect(differs).To(BeTrue())
			})
		})

		Context("when team uses custom privacy 'secret'", func() {
			It("should return false when GitHub matches", func() {
				team.Spec.Privacy = "secret"
				githubTeam := &github.Team{
					Name:                new("my-team"),
					Description:         new("Test team description"),
					Privacy:             new("secret"),
					Permission:          new("pull"),
					NotificationSetting: new("notifications_disabled"),
				}

				differs := TeamDiffers(team, githubTeam, "my-org")

				Expect(differs).To(BeFalse())
			})

			It("should return true when GitHub has different privacy", func() {
				team.Spec.Privacy = "secret"
				githubTeam := &github.Team{
					Name:                new("my-team"),
					Description:         new("Test team description"),
					Privacy:             new("closed"),
					Permission:          new("pull"),
					NotificationSetting: new("notifications_disabled"),
				}

				differs := TeamDiffers(team, githubTeam, "my-org")

				Expect(differs).To(BeTrue())
			})
		})

		Context("when team uses custom permission 'push'", func() {
			It("should return false when GitHub matches", func() {
				team.Spec.Permission = "push"
				githubTeam := &github.Team{
					Name:                new("my-team"),
					Description:         new("Test team description"),
					Privacy:             new("closed"),
					Permission:          new("push"),
					NotificationSetting: new("notifications_disabled"),
				}

				differs := TeamDiffers(team, githubTeam, "my-org")

				Expect(differs).To(BeFalse())
			})
		})

		Context("when team uses custom notification setting", func() {
			It("should return false when GitHub matches", func() {
				team.Spec.NotificationSetting = "notifications_enabled"
				githubTeam := &github.Team{
					Name:                new("my-team"),
					Description:         new("Test team description"),
					Privacy:             new("closed"),
					Permission:          new("pull"),
					NotificationSetting: new("notifications_enabled"),
				}

				differs := TeamDiffers(team, githubTeam, "my-org")

				Expect(differs).To(BeFalse())
			})
		})

		Context("when all team settings are customized", func() {
			It("should return false when all match", func() {
				team.Spec.Privacy = "secret"
				team.Spec.Permission = "push"
				team.Spec.NotificationSetting = "notifications_enabled"
				githubTeam := &github.Team{
					Name:                new("my-team"),
					Description:         new("Test team description"),
					Privacy:             new("secret"),
					Permission:          new("push"),
					NotificationSetting: new("notifications_enabled"),
				}

				differs := TeamDiffers(team, githubTeam, "my-org")

				Expect(differs).To(BeFalse())
			})
		})
	})

	Describe("TeamNameToSlug", func() {
		Context("when converting simple team names", func() {
			It("should convert to lowercase", func() {
				slug := TeamNameToSlug("MyTeam")
				Expect(*slug).To(Equal("myteam"))
			})

			It("should convert spaces to hyphens", func() {
				slug := TeamNameToSlug("My Team")
				Expect(*slug).To(Equal("my-team"))
			})

			It("should handle multiple spaces", func() {
				slug := TeamNameToSlug("My   Team   Name")
				Expect(*slug).To(Equal("my-team-name"))
			})
		})

		Context("when converting team names with special characters", func() {
			It("should remove special characters", func() {
				slug := TeamNameToSlug("My@Team#Name")
				Expect(*slug).To(Equal("myteamname"))
			})

			It("should remove parentheses and brackets", func() {
				slug := TeamNameToSlug("Team (A) [B]")
				Expect(*slug).To(Equal("team-a-b"))
			})

			It("should remove punctuation", func() {
				slug := TeamNameToSlug("Team.Name,Here!")
				Expect(*slug).To(Equal("teamnamehere"))
			})
		})

		Context("when converting team names with accented characters", func() {
			It("should normalize accented characters to base form", func() {
				slug := TeamNameToSlug("My Téam Nâme")
				Expect(*slug).To(Equal("my-team-name"))
			})

			It("should handle German umlauts", func() {
				slug := TeamNameToSlug("Mein Täam Nämé")
				Expect(*slug).To(Equal("mein-taam-name"))
			})

			It("should handle various diacritics", func() {
				slug := TeamNameToSlug("Café Résumé")
				Expect(*slug).To(Equal("cafe-resume"))
			})
		})

		Context("when converting team names with hyphens", func() {
			It("should preserve single hyphens", func() {
				slug := TeamNameToSlug("my-team")
				Expect(*slug).To(Equal("my-team"))
			})

			It("should collapse multiple consecutive hyphens", func() {
				slug := TeamNameToSlug("my---team")
				Expect(*slug).To(Equal("my-team"))
			})

			It("should trim leading hyphens", func() {
				slug := TeamNameToSlug("-my-team")
				Expect(*slug).To(Equal("my-team"))
			})

			It("should trim trailing hyphens", func() {
				slug := TeamNameToSlug("my-team-")
				Expect(*slug).To(Equal("my-team"))
			})

			It("should trim both leading and trailing hyphens", func() {
				slug := TeamNameToSlug("--my-team--")
				Expect(*slug).To(Equal("my-team"))
			})
		})

		Context("when converting team names with numbers", func() {
			It("should preserve numbers", func() {
				slug := TeamNameToSlug("Team 123")
				Expect(*slug).To(Equal("team-123"))
			})

			It("should handle mixed alphanumeric", func() {
				slug := TeamNameToSlug("Team2023Alpha")
				Expect(*slug).To(Equal("team2023alpha"))
			})
		})

		Context("when converting edge cases", func() {
			It("should handle all uppercase", func() {
				slug := TeamNameToSlug("MYTEAM")
				Expect(*slug).To(Equal("myteam"))
			})

			It("should handle mixed case", func() {
				slug := TeamNameToSlug("MyTEAMName")
				Expect(*slug).To(Equal("myteamname"))
			})

			It("should handle team name with only special characters", func() {
				slug := TeamNameToSlug("@#$%")
				Expect(*slug).To(Equal(""))
			})

			It("should handle team name with spaces and special chars only", func() {
				slug := TeamNameToSlug("  @#$  %^&  ")
				Expect(*slug).To(Equal(""))
			})

			It("should handle single word", func() {
				slug := TeamNameToSlug("team")
				Expect(*slug).To(Equal("team"))
			})

			It("should handle empty string", func() {
				slug := TeamNameToSlug("")
				Expect(*slug).To(Equal(""))
			})
		})

		Context("when converting complex real-world team names", func() {
			It("should handle 'My TEam Näme' example from code", func() {
				slug := TeamNameToSlug("My TEam Näme")
				Expect(*slug).To(Equal("my-team-name"))
			})

			It("should handle department-style names", func() {
				slug := TeamNameToSlug("Engineering & Operations Team")
				Expect(*slug).To(Equal("engineering-operations-team"))
			})

			It("should handle project code names", func() {
				slug := TeamNameToSlug("Project-X Team (2024)")
				Expect(*slug).To(Equal("project-x-team-2024"))
			})

			It("should handle names with apostrophes", func() {
				slug := TeamNameToSlug("John's Team")
				Expect(*slug).To(Equal("johns-team"))
			})
		})
	})
})
