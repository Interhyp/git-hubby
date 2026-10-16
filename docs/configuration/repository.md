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
    - name: checkout-team
      permission: push
    - name: platform-team
      permission: admin
    - name: security-team
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
  - payloadUrl: "https://deploy.acme-corp.com/webhook/{{ .Name }}"   # Go templates are rendered against the GitHub repository
    contentType: json
    secret:
      name: webhook-secrets      # Secret in the Repository's namespace
      key: deploy-secret
    events:
      - push
      - release
    active: true
```

Inline webhooks support exactly the same fields as a `WebhookPreset` spec. Their behaviour:

- The operator reconciles the **union** of inline `webhooks` and the referenced `webhookPresets`. At most **10** inline webhooks are allowed.
- A webhook is identified by its (templated) payload URL, content type and events. If several definitions (inline or from presets) share this identity:
    - **identical definitions** are applied once and a warning is written to the operator log;
    - **different definitions** (e.g. a different `active`, `sslVerify` or secret) are a conflict: no webhooks are changed on GitHub, the `WebhooksSynced` condition is set to `False` (reason `SyncFailed`) with the conflict in its message, and `Ready` becomes `False`.
- **Secrets of inline webhooks must live in the Repository's namespace.** `secret.namespace` may only be omitted or set to the Repository's namespace; other values are rejected by the validation webhook and by the reconciler. Prefer `secret` over the plaintext `secretValue`.
- A rotated secret is picked up on the next reconciliation (at the latest with the periodic success requeue); referenced Secrets are not watched.

### Webhook Ignores

Existing webhooks whose URL matches an ignore rule are never deleted by the operator, e.g. webhooks managed by other tools.

```yaml
webhookIgnorePresets:
  - name: ignore-legacy-hooks    # References a WebhookIgnorePreset CR
webhookIgnores:
  - ignoreURLRegex: "^https://legacy\\.acme-corp\\.com/.*"
```

The operator uses the **union** of inline `webhookIgnores` and the referenced `webhookIgnorePresets`. At most **10** inline ignore rules are allowed; each must define a valid `ignoreURLRegex`.

### Autolinks

```yaml
autolinksPresets:
  - name: jira-autolinks         # References an AutolinksPreset CR
autolinks:
  - keyPrefix: "TICKET-"
    urlTemplate: "https://tickets.acme-corp.com/browse/TICKET-<num>"
    isAlphanumeric: false
```

- The operator reconciles the **union** of inline `autolinks` and the autolinks of the referenced `autolinksPresets`. At most **25** inline autolinks are allowed, and their `keyPrefix`es must be unique.
- An autolink is identified by its `keyPrefix` (GitHub requires unique key prefixes per repository). If several definitions share a `keyPrefix`:
    - **identical definitions** are applied once and a warning is written to the operator log;
    - **different definitions** are a conflict: no autolinks are changed on GitHub and the `AutolinksSynced` condition is set to `False` (reason `SyncFailed`). This also applies to conflicts between two `AutolinksPreset`s.

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
        checks:
          - context: "ci/build"
          - context: "ci/test"
      deletion: true           # Prevent branch deletion
      nonFastForward: true     # Prevent force-push
```

Inline rulesets support exactly the same fields as a `RulesetPreset` spec. Their behaviour:

- The operator reconciles the **union** of inline `rulesets` and the referenced `rulesetPresets`.
- At most **10** inline rulesets are allowed, and their names must be unique. Larger or shared sets of rules belong in `RulesetPreset`s.
- If an inline ruleset and a referenced preset (or two referenced presets) have the same name:
    - **identical definitions** are applied once and a warning is written to the operator log;
    - **different definitions** are a conflict: no rulesets are changed on GitHub, the `RulesetsSynced` condition is set to `False` (reason `SyncFailed`) with the conflict in its message, and `Ready` becomes `False`.
- Rulesets with `target: repository` (inline or from presets) are only supported at organization level. At repository level they are filtered out, i.e. neither checked nor applied.

#### Extending rulesets

A `RulesetPreset` or an inline ruleset can **extend** another `RulesetPreset` (in the same namespace) via
`extends` and only state its differences. This avoids copying a whole preset just to add a bypass actor or a
status check:

```yaml
apiVersion: github.interhyp.de/v1alpha1
kind: RulesetPreset
metadata:
  name: default-branch-protection-release-bot
spec:
  name: default-branch-protection-release-bot
  extends: default-branch-protection    # parent preset, all its settings are inherited
  bypassActors:                         # added to the parent's bypass actors
    - actorType: Integration
      actorSlug: release-bot
      bypassMode: always
  rules:
    requiredStatusChecks:
      checks:                           # added to the parent's checks
        - context: "build / build"
```

The resolved ruleset is computed with fixed rules per field; there is no way to remove inherited values
(restructure the chain instead, e.g. extract a smaller base):

| Field | Merge behaviour |
|-------|-----------------|
| `name`, `extends` | Always the child's own value, never inherited |
| `target`, `enforcement`, boolean rules, `requiredApprovingReviewCount`, `strictPolicy`, … | Inherited unless set by the child (`0`/unset counts as "not set") |
| `conditions`, `allowedMergeMethods`, `*Pattern` rules | Replaced as a whole if set by the child |
| `rules`, `rules.pullRequest`, `rules.requiredStatusChecks`, `rules.copilotReview`, `rules.workflows` | Merged field by field |
| `bypassActors` | Union; same `actorType` + slug/id → child entry wins |
| `requiredStatusChecks.checks` | Union; same `context` + `appSlug`/`integrationId` → applied once (the same check from different apps is kept separately) |
| `pullRequest.requiredReviewers` | Union; same reviewer type + slug/id → child entry wins |
| `workflows.workflows` | Union; same repository + path → child entry wins |

- Chains may be up to **5** levels deep. `target` defaults to `branch` after resolution; `enforcement` and
  `rules` are only required on rulesets without `extends`.
- Resolution happens in memory from the operator cache and is fast. Changing a preset reconciles all
  Repositories/Organizations using it directly, via an inline `extends`, or via any preset extending it.
- **Fail closed**: if a chain cannot be resolved (missing parent, cycle, too deep, no enforcement), only that
  ruleset is skipped. The existing GitHub ruleset is neither updated nor deleted, all other rulesets are still
  synced, and `RulesetsSynced` becomes `False` with the reason in its message. The resource is not retried with
  backoff; it is reconciled again as soon as a preset in the chain is created or changed (or the resource itself
  changes). Transient read errors are retried as usual. A directly referenced preset that does not exist still
  fails the whole ruleset sync, as before.
- A preset extending itself is rejected at admission. A missing parent is deliberately **not** rejected, because
  GitOps tools may apply a child before its parent; the ruleset is skipped until the parent exists.

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
