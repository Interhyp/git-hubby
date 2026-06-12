package orgrec

import (
	"context"
	"errors"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/mapper"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/Interhyp/git-hubby/test/mock/ghclientmock"
	"github.com/google/go-github/v86/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ReconcileCustomProperties", func() {
	var (
		ctx               context.Context
		mockClient        *ghclientmock.MockGitHubClientWrapper
		k8sClient         client.Client
		rec               *GitHubOrgReconciler
		scheme            *runtime.Scheme
		org               *v1alpha1.Organization
		customProperties  []v1alpha1.OrgCustomProperty
		setCustomProps    bool
		err               error
		appliedProperties []*github.CustomProperty
		getCurrentProps   func(ctx context.Context, org string) ([]*github.CustomProperty, error)
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		customProperties = []v1alpha1.OrgCustomProperty{}
		setCustomProps = true
		appliedProperties = nil

		// Default mock for get - returns empty list
		getCurrentProps = func(_ context.Context, _ string) ([]*github.CustomProperty, error) {
			return []*github.CustomProperty{}, nil
		}
	})

	JustBeforeEach(func() {
		org = &v1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-org",
				Namespace: "default",
			},
			Spec: v1alpha1.OrganizationSpec{
				Name:                    "test-org",
				GitHubAppInstallationId: 12345,
			},
		}

		if setCustomProps {
			org.Spec.CustomProperties = customProperties
		}

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

		mockClient.GetAllOrganizationCustomPropertiesFunc = getCurrentProps

		mockClient.CreateOrUpdateOrganizationCustomPropertiesFunc = func(_ context.Context, _ string, properties []*github.CustomProperty) ([]*github.CustomProperty, error) {
			appliedProperties = properties
			return properties, nil
		}

		err = rec.reconcileCustomProperties(ctx)
	})

	Context("when no custom properties are set", func() {
		BeforeEach(func() {
			setCustomProps = false
		})

		It("should reconcile successfully with no changes", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(appliedProperties).To(BeNil())
		})
	})

	Context("when custom properties list is empty", func() {
		BeforeEach(func() {
			customProperties = []v1alpha1.OrgCustomProperty{}
		})

		It("should reconcile successfully with no changes", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(appliedProperties).To(BeNil())
		})
	})

	Context("when creating a new custom property", func() {
		BeforeEach(func() {
			customProperties = []v1alpha1.OrgCustomProperty{
				{
					PropertyName:     "environment",
					ValueType:        "single_select",
					Required:         new(true),
					DefaultValue:     &v1alpha1.OrgCustomPropertyDefaultValue{Value: new("dev")},
					Description:      new("Environment type"),
					AllowedValues:    []string{"dev", "staging", "prod"},
					ValuesEditableBy: "org_actors",
				},
			}
		})

		It("should create the custom property successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(appliedProperties).To(HaveLen(1))
			Expect(appliedProperties[0].GetPropertyName()).To(Equal("environment"))
			Expect(appliedProperties[0].ValueType).To(Equal(github.PropertyValueType("single_select")))
			Expect(appliedProperties[0].GetRequired()).To(BeTrue())
			Expect(appliedProperties[0].GetDescription()).To(Equal("Environment type"))
			Expect(appliedProperties[0].AllowedValues).To(ConsistOf("dev", "staging", "prod"))
			Expect(appliedProperties[0].GetValuesEditableBy()).To(Equal("org_actors"))
		})
	})

	Context("when creating multiple custom properties", func() {
		BeforeEach(func() {
			customProperties = []v1alpha1.OrgCustomProperty{
				{
					PropertyName:     "team",
					ValueType:        "string",
					Required:         new(false),
					Description:      new("Team name"),
					ValuesEditableBy: "org_and_repo_actors",
				},
				{
					PropertyName:     "environment",
					ValueType:        "single_select",
					Required:         new(true),
					DefaultValue:     &v1alpha1.OrgCustomPropertyDefaultValue{Value: new("dev")},
					AllowedValues:    []string{"dev", "staging", "prod"},
					ValuesEditableBy: "org_actors",
				},
			}
		})

		It("should create all custom properties successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(appliedProperties).To(HaveLen(2))

			// Check first property
			Expect(appliedProperties[0].GetPropertyName()).To(Equal("team"))
			Expect(appliedProperties[0].ValueType).To(Equal(github.PropertyValueType("string")))
			Expect(appliedProperties[0].GetRequired()).To(BeFalse())

			// Check second property
			Expect(appliedProperties[1].GetPropertyName()).To(Equal("environment"))
			Expect(appliedProperties[1].ValueType).To(Equal(github.PropertyValueType("single_select")))
			Expect(appliedProperties[1].GetRequired()).To(BeTrue())
		})
	})

	Context("when updating an existing custom property", func() {
		BeforeEach(func() {
			// Current state has a property with different values
			getCurrentProps = func(_ context.Context, _ string) ([]*github.CustomProperty, error) {
				return []*github.CustomProperty{
					{
						PropertyName:     new("environment"),
						ValueType:        github.PropertyValueTypeSingleSelect,
						Required:         new(true),
						DefaultValue:     "dev",
						Description:      new("Old description"),
						AllowedValues:    []string{"dev", "prod"},
						ValuesEditableBy: new("org_actors"),
						SourceType:       github.Ptr(mapper.CustomPropertySourceTypeOrganization),
					},
				}, nil
			}

			// Desired state has updated values
			customProperties = []v1alpha1.OrgCustomProperty{
				{
					PropertyName:     "environment",
					ValueType:        "single_select",
					Required:         new(true),
					DefaultValue:     &v1alpha1.OrgCustomPropertyDefaultValue{Value: new("staging")},
					Description:      new("Updated environment description"),
					AllowedValues:    []string{"dev", "staging", "prod"},
					ValuesEditableBy: "org_actors",
				},
			}
		})

		It("should update the custom property successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(appliedProperties).To(HaveLen(1))
			Expect(appliedProperties[0].GetPropertyName()).To(Equal("environment"))
			Expect(appliedProperties[0].GetDescription()).To(Equal("Updated environment description"))
			Expect(appliedProperties[0].AllowedValues).To(ConsistOf("dev", "staging", "prod"))
		})
	})

	Context("when custom property already matches desired state", func() {
		BeforeEach(func() {
			getCurrentProps = func(_ context.Context, _ string) ([]*github.CustomProperty, error) {
				return []*github.CustomProperty{
					{
						PropertyName:     new("environment"),
						ValueType:        github.PropertyValueTypeSingleSelect,
						Required:         new(true),
						DefaultValue:     "dev",
						Description:      new("Environment type"),
						AllowedValues:    []string{"dev", "staging", "prod"},
						ValuesEditableBy: new("org_actors"),
						SourceType:       github.Ptr(mapper.CustomPropertySourceTypeOrganization),
					},
				}, nil
			}

			customProperties = []v1alpha1.OrgCustomProperty{
				{
					PropertyName:     "environment",
					ValueType:        "single_select",
					Required:         new(true),
					DefaultValue:     &v1alpha1.OrgCustomPropertyDefaultValue{Value: new("dev")},
					Description:      new("Environment type"),
					AllowedValues:    []string{"dev", "staging", "prod"},
					ValuesEditableBy: "org_actors",
				},
			}
		})

		It("should not trigger an update", func() {
			Expect(err).NotTo(HaveOccurred())
			// No update should be triggered when properties match
			Expect(appliedProperties).To(BeNil())
		})
	})

	Context("when removing a custom property", func() {
		BeforeEach(func() {
			// Current state has two properties
			getCurrentProps = func(_ context.Context, _ string) ([]*github.CustomProperty, error) {
				return []*github.CustomProperty{
					{
						PropertyName:     new("environment"),
						ValueType:        github.PropertyValueTypeSingleSelect,
						Required:         new(true),
						DefaultValue:     "dev",
						AllowedValues:    []string{"dev", "prod"},
						ValuesEditableBy: new("org_actors"),
						SourceType:       github.Ptr(mapper.CustomPropertySourceTypeOrganization),
					},
					{
						PropertyName:     new("team"),
						ValueType:        github.PropertyValueTypeString,
						Required:         new(false),
						ValuesEditableBy: new("org_actors"),
						SourceType:       github.Ptr(mapper.CustomPropertySourceTypeOrganization),
					},
				}, nil
			}

			// Desired state only has one property
			customProperties = []v1alpha1.OrgCustomProperty{
				{
					PropertyName:     "environment",
					ValueType:        "single_select",
					Required:         new(true),
					DefaultValue:     &v1alpha1.OrgCustomPropertyDefaultValue{Value: new("dev")},
					AllowedValues:    []string{"dev", "prod"},
					ValuesEditableBy: "org_actors",
				},
			}
		})

		It("should trigger an update to remove the extra property", func() {
			Expect(err).NotTo(HaveOccurred())
			// Update should be triggered because lengths differ
			Expect(appliedProperties).To(HaveLen(1))
			Expect(appliedProperties[0].GetPropertyName()).To(Equal("environment"))
		})
	})

	Context("when handling multi_select value type", func() {
		BeforeEach(func() {
			customProperties = []v1alpha1.OrgCustomProperty{
				{
					PropertyName:     "tags",
					ValueType:        "multi_select",
					Required:         new(true),
					DefaultValue:     &v1alpha1.OrgCustomPropertyDefaultValue{Values: []string{"backend", "api"}},
					Description:      new("Repository tags"),
					AllowedValues:    []string{"frontend", "backend", "api", "database"},
					ValuesEditableBy: "org_actors",
				},
			}
		})

		It("should create multi_select property with array default value", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(appliedProperties).To(HaveLen(1))
			Expect(appliedProperties[0].GetPropertyName()).To(Equal("tags"))
			Expect(appliedProperties[0].ValueType).To(Equal(github.PropertyValueType("multi_select")))

			// DefaultValue for multi_select should be a slice
			defaultValue, ok := appliedProperties[0].DefaultValue.([]string)
			Expect(ok).To(BeTrue())
			Expect(defaultValue).To(ConsistOf("backend", "api"))
			Expect(appliedProperties[0].AllowedValues).To(ConsistOf("frontend", "backend", "api", "database"))
		})
	})

	Context("when handling true_false value type", func() {
		BeforeEach(func() {
			customProperties = []v1alpha1.OrgCustomProperty{
				{
					PropertyName:     "is_production",
					ValueType:        "true_false",
					Required:         new(true),
					DefaultValue:     &v1alpha1.OrgCustomPropertyDefaultValue{Value: new("false")},
					Description:      new("Is production repository"),
					ValuesEditableBy: "org_actors",
				},
			}
		})

		It("should create true_false property with boolean-like string", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(appliedProperties).To(HaveLen(1))
			Expect(appliedProperties[0].GetPropertyName()).To(Equal("is_production"))
			Expect(appliedProperties[0].ValueType).To(Equal(github.PropertyValueType("true_false")))

			// For true_false type, the default value should be a string "true" or "false"
			defaultValue, ok := appliedProperties[0].DefaultValue.(string)
			Expect(ok).To(BeTrue())
			Expect(defaultValue).To(Equal("false"))
		})
	})

	Context("when handling property without default value", func() {
		BeforeEach(func() {
			customProperties = []v1alpha1.OrgCustomProperty{
				{
					PropertyName:     "optional_tag",
					ValueType:        "string",
					Required:         new(false),
					Description:      new("Optional tag"),
					ValuesEditableBy: "org_and_repo_actors",
				},
			}
		})

		It("should create property without default value", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(appliedProperties).To(HaveLen(1))
			Expect(appliedProperties[0].GetPropertyName()).To(Equal("optional_tag"))
			Expect(appliedProperties[0].DefaultValue).To(BeNil())
			Expect(appliedProperties[0].GetRequired()).To(BeFalse())
		})
	})

	Context("when GetAllCustomPropertiesForOrganization fails", func() {
		BeforeEach(func() {
			customProperties = []v1alpha1.OrgCustomProperty{
				{
					PropertyName:     "test",
					ValueType:        "string",
					ValuesEditableBy: "org_actors",
				},
			}

			getCurrentProps = func(_ context.Context, _ string) ([]*github.CustomProperty, error) {
				return nil, errors.New("GitHub API error")
			}
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GitHub API error"))
			Expect(appliedProperties).To(BeNil())
		})
	})

	Context("when CreateOrUpdateOrganizationCustomProperties fails", func() {
		var createUpdateErr error

		BeforeEach(func() {
			customProperties = []v1alpha1.OrgCustomProperty{
				{
					PropertyName:     "test",
					ValueType:        "string",
					ValuesEditableBy: "org_actors",
				},
			}

			createUpdateErr = errors.New("GitHub API error during update")
		})

		JustBeforeEach(func() {
			// Need to override the default mock after it's set in the parent JustBeforeEach
			mockClient.CreateOrUpdateOrganizationCustomPropertiesFunc = func(ctx context.Context, _ string, _ []*github.CustomProperty) ([]*github.CustomProperty, error) {
				return nil, createUpdateErr
			}
			// Re-run the reconciliation with the new mock
			err = rec.reconcileCustomProperties(ctx)
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GitHub API error during update"))
		})
	})

	Context("when comparing properties with mapper error", func() {
		BeforeEach(func() {
			// Set up a current property that cannot be properly compared
			// This is a bit artificial but tests error handling
			getCurrentProps = func(_ context.Context, _ string) ([]*github.CustomProperty, error) {
				return []*github.CustomProperty{
					{
						PropertyName:     new("broken"),
						ValueType:        github.PropertyValueTypeMultiSelect,
						DefaultValue:     "not-a-slice", // Invalid type for multi_select
						ValuesEditableBy: new("org_actors"),
						SourceType:       github.Ptr(mapper.CustomPropertySourceTypeOrganization),
					},
				}, nil
			}

			customProperties = []v1alpha1.OrgCustomProperty{
				{
					PropertyName:     "broken",
					ValueType:        "multi_select",
					DefaultValue:     &v1alpha1.OrgCustomPropertyDefaultValue{Values: []string{"test"}},
					AllowedValues:    []string{"test"},
					ValuesEditableBy: "org_actors",
				},
			}
		})

		It("should handle mapper conversion error gracefully", func() {
			Expect(err).To(HaveOccurred())
			// Should get an error from the mapper when trying to convert
		})
	})

	Context("when current properties include repository-sourced properties", func() {
		BeforeEach(func() {
			// Current state has both org and repo sourced properties
			getCurrentProps = func(_ context.Context, _ string) ([]*github.CustomProperty, error) {
				return []*github.CustomProperty{
					{
						PropertyName:     new("org_property"),
						ValueType:        github.PropertyValueTypeString,
						Required:         new(false), // Must match default
						ValuesEditableBy: new("org_actors"),
						SourceType:       github.Ptr(mapper.CustomPropertySourceTypeOrganization),
					},
					{
						PropertyName:     new("repo_property"),
						ValueType:        github.PropertyValueTypeString,
						ValuesEditableBy: new("org_actors"),
						SourceType:       new("repository"), // This should be filtered out
					},
				}, nil
			}

			customProperties = []v1alpha1.OrgCustomProperty{
				{
					PropertyName:     "org_property",
					ValueType:        "string",
					Required:         new(false), // Must be explicit
					ValuesEditableBy: "org_actors",
				},
			}
		})

		It("should only process organization-sourced properties", func() {
			Expect(err).NotTo(HaveOccurred())
			// Should not trigger update because org property matches and repo property is ignored
			Expect(appliedProperties).To(BeNil())
		})
	})

	Context("when property values are in different order but equivalent", func() {
		BeforeEach(func() {
			getCurrentProps = func(_ context.Context, _ string) ([]*github.CustomProperty, error) {
				return []*github.CustomProperty{
					{
						PropertyName:     new("environment"),
						ValueType:        github.PropertyValueTypeSingleSelect,
						Required:         new(true),
						DefaultValue:     "dev",
						AllowedValues:    []string{"prod", "dev", "staging"}, // Different order
						ValuesEditableBy: new("org_actors"),
						SourceType:       github.Ptr(mapper.CustomPropertySourceTypeOrganization),
					},
				}, nil
			}

			customProperties = []v1alpha1.OrgCustomProperty{
				{
					PropertyName:     "environment",
					ValueType:        "single_select",
					Required:         new(true),
					DefaultValue:     &v1alpha1.OrgCustomPropertyDefaultValue{Value: new("dev")},
					AllowedValues:    []string{"dev", "staging", "prod"}, // Different order
					ValuesEditableBy: "org_actors",
				},
			}
		})

		It("should recognize them as equivalent", func() {
			Expect(err).NotTo(HaveOccurred())
			// Comparison should handle order differences
			Expect(appliedProperties).To(BeNil())
		})
	})

	Context("when ValuesEditableBy changes", func() {
		BeforeEach(func() {
			getCurrentProps = func(_ context.Context, _ string) ([]*github.CustomProperty, error) {
				return []*github.CustomProperty{
					{
						PropertyName:     new("team"),
						ValueType:        github.PropertyValueTypeString,
						ValuesEditableBy: new("org_actors"),
						SourceType:       github.Ptr(mapper.CustomPropertySourceTypeOrganization),
					},
				}, nil
			}

			customProperties = []v1alpha1.OrgCustomProperty{
				{
					PropertyName:     "team",
					ValueType:        "string",
					ValuesEditableBy: "org_and_repo_actors", // Changed
				},
			}
		})

		It("should trigger update", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(appliedProperties).To(HaveLen(1))
			Expect(appliedProperties[0].GetValuesEditableBy()).To(Equal("org_and_repo_actors"))
		})
	})

	Context("when Required flag changes", func() {
		BeforeEach(func() {
			getCurrentProps = func(_ context.Context, _ string) ([]*github.CustomProperty, error) {
				return []*github.CustomProperty{
					{
						PropertyName:     new("team"),
						ValueType:        github.PropertyValueTypeString,
						Required:         new(false),
						ValuesEditableBy: new("org_actors"),
						SourceType:       github.Ptr(mapper.CustomPropertySourceTypeOrganization),
					},
				}, nil
			}

			customProperties = []v1alpha1.OrgCustomProperty{
				{
					PropertyName:     "team",
					ValueType:        "string",
					Required:         new(true), // Changed to required
					DefaultValue:     &v1alpha1.OrgCustomPropertyDefaultValue{Value: new("default-team")},
					ValuesEditableBy: "org_actors",
				},
			}
		})

		It("should trigger update with new required value", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(appliedProperties).To(HaveLen(1))
			Expect(appliedProperties[0].GetRequired()).To(BeTrue())
		})
	})

	Context("when Required field is not provided", func() {
		BeforeEach(func() {
			getCurrentProps = func(ctx context.Context, _ string) ([]*github.CustomProperty, error) {
				return []*github.CustomProperty{}, nil
			}

			customProperties = []v1alpha1.OrgCustomProperty{
				{
					PropertyName:     "new_property",
					ValueType:        "string",
					Required:         nil, // Not provided
					ValuesEditableBy: "org_actors",
				},
			}
		})

		It("should default Required to false", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(appliedProperties).NotTo(BeNil())
			Expect(appliedProperties).To(HaveLen(1))
			Expect(appliedProperties[0].GetPropertyName()).To(Equal("new_property"))
			Expect(appliedProperties[0].Required).NotTo(BeNil())
			Expect(*appliedProperties[0].Required).To(BeFalse())
		})
	})

	Context("when Required field defaults prevent unnecessary updates", func() {
		BeforeEach(func() {
			getCurrentProps = func(_ context.Context, _ string) ([]*github.CustomProperty, error) {
				return []*github.CustomProperty{
					{
						PropertyName:     new("existing_property"),
						ValueType:        github.PropertyValueTypeString,
						Required:         new(false), // GitHub has default
						ValuesEditableBy: new("org_actors"),
						SourceType:       github.Ptr(mapper.CustomPropertySourceTypeOrganization),
					},
				}, nil
			}

			customProperties = []v1alpha1.OrgCustomProperty{
				{
					PropertyName:     "existing_property",
					ValueType:        "string",
					Required:         nil, // Not specified, should default to false
					ValuesEditableBy: "org_actors",
				},
			}
		})

		It("should not trigger update when default matches GitHub", func() {
			Expect(err).NotTo(HaveOccurred())
			// The default is applied before comparison, so no update needed
			Expect(appliedProperties).To(BeNil())
		})
	})
})

