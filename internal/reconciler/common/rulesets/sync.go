package rulesets

import (
	"context"
	"errors"
	"fmt"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/mapper"
	"github.com/google/go-github/v92/github"
	logPkg "sigs.k8s.io/controller-runtime/pkg/log"
)

// Resolver resolves slug/name references inside a ruleset to numeric GitHub IDs.
type Resolver interface {
	ResolveRuleset(ctx context.Context, rs v1alpha1.RulesetSpec) (v1alpha1.RulesetSpec, error)
}

// Target abstracts the GitHub ruleset API of a ruleset owner (organization or repository).
type Target struct {
	// Scope is used in log and error messages, e.g. "organization" or "repository".
	Scope  string
	List   func(ctx context.Context) ([]*github.RepositoryRuleset, error)
	Get    func(ctx context.Context, id int64) (*github.RepositoryRuleset, error)
	Create func(ctx context.Context, githubRuleset *github.RepositoryRuleset, desired Desired) error
	Update func(ctx context.Context, id int64, githubRuleset *github.RepositoryRuleset, desired Desired) error
	Delete func(ctx context.Context, id int64, name string) error
}

// SyncOptions controls how desired rulesets are prepared before being applied.
type SyncOptions struct {
	Resolver                     Resolver
	EnableRequiredReviewersRules bool
	// Skip optionally excludes a desired ruleset from being applied (it is treated as not desired, i.e. an
	// existing ruleset with the same name will be deleted).
	Skip func(ctx context.Context, desired Desired) bool
	// Adjust optionally modifies a desired ruleset after slug resolution and before mapping it to GitHub.
	Adjust func(spec *v1alpha1.RulesetSpec)
}

// Sync applies the desired rulesets to the given target: missing rulesets are created, differing rulesets
// are updated and existing rulesets that are not desired are deleted.
// Desired rulesets with Unresolved set are left untouched on GitHub (in particular, they are not deleted) while all
// other rulesets are synced; their UnresolvedErrors are returned afterwards.
func Sync(ctx context.Context, target Target, desiredRulesets []Desired, opts SyncOptions) error {
	log := logPkg.FromContext(ctx)
	existingRulesets, err := target.List(ctx)
	if err != nil {
		log.Error(err, "failed to get existing rulesets", "scope", target.Scope)
		return fmt.Errorf("failed to get existing %s rulesets: %w", target.Scope, err)
	}
	rulesetsToDelete := make(map[string]*github.RepositoryRuleset)
	existingRulesetsByName := make(map[string]*github.RepositoryRuleset)
	for _, ruleset := range existingRulesets {
		if ruleset == nil || ruleset.Name == "" {
			continue
		}
		existingRulesetsByName[ruleset.Name] = ruleset
		rulesetsToDelete[ruleset.Name] = ruleset
	}
	var unresolved []error
	for _, desired := range desiredRulesets {
		log := log.WithValues("rulesetSource", desired.Source, "ruleset", desired.Spec.Name)
		if desired.Unresolved != nil {
			log.Info("Keeping existing ruleset unchanged because its extends chain could not be resolved",
				"scope", target.Scope, "reason", desired.Unresolved.Reason)
			delete(rulesetsToDelete, desired.Spec.Name)
			unresolved = append(unresolved, desired.Unresolved)
			continue
		}
		log.V(1).Info("Processing ruleset")
		if opts.Skip != nil && opts.Skip(ctx, desired) {
			continue
		}
		// Strip spec fields for disabled beta features before any processing.
		if !opts.EnableRequiredReviewersRules && desired.Spec.Rules.PullRequest != nil {
			desired.Spec.Rules.PullRequest.RequiredReviewers = nil
		}
		desired.Spec, err = opts.Resolver.ResolveRuleset(ctx, desired.Spec)
		if err != nil {
			log.Error(err, "failed to resolve ruleset slugs to IDs")
			return fmt.Errorf("failed to resolve slugs of ruleset %s (%s) to IDs: %w", desired.Spec.Name, desired.Source, err)
		}
		if opts.Adjust != nil {
			opts.Adjust(&desired.Spec)
		}
		githubRuleset, err := mapper.RulesetSpecToGithubRuleset(desired.Spec)
		if err != nil {
			log.Error(err, "failed to convert ruleset to GitHub ruleset")
			return fmt.Errorf("failed to convert ruleset %s (%s) to GitHub ruleset: %w", desired.Spec.Name, desired.Source, err)
		}
		existingRuleset, exists := existingRulesetsByName[desired.Spec.Name]
		if !exists {
			if err := target.Create(ctx, githubRuleset, desired); err != nil {
				return err
			}
			continue
		}
		if existingRuleset.ID == nil {
			log.Info("WARNING: Existing ruleset has nil ID, skipping")
			continue
		}
		fullRuleset, err := target.Get(ctx, *existingRuleset.ID)
		if err != nil {
			log.Error(err, "failed to get existing ruleset", "scope", target.Scope)
			return fmt.Errorf("failed to get existing %s ruleset %s: %w", target.Scope, desired.Spec.Name, err)
		}
		delete(rulesetsToDelete, desired.Spec.Name)
		if !mapper.RulesetsDiffer(desired.Spec, *fullRuleset) {
			log.V(1).Info("Ruleset already matches desired state, skipping")
			continue
		}
		if fullRuleset.ID == nil {
			continue
		}
		if err := target.Update(ctx, *fullRuleset.ID, githubRuleset, desired); err != nil {
			return err
		}
	}
	for name, ruleset := range rulesetsToDelete {
		if ruleset.ID == nil {
			continue
		}
		log.V(1).Info("Deleting orphaned ruleset", "scope", target.Scope, "ruleset", name)
		if err := target.Delete(ctx, *ruleset.ID, name); err != nil {
			return err
		}
	}
	return errors.Join(unresolved...)
}
