package reconciler

import (
	"context"
	"fmt"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/ghclient"
	"github.com/google/go-github/v89/github"
)

// GitHubIDResolver resolves GitHub slug/name references to numeric IDs during reconciliation.
//
// Use NewGitHubIDResolver to create a production instance: it pre-loads all GitHub App
// installations, org teams, and org roles with three bulk API calls, so that every
// subsequent resolution is a lock-free in-memory map lookup with no further API calls.
type GitHubIDResolver struct {
	client  ghclient.GitHubClient
	orgName string

	// Populated at construction time. Read-only after creation — safe for concurrent use.
	appsBySlug  map[string]*int64
	teamsBySlug map[string]int64
	rolesByName map[string]int64
}

func NewGitHubIDResolver(ctx context.Context, client ghclient.GitHubClient, orgName string) (*GitHubIDResolver, error) {
	r := &GitHubIDResolver{client: client, orgName: orgName}
	if err := r.loadLookUpMaps(ctx); err != nil {
		return nil, err
	}
	return r, nil
}

// loadLookUpMaps fetches teams, app installations, and org roles from GitHub and populates
// the resolver's internal lookup maps. It uses a cache-enabled context so that
// concurrent reconciliations for the same org share cached results.
func (r *GitHubIDResolver) loadLookUpMaps(ctx context.Context) error {
	cacheCtx := ghclient.WithCache(ctx)

	apps, err := r.client.GetGitHubAppsInstallations(cacheCtx, r.orgName)
	if err != nil {
		return fmt.Errorf("failed to load GitHub App installations for org %s: %w", r.orgName, err)
	}
	appsBySlug := make(map[string]*int64, len(apps))
	for _, inst := range apps {
		appsBySlug[inst.GetAppSlug()] = inst.AppID
	}

	teams, err := r.client.GetAllTeamsForOrg(cacheCtx, r.orgName)
	if err != nil {
		return fmt.Errorf("failed to load teams for org %s: %w", r.orgName, err)
	}
	teamsBySlug := make(map[string]int64, len(teams))
	for _, t := range teams {
		if t.Slug != nil {
			teamsBySlug[*t.Slug] = t.GetID()
		}
	}

	roles, err := r.client.GetAllOrgRoles(cacheCtx, r.orgName)
	if err != nil {
		return fmt.Errorf("failed to load org roles for org %s: %w", r.orgName, err)
	}
	rolesByName := make(map[string]int64, len(roles))
	for _, role := range roles {
		if role.Name != nil {
			rolesByName[*role.Name] = role.GetID()
		}
	}

	r.appsBySlug = appsBySlug
	r.teamsBySlug = teamsBySlug
	r.rolesByName = rolesByName
	return nil
}

// refreshMaps invalidates the client's response cache and re-populates the resolver's
// internal maps with fresh data from the API. Returns true if the refresh succeeded.
// This is called automatically when a slug/name lookup fails, handling the case where
// a newly created resource is not yet in the cached data.
func (r *GitHubIDResolver) refreshMaps(ctx context.Context) bool {
	if r.client == nil {
		return false
	}
	ghclient.InvalidateCache(r.client)
	return r.loadLookUpMaps(ctx) == nil
}

// resolveWorkflowRepositoryNames resolves RepositoryName fields in workflow rules to
// their numeric IDs. This requires a live API call per unresolved repository and is
// intentionally kept separate from GitHubIDResolver so that GitHubIDResolver remains a
// pure map-lookup type with no I/O dependency.
func (r *GitHubIDResolver) resolveWorkflowRepositoryNames(ctx context.Context, rs v1alpha1.RulesetPreset) (v1alpha1.RulesetPreset, error) {
	if rs.Spec.Rules.Workflows == nil {
		return rs, nil
	}
	for i, wf := range rs.Spec.Rules.Workflows.Workflows {
		if wf.ResolvedRepositoryID != nil {
			continue
		}
		repo, err := r.client.GetRepository(ctx, r.orgName, wf.RepositoryName)
		if err != nil {
			return rs, fmt.Errorf("failed to resolve workflow repository %q to ID: %w", wf.RepositoryName, err)
		}
		rs.Spec.Rules.Workflows.Workflows[i].ResolvedRepositoryID = repo.ID
	}
	return rs, nil
}

