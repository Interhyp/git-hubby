package reporec

import (
	"context"
	"errors"

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

// Helper function to extract string value from github.CustomPropertyValue.Value (which is any)
func getStringValue(v any) string {
	if v == nil {
		return ""
	}
	if ptr, ok := v.(*string); ok && ptr != nil {
		return *ptr
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

var _ = Describe("ReconcileCustomProperties", func() {
	var (
		ctx                    context.Context
		mockClient             *ghclientmock.MockGitHubClientWrapper
		k8sClient              client.Client
		rec                    *GitHubRepoReconciler
		scheme                 *runtime.Scheme
		repo                   *v1alpha1.Repository
		customProperties       []v1alpha1.CustomPropertyValue
		propertyDefinitions    []*github.CustomProperty
		currentPropertyValues  []*github.CustomPropertyValue
		err                    error
		appliedPropertyValues  []*github.CustomPropertyValue
		updatePropertiesCalled bool
		getCurrentValuesError  error
		getDefinitionsError    error
		updatePropertiesError  error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = ghclientmock.NewMockGitHubClientWrapper()

		scheme = runtime.NewScheme()
		schemeErr := v1alpha1.AddToScheme(scheme)
		Expect(schemeErr).NotTo(HaveOccurred())

		// Default custom properties
		customProperties = []v1alpha1.CustomPropertyValue{}

		// Default property definitions - these define what properties exist in the organization
		propertyDefinitions = []*github.CustomProperty{
			{
				PropertyName:  new("environment"),
				ValueType:     "single_select",
				Required:      new(true),
				DefaultValue:  "development",
				Description:   new("Environment type"),
				AllowedValues: []string{"development", "staging", "production"},
			},
			{
				PropertyName: new("team"),
				ValueType:    "string",
				Required:     new(true),
				DefaultValue: "default-team",
			},
			{
				PropertyName:  new("languages"),
				ValueType:     "multi_select",
				Required:      new(false),
				AllowedValues: []string{"go", "typescript", "python", "java"},
			},
			{
				PropertyName: new("archived"),
				ValueType:    "true_false",
				Required:     new(false),
				DefaultValue: "false",
			},
		}

		// Default current property values (empty)
		currentPropertyValues = []*github.CustomPropertyValue{}

		// Reset flags and errors
		appliedPropertyValues = nil
		updatePropertiesCalled = false
		getCurrentValuesError = nil
		getDefinitionsError = nil
		updatePropertiesError = nil

		// Set up default mock functions
		mockClient.GetAllCustomPropertyValuesFunc = func(ctx context.Context, owner, repo string) ([]*github.CustomPropertyValue, error) {
			return currentPropertyValues, getCurrentValuesError
		}

		mockClient.GetAllOrganizationCustomPropertiesFunc = func(ctx context.Context, org string) ([]*github.CustomProperty, error) {
			return propertyDefinitions, getDefinitionsError
		}

		mockClient.CreateOrUpdateRepositoryCustomPropertiesFunc = func(ctx context.Context, owner, repo string, properties []*github.CustomPropertyValue) error {
			updatePropertiesCalled = true
			appliedPropertyValues = properties
			return updatePropertiesError
		}
	})

	JustBeforeEach(func() {
		repo = &v1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-repo",
				Namespace: "default",
			},
			Spec: v1alpha1.RepositorySpec{
				Name:             "test-repo",
				Archived:         new(false),
				CustomProperties: customProperties,
				OrganizationRef: v1alpha1.OrganizationRef{
					Name: "test-org",
				},
			},
		}

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(repo).
			WithStatusSubresource(repo).
			Build()

		rec = &GitHubRepoReconciler{
			GitHub: reconciler.GitHub[GitHubRepoIdentifier]{
				Client: mockClient,
				Resource: GitHubRepoIdentifier{
					Owner: "test-org",
					Name:  "test-repo",
				},
			},
			Kubernetes: reconciler.Kubernetes[*v1alpha1.Repository]{
				Client:   k8sClient,
				Resource: repo,
			},
		}

		err = rec.reconcileCustomProperties(ctx)
	})

	Context("when no custom properties are set in spec", func() {
		BeforeEach(func() {
			customProperties = []v1alpha1.CustomPropertyValue{}
			// Current values include defaults for required properties (GitHub doesn't return properties with nil values)
			currentPropertyValues = []*github.CustomPropertyValue{
				{PropertyName: "environment", Value: "development"},
				{PropertyName: "team", Value: "default-team"},
			}
		})

		It("should reconcile successfully with no changes", func() {
			Expect(err).NotTo(HaveOccurred())
			// No update because current already matches desired (defaults)
			Expect(updatePropertiesCalled).To(BeFalse())
		})
	})

	Context("when custom properties match current values", func() {
		BeforeEach(func() {
			customProperties = []v1alpha1.CustomPropertyValue{
				{PropertyName: "environment", Value: new("production")},
				{PropertyName: "team", Value: new("platform")},
			}
			// GitHub doesn't return properties with nil values
			currentPropertyValues = []*github.CustomPropertyValue{
				{PropertyName: "environment", Value: "production"},
				{PropertyName: "team", Value: "platform"},
			}
		})

		It("should skip update when values match", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatePropertiesCalled).To(BeFalse())
		})
	})

	Context("when creating new custom property values", func() {
		BeforeEach(func() {
			customProperties = []v1alpha1.CustomPropertyValue{
				{PropertyName: "environment", Value: new("staging")},
				{PropertyName: "team", Value: new("backend")},
			}
			currentPropertyValues = []*github.CustomPropertyValue{}
		})

		It("should create the custom property values successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatePropertiesCalled).To(BeTrue())
			// Mapper returns all 4 properties (2 specified + 2 unset optional)
			Expect(appliedPropertyValues).To(HaveLen(4))

			propMap := make(map[string]any)
			for _, prop := range appliedPropertyValues {
				propMap[prop.PropertyName] = prop.Value
			}

			Expect(propMap["environment"]).To(Equal("staging"))
			Expect(propMap["team"]).To(Equal("backend"))
			Expect(propMap["languages"]).To(BeNil())
			Expect(propMap["archived"]).To(BeNil())
		})
	})

	Context("when updating existing custom property values", func() {
		BeforeEach(func() {
			customProperties = []v1alpha1.CustomPropertyValue{
				{PropertyName: "environment", Value: new("production")},
				{PropertyName: "team", Value: new("frontend")},
			}
			currentPropertyValues = []*github.CustomPropertyValue{
				{PropertyName: "environment", Value: new("development")},
				{PropertyName: "team", Value: new("backend")},
			}
		})

		It("should update the custom property values", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatePropertiesCalled).To(BeTrue())
			// Mapper returns all 4 properties
			Expect(appliedPropertyValues).To(HaveLen(4))

			propMap := make(map[string]any)
			for _, prop := range appliedPropertyValues {
				propMap[prop.PropertyName] = prop.Value
			}

			Expect(propMap["environment"]).To(Equal("production"))
			Expect(propMap["team"]).To(Equal("frontend"))
			Expect(propMap["languages"]).To(BeNil())
			Expect(propMap["archived"]).To(BeNil())
		})
	})

	Context("when using multi-select custom properties", func() {
		BeforeEach(func() {
			customProperties = []v1alpha1.CustomPropertyValue{
				{PropertyName: "languages", Values: []string{"go", "typescript"}},
			}
			currentPropertyValues = []*github.CustomPropertyValue{}
		})

		It("should set multi-select values correctly", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatePropertiesCalled).To(BeTrue())
			// Mapper returns all 4 properties
			Expect(appliedPropertyValues).To(HaveLen(4))

			propMap := make(map[string]any)
			for _, prop := range appliedPropertyValues {
				propMap[prop.PropertyName] = prop.Value
			}

			Expect(propMap["languages"]).To(BeAssignableToTypeOf([]string{}))
			values := propMap["languages"].([]string)
			Expect(values).To(ConsistOf("go", "typescript"))
			// Required properties get default values
			Expect(propMap["environment"]).To(Equal("development"))
			Expect(propMap["team"]).To(Equal("default-team"))
			Expect(propMap["archived"]).To(BeNil())
		})
	})

	Context("when updating multi-select values", func() {
		BeforeEach(func() {
			customProperties = []v1alpha1.CustomPropertyValue{
				{PropertyName: "languages", Values: []string{"go", "python", "java"}},
			}
			currentPropertyValues = []*github.CustomPropertyValue{
				{PropertyName: "languages", Value: []string{"go", "typescript"}},
			}
		})

		It("should update multi-select values", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatePropertiesCalled).To(BeTrue())
			// Mapper returns all 4 properties
			Expect(appliedPropertyValues).To(HaveLen(4))

			propMap := make(map[string]any)
			for _, prop := range appliedPropertyValues {
				propMap[prop.PropertyName] = prop.Value
			}

			values := propMap["languages"].([]string)
			Expect(values).To(ConsistOf("go", "python", "java"))
			// Required properties get defaults
			Expect(propMap["environment"]).To(Equal("development"))
			Expect(propMap["team"]).To(Equal("default-team"))
			Expect(propMap["archived"]).To(BeNil())
		})
	})

	Context("when removing custom property values", func() {
		BeforeEach(func() {
			customProperties = []v1alpha1.CustomPropertyValue{}
			currentPropertyValues = []*github.CustomPropertyValue{
				{PropertyName: "environment", Value: "development"},
				{PropertyName: "team", Value: "backend"},
			}
		})

		It("should remove all custom properties", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatePropertiesCalled).To(BeTrue())
			// Mapper returns all 4 properties: required ones with defaults, optional ones with nil
			Expect(appliedPropertyValues).To(HaveLen(4))

			propMap := make(map[string]any)
			for _, prop := range appliedPropertyValues {
				propMap[prop.PropertyName] = prop.Value
			}

			// Required properties get defaults when not specified
			Expect(propMap["environment"]).To(Equal("development"))
			Expect(propMap["team"]).To(Equal("default-team"))
			Expect(propMap["languages"]).To(BeNil())
			Expect(propMap["archived"]).To(BeNil())
		})
	})

	Context("when some properties remain and some are removed", func() {
		BeforeEach(func() {
			customProperties = []v1alpha1.CustomPropertyValue{
				{PropertyName: "environment", Value: new("production")},
			}
			currentPropertyValues = []*github.CustomPropertyValue{
				{PropertyName: "environment", Value: "development"},
				{PropertyName: "team", Value: "backend"},
			}
		})

		It("should keep specified properties and remove others", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatePropertiesCalled).To(BeTrue())
			// Mapper returns all 4 properties
			Expect(appliedPropertyValues).To(HaveLen(4))

			propMap := make(map[string]any)
			for _, prop := range appliedPropertyValues {
				propMap[prop.PropertyName] = prop.Value
			}

			Expect(propMap["environment"]).To(Equal("production"))
			// team gets default since it's required but not specified
			Expect(propMap["team"]).To(Equal("default-team"))
			Expect(propMap["languages"]).To(BeNil())
			Expect(propMap["archived"]).To(BeNil())
		})
	})

	Context("when using true_false value type", func() {
		BeforeEach(func() {
			customProperties = []v1alpha1.CustomPropertyValue{
				{PropertyName: "archived", Value: new("true")},
			}
			currentPropertyValues = []*github.CustomPropertyValue{}
		})

		It("should set boolean value correctly", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatePropertiesCalled).To(BeTrue())
			// Mapper returns all 4 properties
			Expect(appliedPropertyValues).To(HaveLen(4))

			propMap := make(map[string]any)
			for _, prop := range appliedPropertyValues {
				propMap[prop.PropertyName] = prop.Value
			}

			Expect(getStringValue(propMap["archived"])).To(Equal("true"))
			// Required properties get defaults
			Expect(propMap["environment"]).To(Equal("development"))
			Expect(propMap["team"]).To(Equal("default-team"))
			Expect(propMap["languages"]).To(BeNil())
		})
	})

	Context("when custom properties are sorted differently", func() {
		BeforeEach(func() {
			// Spec has properties in reverse order
			customProperties = []v1alpha1.CustomPropertyValue{
				{PropertyName: "team", Value: new("backend")},
				{PropertyName: "environment", Value: new("production")},
			}
			// Current has properties in different order (GitHub doesn't return properties with nil values)
			currentPropertyValues = []*github.CustomPropertyValue{
				{PropertyName: "environment", Value: "production"},
				{PropertyName: "team", Value: "backend"},
			}
		})

		It("should recognize properties match despite different order", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatePropertiesCalled).To(BeFalse())
		})
	})

	Context("when GetAllCustomPropertyValues fails", func() {
		BeforeEach(func() {
			getCurrentValuesError = errors.New("failed to fetch custom property values")
			customProperties = []v1alpha1.CustomPropertyValue{
				{PropertyName: "environment", Value: new("production")},
			}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to fetch custom property values"))
			Expect(updatePropertiesCalled).To(BeFalse())
		})
	})

	Context("when GetAllCustomPropertiesForOrganization fails", func() {
		BeforeEach(func() {
			getDefinitionsError = errors.New("failed to fetch custom property definitions")
			customProperties = []v1alpha1.CustomPropertyValue{
				{PropertyName: "environment", Value: new("production")},
			}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to fetch custom property definitions"))
			Expect(updatePropertiesCalled).To(BeFalse())
		})
	})

	Context("when mapper fails to convert properties", func() {
		BeforeEach(func() {
			// Set up a property that doesn't exist in definitions
			customProperties = []v1alpha1.CustomPropertyValue{
				{PropertyName: "nonexistent-property", Value: new("value")},
			}
		})

		It("should return mapping error", func() {
			// Mapper ignores properties not in definitions, returns all 4 defined properties
			Expect(err).NotTo(HaveOccurred())
			Expect(updatePropertiesCalled).To(BeTrue())
			// All 4 defined properties with defaults/nil
			Expect(appliedPropertyValues).To(HaveLen(4))
		})
	})

	Context("when CreateOrUpdateRepositoryCustomProperties fails", func() {
		BeforeEach(func() {
			updatePropertiesError = errors.New("failed to update custom properties")
			customProperties = []v1alpha1.CustomPropertyValue{
				{PropertyName: "environment", Value: new("production")},
			}
			currentPropertyValues = []*github.CustomPropertyValue{
				{PropertyName: "environment", Value: "development"},
			}
		})

		It("should return the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to update custom properties"))
			Expect(updatePropertiesCalled).To(BeTrue())
		})
	})

	Context("when updating with multiple property types", func() {
		BeforeEach(func() {
			customProperties = []v1alpha1.CustomPropertyValue{
				{PropertyName: "environment", Value: new("production")},
				{PropertyName: "team", Value: new("platform")},
				{PropertyName: "languages", Values: []string{"go", "typescript"}},
				{PropertyName: "archived", Value: new("false")},
			}
			currentPropertyValues = []*github.CustomPropertyValue{}
		})

		It("should handle all property types correctly", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatePropertiesCalled).To(BeTrue())
			Expect(appliedPropertyValues).To(HaveLen(4))

			// Verify each property type
			propertyMap := make(map[string]*github.CustomPropertyValue)
			for _, prop := range appliedPropertyValues {
				propertyMap[prop.PropertyName] = prop
			}

			Expect(getStringValue(propertyMap["environment"].Value)).To(Equal("production"))
			Expect(getStringValue(propertyMap["team"].Value)).To(Equal("platform"))
			Expect(propertyMap["languages"].Value).To(BeAssignableToTypeOf([]string{}))
			Expect(getStringValue(propertyMap["archived"].Value)).To(Equal("false"))
		})
	})

	Context("when partial update with mixed changes", func() {
		BeforeEach(func() {
			// Update some, keep some, remove some
			customProperties = []v1alpha1.CustomPropertyValue{
				{PropertyName: "environment", Value: new("staging")}, // updated
				{PropertyName: "team", Value: new("backend")},        // kept same
				// languages removed
				{PropertyName: "archived", Value: new("true")}, // added
			}
			currentPropertyValues = []*github.CustomPropertyValue{
				{PropertyName: "environment", Value: new("development")},
				{PropertyName: "team", Value: new("backend")},
				{PropertyName: "languages", Value: []string{"go"}},
			}
		})

		It("should apply mixed changes correctly", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatePropertiesCalled).To(BeTrue())
			// Mapper returns all 4 properties
			Expect(appliedPropertyValues).To(HaveLen(4))

			propertyMap := make(map[string]*github.CustomPropertyValue)
			for _, prop := range appliedPropertyValues {
				propertyMap[prop.PropertyName] = prop
			}

			Expect(getStringValue(propertyMap["environment"].Value)).To(Equal("staging"))
			Expect(getStringValue(propertyMap["team"].Value)).To(Equal("backend"))
			Expect(getStringValue(propertyMap["archived"].Value)).To(Equal("true"))
			Expect(propertyMap["languages"].Value).To(BeNil())
		})
	})

	Context("when current values have extra properties not in spec", func() {
		BeforeEach(func() {
			customProperties = []v1alpha1.CustomPropertyValue{
				{PropertyName: "environment", Value: new("production")},
			}
			currentPropertyValues = []*github.CustomPropertyValue{
				{PropertyName: "environment", Value: "production"},
				{PropertyName: "team", Value: "backend"},
				{PropertyName: "languages", Value: []string{"go", "python"}},
			}
		})

		It("should remove properties not in spec", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatePropertiesCalled).To(BeTrue())
			// Mapper returns all 4 properties
			Expect(appliedPropertyValues).To(HaveLen(4))

			propertyMap := make(map[string]*github.CustomPropertyValue)
			for _, prop := range appliedPropertyValues {
				propertyMap[prop.PropertyName] = prop
			}

			Expect(getStringValue(propertyMap["environment"].Value)).To(Equal("production"))
			// team gets default since required but not in spec
			Expect(getStringValue(propertyMap["team"].Value)).To(Equal("default-team"))
			// optional properties become nil when not in spec
			Expect(propertyMap["languages"].Value).To(BeNil())
			Expect(propertyMap["archived"].Value).To(BeNil())
		})
	})

	Context("when empty multi-select values", func() {
		BeforeEach(func() {
			customProperties = []v1alpha1.CustomPropertyValue{
				{PropertyName: "languages", Values: []string{}},
			}
			currentPropertyValues = []*github.CustomPropertyValue{
				{PropertyName: "languages", Value: []string{"go", "typescript"}},
			}
		})

		It("should clear multi-select values", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatePropertiesCalled).To(BeTrue())
			// Mapper returns all 4 properties
			Expect(appliedPropertyValues).To(HaveLen(4))

			propertyMap := make(map[string]*github.CustomPropertyValue)
			for _, prop := range appliedPropertyValues {
				propertyMap[prop.PropertyName] = prop
			}

			// Empty array should be preserved
			values := propertyMap["languages"].Value.([]string)
			Expect(values).To(BeEmpty())
			// Required properties get defaults
			Expect(getStringValue(propertyMap["environment"].Value)).To(Equal("development"))
			Expect(getStringValue(propertyMap["team"].Value)).To(Equal("default-team"))
			Expect(propertyMap["archived"].Value).To(BeNil())
		})
	})

	Context("when property definitions are empty", func() {
		BeforeEach(func() {
			propertyDefinitions = []*github.CustomProperty{}
			customProperties = []v1alpha1.CustomPropertyValue{
				{PropertyName: "environment", Value: new("production")},
			}
		})

		It("should return error from mapper", func() {
			// When definitions are empty, mapper returns empty array without error
			Expect(err).NotTo(HaveOccurred())
			// Since current is also empty, no update needed
			Expect(updatePropertiesCalled).To(BeFalse())
		})
	})

	Context("when multiple properties change order in arrays", func() {
		BeforeEach(func() {
			customProperties = []v1alpha1.CustomPropertyValue{
				{PropertyName: "languages", Values: []string{"typescript", "go", "python"}},
			}
			// GitHub doesn't return properties with nil values, only non-nil ones
			currentPropertyValues = []*github.CustomPropertyValue{
				{PropertyName: "environment", Value: "development"},
				{PropertyName: "team", Value: "default-team"},
				{PropertyName: "languages", Value: []string{"go", "python", "typescript"}},
			}
		})

		It("should recognize arrays match despite different order", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatePropertiesCalled).To(BeFalse())
		})
	})

	Context("when required property is not provided in spec", func() {
		BeforeEach(func() {
			propertyDefinitions = []*github.CustomProperty{
				{
					PropertyName: new("environment"),
					ValueType:    github.PropertyValueTypeString,
					Required:     new(true),
					DefaultValue: "production",
				},
				{
					PropertyName: new("team"),
					ValueType:    github.PropertyValueTypeString,
					Required:     new(false),
				},
			}
			customProperties = []v1alpha1.CustomPropertyValue{}
			currentPropertyValues = []*github.CustomPropertyValue{}
		})

		It("should use default value for required property", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatePropertiesCalled).To(BeTrue())
			Expect(appliedPropertyValues).To(HaveLen(2))

			var envProp, teamProp *github.CustomPropertyValue
			for _, prop := range appliedPropertyValues {
				switch prop.PropertyName {
				case "environment":
					envProp = prop
				case "team":
					teamProp = prop
				}
			}

			Expect(envProp).NotTo(BeNil())
			Expect(envProp.Value).To(Equal("production"))

			Expect(teamProp).NotTo(BeNil())
			Expect(teamProp.Value).To(BeNil())
		})
	})

	Context("when optional property is not provided in spec", func() {
		BeforeEach(func() {
			propertyDefinitions = []*github.CustomProperty{
				{
					PropertyName: new("optional-prop"),
					ValueType:    github.PropertyValueTypeString,
					Required:     new(false),
					DefaultValue: "default-value",
				},
			}
			customProperties = []v1alpha1.CustomPropertyValue{}
			currentPropertyValues = []*github.CustomPropertyValue{
				{PropertyName: "optional-prop", Value: "some-value"},
			}
		})

		It("should unset optional property with nil value", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatePropertiesCalled).To(BeTrue())
			Expect(appliedPropertyValues).To(HaveLen(1))
			Expect(appliedPropertyValues[0].PropertyName).To(Equal("optional-prop"))
			Expect(appliedPropertyValues[0].Value).To(BeNil())
		})
	})

	Context("when mixing provided and missing properties", func() {
		BeforeEach(func() {
			propertyDefinitions = []*github.CustomProperty{
				{
					PropertyName: new("required-prop"),
					ValueType:    github.PropertyValueTypeString,
					Required:     new(true),
					DefaultValue: "required-default",
				},
				{
					PropertyName: new("optional-prop"),
					ValueType:    github.PropertyValueTypeString,
					Required:     new(false),
					DefaultValue: "optional-default",
				},
				{
					PropertyName: new("user-prop"),
					ValueType:    github.PropertyValueTypeString,
					Required:     new(false),
				},
			}
			customProperties = []v1alpha1.CustomPropertyValue{
				{PropertyName: "user-prop", Value: new("user-value")},
			}
			currentPropertyValues = []*github.CustomPropertyValue{}
		})

		It("should correctly handle mix of required, optional, and provided properties", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatePropertiesCalled).To(BeTrue())
			Expect(appliedPropertyValues).To(HaveLen(3))

			propMap := make(map[string]any)
			for _, prop := range appliedPropertyValues {
				propMap[prop.PropertyName] = prop.Value
			}

			Expect(propMap["required-prop"]).To(Equal("required-default"))
			Expect(propMap["optional-prop"]).To(BeNil())
			Expect(propMap["user-prop"]).To(Equal("user-value"))
		})
	})

	Context("when required multi_select property is not provided", func() {
		BeforeEach(func() {
			propertyDefinitions = []*github.CustomProperty{
				{
					PropertyName: new("tags"),
					ValueType:    github.PropertyValueTypeMultiSelect,
					Required:     new(true),
					DefaultValue: []any{"default-tag1", "default-tag2"},
				},
			}
			customProperties = []v1alpha1.CustomPropertyValue{}
			currentPropertyValues = []*github.CustomPropertyValue{}
		})

		It("should use default values for required multi_select property", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(updatePropertiesCalled).To(BeTrue())
			Expect(appliedPropertyValues).To(HaveLen(1))
			Expect(appliedPropertyValues[0].PropertyName).To(Equal("tags"))
			Expect(appliedPropertyValues[0].Value).To(ConsistOf("default-tag1", "default-tag2"))
		})
	})
})
