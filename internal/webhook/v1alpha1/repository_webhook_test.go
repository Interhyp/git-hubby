package v1alpha1

import (
	"context"
	"errors"

	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	gogithub "github.com/google/go-github/v89/github"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
)

// Mock implementation of client.Client for testing
type mockK8sClient struct {
	organization *githubv1alpha1.Organization
	getError     error
}

func (m *mockK8sClient) Get(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	if m.getError != nil {
		return m.getError
	}
	if org, ok := obj.(*githubv1alpha1.Organization); ok && m.organization != nil {
		*org = *m.organization
		return nil
	}
	return apierrors.NewNotFound(githubv1alpha1.GroupVersion.WithResource("organizations").GroupResource(), key.Name)
}

func (m *mockK8sClient) Apply(ctx context.Context, obj runtime.ApplyConfiguration, opts ...client.ApplyOption) error {
	return nil
}
func (m *mockK8sClient) List(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
	return nil
}
func (m *mockK8sClient) Create(_ context.Context, _ client.Object, _ ...client.CreateOption) error {
	return nil
}
func (m *mockK8sClient) Delete(_ context.Context, _ client.Object, _ ...client.DeleteOption) error {
	return nil
}
func (m *mockK8sClient) Update(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
	return nil
}
func (m *mockK8sClient) Patch(_ context.Context, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
	return nil
}
func (m *mockK8sClient) DeleteAllOf(_ context.Context, _ client.Object, _ ...client.DeleteAllOfOption) error {
	return nil
}
func (m *mockK8sClient) Status() client.StatusWriter                   { return nil }
func (m *mockK8sClient) Scheme() *runtime.Scheme                       { return nil }
func (m *mockK8sClient) SubResource(_ string) client.SubResourceClient { return nil }
func (m *mockK8sClient) GroupVersionKindFor(_ runtime.Object) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}
func (m *mockK8sClient) IsObjectNamespaced(_ runtime.Object) (bool, error) { return true, nil }
func (m *mockK8sClient) RESTMapper() meta.RESTMapper {
	return nil
}

