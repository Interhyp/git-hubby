# Git-Hubby documentation

## Description
A Kubernetes operator for managing GitHub enterprise organizations and repositories as code using Custom Resource Definitions (CRDs).

## Key Features

- **Declarative GitHub Management**: Define organizations and repositories as Kubernetes resources
- **GitHub App Integration**: Secure authentication using GitHub App credentials
- **Advanced Features**: Manage repository rulesets, webhooks, and organization custom properties
- **Rate Limit Awareness**: Built-in GitHub API rate limit handling with intelligent backoff
- **Startup Spreading**: Distributes reconciliations over time during pod startup to prevent API thundering herd
- **Webhook Validation**: Comprehensive validation of resource specifications
- **Status Tracking**: Detailed status conditions for monitoring reconciliation state with sub-resource generation tracking
- **Parallel Reconciliation**: Concurrent execution of independent reconciliation tasks for improved performance
- **High Availability**: Safe multi-replica deployment with zero-downtime rolling updates and node drain protection ([details](high-availability.md))

## Architecture

The architecture follows the default [kubebuilder architecture](https://book.kubebuilder.io/architecture.html).