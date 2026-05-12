package reporec

import (
	"context"
	"strings"

	"github.com/Interhyp/git-hubby/internal/mapper"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/go-github/v86/github"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

// applyRepositoryCustomProperties creates or applies the custom properties of a repository on GitHub.
func (r *GitHubRepoReconciler) reconcileCustomProperties(ctx context.Context) error {
	log := logPkg.FromContext(ctx)
	log.V(1).Info("Reconciling repository custom properties on GitHub")

	currentValues, err := r.GitHub.Client.GetAllCustomPropertyValues(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name)
	if err != nil {
		log.Error(err, "failed to get custom properties values from GitHub")
		return err
	}
	definitions, err := r.GitHub.Client.GetAllCustomPropertiesForOrganization(ctx, r.GitHub.Resource.Owner)
	if err != nil {
		log.Error(err, "failed to get custom properties definitions from GitHub")
		return err
	}
	desiredValues, err := mapper.ToGitHubCustomPropertyValues(r.Kubernetes.Resource.Spec.CustomProperties, definitions)
	if err != nil {
		log.Error(err, "failed to map all repository custom properties to GitHub representations")
		return err
	}
	expected := expectedFromDesiredCustomPropertyValues(desiredValues)
	isDifferent := !cmp.Equal(currentValues, expected,
		cmpopts.EquateEmpty(),
		cmpopts.SortSlices(func(a, b *github.CustomPropertyValue) int {
			return strings.Compare(a.PropertyName, b.PropertyName)
		}),
		cmpopts.SortSlices(strings.Compare), // for sorting string array fields
	)
	if isDifferent {
		log.V(1).Info("Repository custom properties differ from desired state, updating them")
		if err := r.GitHub.Client.CreateOrUpdateRepositoryCustomProperties(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, desiredValues); err != nil {
			log.Error(err, "failed to update repository custom properties on GitHub")
			return err
		}
	} else {
		log.V(1).Info("Repository custom properties already match desired state, skipping update")
		return nil
	}
	log.V(1).Info("Successfully reconciled repository custom properties on GitHub")
	return nil
}

func expectedFromDesiredCustomPropertyValues(desiredValues []*github.CustomPropertyValue) []*github.CustomPropertyValue {
	// nil desired values mean unsetting the value.
	// If the value is unset, it is not returned from github and thus we should not expect it to be present in the current values,
	// so we filter them out from the expected values for comparison.
	expected := make([]*github.CustomPropertyValue, 0)
	for _, val := range desiredValues {
		if val.Value != nil {
			expected = append(expected, val)
		}
	}
	return expected
}
