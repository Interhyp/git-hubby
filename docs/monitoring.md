# Monitoring

git-hubby exposes Prometheus metrics on the metrics endpoint of the manager (`/metrics`, HTTPS on port `8443` in the
default deployment). The
Grafana dashboard in [`grafana/git-hubby.json`](https://github.com/Interhyp/git-hubby/blob/main/grafana/git-hubby.json)
visualizes them.

## Metrics

Besides the [controller-runtime metrics](https://book.kubebuilder.io/reference/metrics-reference) (reconciliations,
work queues, process), git-hubby exposes:

| Metric | Labels | Description |
|---|---|---|
| `http_client_request_duration_seconds` | `http_request_method`, `http_response_status_code`, `error_type`, `server_address`, ... | Histogram of the GitHub API request attempts. Retries and pages are measured individually; the duration includes waiting for the secondary rate limit. Requests for installation tokens are not measured. |
| `git_hubby_requeues_total` | `controller`, `reason` | Requeue decisions. Reasons: `success` (drift detection), `rate_limit`, `startup_spread`, `user_action`, `awaits_github`, `retry_later`, `archived_repository`, `new_repository`, `dependents_exist`, `error`, `shutdown`. |
| `git_hubby_component_reconcile_duration_seconds` | `kind`, `component`, `outcome` | Histogram of the reconciliation of each component (condition) of a resource. Outcomes: `success`, `error`, `user_action`, `retry_later`, `stopped`. |
| `git_hubby_component_reconcile_skipped_total` | `kind`, `component` | Components skipped because an earlier component stopped the run on purpose (e.g. archived repositories). |
| `git_hubby_github_response_cache_lookups_total` | `cache`, `result` | Hits and misses of the response cache for teams, apps and roles. Calls that bypass the cache are not counted. |
| `git_hubby_github_response_cache_invalidations_total` | `cache` | Invalidations of the response cache. |
| `git_hubby_github_rate_limit_remaining` | `organization`, `category` | Last observed remaining primary rate-limit budget. |
| `git_hubby_github_rate_limit_budget` | `organization`, `category` | Last observed primary rate-limit budget. |
| `git_hubby_github_rate_limit_reset_timestamp_seconds` | `organization`, `category` | Last observed reset time as a Unix timestamp. |
| `git_hubby_github_rate_limit_observed_timestamp_seconds` | `organization`, `category` | Time of the last observation as a Unix timestamp. |
| `git_hubby_github_rate_limit_stall_threshold` | `organization`, `category` | Configured stall threshold; zero means stalling is disabled. |
| `git_hubby_github_rate_limit_stall_until_timestamp_seconds` | `organization`, `category` | Reset time plus grace period as a Unix timestamp when the last observed budget is below the stall threshold; otherwise `0`. |

Rate-limit gauges appear after the first observation and cover primary rate limits only. Their values may be stale;
use the observation timestamp to check their age. Organizations can share a quota, so their budgets should not be
summed.

## Resource state via kube-state-metrics

The readiness of the managed resources is part of their status conditions. git-hubby does not export it itself;
[kube-state-metrics](https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/extend/customresourcestate-metrics.md)
can export it with its custom resource state configuration. The "Managed resources" panels of the dashboard expect the
following configuration. If you use a different `metricNamePrefix`, set the dashboard variable `ksm_prefix`
accordingly.

```yaml
kind: CustomResourceStateMetrics
spec:
  resources:
    - groupVersionKind:
        group: github.interhyp.de
        version: "v1alpha1"
        kind: "*"
      metricNamePrefix: git_hubby
      labelsFromPath:
        namespace: [metadata, namespace]
        name: [metadata, name]
      metrics:
        - name: condition
          help: "Status of each condition in status.conditions (1 = True, 0 = False or Unknown)"
          each:
            type: Gauge
            gauge:
              path: [status, conditions]
              labelsFromPath:
                condition: [type]
                reason: [reason]
              valueFrom: [status]
        - name: condition_observed_generation
          help: "metadata.generation the condition was last computed for"
          each:
            type: Gauge
            gauge:
              path: [status, conditions]
              labelsFromPath:
                condition: [type]
              valueFrom: [observedGeneration]
        - name: generation
          help: "metadata.generation"
          each:
            type: Gauge
            gauge:
              path: [metadata, generation]
```

kube-state-metrics needs `list` and `watch` permissions for the resources:

```yaml
- apiGroups: ["github.interhyp.de"]
  resources: ["*"]
  verbs: ["list", "watch"]
```

With the [kube-state-metrics Helm chart](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-state-metrics),
set the configuration as `customResourceState.config` (with `customResourceState.enabled: true`) and the permissions as
`rbac.extraRules`.

Notes:

- The condition message is not used as a label because it contains timestamps and would create a new series on
  every reconciliation.
- kube-state-metrics exports both `False` and `Unknown` as `0`. Conditions are `Unknown` in two harmless cases: `Ready`
  while a resource is reconciled (reason `ReconcileStarted`), and the components of archived repositories (reason
  `Skipped`). Select failures by reason: `ReconcileFailed` or `UserActionRequired` for `Ready`, `SyncFailed` or
  `UserActionRequired` for the components. The dashboard does so.
- The `namespace` label is the namespace of the resource. If kube-state-metrics is scraped without `honorLabels: true`,
  Prometheus renames it to `exported_namespace`.
- Each resource creates about two series per condition (a Repository has nine conditions) plus one for the
  generation.

## Dashboard

Import [`grafana/git-hubby.json`](https://github.com/Interhyp/git-hubby/blob/main/grafana/git-hubby.json) into Grafana
and select the Prometheus data source. Variables:

| Variable | Description |
|---|---|
| `datasource` | Prometheus data source. |
| `job`, `namespace` | Scrape job and namespace of the git-hubby metrics endpoint. |
| `ksm_prefix` | `metricNamePrefix` of the kube-state-metrics configuration (default `git_hubby`). |
| `filters` | Additional label filters, e.g. `cluster` or `environment` if several clusters write to the same Prometheus. They apply to all queries. |

Without the kube-state-metrics configuration, the "Managed resources" panels show a hint instead of data.
The "GitHub API rate limits" panels require the rate-limit metrics introduced in git-hubby 0.13.0; they have no
series for an organization and category until the operator observes them.

Interpretation:

- The primary quota panels show the last observation per organization, category and pod, not a live GitHub query.
  Check "Rate-limit observation age": inactive categories and former leaders may retain old values. An observed
  reset countdown of zero does not mean the cached quota has refreshed. Do not sum budgets across organizations,
  which can share a quota.
- The stall threshold panel omits zero thresholds, which mean stalling is disabled for that category. A stall
  countdown of zero means the last observed primary-rate-limit stall deadline is absent or has passed; it does not
  rule out secondary rate limits or other reconciliation delays.

- `requeue_after` is the regular result of a reconciliation: successful reconciliations are requeued for drift
  detection. "Requeues by reason" shows why resources are reconciled again.
- `workqueue_retries_total` is not shown: with the priority queue of controller-runtime it also counts delayed requeues
  and does not indicate errors.
- Failed reconciliations are retried, so alert on sustained failures only.