var _ = Describe("getGitHubOrgCustomPropertiesByPropertyName", func() {
	var (
		ctx        context.Context
		mockClient *ghclientmock.MockGitHubClientWrapper
		k8sClient  client.Client
		rec        *GitHubOrgReconciler
		scheme     *runtime.Scheme
		org        *v1alpha1.Organization
		result     map[string]*github.CustomProperty
		err        error
		allProps   []*github.CustomProperty
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
				GitHubAppInstallationId: 12345,
			},
		}

		allProps = []*github.CustomProperty{}
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

		mockClient.GetAllOrganizationCustomPropertiesFunc = func(_ context.Context, _ string) ([]*github.CustomProperty, error) {
			return allProps, nil
		}

		result, err = rec.getGitHubOrgCustomPropertiesByPropertyName(ctx)
	})

	Context("when no custom properties exist", func() {
		BeforeEach(func() {
			allProps = []*github.CustomProperty{}
		})

		It("should return empty map", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeEmpty())
		})
	})

	Context("when only organization properties exist", func() {
		BeforeEach(func() {
			allProps = []*github.CustomProperty{
				{
					PropertyName: new("prop1"),
					ValueType:    github.PropertyValueTypeString,
					SourceType:   github.Ptr(mapper.CustomPropertySourceTypeOrganization),
				},
				{
					PropertyName: new("prop2"),
					ValueType:    github.PropertyValueTypeString,
					SourceType:   github.Ptr(mapper.CustomPropertySourceTypeOrganization),
				},
			}
		})

		It("should return map with both properties", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(2))
			Expect(result).To(HaveKey("prop1"))
			Expect(result).To(HaveKey("prop2"))
		})
	})

	Context("when both organization and repository properties exist", func() {
		BeforeEach(func() {
			allProps = []*github.CustomProperty{
				{
					PropertyName: new("org_prop"),
					ValueType:    github.PropertyValueTypeString,
					SourceType:   github.Ptr(mapper.CustomPropertySourceTypeOrganization),
				},
				{
					PropertyName: new("repo_prop"),
					ValueType:    github.PropertyValueTypeString,
					SourceType:   new("repository"),
				},
			}
		})

		It("should only return organization properties", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result).To(HaveKey("org_prop"))
			Expect(result).NotTo(HaveKey("repo_prop"))
		})
	})

	Context("when property has empty name", func() {
		BeforeEach(func() {
			allProps = []*github.CustomProperty{
				{
					PropertyName: new(""),
					ValueType:    github.PropertyValueTypeString,
					SourceType:   github.Ptr(mapper.CustomPropertySourceTypeOrganization),
				},
				{
					PropertyName: new("valid_prop"),
					ValueType:    github.PropertyValueTypeString,
					SourceType:   github.Ptr(mapper.CustomPropertySourceTypeOrganization),
				},
			}
		})

		It("should skip property with empty name", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result).To(HaveKey("valid_prop"))
		})
	})

	Context("when GetAllCustomPropertiesForOrganization fails", func() {
		var apiErr error

		BeforeEach(func() {
			apiErr = errors.New("API error")
		})

		JustBeforeEach(func() {
			// Need to override the default mock after it's set in the parent JustBeforeEach
			mockClient.GetAllOrganizationCustomPropertiesFunc = func(_ context.Context, _ string) ([]*github.CustomProperty, error) {
				return nil, apiErr
			}
			// Re-run the function with the new mock
			result, err = rec.getGitHubOrgCustomPropertiesByPropertyName(ctx)
		})

		It("should return error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API error"))
			Expect(result).To(BeNil())
		})
	})
})

