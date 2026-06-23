# Organization Configuration

This guide demonstrates how to configure a GitHub Organization using git-hubby.
We use **Acme Corp** as a realistic example throughout.

For the complete field reference, see the [API Documentation](../crds.md#organization).

## Example: Acme Corp Engineering

```yaml
apiVersion: github.interhyp.de/v1alpha1
kind: Organization
metadata:
  name: acme-corp
  namespace: git-hubby-system
spec:
  # --- Identity ---
  login: acme-corp                          # GitHub org slug (immutable)
  name: Acme Corp Engineering               # Display name
  description: "Building the future of e-commerce"
  location: "Munich, Germany"
  website: "https://engineering.acme-corp.com"

  # --- Authentication ---
  githubAppInstallationId: 12345678         # From GitHub App settings
  plan: enterprise                          # enterprise | team | free

  # --- Custom Properties ---
  # Define metadata fields that repositories must/can set
  customProperties:
    - propertyName: team
      valueType: single_select
      required: true
      allowedValues:
        - platform
        - checkout
        - payments
        - logistics
      defaultValue:
        value: platform
      valuesEditableBy: org_and_repo_actors
      description: "Owning team for this repository"

    - propertyName: service-tier
      valueType: single_select
      required: true
      allowedValues:
        - critical      # 99.99% SLA, on-call required
        - standard      # 99.9% SLA
        - experimental  # No SLA
      defaultValue:
        value: standard

    - propertyName: compliance
      valueType: multi_select
      allowedValues:
        - gdpr
        - pci-dss
        - sox
      description: "Applicable compliance frameworks"

    - propertyName: slack-channel
      valueType: string
      description: "Team Slack channel for alerts"

  # --- GitHub Actions ---
  actionsSettings:
    enabledRepositories: all
    allowedActions: selected
    selectedAllowedActions:
      githubOwnedAllowed: true
      verifiedAllowed: true
      patternsAllowed:
        - "docker/*"
        - "actions/*"
        - "acme-corp/*"
    shaPinningRequired: false
    defaultWorkflowPermissions: read
    canApprovePullRequestReviews: false
    artifactAndLogRetentionDays: 90

    runnerGroups:
      - name: production-runners
        visibility: selected
        selectedRepositories:
          - order-service
          - payment-service
        allowsPublicRepositories: false
        restrictedToWorkflows: true
        selectedWorkflows:
          - .github/workflows/deploy-prod.yml

  # --- Branch Protection ---
  rulesetPresets:
    - name: default-branch-protection

  # --- Security ---
  codeSecurityConfigurations:
    - name: standard-security
      attachmentScope: all
```

## Key Concepts

### Custom Properties

Custom properties let you attach structured metadata to all repositories in your organization.
They appear in GitHub's repository settings and can be used for filtering and automation.

**Property Types:**

| Type | Use Case |
|------|----------|
| `single_select` | Enforce one value from a predefined list (e.g., owning team) |
| `multi_select` | Allow multiple values (e.g., compliance frameworks) |
| `string` | Free-form text (e.g., Slack channel) |
| `true_false` | Boolean flags (e.g., production-ready) |

**Editability:**

- `org_actors` (default): Only org admins can set values
- `org_and_repo_actors`: Repo admins can also set values

### Actions Settings

Control which GitHub Actions can run in your organization:

| Setting | Recommended for Enterprise |
|---------|---------------------------|
| `allowedActions: selected` | Restrict to trusted actions only |
| `shaPinningRequired: true` | Prevent tag manipulation attacks |
| `defaultWorkflowPermissions: read` | Least-privilege for GITHUB_TOKEN |
| `canApprovePullRequestReviews: false` | Prevent automated PR approvals |

**Runner Groups** restrict which repositories can use self-hosted runners:

```yaml
runnerGroups:
  - name: production-runners
    visibility: selected
    selectedRepositories:
      - order-service    # Only these repos can use prod runners
    restrictedToWorkflows: true
    selectedWorkflows:
      - .github/workflows/deploy-prod.yml  # Only deploy workflow
```

### Rulesets and Security

Reference shared configurations defined as separate CRDs:

```yaml
# Organization-wide branch protection
rulesetPresets:
  - name: default-branch-protection    # References a RulesetPreset CR

# Security scanning configurations
codeSecurityConfigurations:
  - name: standard-security            # References a CodeSecurityConfiguration CR
    attachmentScope: all               # Apply to all repos
```

**Attachment Scopes** for security configurations:

| Scope | Description |
|-------|-------------|
| `all` | Apply to all repositories |
| `public` | Only public repositories |
| `private_or_internal` | Only private/internal repositories |
| `selected` | Only repos that explicitly reference it |

## Plan Requirements

Some features require specific GitHub plans:

| Feature | Required Plan |
|---------|--------------|
| Organization rulesets | `team` or `enterprise` |
| Code security configurations | `enterprise` |
| Internal repository visibility | `enterprise` |
| Runner groups | `team` or `enterprise` |

## Related Resources

- [Repository Configuration](repository.md) - Configure repositories in this organization
- [API Reference](../crds.md#organization) - Complete field reference
