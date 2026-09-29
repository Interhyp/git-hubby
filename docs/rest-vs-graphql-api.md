# REST vs. GraphQL: what would change if git-hubby also used GraphQL

git-hubby is currently 100% REST, built on `github.com/google/go-github/v92`. There is no GraphQL
client dependency anywhere in the codebase (no `shurcooL/githubv4`, no hand-rolled GraphQL calls).
That said, `internal/ratelimit` already reserves a `CategoryGraphQL` bucket, and `internal/ghclient`
concentrates every GitHub call behind a single `GitHubClientWrapper`, so the codebase is structurally
ready to add a second transport if it were ever justified.

This document captures the trade-off in both directions:

1. What GraphQL would give us that REST cannot do at all today.
2. What we would lose if we replaced REST with GraphQL instead of adding it alongside.

It also notes where the pain we already have in the codebase (documented in code comments) lines up
with GraphQL's usual strengths, and where our own REST-limitation workarounds are for capabilities
GraphQL doesn't have either — i.e. no API layer would fix them.

## Where we stand today (evidence from the codebase)

All REST calls happen through `internal/ghclient/wrapper.go`, grouped as:

- **Organizations**: profile, custom properties (org-level schema), rulesets, code security
  configurations, organization roles, membership, installations.
- **Repositories**: CRUD, custom property values, topics, autolinks, deploy keys, webhooks,
  rulesets, Actions access level.
- **Teams**: CRUD, membership, external-group (IdP) sync.
- **Actions**: org-level permissions, artifact/log retention, allowed-actions, default workflow
  permissions, self-hosted runner settings, runner groups, per-repo enablement.

Several code comments show REST-specific friction that has already cost engineering effort:

- `internal/webhook/v1alpha1/repository_webhook.go:89` — every repository admission-webhook
  invocation re-fetches the parent org + its custom-property schema from GitHub (a classic N+1
  pattern GraphQL nesting is designed to eliminate).
- `internal/ghclient/wrapper.go:443-460` / `:543-560` — `ListInstallations` and
  `ListEnabledReposInOrg` need a hand-rolled pagination loop because the pagination middleware
  produces EOF errors on those two endpoints.
- `internal/reconciler/orgrec/rec_code_security_configurations.go:220-256` — to compute "all
  public repos" or "all private/internal repos" for a security-configuration scope, the operator
  lists *every* repository in the org via REST and filters by `Visibility` client-side, because
  there's no server-side filtered query.
- `internal/reconciler/orgrec/rec_code_security_configurations.go:258-326` — attaching a code
  security configuration to repositories is asynchronous (`202 Accepted`); the operator polls a
  list endpoint every 5s for up to 2 minutes waiting for per-repo status strings to settle.
- `api/v1alpha1/organization_types.go:260-270` — explicit doc-comment limitation: *"GitHub's API
  does not provide a way to retrieve the current attachment scope type... there is no reliable
  way to determine which repositories should be included"* for the `all_without_configurations`
  scope — worked around by unconditional reattachment.
- `internal/reconciler/orgrec/rec_actions_settings.go:32-39` — Actions settings are reconciled via
  six separate sequential REST calls per organization (permissions, retention, allowed actions,
  default workflow permissions, self-hosted runner settings, runner groups) instead of one
  combined read/write.
- `internal/reconciler/orgrec/rec_actions_settings.go:73-75` — runner groups are matched by name,
  not ID, because the ID is only known after creation, so a rename is treated as delete+recreate
  (losing/reassigning runners).
- `internal/config/config.go:17-23` — the whole "startup spreading" mechanism exists specifically
  to avoid exhausting REST per-category rate limits when many CRs reconcile at once.

These are exactly the shapes of problem GraphQL is usually pitched to solve (N+1 fetches, no
server-side filtering, no combined queries) — see the next section for which of them GraphQL
would actually fix, and which are really about missing *mutations*, not missing *query
flexibility*, and therefore wouldn't be solved by GraphQL either.

## 1. Capabilities GraphQL adds that REST doesn't have at all

These are things the REST API has **no endpoint for**, confirmed against the public GraphQL
schema reference (`docs.github.com/en/graphql/reference`).

