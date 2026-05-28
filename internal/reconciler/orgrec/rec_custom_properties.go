package orgrec

import (
	"context"

	"github.com/Interhyp/git-hubby/internal/mapper"
	"github.com/google/go-github/v86/github"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

// reconcileCustomProperties configures the custom properties of the GitHub organization to match the desired state defined in the Kubernetes resource.
func (o *GitHubOrgReconciler) reconcileCustomProperties(ctx context.Context) error {
	log := logPkg.FromContext(ctx)
	log.V(1).Info("Reconciling organization level custom properties on GitHub")

	currentOrgCustomProps, err := o.getGitHubOrgCustomPropertiesByPropertyName(ctx)
	if err != nil {
		return err
	}
	desiredOrgCustomProps := o.Kubernetes.Resource.Spec.CustomProperties
	orgCustomPropsToApply := make([]*github.CustomProperty, 0)
	hasNewOrUpdatedProps := false
	for _, desired := range desiredOrgCustomProps {
		if desired.Required == nil {
			desired.Required = new(false)
		}
		desiredGhRep := mapper.ToGitHubCustomProperty(desired)
		// always add to apply list, we will only apply if update is needed
		orgCustomPropsToApply = append(orgCustomPropsToApply, desiredGhRep)

		matchingCurrent, matchFound := currentOrgCustomProps[desired.PropertyName]
		if matchFound {
			// currently present -> has updated props if it does not match
			matches, mErr := mapper.K8sOrgCustomPropertyMatchesGitHubCustomProperty(desired, matchingCurrent)
			if mErr != nil {
				log.Error(mErr, "failed to compare desired custom property to matching GitHub custom property", "property", desired.PropertyName)
				return mErr
			}
			hasNewOrUpdatedProps = hasNewOrUpdatedProps || !matches
		} else {
			// currently missing
			hasNewOrUpdatedProps = true
		}
	}

	// unequal length means that some properties are no longer desired, so we need to remove them
	if hasNewOrUpdatedProps || len(orgCustomPropsToApply) != len(currentOrgCustomProps) {
		// update as batch to save on requests
		if _, err := o.GitHub.Client.CreateOrUpdateOrganizationCustomProperties(ctx, o.GitHub.Resource, orgCustomPropsToApply); err != nil {
			log.Error(err, "failed to update organization custom properties on GitHub")
			return err
		}
	}

	log.V(1).Info("Successfully reconciled organization level custom properties on GitHub")
	return nil
}

func (o *GitHubOrgReconciler) getGitHubOrgCustomPropertiesByPropertyName(ctx context.Context) (map[string]*github.CustomProperty, error) {
	log := logPkg.FromContext(ctx)

	allCustomProps, err := o.GitHub.Client.GetAllCustomPropertiesForOrganization(ctx, o.GitHub.Resource)
	if err != nil {
		log.Error(err, "failed get all custom properties for organization")
		return nil, err
	}

	orgCustomProps := retainOnlyOrgProperties(allCustomProps)
	mapped := make(map[string]*github.CustomProperty)
	for _, prop := range orgCustomProps {
		if prop.GetPropertyName() == "" {
			continue
		}
		mapped[prop.GetPropertyName()] = prop
	}
	return mapped, nil
}

func retainOnlyOrgProperties(current []*github.CustomProperty) []*github.CustomProperty {
	filtered := make([]*github.CustomProperty, 0, len(current))
	for _, property := range current {
		if property.GetSourceType() == mapper.CustomPropertySourceTypeOrganization {
			filtered = append(filtered, property)
		}
	}
	return filtered
}
