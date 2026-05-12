package mapper

import (
	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/utils"
	"github.com/google/go-github/v86/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GitHub Repo Mapper", func() {

	Describe("RepoToGithubRepo", func() {
		Context("when converting a repository with all fields set", func() {
			It("should successfully convert to GitHub repository", func() {
				repo := &v1alpha1.Repository{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-repo",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:               "my-repo",
						Archived:           github.Ptr(false),
						Visibility:         "internal",
						HasIssues:          github.Ptr(true),
						HasProjects:        github.Ptr(false),
						HasWiki:            github.Ptr(false),
						HasDownloads:       github.Ptr(false),
						IsTemplate:         github.Ptr(false),
						MergeCommitMessage: "PR_TITLE",
						MergeCommitTitle:   "MERGE_MESSAGE",
					},
				}

				githubRepo := RepoToGithubRepo(repo)

				Expect(githubRepo).NotTo(BeNil())
				Expect(githubRepo.Name).To(Equal(github.Ptr("my-repo")))
				Expect(githubRepo.Archived).To(Equal(github.Ptr(false)))
				Expect(githubRepo.Visibility).To(Equal(github.Ptr("internal")))
			})
		})

		Context("when converting an archived repository", func() {
			It("should set Archived to true", func() {
				repo := &v1alpha1.Repository{
					ObjectMeta: metav1.ObjectMeta{
						Name: "archived-repo",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:     "archived-repo",
						Archived: github.Ptr(true),
					},
				}

				githubRepo := RepoToGithubRepo(repo)

				Expect(githubRepo).NotTo(BeNil())
				Expect(githubRepo.Archived).To(Equal(github.Ptr(true)))
			})
		})

		Context("when converting a repository", func() {
			It("should always set visibility to internal", func() {
				repo := &v1alpha1.Repository{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-repo",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:               "my-repo",
						Archived:           github.Ptr(false),
						Visibility:         "internal",
						HasIssues:          github.Ptr(true),
						HasProjects:        github.Ptr(false),
						HasWiki:            github.Ptr(false),
						HasDownloads:       github.Ptr(false),
						IsTemplate:         github.Ptr(false),
						MergeCommitMessage: "PR_TITLE",
						MergeCommitTitle:   "MERGE_MESSAGE",
					},
				}

				githubRepo := RepoToGithubRepo(repo)

				Expect(githubRepo).NotTo(BeNil())
				Expect(githubRepo.Visibility).To(Equal(github.Ptr("internal")))
			})
		})

		Context("when converting a repository with special characters in name", func() {
			It("should preserve the name", func() {
				repo := &v1alpha1.Repository{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-repo",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:     "my-special-repo_123",
						Archived: github.Ptr(false),
					},
				}

				githubRepo := RepoToGithubRepo(repo)

				Expect(githubRepo).NotTo(BeNil())
				Expect(githubRepo.Name).To(Equal(github.Ptr("my-special-repo_123")))
			})
		})
	})

	Describe("RepoDiffers", func() {
		var repo *v1alpha1.Repository

		BeforeEach(func() {
			repo = &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-repo",
				},
				Spec: v1alpha1.RepositorySpec{
					Name:                "my-repo",
					Archived:            github.Ptr(false),
					Visibility:          "internal",
					HasIssues:           github.Ptr(true),
					HasProjects:         github.Ptr(false),
					HasWiki:             github.Ptr(false),
					HasDownloads:        github.Ptr(false),
					IsTemplate:          github.Ptr(false),
					DeleteBranchOnMerge: github.Ptr(true),
					MergeCommitMessage:  "PR_TITLE",
					MergeCommitTitle:    "MERGE_MESSAGE",
				},
			}
		})

		Context("when repositories match exactly", func() {
			It("should return false", func() {
				githubRepo := github.Repository{
					Name:                github.Ptr("my-repo"),
					Archived:            github.Ptr(false),
					Visibility:          github.Ptr("internal"),
					HasIssues:           github.Ptr(true),
					HasProjects:         github.Ptr(false),
					HasWiki:             github.Ptr(false),
					HasDownloads:        github.Ptr(false),
					IsTemplate:          github.Ptr(false),
					AutoInit:            github.Ptr(true),
					AllowSquashMerge:    github.Ptr(false),
					AllowRebaseMerge:    getMergeStrategy(repo, "rebase"),
					AllowMergeCommit:    getMergeStrategy(repo, "merge"),
					DeleteBranchOnMerge: repo.Spec.DeleteBranchOnMerge,
					MergeCommitTitle:    github.Ptr("MERGE_MESSAGE"),
					MergeCommitMessage:  github.Ptr("PR_TITLE"),
					Homepage:            utils.WithDefaultAsPtr(github.Ptr(repo.Spec.About.Website), ""),
					Description:         utils.WithDefaultAsPtr(github.Ptr(repo.Spec.About.Description), ""),
					DefaultBranch:       utils.WithDefaultAsPtr(github.Ptr(repo.Spec.DefaultBranch), ""),
				}

				differs := RepoDiffers(repo, githubRepo)

				Expect(differs).To(BeFalse())
			})
		})

		Context("when name differs", func() {
			It("should return true", func() {
				githubRepo := github.Repository{
					Name:       github.Ptr("different-repo"),
					Archived:   github.Ptr(false),
					Visibility: github.Ptr("internal"),
				}

				differs := RepoDiffers(repo, githubRepo)

				Expect(differs).To(BeTrue())
			})
		})

		Context("when GitHub repository has nil name", func() {
			It("should return true", func() {
				githubRepo := github.Repository{
					Name:       nil,
					Archived:   github.Ptr(false),
					Visibility: github.Ptr("internal"),
				}

				differs := RepoDiffers(repo, githubRepo)

				Expect(differs).To(BeTrue())
			})
		})

		Context("when archived status differs", func() {
			It("should return true when K8s is archived but GitHub is not", func() {
				repo.Spec.Archived = github.Ptr(true)
				githubRepo := github.Repository{
					Name:       github.Ptr("my-repo"),
					Archived:   github.Ptr(false),
					Visibility: github.Ptr("internal"),
				}

				differs := RepoDiffers(repo, githubRepo)

				Expect(differs).To(BeTrue())
			})

			It("should return true when K8s is not archived but GitHub is", func() {
				repo.Spec.Archived = github.Ptr(false)
				githubRepo := github.Repository{
					Name:       github.Ptr("my-repo"),
					Archived:   github.Ptr(true),
					Visibility: github.Ptr("internal"),
				}

				differs := RepoDiffers(repo, githubRepo)

				Expect(differs).To(BeTrue())
			})
		})

		Context("when GitHub repository has nil Archived field", func() {
			It("should return true if K8s Archived is true", func() {
				repo.Spec.Archived = github.Ptr(true)
				githubRepo := github.Repository{
					Name:       github.Ptr("my-repo"),
					Archived:   nil,
					Visibility: github.Ptr("internal"),
				}

				differs := RepoDiffers(repo, githubRepo)

				Expect(differs).To(BeTrue())
			})

			It("should return false if K8s Archived is false", func() {
				repo.Spec.Archived = github.Ptr(false)
				githubRepo := github.Repository{
					Name:                github.Ptr("my-repo"),
					Archived:            nil,
					Visibility:          github.Ptr("internal"),
					HasIssues:           github.Ptr(true),
					HasProjects:         github.Ptr(false),
					HasWiki:             github.Ptr(false),
					HasDownloads:        github.Ptr(false),
					IsTemplate:          github.Ptr(false),
					AutoInit:            github.Ptr(true),
					AllowSquashMerge:    github.Ptr(false),
					AllowRebaseMerge:    getMergeStrategy(repo, "rebase"),
					AllowMergeCommit:    getMergeStrategy(repo, "merge"),
					DeleteBranchOnMerge: repo.Spec.DeleteBranchOnMerge,
					MergeCommitTitle:    github.Ptr("MERGE_MESSAGE"),
					MergeCommitMessage:  github.Ptr("PR_TITLE"),
					Homepage:            utils.WithDefaultAsPtr(github.Ptr(repo.Spec.About.Website), ""),
					Description:         utils.WithDefaultAsPtr(github.Ptr(repo.Spec.About.Description), ""),
					DefaultBranch:       utils.WithDefaultAsPtr(github.Ptr(repo.Spec.DefaultBranch), ""),
				}

				differs := RepoDiffers(repo, githubRepo)

				Expect(differs).To(BeFalse())
			})
		})

		Context("when visibility differs", func() {
			It("should return true for public visibility", func() {
				githubRepo := github.Repository{
					Name:       github.Ptr("my-repo"),
					Archived:   github.Ptr(false),
					Visibility: github.Ptr("public"),
				}

				differs := RepoDiffers(repo, githubRepo)

				Expect(differs).To(BeTrue())
			})

			It("should return true for private visibility", func() {
				githubRepo := github.Repository{
					Name:                github.Ptr("my-repo"),
					Archived:            github.Ptr(false),
					Visibility:          github.Ptr("private"),
					HasIssues:           github.Ptr(true),
					HasProjects:         github.Ptr(false),
					HasWiki:             github.Ptr(false),
					HasDownloads:        github.Ptr(false),
					IsTemplate:          github.Ptr(false),
					AutoInit:            github.Ptr(true),
					AllowSquashMerge:    github.Ptr(false),
					AllowRebaseMerge:    getMergeStrategy(repo, "rebase"),
					AllowMergeCommit:    getMergeStrategy(repo, "merge"),
					DeleteBranchOnMerge: repo.Spec.DeleteBranchOnMerge,
					MergeCommitTitle:    github.Ptr("MERGE_MESSAGE"),
					MergeCommitMessage:  github.Ptr("PR_TITLE"),
					Homepage:            utils.WithDefaultAsPtr(github.Ptr(repo.Spec.About.Website), ""),
					Description:         utils.WithDefaultAsPtr(github.Ptr(repo.Spec.About.Description), ""),
				}

				differs := RepoDiffers(repo, githubRepo)

				Expect(differs).To(BeTrue())
			})

			It("should return true when visibility is nil", func() {
				githubRepo := github.Repository{
					Name:       github.Ptr("my-repo"),
					Archived:   github.Ptr(false),
					Visibility: nil,
				}

				differs := RepoDiffers(repo, githubRepo)

				Expect(differs).To(BeTrue())
			})

			It("should return false for internal visibility", func() {
				githubRepo := github.Repository{
					Name:                github.Ptr("my-repo"),
					Archived:            github.Ptr(false),
					Visibility:          github.Ptr("internal"),
					HasIssues:           github.Ptr(true),
					HasProjects:         github.Ptr(false),
					HasWiki:             github.Ptr(false),
					HasDownloads:        github.Ptr(false),
					IsTemplate:          github.Ptr(false),
					AutoInit:            github.Ptr(true),
					AllowSquashMerge:    github.Ptr(false),
					AllowRebaseMerge:    getMergeStrategy(repo, "rebase"),
					AllowMergeCommit:    getMergeStrategy(repo, "merge"),
					DeleteBranchOnMerge: repo.Spec.DeleteBranchOnMerge,
					MergeCommitTitle:    github.Ptr("MERGE_MESSAGE"),
					MergeCommitMessage:  github.Ptr("PR_TITLE"),
					Homepage:            utils.WithDefaultAsPtr(github.Ptr(repo.Spec.About.Website), ""),
					Description:         utils.WithDefaultAsPtr(github.Ptr(repo.Spec.About.Description), ""),
					DefaultBranch:       utils.WithDefaultAsPtr(github.Ptr(repo.Spec.DefaultBranch), ""),
				}

				differs := RepoDiffers(repo, githubRepo)

				Expect(differs).To(BeFalse())
			})
		})

		Context("when multiple fields differ", func() {
			It("should return true", func() {
				githubRepo := github.Repository{
					Name:       github.Ptr("different-repo"),
					Archived:   github.Ptr(true),
					Visibility: github.Ptr("public"),
				}

				differs := RepoDiffers(repo, githubRepo)

				Expect(differs).To(BeTrue())
			})
		})

		Context("when checking archived repositories", func() {
			It("should not differ if both are archived with internal visibility", func() {
				repo.Spec.Archived = github.Ptr(true)
				githubRepo := github.Repository{
					Name:                github.Ptr("my-repo"),
					Archived:            github.Ptr(true),
					Visibility:          github.Ptr("internal"),
					HasIssues:           github.Ptr(true),
					HasProjects:         github.Ptr(false),
					HasWiki:             github.Ptr(false),
					HasDownloads:        github.Ptr(false),
					IsTemplate:          github.Ptr(false),
					AutoInit:            github.Ptr(true),
					AllowSquashMerge:    github.Ptr(false),
					AllowRebaseMerge:    getMergeStrategy(repo, "rebase"),
					AllowMergeCommit:    getMergeStrategy(repo, "merge"),
					DeleteBranchOnMerge: repo.Spec.DeleteBranchOnMerge,
					MergeCommitTitle:    github.Ptr("MERGE_MESSAGE"),
					MergeCommitMessage:  github.Ptr("PR_TITLE"),
					Homepage:            utils.WithDefaultAsPtr(github.Ptr(repo.Spec.About.Website), ""),
					Description:         utils.WithDefaultAsPtr(github.Ptr(repo.Spec.About.Description), ""),
					DefaultBranch:       utils.WithDefaultAsPtr(github.Ptr(repo.Spec.DefaultBranch), ""),
				}

				differs := RepoDiffers(repo, githubRepo)

				Expect(differs).To(BeFalse())
			})

			It("should differ if archived but visibility is not internal", func() {
				repo.Spec.Archived = github.Ptr(true)
				githubRepo := github.Repository{
					Name:       github.Ptr("my-repo"),
					Archived:   github.Ptr(true),
					Visibility: github.Ptr("public"),
				}

				differs := RepoDiffers(repo, githubRepo)

				Expect(differs).To(BeTrue())
			})
		})

		Context("when checking edge cases", func() {
			It("should handle empty name string", func() {
				githubRepo := github.Repository{
					Name:       github.Ptr(""),
					Archived:   github.Ptr(false),
					Visibility: github.Ptr("internal"),
				}

				differs := RepoDiffers(repo, githubRepo)

				Expect(differs).To(BeTrue())
			})

			It("should handle name with whitespace", func() {
				githubRepo := github.Repository{
					Name:       github.Ptr("my-repo "),
					Archived:   github.Ptr(false),
					Visibility: github.Ptr("internal"),
				}

				differs := RepoDiffers(repo, githubRepo)

				Expect(differs).To(BeTrue())
			})
		})

		Context("when K8s repository has nil bool fields (using defaults)", func() {
			BeforeEach(func() {
				// Set all bool fields to nil to test default behavior
				repo.Spec.HasIssues = nil
				repo.Spec.HasProjects = nil
				repo.Spec.HasWiki = nil
				repo.Spec.HasDownloads = nil
				repo.Spec.IsTemplate = nil
				repo.Spec.DeleteBranchOnMerge = nil
				repo.Spec.Archived = nil
			})

			It("should not differ when GitHub matches defaults (HasIssues=true, others=false)", func() {
				githubRepo := github.Repository{
					Name:                github.Ptr("my-repo"),
					Archived:            github.Ptr(false), // default
					Visibility:          github.Ptr("internal"),
					HasIssues:           github.Ptr(true),  // default
					HasProjects:         github.Ptr(false), // default
					HasWiki:             github.Ptr(false), // default
					HasDownloads:        github.Ptr(false), // default
					IsTemplate:          github.Ptr(false), // default
					AutoInit:            github.Ptr(true),
					AllowSquashMerge:    github.Ptr(false),
					AllowRebaseMerge:    getMergeStrategy(repo, "rebase"),
					AllowMergeCommit:    getMergeStrategy(repo, "merge"),
					DeleteBranchOnMerge: github.Ptr(true), // default
					MergeCommitTitle:    github.Ptr("MERGE_MESSAGE"),
					MergeCommitMessage:  github.Ptr("PR_TITLE"),
					Homepage:            utils.WithDefaultAsPtr(github.Ptr(repo.Spec.About.Website), ""),
					Description:         utils.WithDefaultAsPtr(github.Ptr(repo.Spec.About.Description), ""),
					DefaultBranch:       utils.WithDefaultAsPtr(github.Ptr(repo.Spec.DefaultBranch), ""),
				}

				differs := RepoDiffers(repo, githubRepo)

				Expect(differs).To(BeFalse())
			})

			It("should differ when GitHub HasIssues differs from default", func() {
				githubRepo := github.Repository{
					Name:       github.Ptr("my-repo"),
					Archived:   github.Ptr(false),
					Visibility: github.Ptr("internal"),
					HasIssues:  github.Ptr(false), // differs from default (true)
				}

				differs := RepoDiffers(repo, githubRepo)

				Expect(differs).To(BeTrue())
			})

			It("should differ when GitHub DeleteBranchOnMerge differs from default", func() {
				githubRepo := github.Repository{
					Name:                github.Ptr("my-repo"),
					Archived:            github.Ptr(false),
					Visibility:          github.Ptr("internal"),
					HasIssues:           github.Ptr(true),
					HasProjects:         github.Ptr(false),
					HasWiki:             github.Ptr(false),
					HasDownloads:        github.Ptr(false),
					IsTemplate:          github.Ptr(false),
					AutoInit:            github.Ptr(true),
					AllowSquashMerge:    github.Ptr(false),
					AllowRebaseMerge:    getMergeStrategy(repo, "rebase"),
					AllowMergeCommit:    getMergeStrategy(repo, "merge"),
					DeleteBranchOnMerge: github.Ptr(false), // differs from default (true)
					MergeCommitTitle:    github.Ptr("MERGE_MESSAGE"),
					MergeCommitMessage:  github.Ptr("PR_TITLE"),
					Homepage:            utils.WithDefaultAsPtr(github.Ptr(repo.Spec.About.Website), ""),
					Description:         utils.WithDefaultAsPtr(github.Ptr(repo.Spec.About.Description), ""),
					DefaultBranch:       utils.WithDefaultAsPtr(github.Ptr(repo.Spec.DefaultBranch), ""),
				}

				differs := RepoDiffers(repo, githubRepo)

				Expect(differs).To(BeTrue())
			})

			It("should differ when GitHub Archived is true but K8s defaults to false", func() {
				githubRepo := github.Repository{
					Name:       github.Ptr("my-repo"),
					Archived:   github.Ptr(true), // differs from default (false)
					Visibility: github.Ptr("internal"),
				}

				differs := RepoDiffers(repo, githubRepo)

				Expect(differs).To(BeTrue())
			})
		})
	})

	Describe("RepoToGithubRepo with nil bool fields", func() {
		Context("when repository has nil bool fields", func() {
			It("should pass nil values through to GitHub repo (no defaults applied in mapper)", func() {
				repo := &v1alpha1.Repository{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-repo",
					},
					Spec: v1alpha1.RepositorySpec{
						Name:                "my-repo",
						Visibility:          "internal",
						HasIssues:           nil,
						HasProjects:         nil,
						HasWiki:             nil,
						HasDownloads:        nil,
						IsTemplate:          nil,
						DeleteBranchOnMerge: nil,
						Archived:            nil,
						MergeCommitMessage:  "PR_TITLE",
						MergeCommitTitle:    "MERGE_MESSAGE",
					},
				}

				githubRepo := RepoToGithubRepo(repo)

				Expect(githubRepo).NotTo(BeNil())
				Expect(githubRepo.HasIssues).To(BeNil())
				Expect(githubRepo.HasProjects).To(BeNil())
				Expect(githubRepo.HasWiki).To(BeNil())
				Expect(githubRepo.HasDownloads).To(BeNil())
				Expect(githubRepo.IsTemplate).To(BeNil())
				Expect(githubRepo.DeleteBranchOnMerge).To(BeNil())
				Expect(githubRepo.Archived).To(BeNil())
			})
		})
	})
})
