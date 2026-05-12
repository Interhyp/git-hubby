# High Availability & Multiple Replicas

This document explains why running the git-hubby operator with multiple replicas (default: 2) is safe, and how the various subsystems behave in a multi-replica deployment.

## Overview

The operator is designed to run with `replicas: 2` (or more) to ensure **zero-downtime** during pod restarts, rolling deployments, and cluster node drains. Leader election guarantees that only one replica performs reconciliation at any time, while all replicas serve webhook requests.

## Leader Election

Leader election is enabled via the `--leader-elect` flag and configured in `cmd/main.go`:

- **Lease ID**: `6cee1c41.interhyp.de`
- **`LeaderElectionReleaseOnCancel: true`**: When a pod is terminated, it immediately releases the lease instead of waiting for the full lease duration to expire. This ensures fast leader transitions (~seconds instead of ~15s).

Only the **leader replica** runs the controller reconciliation loops (Organization, Repository, Team). The standby replica idles with respect to reconciliation but remains ready to take over.

## Webhook Serving

Webhook serving is **not gated by leader election**. All replicas register and serve the validation webhooks:

- `/validate-github-interhyp-de-v1alpha1-organization`
- `/validate-github-interhyp-de-v1alpha1-repository`

This is the primary reason for running multiple replicas: the Kubernetes API server load-balances webhook calls across all healthy pod endpoints. If one pod restarts, the other continues serving webhook traffic, preventing the `"no endpoints available for service"` errors that occur with a single replica.

Webhook validation is stateless — it reads the Kubernetes API (to fetch the referenced Organization) and may call the GitHub API (to fetch custom property definitions for repository validation). Both operations are safe to perform from any replica.

## Subsystem Behavior

### GitHub Rate Limiter

The `globalLimiter` (in-process token bucket) is instantiated per replica. Since only the leader performs reconciliation, only one replica consumes the GitHub API budget for reconciliation workloads. Webhook validation calls are infrequent (only on Repository CREATE/UPDATE) and do not significantly impact rate limits.

### Startup Spreading

Each replica creates its own `SpreadingManager` with its own start time. Only the leader's spreading logic is active because spreading is evaluated in the reconciler factory, which is only called during reconciliation (leader-only). When leadership transfers, the new leader starts a fresh spread period — this is correct and expected behavior.

### GitHub Client Caching

The `GitHubCachingClientFactory` maintains a per-process cache of authenticated GitHub clients. Each replica holds its own cache. The standby replica may create clients for webhook validation (repository custom property checks), but the reconciliation client cache is only populated on the leader. Memory overhead is minimal.

### Informer Caches

Both replicas run controller-runtime informer caches that watch the configured namespace(s). This doubles the number of watch connections to the API server but is standard and negligible for typical cluster sizes.

### Success Requeue Interval

The `SuccessRequeueInterval` (defaults to the spread interval, ~180 minutes) is only relevant on the leader replica, which is the only one processing reconciliation results.

## Deployment Strategy

The Helm chart configures the deployment for zero-downtime rolling updates:

```yaml
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxUnavailable: 0
    maxSurge: 1
```

- **`maxUnavailable: 0`**: Kubernetes will not terminate any existing pod until its replacement is ready. This ensures at least 2 pods are available during a rollout (briefly 3 with `maxSurge: 1`).
- Combined with `replicas: 2`, there is always at least one healthy webhook endpoint.

### Graceful Shutdown (preStop Hook)

A `preStop` lifecycle hook delays pod shutdown by 5 seconds:

```yaml
lifecycle:
  preStop:
    exec:
      command: ["sleep", "5"]
```

**Why this is needed:** When a pod enters `Terminating` state, Kubernetes simultaneously sends SIGTERM to the container and removes the pod from Service endpoints. However, kube-proxy/iptables rules update asynchronously — during a brief window (typically 1–3 seconds), traffic may still route to the terminating pod. Without the `preStop` sleep, the webhook server shuts down immediately on SIGTERM while traffic is still arriving, causing `context deadline exceeded` errors.

The 5-second sleep ensures the pod continues serving webhook requests long enough for all nodes' iptables rules to converge, after which no new traffic arrives and the pod shuts down cleanly.

The `terminationGracePeriodSeconds` is set to `20` to accommodate: preStop (5s) + graceful manager shutdown (~15s for finishing in-flight reconciliations, releasing leader lease, closing informers). In-flight reconciliations that cannot complete before shutdown receive a context cancellation — this is handled gracefully by the controllers (logged at debug level, no error requeue).

## Pod Disruption Budget

A `PodDisruptionBudget` with `minAvailable: 1` protects against voluntary disruptions such as node drains during cluster upgrades:

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
spec:
  minAvailable: 1
```

This tells the Kubernetes eviction API to drain pods one at a time, waiting for a replacement to become ready before evicting the next. Without a PDB, `kubectl drain` evicts all pods simultaneously.

## Topology Spread

The default `topologySpreadConstraints` ensure replicas are distributed across different nodes:

```yaml
controllerManager:
  topologySpreadConstraints:
    - maxSkew: 1
      topologyKey: kubernetes.io/hostname
      whenUnsatisfiable: ScheduleAnyway
```

The `labelSelector` is automatically injected by the deployment template using the chart's selector labels — you only need to specify the topology parameters in `values.yaml`. If you need a custom `labelSelector`, you can provide one explicitly and it will be used as-is.

This prevents a single node failure or drain from affecting both replicas. `ScheduleAnyway` (soft constraint) avoids blocking scheduling when only one node is available (e.g., development clusters).

## Disruption Scenario Summary

| Scenario | Protection Mechanism | Webhook Availability |
|---|---|---|
| Deployment rollout (image update) | `maxUnavailable: 0`, `maxSurge: 1` | ✅ Uninterrupted |
| Single node drain (cluster upgrade) | PDB `minAvailable: 1`, topology spread | ✅ Uninterrupted |
| Multi-node drain (major upgrade) | PDB forces sequential eviction | ✅ Uninterrupted |
| Pod crash (OOM, bug) | Second replica continues serving | ✅ Uninterrupted |
| Leader pod termination | `LeaderElectionReleaseOnCancel` for fast failover | ✅ Webhooks unaffected; reconciliation resumes within seconds |

## Configuration

All relevant settings are in `chart/values.yaml`:

```yaml
controllerManager:
  replicas: 2                    # Number of replicas
  strategy:                      # Rolling update strategy
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1
  topologySpreadConstraints:     # Node distribution (labelSelector auto-injected)
    - maxSkew: 1
      topologyKey: kubernetes.io/hostname
      whenUnsatisfiable: ScheduleAnyway
```

To revert to a single replica (not recommended for production):

```yaml
controllerManager:
  replicas: 1
  strategy: {}
  topologySpreadConstraints: []
```

