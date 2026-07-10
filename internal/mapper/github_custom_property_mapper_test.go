package mapper

import (
	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/google/go-github/v86/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GitHub Custom Property Mapper", func() {

	Describe("K8sOrgCustomPropertyMatchesGitHubCustomProperty", func() {
		var (
			desired v1alpha1.OrgCustomProperty
			current *github.CustomProperty
		)

		BeforeEach(func() {
			desired = v1alpha1.OrgCustomProperty{
				PropertyName:     "test-property",
				ValueType:        "string",
				Required:         new(true),
				DefaultValue:     &v1alpha1.OrgCustomPropertyDefaultValue{Value: new("default")},
				Description:      new("test description"),
				AllowedValues:    nil,
				ValuesEditableBy: "org_actors",
			}
		})

		Context("when current property is nil", func() {
			It("should return false and no error", func() {
				matches, err := K8sOrgCustomPropertyMatchesGitHubCustomProperty(desired, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(matches).To(BeFalse())
			})
		})

		Context("when current property is not organization type", func() {
			BeforeEach(func() {
				current = &github.CustomProperty{
					PropertyName: new("test-property"),
					ValueType:    "string",
					SourceType:   new("repository"),
				}
			})

			It("should return false and no error", func() {
				matches, err := K8sOrgCustomPropertyMatchesGitHubCustomProperty(desired, current)
				Expect(err).NotTo(HaveOccurred())
				Expect(matches).To(BeFalse())
			})
		})

		Context("when current property is organization type and matches", func() {
			BeforeEach(func() {
				current = &github.CustomProperty{
					PropertyName:     new("test-property"),
					ValueType:        "string",
					SourceType:       new(CustomPropertySourceTypeOrganization),
					Required:         new(true),
					DefaultValue:     "default",
					Description:      new("test description"),
					AllowedValues:    []string{},
					ValuesEditableBy: new("org_actors"),
				}
			})

			It("should return true and no error", func() {
				matches, err := K8sOrgCustomPropertyMatchesGitHubCustomProperty(desired, current)
				Expect(err).NotTo(HaveOccurred())
				Expect(matches).To(BeTrue())
			})
		})

		Context("when current property is organization type but does not match", func() {
			BeforeEach(func() {
				current = &github.CustomProperty{
					PropertyName:     new("test-property"),
					ValueType:        "string",
					SourceType:       new(CustomPropertySourceTypeOrganization),
					Required:         new(false),
					DefaultValue:     "different",
					Description:      new("different description"),
					AllowedValues:    []string{},
					ValuesEditableBy: new("org_actors"),
				}
			})

			It("should return false and no error", func() {
				matches, err := K8sOrgCustomPropertyMatchesGitHubCustomProperty(desired, current)
				Expect(err).NotTo(HaveOccurred())
				Expect(matches).To(BeFalse())
			})
		})

	})

	Describe("IsK8sOrgCustomProperty", func() {
		Context("when property is organization type", func() {
			It("should return true", func() {
				property := &github.CustomProperty{
					SourceType: new(CustomPropertySourceTypeOrganization),
				}
				result := IsK8sOrgCustomProperty(property)
				Expect(result).To(BeTrue())
			})
		})

		Context("when property is repository type", func() {
			It("should return false", func() {
				property := &github.CustomProperty{
					SourceType: new("repository"),
				}
				result := IsK8sOrgCustomProperty(property)
				Expect(result).To(BeFalse())
			})
		})

		Context("when property has nil source type", func() {
			It("should return false", func() {
				property := &github.CustomProperty{
					SourceType: nil,
				}
				result := IsK8sOrgCustomProperty(property)
				Expect(result).To(BeFalse())
			})
		})

		Context("when property has empty source type", func() {
			It("should return false", func() {
				property := &github.CustomProperty{
					SourceType: new(""),
				}
				result := IsK8sOrgCustomProperty(property)
				Expect(result).To(BeFalse())
			})
		})
	})

	Describe("ToK8sOrgCustomProperty", func() {
		Context("when converting valid GitHub custom property", func() {
			It("should successfully convert string property", func() {
				githubProperty := &github.CustomProperty{
					PropertyName:     new("test-string"),
					ValueType:        "string",
					Required:         new(true),
					DefaultValue:     "default-value",
					Description:      new("test description"),
					AllowedValues:    []string{},
					ValuesEditableBy: new("org_actors"),
				}

				result, err := ToK8sOrgCustomProperty(githubProperty)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.PropertyName).To(Equal("test-string"))
				Expect(result.ValueType).To(Equal("string"))
				Expect(*result.Required).To(BeTrue())
				Expect(*result.DefaultValue.Value).To(Equal("default-value"))
				Expect(*result.Description).To(Equal("test description"))
				Expect(result.AllowedValues).To(BeEmpty())
				Expect(result.ValuesEditableBy).To(Equal("org_actors"))
			})

			It("should successfully convert single_select property", func() {
				githubProperty := &github.CustomProperty{
					PropertyName:     new("test-select"),
					ValueType:        "single_select",
					Required:         new(false),
					DefaultValue:     "option1",
					Description:      new("select description"),
					AllowedValues:    []string{"option1", "option2", "option3"},
					ValuesEditableBy: new("org_and_repo_actors"),
				}

				result, err := ToK8sOrgCustomProperty(githubProperty)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.PropertyName).To(Equal("test-select"))
				Expect(result.ValueType).To(Equal("single_select"))
				Expect(*result.Required).To(BeFalse())
				Expect(*result.DefaultValue.Value).To(Equal("option1"))
				Expect(result.AllowedValues).To(ConsistOf("option1", "option2", "option3"))
				Expect(result.ValuesEditableBy).To(Equal("org_and_repo_actors"))
			})

			It("should successfully convert multi_select property", func() {
				githubProperty := &github.CustomProperty{
					PropertyName:     new("test-multi"),
					ValueType:        "multi_select",
					Required:         new(true),
					DefaultValue:     []string{"option1", "option2"},
					Description:      new("multi select description"),
					AllowedValues:    []string{"option1", "option2", "option3", "option4"},
					ValuesEditableBy: new("org_actors"),
				}

				result, err := ToK8sOrgCustomProperty(githubProperty)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.PropertyName).To(Equal("test-multi"))
				Expect(result.ValueType).To(Equal("multi_select"))
				Expect(*result.Required).To(BeTrue())
				Expect(result.DefaultValue.Values).To(HaveLen(2))
				Expect(result.DefaultValue.Values).To(ContainElements("option1", "option2"))
				Expect(result.AllowedValues).To(ConsistOf("option1", "option2", "option3", "option4"))
			})

			It("should successfully convert true_false property", func() {
				githubProperty := &github.CustomProperty{
					PropertyName:     new("test-boolean"),
					ValueType:        "true_false",
					Required:         new(false),
					DefaultValue:     "true",
					Description:      nil,
					AllowedValues:    []string{},
					ValuesEditableBy: new("org_actors"),
				}

				result, err := ToK8sOrgCustomProperty(githubProperty)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.PropertyName).To(Equal("test-boolean"))
				Expect(result.ValueType).To(Equal("true_false"))
				Expect(*result.Required).To(BeFalse())
				Expect(*result.DefaultValue.Value).To(Equal("true"))
				Expect(result.Description).To(BeNil())
			})

			It("should handle property with nil values", func() {
				githubProperty := &github.CustomProperty{
					PropertyName:     new("minimal-property"),
					ValueType:        "string",
					Required:         nil,
					DefaultValue:     nil,
					Description:      nil,
					AllowedValues:    nil,
					ValuesEditableBy: nil,
				}

				result, err := ToK8sOrgCustomProperty(githubProperty)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.PropertyName).To(Equal("minimal-property"))
				Expect(result.ValueType).To(Equal("string"))
				Expect(result.Required).To(BeNil())
				Expect(result.DefaultValue).To(BeNil())
				Expect(result.Description).To(BeNil())
				Expect(result.AllowedValues).To(BeNil())
				Expect(result.ValuesEditableBy).To(BeEmpty())
			})
		})
	})

	Describe("ToGitHubCustomProperty", func() {
		Context("when converting valid K8s custom property", func() {
			It("should successfully convert string property", func() {
				k8sProperty := v1alpha1.OrgCustomProperty{
					PropertyName:     "test-string",
					ValueType:        "string",
					Required:         new(true),
					DefaultValue:     &v1alpha1.OrgCustomPropertyDefaultValue{Value: new("default-value")},
					Description:      new("test description"),
					AllowedValues:    []string{},
					ValuesEditableBy: "org_actors",
				}

				result := ToGitHubCustomProperty(k8sProperty)
				Expect(result).NotTo(BeNil())
				Expect(*result.PropertyName).To(Equal("test-string"))
				Expect(string(result.ValueType)).To(Equal("string"))
				Expect(*result.Required).To(BeTrue())
				Expect(result.DefaultValue).To(Equal("default-value"))
				Expect(*result.Description).To(Equal("test description"))
				Expect(result.AllowedValues).To(BeEmpty())
				Expect(*result.ValuesEditableBy).To(Equal("org_actors"))
				Expect(result.SourceType).To(BeNil())
			})

			It("should successfully convert single_select property", func() {
				k8sProperty := v1alpha1.OrgCustomProperty{
					PropertyName:     "test-select",
					ValueType:        "single_select",
					Required:         new(false),
					DefaultValue:     &v1alpha1.OrgCustomPropertyDefaultValue{Value: new("option1")},
					Description:      new("select description"),
					AllowedValues:    []string{"option1", "option2", "option3"},
					ValuesEditableBy: "org_and_repo_actors",
				}

				result := ToGitHubCustomProperty(k8sProperty)
				Expect(result).NotTo(BeNil())
				Expect(*result.PropertyName).To(Equal("test-select"))
				Expect(string(result.ValueType)).To(Equal("single_select"))
				Expect(*result.Required).To(BeFalse())
				Expect(result.DefaultValue).To(Equal("option1"))
				Expect(result.AllowedValues).To(ConsistOf("option1", "option2", "option3"))
				Expect(*result.ValuesEditableBy).To(Equal("org_and_repo_actors"))
			})

			It("should successfully convert multi_select property", func() {
				k8sProperty := v1alpha1.OrgCustomProperty{
					PropertyName:     "test-multi",
					ValueType:        "multi_select",
					Required:         new(true),
					DefaultValue:     &v1alpha1.OrgCustomPropertyDefaultValue{Values: []string{"option1", "option2"}},
					Description:      new("multi select description"),
					AllowedValues:    []string{"option1", "option2", "option3", "option4"},
					ValuesEditableBy: "org_actors",
				}

				result := ToGitHubCustomProperty(k8sProperty)
				Expect(result).NotTo(BeNil())
				Expect(*result.PropertyName).To(Equal("test-multi"))
				Expect(string(result.ValueType)).To(Equal("multi_select"))
				Expect(*result.Required).To(BeTrue())
				Expect(result.DefaultValue).To(HaveLen(2))
				Expect(result.DefaultValue).To(ContainElements("option1", "option2"))
				Expect(result.AllowedValues).To(ConsistOf("option1", "option2", "option3", "option4"))
			})

			It("should successfully convert true_false property", func() {
				k8sProperty := v1alpha1.OrgCustomProperty{
					PropertyName:     "test-boolean",
					ValueType:        "true_false",
					Required:         new(false),
					DefaultValue:     &v1alpha1.OrgCustomPropertyDefaultValue{Value: new("false")},
					Description:      nil,
					AllowedValues:    []string{},
					ValuesEditableBy: "org_actors",
				}

				result := ToGitHubCustomProperty(k8sProperty)
				Expect(result).NotTo(BeNil())
				Expect(*result.PropertyName).To(Equal("test-boolean"))
				Expect(string(result.ValueType)).To(Equal("true_false"))
				Expect(*result.Required).To(BeFalse())
				Expect(result.DefaultValue).To(Equal("false"))
				Expect(result.Description).To(BeNil())
			})

			It("should handle property with nil values", func() {
				k8sProperty := v1alpha1.OrgCustomProperty{
					PropertyName:     "minimal-property",
					ValueType:        "string",
					Required:         new(false),
					DefaultValue:     nil,
					Description:      nil,
					AllowedValues:    nil,
					ValuesEditableBy: "",
				}

				result := ToGitHubCustomProperty(k8sProperty)
				Expect(result).NotTo(BeNil())
				Expect(*result.PropertyName).To(Equal("minimal-property"))
				Expect(string(result.ValueType)).To(Equal("string"))
				Expect(*result.Required).To(BeFalse())
				Expect(result.DefaultValue).To(BeNil())
				Expect(result.Description).To(BeNil())
				Expect(result.AllowedValues).To(BeNil())
				Expect(result.SourceType).To(BeNil())
			})
		})
	})

	Describe("ToGitHubCustomPropertyValues", func() {
		var definitions []*github.CustomProperty

		const stringProp = "string-prop"
		const selectProp = "select-prop"
		const multiProp = "multi-prop"
		const boolProp = "bool-prop"
		BeforeEach(func() {
			definitions = []*github.CustomProperty{
				{
					PropertyName: github.Ptr(stringProp),
					ValueType:    "string",
				},
				{
					PropertyName: github.Ptr(selectProp),
					ValueType:    "single_select",
				},
				{
					PropertyName: github.Ptr(multiProp),
					ValueType:    "multi_select",
				},
				{
					PropertyName: github.Ptr(boolProp),
					ValueType:    "true_false",
				},
			}
		})

		Context("when converting valid property values", func() {
			It("should successfully convert all supported types", func() {
				raw := []v1alpha1.CustomPropertyValue{
					{PropertyName: stringProp, Value: new("test-value")},
					{PropertyName: selectProp, Value: new("option1")},
					{PropertyName: multiProp, Values: []string{"option1", "option2"}},
					{PropertyName: boolProp, Value: new("true")},
				}

				result, err := ToGitHubCustomPropertyValues(raw, definitions)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(4))

				propertyNames := make([]string, len(result))
				for i, prop := range result {
					propertyNames[i] = prop.PropertyName
				}
				Expect(propertyNames).To(ConsistOf(stringProp, selectProp, multiProp, boolProp))

				for _, prop := range result {
					switch prop.PropertyName {
					case stringProp:
						Expect(prop.Value).To(HaveValue(Equal("test-value")))
					case selectProp:
						Expect(prop.Value).To(HaveValue(Equal("option1")))
					case multiProp:
						Expect(prop.Value).To(ConsistOf("option1", "option2"))
					case boolProp:
						Expect(prop.Value).To(HaveValue(Equal("true")))
					}
				}
			})

			It("should handle empty input map", func() {
				raw := []v1alpha1.CustomPropertyValue{}
				result, err := ToGitHubCustomPropertyValues(raw, definitions)
				Expect(err).NotTo(HaveOccurred())
				// Should return all definitions with nil values (to unset them)
				Expect(result).To(HaveLen(4))
				for _, prop := range result {
					Expect(prop.Value).To(BeNil(), "Property %s should have nil value", prop.PropertyName)
				}
			})

			It("should handle nil values", func() {
				raw := []v1alpha1.CustomPropertyValue{
					{PropertyName: stringProp, Value: nil},
					{PropertyName: selectProp, Value: new("option1")},
				}

				result, err := ToGitHubCustomPropertyValues(raw, definitions)
				Expect(err).NotTo(HaveOccurred())
				// Should include all definitions: 2 with specified values, 2 with nil (to unset)
				Expect(result).To(HaveLen(4))

				for _, prop := range result {
					switch prop.PropertyName {
					case stringProp:
						Expect(prop.Value).To(BeNil())
					case selectProp:
						Expect(prop.Value).To(HaveValue(Equal("option1")))
					case multiProp, boolProp:
						Expect(prop.Value).To(BeNil(), "Property %s should have nil value", prop.PropertyName)
					}
				}
			})
		})

		Context("when property is not defined", func() {
			It("should return error for undefined property", func() {
				raw := []v1alpha1.CustomPropertyValue{
					{PropertyName: "undefined-prop", Value: new("value")},
				}

				result, err := ToGitHubCustomPropertyValues(raw, definitions)
				// No longer errors - undefined properties are ignored, all defined ones get nil values
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(4))
				// All defined properties should have nil values
				for _, prop := range result {
					Expect(prop.Value).To(BeNil(), "Property %s should have nil value", prop.PropertyName)
				}
			})

			It("should return multiple errors for multiple undefined properties", func() {
				raw := []v1alpha1.CustomPropertyValue{
					{PropertyName: "undefined-prop1", Value: new("value1")},
					{PropertyName: "undefined-prop2", Value: new("value2")},
					{PropertyName: stringProp, Value: new("valid-value")},
				}
				result, err := ToGitHubCustomPropertyValues(raw, definitions)
				// No longer errors - undefined properties are ignored
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(4))

				// Verify the valid property has its value
				for _, prop := range result {
					switch prop.PropertyName {
					case stringProp:
						Expect(prop.Value).To(HaveValue(Equal("valid-value")))
					default:
						Expect(prop.Value).To(BeNil(), "Property %s should have nil value", prop.PropertyName)
					}
				}
			})
		})

		Context("when definitions are empty", func() {
			It("should return error for any property", func() {
				raw := []v1alpha1.CustomPropertyValue{
					{PropertyName: "any-prop", Value: new("value")},
				}
				result, err := ToGitHubCustomPropertyValues(raw, []*github.CustomProperty{})
				// No longer errors - empty definitions means nothing to process
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeEmpty())
			})
		})

		Context("when definitions are nil", func() {
			It("should return error for any property", func() {
				raw := []v1alpha1.CustomPropertyValue{
					{PropertyName: "any-prop", Value: new("value")},
				}
				result, err := ToGitHubCustomPropertyValues(raw, nil)
				// No longer errors - nil definitions means nothing to process
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeEmpty())
			})
		})

		Context("when property is required but not provided in raw input", func() {
			It("should use default value for required string property", func() {
				definitions := []*github.CustomProperty{
					{
						PropertyName: new("required-string"),
						ValueType:    github.PropertyValueTypeString,
						Required:     new(true),
						DefaultValue: "default-value",
					},
				}
				raw := []v1alpha1.CustomPropertyValue{}

				result, err := ToGitHubCustomPropertyValues(raw, definitions)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].PropertyName).To(Equal("required-string"))
				Expect(result[0].Value).To(Equal("default-value"))
			})

			It("should use default value for required single_select property", func() {
				definitions := []*github.CustomProperty{
					{
						PropertyName: new("required-select"),
						ValueType:    github.PropertyValueTypeSingleSelect,
						Required:     new(true),
						DefaultValue: "option1",
					},
				}
				raw := []v1alpha1.CustomPropertyValue{}

				result, err := ToGitHubCustomPropertyValues(raw, definitions)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].PropertyName).To(Equal("required-select"))
				Expect(result[0].Value).To(Equal("option1"))
			})

			It("should use default values for required multi_select property", func() {
				definitions := []*github.CustomProperty{
					{
						PropertyName: new("required-multi"),
						ValueType:    github.PropertyValueTypeMultiSelect,
						Required:     new(true),
						DefaultValue: []any{"option1", "option2"},
					},
				}
				raw := []v1alpha1.CustomPropertyValue{}

				result, err := ToGitHubCustomPropertyValues(raw, definitions)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].PropertyName).To(Equal("required-multi"))
				Expect(result[0].Value).To(ConsistOf("option1", "option2"))
			})

			It("should use default value 'true' for required true_false property", func() {
				definitions := []*github.CustomProperty{
					{
						PropertyName: new("required-bool-true"),
						ValueType:    github.PropertyValueTypeTrueFalse,
						Required:     new(true),
						DefaultValue: "true",
					},
				}
				raw := []v1alpha1.CustomPropertyValue{}

				result, err := ToGitHubCustomPropertyValues(raw, definitions)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].PropertyName).To(Equal("required-bool-true"))
				Expect(result[0].Value).To(Equal("true"))
			})

			It("should use default value 'false' for required true_false property", func() {
				definitions := []*github.CustomProperty{
					{
						PropertyName: new("required-bool-false"),
						ValueType:    github.PropertyValueTypeTrueFalse,
						Required:     new(true),
						DefaultValue: "false",
					},
				}
				raw := []v1alpha1.CustomPropertyValue{}

				result, err := ToGitHubCustomPropertyValues(raw, definitions)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].PropertyName).To(Equal("required-bool-false"))
				Expect(result[0].Value).To(Equal("false"))
			})

			It("should return error for required true_false property with non-boolean default value", func() {
				definitions := []*github.CustomProperty{
					{
						PropertyName: new("required-bool-invalid"),
						ValueType:    github.PropertyValueTypeTrueFalse,
						Required:     new(true),
						DefaultValue: "not-a-bool",
					},
				}
				raw := []v1alpha1.CustomPropertyValue{}

				result, err := ToGitHubCustomPropertyValues(raw, definitions)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeEmpty())
			})

			It("should set nil for optional property not in raw input", func() {
				definitions := []*github.CustomProperty{
					{
						PropertyName: new("optional-string"),
						ValueType:    github.PropertyValueTypeString,
						Required:     new(false),
						DefaultValue: "default-value",
					},
				}
				raw := []v1alpha1.CustomPropertyValue{}

				result, err := ToGitHubCustomPropertyValues(raw, definitions)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].PropertyName).To(Equal("optional-string"))
				Expect(result[0].Value).To(BeNil())
			})

			It("should handle mix of required and optional properties", func() {
				definitions := []*github.CustomProperty{
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
						PropertyName: new("provided-prop"),
						ValueType:    github.PropertyValueTypeString,
						Required:     new(false),
					},
				}
				raw := []v1alpha1.CustomPropertyValue{
					{PropertyName: "provided-prop", Value: new("user-value")},
				}

				result, err := ToGitHubCustomPropertyValues(raw, definitions)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(3))

				for _, prop := range result {
					switch prop.PropertyName {
					case "required-prop":
						Expect(prop.Value).To(Equal("required-default"))
					case "optional-prop":
						Expect(prop.Value).To(BeNil())
					case "provided-prop":
						Expect(prop.Value).To(Equal("user-value"))
					}
				}
			})
		})
	})
})
