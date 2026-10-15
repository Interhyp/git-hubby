package orgrec

import (
	"context"
	"fmt"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler/common/rulesets"
	"github.com/google/go-github/v92/github"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

// reconcileRulesets reconciles the union of inline rulesets (spec.rulesets) and the rulesets referenced
// via spec.rulesetPresets against the organization rulesets on GitHub.
func (o *GitHubOrgReconciler) reconcileRulesets(ctx context.Context) error {
	log := logPkg.FromContext(ctx)

	// Org rulesets are not supported on the free plan
	if !o.Kubernetes.Resource.HasEnterpriseFeatures() {
		log.V(1).Info("Skipping organization rulesets reconciliation for non-public repository on free plan")
		return nil
	}

	// Collect desired rulesets before any GitHub call so that conflicts prevent all ruleset changes.
	desiredRulesets, err := rulesets.Collect(ctx, o.Kubernetes.Client, o.Kubernetes.Resource.Namespace,
		o.Kubernetes.Resource.Spec.RulesetPresetList, o.Kubernetes.Resource.Spec.Rulesets)
	if err != nil {
		return err
	}

	target := rulesets.Target{
		Scope: "organization",
		List: func(ctx context.Context) ([]*github.RepositoryRuleset, error) {
			return o.GitHub.Client.GetAllOrganizationRulesets(ctx, o.GitHub.Resource, false)
		},
		Get: func(ctx context.Context, id int64) (*github.RepositoryRuleset, error) {
			return o.GitHub.Client.GetOrganizationRuleset(ctx, o.GitHub.Resource, id)
		},
		Create: o.createRuleset,
		Update: o.updateRuleset,
		Delete: o.deleteRuleSet,
	}
	opts := rulesets.SyncOptions{
		Resolver:                     o.IdResolver,
		EnableRequiredReviewersRules: o.Features.EnableRequiredReviewersRules,
		Adjust: func(spec *v1alpha1.RulesetSpec) {
			spec.Conditions = addDefaultOrgRepositoryConditions(spec.Conditions)
		},
	}
	if err := rulesets.Sync(ctx, target, desiredRulesets, opts); err != nil {
		return err
	}

	log.V(1).Info("Successfully reconciled organization rule sets on GitHub")
	return nil
}

func (o *GitHubOrgReconciler) deleteRuleSet(ctx context.Context, rulesetID int64, rulesetName string) error {
	log := logPkg.FromContext(ctx).WithValues("RuleSet", rulesetName)
	log.V(1).Info("Deleting organization ruleset")

	err := o.GitHub.Client.DeleteOrganizationRuleset(ctx, o.GitHub.Resource, rulesetID)
	if err != nil {
		log.Error(err, "failed to delete organization ruleset")
		return fmt.Errorf("failed to delete organization ruleset %s: %w", rulesetName, err)
	}
	log.V(1).Info("Organization ruleset deleted successfully")
	return nil
}

func (o *GitHubOrgReconciler) updateRuleset(ctx context.Context, rulesetID int64, githubRuleset *github.RepositoryRuleset, desired rulesets.Desired) error {
	log := logPkg.FromContext(ctx).WithValues("rulesetSource", desired.Source, "RuleSet", githubRuleset.Name)
	log.V(1).Info("Updating existing organization ruleset")

	_, err := o.GitHub.Client.UpdateOrganizationRuleset(ctx, o.GitHub.Resource, rulesetID, githubRuleset)
	if err != nil {
		log.Error(err, "failed to update organization ruleset")
		return fmt.Errorf("failed to update organization ruleset %s (%s): %w", desired.Spec.Name, desired.Source, err)
	}
	log.V(1).Info("Organization ruleset updated successfully")
	return nil
}

func (o *GitHubOrgReconciler) createRuleset(ctx context.Context, githubRuleset *github.RepositoryRuleset, desired rulesets.Desired) error {
	log := logPkg.FromContext(ctx).WithValues("rulesetSource", desired.Source, "RuleSet", githubRuleset.Name)
	log.V(1).Info("Creating new organization ruleset")
	_, err := o.GitHub.Client.CreateOrganizationRuleset(ctx, o.GitHub.Resource, githubRuleset)
	if err != nil {
		log.Error(err, "failed to create organization ruleset")
		return fmt.Errorf("failed to create organization ruleset %s (%s): %w", desired.Spec.Name, desired.Source, err)
	}
	log.V(1).Info("Organization ruleset created successfully")
	return nil
}

// addDefaultOrgRepositoryConditions sets the default ~ALL repository name condition
// when no explicit repository conditions are specified.
func addDefaultOrgRepositoryConditions(conditions *v1alpha1.RulesetConditions) *v1alpha1.RulesetConditions {
	if conditions == nil {
		return nil
	}
	if conditions.RepositoryName == nil && conditions.RepositoryProperty == nil {
		conditions.RepositoryName = &v1alpha1.RepositoryNameCondition{
			Include:   []string{"~ALL"},
			Exclude:   []string{},
			Protected: new(false),
		}
	}
	return conditions
}