| Capability | GraphQL type/mutation | Notes |
|---|---|---|
| Organization/Enterprise **IP allow lists** | `IpAllowListEntry`, `createIpAllowListEntry`, `deleteIpAllowListEntry`, `ipAllowListEnabledSetting`, `ipAllowListForInstalledAppsEnabledSetting` | No REST equivalent exists at all. Natural fit for the `Organization` CRD (`spec.ipAllowList`) if this is ever needed. |
| Classic branch protection **"required deployments before merge"** | `BranchProtectionRule.requiresDeployments` / `requiredDeploymentEnvironments` (in `createBranchProtectionRule`/`updateBranchProtectionRule`) | Only relevant if git-hubby manages classic branch protection rules; the modern Rulesets API (which we use) doesn't have this rule type either, so it's a real gap either way. |
| Repository **Discussions categories** | Discussion category CRUD | REST only has the `has_discussions` on/off toggle; category management (format, emoji, description, answerable flag) is GraphQL-only. |
| **Projects (v2)** | `projectsV2`, `ProjectV2`, related mutations | Projects Classic is REST-and-GraphQL but deprecated (removal 2025-04-01 UTC per docs). The new Projects experience (fields, views, workflows, item linking) has essentially no REST support. |
| **Enterprise-wide policy settings** | `Enterprise`/`EnterpriseOwnerInfo` fields, e.g. `defaultRepositoryPermissionSetting`, `membersCanCreateRepositoriesSetting` (+ public/private/internal variants), `membersCanDeleteRepositoriesSetting`, `membersCanDeleteIssuesSetting`, `membersCanInviteCollaboratorsSetting`, `membersCanChangeRepositoryVisibilitySetting`, `allowPrivateRepositoryForkingSetting`, `announcementBanner` | No REST endpoints for these at all. This is the single biggest capability delta, but it's a new axis for git-hubby — an `Enterprise` CRD, which doesn't exist today. |
| `Repository.codeownersErrors` | read-only query | Validates CODEOWNERS syntax/paths; useful only if git-hubby starts managing/linting CODEOWNERS as code. |

Everything git-hubby currently manages via REST (rulesets, webhooks, custom properties, code
security configurations, teams, Actions settings, deploy keys) is fully covered by REST — none of
it requires GraphQL. Adding GraphQL today would be additive (new capabilities), not a fix for an
existing gap in what we manage.

## 2. Capabilities we would lose by moving to GraphQL-only

This is the more important question for git-hubby specifically, since **most of the operator's
actual REST surface has no GraphQL equivalent for writes**. A GraphQL-only client would be a
regression, not a lateral move. Concretely, mapped to what `internal/ghclient/wrapper.go` uses
today:

