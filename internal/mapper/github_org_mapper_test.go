package mapper

import (
	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/google/go-github/v86/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GitHub Org Mapper", func() {

	Describe("OrgToGithubOrg", func() {
		Context("when converting an organization with all fields set", func() {
			It("should successfully convert to GitHub organization", func() {
				org := &v1alpha1.Organization{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-org",
					},
					Spec: v1alpha1.OrganizationSpec{
						Name:        "my-org",
						Description: "This is a test organization",
					},
				}

				githubOrg := OrgToGithubOrg(org)

				Expect(githubOrg).NotTo(BeNil())
				Expect(githubOrg.Name).To(Equal(new("my-org")))
				Expect(githubOrg.Description).To(Equal(new("This is a test organization")))
			})
		})

		Context("when converting an organization with empty description", func() {
			It("should set empty description", func() {
				org := &v1alpha1.Organization{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-org",
					},
					Spec: v1alpha1.OrganizationSpec{
						Name:        "my-org",
						Description: "",
					},
				}

				githubOrg := OrgToGithubOrg(org)

				Expect(githubOrg).NotTo(BeNil())
				Expect(githubOrg.Name).To(Equal(new("my-org")))
				Expect(githubOrg.Description).To(Equal(new("")))
			})
		})

		Context("when converting an organization with long description", func() {
			It("should preserve the full description", func() {
				longDesc := "This is a very long description that contains multiple sentences. " +
					"It describes the organization's purpose, mission, and values. " +
					"Organizations can have descriptions up to a certain character limit."

				org := &v1alpha1.Organization{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-org",
					},
					Spec: v1alpha1.OrganizationSpec{
						Name:        "my-org",
						Description: longDesc,
					},
				}

				githubOrg := OrgToGithubOrg(org)

				Expect(githubOrg).NotTo(BeNil())
				Expect(githubOrg.Name).To(Equal(new("my-org")))
				Expect(githubOrg.Description).To(Equal(new(longDesc)))
			})
		})

		Context("when converting an organization with special characters in description", func() {
			It("should preserve special characters", func() {
				specialDesc := "Org with special chars: @#$%^&*()_+-=[]{}|;:',.<>?/~`"

				org := &v1alpha1.Organization{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-org",
					},
					Spec: v1alpha1.OrganizationSpec{
						Name:        "my-org",
						Description: specialDesc,
					},
				}

				githubOrg := OrgToGithubOrg(org)

				Expect(githubOrg).NotTo(BeNil())
				Expect(githubOrg.Description).To(Equal(new(specialDesc)))
			})
		})
	})

	Describe("OrgDiffers", func() {
		var org *v1alpha1.Organization

		BeforeEach(func() {
			org = &v1alpha1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-org",
				},
				Spec: v1alpha1.OrganizationSpec{
					Name:        "my-org",
					Description: "Test organization",
				},
			}
		})

		Context("when organizations match exactly", func() {
			It("should return false", func() {
				githubOrg := github.Organization{
					Name:        new("my-org"),
					Description: new("Test organization"),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeFalse())
			})
		})

		Context("when name differs", func() {
			It("should return true", func() {
				githubOrg := github.Organization{
					Name:        new("different-org"),
					Description: new("Test organization"),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeTrue())
			})
		})

		Context("when description differs", func() {
			It("should return true", func() {
				githubOrg := github.Organization{
					Name:        new("my-org"),
					Description: new("Different description"),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeTrue())
			})
		})

		Context("when both name and description differ", func() {
			It("should return true", func() {
				githubOrg := github.Organization{
					Name:        new("different-org"),
					Description: new("Different description"),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeTrue())
			})
		})

		Context("when GitHub organization has nil name", func() {
			It("should return true", func() {
				githubOrg := github.Organization{
					Name:        nil,
					Description: new("Test organization"),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeTrue())
			})
		})

		Context("when GitHub organization has nil description", func() {
			It("should return true", func() {
				githubOrg := github.Organization{
					Name:        new("my-org"),
					Description: nil,
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeTrue())
			})
		})

		Context("when both organizations have empty descriptions", func() {
			It("should return false", func() {
				org.Spec.Description = ""
				githubOrg := github.Organization{
					Name:        new("my-org"),
					Description: new(""),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeFalse())
			})
		})

		Context("when K8s description is empty but GitHub has description", func() {
			It("should return true", func() {
				org.Spec.Description = ""
				githubOrg := github.Organization{
					Name:        new("my-org"),
					Description: new("Some description"),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeTrue())
			})
		})

		Context("when K8s has description but GitHub description is empty", func() {
			It("should return true", func() {
				githubOrg := github.Organization{
					Name:        new("my-org"),
					Description: new(""),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeTrue())
			})
		})

		Context("when checking whitespace differences", func() {
			It("should detect trailing whitespace differences", func() {
				githubOrg := github.Organization{
					Name:        new("my-org"),
					Description: new("Test organization "),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeTrue())
			})

			It("should detect leading whitespace differences", func() {
				githubOrg := github.Organization{
					Name:        new("my-org"),
					Description: new(" Test organization"),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeTrue())
			})
		})
	})
})
