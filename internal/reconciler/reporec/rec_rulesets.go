package reporec

import (
	"context"
	"fmt"

	githubv1alpha1 "github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/reconciler/common/rulesets"
	"github.com/google/go-github/v92/github"
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

	// Collect desired rulesets before any GitHub call so that conflicts prevent all ruleset changes.
	desiredRulesets, err := rulesets.Collect(ctx, r.Kubernetes.Client, r.Kubernetes.Resource.Namespace,
		r.Kubernetes.Resource.Spec.RulesetPresetList, r.Kubernetes.Resource.Spec.Rulesets)
	if err != nil {
		return err
	}

	owner, name := r.GitHub.Resource.Owner, r.GitHub.Resource.Name
	target := rulesets.Target{
		Scope: "repository",
		List: func(ctx context.Context) ([]*github.RepositoryRuleset, error) {
			return r.GitHub.Client.GetAllRepositoryRulesets(ctx, owner, name, false)
		},
		Get: func(ctx context.Context, id int64) (*github.RepositoryRuleset, error) {
			return r.GitHub.Client.GetRepositoryRuleset(ctx, owner, name, id, false)
		},
		Create: r.createRuleset,
		Update: r.updateRuleset,
		Delete: r.deleteRuleset,
	}
	opts := rulesets.SyncOptions{
		Resolver:                     r.NameResolver,
		EnableRequiredReviewersRules: r.Features.EnableRequiredReviewersRules,
		Skip: func(ctx context.Context, desired rulesets.Desired) bool {
			if desired.Spec.Target != "repository" {
				return false
			}
			// target "repository" is only allowed for Organization-level rulesets. We skip these here as documented in the API.
			logPkg.FromContext(ctx).V(1).Info("Skipping ruleset with target 'repository' as it is only supported at organization level",
				"rulesetSource", desired.Source, "ruleset", desired.Spec.Name)
			return true
		},
	}
	if err := rulesets.Sync(ctx, target, desiredRulesets, opts); err != nil {
		return err
	}

	log.V(1).Info("Successfully reconciled repository rule sets on GitHub")
	return nil
}

func (r *GitHubRepoReconciler) updateRuleset(ctx context.Context, rulesetID int64, githubRuleset *github.RepositoryRuleset, desired rulesets.Desired) error {
	log := logPkg.FromContext(ctx).WithValues("rulesetSource", desired.Source, "ruleSet", githubRuleset.Name)
	log.V(1).Info("Updating existing ruleset")
	_, err := r.GitHub.Client.UpdateRepositoryRuleset(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, rulesetID, githubRuleset)
	if err != nil {
		log.Error(err, "failed to update repository ruleset")
		return fmt.Errorf("failed to update repository ruleset %s (%s): %w", desired.Spec.Name, desired.Source, err)
	}
	log.V(1).Info("Ruleset updated successfully")
	return nil
}

func (r *GitHubRepoReconciler) createRuleset(ctx context.Context, githubRuleset *github.RepositoryRuleset, desired rulesets.Desired) error {
	log := logPkg.FromContext(ctx).WithValues("rulesetSource", desired.Source, "ruleSet", githubRuleset.Name)
	log.V(1).Info("Creating new ruleset")
	_, err := r.GitHub.Client.CreateRepositoryRuleset(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, githubRuleset)
	if err != nil {
		log.Error(err, "failed to create repository ruleset")
		return fmt.Errorf("failed to create repository ruleset %s (%s): %w", desired.Spec.Name, desired.Source, err)
	}
	log.V(1).Info("Ruleset created successfully")
	return nil
}

func (r *GitHubRepoReconciler) deleteRuleset(ctx context.Context, rulesetID int64, rulesetName string) error {
	log := logPkg.FromContext(ctx).WithValues("ruleset", rulesetName)
	log.V(1).Info("Deleting unused ruleset")
	if err := r.GitHub.Client.DeleteRepositoryRuleset(ctx, r.GitHub.Resource.Owner, r.GitHub.Resource.Name, rulesetID); err != nil {
		log.Error(err, "failed to delete repository ruleset")
		return fmt.Errorf("failed to delete repository ruleset %s: %w", rulesetName, err)
	}
	log.V(1).Info("Ruleset deleted successfully")
	return nil
}
