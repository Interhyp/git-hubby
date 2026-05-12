/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
//nolint:goconst // Ignoring constant strings in tests for better readability
package v1alpha1

import (
	"context"

	"github.com/google/go-github/v86/github"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
)

var _ = Describe("Organization Webhook", func() {
	var (
		ctx       context.Context
		obj       *githubv1alpha1.Organization
		oldObj    *githubv1alpha1.Organization
		validator OrganizationCustomValidator
	)

	BeforeEach(func() {
		ctx = context.Background()
		obj = &githubv1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-org",
				Namespace: "default",
			},
			Spec: githubv1alpha1.OrganizationSpec{
				Name:                    "test-org",
				GitHubAppInstallationId: 12345,
				CustomProperties:        []githubv1alpha1.OrgCustomProperty{},
			},
		}
		oldObj = &githubv1alpha1.Organization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-org",
				Namespace: "default",
			},
			Spec: githubv1alpha1.OrganizationSpec{
				Name:                    "test-org",
				GitHubAppInstallationId: 12345,
				CustomProperties:        []githubv1alpha1.OrgCustomProperty{},
			},
		}
		validator = OrganizationCustomValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	AfterEach(func() {
		// No teardown logic needed
	})

	Context("When validating Organization creation", func() {
		It("Should allow valid organization without custom properties", func() {
			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("Should reject creation with wrong object type", func() {
			warnings, err := validator.ValidateCreate(ctx, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected an Organization object but got nil"))
			Expect(warnings).To(BeEmpty())
		})

		Context("Custom Properties Validation", func() {
			It("Should allow valid string type custom property", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("test-value")}
				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "test-prop",
						ValueType:    "string",
						Required:     &required,
						DefaultValue: &defaultValue,
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("Should allow valid single_select custom property", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("option1")}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName:  "test-select",
						ValueType:     "single_select",
						Required:      &required,
						DefaultValue:  &defaultValue,
						AllowedValues: []string{"option1", "option2", "option3"},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("Should allow valid multi_select custom property", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Values: []string{"option1", "option2"}}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName:  "test-multi",
						ValueType:     "multi_select",
						Required:      &required,
						DefaultValue:  &defaultValue,
						AllowedValues: []string{"option1", "option2", "option3"},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("Should allow valid true_false custom property", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("true")}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "test-bool",
						ValueType:    "true_false",
						Required:     &required,
						DefaultValue: &defaultValue,
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("Should reject single_select without allowed_values", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("option1")}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "test-select",
						ValueType:    "single_select",
						Required:     &required,
						DefaultValue: &defaultValue,
						// Missing AllowedValues
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsInvalid(err)).To(BeTrue())
				Expect(warnings).To(BeEmpty())
			})

			It("Should reject multi_select without allowed_values", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Values: []string{"option1"}}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "test-multi",
						ValueType:    "multi_select",
						Required:     &required,
						DefaultValue: &defaultValue,
						// Missing AllowedValues
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsInvalid(err)).To(BeTrue())
				Expect(warnings).To(BeEmpty())
			})

			It("Should reject string type with allowed_values", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("test-value")}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName:  "test-string",
						ValueType:     "string",
						Required:      &required,
						DefaultValue:  &defaultValue,
						AllowedValues: []string{"should", "not", "be", "here"},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsInvalid(err)).To(BeTrue())
				Expect(warnings).To(BeEmpty())
			})

			It("Should reject required property without default_value", func() {
				required := true
				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "test-required",
						ValueType:    "string",
						Required:     &required,
						// Missing DefaultValue
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsInvalid(err)).To(BeTrue())
				Expect(warnings).To(BeEmpty())
			})

			It("Should reject non-required property with default_value", func() {
				required := false
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("should-not-be-here")}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "test-optional",
						ValueType:    "string",
						Required:     &required,
						DefaultValue: &defaultValue,
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsInvalid(err)).To(BeTrue())
				Expect(warnings).To(BeEmpty())
			})

			It("Should reject duplicate property names", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("test-value")}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "duplicate-prop",
						ValueType:    "string",
						Required:     &required,
						DefaultValue: &defaultValue,
					},
					{
						PropertyName: "duplicate-prop",
						ValueType:    "string",
						Required:     &required,
						DefaultValue: &defaultValue,
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsInvalid(err)).To(BeTrue())
				Expect(warnings).To(BeEmpty())
			})

			It("Should reject invalid true_false default value", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("invalid-bool")}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "test-bool",
						ValueType:    "true_false",
						Required:     &required,
						DefaultValue: &defaultValue,
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsInvalid(err)).To(BeTrue())
				Expect(warnings).To(BeEmpty())
			})

			It("Should reject single_select with invalid default value", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("invalid-option")}
				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName:  "test-select",
						ValueType:     "single_select",
						Required:      &required,
						DefaultValue:  &defaultValue,
						AllowedValues: []string{"option1", "option2", "option3"},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsInvalid(err)).To(BeTrue())
				Expect(warnings).To(BeEmpty())
			})

			It("Should reject multi_select with invalid values in array", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Values: []string{"option1", "invalid-option"}}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName:  "test-multi",
						ValueType:     "multi_select",
						Required:      &required,
						DefaultValue:  &defaultValue,
						AllowedValues: []string{"option1", "option2", "option3"},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsInvalid(err)).To(BeTrue())
				Expect(warnings).To(BeEmpty())
			})

			It("Should reject multi_select with single value instead of array", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("option1")}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName:  "test-multi",
						ValueType:     "multi_select",
						Required:      &required,
						DefaultValue:  &defaultValue,
						AllowedValues: []string{"option1", "option2", "option3"},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				statusErr := err.(*errors.StatusError)
				Expect(statusErr.ErrStatus.Message).To(ContainSubstring("default value for custom property of type multi_select must be an array given as 'values'"))
				Expect(warnings).To(BeEmpty())
			})

			It("Should allow empty array for multi_select default value", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Values: []string{}}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName:  "test-multi",
						ValueType:     "multi_select",
						Required:      &required,
						DefaultValue:  &defaultValue,
						AllowedValues: []string{"option1", "option2", "option3"},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("Should reject empty allowed_values for single_select", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("option1")}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName:  "test-select",
						ValueType:     "single_select",
						Required:      &required,
						DefaultValue:  &defaultValue,
						AllowedValues: []string{},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsInvalid(err)).To(BeTrue())
				Expect(warnings).To(BeEmpty())
			})

			It("Should allow nil required field (defaults to false)", func() {
				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "test-optional",
						ValueType:    "string",
						Required:     nil, // nil means false
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("Should validate property name pattern with special characters", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("value")}

				// Valid property names with special characters
				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "valid_prop",
						ValueType:    "string",
						Required:     &required,
						DefaultValue: &defaultValue,
					},
					{
						PropertyName: "valid-prop",
						ValueType:    "string",
						Required:     &required,
						DefaultValue: &defaultValue,
					},
					{
						PropertyName: "valid$prop",
						ValueType:    "string",
						Required:     &required,
						DefaultValue: &defaultValue,
					},
					{
						PropertyName: "valid#prop",
						ValueType:    "string",
						Required:     &required,
						DefaultValue: &defaultValue,
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("Should handle multiple validation errors at once", func() {
				required := true
				invalidDefault := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("invalid")}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "duplicate",
						ValueType:    "string",
						Required:     &required,
						DefaultValue: &invalidDefault,
					},
					{
						PropertyName: "duplicate", // Duplicate name
						ValueType:    "string",
						Required:     &required,
						DefaultValue: &invalidDefault,
					},
					{
						PropertyName:  "missing-allowed",
						ValueType:     "single_select", // Missing allowed_values
						Required:      &required,
						DefaultValue:  &invalidDefault,
						AllowedValues: nil,
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				statusErr := err.(*errors.StatusError)
				// Should report multiple errors
				Expect(len(statusErr.ErrStatus.Details.Causes)).To(BeNumerically(">", 1))
				Expect(warnings).To(BeEmpty())
			})

			It("Should verify field paths in error messages for invalid bool", func() {
				required := true
				invalidBool := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("not-a-bool")}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "test-bool",
						ValueType:    "true_false",
						Required:     &required,
						DefaultValue: &invalidBool,
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				statusErr := err.(*errors.StatusError)
				Expect(statusErr.ErrStatus.Details.Causes).NotTo(BeEmpty())
				// Verify the field path is correct
				Expect(statusErr.ErrStatus.Details.Causes[0].Field).To(Equal("spec.customProperties[0].default_value"))
				Expect(warnings).To(BeEmpty())
			})

			It("Should verify field paths for duplicate property names", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("value")}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "duplicate",
						ValueType:    "string",
						Required:     &required,
						DefaultValue: &defaultValue,
					},
					{
						PropertyName: "duplicate",
						ValueType:    "string",
						Required:     &required,
						DefaultValue: &defaultValue,
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				statusErr := err.(*errors.StatusError)
				Expect(statusErr.ErrStatus.Details.Causes).NotTo(BeEmpty())
				// Duplicate error should point to the second occurrence
				Expect(statusErr.ErrStatus.Details.Causes[0].Field).To(Equal("spec.customProperties[1].property_name"))
				Expect(warnings).To(BeEmpty())
			})

			It("Should verify field paths for missing allowed_values", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("option1")}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "test-select",
						ValueType:    "single_select",
						Required:     &required,
						DefaultValue: &defaultValue,
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				statusErr := err.(*errors.StatusError)
				Expect(statusErr.ErrStatus.Details.Causes).NotTo(BeEmpty())
				Expect(statusErr.ErrStatus.Details.Causes[0].Field).To(Equal("spec.customProperties[0].allowed_values"))
				Expect(warnings).To(BeEmpty())
			})

			It("Should verify field paths for invalid allowed_values usage", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("value")}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName:  "test-string",
						ValueType:     "string",
						Required:      &required,
						DefaultValue:  &defaultValue,
						AllowedValues: []string{"not", "allowed"},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				statusErr := err.(*errors.StatusError)
				Expect(statusErr.ErrStatus.Details.Causes).NotTo(BeEmpty())
				Expect(statusErr.ErrStatus.Details.Causes[0].Field).To(Equal("spec.customProperties[0].allowed_values"))
				Expect(warnings).To(BeEmpty())
			})

			It("Should verify error message for single_select with invalid default", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("invalid-option")}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName:  "test-select",
						ValueType:     "single_select",
						Required:      &required,
						DefaultValue:  &defaultValue,
						AllowedValues: []string{"option1", "option2", "option3"},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				statusErr := err.(*errors.StatusError)
				Expect(statusErr.ErrStatus.Details.Causes).NotTo(BeEmpty())
				// Error message should list supported values
				Expect(statusErr.ErrStatus.Message).To(ContainSubstring("option1"))
				Expect(statusErr.ErrStatus.Message).To(ContainSubstring("option2"))
				Expect(statusErr.ErrStatus.Message).To(ContainSubstring("option3"))
				Expect(warnings).To(BeEmpty())
			})

			It("Should allow valid multi_select with all allowed values", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Values: []string{"option1", "option2", "option3"}}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName:  "test-multi-all",
						ValueType:     "multi_select",
						Required:      &required,
						DefaultValue:  &defaultValue,
						AllowedValues: []string{"option1", "option2", "option3"},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("Should reject multi_select with multiple invalid values", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Values: []string{"invalid1", "invalid2"}}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName:  "test-multi",
						ValueType:     "multi_select",
						Required:      &required,
						DefaultValue:  &defaultValue,
						AllowedValues: []string{"option1", "option2", "option3"},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				statusErr := err.(*errors.StatusError)
				// Should have errors for both invalid values
				Expect(len(statusErr.ErrStatus.Details.Causes)).To(BeNumerically(">=", 2))
				Expect(warnings).To(BeEmpty())
			})

			It("Should allow description field in custom properties", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("value")}
				description := "A test property"

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "test-with-desc",
						ValueType:    "string",
						Required:     &required,
						DefaultValue: &defaultValue,
						Description:  &description,
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("Should validate all value types in one organization", func() {
				required := true
				stringDefault := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("string-value")}
				boolDefault := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("true")}
				singleSelectDefault := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("opt1")}
				multiSelectDefault := githubv1alpha1.OrgCustomPropertyDefaultValue{Values: []string{"opt1", "opt2"}}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "string-prop",
						ValueType:    "string",
						Required:     &required,
						DefaultValue: &stringDefault,
					},
					{
						PropertyName: "bool-prop",
						ValueType:    "true_false",
						Required:     &required,
						DefaultValue: &boolDefault,
					},
					{
						PropertyName:  "single-prop",
						ValueType:     "single_select",
						Required:      &required,
						DefaultValue:  &singleSelectDefault,
						AllowedValues: []string{"opt1", "opt2", "opt3"},
					},
					{
						PropertyName:  "multi-prop",
						ValueType:     "multi_select",
						Required:      &required,
						DefaultValue:  &multiSelectDefault,
						AllowedValues: []string{"opt1", "opt2", "opt3"},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("Should allow 'false' as valid true_false default value", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("false")}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "test-bool-false",
						ValueType:    "true_false",
						Required:     &required,
						DefaultValue: &defaultValue,
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("Should reject true_false with case-incorrect values", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("True")}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "test-bool",
						ValueType:    "true_false",
						Required:     &required,
						DefaultValue: &defaultValue,
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				statusErr := err.(*errors.StatusError)
				Expect(statusErr.ErrStatus.Message).To(ContainSubstring("true"))
				Expect(statusErr.ErrStatus.Message).To(ContainSubstring("false"))
				Expect(warnings).To(BeEmpty())
			})

			It("Should verify field paths for required field missing default", func() {
				required := true

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "test-required",
						ValueType:    "string",
						Required:     &required,
						// Missing DefaultValue
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				statusErr := err.(*errors.StatusError)
				Expect(statusErr.ErrStatus.Details.Causes).NotTo(BeEmpty())
				Expect(statusErr.ErrStatus.Details.Causes[0].Field).To(Equal("spec.customProperties[0].default_value"))
				Expect(warnings).To(BeEmpty())
			})

			It("Should verify field paths for non-required with default", func() {
				required := false
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("should-not-be-here")}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "test-optional",
						ValueType:    "string",
						Required:     &required,
						DefaultValue: &defaultValue,
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				statusErr := err.(*errors.StatusError)
				Expect(statusErr.ErrStatus.Details.Causes).NotTo(BeEmpty())
				Expect(statusErr.ErrStatus.Details.Causes[0].Field).To(Equal("spec.customProperties[0].default_value"))
				Expect(warnings).To(BeEmpty())
			})

			It("Should allow single value from allowed values in multi_select", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Values: []string{"option1"}}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName:  "test-multi-single",
						ValueType:     "multi_select",
						Required:      &required,
						DefaultValue:  &defaultValue,
						AllowedValues: []string{"option1", "option2", "option3"},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("Should verify error message contains property name context", func() {
				required := true
				defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("invalid-bool")}

				obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
					{
						PropertyName: "my-boolean-property",
						ValueType:    "true_false",
						Required:     &required,
						DefaultValue: &defaultValue,
					},
				}

				warnings, err := validator.ValidateCreate(ctx, obj)
				Expect(err).To(HaveOccurred())
				statusErr := err.(*errors.StatusError)
				// The field path should contain enough context
				Expect(statusErr.ErrStatus.Details.Causes[0].Field).To(ContainSubstring("customProperties"))
				Expect(warnings).To(BeEmpty())
			})
		})
	})

	Context("When validating Organization update", func() {
		It("Should allow valid organization update", func() {
			warnings, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("Should reject update with wrong object type for newObj", func() {
			warnings, err := validator.ValidateUpdate(ctx, oldObj, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected an Organization object for the new object but got nil"))
			Expect(warnings).To(BeEmpty())
		})

		It("Should validate updated custom properties", func() {
			required := true
			defaultValue := githubv1alpha1.OrgCustomPropertyDefaultValue{Value: github.Ptr("updated-value")}

			obj.Spec.CustomProperties = []githubv1alpha1.OrgCustomProperty{
				{
					PropertyName: "updated-prop",
					ValueType:    "string",
					Required:     &required,
					DefaultValue: &defaultValue,
				},
			}

			warnings, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

	})

	Context("When validating Organization deletion", func() {
		It("Should allow deletion", func() {
			warnings, err := validator.ValidateDelete(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("Should reject deletion with wrong object type", func() {
			warnings, err := validator.ValidateDelete(ctx, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected an Organization object but got nil"))
			Expect(warnings).To(BeEmpty())
		})
	})
})
