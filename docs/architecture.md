# Architecture

This document describes the internal architecture of the git-hubby operator.

## Overview

git-hubby follows the standard [Kubebuilder architecture](https://book.kubebuilder.io/architecture.html) with additional patterns for GitHub API integration, rate limiting, and high availability.

## Reconciliation Flow

The operator uses a factory-based reconciliation pattern:

1. **Controller** receives event → checks predicates (generation/annotation changes)
2. **Spreading Check** evaluates if reconciliation should be delayed during startup window
3. **Factory** creates reconciler → fetches CR, builds GitHub client, checks rate limits
4. **Reconciler** executes reconciliation groups in sequence, with parallel execution within each group
5. **Mapper** produces GitHub API request objects with opinionated defaults
6. **GitHub Client** applies changes via GitHub API
7. **Conditions** updated to reflect sync status for each reconciliation task
8. **Status** written back to resource, including sub-resource generation tracking
9. **Requeue** scheduled after configurable interval for continuous drift detection

## Startup Spreading

To prevent API rate limit exhaustion during pod restarts (e.g., rolling deployments), the operator implements a startup spreading mechanism:

- **Spread Period** (default 5 minutes): Window after startup during which reconciliations may be delayed
- **Spread Interval** (default 180 minutes): Time window across which reconciliations are distributed
- **Smart Detection**: Only spreads warm-start reconciliations (healthy resources with unchanged specs)
- **Immediate Processing**: Changed resources, unhealthy resources, and deletions bypass spreading

### Configuration

Control via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `ENABLE_STARTUP_SPREADING` | `true` | Enable/disable spreading |
| `STARTUP_SPREAD_PERIOD_MINUTES` | `5` | Window after startup for spreading |
| `SPREAD_INTERVAL_MINUTES` | `180` | Time window for distribution |

## Parallel Reconciliation

Reconciliation logic is organized into sequential groups, with tasks within each group executing concurrently. For example:

- **Group 1**: Independent tasks that can run in parallel (e.g., org settings, custom properties, rulesets)
- **Group 2**: Dependent tasks that require Group 1 completion
- **Additional groups**: Can be added as needed based on dependencies

Common patterns:

- **Timeout Protection**: Each reconciliation task has a 5-minute timeout
- **Error Handling**: All errors collected and reported; execution stops at first failed group
- **Intended stop**: A task can return `reconciler.StopReconciliation{Message}` when the desired final state is reached and the remaining groups must not run (e.g. an archived, read-only repository). Its condition becomes `True`, the conditions of all later groups become `Unknown` (reason `Skipped`, message names the cause), and the run counts as successful (`Ready=True`, sub-resource generations updated, regular success requeue).

## Status Conditions

Each reconciliation task sets its own condition (e.g. `GitHubSynced`, `RulesetsSynced`, `WebhooksSynced`); `Ready` aggregates them. A failed task sets `Status=False` with one of two reasons; the message always carries the concrete cause:

| Reason | Meaning | Retry behaviour |
|--------|---------|-----------------|
| `SyncFailed` | Transient or unknown failure (network, GitHub 5xx, timeouts) | Exponential backoff |
| `UserActionRequired` | The spec or the state on GitHub has to be changed by a user | No backoff; see below |

`UserActionRequired` is used if `reconciler.RequiresUserAction(err)` reports that the error (including all wrapped and joined errors) consists only of errors requiring user action:

- errors implementing `UserActionRequired() bool` returning `true`: ruleset/webhook/autolink conflicts, unresolvable ruleset `extends` chains (except transient read errors of a parent preset), invalid webhook secret references, a missing default branch, and unresolvable team/org role/GitHub App/bypass actor references (`idresolver.UnresolvableReferenceError`)
- Kubernetes `NotFound` errors: missing referenced resources (presets, code security configurations, secrets, ...). Classified at the error itself, so fetch sites need no special handling and new references are covered automatically.
- GitHub `422 Unprocessable Entity` responses. Their condition (and log) message uses GitHub's response message and validation errors instead of the raw error, e.g. `Validation Failed: default_branch: Cannot update default branch for an empty repository`.

`Ready` uses `UserActionRequired` if all failed tasks require user action, otherwise `ReconcileFailed`. Skipped tasks (see *Intended stop* above) use `Status=Unknown` with reason `Skipped`.

Retry behaviour of `UserActionRequired` errors depends on where the user has to act:

- **Kubernetes resources only** (conflicts, unresolvable `extends` chains, invalid references in the spec): no requeue until a watch event (spec or preset change) or the success requeue interval (`SPREAD_INTERVAL_MINUTES`).
- **GitHub** (`reconciler.AwaitsGitHubChange`: missing team, org role, GitHub App or default branch, GitHub `422` responses): no watch event signals such changes, so the resource is requeued after `GITHUB_STATE_REQUEUE_INTERVAL_MINUTES` (default `15`). Errors mark this by implementing `AwaitsGitHubChange() bool`.
- **Missing referenced Kubernetes resources** (`NotFound`): requeued after `GITHUB_STATE_REQUEUE_INTERVAL_MINUTES` as well. Watched kinds (presets, code security configurations) are reconciled immediately once created; the requeue covers unwatched kinds (e.g. secrets created in a later Argo CD sync wave) without tracking which kinds are watched.

If a run fails with several kinds, the shorter interval is used.

Errors raised before the reconciliation tasks run (rate limits, startup spreading) and blocked organization finalization do not set conditions.

## Rate Limit Handling

The operator uses a **per-organization, per-category** rate limit registry (`OrgRateLimitRegistry`) to protect against GitHub API quota exhaustion:

- **Passive tracking**: A `rateLimitTrackerTransport` HTTP middleware reads the `X-RateLimit-Remaining`, `X-RateLimit-Limit`, and `X-RateLimit-Reset` response headers from every GitHub API call and updates the registry — no extra API requests needed.
- **Stall check on `GetClient`**: Before returning a client to a reconciler, the factory checks the registry. If the remaining quota for any monitored category is below its configured threshold, a `RateLimitedError` is returned and the reconciliation is requeued until after the reset time (plus a configurable grace period).
- **Staleness recovery**: If registry data for an org is older than the configured staleness threshold, the factory refreshes it via a single `GET /rate_limit` call (which is free and not counted against quota).
- **Per-category monitoring**: Four categories are tracked — `core` (all general REST calls), `graphql`, `search`, and `code_search`. Only `core` has a non-zero stall threshold by default; the others are tracked and will emit a runtime warning if traffic is observed without a configured threshold.
- **Priority queue**: Ensures new resources reconcile first when quota is available.

### Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `RATE_LIMIT_STALL_THRESHOLD_CORE` | `100` | Minimum remaining core API calls before stalling reconciliation for an org |
| `RATE_LIMIT_RESET_GRACE_PERIOD_SECONDS` | `10` | Seconds added to the GitHub-reported reset time before allowing reconciliation to resume |
| `RATE_LIMIT_STALENESS_THRESHOLD_MINUTES` | `5` | Minutes after which registry data is considered stale and refreshed via `GET /rate_limit` |

Each organization is tracked independently, so a heavily used org does not delay reconciliation of others.

## Deletion Semantics

The operator implements safe deletion semantics to prevent accidental data loss:

- **Organizations**: The GitHub organization is **never deleted**. The Kubernetes CR can only be removed when no `Repository` or `Team` CRs reference it (enforced via finalizer). This ensures the organization remains intact on GitHub while allowing cleanup of Kubernetes resources.
- **Repositories**: Behavior depends on the `REPOSITORY_FINALIZER_MODE` environment variable:
    - `ignore` or unset (default): Repository remains unchanged on GitHub, only the Kubernetes CR is removed
    - `archive`: Repository is archived on GitHub before the Kubernetes CR is removed, preserving all data while marking it as read-only
    - `delete`: Repository is permanently deleted from GitHub (use with caution)

## GitHub Client Caching

The `GitHubCachingClientFactory` maintains a per-process cache of authenticated GitHub clients:

- Each replica holds its own cache
- Clients are cached per GitHub App installation
- Memory overhead is minimal
- Automatic token refresh on expiration

### Response Caching (`CachingClient`)

Expensive paginated list operations (teams, app installations, org roles) are wrapped in an opt-in TTL cache via `CachingClient`. Caching is controlled at the call site by decorating the context:

```go
ctx = ghclient.WithCache(ctx)  // enables caching for this call
```

Without `WithCache`, all calls pass through to the GitHub API directly. This allows callers that need fresh data (e.g. team membership reconciliation) to bypass the cache while callers that tolerate slight staleness (e.g. slug-to-ID resolution) share cached results across concurrent reconciliations of the same org.

The cache TTL defaults to 5 minutes and can be adjusted via `ClientConfig.ResponseCacheTTL`. Callers can force a cache invalidation via `ghclient.InvalidateCache(client)`, which is done automatically by the ID resolver (`idresolver.Resolver`) when a slug lookup fails (to handle newly created resources not yet in the cached list).

### ID Resolution (`idresolver.Resolver`)

Ruleset bypass actors, required-status-check apps, required-reviewer teams, and code security configuration bypass reviewers all reference GitHub resources by slug or name, which must be resolved to numeric IDs before the GitHub API will accept them.

`idresolver.Resolver` ([internal/reconciler/common/idresolver](../internal/reconciler/common/idresolver)) handles this efficiently:

1. At construction time it makes exactly **three bulk API calls** — `GetAllTeamsForOrg`, `GetGitHubAppsInstallations`, `GetAllOrgRoles` — using the cached context so concurrent reconciliations share results.
2. All subsequent `ResolveTeamSlug`, `ResolveRoleName`, and app-slug lookups are **in-memory map reads** with no further API calls.
3. On lookup failure the resolver invalidates the cache and retries once, gracefully handling newly created resources that are not yet reflected in the cached list.

