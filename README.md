# git-hubby

A Kubernetes operator for managing GitHub organizations and repositories as code using Custom Resource Definitions (CRDs).

> **Documentation**: For detailed configuration guides and examples, visit the [full documentation](https://interhyp.github.io/git-hubby/).

## Overview

**git-hubby** is a [Kubebuilder](https://kubebuilder.io/)-based Kubernetes operator that enables declarative management of GitHub resources. It synchronizes GitHub organizations and repositories with their desired state defined in Kubernetes custom resources, including advanced features like rulesets, webhooks, and custom properties.

### Key Features

- **Declarative GitHub Management**: Define organizations and repositories as Kubernetes resources
- **Multiple GitHub Apps**: Each organization can reference its own GitHub App credentials secret, enabling multi-tenant and multi-app setups
- **Multi-Plan Support**: Works with GitHub `free`, `team`, and `enterprise` plans — plan-gated features are automatically skipped when not available
- **Advanced Features**: Manage repository rulesets, webhooks, organization custom properties, and code security configurations
- **Rate Limit Awareness**: Built-in GitHub API rate limit handling with intelligent backoff
- **Startup Spreading**: Distributes reconciliations over time during pod startup to prevent API thundering herd
- **Webhook Validation**: Comprehensive validation of resource specifications
- **Status Tracking**: Detailed status conditions for monitoring reconciliation state with sub-resource generation tracking
- **Parallel Reconciliation**: Concurrent execution of independent reconciliation tasks for improved performance
- **High Availability**: Safe multi-replica deployment with zero-downtime rolling updates ([details](https://interhyp.github.io/git-hubby/techdocs/high-availability/))

### Managed Resources

- **Organization** (`github.interhyp.de/v1alpha1`): GitHub organization settings, custom properties, and rulesets
- **Repository** (`github.interhyp.de/v1alpha1`): GitHub repositories with webhooks, rulesets, and configurations
- **RulesetPreset** (`github.interhyp.de/v1alpha1`): Reusable ruleset templates for organizations and repositories
- **WebhookPreset** (`github.interhyp.de/v1alpha1`): Reusable webhook configurations for repositories
- **Team** (`github.interhyp.de/v1alpha1`): GitHub team management with IDP group sync
- **CodeSecurityConfiguration** (`github.interhyp.de/v1alpha1`): Security settings like dependency and secret scanning

## Getting Started

### Prerequisites

- A GitHub organization on any plan (`free`, `team`, or `enterprise`). Set `spec.plan` on the `Organization` resource to match your GitHub plan — defaults to `enterprise` for backward compatibility. Feature availability varies by plan (see [GitHub Plan Compatibility](#github-plan-compatibility) below).
- Go 1.25.5 or later
- Kubernetes cluster (v1.34+ recommended)
- kubectl configured to access your cluster
- [mise](https://mise.jdx.dev/) for environment management (optional but recommended)

### Quick Start

```bash
git clone <repository-url>
cd git-hubby
mise install          # optional — installs Go and Kubebuilder
go mod download
make env              # create .env from template (git-ignored)
make install          # install CRDs into your cluster
make run              # run locally with webhooks disabled
```

Edit `.env` to configure local settings such as `LOG_LEVEL`, `LOG_FORMAT`, and `WATCH_NAMESPACE`.

For the full development setup, make targets, testing, and code conventions, see [CONTRIBUTING.md](CONTRIBUTING.md).

## GitHub Plan Compatibility

The operator supports GitHub organizations on all billing plans. Feature availability is automatically gated by the `spec.plan` field on the `Organization` resource:

| Feature | free | team | enterprise |
|---|---|---|---|
| Repository & organization settings | ✓ | ✓ | ✓ |
| Repository rulesets (public repos) | ✓ | ✓ | ✓ |
| Repository rulesets (private/internal repos) | ✗ | ✓ | ✓ |
| Organization rulesets | ✗ | ✓ | ✓ |
| Code security configurations | ✗ | ✗ | ✓ |
| IDP group sync (Teams) | ✗ | ✗ | ✓ |
| Internal repository visibility | ✗ | ✗ | ✓ |

Invalid plan and feature combinations are rejected during resource validation (admission webhook). Plan defaults to `enterprise` for backward compatibility.

## Configuration

### Logging

The operator's log level can be configured via:

- **Environment variable**: `LOG_LEVEL` — accepts `debug`, `info`, `error` (case-insensitive). Overrides the CLI flag.
- **CLI flag**: `--zap-log-level` — standard controller-runtime zap flag.
- **`.env` file**: Set `LOG_LEVEL=debug` in your `.env` file (loaded automatically on startup).

The log output format can be configured via:

- **Environment variable**: `LOG_FORMAT` — accepts the following values (case-insensitive):
  - `json` (default) — structured JSON, the standard kubebuilder format.
  - `ecs` — [Elastic Common Schema](https://www.elastic.co/guide/en/ecs/current/index.html) JSON format, suitable for Elasticsearch/Kibana environments.
  - `console` — human-readable console output, ideal for local development.

### GitHub App Credentials

The operator authenticates with GitHub using GitHub App credentials stored in Kubernetes Secrets. Each organization can reference its own credentials secret, enabling multiple GitHub Apps across organizations.

#### Secret Format

Create one or more Secrets, each containing credentials for a GitHub App:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: git-hubby-app-credentials    # default secret name
  namespace: github-controller
type: Opaque
stringData:
  app-id: "<your-github-app-id>"
  private-key: |
    -----BEGIN RSA PRIVATE KEY-----
    <your-private-key>
    -----END RSA PRIVATE KEY-----
```

All credential secrets must reside in the same namespace, configured via:
- `--app-credentials-secret-namespace` (default: `github-controller`)

The default secret name is configured via:
- `--app-credentials-secret-name` (default: `git-hubby-app-credentials`)

#### Per-Organization App Configuration (recommended)

Use `spec.githubAppConfig` on the `Organization` resource to specify both the installation ID and which credentials secret to use:

```yaml
spec:
  githubAppConfig:
    installationId: 12345678
    credentialsSecretName: my-org-app-credentials  # references a Secret in the credentials namespace
```

This is the preferred approach and supports multiple GitHub Apps across different organizations.

#### Legacy: Single App via Installation ID (deprecated)

The older `spec.githubAppInstallationId` field is still supported for backward compatibility. When set alone, it uses the default credentials secret configured via `--app-credentials-secret-name`:

```yaml
spec:
  githubAppInstallationId: 12345678   # deprecated; use githubAppConfig instead
```

If both `githubAppConfig` and `githubAppInstallationId` are set, `githubAppConfig` takes precedence.

#### Secret Rotation

Updating a credentials Secret in Kubernetes does **not** automatically invalidate the operator's in-memory client cache. To force the operator to pick up rotated credentials, restart the operator pod.

For automated rotation workflows, we recommend [Stakater Reloader](https://github.com/stakater/Reloader). Add the reload annotation to the operator `Deployment` and list the Secret(s) that should trigger a restart:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: git-hubby-controller-manager
  annotations:
    secret.reloader.stakater.com/reload: "git-hubby-app-credentials,my-org-app-credentials"
```

When Reloader detects a change in any of the listed Secrets, it rolls the Deployment, causing the operator to restart and re-fetch fresh credentials.

## Architecture Highlights

### Reconciliation Flow

1. **Controller** receives event → checks predicates (generation/annotation changes)
2. **Spreading Check** evaluates if reconciliation should be delayed during startup window
3. **Factory** creates reconciler → fetches CR, builds GitHub client, checks rate limits
4. **Reconciler** executes reconciliation groups in sequence, with parallel execution within each group
5. **Mapper** produces GitHub API request objects with opinionated defaults
6. **GitHub Client** applies changes via GitHub API
7. **Conditions** updated to reflect sync status for each reconciliation task
8. **Status** written back to resource, including sub-resource generation tracking
9. **Requeue** scheduled after configurable interval for continuous drift detection

### Startup Spreading

To prevent API rate limit exhaustion during pod restarts (e.g., rolling deployments), the operator implements a startup spreading mechanism:

- **Spread Period** (default 5 minutes): Window after startup during which reconciliations may be delayed
- **Spread Interval** (default 180 minutes): Time window across which reconciliations are distributed
- **Smart Detection**: Only spreads warm-start reconciliations (healthy resources with unchanged specs)
- **Immediate Processing**: Changed resources, unhealthy resources, and deletions bypass spreading
- **Configuration**: Control via environment variables:
  - `ENABLE_STARTUP_SPREADING` (default: true)
  - `STARTUP_SPREAD_PERIOD_MINUTES` (default: 5)
  - `SPREAD_INTERVAL_MINUTES` (default: 180)

### Parallel Reconciliation

Reconciliation logic is organized into sequential groups, with tasks within each group executing concurrently. For example:

- **Group 1**: Independent tasks that can run in parallel (e.g., org settings, custom properties, rulesets)
- **Group 2**: Dependent tasks that require Group 1 completion
- **Additional groups**: Can be added as needed based on dependencies

Common patterns:

- **Timeout Protection**: Each reconciliation task has a 5-minute timeout
- **Error Handling**: All errors collected and reported; execution stops at first failed group

### Rate Limit Handling

- Factory checks remaining GitHub API quota before reconciliation
- Returns `RateLimitedError` when quota is low
- Controllers use exponential backoff + global GitHub limiter
- Priority queue ensures new resources reconcile immediately
- Successful reconciliations requeue after spread interval for continuous monitoring

### Deletion Semantics

- **Organizations**: Only deleted when no `Repository` references remain (enforced via finalizer)
- **Repositories**: Archived instead of hard-deleted

### Feature Flags

Boolean environment variables that enable or disable operator functionality, loaded once at startup. Invalid values cause the operator to exit with a clear error.

| Variable | Default | Description |
|----------|---------|-------------|
| `ENABLE_STARTUP_SPREADING` | `true` | Enable startup spreading to prevent API rate-limit exhaustion after pod restarts |
| `ENABLE_WEBHOOKS` | `true` | Enable the admission webhook server. Set `false` for local development without cert-manager |
| `ENABLE_REQUIRED_REVIEWERS_RULES` | `false` | Enable `requiredReviewers` in pull-request ruleset rules (GitHub API feature is in **beta**) |

See [Architecture → Feature Flags](https://interhyp.github.io/git-hubby/architecture/#feature-flags) for details.

## Documentation

For detailed configuration and usage information, see the [full documentation](https://interhyp.github.io/git-hubby/):

- [Organization Configuration](https://interhyp.github.io/git-hubby/configuration/organization/) - Custom properties, Actions settings, rulesets
- [Repository Configuration](https://interhyp.github.io/git-hubby/configuration/repository/) - Teams, webhooks, deploy keys

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, coding conventions, testing, and how to submit changes.

## Resources

- [Full Documentation](https://interhyp.github.io/git-hubby/)
- [Helm Chart Repository](https://github.com/Interhyp/git-hubby-helm)
- [Kubebuilder Documentation](https://book.kubebuilder.io/)
- [Controller Runtime](https://github.com/kubernetes-sigs/controller-runtime)
- [GitHub API Documentation](https://docs.github.com/en/rest)

---

Built with ❤️ using [Kubebuilder](https://kubebuilder.io/)
