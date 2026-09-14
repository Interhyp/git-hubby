# git-hubby

A Kubernetes operator for managing GitHub organizations and repositories as code using Custom Resource Definitions (CRDs).

## Overview

**git-hubby** is a [Kubebuilder](https://kubebuilder.io/)-based Kubernetes operator that enables declarative management of GitHub resources. It synchronizes GitHub organizations and repositories with their desired state defined in Kubernetes custom resources, including advanced features like rulesets, webhooks, and custom properties.

## Key Features

- **Declarative GitHub Management**: Define organizations and repositories as Kubernetes resources
- **GitHub App Integration**: Secure authentication using GitHub App credentials
- **Multi-Plan Support**: Works with GitHub `free`, `team`, and `enterprise` plans — plan-gated features are automatically skipped when not available
- **Advanced Features**: Manage repository rulesets, webhooks, organization custom properties, and code security configurations
- **Rate Limit Awareness**: Built-in GitHub API rate limit handling with intelligent backoff
- **High Availability**: Safe multi-replica deployment with zero-downtime rolling updates

## Managed Resources

| Resource | Description |
|----------|-------------|
| **Organization** | GitHub organization settings, custom properties, and rulesets |
| **Repository** | GitHub repositories with webhooks, rulesets, and configurations |
| **RulesetPreset** | Reusable ruleset templates for organizations and repositories |
| **WebhookPreset** | Reusable webhook configurations for repositories |
| **Team** | GitHub team management with IDP group sync |
| **CodeSecurityConfiguration** | Security settings like dependency and secret scanning |

## Quick Start

### Prerequisites

- Kubernetes cluster (v1.34+)
- GitHub App with appropriate permissions
- kubectl configured to access your cluster

### Installation

```bash
# Add the Helm repository
helm repo add git-hubby https://interhyp.github.io/git-hubby-helm

# Install the operator
helm install git-hubby git-hubby/git-hubby-helm \
  --namespace git-hubby-system \
  --create-namespace
```

### Create GitHub App Credentials

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: git-hubby-app-credentials
  namespace: git-hubby-system
type: Opaque
stringData:
  app-id: "<your-github-app-id>"
  private-key: |
    -----BEGIN RSA PRIVATE KEY-----
    <your-private-key>
    -----END RSA PRIVATE KEY-----
```

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

## Documentation

- [Organization Configuration](configuration/organization.md) - Custom properties, Actions settings, rulesets
- [Repository Configuration](configuration/repository.md) - Teams, webhooks, deploy keys
- [Architecture](architecture.md) - Reconciliation flow, rate limiting, spreading
- [API Reference](crds.md) - Complete CRD API documentation

## Contributing

We welcome contributions! See [CONTRIBUTING.md](https://github.com/Interhyp/git-hubby/blob/main/CONTRIBUTING.md) for development setup, coding conventions, testing, and how to submit changes.

## Resources

- [GitHub Repository](https://github.com/Interhyp/git-hubby)
- [Helm Chart Repository](https://github.com/Interhyp/git-hubby-helm)
- [Kubebuilder Documentation](https://book.kubebuilder.io/)
- [GitHub API Documentation](https://docs.github.com/en/rest)
