package reporec

import (
	"context"
	"fmt"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/mapper"
	"github.com/Interhyp/git-hubby/internal/reconciler"
	"github.com/google/go-github/v89/github"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *GitHubRepoReconciler) reconcileRuleSets(ctx context.Context) error {
	log := logPkg.FromContext(ctx)
	log.V(1).Info("Reconciling repository rule sets on GitHub")

	// Rulesets are not supported on non-public repositories in the free plan
	var org githubv1alpha1.Organization
	if err := r.Kubernetes.Client.Get(ctx, client.ObjectKey{
		Name:      r.Kubernetes.Resource.Spec.OrganizationRef.Name,
		Namespace: r.Kubernetes.Resource.Namespace,
	}, &org); err != nil {
		log.Error(err, "unable to fetch Organization for Repository")
		return err
	}

	if org.GetPlan() == githubv1alpha1.PlanFree && r.Kubernetes.Resource.Spec.Visibility != githubv1alpha1.VisibilityPublic {
		log.V(1).Info("Skipping rulesets reconciliation for non-public repository on free plan")
		return nil
	}

	existingRulesets, err := r.GitHub.Client.GetAllRepositoryRulesets(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, false)
	if err != nil {
		log.Error(err, "failed to get existing repository rulesets")
		return fmt.Errorf("failed to get existing repository rulesets: %w", err)
	}

	rulesetsToDelete := make(map[string]*github.RepositoryRuleset)
	existingRulesetsByName := make(map[string]*github.RepositoryRuleset)
	for _, ruleset := range existingRulesets {
		if ruleset.Name != "" && ruleset.ID != nil {
			existingRulesetsByName[ruleset.Name] = ruleset
			rulesetsToDelete[ruleset.Name] = ruleset
		}
	}

	for _, rulesetRef := range r.Kubernetes.Resource.Spec.RulesetPresetList {
		log := log.WithValues("rulesetPreset", rulesetRef.Name)
		log.V(1).Info("Processing ruleset preset")

		var rulesetPreset githubv1alpha1.RulesetPreset
		if err := r.Kubernetes.Client.Get(ctx, client.ObjectKey{
			Name:      rulesetRef.Name,
			Namespace: r.Kubernetes.Resource.Namespace,
		}, &rulesetPreset); err != nil {
			log.Error(err, "unable to get ruleset preset")
			return fmt.Errorf("failed to get ruleset preset %s: %w", rulesetRef.Name, err)
		}

		if rulesetPreset.Spec.Target == "repository" {
			// target "repository" is only allowed for Organization-level rulesets. We skip these here as documented in the API.
			continue
		}

		rulesetPreset, err := reconciler.ResolveNamesToIDsInRuleset(ctx, r.GitHub.Client, r.GitHub.Resource.Owner, rulesetPreset)
		if err != nil {
			log.Error(err, "failed to resolve ruleset slugs to IDs")
			return fmt.Errorf("failed to resolve slugs in ruleset %s to IDs: %w", rulesetRef.Name, err)
		}

		githubRuleset, err := mapper.RulesetPresetToGithubRuleset(rulesetPreset)
		if err != nil {
			log.Error(err, "failed to convert ruleset preset to GitHub ruleset")
			return fmt.Errorf("failed to convert ruleset preset %s to GitHub ruleset: %w", rulesetRef.Name, err)
		}

		// Check if the ruleset already exists
		if existingRuleset, exists := existingRulesetsByName[rulesetPreset.Spec.Name]; exists {
			// Skip if the existing ruleset has no ID
			if existingRuleset.ID == nil {
				log.Info("WARNING: Existing ruleset has nil ID, skipping")
				continue
			}
			fullRuleset, getErr := r.GitHub.Client.GetRepositoryRuleset(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, *existingRuleset.ID, false)
			if getErr != nil {
				log.Error(getErr, "failed to get existing repository ruleset")
				return fmt.Errorf("failed to get existing repository ruleset %s: %w", rulesetPreset.Spec.Name, getErr)
			}
			delete(rulesetsToDelete, rulesetPreset.Spec.Name)
			if mapper.RulesetsDiffer(rulesetPreset, *fullRuleset) {
				rulesetID := fullRuleset.ID
				if rulesetID == nil {
					continue
				}
				if err := r.updateRuleset(ctx, *rulesetID, githubRuleset, rulesetPreset); err != nil {
					return err
				}
			} else {
				log.V(1).Info("Ruleset already matches desired state, skipping")
			}
		} else {
			if err := r.createRuleset(ctx, githubRuleset, rulesetPreset); err != nil {
				return err
			}
		}
	}

	for _, ruleset := range rulesetsToDelete {
		if err := r.deleteRuleset(ctx, ruleset); err != nil {
			return err
		}
	}

	log.V(1).Info("Successfully reconciled repository rule sets on GitHub")
	return nil
}

func (r *GitHubRepoReconciler) updateRuleset(ctx context.Context, rulesetID int64, githubRuleset *github.RepositoryRuleset, rulesetPreset githubv1alpha1.RulesetPreset) error {
	log := logPkg.FromContext(ctx).WithValues("rulesetPreset", rulesetPreset.Name, "ruleSet", githubRuleset.Name)
	log.V(1).Info("Updating existing ruleset")
	_, err := r.GitHub.Client.UpdateRepositoryRuleset(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, rulesetID, githubRuleset)
	if err != nil {
		log.Error(err, "failed to update repository ruleset")
		return fmt.Errorf("failed to update repository ruleset %s: %w", rulesetPreset.Spec.Name, err)
	}
	log.V(1).Info("Ruleset updated successfully")
	return nil
}

func (r *GitHubRepoReconciler) createRuleset(ctx context.Context, githubRuleset *github.RepositoryRuleset, rulesetPreset githubv1alpha1.RulesetPreset) error {
	log := logPkg.FromContext(ctx).WithValues("rulesetPreset", rulesetPreset.Name, "ruleSet", githubRuleset.Name)
	log.V(1).Info("Creating new ruleset")
	_, err := r.GitHub.Client.CreateRepositoryRuleset(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, githubRuleset)
	if err != nil {
		log.Error(err, "failed to create repository ruleset")
		return fmt.Errorf("failed to create repository ruleset %s: %w", rulesetPreset.Spec.Name, err)
	}
	log.V(1).Info("Ruleset created successfully")
	return nil
}

func (r *GitHubRepoReconciler) deleteRuleset(ctx context.Context, ruleset *github.RepositoryRuleset) error {
	log := logPkg.FromContext(ctx).WithValues("ruleset", ruleset.Name)
	log.V(1).Info("Deleting unused ruleset")
	if err := r.GitHub.Client.DeleteRepositoryRuleset(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, *ruleset.ID); err != nil {
		log.Error(err, "failed to delete repository ruleset")
		return fmt.Errorf("failed to delete repository ruleset %s: %w", ruleset.Name, err)
	}
	log.V(1).Info("Ruleset deleted successfully")
	return nil
}
