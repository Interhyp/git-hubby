# Packaging & Deployment: Kustomize and Helm

This document explains how git-hubby is packaged for deployment and the relationship between the Kustomize and Helm workflows.

## Overview

git-hubby supports two deployment methods:

| Method | Primary Use | Source of Truth |
|--------|-------------|-----------------|
| **Kustomize** (`make deploy`) | Local development, CI/CD testing | `config/` manifests |
| **Helm** (`helm install`) | Production, staging | [Interhyp/git-hubby-helm](https://github.com/Interhyp/git-hubby-helm) |

> **Important**: The Helm chart is maintained as a separate repository. Only CRDs are automatically synced from this repo via CI. All other Helm templates (deployment, RBAC, webhooks, services) are maintained manually in the Helm chart repo.

## Automated CRD Sync

The **Update Helm Chart** workflow automatically copies generated CRDs to the Helm chart repo:

```
  config/crd/bases/*.yaml  ──►  git-hubby-helm/crds/*.yaml
```

This runs after every successful "Build & Release" workflow. For main branch releases, a PR is created in the Helm chart repo.

## What Requires Manual Helm Chart Updates

| Change in this repo | Helm chart file to update |
|---|---|
| `+kubebuilder:rbac` markers | RBAC templates (roles) |
| `+kubebuilder:webhook` markers | `validating-webhook-configuration.yaml` |
| `config/manager/manager.yaml` (env vars, args, ports) | `deployment.yaml` |
| New CRD types (types.go) | CRDs updated automatically; may need RBAC + webhook updates |

The CI workflow comments on PRs when it detects these changes.

## Kustomize Deployment

Kustomize is used for local development and serves as the source for CRD generation:

```bash
make install    # Install CRDs into current cluster
make deploy     # Deploy full operator via kustomize
make run        # Run operator locally (CRDs must be installed)
```

### Namespace Model

Kustomize collapses everything into a single namespace (`git-hubby-system`) via the global namespace transformer in `config/default/kustomization.yaml`. Patches align `WATCH_NAMESPACE` and webhook `namespaceSelector` with this namespace.

## Helm Deployment (Production)

The Helm chart in [Interhyp/git-hubby-helm](https://github.com/Interhyp/git-hubby-helm) supports multi-namespace deployment:

- **Controller namespace**: Release namespace
- **Watch namespaces**: Configurable list via `controllerManager.watchedNamespaces`
- **Credentials namespace**: Configurable via `controllerManager.appCredentialsSecretNamespace`

The chart creates per-namespace RBAC (Role + RoleBinding) for each watched namespace.

## Why Kustomize is Still Required

Even though the Helm chart is maintained separately:

1. **CRD generation**: `make manifests` generates CRDs from kubebuilder markers into `config/crd/bases/`, which are then synced to the Helm chart.
2. **RBAC generation**: `config/rbac/role.yaml` serves as a reference when updating Helm RBAC templates.
3. **Local development**: `make deploy` and `make run` use kustomize directly.
4. **CI/CD**: E2E tests use kustomize-based deployment on Kind clusters.

## Adding a New Namespace-Sensitive Resource

When adding manifests that need different namespaces in kustomize vs Helm:

1. **Set the production namespace** in the base manifest (e.g., `config/rbac/`).
2. **Add a kustomize patch** in `config/default/` if the kustomize flow needs a different namespace.
3. **Update the Helm chart** in `git-hubby-helm` with proper namespace templating.
