/*
Copyright 2026.

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

package v1alpha1

import (
	"context"
	"fmt"

	"github.com/google/go-github/v86/github"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
)

// nolint:unused
// log is for logging in this package.
var organizationlog = logf.Log.WithName("organization-resource")

// SetupOrganizationWebhookWithManager registers the webhook for Organization in the manager.
func SetupOrganizationWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &githubv1alpha1.Organization{}).
		WithValidator(&OrganizationCustomValidator{}).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: If you want to customise the 'path', use the flags '--defaulting-path' or '--validation-path'.
// +kubebuilder:webhook:path=/validate-github-interhyp-de-v1alpha1-organization,mutating=false,failurePolicy=fail,sideEffects=None,groups=github.interhyp.de,resources=organizations,verbs=create;update,versions=v1alpha1,name=vorganization-v1alpha1.kb.io,admissionReviewVersions=v1

// OrganizationCustomValidator struct is responsible for validating the Organization resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type OrganizationCustomValidator struct{}

var _ admission.Validator[*githubv1alpha1.Organization] = &OrganizationCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Organization.
func (v *OrganizationCustomValidator) ValidateCreate(_ context.Context, organization *githubv1alpha1.Organization) (admission.Warnings, error) {
	if organization == nil {
		return nil, fmt.Errorf("expected an Organization object but got nil")
	}
	organizationlog.Info("Validation for Organization upon creation", "name", organization.GetName())

	return nil, validateOrganization(organization)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Organization.
func (v *OrganizationCustomValidator) ValidateUpdate(_ context.Context, _ *githubv1alpha1.Organization, organization *githubv1alpha1.Organization) (admission.Warnings, error) {
	if organization == nil {
		return nil, fmt.Errorf("expected an Organization object for the new object but got nil")
	}
	organizationlog.Info("Validation for Organization upon update", "name", organization.GetName())

	return nil, validateOrganization(organization)
}

func validateOrganization(organization *githubv1alpha1.Organization) error {
	var allErrs field.ErrorList

	// Validate that at least one of login or name is set
	if organization.Spec.Login == "" && organization.Spec.Name == "" {
		allErrs = append(allErrs, field.Required(
			field.NewPath("spec"),
			"either 'login' or 'name' must be specified",
		))
	}

	customPropertiesField := field.NewPath("spec").Child("customProperties")
	allErrs = append(allErrs, validateCustomProperties(organization.Spec.CustomProperties, customPropertiesField)...)

	allErrs = append(allErrs, validatePlanFeatureCombinations(organization)...)
	allErrs = append(allErrs, validateGitHubAppConfig(organization)...)

	if len(allErrs) == 0 {
		return nil
	}
	return errors.NewInvalid(
		organization.GroupVersionKind().GroupKind(),
		organization.Name, allErrs)
}

// validateGitHubAppConfig ensures that at least one of githubAppConfig or githubAppInstallationId is set.
func validateGitHubAppConfig(organization *githubv1alpha1.Organization) field.ErrorList {
	if organization.Spec.GitHubAppConfig != nil || organization.Spec.GitHubAppInstallationId != nil {
		return nil
	}
	specPath := field.NewPath("spec")
	return field.ErrorList{
		field.Required(specPath, "at least one of githubAppConfig or githubAppInstallationId must be set"),
	}
}

func validatePlanFeatureCombinations(organization *githubv1alpha1.Organization) field.ErrorList {
	plan := organization.Spec.Plan
	if plan == "" || plan == githubv1alpha1.PlanEnterprise {
		return nil
	}

	var errs field.ErrorList
	specPath := field.NewPath("spec")

	// Rulesets are available on 'team' and 'enterprise' plans
	if plan != githubv1alpha1.PlanTeam && len(organization.Spec.RulesetPresetList) > 0 {
		errs = append(errs, field.Forbidden(
			specPath.Child("rulesetPresets"),
			fmt.Sprintf("organization rulesets require the 'enterprise' or 'team' plan, but plan is '%s'", plan),
		))
	}

	// Code security configurations are only available on the 'enterprise' plan
	if len(organization.Spec.CodeSecurityConfigurations) > 0 {
		errs = append(errs, field.Forbidden(
			specPath.Child("codeSecurityConfigurations"),
			fmt.Sprintf("code security configurations require the 'enterprise' plan, but plan is '%s'", plan),
		))
	}

	return errs
}

func validateCustomProperties(customProperties []githubv1alpha1.OrgCustomProperty, customPropertiesField *field.Path) field.ErrorList {
	errs := make([]*field.Error, 0, 1)
	seen := make(map[string]any)
	duplicates := make(map[string]int)
	for index, property := range customProperties {
		currentField := customPropertiesField.Index(index)
		if _, alreadyPresent := seen[property.PropertyName]; alreadyPresent {
			duplicates[property.PropertyName] = index
		}
		seen[property.PropertyName] = struct{}{}
		if err := validateCustomPropertyAllowedValues(property, currentField.Child("allowed_values")); err != nil {
			errs = append(errs, err)
		}
		// validate allowed_values first because default_value validation assumes that allowed_values are valid
		errs = append(errs, validateCustomPropertyDefaultValues(property, currentField.Child("default_value"))...)
	}
	for duplicate, index := range duplicates {
		errs = append(errs, field.Duplicate(customPropertiesField.Index(index).Child("property_name"), duplicate))
	}
	return errs
}

func validateCustomPropertyAllowedValues(property githubv1alpha1.OrgCustomProperty, allowedValuesField *field.Path) *field.Error {
	switch property.ValueType {
	case "single_select", "multi_select":
		if len(property.AllowedValues) == 0 {
			return field.Required(allowedValuesField, "allowed_values must be set for custom properties with value_type 'single_select' or 'multi_select'")
		}
	default:
		if property.AllowedValues != nil {
			return field.Invalid(allowedValuesField, property.AllowedValues, "allowed_values must not be set for custom properties with value_type 'string' or 'true_false'")
		}
	}
	return nil
}

func validateCustomPropertyDefaultValues(property githubv1alpha1.OrgCustomProperty, defaultValueField *field.Path) field.ErrorList {
	required := false
	if property.Required != nil {
		required = *property.Required
	}
	if required && property.DefaultValue == nil {
		return field.ErrorList{field.Required(defaultValueField, "default_value must be set for required custom properties")}
	}
	if !required && property.DefaultValue != nil {
		return field.ErrorList{field.Invalid(defaultValueField, *property.DefaultValue, "default_value must not be set for non-required custom properties")}
	}
	return validateDefaultValue(property, defaultValueField)
}

// validateValueAgainstValueTypeAndAllowedValues checks whether the default value of the given definition of an organization level custom property is valid.
// The default value is valid if it conforms to the value_type and the allowed_values.
// The *field.Path parameter child is the field path to the value being validated, which is used to create meaningful field errors.
// This method assumes that the allowed_values field of the OrgCustomProperty is valid (i.e. not empty for selection based value_types).
func validateDefaultValue(propDefinition githubv1alpha1.OrgCustomProperty, validatedField *field.Path) field.ErrorList {
	return validateValueAgainstValueTypeAndAllowedValues(defaultValueValidation, propDefinition.DefaultValue, github.PropertyValueType(propDefinition.ValueType), propDefinition.AllowedValues, validatedField)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Organization.
func (v *OrganizationCustomValidator) ValidateDelete(_ context.Context, organization *githubv1alpha1.Organization) (admission.Warnings, error) {
	if organization == nil {
		return nil, fmt.Errorf("expected an Organization object but got nil")
	}
	organizationlog.Info("Validation for Organization upon deletion", "name", organization.GetName())
	// nothing to do here as deletion validation is not activated
	return nil, nil
}
