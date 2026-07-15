package mapper

import (
	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/google/go-github/v89/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GitHub Org Mapper", func() {

	Describe("OrgToGithubOrg", func() {
		Context("when converting an organization with only name field (legacy mode)", func() {
			It("should use name as both login and display name", func() {
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

		Context("when converting an organization with both login and name", func() {
			It("should use name as display name", func() {
				org := &v1alpha1.Organization{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-org",
					},
					Spec: v1alpha1.OrganizationSpec{
						Login:       "my-org-login",
						Name:        "My Organization Display Name",
						Description: "Test description",
					},
				}

				githubOrg := OrgToGithubOrg(org)

				Expect(githubOrg).NotTo(BeNil())
				Expect(githubOrg.Name).To(Equal(new("My Organization Display Name")))
				Expect(githubOrg.Description).To(Equal(new("Test description")))
			})
		})

		Context("when converting an organization with only login field", func() {
			It("should use login as display name", func() {
				org := &v1alpha1.Organization{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-org",
					},
					Spec: v1alpha1.OrganizationSpec{
						Login:       "my-org-login",
						Description: "Test description",
					},
				}

				githubOrg := OrgToGithubOrg(org)

				Expect(githubOrg).NotTo(BeNil())
				Expect(githubOrg.Name).To(Equal(new("my-org-login")))
				Expect(githubOrg.Description).To(Equal(new("Test description")))
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

		Context("when converting an organization with Location set", func() {
			It("should set Location in GitHub organization", func() {
				org := &v1alpha1.Organization{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-org",
					},
					Spec: v1alpha1.OrganizationSpec{
						Name:        "my-org",
						Location:    "Munich, Germany",
						Description: "Test description",
					},
				}

				githubOrg := OrgToGithubOrg(org)

				Expect(githubOrg).NotTo(BeNil())
				Expect(githubOrg.Location).To(Equal(new("Munich, Germany")))
			})
		})

		Context("when converting an organization with Website set", func() {
			It("should set Blog in GitHub organization", func() {
				org := &v1alpha1.Organization{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-org",
					},
					Spec: v1alpha1.OrganizationSpec{
						Name:        "my-org",
						Website:     "https://example.com",
						Description: "Test description",
					},
				}

				githubOrg := OrgToGithubOrg(org)

				Expect(githubOrg).NotTo(BeNil())
				Expect(githubOrg.Blog).To(Equal(new("https://example.com")))
			})
		})

		Context("when converting an organization with all fields set", func() {
			It("should set all fields correctly", func() {
				org := &v1alpha1.Organization{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-org",
					},
					Spec: v1alpha1.OrganizationSpec{
						Login:       "my-org-login",
						Name:        "My Org Display Name",
						Location:    "Munich, Germany",
						Website:     "https://example.com",
						Description: "Test description",
					},
				}

				githubOrg := OrgToGithubOrg(org)

				Expect(githubOrg).NotTo(BeNil())
				Expect(githubOrg.Name).To(Equal(new("My Org Display Name")))
				Expect(githubOrg.Location).To(Equal(new("Munich, Germany")))
				Expect(githubOrg.Blog).To(Equal(new("https://example.com")))
				Expect(githubOrg.Description).To(Equal(new("Test description")))
			})
		})

		Context("when converting an organization with empty optional fields", func() {
			It("should not set Location and Blog when empty", func() {
				org := &v1alpha1.Organization{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-org",
					},
					Spec: v1alpha1.OrganizationSpec{
						Name:        "my-org",
						Location:    "",
						Website:     "",
						Description: "Test description",
					},
				}

				githubOrg := OrgToGithubOrg(org)

				Expect(githubOrg).NotTo(BeNil())
				Expect(githubOrg.Name).To(Equal(new("my-org")))
				Expect(githubOrg.Location).To(BeNil())
				Expect(githubOrg.Blog).To(BeNil())
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

		Context("when organizations match exactly (legacy mode)", func() {
			It("should return false", func() {
				githubOrg := github.Organization{
					Login:       new("my-org"),
					Name:        new("my-org"),
					Description: new("Test organization"),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeFalse())
			})
		})

		Context("when login differs", func() {
			It("should return true", func() {
				githubOrg := github.Organization{
					Login:       new("different-org"),
					Name:        new("my-org"),
					Description: new("Test organization"),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeTrue())
			})
		})

		Context("when display name differs", func() {
			It("should return true", func() {
				githubOrg := github.Organization{
					Login:       new("my-org"),
					Name:        new("Different Name"),
					Description: new("Test organization"),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeTrue())
			})
		})

		Context("when description differs", func() {
			It("should return true", func() {
				githubOrg := github.Organization{
					Login:       new("my-org"),
					Name:        new("my-org"),
					Description: new("Different description"),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeTrue())
			})
		})

		Context("when GitHub organization has nil login", func() {
			It("should return true", func() {
				githubOrg := github.Organization{
					Login:       nil,
					Name:        new("my-org"),
					Description: new("Test organization"),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeTrue())
			})
		})

		Context("when Location differs", func() {
			It("should return true when K8s has Location but GitHub does not", func() {
				org.Spec.Location = "Munich, Germany"
				githubOrg := github.Organization{
					Login:       new("my-org"),
					Name:        new("my-org"),
					Description: new("Test organization"),
					Location:    nil,
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeTrue())
			})

			It("should return true when Location values differ", func() {
				org.Spec.Location = "Munich, Germany"
				githubOrg := github.Organization{
					Login:       new("my-org"),
					Name:        new("my-org"),
					Description: new("Test organization"),
					Location:    new("Berlin, Germany"),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeTrue())
			})

			It("should return false when Location values match", func() {
				org.Spec.Location = "Munich, Germany"
				githubOrg := github.Organization{
					Login:       new("my-org"),
					Name:        new("my-org"),
					Description: new("Test organization"),
					Location:    new("Munich, Germany"),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeFalse())
			})
		})

		Context("when Website differs", func() {
			It("should return true when K8s has Website but GitHub does not", func() {
				org.Spec.Website = "https://example.com"
				githubOrg := github.Organization{
					Login:       new("my-org"),
					Name:        new("my-org"),
					Description: new("Test organization"),
					Blog:        nil,
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeTrue())
			})

			It("should return false when Website values match", func() {
				org.Spec.Website = "https://example.com"
				githubOrg := github.Organization{
					Login:       new("my-org"),
					Name:        new("my-org"),
					Description: new("Test organization"),
					Blog:        new("https://example.com"),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeFalse())
			})
		})

		Context("when using explicit login and name", func() {
			BeforeEach(func() {
				org.Spec.Login = "my-org-login"
				org.Spec.Name = "My Organization Display Name"
			})

			It("should compare login and display name separately", func() {
				githubOrg := github.Organization{
					Login:       new("my-org-login"),
					Name:        new("My Organization Display Name"),
					Description: new("Test organization"),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeFalse())
			})

			It("should return true when login matches but display name differs", func() {
				githubOrg := github.Organization{
					Login:       new("my-org-login"),
					Name:        new("Different Display Name"),
					Description: new("Test organization"),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeTrue())
			})

			It("should return true when display name matches but login differs", func() {
				githubOrg := github.Organization{
					Login:       new("different-login"),
					Name:        new("My Organization Display Name"),
					Description: new("Test organization"),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeTrue())
			})
		})

		Context("when all fields match", func() {
			It("should return false", func() {
				org.Spec.Login = "my-org-login"
				org.Spec.Name = "My Org Display Name"
				org.Spec.Location = "Munich, Germany"
				org.Spec.Website = "https://example.com"
				githubOrg := github.Organization{
					Login:       new("my-org-login"),
					Name:        new("My Org Display Name"),
					Description: new("Test organization"),
					Location:    new("Munich, Germany"),
					Blog:        new("https://example.com"),
				}

				differs := OrgDiffers(org, githubOrg)

				Expect(differs).To(BeFalse())
			})
		})
	})

	Describe("Organization Helper Methods", func() {
		Describe("GetLogin", func() {
			It("should return Login when explicitly set", func() {
				org := &v1alpha1.Organization{
					Spec: v1alpha1.OrganizationSpec{
						Login: "my-login",
						Name:  "My Display Name",
					},
				}

				Expect(org.GetLogin()).To(Equal("my-login"))
			})

			It("should return Name when Login is not set (legacy mode)", func() {
				org := &v1alpha1.Organization{
					Spec: v1alpha1.OrganizationSpec{
						Name: "my-org",
					},
				}

				Expect(org.GetLogin()).To(Equal("my-org"))
			})

			It("should return empty string when nil", func() {
				var org *v1alpha1.Organization
				Expect(org.GetLogin()).To(Equal(""))
			})
		})

		Describe("GetDisplayName", func() {
			It("should return Name when both Login and Name are set", func() {
				org := &v1alpha1.Organization{
					Spec: v1alpha1.OrganizationSpec{
						Login: "my-login",
						Name:  "My Display Name",
					},
				}

				Expect(org.GetDisplayName()).To(Equal("My Display Name"))
			})

			It("should return Login when only Login is set", func() {
				org := &v1alpha1.Organization{
					Spec: v1alpha1.OrganizationSpec{
						Login: "my-login",
					},
				}

				Expect(org.GetDisplayName()).To(Equal("my-login"))
			})

			It("should return Name when only Name is set (legacy mode)", func() {
				org := &v1alpha1.Organization{
					Spec: v1alpha1.OrganizationSpec{
						Name: "my-org",
					},
				}

				Expect(org.GetDisplayName()).To(Equal("my-org"))
			})

			It("should return empty string when nil", func() {
				var org *v1alpha1.Organization
				Expect(org.GetDisplayName()).To(Equal(""))
			})
		})

		Describe("IsUsingLegacyNameField", func() {
			It("should return true when only Name is set", func() {
				org := &v1alpha1.Organization{
					Spec: v1alpha1.OrganizationSpec{
						Name: "my-org",
					},
				}

				Expect(org.IsUsingLegacyNameField()).To(BeTrue())
			})

			It("should return false when Login is set", func() {
				org := &v1alpha1.Organization{
					Spec: v1alpha1.OrganizationSpec{
						Login: "my-login",
						Name:  "My Display Name",
					},
				}

				Expect(org.IsUsingLegacyNameField()).To(BeFalse())
			})

			It("should return false when only Login is set", func() {
				org := &v1alpha1.Organization{
					Spec: v1alpha1.OrganizationSpec{
						Login: "my-login",
					},
				}

				Expect(org.IsUsingLegacyNameField()).To(BeFalse())
			})

			It("should return false when nil", func() {
				var org *v1alpha1.Organization
				Expect(org.IsUsingLegacyNameField()).To(BeFalse())
			})

			It("should return false when both are empty", func() {
				org := &v1alpha1.Organization{
					Spec: v1alpha1.OrganizationSpec{},
				}

				Expect(org.IsUsingLegacyNameField()).To(BeFalse())
			})
		})
	})
})
