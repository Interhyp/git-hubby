package v1alpha1

import (
	"context"

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
		WithValidator(&OrganizationValidator{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: If you want to customise the 'path', use the flags '--defaulting-path' or '--validation-path'.
// +kubebuilder:webhook:path=/validate-github-interhyp-de-v1alpha1-organization,mutating=false,failurePolicy=fail,sideEffects=None,groups=github.interhyp.de,resources=organizations,verbs=create;update,versions=v1alpha1,name=vorganization-v1alpha1.kb.io,admissionReviewVersions=v1

// OrganizationValidator struct is responsible for validating the Organization resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type OrganizationValidator struct {
	// TODO(user): Add more fields as needed for validation
}

// ValidateCreate implements admission.Validator so a webhook will be registered for the type Organization.
func (v *OrganizationValidator) ValidateCreate(_ context.Context, obj *githubv1alpha1.Organization) (admission.Warnings, error) {
	organizationlog.Info("Validation for Organization upon creation", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for the type Organization.
func (v *OrganizationValidator) ValidateUpdate(_ context.Context, oldObj, newObj *githubv1alpha1.Organization) (admission.Warnings, error) {
	organizationlog.Info("Validation for Organization upon update", "name", newObj.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements admission.Validator so a webhook will be registered for the type Organization.
func (v *OrganizationValidator) ValidateDelete(_ context.Context, obj *githubv1alpha1.Organization) (admission.Warnings, error) {
	organizationlog.Info("Validation for Organization upon deletion", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