var _ = Describe("retainOnlyOrgProperties", func() {
	Context("when input is empty", func() {
		It("should return empty slice", func() {
			result := retainOnlyOrgProperties([]*github.CustomProperty{})
			Expect(result).To(BeEmpty())
		})
	})

	Context("when all properties are organization-sourced", func() {
		It("should return all properties", func() {
			input := []*github.CustomProperty{
				{
					PropertyName: new("prop1"),
					SourceType:   github.Ptr(mapper.CustomPropertySourceTypeOrganization),
				},
				{
					PropertyName: new("prop2"),
					SourceType:   github.Ptr(mapper.CustomPropertySourceTypeOrganization),
				},
			}
			result := retainOnlyOrgProperties(input)
			Expect(result).To(HaveLen(2))
		})
	})

	Context("when all properties are repository-sourced", func() {
		It("should return empty slice", func() {
			input := []*github.CustomProperty{
				{
					PropertyName: new("prop1"),
					SourceType:   new("repository"),
				},
				{
					PropertyName: new("prop2"),
					SourceType:   new("repository"),
				},
			}
			result := retainOnlyOrgProperties(input)
			Expect(result).To(BeEmpty())
		})
	})

	Context("when properties are mixed", func() {
		It("should only return organization-sourced properties", func() {
			input := []*github.CustomProperty{
				{
					PropertyName: new("org1"),
					SourceType:   github.Ptr(mapper.CustomPropertySourceTypeOrganization),
				},
				{
					PropertyName: new("repo1"),
					SourceType:   new("repository"),
				},
				{
					PropertyName: new("org2"),
					SourceType:   github.Ptr(mapper.CustomPropertySourceTypeOrganization),
				},
			}
			result := retainOnlyOrgProperties(input)
			Expect(result).To(HaveLen(2))
			Expect(result[0].GetPropertyName()).To(Equal("org1"))
			Expect(result[1].GetPropertyName()).To(Equal("org2"))
		})
	})

	Context("when source type is nil", func() {
		It("should not include properties with nil source type", func() {
			input := []*github.CustomProperty{
				{
					PropertyName: new("org1"),
					SourceType:   github.Ptr(mapper.CustomPropertySourceTypeOrganization),
				},
				{
					PropertyName: new("unknown"),
					SourceType:   nil,
				},
			}
			result := retainOnlyOrgProperties(input)
			Expect(result).To(HaveLen(1))
			Expect(result[0].GetPropertyName()).To(Equal("org1"))
		})
	})

	Context("when source type has unexpected value", func() {
		It("should not include properties with unexpected source type", func() {
			input := []*github.CustomProperty{
				{
					PropertyName: new("org1"),
					SourceType:   github.Ptr(mapper.CustomPropertySourceTypeOrganization),
				},
				{
					PropertyName: new("unknown"),
					SourceType:   new("unknown_source"),
				},
			}
			result := retainOnlyOrgProperties(input)
			Expect(result).To(HaveLen(1))
			Expect(result[0].GetPropertyName()).To(Equal("org1"))
		})
	})
})
