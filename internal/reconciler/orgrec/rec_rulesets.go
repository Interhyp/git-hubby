package orgrec

import (
	"context"
	"fmt"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/mapper"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/google/go-github/v86/github"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

func (o *GitHubOrgReconciler) reconcileRulesetPresets(ctx context.Context) error {
	log := logPkg.FromContext(ctx)

	// Rulesets are not supported on the free plan
	if o.Kubernetes.Resource.GetPlan() == "free" {
		log.V(1).Info("Skipping rulesets reconciliation for non-public repository on free plan")
		return nil
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

	for _, rulesetRef := range o.Kubernetes.Resource.Spec.RulesetPresetList {
		log := log.WithValues("rulesetPreset", rulesetRef.Name)
		log.V(1).Info("Processing ruleset preset")

		var rulesetPreset v1alpha1.RulesetPreset
		if err := o.Kubernetes.Client.Get(ctx, client.ObjectKey{
			Name:      rulesetRef.Name,
			Namespace: o.Kubernetes.Resource.Namespace,
		}, &rulesetPreset); err != nil {
			log.Error(err, "unable to get ruleset preset")
			return fmt.Errorf("failed to get ruleset preset %s: %w", rulesetRef.Name, err)
		}
		rulesetPreset, err := reconciler.ResolveNamesToIDsInRuleset(ctx, o.GitHub.Client, o.GitHub.Resource, rulesetPreset)
		if err != nil {
			log.Error(err, "failed to resolve ruleset slugs to IDs")
			return fmt.Errorf("failed to resolve slugs of ruleset %s to IDs: %w", rulesetRef.Name, err)
		}
		rulesetPreset.Spec.Conditions = addDefaultOrgRepositoryConditions(rulesetPreset.Spec.Conditions)

		githubRuleset, err := mapper.RulesetPresetToGithubRuleset(rulesetPreset)
		if err != nil {
			log.Error(err, "failed to convert ruleset preset to GitHub ruleset")
			return fmt.Errorf("failed to convert ruleset preset %s to GitHub ruleset: %w", rulesetRef.Name, err)
		}

		if existingRuleset, exists := existingRulesetsByName[rulesetPreset.Spec.Name]; exists {
			if existingRuleset.ID == nil {
				log.Info("WARNING: Existing ruleset has nil ID, skipping")
				continue
			}
			fullRuleset, getErr := o.GitHub.Client.GetOrganizationRuleset(ctx, o.GitHub.Resource, *existingRuleset.ID)
			if getErr != nil {
				log.Error(getErr, "failed to get existing organization ruleset")
				return fmt.Errorf("failed to get existing organization ruleset %s: %w", rulesetPreset.Spec.Name, getErr)
			}
			delete(rulesetsToDelete, rulesetPreset.Spec.Name)
			if mapper.RulesetsDiffer(rulesetPreset, *fullRuleset) {
				rulesetID := fullRuleset.ID
				if rulesetID == nil {
					continue
				}
				if err := o.updateRuleset(ctx, *rulesetID, githubRuleset, rulesetPreset); err != nil {
					return err
				}
			} else {
				log.V(1).Info("Ruleset already matches desired state, skipping")
			}
		} else {
			if err := o.createRuleset(ctx, githubRuleset, rulesetPreset); err != nil {
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
	log.V(1).Info("Updating existing organization ruleset")

	err := o.GitHub.Client.DeleteOrganizationRuleset(ctx, o.GitHub.Resource, rulesetID)
	if err != nil {
		log.Error(err, "failed to delete organization ruleset")
		return fmt.Errorf("failed to delete organization ruleset %s: %w", rulesetName, err)
	}
	log.V(1).Info("Organization ruleset deleted successfully")
	return nil
}

func (o *GitHubOrgReconciler) updateRuleset(ctx context.Context, rulesetID int64, githubRuleset *github.RepositoryRuleset, rulesetPreset v1alpha1.RulesetPreset) error {
	log := logPkg.FromContext(ctx).WithValues("rulesetPreset", rulesetPreset.Name, "RuleSet", githubRuleset.Name)
	log.V(1).Info("Updating existing organization ruleset")

	_, err := o.GitHub.Client.UpdateOrganizationRuleset(ctx, o.GitHub.Resource, rulesetID, githubRuleset)
	if err != nil {
		log.Error(err, "failed to update organization ruleset")
		return fmt.Errorf("failed to update organization ruleset %s: %w", rulesetPreset.Spec.Name, err)
	}
	log.V(1).Info("Organization ruleset updated successfully")
	return nil
}

func (o *GitHubOrgReconciler) createRuleset(ctx context.Context, githubRuleset *github.RepositoryRuleset, rulesetPreset v1alpha1.RulesetPreset) error {
	log := logPkg.FromContext(ctx).WithValues("rulesetPreset", rulesetPreset.Name, "RuleSet", githubRuleset.Name)
	log.V(1).Info("Creating new organization ruleset")
	_, err := o.GitHub.Client.CreateOrganizationRuleset(ctx, o.GitHub.Resource, githubRuleset)
	if err != nil {
		log.Error(err, "failed to create organization ruleset")
		return fmt.Errorf("failed to create organization ruleset %s: %w", rulesetPreset.Spec.Name, err)
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
