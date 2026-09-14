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
var repositorylog = logf.Log.WithName("repository-resource")

// SetupRepositoryWebhookWithManager registers the webhook for Repository in the manager.
func SetupRepositoryWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &githubv1alpha1.Repository{}).
		WithValidator(&RepositoryValidator{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: If you want to customise the 'path', use the flags '--defaulting-path' or '--validation-path'.
// +kubebuilder:webhook:path=/validate-github-interhyp-de-v1alpha1-repository,mutating=false,failurePolicy=fail,sideEffects=None,groups=github.interhyp.de,resources=repositories,verbs=create;update,versions=v1alpha1,name=vrepository-v1alpha1.kb.io,admissionReviewVersions=v1

// RepositoryValidator struct is responsible for validating the Repository resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type RepositoryValidator struct {
	// TODO(user): Add more fields as needed for validation
}

// ValidateCreate implements admission.Validator so a webhook will be registered for the type Repository.
func (v *RepositoryValidator) ValidateCreate(_ context.Context, obj *githubv1alpha1.Repository) (admission.Warnings, error) {
	repositorylog.Info("Validation for Repository upon creation", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for the type Repository.
func (v *RepositoryValidator) ValidateUpdate(_ context.Context, oldObj, newObj *githubv1alpha1.Repository) (admission.Warnings, error) {
	repositorylog.Info("Validation for Repository upon update", "name", newObj.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements admission.Validator so a webhook will be registered for the type Repository.
func (v *RepositoryValidator) ValidateDelete(_ context.Context, obj *githubv1alpha1.Repository) (admission.Warnings, error) {
	repositorylog.Info("Validation for Repository upon deletion", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
