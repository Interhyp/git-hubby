package mapper

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/go-github/v86/github"
)

const CustomPropertySourceTypeOrganization = "organization"

func K8sOrgCustomPropertyMatchesGitHubCustomProperty(desired v1alpha1.OrgCustomProperty, current *github.CustomProperty) (bool, error) {
	if current == nil || !IsK8sOrgCustomProperty(current) {
		return false, nil
	}
	currentK8sRep, err := ToK8sOrgCustomProperty(current)
	if err != nil {
		return false, err
	}
	return cmp.Equal(desired, *currentK8sRep,
		cmpopts.EquateEmpty(),
		cmpopts.SortSlices(strings.Compare),
	), nil
}

func IsK8sOrgCustomProperty(current *github.CustomProperty) bool {
	return current.GetSourceType() == CustomPropertySourceTypeOrganization
}

func ToK8sOrgCustomProperty(current *github.CustomProperty) (*v1alpha1.OrgCustomProperty, error) {
	defaultVal, err := getK8sDefaultValue(current)
	if err != nil {
		return nil, err
	}
	result := v1alpha1.OrgCustomProperty{
		PropertyName:     current.GetPropertyName(),
		ValueType:        string(current.ValueType),
		Required:         current.Required,
		DefaultValue:     defaultVal,
		Description:      current.Description,
		AllowedValues:    current.AllowedValues,
		ValuesEditableBy: current.GetValuesEditableBy(),
	}

	return &result, nil
}

func getK8sDefaultValue(current *github.CustomProperty) (*v1alpha1.OrgCustomPropertyDefaultValue, error) {
	if current.DefaultValue == nil {
		return nil, nil
	}
	switch current.ValueType {
	case github.PropertyValueTypeMultiSelect:
		v, ok := current.DefaultValueStrings()
		if !ok {
			return nil, fmt.Errorf("failed to convert default values for custom property %s to []string: %+v", current.GetPropertyName(), current.DefaultValue)
		}
		return &v1alpha1.OrgCustomPropertyDefaultValue{
			Values: v,
		}, nil

	case github.PropertyValueTypeTrueFalse:
		v, ok := current.DefaultValueBool()
		if !ok {
			return nil, fmt.Errorf("failed to convert default value for custom property %s to bool: %+v", current.GetPropertyName(), current.DefaultValue)
		}
		b := fmt.Sprintf("%t", v)
		return &v1alpha1.OrgCustomPropertyDefaultValue{
			Value: &b,
		}, nil

	default: // string, url, single_select
		v, ok := current.DefaultValueString()
		if !ok {
			return nil, fmt.Errorf("failed to convert default value for custom property %s to string: %+v", current.GetPropertyName(), current.DefaultValue)
		}
		return &v1alpha1.OrgCustomPropertyDefaultValue{
			Value: &v,
		}, nil
	}
}

func ToGitHubCustomProperty(current v1alpha1.OrgCustomProperty) *github.CustomProperty {
	result := github.CustomProperty{
		PropertyName:     &current.PropertyName,
		ValueType:        github.PropertyValueType(current.ValueType),
		Required:         current.Required,
		DefaultValue:     getGitHubDefaultValue(current),
		Description:      current.Description,
		AllowedValues:    current.AllowedValues,
		ValuesEditableBy: &current.ValuesEditableBy,
	}
	return &result
}

func getGitHubDefaultValue(current v1alpha1.OrgCustomProperty) any {
	if current.DefaultValue == nil {
		return nil
	}
	switch current.ValueType {
	case "multi_select":
		return current.DefaultValue.Values
	default:
		v := current.DefaultValue.Value
		if v != nil {
			return *v
		}
		return v
	}
}

// ToGitHubCustomPropertyValues converts a map of property names to *string values to a list of GitHub CustomPropertyValues based on the provided definitions.
// The map keys are the property names, and the values are the values to be set for those properties.
// If a property defined in the definitions is not present in the map, it will be added to the result depending on the definition:
// if the custom property is required, the default value is used. Otherwise, the value is set to nil which "unsets"/removes the property on GitHub.
// This method does not validate the values against the value_type of the definitions, it only converts them accordingly.
func ToGitHubCustomPropertyValues(raw []v1alpha1.CustomPropertyValue, definitions []*github.CustomProperty) ([]*github.CustomPropertyValue, error) {
	values := make([]*github.CustomPropertyValue, 0, len(raw))
	rawByName := make(map[string]v1alpha1.CustomPropertyValue)
	for _, cpv := range raw {
		rawByName[cpv.PropertyName] = cpv
	}
	errs := make([]error, 0)
	for _, definition := range definitions {
		rawCPV, found := rawByName[definition.GetPropertyName()]
		if found {
			values = append(values, ToGitHubCustomPropertyValue(rawCPV, definition))
		} else {
			if definition.Required != nil && *definition.Required {
				defaultVal, err := getDefaultValueAsCPV(definition)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				values = append(values, defaultVal)
			} else {
				values = append(values, &github.CustomPropertyValue{
					PropertyName: definition.GetPropertyName(),
					Value:        nil, // unset value
				})
			}
		}
	}
	return values, errors.Join(errs...)
}

func getDefaultValueAsCPV(definition *github.CustomProperty) (*github.CustomPropertyValue, error) {
	switch definition.ValueType {
	case github.PropertyValueTypeMultiSelect:
		val, ok := definition.DefaultValueStrings()
		if !ok {
			return nil, fmt.Errorf("failed to convert default values for required custom property %s to []string: %+v", definition.GetPropertyName(), definition.DefaultValue)
		}
		return &github.CustomPropertyValue{
			PropertyName: definition.GetPropertyName(),
			Value:        val,
		}, nil
	default:
		val, ok := definition.DefaultValueString()
		if !ok {
			return nil, fmt.Errorf("failed to convert default value for required custom property %s to string: %+v", definition.GetPropertyName(), definition.DefaultValue)
		}
		return &github.CustomPropertyValue{
			PropertyName: definition.GetPropertyName(),
			Value:        val,
		}, nil
	}
}

// ToGitHubCustomPropertyValue converts a v1alpha1.CustomPropertyValue to a GitHub CustomPropertyValue based on the definition.
// If value is nil, the Value field of the CustomPropertyValue will also be nil which "unsets"/removes the property.
// This method does not validate the value against the definition, it only converts it. Validation should have been
// done by the validating webhook. Any errors occurring during conversion will be ignored and the value will be set as is.
func ToGitHubCustomPropertyValue(cp v1alpha1.CustomPropertyValue, definition *github.CustomProperty) *github.CustomPropertyValue {
	result := &github.CustomPropertyValue{
		PropertyName: definition.GetPropertyName(),
		Value:        nil,
	}
	switch definition.ValueType {
	case "string", "single_select", "true_false":
		if cp.Value != nil {
			result.Value = *cp.Value
		}
	case "multi_select":
		result.Value = cp.Values
	}

	return result
}