// ResolveRuleset resolves all slug/name references inside rs to numeric IDs.
// If a lookup fails (e.g., a newly created team not yet in cache), the resolver
// invalidates the client cache, refreshes its maps, and retries once.
func (r *GitHubIDResolver) ResolveRuleset(ctx context.Context, rs v1alpha1.RulesetPreset) (v1alpha1.RulesetPreset, error) {
	result, err := r.doResolveRuleset(ctx, rs)
	if err != nil {
		if r.refreshMaps(ctx) {
			return r.doResolveRuleset(ctx, rs)
		}
	}
	return result, err
}

func (r *GitHubIDResolver) doResolveRuleset(ctx context.Context, rs v1alpha1.RulesetPreset) (v1alpha1.RulesetPreset, error) {
	var err error

	rs, err = r.resolveBypassActors(rs)
	if err != nil {
		return rs, err
	}

	rs, err = r.resolveWorkflowRepositoryNames(ctx, rs)
	if err != nil {
		return rs, err
	}

	rs = r.resolveStatusCheckAppSlugs(rs)

	rs, err = r.resolveRequiredReviewerSlugs(rs)
	if err != nil {
		return rs, err
	}

	return rs, nil
}

// ResolveCscBypassReviewers resolves ReviewerName fields in the BypassReviewers of a
// CodeSecurityConfiguration to ReviewerId values.
// If a lookup fails, the resolver invalidates the client cache, refreshes its maps, and retries once.
func (r *GitHubIDResolver) ResolveCscBypassReviewers(ctx context.Context, csc *v1alpha1.CodeSecurityConfiguration) (*v1alpha1.CodeSecurityConfiguration, error) {
	result, err := r.doResolveCscBypassReviewers(csc)
	if err != nil {
		if r.refreshMaps(ctx) {
			return r.doResolveCscBypassReviewers(csc)
		}
	}
	return result, err
}

func (r *GitHubIDResolver) doResolveCscBypassReviewers(csc *v1alpha1.CodeSecurityConfiguration) (*v1alpha1.CodeSecurityConfiguration, error) {
	if csc.Spec.SecretScanningDelegatedBypassOptions == nil {
		return csc, nil
	}
	updated := make([]*v1alpha1.BypassReviewer, len(csc.Spec.SecretScanningDelegatedBypassOptions.Reviewers))
	for i, reviewer := range csc.Spec.SecretScanningDelegatedBypassOptions.Reviewers {
		if reviewer.ReviewerName != nil {
			switch reviewer.ReviewerType {
			case "TEAM":
				id, err := r.ResolveTeamSlug(*reviewer.ReviewerName)
				if err != nil {
					return nil, err
				}
				reviewer.ReviewerId = &id
			case "ROLE":
				id, err := r.ResolveRoleName(*reviewer.ReviewerName)
				if err != nil {
					return nil, err
				}
				reviewer.ReviewerId = &id
			}
		}
		updated[i] = reviewer
	}
	csc.Spec.SecretScanningDelegatedBypassOptions.Reviewers = updated
	return csc, nil
}

// ResolveTeamSlug resolves a team slug to its numeric ID from the pre-loaded map.
func (r *GitHubIDResolver) ResolveTeamSlug(slug string) (int64, error) {
	id, ok := r.teamsBySlug[slug]
	if !ok {
		return 0, fmt.Errorf("team with slug %q not found in org %s", slug, r.orgName)
	}
	return id, nil
}

// ResolveRoleName resolves an org role name to its numeric ID from the pre-loaded map.
func (r *GitHubIDResolver) ResolveRoleName(name string) (int64, error) {
	id, ok := r.rolesByName[name]
	if !ok {
		return 0, fmt.Errorf("org role with name %q not found in org %s", name, r.orgName)
	}
	return id, nil
}

// resolveAppSlug resolves a GitHub App slug to its AppID from the pre-loaded map.
// Returns nil, nil if not found (nil IntegrationID = accept any check source).
func (r *GitHubIDResolver) resolveAppSlug(slug string) (*int64, error) {
	id, ok := r.appsBySlug[slug]
	if !ok {
		return nil, fmt.Errorf("no GitHub App with slug %s installed on org %s", slug, r.orgName)
	}
	return id, nil
}

