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
	baseerrors "errors"
	"fmt"

	"github.com/Interhyp/git-hubby/internal/ghclient"
	"github.com/google/go-github/v86/github"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
)

// nolint:unused
// log is for logging in this package.
var repositorylog = logf.Log.WithName("repository-resource")

// SetupRepositoryWebhookWithManager registers the webhook for Repository in the manager.
func SetupRepositoryWebhookWithManager(mgr ctrl.Manager, clientManager GitHubClientManager) error {
	return ctrl.NewWebhookManagedBy(mgr, &githubv1alpha1.Repository{}).
		WithValidator(&RepositoryCustomValidator{
			K8sClient:           mgr.GetClient(),
			GitHubClientManager: clientManager,
		}).
		Complete()
}

type GitHubClientManager interface {
	GetClient(ctx context.Context, orgName string, appInstallationID int64) (ghclient.GitHubClient, error)
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: If you want to customise the 'path', use the flags '--defaulting-path' or '--validation-path'.
// +kubebuilder:webhook:path=/validate-github-interhyp-de-v1alpha1-repository,mutating=false,failurePolicy=fail,sideEffects=None,groups=github.interhyp.de,resources=repositories,verbs=create;update,versions=v1alpha1,name=vrepository-v1alpha1.kb.io,admissionReviewVersions=v1

// RepositoryCustomValidator struct is responsible for validating the Repository resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type RepositoryCustomValidator struct {
	// TODO fugly: find a way to validate without doing either k8s or github api calls
	K8sClient           client.Client
	GitHubClientManager GitHubClientManager
}

var _ admission.Validator[*githubv1alpha1.Repository] = &RepositoryCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Repository.
func (v *RepositoryCustomValidator) ValidateCreate(ctx context.Context, repository *githubv1alpha1.Repository) (admission.Warnings, error) {
	if repository == nil {
		return nil, fmt.Errorf("expected a Repository object but got nil")
	}
	repositorylog.Info("Validation for Repository upon creation", "name", repository.GetName())

	return nil, v.validateRepository(ctx, repository)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Repository.
func (v *RepositoryCustomValidator) ValidateUpdate(ctx context.Context, _ *githubv1alpha1.Repository, repository *githubv1alpha1.Repository) (admission.Warnings, error) {
	if repository == nil {
		return nil, fmt.Errorf("expected a Repository object for the new object but got nil")
	}
	repositorylog.Info("Validation for Repository upon update", "name", repository.GetName())

	return nil, v.validateRepository(ctx, repository)

}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Repository.
func (v *RepositoryCustomValidator) ValidateDelete(_ context.Context, repository *githubv1alpha1.Repository) (admission.Warnings, error) {
	if repository == nil {
		return nil, fmt.Errorf("expected a Repository object but got nil")
	}
	repositorylog.Info("Validation for Repository upon deletion", "name", repository.GetName())

	// nothing to do here as deletion validation is not activated
	return nil, nil
}
func (v *RepositoryCustomValidator) validateRepository(ctx context.Context, repo *githubv1alpha1.Repository) error {
	allErrs := make([]*field.Error, 0, 1)

	// TODO find better or cached solution to avoid fetching organization and custom property definitions for every repository validation
	var org githubv1alpha1.Organization
	if err := v.K8sClient.Get(ctx, client.ObjectKey{Name: repo.Spec.OrganizationRef.Name, Namespace: repo.Namespace}, &org); err != nil {
		return fmt.Errorf("failed to fetch organization during validation of repository %s: %w", repo.Name, err)
	}
	githubClient, err := v.GitHubClientManager.GetClient(ctx, org.GetLogin(), org.Spec.GitHubAppInstallationId)
	if err != nil {
		return fmt.Errorf("failed to create GitHub client for organization %s during validation of repository %s: %w", org.GetLogin(), repo.Name, err)
	}
	customPropertyDefinitions, err := githubClient.GetAllCustomPropertiesForOrganization(ctx, org.GetLogin())
	if err != nil {
		return fmt.Errorf("failed to fetch custom properties for GitHub organization %s during validation of repository %s: %w", org.GetLogin(), repo.Name, err)
	}
	// TODO end of external requests

	allErrs = append(allErrs, validateCustomPropertyValuesHaveCorrectTypes(customPropertyDefinitions, repo.Spec.CustomProperties, field.NewPath("spec").Child("customProperties"))...)
	if len(allErrs) == 0 {
		return nil
	}
	return errors.NewInvalid(
		repo.GroupVersionKind().GroupKind(),
		repo.Name, allErrs)
}

func validateCustomPropertyValuesHaveCorrectTypes(customPropertyDefinitions []*github.CustomProperty, rawValues []githubv1alpha1.CustomPropertyValue, fldPath *field.Path) field.ErrorList {
	errs := make([]*field.Error, 0, len(rawValues))
	rawValuesMap := make(map[string]githubv1alpha1.CustomPropertyValue)
	for _, rawValue := range rawValues {
		rawValuesMap[rawValue.PropertyName] = rawValue
	}
	for _, propDefinition := range customPropertyDefinitions {
		if propDefinition == nil {
			errs = append(errs, field.InternalError(fldPath, baseerrors.New("received nil custom property definition from GitHub")))
			continue // skip nil definitions
		}
		propertyName := propDefinition.GetPropertyName()
		if rawValue, ok := rawValuesMap[propertyName]; ok {
			vErrs := validateValueAgainstCustomPropertyDefinition(rawValue, *propDefinition, fldPath.Child(propertyName))
			if vErrs != nil {
				errs = append(errs, vErrs...)
			}
		}
	}
	return errs
}

// validateValueAgainstCustomPropertyDefinition checks whether the given value is valid against the given definition of an organization level custom property.
// The validationType parameter indicates whether an actual value set for a repository or the default value of an
// OrgCustomProperty is validated.
// The *field.Path parameter child is the field path to the value being validated, which is used to create meaningful field errors.
// This method assumes that the allowed_values field of the OrgCustomProperty is valid (i.e. not empty for selection based value_types).
func validateValueAgainstCustomPropertyDefinition(value githubv1alpha1.CustomPropertyValue, propDefinition github.CustomProperty, validatedField *field.Path) field.ErrorList {
	return validateValueAgainstValueTypeAndAllowedValues(actualValueValidation, &value, propDefinition.ValueType, propDefinition.AllowedValues, validatedField)
}
