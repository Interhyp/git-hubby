package orgrec

import (
	"context"
	"fmt"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/mapper"
	"github.com/Interhyp/git-hubby/internal/reconciler"
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
	desiredRulesets, err := reconciler.CollectDesiredRulesets(ctx, o.Kubernetes.Client, o.Kubernetes.Resource.Namespace,
		o.Kubernetes.Resource.Spec.RulesetPresetList, o.Kubernetes.Resource.Spec.Rulesets)
	if err != nil {
		return err
	}

	existingRulesets, err := o.GitHub.Client.GetAllOrganizationRulesets(ctx, o.GitHub.Resource, false)
	if err != nil {
		log.Error(err, "failed to get existing organization rulesets")
		return fmt.Errorf("failed to get existing organization rulesets: %w", err)
	}

	rulesetsToDelete := make(map[string]*github.RepositoryRuleset)
	existingRulesetsByName := make(map[string]*github.RepositoryRuleset)
	for _, ruleset := range existingRulesets {
		if ruleset.Name == "" {
			continue
		}
		existingRulesetsByName[ruleset.Name] = ruleset
		rulesetsToDelete[ruleset.Name] = ruleset
	}

	for _, desired := range desiredRulesets {
		log := log.WithValues("rulesetSource", desired.Source, "ruleset", desired.Spec.Name)
		log.V(1).Info("Processing ruleset")

		// Strip spec fields for disabled beta features before any processing.
		if !o.Features.EnableRequiredReviewersRules && desired.Spec.Rules.PullRequest != nil {
			desired.Spec.Rules.PullRequest.RequiredReviewers = nil
		}

		desired.Spec, err = o.IdResolver.ResolveRuleset(ctx, desired.Spec)
		if err != nil {
			log.Error(err, "failed to resolve ruleset slugs to IDs")
			return fmt.Errorf("failed to resolve slugs of ruleset %s (%s) to IDs: %w", desired.Spec.Name, desired.Source, err)
		}
		desired.Spec.Conditions = addDefaultOrgRepositoryConditions(desired.Spec.Conditions)

		githubRuleset, err := mapper.RulesetSpecToGithubRuleset(desired.Spec)
		if err != nil {
			log.Error(err, "failed to convert ruleset to GitHub ruleset")
			return fmt.Errorf("failed to convert ruleset %s (%s) to GitHub ruleset: %w", desired.Spec.Name, desired.Source, err)
		}

		if existingRuleset, exists := existingRulesetsByName[desired.Spec.Name]; exists {
			if existingRuleset.ID == nil {
				log.Info("WARNING: Existing ruleset has nil ID, skipping")
				continue
			}
			fullRuleset, getErr := o.GitHub.Client.GetOrganizationRuleset(ctx, o.GitHub.Resource, *existingRuleset.ID)
			if getErr != nil {
				log.Error(getErr, "failed to get existing organization ruleset")
				return fmt.Errorf("failed to get existing organization ruleset %s: %w", desired.Spec.Name, getErr)
			}
			delete(rulesetsToDelete, desired.Spec.Name)
			if mapper.RulesetsDiffer(desired.Spec, *fullRuleset) {
				rulesetID := fullRuleset.ID
				if rulesetID == nil {
					continue
				}
				if err := o.updateRuleset(ctx, *rulesetID, githubRuleset, desired); err != nil {
					return err
				}
			} else {
				log.V(1).Info("Ruleset already matches desired state, skipping")
			}
		} else {
			if err := o.createRuleset(ctx, githubRuleset, desired); err != nil {
				return err
			}
		}
	}

	for rulesetName, rulesetToDelete := range rulesetsToDelete {
		log := log.WithValues("rulesetName", rulesetName)
		log.V(1).Info("Deleting orphaned organization ruleset")
		rulesetID := rulesetToDelete.ID
		if rulesetID != nil {
			if err := o.deleteRuleSet(ctx, *rulesetID, rulesetName); err != nil {
				return err
			}
		}
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

func (o *GitHubOrgReconciler) updateRuleset(ctx context.Context, rulesetID int64, githubRuleset *github.RepositoryRuleset, desired reconciler.DesiredRuleset) error {
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

func (o *GitHubOrgReconciler) createRuleset(ctx context.Context, githubRuleset *github.RepositoryRuleset, desired reconciler.DesiredRuleset) error {
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