func (r *GitHubIDResolver) resolveBypassActors(rs v1alpha1.RulesetPreset) (v1alpha1.RulesetPreset, error) {
	result := make([]v1alpha1.RulesetBypassActor, 0, len(rs.Spec.BypassActors))
	for _, bypassActor := range rs.Spec.BypassActors {
		actorType := github.BypassActorType(bypassActor.ActorType)
		var err error
		switch actorType {
		case github.BypassActorTypeTeam:
			var updated *v1alpha1.RulesetBypassActor
			updated, err = resolveBypassActorWith(&bypassActor, func(slug string) (*int64, error) {
				id, resolveErr := r.ResolveTeamSlug(slug)
				if resolveErr != nil {
					return nil, resolveErr
				}
				return &id, nil
			})
			if err != nil {
				return rs, err
			}
			bypassActor = *updated
		case github.BypassActorTypeIntegration:
			var updated *v1alpha1.RulesetBypassActor
			updated, err = resolveBypassActorWith(&bypassActor, r.resolveAppSlug)
			if err != nil {
				return rs, err
			}
			bypassActor = *updated
		case github.BypassActorTypeRepositoryRole:
			var updated *v1alpha1.RulesetBypassActor
			updated, err = resolveBypassActorWith(&bypassActor, func(slug string) (*int64, error) {
				id, resolveErr := r.ResolveRoleName(slug)
				if resolveErr != nil {
					return nil, resolveErr
				}
				return &id, nil
			})
			if err != nil {
				return rs, err
			}
			bypassActor = *updated
		case github.BypassActorTypeDeployKey:
			bypassActor.ActorID = nil
			bypassActor.ActorSlug = nil
		}
		result = append(result, bypassActor)
	}
	rs.Spec.BypassActors = result
	return rs, nil
}

// resolveStatusCheckAppSlugs resolves AppSlug references in required-status-check rules.
// Errors are silently ignored: a nil IntegrationID means "accept any check source".
func (r *GitHubIDResolver) resolveStatusCheckAppSlugs(rs v1alpha1.RulesetPreset) v1alpha1.RulesetPreset {
	if rs.Spec.Rules.RequiredStatusChecks == nil {
		return rs
	}
	newChecks := make([]v1alpha1.StatusCheck, len(rs.Spec.Rules.RequiredStatusChecks.Checks))
	for i, check := range rs.Spec.Rules.RequiredStatusChecks.Checks {
		if check.AppSlug != nil {
			// Ignore error: nil IntegrationID equals accepting any check source.
			check.IntegrationID, _ = r.resolveAppSlug(*check.AppSlug)
		}
		newChecks[i] = check
	}
	rs.Spec.Rules.RequiredStatusChecks.Checks = newChecks
	return rs
}

func (r *GitHubIDResolver) resolveRequiredReviewerSlugs(rs v1alpha1.RulesetPreset) (v1alpha1.RulesetPreset, error) {
	if rs.Spec.Rules.PullRequest == nil {
		return rs, nil
	}
	updated := make([]v1alpha1.RequiredPullRequestReviewer, len(rs.Spec.Rules.PullRequest.RequiredReviewers))
	for i, reviewer := range rs.Spec.Rules.PullRequest.RequiredReviewers {
		if reviewer.Reviewer.ID == nil && reviewer.Reviewer.Slug != nil {
			reviewerType := github.RulesetReviewerType(reviewer.Reviewer.Type)
			switch reviewerType {
			case github.RulesetReviewerTypeTeam:
				id, err := r.ResolveTeamSlug(*reviewer.Reviewer.Slug)
				if err != nil {
					return rs, fmt.Errorf("failed to resolve required reviewer slug %q to ID: %w", *reviewer.Reviewer.Slug, err)
				}
				reviewer.Reviewer.ID = &id
			default:
				return rs, fmt.Errorf("unsupported required reviewer type %q for slug resolution", reviewer.Reviewer.Type)
			}
		}
		updated[i] = reviewer
	}
	rs.Spec.Rules.PullRequest.RequiredReviewers = updated
	return rs, nil
}

// resolveBypassActorWith resolves the ActorID of a bypass actor using the provided resolver.
func resolveBypassActorWith(actor *v1alpha1.RulesetBypassActor, resolver func(slug string) (*int64, error)) (*v1alpha1.RulesetBypassActor, error) {
	if actor == nil {
		return nil, fmt.Errorf("unable to resolve nil bypass actor")
	}
	if actor.ActorID != nil {
		return actor, nil
	}
	if actor.ActorSlug == nil {
		return nil, fmt.Errorf("bypass actor with type %s requires either actor_id or actor_slug to be set", actor.ActorType)
	}
	id, err := resolver(*actor.ActorSlug)
	if err != nil {
		return nil, err
	}
	actor.ActorID = id
	return actor, nil
}
