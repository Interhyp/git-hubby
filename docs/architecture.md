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

## Rate Limit Handling

The operator manages reconciliation timing to conserve GitHub API quota:

- Checks remaining quota before each reconciliation (threshold: 100 requests)
- Delays reconciliations until rate limit resets when quota is low
- Global limiter synchronizes delays across all controller instances
- Priority queue ensures new resources reconcile first when quota becomes available

This protects against self-inflicted rate limit exhaustion but does not prevent exhaustion from external sources (CI/CD, other tools).

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
