package reconciler

import (
	"context"
	"fmt"

	"github.com/Interhyp/git-hubby/api/v1alpha1"
	"github.com/Interhyp/git-hubby/internal/ghclient"
	"github.com/google/go-github/v86/github"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v2 "k8s.io/client-go/applyconfigurations/meta/v1"
)

func IsActionsDisabledForOrgSpec(org *v1alpha1.Organization) bool {
	return org.Spec.ActionsSettings.EnabledRepositories == nil ||
		*org.Spec.ActionsSettings.EnabledRepositories == "none"
}

func ResolveNamesToIDsInRuleset(ctx context.Context, client ghclient.GitHubClient, orgName string, rs v1alpha1.RulesetPreset) (v1alpha1.RulesetPreset, error) {
	installations, err := client.GetGitHubAppsInstallations(ctx, orgName) // fetch installations only once
	if err != nil {
		return rs, err
	}

	rs, err = resolveBypassActors(ctx, client, orgName, installations, rs)
	if err != nil {
		return rs, err
	}

	rs, err = resolveWorkflowRepositoryNames(ctx, client, orgName, rs)
	if err != nil {
		return rs, err
	}

	rs = resolveStatusCheckAppSlugs(orgName, installations, rs)

	return rs, nil
}

// resolveBypassActors resolves slugs/names in bypass actors to their numeric IDs.
func resolveBypassActors(ctx context.Context, client ghclient.GitHubClient, orgName string, installations []*github.Installation, rs v1alpha1.RulesetPreset) (v1alpha1.RulesetPreset, error) {
	result := make([]v1alpha1.RulesetBypassActor, 0, len(rs.Spec.BypassActors))
	for _, bypassActor := range rs.Spec.BypassActors {
		actorType := github.BypassActorType(bypassActor.ActorType)
		switch actorType {
		case github.BypassActorTypeTeam:
			updatedActor, err := resolveBypassActor(&bypassActor, teamSlugResolver(ctx, client, orgName))
			if err != nil {
				return rs, err
			}
			bypassActor = *updatedActor
		case github.BypassActorTypeIntegration:
			updatedActor, err := resolveBypassActor(&bypassActor, appSlugResolver(installations, orgName))
			if err != nil {
				return rs, err
			}
			bypassActor = *updatedActor
		case github.BypassActorTypeRepositoryRole:
			updatedActor, err := resolveBypassActor(&bypassActor, repoRoleNameResolver(ctx, client, orgName))
			if err != nil {
				return rs, err
			}
			bypassActor = *updatedActor
		case github.BypassActorTypeDeployKey:
			// actor_id: If actor_type is DeployKey, this should be null.
			// for "EnterpriseOwner" and "OrganizationAdmin" it is ignored
			// see https://docs.github.com/en/enterprise-cloud@latest/rest/repos/rules?apiVersion=2026-03-10#create-a-repository-ruleset
			bypassActor.ActorID = nil
			bypassActor.ActorSlug = nil
		}
		result = append(result, bypassActor)
	}
	rs.Spec.BypassActors = result
	return rs, nil
}

// resolveWorkflowRepositoryNames resolves RepositoryName to ResolvedRepositoryID for each workflow rule in the ruleset.
func resolveWorkflowRepositoryNames(ctx context.Context, client ghclient.GitHubClient, orgName string, rs v1alpha1.RulesetPreset) (v1alpha1.RulesetPreset, error) {
	if rs.Spec.Rules.Workflows == nil {
		return rs, nil
	}
	for i, wf := range rs.Spec.Rules.Workflows.Workflows {
		if wf.ResolvedRepositoryID != nil {
			continue
		}
		repo, err := client.GetRepository(ctx, orgName, wf.RepositoryName)
		if err != nil {
			return rs, fmt.Errorf("failed to resolve workflow repository %q to ID: %w", wf.RepositoryName, err)
		}
		rs.Spec.Rules.Workflows.Workflows[i].ResolvedRepositoryID = repo.ID
	}
	return rs, nil
}

// resolveStatusCheckAppSlugs resolves app slugs in required status checks to their integration IDs.
func resolveStatusCheckAppSlugs(orgName string, installations []*github.Installation, rs v1alpha1.RulesetPreset) v1alpha1.RulesetPreset {
	if rs.Spec.Rules.RequiredStatusChecks == nil {
		return rs
	}
	newChecks := make([]v1alpha1.StatusCheck, len(rs.Spec.Rules.RequiredStatusChecks.Checks))
	for i, check := range rs.Spec.Rules.RequiredStatusChecks.Checks {
		if check.AppSlug != nil {
			// sets integrationID to nil in case of err, which equals accepting any check source
			check.IntegrationID, _ = appSlugResolver(installations, orgName)(*check.AppSlug)
		}
		newChecks[i] = check
	}
	rs.Spec.Rules.RequiredStatusChecks.Checks = newChecks
	return rs
}

type slugResolverFunc = func(slug string) (*int64, error)

func resolveBypassActor(actor *v1alpha1.RulesetBypassActor, resolver slugResolverFunc) (*v1alpha1.RulesetBypassActor, error) {
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

func appSlugResolver(installations []*github.Installation, orgName string) slugResolverFunc {
	return func(slug string) (*int64, error) {
		var appID *int64

		for _, installation := range installations {
			if installation.GetAppSlug() == slug {
				appID = installation.AppID
				break
			}
		}

		if appID == nil {
			return nil, fmt.Errorf("failed to resolve ruleset bypass actor: no GitHub App with slug %s installed on organization %s", slug, orgName)
		}

		return appID, nil
	}
}

func teamSlugResolver(ctx context.Context, client ghclient.GitHubClient, orgName string) slugResolverFunc {
	return func(slug string) (*int64, error) {
		team, err := client.GetTeamBySlug(ctx, orgName, slug)
		if err != nil {
			return nil, err
		}
		return team.ID, nil
	}
}
func repoRoleNameResolver(ctx context.Context, client ghclient.GitHubClient, orgName string) slugResolverFunc {
	return func(slug string) (*int64, error) {
		role, err := client.GetRoleByName(ctx, orgName, slug)
		if err != nil {
			return nil, err
		}
		return role.ID, nil
	}
}

// ConditionToApplyConfig converts a metav1.Condition to a ConditionApplyConfiguration for SSA.
func ConditionToApplyConfig(c v1.Condition) *v2.ConditionApplyConfiguration {
	return v2.Condition().
		WithType(c.Type).
		WithStatus(c.Status).
		WithObservedGeneration(c.ObservedGeneration).
		WithLastTransitionTime(c.LastTransitionTime).
		WithReason(c.Reason).
		WithMessage(c.Message)
}

// ConditionsToApplyConfigs converts a slice of metav1.Condition to apply configurations.
func ConditionsToApplyConfigs(conditions []v1.Condition) []*v2.ConditionApplyConfiguration {
	result := make([]*v2.ConditionApplyConfiguration, len(conditions))
	for i, c := range conditions {
		result[i] = ConditionToApplyConfig(c)
	}
	return result
}