var _ = Describe("Repository Webhook", func() {
	var (
		ctx        context.Context
		obj        *githubv1alpha1.Repository
		oldObj     *githubv1alpha1.Repository
		validator  RepositoryCustomValidator
		mockK8s    *mockK8sClient
		mockClient *ghclientmock.MockGitHubClientWrapper
		testOrg    *githubv1alpha1.Organization
	)

	BeforeEach(func() {
		ctx = context.Background()

		// Set up test organization
		testOrg = &githubv1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-org",
				Namespace: "default",
			},
			Spec: githubv1alpha1.OrganizationSpec{
				Name: "test-org",
				GitHubAppConfig: &githubv1alpha1.GitHubAppConfig{
					InstallationId:        12345,
					CredentialsSecretName: "test-credentials",
				},
			},
		}

		// Set up mock clients
		mockK8s = &mockK8sClient{
			organization: testOrg,
		}
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		// Set up test repository
		obj = &githubv1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-repo",
				Namespace: "default",
			},
			Spec: githubv1alpha1.RepositorySpec{
				Name: "test-repo",
				OrganizationRef: githubv1alpha1.OrganizationRef{
					Name: "test-org",
				},
				CustomProperties: []githubv1alpha1.CustomPropertyValue{},
			},
		}

		oldObj = &githubv1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-repo",
				Namespace: "default",
			},
			Spec: githubv1alpha1.RepositorySpec{
				Name: "test-repo",
				OrganizationRef: githubv1alpha1.OrganizationRef{
					Name: "test-org",
				},
				CustomProperties: []githubv1alpha1.CustomPropertyValue{},
			},
		}

		validator = RepositoryCustomValidator{
			K8sClient:           mockK8s,
			GitHubClientManager: ghclientmock.NewGitHubMockClientFactory(mockClient),
			LegacySecretName:    "test-credentials",
		}

		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	AfterEach(func() {
		// No teardown logic needed
	})

	Context("When validating Repository creation", func() {
		It("Should allow valid repository without custom properties", func() {
			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("Should reject creation with wrong object type", func() {
			warnings, err := validator.ValidateCreate(ctx, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected a Repository object but got nil"))
			Expect(warnings).To(BeEmpty())
		})

		It("Should return error when organization is not found", func() {
			mockK8s.getError = apierrors.NewNotFound(githubv1alpha1.GroupVersion.WithResource("organizations").GroupResource(), "test-org")

			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to fetch organization during validation"))
			Expect(warnings).To(BeEmpty())
		})

		It("Should return error when GitHub client creation fails", func() {
			mockClient.SetError(errors.New("failed to create client"))

			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to fetch custom properties"))
			Expect(warnings).To(BeEmpty())
		})

		It("Should return error when fetching custom properties fails", func() {
			mockClient.SetError(errors.New("failed to fetch properties"))

			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to fetch custom properties"))
			Expect(warnings).To(BeEmpty())
		})

		Context("Custom Properties Validation", func() {
			BeforeEach(func() {
				// Set up mock custom property definitions
				propName1 := "string-prop"
				propName2 := "select-prop"
				propName3 := "multi-prop"
				propName4 := "bool-prop"

				mockClient.GetAllOrganizationCustomPropertiesFunc = func(ctx context.Context, org string) ([]*gogithub.CustomProperty, error) {
					return []*gogithub.CustomProperty{
						{
							PropertyName: &propName1,
							ValueType:    "string",
						},
						{
							PropertyName:  &propName2,
							ValueType:     "single_select",
							AllowedValues: []string{"option1", "option2", "option3"},
						},
						{
							PropertyName:  &propName3,
							ValueType:     "multi_select",
							AllowedValues: []string{"value1", "value2", "value3"},
						},
						{
							PropertyName: &propName4,
							ValueType:    "true_false",
						},
					}, nil
				}
			})

			It("Should allow valid string custom property", func() {
				obj.Spec.CustomProperties = []githubv1alpha1.CustomPropertyValue{
					{PropertyName: "string-prop", Value: new("test-string-value")},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("Should allow valid single_select custom property", func() {
				obj.Spec.CustomProperties = []githubv1alpha1.CustomPropertyValue{
					{PropertyName: "select-prop", Value: new("option2")},
				}
				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("Should allow valid multi_select custom property", func() {
				obj.Spec.CustomProperties = []githubv1alpha1.CustomPropertyValue{
					{PropertyName: "multi-prop", Values: []string{"value1", "value3"}},
				}
				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("Should allow valid true_false custom property", func() {
				obj.Spec.CustomProperties = []githubv1alpha1.CustomPropertyValue{
					{PropertyName: "bool-prop", Value: new("true")},
				}
				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("Should reject invalid true_false value", func() {
				obj.Spec.CustomProperties = []githubv1alpha1.CustomPropertyValue{
					{PropertyName: "bool-prop", Value: new("invalid-bool")},
				}
				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsInvalid(err)).To(BeTrue())
				Expect(warnings).To(BeEmpty())
			})

			It("Should reject invalid single_select value", func() {
				obj.Spec.CustomProperties = []githubv1alpha1.CustomPropertyValue{
					{PropertyName: "select-prop", Value: new("invalid-option")},
				}
				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsInvalid(err)).To(BeTrue())
				Expect(warnings).To(BeEmpty())
			})

			It("Should reject multi_select with invalid values", func() {
				obj.Spec.CustomProperties = []githubv1alpha1.CustomPropertyValue{
					{PropertyName: "multi-prop", Values: []string{"value1", "invalid-value"}},
				}
				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsInvalid(err)).To(BeTrue())
				Expect(warnings).To(BeEmpty())
			})

			It("Should handle nil custom property definition", func() {
				mockClient.GetAllOrganizationCustomPropertiesFunc = func(ctx context.Context, org string) ([]*gogithub.CustomProperty, error) {
					return []*gogithub.CustomProperty{nil}, nil
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsInvalid(err)).To(BeTrue())
				Expect(warnings).To(BeEmpty())
			})

			It("Should allow properties not defined in organization (they are ignored)", func() {
				obj.Spec.CustomProperties = []githubv1alpha1.CustomPropertyValue{
					{PropertyName: "undefined-prop", Value: new("some-value")},
				}
				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("Should allow nil values for properties", func() {
				obj.Spec.CustomProperties = []githubv1alpha1.CustomPropertyValue{
					{PropertyName: "undefined-prop", Value: nil},
				}
				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
		})
	})

	Context("When validating Repository update", func() {
		It("Should allow valid repository update", func() {
			warnings, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("Should reject update with wrong object type for newObj", func() {
			warnings, err := validator.ValidateUpdate(ctx, oldObj, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected a Repository object for the new object but got nil"))
			Expect(warnings).To(BeEmpty())
		})

		It("Should validate updated custom properties", func() {
			// Set up a string property definition
			propName := "updated-prop"
			mockClient.GetAllOrganizationCustomPropertiesFunc = func(ctx context.Context, org string) ([]*gogithub.CustomProperty, error) {
				return []*gogithub.CustomProperty{
					{
						PropertyName: &propName,
						ValueType:    "string",
					},
				}, nil
			}

			obj.Spec.CustomProperties = []githubv1alpha1.CustomPropertyValue{
				{PropertyName: "updated-prop", Value: new("updated-value")},
			}
			warnings, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
	})

	Context("When validating Repository deletion", func() {
		It("Should allow deletion", func() {
			warnings, err := validator.ValidateDelete(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("Should reject deletion with wrong object type", func() {
			warnings, err := validator.ValidateDelete(ctx, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected a Repository object but got nil"))
			Expect(warnings).To(BeEmpty())
		})
	})
})
