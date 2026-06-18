# Plan: Multi-GitHub-App Support

## Goal

Rework GitHub client creation so the operator supports **multiple GitHub Apps**, each identified by its own Kubernetes Secret containing `app-id` and `private-key`. All credential secrets must reside in a **single namespace** configured via the `APP_CREDENTIALS_SECRET_NAMESPACE` env var (defaulted to the controller's namespace in the Helm chart). The secret to use is referenced **by name** in each `Organization` spec via a `GitHubAppConfig` struct that also contains the installation ID. When a credential secret is updated, the operator automatically invalidates affected cached clients and credentials. Clients remain cached per organization. Metrics, rate-limit handling, and caching continue to work as they do today.

---

## Current Architecture (Summary)

| Component | Current Behaviour |
|---|---|
| **Secret** | Single secret, name passed via `--app-credentials-secret-name` flag, namespace from `APP_CREDENTIALS_SECRET_NAMESPACE` env var. Fetched lazily on first client creation via `SecretProviderFunc`. Parsed `AppCredentials` (AppID + RSA key) are stored **once** on the factory. |
| **Organization CRD** | Contains `spec.githubAppInstallationId` (int64) — the GitHub App *installation* ID for this org. Separate `GitHubAppCredentials` type exists but is **not** wired into the spec. |
| **`CachingGitHubClientFactory`** | Holds a single `*AppCredentials`, a shared `rateLimitState`, and a `map[string]*ClientInfo` cache keyed by org name. `createClient()` always uses the stored single credentials. |
| **Middleware stack** | `http.DefaultTransport → PrimaryRateLimiter (shared state) → SecondaryRateLimiter → AuthorizeGitHubAccess (per-installation JWT) → Retry → Pagination → OTel`. The rate-limit state is shared across all clients by design. |
| **Reconciler factory** | Calls `ClientManager.GetGitHubClientAndCheckRateLimit(orgName, installationID, threshold)`. Resolves `installationID` from the `Organization` CRD for repos and teams. |
| **Repository webhook** | Uses `GitHubClientManager.GetClient(orgName, installationID)` to fetch custom property definitions during validation. |
| **Rate limiting** | Three layers: HTTP middleware (shared `rateLimitState`), pre-reconciliation budget check (`GetRateLimit()`), controller work-queue token bucket (`GitHubRateLimiter`). |

### Key Constraint

The current `AuthorizeGitHubAccessOptions` transport embeds a **single** `appID + privateKey`. When a second GitHub App is introduced, a client with a different app's credentials must use a different transport. This requires that the cache key and credential lookup are tied together.

---

## Proposed Design

### 1. Single Secrets Namespace

All GitHub App credential secrets must reside in **one namespace**. This namespace is configured via the existing `APP_CREDENTIALS_SECRET_NAMESPACE` env var (reused, not renamed).

**Why single-namespace:**
- RBAC can be scoped to one namespace instead of cluster-wide — no need for a `ClusterRole` granting Secret access.
- The manager cache only needs to include one additional namespace for Secret watches — simple cache configuration.
- Secret watch/invalidation is straightforward — watch Secrets in one known namespace, not arbitrary namespaces.
- Consistent with the current architecture — the env var already exists and works.

**Helm chart default:** The Helm chart sets `APP_CREDENTIALS_SECRET_NAMESPACE` to the release namespace (controller's own namespace) via the Downward API:

```yaml
env:
- name: APP_CREDENTIALS_SECRET_NAMESPACE
  valueFrom:
    fieldRef:
      fieldPath: metadata.namespace
```

> **TODO (kustomize):** Update `config/manager/manager.yaml` to use the Downward API for `APP_CREDENTIALS_SECRET_NAMESPACE` instead of a hardcoded value. Currently it is hardcoded to `git-hubby-system`.

### 2. CRD Changes — `OrganizationSpec`

Combine the existing top-level `GitHubAppInstallationId` field and the (currently unused) `GitHubAppCredentials` type into a single `GitHubAppConfig` struct. This groups all GitHub App identity and authentication concerns in one place.

#### New types

```go
// GitHubAppConfig defines the GitHub App identity and credentials for authenticating with the GitHub API.
// It groups the installation ID (which identifies the app's installation in a specific organization)
// with the name of a Kubernetes Secret that holds the app's credentials.
// The secret must reside in the namespace configured via APP_CREDENTIALS_SECRET_NAMESPACE.
type GitHubAppConfig struct {
    // InstallationId is the numeric ID of the GitHub App installation for this organization.
    // This is used to authenticate API requests to GitHub. You can find this ID in your GitHub App's
    // installation settings or via the GitHub API.
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Minimum=1
    InstallationId int64 `json:"installationId"`

    // CredentialsSecretName is the name of a Kubernetes Secret containing GitHub App credentials.
    // The secret must reside in the namespace configured via APP_CREDENTIALS_SECRET_NAMESPACE
    // and must contain the following keys:
    // - app-id: The GitHub App ID
    // - private-key: The GitHub App private key in PEM format
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:MinLength=1
    CredentialsSecretName string `json:"credentialsSecretName"`
}
```

Note: No `SecretReference` struct needed — just a plain `string` for the secret name. The namespace is always `APP_CREDENTIALS_SECRET_NAMESPACE`.

#### Updated `OrganizationSpec`

Both the old and new fields coexist during the deprecation period. The new `GitHubAppConfig` field is optional; when absent, the reconciler falls back to the legacy fields.

```go
type OrganizationSpec struct {
    // Name is the GitHub organization name (also known as the organization login).
    // +kubebuilder:validation:Required
    Name string `json:"name"`

    // GitHubAppConfig defines the GitHub App used to authenticate against this organization.
    // It includes the installation ID and the name of the credentials secret.
    // When set, this takes precedence over the legacy GitHubAppInstallationId field.
    // +optional
    GitHubAppConfig *GitHubAppConfig `json:"githubAppConfig,omitempty"`

    // GitHubAppInstallationId is the numeric ID of the GitHub App installation for this organization.
    // Deprecated: Use GitHubAppConfig.InstallationId instead. This field will be removed in a future release.
    // When GitHubAppConfig is set, this field is ignored.
    // +kubebuilder:validation:Minimum=1
    // +optional
    GitHubAppInstallationId *int64 `json:"githubAppInstallationId,omitempty"`

    // ...remaining fields unchanged (CustomProperties, ActionsSettings, etc.)...
}
```

Key design decisions:
- `GitHubAppConfig` is a **pointer** (`*GitHubAppConfig`) so `omitempty` works correctly and we can distinguish "not set" from "zero value".
- `GitHubAppInstallationId` changes from `int64` (required) to `*int64` (optional) so it can be omitted once the user migrates.
- Neither field is individually required at the CRD level; validation is handled in the webhook (see below).

#### Deprecated types and fields

| Deprecated (keep for now) | Replacement | Removal timeline |
|---|---|---|
| `OrganizationSpec.GitHubAppInstallationId` field | `GitHubAppConfig.InstallationId` | Next minor release after this one |
| `GitHubAppCredentials` struct | `GitHubAppConfig.CredentialsSecretName` (plain string) | Same — remove when old field is removed |
| `--app-credentials-secret-name` flag | `spec.githubAppConfig.credentialsSecretName` per-org | Keep as fallback during deprecation window |

#### Webhook Validation

The Organization webhook enforces that **at least one** of the two paths is configured:

```go
func validateGitHubAppConfig(spec *OrganizationSpec, fldPath *field.Path) field.ErrorList {
    var errs field.ErrorList
    if spec.GitHubAppConfig != nil {
        // New-style: validate the GitHubAppConfig struct
        if spec.GitHubAppConfig.InstallationId < 1 {
            errs = append(errs, field.Invalid(
                fldPath.Child("githubAppConfig", "installationId"),
                spec.GitHubAppConfig.InstallationId,
                "must be >= 1"))
        }
        if spec.GitHubAppConfig.CredentialsSecretName == "" {
            errs = append(errs, field.Required(
                fldPath.Child("githubAppConfig", "credentialsSecretName"),
                "secret name is required"))
        }
    } else if spec.GitHubAppInstallationId == nil || *spec.GitHubAppInstallationId < 1 {
        // Legacy: must have a valid installation ID
        errs = append(errs, field.Required(
            fldPath.Child("githubAppInstallationId"),
            "either githubAppConfig or githubAppInstallationId must be set"))
    }
    return errs
}
```

#### Migration / Backwards Compatibility

This is a **non-breaking, phased migration**. Existing CRs continue to work without changes.

**Phase A — This release (both fields supported):**

Existing CRs are unchanged and continue to work. The operator uses the legacy `--app-credentials-secret-name` flag and `APP_CREDENTIALS_SECRET_NAMESPACE` env var to resolve credentials for orgs using the old field. A deprecation warning is logged.

```yaml
# Existing CR — still works, no changes required
spec:
  name: my-org
  githubAppInstallationId: 12345
```

Users who want multi-app support or per-org credentials can start using the new field:

```yaml
# New-style CR — references a specific secret by name
spec:
  name: my-org
  githubAppConfig:
    installationId: 12345
    credentialsSecretName: git-hubby-app-credentials
```

```yaml
# Multi-app example — two orgs, two different GitHub Apps
---
spec:
  name: org-alpha
  githubAppConfig:
    installationId: 11111
    credentialsSecretName: github-app-alpha
---
spec:
  name: org-beta
  githubAppConfig:
    installationId: 22222
    credentialsSecretName: github-app-beta
```

If both `githubAppConfig` and `githubAppInstallationId` are set, `githubAppConfig` wins. A warning is logged.

**Phase B — Next minor release (old field removed):**

- Remove `OrganizationSpec.GitHubAppInstallationId`.
- Make `OrganizationSpec.GitHubAppConfig` required (`+kubebuilder:validation:Required`, non-pointer).
- Remove `--app-credentials-secret-name` flag.
- Remove the old `GitHubAppCredentials` type.

**Migration steps for users (at their own pace during Phase A):**

1. Ensure the credentials secret(s) exist in the `APP_CREDENTIALS_SECRET_NAMESPACE` namespace.
2. Update each `Organization` CR:
   - Add `spec.githubAppConfig.installationId` with the same value as `spec.githubAppInstallationId`.
   - Add `spec.githubAppConfig.credentialsSecretName` pointing to the credentials secret.
   - Remove `spec.githubAppInstallationId` (optional — it is ignored when `githubAppConfig` is set).
3. Optionally remove the `--app-credentials-secret-name` flag (only needed for legacy fallback).

A **migration guide** in the release notes should document these steps.

### 3. Secret Discovery

Secrets are looked up by **name** in the configured `APP_CREDENTIALS_SECRET_NAMESPACE`. The factory receives a `SecretProvider` that knows the namespace:

```go
// SecretProviderFunc fetches a Kubernetes Secret by name from the configured secrets namespace.
type SecretProviderFunc = func(ctx context.Context, secretName string) (*v1.Secret, error)
```

In `cmd/main.go`, the lambda captures the fixed namespace:

```go
secretsNamespace := os.Getenv("APP_CREDENTIALS_SECRET_NAMESPACE")

fetchSecret := func(ctx context.Context, secretName string) (*v1.Secret, error) {
    var secret v1.Secret
    key := client.ObjectKey{
        Name:      secretName,
        Namespace: secretsNamespace,
    }
    if err := directClient.Get(ctx, key, &secret); err != nil {
        return nil, err
    }
    return &secret, nil
}
```

### 4. Credentials Cache

Replace the single `*AppCredentials` field with a **map** keyed by secret name:

```go
type CachingGitHubClientFactory struct {
    mu              sync.RWMutex
    clients         map[string]*ClientInfo      // keyed by org name
    credentials     map[string]*AppCredentials  // keyed by secret name
    config          *ClientConfig
    secretProvider  SecretProviderFunc
    rateLimitStates map[int64]*github_primary_ratelimit.RateLimitState // keyed by appID
}
```

The `ClientInfo` struct is extended to track which secret was used:

```go
type ClientInfo struct {
    Client         *GitHubClientWrapper
    InstallationID int64
    CacheKey       string
    SecretName     string  // secret name — used for invalidation
}
```

- On `GetClient(ctx, cacheKey, app)`:
  1. Check `clients[cacheKey]` — return if hit.
  2. Lock, then check `credentials[app.CredentialsSecretName]`:
     - If missing: call `secretProvider(ctx, app.CredentialsSecretName)`, parse, store.
  3. Build the middleware stack using the resolved `*AppCredentials` + `app.InstallationId`.
  4. Store the new `ClientInfo` in `clients[cacheKey]` (including `SecretName`).

This approach:

- Still fetches + parses each secret **at most once** (lazy, cached).
- Multiple orgs can share the same secret (same GitHub App, different installations).
- Different orgs can reference different secrets (different GitHub Apps).
- Tracks which secret each client uses, enabling targeted invalidation.

### 5. Interface Changes

#### `GetClient` / `GetGitHubClientAndCheckRateLimit`

Both methods accept the `GitHubAppConfig` struct as a single parameter instead of spreading its fields. This keeps the signatures stable if the struct gains fields later and reduces parameter count:

```go
// ghclient — accepts the resolved GitHubAppConfig
GetClient(ctx context.Context, cacheKey string, app v1alpha1.GitHubAppConfig) (GitHubClient, error)
GetGitHubClientAndCheckRateLimit(ctx context.Context, cacheKey string, app v1alpha1.GitHubAppConfig, rateLimitMinimum int) (GitHubClient, error)

// reconciler.GitHubClientManager interface
GetGitHubClientAndCheckRateLimit(ctx context.Context, cacheKey string, app v1alpha1.GitHubAppConfig, rateLimitMinimum int) (ghclient.GitHubClient, error)

// webhook v1alpha1.GitHubClientManager interface
GetClient(ctx context.Context, cacheKey string, app v1alpha1.GitHubAppConfig) (ghclient.GitHubClient, error)
```

The factory internally reads `app.InstallationId` and `app.CredentialsSecretName` from the struct.

### 6. Reconciler Factory Changes

All three `CreateForXxx` methods already resolve an `Organization` CRD. They call `org.ResolveGitHubAppConfig()` which handles both new and legacy config:

```go
// CreateForOrg
resolvedApp, err := org.ResolveGitHubAppConfig(f.LegacySecretName)
if err != nil {
    return nil, err
}
if org.Spec.GitHubAppConfig == nil {
    log.Info("Organization uses deprecated githubAppInstallationId; migrate to spec.githubAppConfig",
        "organization", org.Spec.Name)
}
ghClient, err := f.ClientManager.GetGitHubClientAndCheckRateLimit(
    ctx,
    org.Spec.Name,
    *resolvedApp,
    orgRateLimitThreshold,
)

// CreateForRepo — org already fetched, same pattern
// CreateForTeam / buildGitHubOrgsSlice — same pattern per org
```

The resolution logic lives as a method on `Organization` in `api/v1alpha1/organization_methods.go`:

```go
// ResolveGitHubAppConfig returns a fully resolved GitHubAppConfig from the Organization spec.
// It supports both the new GitHubAppConfig struct and the legacy GitHubAppInstallationId field.
// If both are set, GitHubAppConfig takes precedence.
// The legacySecretName parameter is the value from the deprecated --app-credentials-secret-name flag.
func (o *Organization) ResolveGitHubAppConfig(legacySecretName string) (*GitHubAppConfig, error) {
    if o.Spec.GitHubAppConfig != nil {
        return o.Spec.GitHubAppConfig, nil
    }

    if o.Spec.GitHubAppInstallationId == nil {
        return nil, fmt.Errorf("organization %s has neither githubAppConfig nor githubAppInstallationId set", o.Spec.Name)
    }

    if legacySecretName == "" {
        return nil, fmt.Errorf(
            "organization %s uses deprecated githubAppInstallationId but --app-credentials-secret-name flag is not set; "+
                "migrate to spec.githubAppConfig", o.Spec.Name)
    }

    return &GitHubAppConfig{
        InstallationId:        *o.Spec.GitHubAppInstallationId,
        CredentialsSecretName: legacySecretName,
    }, nil
}
```

The reconciler factory calls it:

```go
// CreateForOrg
resolvedApp, err := org.ResolveGitHubAppConfig(f.LegacySecretName)
if err != nil {
    return nil, err
}
ghClient, err := f.ClientManager.GetGitHubClientAndCheckRateLimit(
    ctx,
    org.Spec.Name,
    *resolvedApp,
    orgRateLimitThreshold,
)
```

Deprecation warnings are logged by the caller (factory/webhook) when `org.Spec.GitHubAppConfig == nil && org.Spec.GitHubAppInstallationId != nil`, not inside the method itself — keeping the method free of logging dependencies.

The `Factory` struct gets a new field:

```go
type Factory struct {
    ClientManager    reconciler.GitHubClientManager
    K8sClient        client.Client
    SpreadingManager reconciler.SpreadManager
    LegacySecretName string  // from --app-credentials-secret-name flag (deprecated)
}
```

### 7. Repository Webhook Changes

`SetupRepositoryWebhookWithManager` passes the `GitHubClientManager` (the `CachingGitHubClientFactory`). Inside `validateRepository`, the org is already fetched. The webhook validator uses the same resolution logic:

```go
type RepositoryCustomValidator struct {
    K8sClient            client.Client
    GitHubClientManager  GitHubClientManager
    LegacySecretName     string  // from --app-credentials-secret-name flag (deprecated)
}
```

```go
resolvedApp, err := org.ResolveGitHubAppConfig(v.LegacySecretName)
if err != nil {
    return fmt.Errorf("failed to resolve GitHub App config for organization %s: %w", org.Spec.Name, err)
}
githubClient, err := v.GitHubClientManager.GetClient(
    ctx,
    org.Spec.Name,
    *resolvedApp,
)
```

The `ResolveGitHubAppConfig` method on `Organization` is shared between the webhook and factory — no duplication.

### 8. Rate-Limit State Sharing

The shared `rateLimitState` (`github_primary_ratelimit.RateLimitState`) is **per GitHub App**, not per organization. Two orgs using the same GitHub App share a rate limit budget. Two different GitHub Apps have independent budgets.

Change the rate-limit state to be **keyed by App ID** (extracted from the parsed credentials):

```go
type CachingGitHubClientFactory struct {
    // ...
    rateLimitStates map[int64]*github_primary_ratelimit.RateLimitState // keyed by appID
}
```

In `buildMiddlewareStack`:

```go
func (m *CachingGitHubClientFactory) buildMiddlewareStack(..., creds *AppCredentials) http.RoundTripper {
    rt := http.DefaultTransport

    if state, ok := m.rateLimitStates[creds.AppID]; ok {
        rt = github_ratelimit.New(rt, github_primary_ratelimit.WithSharedState(state))
    } else {
        primary := github_ratelimit.NewPrimaryLimiter(rt)
        m.rateLimitStates[creds.AppID] = primary.GetState()
        rt = github_ratelimit.NewSecondaryLimiter(primary)
    }

    rt = AuthorizeGitHubAccess(rt, creds.AppID, appInstallationID, creds.PrivateKey)
    // ... rest unchanged
}
```

This preserves the current behaviour (all clients for the same GitHub App share HTTP-level rate-limit awareness) while correctly isolating apps with different rate-limit budgets.

### 9. Controller Work-Queue Rate Limiter

The global `GitHubRateLimiter` in the controller work queue (`ratelimit.GitHubRateLimiter`) is **not per-app** — it is a general throughput throttle. It should remain a single global instance as today. When a `RateLimitedError` is returned by `GetGitHubClientAndCheckRateLimit`, the controller already blocks the work queue via `handleRequeueError`. No changes needed here.

### 10. Cache Invalidation on Secret Update

When a credential secret is updated (e.g., key rotation), the operator must automatically invalidate the cached credentials **and** all clients that used them, so the next reconciliation picks up the new credentials.

#### Invalidation method on `CachingGitHubClientFactory`

```go
// InvalidateCredentials removes cached credentials for a specific secret
// and all clients that were built using those credentials.
func (m *CachingGitHubClientFactory) InvalidateCredentials(secretName string) {
    m.mu.Lock()
    defer m.mu.Unlock()

    delete(m.credentials, secretName)

    // Remove all clients that were built from this secret
    for cacheKey, info := range m.clients {
        if info.SecretName == secretName {
            delete(m.clients, cacheKey)
        }
    }
    logf.Log.Info("Invalidated credentials and associated clients", "secretName", secretName)
}
```

Note: The `rateLimitStates` map is **not** invalidated. Rate-limit state is per-App-ID and is orthogonal to credential rotation (the App ID doesn't change when the private key is rotated). If a secret is replaced with credentials for an entirely different GitHub App (different App ID), a new rate-limit state will be created automatically on the next client build.

#### Secret watch in the Organization controller

The Organization controller already uses `Watches()` with `handler.EnqueueRequestsFromMapFunc` to watch sub-resources (CodeSecurityConfiguration, RulesetPreset). The same pattern is used to watch `v1.Secret` changes.

Since all credential secrets live in the **single `APP_CREDENTIALS_SECRET_NAMESPACE`**, the approach is simple:

1. Add `APP_CREDENTIALS_SECRET_NAMESPACE` to the manager cache's `DefaultNamespaces` so Secrets in that namespace are watchable.
2. Add a `Watches()` call on the Organization controller for `v1.Secret`.

**Cache configuration** in `cmd/main.go`:

```go
// Add the secrets namespace to the cache so we can watch secrets there
secretsNamespace := os.Getenv("APP_CREDENTIALS_SECRET_NAMESPACE")
if _, exists := defaultNamespaces[secretsNamespace]; !exists {
    defaultNamespaces[secretsNamespace] = cache.Config{}
}
```

**Secret watch** on the Organization controller:

```go
Watches(
    &v1.Secret{},
    handler.EnqueueRequestsFromMapFunc(r.findOrganizationsForSecret),
    builder.WithPredicates(secretDataChangedPredicate),
)
```

Where `findOrganizationsForSecret` lists all Organizations and returns those whose `spec.githubAppConfig.credentialsSecretName` matches the changed secret:

```go
func (r *OrganizationCtl) findOrganizationsForSecret(ctx context.Context, obj client.Object) []reconcile.Request {
    secret := obj.(*v1.Secret)

    // Invalidate credentials cache for this secret
    r.ClientFactory.InvalidateCredentials(secret.Name)

    // Find all organizations referencing this secret
    var orgList githubv1alpha1.OrganizationList
    if err := r.ReconcilerFactory.K8sClient.List(ctx, &orgList); err != nil {
        logf.FromContext(ctx).Error(err, "Failed to list organizations for secret change")
        return nil
    }

    var requests []reconcile.Request
    for _, org := range orgList.Items {
        if org.Spec.GitHubAppConfig != nil && org.Spec.GitHubAppConfig.CredentialsSecretName == secret.Name {
            requests = append(requests, reconcile.Request{
                NamespacedName: types.NamespacedName{
                    Name:      org.Name,
                    Namespace: org.Namespace,
                },
            })
        }
        // Also handle legacy orgs using the global secret
        if org.Spec.GitHubAppConfig == nil && secret.Name == r.LegacySecretName {
            requests = append(requests, reconcile.Request{
                NamespacedName: types.NamespacedName{
                    Name:      org.Name,
                    Namespace: org.Namespace,
                },
            })
        }
    }

    logf.FromContext(ctx).Info("Secret changed, requeuing affected organizations",
        "secret", secret.Name, "orgsAffected", len(requests))
    return requests
}
```

#### Event filtering for Secrets

To avoid unnecessary cache invalidation and reconciliation, the Secret watch uses a custom predicate that filters on `Data` changes:

```go
secretDataChangedPredicate := predicate.Funcs{
    UpdateFunc: func(e event.UpdateEvent) bool {
        oldSecret := e.ObjectOld.(*v1.Secret)
        newSecret := e.ObjectNew.(*v1.Secret)
        return !reflect.DeepEqual(oldSecret.Data, newSecret.Data)
    },
    CreateFunc:  func(e event.CreateEvent) bool { return false },  // don't react to creates
    DeleteFunc:  func(e event.DeleteEvent) bool { return true },   // react to deletes
    GenericFunc: func(e event.GenericEvent) bool { return false },
}
```

This ensures that annotation-only or label-only changes on secrets don't trigger unnecessary client invalidation.

### 11. `cmd/main.go` Changes

| Current | Phase A (this release) | Phase B (next release) |
|---|---|---|
| `--app-credentials-secret-name` flag | **Keep** — used as fallback for legacy CRs | Remove |
| `APP_CREDENTIALS_SECRET_NAMESPACE` env var | **Keep** — now also used for new-style secret lookups | Keep |
| `fetchGitHubAppSecret` closure (hardcoded name) | Replace with generic `fetchSecret(ctx, secretName)` closure (namespace captured from env) | No change |

> **TODO (kustomize):** Update `config/manager/manager.yaml` to use Downward API for `APP_CREDENTIALS_SECRET_NAMESPACE` instead of the current hardcoded `git-hubby-system` value:
> ```yaml
> - name: APP_CREDENTIALS_SECRET_NAMESPACE
>   valueFrom:
>     fieldRef:
>       fieldPath: metadata.namespace
> ```

The `APP_CREDENTIALS_SECRET_NAMESPACE` namespace is added to the manager cache for Secret watches:

```go
secretsNamespace := os.Getenv("APP_CREDENTIALS_SECRET_NAMESPACE")

// Add to cache so we can watch secrets there
if _, exists := defaultNamespaces[secretsNamespace]; !exists {
    defaultNamespaces[secretsNamespace] = cache.Config{}
}
```

The legacy `--app-credentials-secret-name` flag value is passed to the factory and webhook as `LegacySecretName`:

```go
reconcilerFactory := &reconcilerfactory.Factory{
    ClientManager:    clientManager,
    SpreadingManager: spreadingManager,
    K8sClient:        mgr.GetClient(),
    LegacySecretName: appCredentialsSecretName,  // from flag (may be empty in Phase B)
}
```

### 12. RBAC Changes

Since all secrets live in the **single `APP_CREDENTIALS_SECRET_NAMESPACE`**, RBAC can be **namespace-scoped** (not cluster-wide). The existing RBAC binding for this namespace is already in place.

The required permissions are `get`, `list`, and `watch` (list + watch needed for the cache informer):

```go
// +kubebuilder:rbac:groups="",namespace=system,resources=secrets,verbs=get;list;watch
```

Note: The `namespace=system` is a placeholder — the actual namespace is substituted by kustomize namePrefix. This is the same pattern used for the existing `app_credentials_role_binding.yaml`.

Since the operator already has a `RoleBinding` in `APP_CREDENTIALS_SECRET_NAMESPACE` for `get` on Secrets, we only need to **extend** it to include `list` and `watch`.

---

## Affected Files

| File | Change |
|---|---|
| `api/v1alpha1/organization_types.go` | Keep `GitHubAppInstallationId` as optional/deprecated `*int64`. Keep `GitHubAppCredentials` (deprecated). Add `GitHubAppConfig` type and optional `OrganizationSpec.GitHubAppConfig *GitHubAppConfig` field. |
| `api/v1alpha1/organization_methods.go` | Add `ResolveGitHubAppConfig(legacySecretName string) (*GitHubAppConfig, error)` method on `Organization` |
| `api/v1alpha1/zz_generated.deepcopy.go` | Regenerated by `make generate` |
| `config/crd/bases/github.interhyp.de_organizations.yaml` | Regenerated by `make manifests` — both `githubAppInstallationId` and `githubAppConfig` present |
| `config/samples/` | Update sample Organization CRs to show **both** old and new style; recommend new style |
| `internal/ghclient/factory.go` | Multi-credential cache (`map[string]*AppCredentials` keyed by secret name), updated `GetClient`/`GetGitHubClientAndCheckRateLimit` signatures (accept `GitHubAppConfig` struct), per-app `rateLimitStates` map, `SecretProviderFunc` takes `(ctx, secretName)`, add `InvalidateCredentials()` method, extend `ClientInfo` with `SecretName` |
| `internal/ghclient/interface.go` | No changes (this is the `GitHubClient` interface, not the factory interface) |
| `internal/ghclient/transport.go` | No changes (already parameterised by appID + key) |
| `internal/ghclient/mock.go` | Update mock to match new `GetClient`/`GetGitHubClientAndCheckRateLimit` signatures |
| `internal/reconciler/types.go` | Update `GitHubClientManager` interface signature (accept `GitHubAppConfig` struct instead of individual fields) |
| `internal/reconciler/reconcilerfactory/factory.go` | Add `LegacySecretName` field. Call `org.ResolveGitHubAppConfig()` and pass resolved `GitHubAppConfig` struct to `GetGitHubClientAndCheckRateLimit` |
| `internal/controller/organization_controller.go` | Add `LegacySecretName` and `ClientFactory` fields; add `Watches()` for `v1.Secret` with `findOrganizationsForSecret` mapFunc and data-change predicate |
| `internal/controller/shared.go` | No changes |
| `internal/webhook/v1alpha1/repository_webhook.go` | Add `LegacySecretName` field to `RepositoryCustomValidator`, update `GetClient` call using resolved `GitHubAppConfig` |
| `internal/webhook/v1alpha1/organization_webhook.go` | Add validation for `githubAppConfig` / `githubAppInstallationId` mutual presence |
| `internal/webhook/v1alpha1/organization_webhook_test.go` | Add tests for both old-style and new-style Organization specs, and both-set scenario |
| `internal/webhook/v1alpha1/repository_webhook_test.go` | Add tests with new-style Organization |
| `internal/reconciler/executor_test.go` | Keep existing `GitHubAppInstallationId` tests (they still work); add parallel tests using `GitHubAppConfig` struct |
| `cmd/main.go` | Keep `--app-credentials-secret-name` flag (deprecated) and `APP_CREDENTIALS_SECRET_NAMESPACE` env var; add secrets namespace to cache config; replace `fetchGitHubAppSecret` with generic `fetchSecret(ctx, secretName)` closure; pass legacy config to factory/controllers/webhook |
| `config/rbac/` | Extend existing Secret Role in `APP_CREDENTIALS_SECRET_NAMESPACE` to include `list;watch` (currently only `get`) |
| `chart/` (Helm chart) | Update `APP_CREDENTIALS_SECRET_NAMESPACE` to use Downward API; keep `appCredentialsSecretName` (deprecated); update CRD templates (regenerated) |

> **TODO (kustomize):** Update `config/manager/manager.yaml` to default `APP_CREDENTIALS_SECRET_NAMESPACE` via Downward API.

---

## Implementation Steps

### Phase 1 — CRD & API Changes

1. Define new `GitHubAppConfig` struct (with `InstallationId` + `CredentialsSecretName`) in `api/v1alpha1/organization_types.go`.
2. **Keep** the old `GitHubAppCredentials` type (mark as deprecated in comments).
3. Change `OrganizationSpec.GitHubAppInstallationId` from `int64` (required) to `*int64` (optional, deprecated).
4. Add `OrganizationSpec.GitHubAppConfig *GitHubAppConfig` as optional field.
5. Run `make generate && make manifests` to regenerate deepcopy and CRDs.
6. Update sample CRs in `config/samples/` to show both old and new style.
7. Add webhook validation: at least one of `githubAppConfig` or `githubAppInstallationId` must be set.

### Phase 2 — Client Factory Refactor

8. Change `SecretProviderFunc` signature to `func(ctx, secretName string) (*v1.Secret, error)`.
9. Replace `credentials *AppCredentials` with `credentials map[string]*AppCredentials` in `CachingGitHubClientFactory` (keyed by secret name).
10. Replace `rateLimitState *RateLimitState` with `rateLimitStates map[int64]*RateLimitState` keyed by App ID.
11. Extend `ClientInfo` with `SecretName string` to track which secret was used.
12. Update `GetClient()` to accept `app v1alpha1.GitHubAppConfig` struct, look up (or lazily fetch + parse) the correct `AppCredentials`.
13. Update `GetGitHubClientAndCheckRateLimit()` similarly.
14. Update `buildMiddlewareStack()` to accept `*AppCredentials` and use per-app rate-limit state.
15. Update `createClient()` to extract secret name from the `GitHubAppConfig` struct and resolve credentials.
16. Add `InvalidateCredentials(secretName string)` method that removes cached credentials and all clients built from them.

### Phase 3 — Interface & Consumer Updates

17. Update `reconciler.GitHubClientManager` interface in `internal/reconciler/types.go`.
18. Add `ResolveGitHubAppConfig(legacySecretName string) (*GitHubAppConfig, error)` method to `Organization` in `api/v1alpha1/organization_methods.go`.
19. Add `LegacySecretName` field to `reconcilerfactory.Factory`.
20. Update all call sites in `internal/reconciler/reconcilerfactory/factory.go`:
    - `CreateForOrg`, `CreateForRepo`, `CreateForTeam`, `buildGitHubOrgsSlice`.
    - Call `org.ResolveGitHubAppConfig(f.LegacySecretName)` to handle both old and new config.
    - Pass resolved `GitHubAppConfig` struct to client manager.
21. Update `webhook/v1alpha1.GitHubClientManager` interface, add `LegacySecretName` to `RepositoryCustomValidator`, call `org.ResolveGitHubAppConfig()` in `validateRepository()`.
21. Update `cmd/main.go`:
    - **Keep** `--app-credentials-secret-name` flag (deprecated, used for legacy fallback).
    - **Keep** `APP_CREDENTIALS_SECRET_NAMESPACE` env var.
    - Add secrets namespace to manager cache config.
    - Replace `fetchGitHubAppSecret` with generic `fetchSecret(ctx, secretName)` closure.
    - Pass `LegacySecretName` to factory, controllers, and webhook.
22. Extend RBAC Role in `APP_CREDENTIALS_SECRET_NAMESPACE` to include `list;watch` (in addition to existing `get`).

### Phase 4 — Secret Watch & Cache Invalidation

23. Add `LegacySecretName` and `ClientFactory` fields to `OrganizationCtl`.
24. Add `Watches()` for `v1.Secret` to the Organization controller's `SetupWithManager`.
25. Implement `findOrganizationsForSecret` mapFunc that calls `InvalidateCredentials()` and returns affected org reconcile requests.
26. Add `secretDataChangedPredicate` to filter out non-data changes on Secrets.

> **TODO (kustomize):** Update `config/manager/manager.yaml` to use Downward API for `APP_CREDENTIALS_SECRET_NAMESPACE`.

### Phase 5 — Test Updates

27. Update mock client in `internal/ghclient/mock.go` to match new signatures.
28. Keep existing `internal/reconciler/executor_test.go` tests (they use old-style and still work). Add new tests using `GitHubAppConfig` struct.
29. Update `internal/webhook/v1alpha1/organization_webhook_test.go` — add tests for new validation (both-set, only-old, only-new, neither-set).
30. Update `internal/webhook/v1alpha1/repository_webhook_test.go` — add tests with new-style Organization.
31. Update existing factory and reconciler factory tests.
32. Add new tests:
    - Multi-credential caching (two orgs, two different secrets → two `AppCredentials` entries).
    - Same-secret sharing (two orgs, same secret → single `AppCredentials` entry, two `ClientInfo` entries).
    - Per-app rate-limit state isolation.
    - Secret not found error path.
    - `InvalidateCredentials()`: verify credentials and all associated clients are removed.
    - `InvalidateCredentials()`: verify unrelated clients are preserved.
    - Secret watch integration: secret update triggers cache invalidation and org re-reconciliation.
    - **Legacy fallback**: org with only `githubAppInstallationId` uses global secret config.
    - **Precedence**: org with both fields uses `githubAppConfig`, ignores `githubAppInstallationId`.
    - **Deprecation warning**: org using legacy field logs deprecation message.

### Phase 6 — Helm & Docs

33. Update Helm chart: update `APP_CREDENTIALS_SECRET_NAMESPACE` to use Downward API; keep `appCredentialsSecretName` (deprecated).
34. Run `make helm` to regenerate chart from kustomize.
35. Update `README.md` and `copilot-instructions.md` with new multi-app architecture.
36. Write migration guide for existing users — emphasize that migration is **optional** in this release and the old fields still work.

### Phase 7 — Validation & Cleanup

37. Run `make lint-fix && make test` — fix any issues.
38. Run `make test-e2e` on a Kind cluster.
39. Run `make build` to verify the binary builds cleanly.

### Phase 8 — Future: Remove Deprecated Fields (next minor release)

40. Remove `OrganizationSpec.GitHubAppInstallationId` field.
41. Change `OrganizationSpec.GitHubAppConfig` from `*GitHubAppConfig` (optional) to `GitHubAppConfig` (required, `+kubebuilder:validation:Required`).
42. Remove `GitHubAppCredentials` type.
43. Remove `--app-credentials-secret-name` flag.
44. Remove `LegacySecretName` from factory and webhook.
45. Remove legacy fallback path from `Organization.ResolveGitHubAppConfig()` (make it simply return `spec.githubAppConfig`).
46. Update all tests that still use old-style Organization specs.
47. Run `make generate && make manifests && make helm`.
48. Update migration guide to note the field removal.

---

## What Does NOT Change

| Concern | Reason |
|---|---|
| `GitHubClient` interface (`ghclient/interface.go`) | Clients are already per-org; the interface operates on an authenticated client, not credentials. |
| `AuthorizeGitHubAccessOptions` transport (`ghclient/transport.go`) | Already parameterised by `appID`, `installationID`, `privateKey`. |
| Controller work-queue rate limiter (`ratelimit.GitHubRateLimiter`) | Global throttle, not per-app. |
| Spreading (`internal/reconciler/spreading/`) | Unrelated to credential management. |
| Reconciler packages (`orgrec/`, `reporec/`, `teamrec/`) | They receive a ready-made `ghclient.GitHubClient`; never touch credentials. |
| Status conditions, finalizers, executor | Orthogonal to client creation. |
| `APP_CREDENTIALS_SECRET_NAMESPACE` env var | Reused as-is — now serves both legacy and new-style secret lookups. |
| Existing Organization CRs | Old `githubAppInstallationId` field continues to work during deprecation window. No forced migration. |
| Existing tests using `GitHubAppInstallationId` | They still compile and pass — the field is deprecated but present. Updated in Phase 8. |

---

## Risks & Mitigations

| Risk | Mitigation |
|---|---|
| **Dual-field complexity** — two ways to configure the same thing during deprecation window | `Organization.ResolveGitHubAppConfig()` encapsulates all resolution logic. Webhook validates that at least one path is configured. Clear precedence rule: `githubAppConfig` wins. Deprecation warnings logged by callers. |
| **Rate-limit isolation** — different apps have separate rate limits; the old single `rateLimitState` would incorrectly share state | Per-app `rateLimitStates` map keyed by AppID ensures isolation. |
| **Thread safety** — concurrent access to `credentials` map | Already protected by `mu sync.RWMutex` in `CachingGitHubClientFactory`. `InvalidateCredentials()` acquires a write lock. |
| **Webhook latency** — webhook validation fetches an org, then builds a client per-request | No change from today — the client is cached after first creation. The only new overhead is a potential one-time secret fetch for a new app. |
| **Test churn** — ~15+ test sites reference `GitHubAppInstallationId` directly | Existing tests continue to work during the deprecation window. New tests added for the new path. Only Phase 8 (future release) requires updating them. |
| **Thundering herd on secret rotation** — invalidating all clients for a secret at once could cause a burst of reconciliations | The spreading mechanism already distributes reconciliations over time. Additionally, the work-queue rate limiter throttles the burst. |
| **Legacy users never migrate** — old field stays in use indefinitely | Phase 8 sets a firm removal deadline. Deprecation warnings in logs and docs nudge users to migrate. |

---

## Open Questions

1. **How long should the deprecation window be?**
   Recommendation: One minor release cycle. The old fields are kept in the release that introduces `githubAppConfig`, then removed in the next minor release. This gives users one upgrade cycle to migrate.

2. **Should the webhook reject CRs that have both `githubAppConfig` and `githubAppInstallationId` set?**


   Recommendation: No — allow both during the deprecation window to simplify progressive migration. `githubAppConfig` takes precedence; a warning is logged. This lets users add `githubAppConfig` first, then remove the old field in a separate commit.






