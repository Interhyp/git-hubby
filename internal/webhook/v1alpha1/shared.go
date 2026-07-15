package v1alpha1

import (
	"fmt"
	"slices"

	"github.com/google/go-github/v89/github"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type orgCustomPropertyValidation string

const (
	actualValueValidation  orgCustomPropertyValidation = "value"
	defaultValueValidation orgCustomPropertyValidation = "default value"
)

// CustomPropertyValueProvider generalizes access to custom property values or default values as both can have a single or multiple string values depending on the custom property type. At most one of GetValue or GetValues should return a non-nil value.
type CustomPropertyValueProvider interface {
	// GetValue returns the single value. It may be nil. It must be safe to call on a nil receiver.
	GetValue() *string
	// GetValues returns the slice of values. It may be nil. It must be safe to call on a nil receiver.
	GetValues() []string
}

// validateValueAgainstValueTypeAndAllowedValues should not be used directly.
// Use validateValueAgainstCustomPropertyDefinition or validateDefaultValue instead.
//
//nolint:goconst // Ignoring constant strings for better readability
func validateValueAgainstValueTypeAndAllowedValues(validationType orgCustomPropertyValidation, rawCp CustomPropertyValueProvider, valueType github.PropertyValueType, allowedValues []string, validatedField *field.Path) field.ErrorList {
	errs := make([]*field.Error, 0, 1)
	switch valueType {
	case github.PropertyValueTypeString, github.PropertyValueTypeURL:
		// value_type "string" is always valid, because values are of base type string
	case github.PropertyValueTypeTrueFalse:
		if rawCp.GetValue() != nil && *rawCp.GetValue() != "true" && *rawCp.GetValue() != "false" {
			errs = append(errs, field.NotSupported(validatedField, rawCp.GetValue(), []string{"true", "false"}))
		}
	case github.PropertyValueTypeSingleSelect:
		if rawCp.GetValue() != nil && !slices.Contains(allowedValues, *rawCp.GetValue()) {
			errs = append(errs, field.NotSupported(validatedField, rawCp.GetValue(), allowedValues))
		}
	case github.PropertyValueTypeMultiSelect:
		if rawCp.GetValue() != nil {
			errs = append(errs, field.Invalid(validatedField, rawCp, fmt.Sprintf("%s for custom property of type multi_select must be an array given as 'values'", validationType)))
			break
		}
		for _, v := range rawCp.GetValues() {
			if !slices.Contains(allowedValues, v) {
				errs = append(errs, field.NotSupported(validatedField, v, allowedValues))
			}
		}
		return errs
	default:
		// should not happen because of enum validation of v1alpha1.OrgCustomProperty ValueType
		errs = append(errs, field.InternalError(validatedField, fmt.Errorf("failed to validate %s against value_type because organization custom property has invalid value type: %s", validationType, valueType)))
	}
	return errs
}
