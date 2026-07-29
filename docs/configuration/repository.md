# Repository Configuration

This guide demonstrates how to configure a GitHub Repository using git-hubby.
We continue with the **Acme Corp** example, configuring their `order-service` repository.

For the complete field reference, see the [API Documentation](../crds.md#repository).

## Example: Order Service

```yaml
apiVersion: github.interhyp.de/v1alpha1
kind: Repository
metadata:
  name: order-service
  namespace: git-hubby-system
spec:
  # --- Organization Reference ---
  organizationRef:
    name: acme-corp                         # Must match an Organization CR

  # --- Identity ---
  name: order-service                       # Repository name on GitHub
  visibility: internal                      # internal | private | public

  # --- Metadata ---
  about:
    description: "Order management service - handles cart, checkout, and order lifecycle"
    website: "https://docs.acme-corp.com/order-service"
    topics:
      - go
      - grpc
      - kubernetes
      - orders
      - checkout

  # --- Custom Properties ---
  # Values for properties defined in the Organization
  customProperties:
    - propertyName: team
      value: checkout
    - propertyName: service-tier
      value: critical
    - propertyName: compliance
      values:
        - pci-dss
        - gdpr
    - propertyName: slack-channel
      value: "#checkout-alerts"

  # --- Merge Settings ---
  mergeStrategies:
    - type: squash                          # Only allow squash merges

  # --- Repository Features ---
  hasIssues: true
  hasWiki: false
  hasProjects: false
  hasDiscussions: false
  allowAutoMerge: true
  deleteBranchOnMerge: true
  allowForking: false
  webCommitSignoffRequired: true

  # --- Team Permissions ---
  teams:
    - teamRef:
        name: checkout-team
      permission: push
    - teamRef:
        name: platform-team
      permission: admin
    - teamRef:
        name: security-team
      permission: triage

  # --- Collaborator Permissions ---
  collaborators:
    - username: alice
      permission: push
    - username: bob
      permission: admin
    - username: charlie
      permission: triage

  # --- Webhooks ---
  webhookPresets:
    - name: ci-webhooks
    - name: security-scanning

  # --- Branch Protection ---
  rulesetPresets:
    - name: default-branch-protection

  rulesets:
    - name: release-branches
      target: branch
      enforcement: active
      conditions:
        refName:
          include:
            - "refs/heads/release/*"
      rules:
        pullRequest:
          requiredApprovingReviewCount: 2
          dismissStaleReviewsOnPush: true
        deletion: true

  # --- Security ---
  attachedCodeSecurityConfiguration:
    name: strict-security

  # --- Deploy Keys ---
  deployKeys:
    - title: "ArgoCD Read-Only"
      key: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample..."
      readOnly: true
```

## Key Concepts

### Organization Reference

Every repository must belong to an Organization:

```yaml
spec:
  organizationRef:
    name: acme-corp    # References the Organization CR by metadata.name
```

The Organization CR must exist before creating Repository CRs that reference it.

### Custom Properties

Set values for properties defined in your Organization:

```yaml
# In Organization: defines the property schema
customProperties:
  - propertyName: team
    valueType: single_select
    allowedValues: [platform, checkout, payments]

# In Repository: sets the value
customProperties:
  - propertyName: team
    value: checkout           # single_select uses 'value'

  - propertyName: compliance
    values:                   # multi_select uses 'values'
      - pci-dss
      - gdpr
```

### Team Permissions

Assign GitHub teams with specific access levels:

| Permission | Capabilities |
|------------|-------------|
| `pull` | Clone, view code and issues |
| `triage` | + Manage issues and PRs (no code changes) |
| `push` | + Push code, create branches |
| `maintain` | + Manage settings (except sensitive) |
| `admin` | Full access including settings and secrets |

### Webhooks

**Using Presets** (recommended for consistency):

```yaml
webhookPresets:
  - name: ci-webhooks          # References a WebhookPreset CR
  - name: security-scanning
```

**Inline webhooks** for repository-specific needs:

```yaml
webhooks:
  - name: deploy-trigger
    config:
      url: "https://deploy.acme-corp.com/webhook"
      contentType: json
      secret:
        secretRef:
          name: webhook-secrets
          key: deploy-secret
    events:
      - push
      - release
    active: true
```

### Rulesets

**Using Presets** applies organization-wide rules:

```yaml
rulesetPresets:
  - name: default-branch-protection
```

**Inline rulesets** for repository-specific rules:

```yaml
rulesets:
  - name: release-branches
    target: branch
    enforcement: active
    conditions:
      refName:
        include:
          - "refs/heads/release/*"
    rules:
      pullRequest:
        requiredApprovingReviewCount: 2
        # requiredReviewers requires ENABLE_REQUIRED_REVIEWERS_RULES=true (GitHub beta feature)
        requiredReviewers:
          - minimumApprovals: 1
            filePatterns:
              - "src/**"
            reviewer:
              slug: security-team   # resolved to ID at reconciliation time
              type: Team
      requiredStatusChecks:
        requiredStatusChecks:
          - context: "ci/build"
          - context: "ci/test"
      deletion: true           # Prevent branch deletion
      nonFastForward: true     # Prevent force-push
```

> **Note**: The `requiredReviewers` field in pull-request rules uses a GitHub API feature that is
> currently in **beta**. Reconciliation of this field is disabled by default and must be explicitly
> opted in by setting the environment variable `ENABLE_REQUIRED_REVIEWERS_RULES=true` on the
> controller manager. When the flag is not set, any `requiredReviewers` entries defined in the spec
> are ignored and will not be applied to or compared against GitHub.
> See [Feature Flags](../architecture.md#feature-flags) for all available flags.

### Deploy Keys

SSH keys for CI/CD systems to access the repository:

```yaml
deployKeys:
  - title: "ArgoCD Read-Only"
    key: "ssh-ed25519 AAAAC3..."
    readOnly: true             # Can only clone/pull

  - title: "Release Bot"
    key: "ssh-ed25519 AAAAC3..."
    readOnly: false            # Can also push
```

## Deletion Behavior

When a Repository CR is deleted, the behavior depends on the `REPOSITORY_FINALIZER_MODE` environment variable:

| Mode | Behavior |
|------|----------|
| `ignore` (default) | Repository unchanged on GitHub, only CR removed |
| `archive` | Repository archived on GitHub (read-only, preserves data) |
| `delete` | Repository permanently deleted from GitHub |

This protects against accidental data loss. The default `ignore` mode requires manual cleanup on GitHub.

## Related Resources

- [Organization Configuration](organization.md) - Configure the parent organization
- [API Reference](../crds.md#repository) - Complete field reference