| Feature area used by git-hubby | REST support | GraphQL support |
|---|---|---|
| **Repository deploy keys** (`ListKeys`/`CreateKey`/`DeleteKey`) | Yes | **None.** No `DeployKey` mutation exists in the GraphQL schema. |
| **Webhooks** — repo and org (`ListHooks`/`CreateHook`/`DeleteHook`) | Yes | **None.** No mutations to create/list/delete repository or organization webhooks. |
| **GitHub Actions settings** — org-level permissions, allowed-actions, artifact/log retention, default workflow permissions, self-hosted runner settings, runner groups, per-repo enablement | Yes (all of `ActionsSettings` in the `Organization` CRD) | **None.** There is no GraphQL surface for any Actions administration settings. This is roughly half of the `Organization` CRD's functionality. |
| **Code Security Configurations** — dependency graph, Dependabot alerts/updates, code scanning default setup, secret scanning (+push protection, validity checks, delegated bypass), attach-to-repos | Yes (entire `CodeSecurityConfiguration` CRD) | **None.** This whole 2024-era API has no GraphQL equivalent for writes. |
| **Repository Rulesets — writes** (`CreateRuleset`/`UpdateRuleset`/`DeleteRuleset`, org and repo level) | Yes | **Read-only.** GraphQL exposes `Organization.ruleset(s)`/`Repository.rulesets` as *queries*, but there is no `createRepositoryRuleset`/`updateRepositoryRuleset` mutation. This is the biggest CRD in the repo (`RulesetPreset`, ~600 lines) and would be entirely unmanageable via GraphQL alone. |
| **Custom properties — writes** (org-level schema `CreateOrUpdateCustomProperties`, repo-level values) | Yes | **Read-only.** GraphQL exposes `repositoryCustomProperties`/`repositoryCustomProperty` as queries only; no mutation to define or set them. |
| **Repository topics / autolinks** (`ReplaceAllTopics`, `ListAutolinks`/`CreateAutolink`/`DeleteAutolink`) | Yes | No GraphQL mutations for repository autolinks. Topic management via GraphQL is likewise not part of the standard mutation set. |
| **Team ↔ IdP external-group sync** (`ListExternalGroups`, `UpdateConnectedExternalGroup`) | Yes | **None.** SCIM/IdP group connection for teams is REST-only. This is used by the `Team` CRD's IDP-group-sync feature, which is Enterprise-plan-gated functionality git-hubby explicitly supports. |
| **Organization roles → team assignment** (`AssignOrgRoleToTeam`/`RemoveOrgRoleFromTeam`) | Yes | No equivalent GraphQL mutation. |
| **GitHub App installation listing** (`ListInstallations`) | Yes | Not modeled the same way in GraphQL's viewer-centric schema; this is how git-hubby's own auth/installation bookkeeping works. |
| **Organization/repository audit log** | Yes | Being actively **removed**: every `*AuditEntry` type and field in the GraphQL schema is now marked *"The GraphQL audit-log is deprecated. Please use the REST API instead. Removal on 2026-04-01 UTC."* GitHub itself is moving this capability GraphQL → REST, the opposite direction — a GraphQL-only strategy would be actively walking into a deprecation. |

**Net effect:** if git-hubby switched to GraphQL-only, it would lose the ability to manage deploy
keys, webhooks, all of Actions settings, all of Code Security Configurations, and — critically —
**writing** rulesets and custom properties (only reading them would still work). That covers the
majority of what the operator exists to do. GraphQL could still be used for read-heavy paths
(fetching org + repos + custom properties + rulesets in one round trip to reduce the N+1 problem
in the admission webhook, for example) but essentially all *mutations* would still need to go
through REST.

## 3. Secondary considerations

- **Rate limiting model**: REST limits are simple request-count based, tracked per category in
  `internal/ratelimit` today. GraphQL uses a point-based "query cost" model that's harder to
  predict statically (cost depends on requested connection sizes/nesting) and would need its own
  budgeting logic layered on top of the existing `rateLimitTrackerTransport`.
- **Tooling and type safety**: `go-github` gives typed request/response structs matching REST 1:1,
  which is what `GitHubClientWrapper` relies on for compile-time safety and mocking in tests.
  There isn't an equivalently complete, actively maintained typed Go client for GitHub's GraphQL
  mutations covering the areas above — using GraphQL would mean hand-written query/mutation
  strings (fragile, string-typed) for a large fraction of new code, increasing maintenance risk in
  a codebase that currently keeps its entire GitHub surface behind one interface.
- **Two auth/transport stacks**: adding GraphQL alongside REST (rather than replacing it) means
  maintaining two HTTP clients, two token/auth code paths (though both can share the same GitHub
  App installation token), and two rate-limit categories to reconcile against the existing
  `RateLimitedError`/stall-and-requeue logic.

## Conclusion

- **GraphQL-in-addition-to-REST** is worth it only for the genuinely REST-less features above (IP
  allow lists, Discussions categories, Projects v2, and — if git-hubby ever gets an `Enterprise`
  CRD — the large set of enterprise-wide policy settings). None of it is needed for what
  git-hubby manages today.
- **GraphQL-instead-of-REST** is not viable: git-hubby's core value (rulesets, webhooks, Actions
  settings, code security configurations, deploy keys, custom properties, IdP-synced teams) is
  either REST-only or REST-write/GraphQL-read-only. A GraphQL-only client would regress the
  operator to a fraction of its current functionality.
