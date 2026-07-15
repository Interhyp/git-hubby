package orgrec

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	"github.com/google/go-github/v89/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ReconcileOrganization", func() {
	var (
		ctx           context.Context
		mockClient    *ghclientmock.MockGitHubClientWrapper
		k8sClient     client.Client
		rec           *GitHubOrgReconciler
		scheme        *runtime.Scheme
		org           *v1alpha1.Organization
		err           error
		currentGHOrg  *github.Organization
		editedOrg     *github.Organization
		editOrgCalled bool
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		org = &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-org",
				Namespace: "default",
			},
			Spec: v1alpha1.OrganizationSpec{
				Name:                    "test-org",
				Description:             "Test Organization",
				GitHubAppInstallationId: new(int64(12345)),
			},
		}

		editedOrg = nil
		editOrgCalled = false

		// Default: current GitHub org matches desired state
		currentGHOrg = &github.Organization{
			Name:        new("test-org"),
			Description: new("Test Organization"),
			Login:       new("test-org"),
		}
	})

	JustBeforeEach(func() {
		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(org).
			WithStatusSubresource(org).
			Build()

		rec = &GitHubOrgReconciler{
			GitHub: reconciler.GitHub[string]{
				Client:   mockClient,
				Resource: "test-org",
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Organization]{
				Client:   k8sClient,
				Resource: org,
			},
		}

		mockClient.GetOrganizationFunc = func(ctx context.Context, orgName string) (*github.Organization, error) {
			return currentGHOrg, nil
		}

		mockClient.EditOrganizationFunc = func(ctx context.Context, orgName string, organization *github.Organization) (*github.Organization, error) {
			editOrgCalled = true
			editedOrg = organization
			return organization, nil
		}

		err = rec.reconcileOrganization(ctx)
	})

	Context("when organization is up to date", func() {
		It("should skip update and return no error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editOrgCalled).To(BeFalse())
			Expect(editedOrg).To(BeNil())
		})
	})

	Context("when organization name differs", func() {
		BeforeEach(func() {
			currentGHOrg = &github.Organization{
				Name:        new("old-name"),
				Description: new("Test Organization"),
				Login:       new("test-org"),
			}
		})

		It("should update the organization", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editOrgCalled).To(BeTrue())
			Expect(editedOrg).NotTo(BeNil())
			Expect(editedOrg.GetName()).To(Equal("test-org"))
		})
	})

	Context("when organization description differs", func() {
		BeforeEach(func() {
			currentGHOrg = &github.Organization{
				Name:        new("test-org"),
				Description: new("Old Description"),
				Login:       new("test-org"),
			}
		})

		It("should update the organization", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editOrgCalled).To(BeTrue())
			Expect(editedOrg).NotTo(BeNil())
			Expect(editedOrg.GetDescription()).To(Equal("Test Organization"))
		})
	})

	Context("when both name and description differ", func() {
		BeforeEach(func() {
			currentGHOrg = &github.Organization{
				Name:        new("old-name"),
				Description: new("Old Description"),
				Login:       new("test-org"),
			}
		})

		It("should update both fields", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editOrgCalled).To(BeTrue())
			Expect(editedOrg).NotTo(BeNil())
			Expect(editedOrg.GetName()).To(Equal("test-org"))
			Expect(editedOrg.GetDescription()).To(Equal("Test Organization"))
		})
	})

	Context("when description is updated to empty string", func() {
		BeforeEach(func() {
			org.Spec.Description = ""
			currentGHOrg = &github.Organization{
				Name:        new("test-org"),
				Description: new("Old Description"),
				Login:       new("test-org"),
			}
		})

		It("should update description to empty", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editOrgCalled).To(BeTrue())
			Expect(editedOrg).NotTo(BeNil())
			Expect(editedOrg.GetDescription()).To(Equal(""))
		})
	})

	Context("when current description is empty and desired is not", func() {
		BeforeEach(func() {
			currentGHOrg = &github.Organization{
				Name:        new("test-org"),
				Description: new(""),
				Login:       new("test-org"),
			}
			org.Spec.Description = "New Description"
		})

		It("should update description", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editOrgCalled).To(BeTrue())
			Expect(editedOrg).NotTo(BeNil())
			Expect(editedOrg.GetDescription()).To(Equal("New Description"))
		})
	})

	Context("when GetOrganization fails", func() {
		var getOrgErr error

		BeforeEach(func() {
			getOrgErr = errors.New("GitHub API error")
		})

		JustBeforeEach(func() {
			// Override after parent JustBeforeEach
			mockClient.GetOrganizationFunc = func(ctx context.Context, orgName string) (*github.Organization, error) {
				return nil, getOrgErr
			}
			err = rec.reconcileOrganization(ctx)
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GitHub API error"))
			Expect(editOrgCalled).To(BeFalse())
		})
	})

	Context("when GetOrganization returns nil", func() {
		JustBeforeEach(func() {
			// Override after parent JustBeforeEach
			mockClient.GetOrganizationFunc = func(ctx context.Context, orgName string) (*github.Organization, error) {
				return nil, nil
			}
			err = rec.reconcileOrganization(ctx)
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("received nil organization from GitHub"))
			Expect(editOrgCalled).To(BeFalse())
		})
	})

	Context("when GetOrganization returns 404", func() {
		JustBeforeEach(func() {
			// Override after parent JustBeforeEach
			mockClient.GetOrganizationFunc = func(ctx context.Context, orgName string) (*github.Organization, error) {
				return nil, &github.ErrorResponse{
					Message: "Not Found",
					Response: &http.Response{
						StatusCode: http.StatusNotFound,
					},
				}
			}
			err = rec.reconcileOrganization(ctx)
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(editOrgCalled).To(BeFalse())
		})
	})

	Context("when EditOrganization fails", func() {
		var editErr error

		BeforeEach(func() {
			currentGHOrg = &github.Organization{
				Name:        new("old-name"),
				Description: new("Test Organization"),
				Login:       new("test-org"),
			}
			editErr = errors.New("GitHub API error during update")
		})

		JustBeforeEach(func() {
			// Override after parent JustBeforeEach
			mockClient.EditOrganizationFunc = func(ctx context.Context, orgName string, organization *github.Organization) (*github.Organization, error) {
				editOrgCalled = true
				return nil, editErr
			}
			err = rec.reconcileOrganization(ctx)
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GitHub API error during update"))
			Expect(editOrgCalled).To(BeTrue())
		})
	})

	Context("when EditOrganization returns 403 Forbidden", func() {
		BeforeEach(func() {
			currentGHOrg = &github.Organization{
				Name:        new("old-name"),
				Description: new("Test Organization"),
				Login:       new("test-org"),
			}
		})

		JustBeforeEach(func() {
			// Override after parent JustBeforeEach
			mockClient.EditOrganizationFunc = func(ctx context.Context, orgName string, organization *github.Organization) (*github.Organization, error) {
				editOrgCalled = true
				return nil, &github.ErrorResponse{
					Message: "Forbidden",
					Response: &http.Response{
						StatusCode: http.StatusForbidden,
					},
				}
			}
			err = rec.reconcileOrganization(ctx)
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(editOrgCalled).To(BeTrue())
		})
	})

	Context("when EditOrganization returns 422 Unprocessable Entity", func() {
		BeforeEach(func() {
			currentGHOrg = &github.Organization{
				Name:        new("old-name"),
				Description: new("Test Organization"),
				Login:       new("test-org"),
			}
		})

		JustBeforeEach(func() {
			// Override after parent JustBeforeEach
			mockClient.EditOrganizationFunc = func(ctx context.Context, orgName string, organization *github.Organization) (*github.Organization, error) {
				editOrgCalled = true
				return nil, &github.ErrorResponse{
					Message: "Validation failed",
					Response: &http.Response{
						StatusCode: http.StatusUnprocessableEntity,
					},
				}
			}
			err = rec.reconcileOrganization(ctx)
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(editOrgCalled).To(BeTrue())
		})
	})

	Context("when organization has special characters in description", func() {
		BeforeEach(func() {
			org.Spec.Description = "Test & Special <chars> \"quotes\" 'apostrophes' 日本語"
			currentGHOrg = &github.Organization{
				Name:        new("test-org"),
				Description: new("Old Description"),
				Login:       new("test-org"),
			}
		})

		It("should update with special characters preserved", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editOrgCalled).To(BeTrue())
			Expect(editedOrg).NotTo(BeNil())
			Expect(editedOrg.GetDescription()).To(Equal("Test & Special <chars> \"quotes\" 'apostrophes' 日本語"))
		})
	})

	Context("when organization has very long description", func() {
		BeforeEach(func() {
			var longDesc strings.Builder
			for range 100 {
				longDesc.WriteString("This is a very long description. ")
			}
			org.Spec.Description = longDesc.String()
			currentGHOrg = &github.Organization{
				Name:        new("test-org"),
				Description: new("Short"),
				Login:       new("test-org"),
			}
		})

		It("should update with long description", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editOrgCalled).To(BeTrue())
			Expect(editedOrg).NotTo(BeNil())
			Expect(len(editedOrg.GetDescription())).To(BeNumerically(">", 1000))
		})
	})

	Context("when organization description contains newlines", func() {
		BeforeEach(func() {
			org.Spec.Description = "Line 1\nLine 2\nLine 3"
			currentGHOrg = &github.Organization{
				Name:        new("test-org"),
				Description: new("Old Description"),
				Login:       new("test-org"),
			}
		})

		It("should update with newlines preserved", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editOrgCalled).To(BeTrue())
			Expect(editedOrg).NotTo(BeNil())
			Expect(editedOrg.GetDescription()).To(Equal("Line 1\nLine 2\nLine 3"))
		})
	})

	Context("when multiple reconciliations occur in sequence", func() {
		var secondErr error
		var secondEditCalled bool

		JustBeforeEach(func() {
			// First reconciliation already happened in parent JustBeforeEach
			// Reset for second reconciliation
			secondEditCalled = false
			mockClient.EditOrganizationFunc = func(ctx context.Context, orgName string, organization *github.Organization) (*github.Organization, error) {
				secondEditCalled = true
				return organization, nil
			}

			// Update the mock to return the already-updated org
			mockClient.GetOrganizationFunc = func(ctx context.Context, orgName string) (*github.Organization, error) {
				return &github.Organization{
					Name:        new("test-org"),
					Description: new("Test Organization"),
					Login:       new("test-org"),
				}, nil
			}

			// Run second reconciliation
			secondErr = rec.reconcileOrganization(ctx)
		})

		BeforeEach(func() {
			// Start with different state
			currentGHOrg = &github.Organization{
				Name:        new("old-name"),
				Description: new("Test Organization"),
				Login:       new("test-org"),
			}
		})

		It("should update on first call and skip on second", func() {
			// First reconciliation
			Expect(err).NotTo(HaveOccurred())
			Expect(editOrgCalled).To(BeTrue())

			// Second reconciliation (no changes needed)
			Expect(secondErr).NotTo(HaveOccurred())
			Expect(secondEditCalled).To(BeFalse())
		})
	})

	Context("when Location needs to be updated", func() {
		BeforeEach(func() {
			org.Spec.Location = "Munich, Germany"
			currentGHOrg = &github.Organization{
				Name:        new("test-org"),
				Description: new("Test Organization"),
				Login:       new("test-org"),
				Location:    nil,
			}
		})

		It("should trigger update to set Location", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editOrgCalled).To(BeTrue())
			Expect(editedOrg.GetLocation()).To(Equal("Munich, Germany"))
		})
	})

	Context("when Website needs to be updated", func() {
		BeforeEach(func() {
			org.Spec.Website = "https://example.com"
			currentGHOrg = &github.Organization{
				Name:        new("test-org"),
				Description: new("Test Organization"),
				Login:       new("test-org"),
				Blog:        nil,
			}
		})

		It("should trigger update to set Website (Blog)", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editOrgCalled).To(BeTrue())
			Expect(editedOrg.GetBlog()).To(Equal("https://example.com"))
		})
	})

	Context("when display name needs to be updated (using login and name)", func() {
		BeforeEach(func() {
			org.Spec.Login = "test-org"
			org.Spec.Name = "My Organization Display Name"
			currentGHOrg = &github.Organization{
				Name:        new("test-org"),
				Description: new("Test Organization"),
				Login:       new("test-org"),
			}
		})

		It("should trigger update to set display name", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editOrgCalled).To(BeTrue())
			Expect(editedOrg.GetName()).To(Equal("My Organization Display Name"))
		})
	})

	Context("when all new profile fields match", func() {
		BeforeEach(func() {
			org.Spec.Login = "test-org"
			org.Spec.Name = "My Organization"
			org.Spec.Location = "Munich, Germany"
			org.Spec.Website = "https://example.com"
			currentGHOrg = &github.Organization{
				Name:        new("My Organization"),
				Description: new("Test Organization"),
				Login:       new("test-org"),
				Location:    new("Munich, Germany"),
				Blog:        new("https://example.com"),
			}
		})

		It("should not trigger update", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editOrgCalled).To(BeFalse())
		})
	})

	Context("when only whitespace changes in description", func() {
		BeforeEach(func() {
			org.Spec.Description = "Test  Organization"
			currentGHOrg = &github.Organization{
				Name:        new("test-org"),
				Description: new("Test Organization"),
				Login:       new("test-org"),
			}
		})

		It("should detect the difference and update", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editOrgCalled).To(BeTrue())
			Expect(editedOrg.GetDescription()).To(Equal("Test  Organization"))
		})
	})

	Context("when description has leading/trailing whitespace", func() {
		BeforeEach(func() {
			org.Spec.Description = "  Test Organization  "
			currentGHOrg = &github.Organization{
				Name:        new("test-org"),
				Description: new("Test Organization"),
				Login:       new("test-org"),
			}
		})

		It("should preserve whitespace in update", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editOrgCalled).To(BeTrue())
			Expect(editedOrg.GetDescription()).To(Equal("  Test Organization  "))
		})
	})

	Context("when EditOrganization succeeds but returns nil", func() {
		BeforeEach(func() {
			currentGHOrg = &github.Organization{
				Name:        new("old-name"),
				Description: new("Test Organization"),
				Login:       new("test-org"),
			}

			mockClient.EditOrganizationFunc = func(ctx context.Context, orgName string, organization *github.Organization) (*github.Organization, error) {
				editOrgCalled = true
				return nil, nil // Returns nil but no error
			}
		})

		It("should not return error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editOrgCalled).To(BeTrue())
		})
	})

	Context("when organization resource name differs from spec name", func() {
		BeforeEach(func() {
			org.Spec.Name = "actual-org-name"
			currentGHOrg = &github.Organization{
				Name:        new("old-name"),
				Description: new("Test Organization"),
				Login:       new("actual-org-name"),
			}
		})

		It("should use spec name for update", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editOrgCalled).To(BeTrue())
			Expect(editedOrg.GetName()).To(Equal("actual-org-name"))
		})
	})

	Context("when GitHub organization has additional fields not in spec", func() {
		BeforeEach(func() {
			currentGHOrg = &github.Organization{
				Name:        new("test-org"),
				Description: new("Test Organization"),
				Login:       new("test-org"),
				// Additional fields that aren't in our spec (Company and Email are truly unmanaged)
				Company: new("Test Company"),
				Email:   new("test@example.com"),
			}
		})

		It("should not trigger update for unmanaged fields", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(editOrgCalled).To(BeFalse())
		})
	})
})
