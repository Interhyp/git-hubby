package mapper

import (
	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/google/go-github/v86/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GitHub Actions Configuration Mapper", func() {

	Describe("EqualActionsPermissions", func() {
		Context("when both are nil", func() {
			It("should return true", func() {
				result := EqualActionsPermissions(nil, nil)
				Expect(result).To(BeTrue())
			})
		})

		Context("when only configuration is nil", func() {
			It("should return false", func() {
				current := &github.ActionsPermissions{
					EnabledRepositories: github.Ptr("all"),
				}
				result := EqualActionsPermissions(nil, current)
				Expect(result).To(BeFalse())
			})
		})

		Context("when only current is nil", func() {
			It("should return false", func() {
				configuration := &github.ActionsPermissions{
					EnabledRepositories: github.Ptr("all"),
				}
				result := EqualActionsPermissions(configuration, nil)
				Expect(result).To(BeFalse())
			})
		})

		Context("when both are equal", func() {
			It("should return true for identical permissions", func() {
				configuration := &github.ActionsPermissions{
					EnabledRepositories: github.Ptr("all"),
					AllowedActions:      github.Ptr("all"),
					SHAPinningRequired:  github.Ptr(true),
				}
				current := &github.ActionsPermissions{
					EnabledRepositories: github.Ptr("all"),
					AllowedActions:      github.Ptr("all"),
					SHAPinningRequired:  github.Ptr(true),
				}
				result := EqualActionsPermissions(configuration, current)
				Expect(result).To(BeTrue())
			})

			It("should return true when all fields are nil", func() {
				configuration := &github.ActionsPermissions{}
				current := &github.ActionsPermissions{}
				result := EqualActionsPermissions(configuration, current)
				Expect(result).To(BeTrue())
			})
		})

		Context("when SHAPinningRequired differs", func() {
			It("should return false", func() {
				configuration := &github.ActionsPermissions{
					SHAPinningRequired: github.Ptr(true),
				}
				current := &github.ActionsPermissions{
					SHAPinningRequired: github.Ptr(false),
				}
				result := EqualActionsPermissions(configuration, current)
				Expect(result).To(BeFalse())
			})

			It("should return false when one is nil", func() {
				configuration := &github.ActionsPermissions{
					SHAPinningRequired: github.Ptr(true),
				}
				current := &github.ActionsPermissions{
					SHAPinningRequired: nil,
				}
				result := EqualActionsPermissions(configuration, current)
				Expect(result).To(BeFalse())
			})
		})

		Context("when AllowedActions differs", func() {
			It("should return false", func() {
				configuration := &github.ActionsPermissions{
					AllowedActions: github.Ptr("all"),
				}
				current := &github.ActionsPermissions{
					AllowedActions: github.Ptr("selected"),
				}
				result := EqualActionsPermissions(configuration, current)
				Expect(result).To(BeFalse())
			})

			It("should return false when one is nil", func() {
				configuration := &github.ActionsPermissions{
					AllowedActions: github.Ptr("all"),
				}
				current := &github.ActionsPermissions{
					AllowedActions: nil,
				}
				result := EqualActionsPermissions(configuration, current)
				Expect(result).To(BeFalse())
			})
		})

		Context("when EnabledRepositories differs", func() {
			It("should return false", func() {
				configuration := &github.ActionsPermissions{
					EnabledRepositories: github.Ptr("all"),
				}
				current := &github.ActionsPermissions{
					EnabledRepositories: github.Ptr("selected"),
				}
				result := EqualActionsPermissions(configuration, current)
				Expect(result).To(BeFalse())
			})

			It("should return false when one is nil", func() {
				configuration := &github.ActionsPermissions{
					EnabledRepositories: github.Ptr("all"),
				}
				current := &github.ActionsPermissions{
					EnabledRepositories: nil,
				}
				result := EqualActionsPermissions(configuration, current)
				Expect(result).To(BeFalse())
			})
		})

		Context("when multiple fields differ", func() {
			It("should return false", func() {
				configuration := &github.ActionsPermissions{
					EnabledRepositories: github.Ptr("all"),
					AllowedActions:      github.Ptr("all"),
					SHAPinningRequired:  github.Ptr(true),
				}
				current := &github.ActionsPermissions{
					EnabledRepositories: github.Ptr("selected"),
					AllowedActions:      github.Ptr("selected"),
					SHAPinningRequired:  github.Ptr(false),
				}
				result := EqualActionsPermissions(configuration, current)
				Expect(result).To(BeFalse())
			})
		})
	})

	Describe("EqualActionsAllowed", func() {
		Context("when both are nil", func() {
			It("should return true", func() {
				result := EqualActionsAllowed(nil, nil)
				Expect(result).To(BeTrue())
			})
		})

		Context("when only actions is nil", func() {
			It("should return false", func() {
				current := &github.ActionsAllowed{
					GithubOwnedAllowed: github.Ptr(true),
				}
				result := EqualActionsAllowed(nil, current)
				Expect(result).To(BeFalse())
			})
		})

		Context("when only current is nil", func() {
			It("should return false", func() {
				actions := &github.ActionsAllowed{
					GithubOwnedAllowed: github.Ptr(true),
				}
				result := EqualActionsAllowed(actions, nil)
				Expect(result).To(BeFalse())
			})
		})

		Context("when both are equal", func() {
			It("should return true for identical actions", func() {
				actions := &github.ActionsAllowed{
					GithubOwnedAllowed: github.Ptr(true),
					VerifiedAllowed:    github.Ptr(true),
					PatternsAllowed:    []string{"org/*", "user/repo@*"},
				}
				current := &github.ActionsAllowed{
					GithubOwnedAllowed: github.Ptr(true),
					VerifiedAllowed:    github.Ptr(true),
					PatternsAllowed:    []string{"org/*", "user/repo@*"},
				}
				result := EqualActionsAllowed(actions, current)
				Expect(result).To(BeTrue())
			})

			It("should return true when all fields are nil", func() {
				actions := &github.ActionsAllowed{}
				current := &github.ActionsAllowed{}
				result := EqualActionsAllowed(actions, current)
				Expect(result).To(BeTrue())
			})

			It("should return true for patterns in different order", func() {
				actions := &github.ActionsAllowed{
					PatternsAllowed: []string{"org/*", "user/repo@*"},
				}
				current := &github.ActionsAllowed{
					PatternsAllowed: []string{"user/repo@*", "org/*"},
				}
				result := EqualActionsAllowed(actions, current)
				Expect(result).To(BeTrue())
			})
		})

		Context("when GithubOwnedAllowed differs", func() {
			It("should return false", func() {
				actions := &github.ActionsAllowed{
					GithubOwnedAllowed: github.Ptr(true),
				}
				current := &github.ActionsAllowed{
					GithubOwnedAllowed: github.Ptr(false),
				}
				result := EqualActionsAllowed(actions, current)
				Expect(result).To(BeFalse())
			})

			It("should return false when one is nil", func() {
				actions := &github.ActionsAllowed{
					GithubOwnedAllowed: github.Ptr(true),
				}
				current := &github.ActionsAllowed{
					GithubOwnedAllowed: nil,
				}
				result := EqualActionsAllowed(actions, current)
				Expect(result).To(BeFalse())
			})
		})

		Context("when VerifiedAllowed differs", func() {
			It("should return false", func() {
				actions := &github.ActionsAllowed{
					VerifiedAllowed: github.Ptr(true),
				}
				current := &github.ActionsAllowed{
					VerifiedAllowed: github.Ptr(false),
				}
				result := EqualActionsAllowed(actions, current)
				Expect(result).To(BeFalse())
			})

			It("should return false when one is nil", func() {
				actions := &github.ActionsAllowed{
					VerifiedAllowed: github.Ptr(true),
				}
				current := &github.ActionsAllowed{
					VerifiedAllowed: nil,
				}
				result := EqualActionsAllowed(actions, current)
				Expect(result).To(BeFalse())
			})
		})

		Context("when PatternsAllowed differs", func() {
			It("should return false for different patterns", func() {
				actions := &github.ActionsAllowed{
					PatternsAllowed: []string{"org/*"},
				}
				current := &github.ActionsAllowed{
					PatternsAllowed: []string{"user/*"},
				}
				result := EqualActionsAllowed(actions, current)
				Expect(result).To(BeFalse())
			})

			It("should return false for different number of patterns", func() {
				actions := &github.ActionsAllowed{
					PatternsAllowed: []string{"org/*", "user/*"},
				}
				current := &github.ActionsAllowed{
					PatternsAllowed: []string{"org/*"},
				}
				result := EqualActionsAllowed(actions, current)
				Expect(result).To(BeFalse())
			})

			It("should return false when one is nil and other is empty", func() {
				actions := &github.ActionsAllowed{
					PatternsAllowed: []string{},
				}
				current := &github.ActionsAllowed{
					PatternsAllowed: nil,
				}
				result := EqualActionsAllowed(actions, current)
				Expect(result).To(BeFalse())
			})

			It("should return true when both are nil", func() {
				actions := &github.ActionsAllowed{
					PatternsAllowed: nil,
				}
				current := &github.ActionsAllowed{
					PatternsAllowed: nil,
				}
				result := EqualActionsAllowed(actions, current)
				Expect(result).To(BeTrue())
			})

			It("should return true when both are empty", func() {
				actions := &github.ActionsAllowed{
					PatternsAllowed: []string{},
				}
				current := &github.ActionsAllowed{
					PatternsAllowed: []string{},
				}
				result := EqualActionsAllowed(actions, current)
				Expect(result).To(BeTrue())
			})
		})

		Context("when multiple fields differ", func() {
			It("should return false", func() {
				actions := &github.ActionsAllowed{
					GithubOwnedAllowed: github.Ptr(true),
					VerifiedAllowed:    github.Ptr(true),
					PatternsAllowed:    []string{"org/*"},
				}
				current := &github.ActionsAllowed{
					GithubOwnedAllowed: github.Ptr(false),
					VerifiedAllowed:    github.Ptr(false),
					PatternsAllowed:    []string{"user/*"},
				}
				result := EqualActionsAllowed(actions, current)
				Expect(result).To(BeFalse())
			})
		})
	})

	Describe("EqualDefaultWorkflowPermissions", func() {
		Context("when both are nil", func() {
			It("should return true", func() {
				result := EqualDefaultWorkflowPermissions(nil, nil)
				Expect(result).To(BeTrue())
			})
		})

		Context("when only expected is nil", func() {
			It("should return false", func() {
				current := &github.DefaultWorkflowPermissionOrganization{
					DefaultWorkflowPermissions: github.Ptr("read"),
				}
				result := EqualDefaultWorkflowPermissions(nil, current)
				Expect(result).To(BeFalse())
			})
		})

		Context("when only current is nil", func() {
			It("should return false", func() {
				expected := &github.DefaultWorkflowPermissionOrganization{
					DefaultWorkflowPermissions: github.Ptr("read"),
				}
				result := EqualDefaultWorkflowPermissions(expected, nil)
				Expect(result).To(BeFalse())
			})
		})

		Context("when both are equal", func() {
			It("should return true for identical permissions", func() {
				expected := &github.DefaultWorkflowPermissionOrganization{
					DefaultWorkflowPermissions:   github.Ptr("read"),
					CanApprovePullRequestReviews: github.Ptr(true),
				}
				current := &github.DefaultWorkflowPermissionOrganization{
					DefaultWorkflowPermissions:   github.Ptr("read"),
					CanApprovePullRequestReviews: github.Ptr(true),
				}
				result := EqualDefaultWorkflowPermissions(expected, current)
				Expect(result).To(BeTrue())
			})

			It("should return true when all fields are nil", func() {
				expected := &github.DefaultWorkflowPermissionOrganization{}
				current := &github.DefaultWorkflowPermissionOrganization{}
				result := EqualDefaultWorkflowPermissions(expected, current)
				Expect(result).To(BeTrue())
			})
		})

		Context("when DefaultWorkflowPermissions differs", func() {
			It("should return false for different values", func() {
				expected := &github.DefaultWorkflowPermissionOrganization{
					DefaultWorkflowPermissions: github.Ptr("read"),
				}
				current := &github.DefaultWorkflowPermissionOrganization{
					DefaultWorkflowPermissions: github.Ptr("write"),
				}
				result := EqualDefaultWorkflowPermissions(expected, current)
				Expect(result).To(BeFalse())
			})

			It("should return false when one is nil", func() {
				expected := &github.DefaultWorkflowPermissionOrganization{
					DefaultWorkflowPermissions: github.Ptr("read"),
				}
				current := &github.DefaultWorkflowPermissionOrganization{
					DefaultWorkflowPermissions: nil,
				}
				result := EqualDefaultWorkflowPermissions(expected, current)
				Expect(result).To(BeFalse())
			})
		})

		Context("when CanApprovePullRequestReviews differs", func() {
			It("should return false for different values", func() {
				expected := &github.DefaultWorkflowPermissionOrganization{
					CanApprovePullRequestReviews: github.Ptr(true),
				}
				current := &github.DefaultWorkflowPermissionOrganization{
					CanApprovePullRequestReviews: github.Ptr(false),
				}
				result := EqualDefaultWorkflowPermissions(expected, current)
				Expect(result).To(BeFalse())
			})

			It("should return false when one is nil", func() {
				expected := &github.DefaultWorkflowPermissionOrganization{
					CanApprovePullRequestReviews: github.Ptr(true),
				}
				current := &github.DefaultWorkflowPermissionOrganization{
					CanApprovePullRequestReviews: nil,
				}
				result := EqualDefaultWorkflowPermissions(expected, current)
				Expect(result).To(BeFalse())
			})
		})

		Context("when both fields differ", func() {
			It("should return false", func() {
				expected := &github.DefaultWorkflowPermissionOrganization{
					DefaultWorkflowPermissions:   github.Ptr("read"),
					CanApprovePullRequestReviews: github.Ptr(true),
				}
				current := &github.DefaultWorkflowPermissionOrganization{
					DefaultWorkflowPermissions:   github.Ptr("write"),
					CanApprovePullRequestReviews: github.Ptr(false),
				}
				result := EqualDefaultWorkflowPermissions(expected, current)
				Expect(result).To(BeFalse())
			})
		})
	})

	Describe("EqualRunnerGroup", func() {
		Context("when all fields are equal", func() {
			It("should return true for identical runner groups", func() {
				k8sGroup := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(false),
					SelectedWorkflows:     nil,
				}
				ghGroup := &github.RunnerGroup{
					Name:                  github.Ptr("test-group"),
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(false),
					SelectedWorkflows:     nil,
				}
				result := EqualRunnerGroup(k8sGroup, ghGroup)
				Expect(result).To(BeTrue())
			})

			It("should return true when RestrictedToWorkflows is false and SelectedWorkflows differ", func() {
				k8sGroup := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("private"),
					RestrictedToWorkflows: github.Ptr(false),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/deploy.yaml@refs/heads/main"},
				}
				ghGroup := &github.RunnerGroup{
					Name:                  github.Ptr("test-group"),
					Visibility:            github.Ptr("private"),
					RestrictedToWorkflows: github.Ptr(false),
					SelectedWorkflows:     []string{"different/workflow@refs/heads/main"},
				}
				result := EqualRunnerGroup(k8sGroup, ghGroup)
				Expect(result).To(BeTrue())
			})

			It("should return true when RestrictedToWorkflows is true and SelectedWorkflows are identical", func() {
				k8sGroup := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("selected"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/ci.yaml@refs/heads/main"},
				}
				ghGroup := &github.RunnerGroup{
					Name:                  github.Ptr("test-group"),
					Visibility:            github.Ptr("selected"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/ci.yaml@refs/heads/main"},
				}
				result := EqualRunnerGroup(k8sGroup, ghGroup)
				Expect(result).To(BeTrue())
			})

			It("should return true when RestrictedToWorkflows is true and SelectedWorkflows are in different order", func() {
				k8sGroup := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("selected"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/a.yaml@main", "org/repo/.github/workflows/b.yaml@main"},
				}
				ghGroup := &github.RunnerGroup{
					Name:                  github.Ptr("test-group"),
					Visibility:            github.Ptr("selected"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/b.yaml@main", "org/repo/.github/workflows/a.yaml@main"},
				}
				result := EqualRunnerGroup(k8sGroup, ghGroup)
				Expect(result).To(BeTrue())
			})

			It("should return true when RestrictedToWorkflows is nil in k8s and false in GitHub", func() {
				k8sGroup := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: nil,
					SelectedWorkflows:     nil,
				}
				ghGroup := &github.RunnerGroup{
					Name:                  github.Ptr("test-group"),
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(false),
					SelectedWorkflows:     nil,
				}
				result := EqualRunnerGroup(k8sGroup, ghGroup)
				Expect(result).To(BeFalse()) // nil != false
			})
		})

		Context("when names differ", func() {
			It("should return false", func() {
				k8sGroup := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(false),
				}
				ghGroup := &github.RunnerGroup{
					Name:                  github.Ptr("different-group"),
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(false),
				}
				result := EqualRunnerGroup(k8sGroup, ghGroup)
				Expect(result).To(BeFalse())
			})
		})

		Context("when visibility differs", func() {
			It("should return false", func() {
				k8sGroup := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(false),
				}
				ghGroup := &github.RunnerGroup{
					Name:                  github.Ptr("test-group"),
					Visibility:            github.Ptr("selected"),
					RestrictedToWorkflows: github.Ptr(false),
				}
				result := EqualRunnerGroup(k8sGroup, ghGroup)
				Expect(result).To(BeFalse())
			})

			It("should return false when one visibility is nil", func() {
				k8sGroup := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(false),
				}
				ghGroup := &github.RunnerGroup{
					Name:                  github.Ptr("test-group"),
					Visibility:            nil,
					RestrictedToWorkflows: github.Ptr(false),
				}
				result := EqualRunnerGroup(k8sGroup, ghGroup)
				Expect(result).To(BeFalse())
			})
		})

		Context("when RestrictedToWorkflows differs", func() {
			It("should return false when values are different", func() {
				k8sGroup := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/ci.yaml@main"},
				}
				ghGroup := &github.RunnerGroup{
					Name:                  github.Ptr("test-group"),
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(false),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/ci.yaml@main"},
				}
				result := EqualRunnerGroup(k8sGroup, ghGroup)
				Expect(result).To(BeFalse())
			})

			It("should return false when one is nil", func() {
				k8sGroup := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/ci.yaml@main"},
				}
				ghGroup := &github.RunnerGroup{
					Name:                  github.Ptr("test-group"),
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: nil,
					SelectedWorkflows:     []string{"org/repo/.github/workflows/ci.yaml@main"},
				}
				result := EqualRunnerGroup(k8sGroup, ghGroup)
				Expect(result).To(BeFalse())
			})
		})

		Context("when RestrictedToWorkflows is true and SelectedWorkflows differ", func() {
			It("should return false when workflows are different", func() {
				k8sGroup := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/ci.yaml@main"},
				}
				ghGroup := &github.RunnerGroup{
					Name:                  github.Ptr("test-group"),
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/deploy.yaml@main"},
				}
				result := EqualRunnerGroup(k8sGroup, ghGroup)
				Expect(result).To(BeFalse())
			})

			It("should return false when number of workflows differs", func() {
				k8sGroup := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/ci.yaml@main", "org/repo/.github/workflows/deploy.yaml@main"},
				}
				ghGroup := &github.RunnerGroup{
					Name:                  github.Ptr("test-group"),
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/ci.yaml@main"},
				}
				result := EqualRunnerGroup(k8sGroup, ghGroup)
				Expect(result).To(BeFalse())
			})

			It("should return false when one has nil workflows and other has workflows", func() {
				k8sGroup := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/ci.yaml@main"},
				}
				ghGroup := &github.RunnerGroup{
					Name:                  github.Ptr("test-group"),
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows:     nil,
				}
				result := EqualRunnerGroup(k8sGroup, ghGroup)
				Expect(result).To(BeFalse())
			})

			It("should return true when both have empty workflows", func() {
				k8sGroup := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows:     []string{},
				}
				ghGroup := &github.RunnerGroup{
					Name:                  github.Ptr("test-group"),
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows:     []string{},
				}
				result := EqualRunnerGroup(k8sGroup, ghGroup)
				Expect(result).To(BeTrue())
			})
		})

		Context("when multiple fields differ", func() {
			It("should return false", func() {
				k8sGroup := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/ci.yaml@main"},
				}
				ghGroup := &github.RunnerGroup{
					Name:                  github.Ptr("different-group"),
					Visibility:            github.Ptr("private"),
					RestrictedToWorkflows: github.Ptr(false),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/deploy.yaml@main"},
				}
				result := EqualRunnerGroup(k8sGroup, ghGroup)
				Expect(result).To(BeFalse())
			})
		})
	})

	Describe("MapRunnerGroupToCreateRequest", func() {
		Context("when visibility is 'all'", func() {
			It("should create request without selected repositories", func() {
				group := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(false),
					SelectedWorkflows:     nil,
				}
				repos := []v1alpha1.Repository{}

				result := MapRunnerGroupToCreateRequest(group, repos)

				Expect(result.Name).To(Equal(github.Ptr("test-group")))
				Expect(result.Visibility).To(Equal(github.Ptr("all")))
				Expect(result.RestrictedToWorkflows).To(Equal(github.Ptr(false)))
				Expect(result.SelectedWorkflows).To(BeNil())
				Expect(result.SelectedRepositoryIDs).To(BeNil())
			})

			It("should not include repositories even if they reference the runner group", func() {
				group := v1alpha1.RunnerGroup{
					Name:       "test-group",
					Visibility: github.Ptr("all"),
				}
				repos := []v1alpha1.Repository{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo1"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"test-group"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(123)),
						},
					},
				}

				result := MapRunnerGroupToCreateRequest(group, repos)

				Expect(result.SelectedRepositoryIDs).To(BeNil())
			})
		})

		Context("when visibility is 'private'", func() {
			It("should create request without selected repositories", func() {
				group := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("private"),
					RestrictedToWorkflows: github.Ptr(false),
					SelectedWorkflows:     nil,
				}
				repos := []v1alpha1.Repository{}

				result := MapRunnerGroupToCreateRequest(group, repos)

				Expect(result.Name).To(Equal(github.Ptr("test-group")))
				Expect(result.Visibility).To(Equal(github.Ptr("private")))
				Expect(result.RestrictedToWorkflows).To(Equal(github.Ptr(false)))
				Expect(result.SelectedWorkflows).To(BeNil())
				Expect(result.SelectedRepositoryIDs).To(BeNil())
			})
		})

		Context("when visibility is 'selected'", func() {
			It("should include repositories that reference the runner group with ID", func() {
				group := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("selected"),
					RestrictedToWorkflows: github.Ptr(false),
				}
				repos := []v1alpha1.Repository{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo1"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"test-group"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(123)),
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo2"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"test-group"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(456)),
						},
					},
				}

				result := MapRunnerGroupToCreateRequest(group, repos)

				Expect(result.Name).To(Equal(github.Ptr("test-group")))
				Expect(result.Visibility).To(Equal(github.Ptr("selected")))
				Expect(result.SelectedRepositoryIDs).To(HaveLen(2))
				Expect(result.SelectedRepositoryIDs).To(ContainElements(int64(123), int64(456)))
			})

			It("should exclude repositories without ID", func() {
				group := v1alpha1.RunnerGroup{
					Name:       "test-group",
					Visibility: github.Ptr("selected"),
				}
				repos := []v1alpha1.Repository{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo1"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"test-group"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(123)),
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo2-no-id"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"test-group"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: nil, // No ID yet
						},
					},
				}

				result := MapRunnerGroupToCreateRequest(group, repos)

				Expect(result.SelectedRepositoryIDs).To(HaveLen(1))
				Expect(result.SelectedRepositoryIDs).To(ContainElements(int64(123)))
			})

			It("should exclude repositories that don't reference the runner group", func() {
				group := v1alpha1.RunnerGroup{
					Name:       "test-group",
					Visibility: github.Ptr("selected"),
				}
				repos := []v1alpha1.Repository{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo1"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"test-group"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(123)),
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo2"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"other-group"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(456)),
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo3"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: nil,
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(789)),
						},
					},
				}

				result := MapRunnerGroupToCreateRequest(group, repos)

				Expect(result.SelectedRepositoryIDs).To(HaveLen(1))
				Expect(result.SelectedRepositoryIDs).To(ContainElements(int64(123)))
			})

			It("should return empty slice when no repositories match", func() {
				group := v1alpha1.RunnerGroup{
					Name:       "test-group",
					Visibility: github.Ptr("selected"),
				}
				repos := []v1alpha1.Repository{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo1"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"other-group"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(123)),
						},
					},
				}

				result := MapRunnerGroupToCreateRequest(group, repos)

				Expect(result.SelectedRepositoryIDs).To(BeEmpty())
				Expect(result.SelectedRepositoryIDs).NotTo(BeNil())
			})

			It("should initialize empty slice when no repos provided", func() {
				group := v1alpha1.RunnerGroup{
					Name:       "test-group",
					Visibility: github.Ptr("selected"),
				}
				repos := []v1alpha1.Repository{}

				result := MapRunnerGroupToCreateRequest(group, repos)

				Expect(result.SelectedRepositoryIDs).To(BeEmpty())
				Expect(result.SelectedRepositoryIDs).NotTo(BeNil())
			})
		})

		Context("when RestrictedToWorkflows is true", func() {
			It("should include selected workflows", func() {
				group := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/ci.yaml@refs/heads/main"},
				}
				repos := []v1alpha1.Repository{}

				result := MapRunnerGroupToCreateRequest(group, repos)

				Expect(result.Name).To(Equal(github.Ptr("test-group")))
				Expect(result.RestrictedToWorkflows).To(Equal(github.Ptr(true)))
				Expect(result.SelectedWorkflows).To(Equal([]string{"org/repo/.github/workflows/ci.yaml@refs/heads/main"}))
			})

			It("should include multiple selected workflows", func() {
				group := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows: []string{
						"org/repo/.github/workflows/ci.yaml@refs/heads/main",
						"org/repo/.github/workflows/deploy.yaml@refs/tags/v1.0.0",
					},
				}
				repos := []v1alpha1.Repository{}

				result := MapRunnerGroupToCreateRequest(group, repos)

				Expect(result.RestrictedToWorkflows).To(Equal(github.Ptr(true)))
				Expect(result.SelectedWorkflows).To(HaveLen(2))
				Expect(result.SelectedWorkflows).To(ContainElements(
					"org/repo/.github/workflows/ci.yaml@refs/heads/main",
					"org/repo/.github/workflows/deploy.yaml@refs/tags/v1.0.0",
				))
			})
		})

		Context("when RestrictedToWorkflows is false", func() {
			It("should not include selected workflows", func() {
				group := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(false),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/ci.yaml@refs/heads/main"},
				}
				repos := []v1alpha1.Repository{}

				result := MapRunnerGroupToCreateRequest(group, repos)

				Expect(result.RestrictedToWorkflows).To(Equal(github.Ptr(false)))
				Expect(result.SelectedWorkflows).To(Equal([]string{"org/repo/.github/workflows/ci.yaml@refs/heads/main"}))
			})
		})

		Context("when RestrictedToWorkflows is nil", func() {
			It("should pass nil RestrictedToWorkflows", func() {
				group := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: nil,
					SelectedWorkflows:     []string{"org/repo/.github/workflows/ci.yaml@refs/heads/main"},
				}
				repos := []v1alpha1.Repository{}

				result := MapRunnerGroupToCreateRequest(group, repos)

				Expect(result.RestrictedToWorkflows).To(BeNil())
				Expect(result.SelectedWorkflows).To(Equal([]string{"org/repo/.github/workflows/ci.yaml@refs/heads/main"}))
			})
		})

		Context("comprehensive scenarios", func() {
			It("should handle selected visibility with workflows and matching repos", func() {
				group := v1alpha1.RunnerGroup{
					Name:                  "production-runners",
					Visibility:            github.Ptr("selected"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows: []string{
						"org/repo1/.github/workflows/deploy.yaml@refs/heads/main",
					},
				}
				repos := []v1alpha1.Repository{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo1"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"production-runners"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(100)),
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo2"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"production-runners", "staging-runners"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(200)),
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo3-no-id"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"production-runners"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: nil,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo4"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"staging-runners"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(400)),
						},
					},
				}

				result := MapRunnerGroupToCreateRequest(group, repos)

				Expect(result.Name).To(Equal(github.Ptr("production-runners")))
				Expect(result.Visibility).To(Equal(github.Ptr("selected")))
				Expect(result.RestrictedToWorkflows).To(Equal(github.Ptr(true)))
				Expect(result.SelectedWorkflows).To(Equal([]string{
					"org/repo1/.github/workflows/deploy.yaml@refs/heads/main",
				}))
				Expect(result.SelectedRepositoryIDs).To(HaveLen(2))
				Expect(result.SelectedRepositoryIDs).To(ContainElements(int64(100), int64(200)))
			})

			It("should handle empty group configuration", func() {
				group := v1alpha1.RunnerGroup{
					Name: "minimal-group",
				}
				repos := []v1alpha1.Repository{}

				result := MapRunnerGroupToCreateRequest(group, repos)

				Expect(result.Name).To(Equal(github.Ptr("minimal-group")))
				Expect(result.Visibility).To(BeNil())
				Expect(result.RestrictedToWorkflows).To(BeNil())
				Expect(result.SelectedWorkflows).To(BeNil())
				Expect(result.SelectedRepositoryIDs).To(BeNil())
			})
		})
	})

	Describe("GetSelectedRepositoryIDsForRunnerGroup", func() {
		Context("when visibility is not 'selected'", func() {
			It("should return nil for 'all' visibility", func() {
				group := v1alpha1.RunnerGroup{
					Name:       "all-runners",
					Visibility: github.Ptr("all"),
				}
				repos := []v1alpha1.Repository{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo1"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"all-runners"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(100)),
						},
					},
				}

				result := GetSelectedRepositoryIDsForRunnerGroup(group, repos)

				Expect(result).To(BeNil())
			})

			It("should return nil for 'private' visibility", func() {
				group := v1alpha1.RunnerGroup{
					Name:       "private-runners",
					Visibility: github.Ptr("private"),
				}
				repos := []v1alpha1.Repository{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo1"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"private-runners"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(100)),
						},
					},
				}

				result := GetSelectedRepositoryIDsForRunnerGroup(group, repos)

				Expect(result).To(BeNil())
			})

			It("should return nil when visibility is nil", func() {
				group := v1alpha1.RunnerGroup{
					Name:       "no-visibility",
					Visibility: nil,
				}
				repos := []v1alpha1.Repository{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo1"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"no-visibility"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(100)),
						},
					},
				}

				result := GetSelectedRepositoryIDsForRunnerGroup(group, repos)

				Expect(result).To(BeNil())
			})
		})

		Context("when visibility is 'selected'", func() {
			It("should return empty slice when no repos match", func() {
				group := v1alpha1.RunnerGroup{
					Name:       "selected-runners",
					Visibility: github.Ptr("selected"),
				}
				repos := []v1alpha1.Repository{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo1"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"other-runners"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(100)),
						},
					},
				}

				result := GetSelectedRepositoryIDsForRunnerGroup(group, repos)

				Expect(result).To(BeEmpty())
			})

			It("should return IDs of matching repos", func() {
				group := v1alpha1.RunnerGroup{
					Name:       "selected-runners",
					Visibility: github.Ptr("selected"),
				}
				repos := []v1alpha1.Repository{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo1"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"selected-runners"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(100)),
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo2"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"selected-runners"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(200)),
						},
					},
				}

				result := GetSelectedRepositoryIDsForRunnerGroup(group, repos)

				Expect(result).To(HaveLen(2))
				Expect(result).To(ContainElements(int64(100), int64(200)))
			})

			It("should skip repos without IDs (not yet reconciled)", func() {
				group := v1alpha1.RunnerGroup{
					Name:       "selected-runners",
					Visibility: github.Ptr("selected"),
				}
				repos := []v1alpha1.Repository{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo1"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"selected-runners"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(100)),
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo2-no-id"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"selected-runners"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: nil,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo3"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"selected-runners"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(300)),
						},
					},
				}

				result := GetSelectedRepositoryIDsForRunnerGroup(group, repos)

				Expect(result).To(HaveLen(2))
				Expect(result).To(ContainElements(int64(100), int64(300)))
				Expect(result).NotTo(ContainElement(int64(0)))
			})

			It("should handle repos with multiple runner groups", func() {
				group := v1alpha1.RunnerGroup{
					Name:       "production-runners",
					Visibility: github.Ptr("selected"),
				}
				repos := []v1alpha1.Repository{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo1"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"production-runners", "staging-runners"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(100)),
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo2"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"staging-runners"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(200)),
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "repo3"},
						Spec: v1alpha1.RepositorySpec{
							AvailableActionsRunnerGroups: []string{"production-runners"},
						},
						Status: v1alpha1.RepositoryStatus{
							ID: github.Ptr(int64(300)),
						},
					},
				}

				result := GetSelectedRepositoryIDsForRunnerGroup(group, repos)

				Expect(result).To(HaveLen(2))
				Expect(result).To(ContainElements(int64(100), int64(300)))
				Expect(result).NotTo(ContainElement(int64(200)))
			})

			It("should return empty slice when repos list is empty", func() {
				group := v1alpha1.RunnerGroup{
					Name:       "selected-runners",
					Visibility: github.Ptr("selected"),
				}
				repos := []v1alpha1.Repository{}

				result := GetSelectedRepositoryIDsForRunnerGroup(group, repos)

				Expect(result).To(BeEmpty())
			})
		})
	})

	Describe("MapRunnerGroupToUpdateRequest", func() {
		Context("with all fields set", func() {
			It("should map all fields correctly", func() {
				group := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("selected"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows: []string{
						"org/repo/.github/workflows/ci.yaml@refs/heads/main",
						"org/repo/.github/workflows/deploy.yaml@refs/heads/main",
					},
				}

				result := MapRunnerGroupToUpdateRequest(group)

				Expect(result.Name).To(Equal(github.Ptr("test-group")))
				Expect(result.Visibility).To(Equal(github.Ptr("selected")))
				Expect(result.RestrictedToWorkflows).To(Equal(github.Ptr(true)))
				Expect(result.SelectedWorkflows).To(HaveLen(2))
				Expect(result.SelectedWorkflows).To(ContainElements(
					"org/repo/.github/workflows/ci.yaml@refs/heads/main",
					"org/repo/.github/workflows/deploy.yaml@refs/heads/main",
				))
			})

			It("should map with visibility 'all'", func() {
				group := v1alpha1.RunnerGroup{
					Name:                  "all-runners",
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(false),
					SelectedWorkflows:     nil,
				}

				result := MapRunnerGroupToUpdateRequest(group)

				Expect(result.Name).To(Equal(github.Ptr("all-runners")))
				Expect(result.Visibility).To(Equal(github.Ptr("all")))
				Expect(result.RestrictedToWorkflows).To(Equal(github.Ptr(false)))
				Expect(result.SelectedWorkflows).To(BeNil())
			})

			It("should map with visibility 'private'", func() {
				group := v1alpha1.RunnerGroup{
					Name:                  "private-runners",
					Visibility:            github.Ptr("private"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/ci.yaml@main"},
				}

				result := MapRunnerGroupToUpdateRequest(group)

				Expect(result.Name).To(Equal(github.Ptr("private-runners")))
				Expect(result.Visibility).To(Equal(github.Ptr("private")))
				Expect(result.RestrictedToWorkflows).To(Equal(github.Ptr(true)))
				Expect(result.SelectedWorkflows).To(Equal([]string{"org/repo/.github/workflows/ci.yaml@main"}))
			})
		})

		Context("with nil fields", func() {
			It("should handle nil visibility", func() {
				group := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            nil,
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows:     []string{"org/repo/.github/workflows/ci.yaml@main"},
				}

				result := MapRunnerGroupToUpdateRequest(group)

				Expect(result.Name).To(Equal(github.Ptr("test-group")))
				Expect(result.Visibility).To(BeNil())
				Expect(result.RestrictedToWorkflows).To(Equal(github.Ptr(true)))
				Expect(result.SelectedWorkflows).To(Equal([]string{"org/repo/.github/workflows/ci.yaml@main"}))
			})

			It("should handle nil RestrictedToWorkflows", func() {
				group := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: nil,
					SelectedWorkflows:     []string{"org/repo/.github/workflows/ci.yaml@main"},
				}

				result := MapRunnerGroupToUpdateRequest(group)

				Expect(result.Name).To(Equal(github.Ptr("test-group")))
				Expect(result.Visibility).To(Equal(github.Ptr("all")))
				Expect(result.RestrictedToWorkflows).To(BeNil())
				Expect(result.SelectedWorkflows).To(Equal([]string{"org/repo/.github/workflows/ci.yaml@main"}))
			})

			It("should handle nil SelectedWorkflows", func() {
				group := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("selected"),
					RestrictedToWorkflows: github.Ptr(false),
					SelectedWorkflows:     nil,
				}

				result := MapRunnerGroupToUpdateRequest(group)

				Expect(result.Name).To(Equal(github.Ptr("test-group")))
				Expect(result.Visibility).To(Equal(github.Ptr("selected")))
				Expect(result.RestrictedToWorkflows).To(Equal(github.Ptr(false)))
				Expect(result.SelectedWorkflows).To(BeNil())
			})

			It("should handle empty SelectedWorkflows slice", func() {
				group := v1alpha1.RunnerGroup{
					Name:                  "test-group",
					Visibility:            github.Ptr("selected"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows:     []string{},
				}

				result := MapRunnerGroupToUpdateRequest(group)

				Expect(result.Name).To(Equal(github.Ptr("test-group")))
				Expect(result.Visibility).To(Equal(github.Ptr("selected")))
				Expect(result.RestrictedToWorkflows).To(Equal(github.Ptr(true)))
				Expect(result.SelectedWorkflows).To(BeEmpty())
			})
		})

		Context("minimal configuration", func() {
			It("should handle only name set", func() {
				group := v1alpha1.RunnerGroup{
					Name:                  "minimal-group",
					Visibility:            nil,
					RestrictedToWorkflows: nil,
					SelectedWorkflows:     nil,
				}

				result := MapRunnerGroupToUpdateRequest(group)

				Expect(result.Name).To(Equal(github.Ptr("minimal-group")))
				Expect(result.Visibility).To(BeNil())
				Expect(result.RestrictedToWorkflows).To(BeNil())
				Expect(result.SelectedWorkflows).To(BeNil())
			})
		})

		Context("with workflow restrictions", func() {
			It("should preserve workflow restrictions when enabled", func() {
				group := v1alpha1.RunnerGroup{
					Name:                  "restricted-group",
					Visibility:            github.Ptr("all"),
					RestrictedToWorkflows: github.Ptr(true),
					SelectedWorkflows: []string{
						"org/repo1/.github/workflows/ci.yaml@refs/heads/main",
						"org/repo2/.github/workflows/deploy.yaml@refs/tags/v1.0.0",
						"org/repo3/.github/workflows/test.yaml@refs/pull/123/merge",
					},
				}

				result := MapRunnerGroupToUpdateRequest(group)

				Expect(result.Name).To(Equal(github.Ptr("restricted-group")))
				Expect(result.Visibility).To(Equal(github.Ptr("all")))
				Expect(result.RestrictedToWorkflows).To(Equal(github.Ptr(true)))
				Expect(result.SelectedWorkflows).To(HaveLen(3))
				Expect(result.SelectedWorkflows).To(ContainElements(
					"org/repo1/.github/workflows/ci.yaml@refs/heads/main",
					"org/repo2/.github/workflows/deploy.yaml@refs/tags/v1.0.0",
					"org/repo3/.github/workflows/test.yaml@refs/pull/123/merge",
				))
			})
		})
	})

	Describe("EqualRepositoryIDs", func() {
		Context("when both are empty", func() {
			It("should return true for empty slices", func() {
				expectedIDs := []int64{}
				currentRepos := []*github.Repository{}

				result := EqualRepositoryIDs(expectedIDs, currentRepos)

				Expect(result).To(BeTrue())
			})

			It("should return true for nil expected and empty current", func() {
				var expectedIDs []int64
				currentRepos := []*github.Repository{}

				result := EqualRepositoryIDs(expectedIDs, currentRepos)

				Expect(result).To(BeTrue())
			})
		})

		Context("when lengths differ", func() {
			It("should return false when expected has more items", func() {
				expectedIDs := []int64{100, 200, 300}
				currentRepos := []*github.Repository{
					{ID: github.Ptr(int64(100))},
					{ID: github.Ptr(int64(200))},
				}

				result := EqualRepositoryIDs(expectedIDs, currentRepos)

				Expect(result).To(BeFalse())
			})

			It("should return false when current has more items", func() {
				expectedIDs := []int64{100, 200}
				currentRepos := []*github.Repository{
					{ID: github.Ptr(int64(100))},
					{ID: github.Ptr(int64(200))},
					{ID: github.Ptr(int64(300))},
				}

				result := EqualRepositoryIDs(expectedIDs, currentRepos)

				Expect(result).To(BeFalse())
			})

			It("should return false when expected is empty but current is not", func() {
				expectedIDs := []int64{}
				currentRepos := []*github.Repository{
					{ID: github.Ptr(int64(100))},
				}

				result := EqualRepositoryIDs(expectedIDs, currentRepos)

				Expect(result).To(BeFalse())
			})

			It("should return false when expected is not empty but current is", func() {
				expectedIDs := []int64{100}
				currentRepos := []*github.Repository{}

				result := EqualRepositoryIDs(expectedIDs, currentRepos)

				Expect(result).To(BeFalse())
			})
		})

		Context("when lengths are equal", func() {
			It("should return true when all IDs match", func() {
				expectedIDs := []int64{100, 200, 300}
				currentRepos := []*github.Repository{
					{ID: github.Ptr(int64(100))},
					{ID: github.Ptr(int64(200))},
					{ID: github.Ptr(int64(300))},
				}

				result := EqualRepositoryIDs(expectedIDs, currentRepos)

				Expect(result).To(BeTrue())
			})

			It("should return true when all IDs match regardless of order", func() {
				expectedIDs := []int64{300, 100, 200}
				currentRepos := []*github.Repository{
					{ID: github.Ptr(int64(100))},
					{ID: github.Ptr(int64(200))},
					{ID: github.Ptr(int64(300))},
				}

				result := EqualRepositoryIDs(expectedIDs, currentRepos)

				Expect(result).To(BeTrue())
			})

			It("should return false when some IDs differ", func() {
				expectedIDs := []int64{100, 200, 300}
				currentRepos := []*github.Repository{
					{ID: github.Ptr(int64(100))},
					{ID: github.Ptr(int64(200))},
					{ID: github.Ptr(int64(400))},
				}

				result := EqualRepositoryIDs(expectedIDs, currentRepos)

				Expect(result).To(BeFalse())
			})

			It("should return false when all IDs are different", func() {
				expectedIDs := []int64{100, 200, 300}
				currentRepos := []*github.Repository{
					{ID: github.Ptr(int64(400))},
					{ID: github.Ptr(int64(500))},
					{ID: github.Ptr(int64(600))},
				}

				result := EqualRepositoryIDs(expectedIDs, currentRepos)

				Expect(result).To(BeFalse())
			})
		})

		Context("with single element", func() {
			It("should return true when single ID matches", func() {
				expectedIDs := []int64{100}
				currentRepos := []*github.Repository{
					{ID: github.Ptr(int64(100))},
				}

				result := EqualRepositoryIDs(expectedIDs, currentRepos)

				Expect(result).To(BeTrue())
			})

			It("should return false when single ID differs", func() {
				expectedIDs := []int64{100}
				currentRepos := []*github.Repository{
					{ID: github.Ptr(int64(200))},
				}

				result := EqualRepositoryIDs(expectedIDs, currentRepos)

				Expect(result).To(BeFalse())
			})
		})

		Context("with duplicate IDs", func() {
			It("should handle duplicates in expected list", func() {
				expectedIDs := []int64{100, 100, 200}
				currentRepos := []*github.Repository{
					{ID: github.Ptr(int64(100))},
					{ID: github.Ptr(int64(200))},
				}

				result := EqualRepositoryIDs(expectedIDs, currentRepos)

				// Length differs, should return false
				Expect(result).To(BeFalse())
			})
		})

		Context("with large ID values", func() {
			It("should handle large repository IDs", func() {
				expectedIDs := []int64{9223372036854775807, 1000000000000, 999999999999}
				currentRepos := []*github.Repository{
					{ID: github.Ptr(int64(9223372036854775807))},
					{ID: github.Ptr(int64(1000000000000))},
					{ID: github.Ptr(int64(999999999999))},
				}

				result := EqualRepositoryIDs(expectedIDs, currentRepos)

				Expect(result).To(BeTrue())
			})
		})

		Context("comprehensive scenarios", func() {
			It("should handle mixed matching and non-matching IDs", func() {
				expectedIDs := []int64{100, 200, 300, 400}
				currentRepos := []*github.Repository{
					{ID: github.Ptr(int64(100))},
					{ID: github.Ptr(int64(200))},
					{ID: github.Ptr(int64(300))},
					{ID: github.Ptr(int64(400))},
				}

				result := EqualRepositoryIDs(expectedIDs, currentRepos)

				Expect(result).To(BeTrue())
			})

			It("should return true for matching unordered large list", func() {
				expectedIDs := []int64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
				currentRepos := []*github.Repository{
					{ID: github.Ptr(int64(100))},
					{ID: github.Ptr(int64(90))},
					{ID: github.Ptr(int64(80))},
					{ID: github.Ptr(int64(70))},
					{ID: github.Ptr(int64(60))},
					{ID: github.Ptr(int64(50))},
					{ID: github.Ptr(int64(40))},
					{ID: github.Ptr(int64(30))},
					{ID: github.Ptr(int64(20))},
					{ID: github.Ptr(int64(10))},
				}

				result := EqualRepositoryIDs(expectedIDs, currentRepos)

				Expect(result).To(BeTrue())
			})
		})
	})
})
