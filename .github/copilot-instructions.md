# AI Coding Guide for git-hubby

These instructions help AI agents work productively in this **Kubebuilder v4**-based Kubernetes operator that manages GitHub organizations, repositories, and teams via CRDs.

## Architecture & Data Flow
- Operator runs a controller-manager that registers three controllers and webhooks; see [cmd/main.go](cmd/main.go).
- **Core CRDs**: 
  - [api/v1alpha1/organization_types.go](api/v1alpha1/organization_types.go) - GitHub organization management
  - [api/v1alpha1/repository_types.go](api/v1alpha1/repository_types.go) - GitHub repository management
  - [api/v1alpha1/team_types.go](api/v1alpha1/team_types.go) - GitHub team management across organizations
- **Configuration CRDs**:
  - [api/v1alpha1/rulesetpreset_types.go](api/v1alpha1/rulesetpreset_types.go) - Reusable ruleset configurations
  - [api/v1alpha1/webhookpreset_types.go](api/v1alpha1/webhookpreset_types.go) - Reusable webhook configurations
  - [api/v1alpha1/codesecurityconfiguration_types.go](api/v1alpha1/codesecurityconfiguration_types.go) - Standalone code security configurations (referenced by organizations)
- **Controllers delegate to reconcilers** in dedicated packages via factory pattern (see [internal/reconciler/reconcilerfactory/factory.go](internal/reconciler/reconcilerfactory/factory.go)):
  - [internal/reconciler/orgrec](internal/reconciler/orgrec) for organizations - manages org settings, custom properties, rulesets, code security configurations, and actions settings
  - [internal/reconciler/reporec](internal/reconciler/reporec) for repositories - manages repo settings, webhooks, and rulesets
  - [internal/reconciler/teamrec](internal/reconciler/teamrec) for teams - manages team creation, membership, and multi-organization support
- GitHub integration via a cached client factory using GitHub App credentials from a Kubernetes Secret; see [internal/ghclient/factory.go](internal/ghclient/factory.go), [internal/ghclient/wrapper.go](internal/ghclient/wrapper.go), [internal/ghclient/interface.go](internal/ghclient/interface.go).
- Status conditions are set consistently via helpers; see [internal/conditions/conditions.go](internal/conditions/conditions.go). Finalizers gate deletion; see reconciler files.
- **Validation-only webhooks** enforce spec rules (e.g., organization custom properties, repository references); see [internal/webhook/v1alpha1/organization_webhook.go](internal/webhook/v1alpha1/organization_webhook.go) and [internal/webhook/v1alpha1/repository_webhook.go](internal/webhook/v1alpha1/repository_webhook.go). **Mutating webhooks have been removed**; labels are now applied in reconcilers. **Note**: Team CRD has no webhook currently.

## Startup Spreading & Parallel Reconciliation

### Startup Spreading Mechanism
The operator implements a sophisticated startup spreading system ([internal/reconciler/spreading](internal/reconciler/spreading)) to prevent GitHub API rate limit exhaustion during pod restarts:

- **Purpose**: Distributes warm-start reconciliations over time to avoid thundering herd problem when all resources reconcile simultaneously after pod restart.
- **Spread Period** (default 5 min): Grace period after startup during which reconciliations may be delayed. Controlled via `STARTUP_SPREAD_PERIOD_MINUTES` env var.
- **Spread Interval** (default 180 min): Time window across which reconciliations are distributed. Controlled via `SPREAD_INTERVAL_MINUTES` env var.
- **Smart Detection**: Only delays reconciliations for:
  - Resources with unchanged spec (`generation == observedGeneration`)
  - Healthy resources (`Ready` condition is `True`)
  - Resources not being deleted
  - Resources with unchanged sub-resource generations (e.g., referenced RulesetPresets, WebhookPresets)
- **Immediate Processing**: Bypasses spreading for:
  - Spec changes (generation mismatch)
  - Unhealthy/degraded resources
  - Deletions
  - Sub-resource changes
- **Implementation**: 
  - `SpreadingManager` created in [cmd/main.go](cmd/main.go) via `spreading.NewDefaultManager()`
  - Factory methods (`CreateForOrg()`, `CreateForRepo()`, `CreateForTeam()`) call `SpreadingManager.Spread()` before creating reconciler
  - Returns `RequiresSpreadError` with calculated delay if spreading is needed
  - Controllers handle this error via `handleRequeueError()` in [internal/controller/shared.go](internal/controller/shared.go)
  - Delay calculated randomly within spread interval to distribute load evenly
- **Configuration**: Enable/disable via `ENABLE_STARTUP_SPREADING` (default: `true`)

### Parallel Reconciliation Execution
The reconciliation executor ([internal/reconciler/executor.go](internal/reconciler/executor.go)) orchestrates concurrent task execution:

- **Reconciliation Groups**: Each reconciler defines groups via `RequiredReconciliations()` method returning `[]ParallelReconciliationGroup`
- **Sequential Groups**: Groups execute sequentially; next group starts only after previous group completes without errors
- **Parallel Within Groups**: All reconciliations within a group execute concurrently using goroutines
- **Timeout Protection**: Each reconciliation task has a 5-minute context timeout (`ReconciliationContextTimeout`)
- **Error Handling**: 
  - All errors within a group are collected
  - If any task in a group fails, subsequent groups are skipped
  - All errors are joined and returned via `errors.Join()`
- **Condition Updates**: Each reconciliation updates its specific condition; `Ready` condition aggregates all results
- **Example**: Organization reconciler might have:
  - Group 1 (parallel): org settings, custom properties, rulesets, code security configs
  - Group 2 (parallel): actions settings (may depend on Group 1 completion)

### Sub-Resource Generation Tracking
To enable intelligent spreading and drift detection:

- **Status Field**: `ObservedSubResourceGenerations map[string]int64` added to Organization, Repository, and Team status
- **Purpose**: Track generations of referenced sub-resources (RulesetPresets, WebhookPresets, CodeSecurityConfigurations)
- **Usage**: 
  - Reconciler calculates current sub-resource generations before reconciliation
  - Factory passes these to `SpreadingManager.Spread()` for comparison with observed values
  - Executor updates `ObservedSubResourceGenerations` in status after successful reconciliation via `SetObservedSubResourceGeneration()`
- **Change Detection**: If current != observed, resource needs reconciliation even if spec unchanged
- **Interface**: Resources implement `GetObservedSubResourceGenerations()` and `SetObservedSubResourceGeneration()` methods

### Success Requeue Interval
Controllers implement continuous drift detection:

- **Requeue on Success**: After successful reconciliation, controllers requeue resource after `SuccessRequeueInterval`
- **Default Value**: Set to spreading interval (typically 180 minutes) to ensure resources are checked regularly
- **Configuration**: Passed from spreading manager via `spreadingManager.GetSpreadInterval()` in [cmd/main.go](cmd/main.go)
- **Implementation**: `return ctrl.Result{RequeueAfter: r.SuccessRequeueInterval}, nil` after successful reconciliation in each controller
- **Benefit**: Detects manual GitHub changes and sub-resource updates without relying solely on K8s watch events

## Recent Architectural Changes (Current Branch)
**Major refactoring**: This branch includes significant architectural improvements:
1. **Removed mutating webhooks**: The `OrganizationDefaulter` and `RepositoryDefaulter` mutating webhooks have been removed. Label management (default labels and resource-specific labels) is now handled in reconcilers via `addLabels()` methods early in the reconciliation loop.
2. **Reconciler package restructuring**: Reconcilers moved from monolithic files in `internal/reconciler/` to dedicated packages:
   - Organization reconciler: [internal/reconciler/orgrec](internal/reconciler/orgrec) with factory function `CreateForOrg()`
   - Repository reconciler: [internal/reconciler/reporec](internal/reconciler/reporec) with factory function `CreateForRepo()`
   - Team reconciler: [internal/reconciler/teamrec](internal/reconciler/teamrec) with factory function `CreateForTeam()`
   - Reconciliation logic split by concern into separate files (e.g., `rec_org.go`, `rec_rulesets.go`, `rec_webhooks.go`, `rec_actions_settings.go`, `rec_code_security_configurations.go`)
3. **New Team CRD and controller**: Added `Team` resource to manage GitHub teams across multiple organizations with support for manual membership and IDP group synchronization; see [api/v1alpha1/team_types.go](api/v1alpha1/team_types.go), [internal/controller/team_controller.go](internal/controller/team_controller.go), and [internal/reconciler/teamrec](internal/reconciler/teamrec).
4. **CodeSecurityConfiguration CRD**: Added standalone `CodeSecurityConfiguration` CRD for managing GitHub code security configurations; see [api/v1alpha1/codesecurityconfiguration_types.go](api/v1alpha1/codesecurityconfiguration_types.go). Note: This is a configuration-only CRD (no controller); configurations are reconciled by the Organization controller via [internal/reconciler/orgrec/rec_code_security_configurations.go](internal/reconciler/orgrec/rec_code_security_configurations.go).
5. **Reconciler factory pattern**: Centralized reconciler creation in [internal/reconciler/reconcilerfactory/factory.go](internal/reconciler/reconcilerfactory/factory.go) with methods `CreateForOrg()`, `CreateForRepo()`, and `CreateForTeam()` that handle fetching K8s resources, building GitHub clients, checking rate limits, **and evaluating startup spreading**.
6. **Removed `internal/defaults` package**: Default label logic moved to [internal/reconciler/utils.go](internal/reconciler/utils.go) with `DefaultLabels()` and `EnforceLabels()` helpers.
7. **Enhanced testing**: Comprehensive unit tests added for reconciler label management in `reconciler_test.go` files, covering label addition, preservation, idempotency, and organization label enforcement.
8. **Improved README**: Major documentation overhaul with clearer project overview, setup instructions, and feature descriptions.
9. **Webhook configuration cleanup**: Removed mutating webhook manifests from `config/webhook/` and Helm chart templates (`chart/templates/mutating-webhook-configuration.yaml`); only validation webhooks remain.
10. **Startup spreading system**: New [internal/reconciler/spreading](internal/reconciler/spreading) package implements intelligent reconciliation distribution during pod startup to prevent API rate limit exhaustion; see `spreading.go`, controlled via environment variables.
11. **Parallel reconciliation execution**: Reconciler executor ([internal/reconciler/executor.go](internal/reconciler/executor.go)) now runs reconciliation groups sequentially with concurrent execution of tasks within each group; each task has a 5-minute timeout.
12. **Sub-resource generation tracking**: Added `ObservedSubResourceGenerations` field to Organization, Repository, and Team status; tracks generation of referenced sub-resources (e.g., RulesetPresets, WebhookPresets) to detect changes and prevent unnecessary spreading.
13. **Success requeue interval**: Controllers now requeue successful reconciliations after a configurable interval (defaults to spread interval, typically 180 minutes) for continuous drift detection; see `SuccessRequeueInterval` in controllers.
14. **Resource health tracking**: Added `IsHealthy()`, `GetObservedGeneration()`, and `GetObservedSubResourceGenerations()` methods to all CRD types in `*_methods.go` files; implements `SpreadableResource` interface for spreading logic.

## Critical Workflows
- Build and run locally:
  - `make build` builds the manager binary.
  - `make run` runs locally with `ENABLE_WEBHOOKS=false`.
- Docker and deploy:
  - `make docker-build docker-push IMG=<registry>/git-hubby:tag`.
  - `make install` installs CRDs; `make deploy IMG=<registry>/git-hubby:tag` deploys the operator; `make undeploy` removes it; `make uninstall` removes CRDs.
- Tests:
  - `make test` runs unit tests with envtest (excludes e2e).
  - `make test-e2e` runs e2e on a Kind cluster; see [Makefile](Makefile) for `KIND_CLUSTER`.
- Codegen/manifests: `make generate` (deepcopy), `make manifests` (CRDs/RBAC), `make build-installer` (dist/install.yaml), `make helm` (chart from kustomize via helmify).
- Lint: `make lint` or `make lint-fix` (golangci-lint).

## Kubebuilder Conventions & Patterns
- **Project scaffolding**: Tracked in [PROJECT](PROJECT) with group `github.interhyp.de`, version `v1alpha1`. Domain: `interhyp.de`. Kubebuilder CLI version 4.10.1.
- **When adding new resources or features**: Always prefer using Kubebuilder CLI scaffolding commands to ensure consistency:
  - New API/CRD: `kubebuilder create api --group github --version v1alpha1 --kind <Kind> --resource --controller`
  - Add webhook: `kubebuilder create webhook --group github --version v1alpha1 --kind <Kind> --defaulting --programmatic-validation`
  - Scaffolding generates proper markers, boilerplate, test suites, and updates [PROJECT](PROJECT) automatically.
  - Manual additions should match the scaffolded patterns; see existing controllers and webhooks as templates.
- **Kubebuilder markers** control code generation:
  - CRD validation: `+kubebuilder:validation:*` (Pattern, Enum, MinLength, MaxLength, Required, Minimum, MaxItems) on struct fields; see examples in [api/v1alpha1/organization_types.go](api/v1alpha1/organization_types.go) (OrgCustomProperty validation), [api/v1alpha1/repository_types.go](api/v1alpha1/repository_types.go), [api/v1alpha1/webhookpreset_types.go](api/v1alpha1/webhookpreset_types.go).
  - RBAC: `+kubebuilder:rbac:groups=...,resources=...,verbs=...` on controller `Reconcile()` methods; see [internal/controller/organization_controller.go](internal/controller/organization_controller.go) and [internal/controller/repository_controller.go](internal/controller/repository_controller.go).
  - Webhooks: `+kubebuilder:webhook:path=...,mutating=false,failurePolicy=fail,...` on webhook validator structs; see [internal/webhook/v1alpha1/organization_webhook.go](internal/webhook/v1alpha1/organization_webhook.go).
  - Subresource status: `+kubebuilder:subresource:status` on root types enables `/status` subresource; see all `*_types.go` files.
  - Object generation: `+kubebuilder:object:root=true` and `+kubebuilder:object:generate=true` control deepcopy generation.
- **Controller-runtime patterns**:
  - Reconcile loop returns `ctrl.Result{}, error`. Requeue with `RequeueAfter` for delays or `Requeue: true` for immediate retry.
  - Use `client.IgnoreNotFound(err)` to skip "not found" errors gracefully; see controllers.
  - Status updates via `client.Status().Update()` or `Status().Patch()`; see reconcilers' `updateStatus()` and `updateResourceStatus()`.
  - Event filters via predicates: `predicate.GenerationChangedPredicate{}` (spec changes) and `predicate.AnnotationChangedPredicate{}` (annotation changes); both are composed with `predicate.Or()` in controllers.
  - Field indexers for efficient queries: `Repository` indexed on `spec.organizationRef.name` in [internal/controller/repository_controller.go](internal/controller/repository_controller.go) for fast org → repos lookup.
  - Priority queue enabled via `controller.Options{UsePriorityQueue: ptr.To[bool](true)}` for immediate reconciliation of new resources.
  - Workqueue rate limiters composed: exponential backoff + global GitHub limiter; see `WithOptions()` in controllers.
- **Finalizers**: Use `controllerutil.AddFinalizer()` / `RemoveFinalizer()` / `ContainsFinalizer()` for deletion logic; see `orgFinalizerName` and `repoFinalizerName` constants in reconcilers.
- **Webhooks**:
  - Validation webhooks implement `webhook.CustomValidator` with `ValidateCreate/Update/Delete()` methods.
  - Registered via `ctrl.NewWebhookManagedBy(mgr).For(&Type{}).WithValidator(&Validator{}).Complete()`.
  - Custom validation returns `field.ErrorList` and `errors.NewInvalid()` for structured errors; see `validateOrganization()` and `validateCustomProperties()` in [internal/webhook/v1alpha1/organization_webhook.go](internal/webhook/v1alpha1/organization_webhook.go).
  - Webhooks disabled locally via `ENABLE_WEBHOOKS=false` env var check in [cmd/main.go](cmd/main.go).
- **Status conditions**:
  - Use `metav1.Condition` with `Type`, `Status`, `Reason`, `Message`, `LastTransitionTime`.
  - Helpers in [internal/conditions/conditions.go](internal/conditions/conditions.go): `SetCondition()` sets individual conditions; `SetReadyCondition()` aggregates required conditions into `Ready`.
  - Standard types: `Ready`, `GitHubSynced`, `RulesetsSynced`, `CustomPropertiesSynced` (orgs), `WebhooksSynced` (repos).
  - List stored with `+listType=map` and `+listMapKey=type` markers for efficient updates.

## Kustomize Structure
- Base manifests in [config/](config/): `crd/`, `rbac/`, `manager/`, `webhook/`, `certmanager/`, `prometheus/`, `network-policy/`.
- Default overlay: [config/default/kustomization.yaml](config/default/kustomization.yaml) with namespace `git-hubby-system`, namePrefix `git-hubby-`, and patches for metrics/webhooks.
- Samples: [config/samples/](config/samples/) contains example CRs for testing.
- CRD patches via [config/crd/kustomization.yaml](config/crd/kustomization.yaml); webhook caBundle injection via cert-manager annotations.

## Testing Patterns
- **Unit tests**: Use envtest (standalone API server + etcd); see controller suite_test.go files. Run with `make test` (excludes e2e via build tag).
- **Test framework**: All operator packages (under `cmd/`, `api/`, `internal/`) must use **Ginkgo v2 + Gomega** (BDD style). Do NOT use plain `testing.T` assertions in these packages. Each package needs a dedicated `suite_test.go` file containing only the Ginkgo bootstrap (`RegisterFailHandler(Fail)` + `RunSpecs(t, "...")`). Behavior-specific tests go in separate `*_test.go` files (e.g., `main_test.go`, `reconciler_test.go`). Test files use `Describe`, `Context`, `It`, `DescribeTable`/`Entry`, `Expect()`, `BeforeEach`, etc. See existing tests for patterns. **Exception**: Standalone tooling under `hack/` and `test/` may use the standard library `testing` package where Ginkgo adds no value.
- **E2E tests**: Use Kind cluster created via `make setup-test-e2e`; tests in [test/e2e/](test/e2e/) run with `make test-e2e` (auto-creates/deletes cluster).
- **Mocking**: GitHub client mock in [internal/ghclient/mock.go](internal/ghclient/mock.go) implements `GitHubClient` interface for unit tests.

## Configuration & Secrets
- The manager reads flags for secret location and TLS; see [cmd/main.go](cmd/main.go) for `--app-credentials-secret-namespace` (default `github-controller`) and `--app-credentials-secret-name` (default `git-hubby-app-credentials`).
- Required secret keys: `app-id`, `private-key` (PEM RSA); parsed in [internal/ghclient/factory.go](internal/ghclient/factory.go). The GitHub App Installation ID is provided per-organization via `Organization.Spec.GitHubAppInstallationId`.
- Metrics and webhook TLS can be configured via flags; HTTP/2 is disabled by default for security.
- **Log level**: Configurable via `LOG_LEVEL` environment variable (accepts `debug`, `info`, `warn`, `error`; case-insensitive). Overrides the `--zap-log-level` CLI flag. Can also be set in `.env` file.

## Project-Specific Conventions
- GitHub rate limit awareness: the factory checks remaining budget and returns a `RateLimitedError`; controllers back off and optionally block using a global limiter; see [internal/reconciler/reconcilerfactory/factory.go](internal/reconciler/reconcilerfactory/factory.go) and [internal/ratelimit/github.go](internal/ratelimit/github.go).
- Startup spreading integration: factory methods evaluate spreading before creating reconcilers; return `RequiresSpreadError` to delay warm-start reconciliations; see [internal/reconciler/spreading](internal/reconciler/spreading) and factory implementation.
- Parallel reconciliation: reconcilers define sequential groups of parallel tasks via `RequiredReconciliations()`; executor runs groups sequentially with concurrent execution within groups; see [internal/reconciler/executor.go](internal/reconciler/executor.go).
- Success requeue: after successful reconciliation, controllers requeue after `SuccessRequeueInterval` (defaults to spread interval, ~180 min) for continuous drift detection.
- Deletion semantics:
  - Organizations: only deleted when no `Repository` references remain; enforced via finalizer; see organization reconciler in [internal/reconciler/orgrec](internal/reconciler/orgrec).
  - Repositories: deletion archives the repo instead of hard delete; see repository reconciler in [internal/reconciler/reporec](internal/reconciler/reporec).
  - Teams: support multi-organization deletion; when an organization is removed from `spec.organizationRefs`, the team is deleted from that organization but preserved in others; see team reconciler in [internal/reconciler/teamrec](internal/reconciler/teamrec).
- Repository indexing: field index on `spec.organizationRef.name` for efficient listing; see [internal/controller/repository_controller.go](internal/controller/repository_controller.go).
- Team multi-organization support: Teams can belong to multiple organizations simultaneously via `spec.organizationRefs`; the reconciler tracks `status.previousOrganizationRefs` to detect removed organizations and clean up teams accordingly; see [internal/reconciler/teamrec/reconciler.go](internal/reconciler/teamrec/reconciler.go).
- Preset application:
  - Rulesets: desired vs current reconciliation for orgs and repos; orphaned rulesets are deleted; see organization and repository reconcilers.
  - Webhooks: desired set derived from `WebhookPresetList`; hash computed from URL/content-type/events; secret hash tracked in status; see [internal/mapper/github_hook_mapper.go](internal/mapper/github_hook_mapper.go), [api/v1alpha1/webhookpreset_methods.go](api/v1alpha1/webhookpreset_methods.go).
- Code Security Configurations: Organizations reference `CodeSecurityConfiguration` CRDs via `spec.codeSecurityConfigurations`; the organization reconciler creates/updates/deletes these configurations in GitHub and manages attachment scopes (all, public, private_or_internal, selected); see [internal/reconciler/orgrec/rec_code_security_configurations.go](internal/reconciler/orgrec/rec_code_security_configurations.go) and [internal/mapper/github_code_security_configuration_mapper.go](internal/mapper/github_code_security_configuration_mapper.go).
- Custom Properties: Repositories support custom properties via `spec.customProperties`; the reconciler resolves property definitions from the organization and applies values; see [internal/reconciler/reporec/rec_custom_properties.go](internal/reconciler/reporec/rec_custom_properties.go).
- Mapping rules: GitHub shapes are produced by `mapper/*` with opinionated defaults, e.g., repository `Visibility` forced to `internal`; see [internal/mapper/github_repo_mapper.go](internal/mapper/github_repo_mapper.go), [internal/mapper/github_org_mapper.go](internal/mapper/github_org_mapper.go), and [internal/mapper/github_team_mapper.go](internal/mapper/github_team_mapper.go).
- Status conditions: set per phase (`GitHubSynced`, `WebhooksSynced`, `RulesetsSynced`, `CustomPropertiesSynced`, `CodeSecurityConfigurationsSynced`, `ActionSettingsSynced`, `MembersSynced`), then `Ready` aggregated; executor updates conditions from reconciliation results.
- Sub-resource generation tracking: `ObservedSubResourceGenerations` in status tracks generations of referenced presets/configurations; enables spreading to detect sub-resource changes; updated via `SetObservedSubResourceGeneration()` after successful reconciliation.

## Implementation Patterns & Examples
- Controller skeletons should:
  - Use `predicate.GenerationChangedPredicate` and `AnnotationChangedPredicate`.
  - Enable `UsePriorityQueue` and compose `workqueue` limiter with global GitHub limiter; see controllers.
  - Set `SuccessRequeueInterval` from spreading manager to enable continuous drift detection.
  - Delegate business logic to reconcilers created by factory functions: `CreateForOrg()`, `CreateForRepo()`, `CreateForTeam()` from [internal/reconciler/reconcilerfactory/factory.go](internal/reconciler/reconcilerfactory/factory.go).
  - Handle `RequiresSpreadError` via `handleRequeueError()` in [internal/controller/shared.go](internal/controller/shared.go).
- Reconciler structure pattern:
  - Each reconciler is in a dedicated package with a `reconciler.go` file containing the main `Reconcile()` method.
  - Reconciliation logic is split into separate files by concern (e.g., `rec_org.go`, `rec_rulesets.go`, `rec_webhooks.go`, `rec_code_security_configurations.go`, `rec_team.go`, `rec_members.go`, `rec_custom_properties.go`).
  - Early in `Reconcile()`: call `addLabels()` to enforce default and resource-specific labels using `EnforceLabels()` from [internal/reconciler/utils.go](internal/reconciler/utils.go).
  - Required reconciliations are defined via `RequiredReconciliations()` method returning `[]ParallelReconciliationGroup` - each group is a slice of `Reconciliation` structs with function and condition type.
  - Groups execute sequentially; reconciliations within each group run concurrently with 5-minute timeout per task.
  - Status conditions are updated automatically by executor from reconciliation results; `Ready` condition aggregates all required conditions.
  - Sub-resource generations are tracked and updated via `SetObservedSubResourceGeneration()` after successful reconciliation.
- When adding fields to CRDs:
  - Update corresponding mappers in [internal/mapper](internal/mapper), validators in [internal/webhook/v1alpha1](internal/webhook/v1alpha1), and status condition handling in reconcilers.
  - Regenerate code/manifests and update Helm chart/CRDs under [chart/crds](chart/crds) and [config/crd/bases](config/crd/bases).
- Example reconciliation paths:
  - Repository: Desired repo computed via `mapper.RepoToGithubRepo()`; current fetched via `GitHubClient.GetRepository()`; diffs checked by `mapper.RepoDiffers()`; updates applied via `EditRepository()`; see [internal/reconciler/reporec](internal/reconciler/reporec).
  - Team: Reconciles team across multiple organizations; checks for existing team via `GetTeamBySlug()` or `GetAllTeamsForOrg()`; creates/updates team; manages members via `AddTeamMembership()` and removes obsolete teams from organizations no longer in `spec.organizationRefs`; see [internal/reconciler/teamrec](internal/reconciler/teamrec).
  - Organization: Reconciles org settings, custom properties, rulesets, code security configurations (referenced by name from `CodeSecurityConfiguration` CRDs), and actions settings in parallel groups; see [internal/reconciler/orgrec](internal/reconciler/orgrec).
- **No mutating webhooks**: All resource mutations (labels, defaults) happen in reconcilers, not in admission webhooks. Webhooks are validation-only.

## Team CRD Specifics
- **Multi-organization support**: Teams can be created across multiple GitHub organizations simultaneously by specifying multiple `OrganizationRef` entries in `spec.organizationRefs`. The reconciler iterates over all organizations and creates/updates/deletes the team in each.
- **Membership modes**: Teams support two mutually exclusive membership modes (enforced by `+kubebuilder:validation:ExactlyOneOf` marker):
  - **Manual membership**: Explicitly list GitHub usernames in `spec.members[]`
  - **IDP synchronization**: Reference an Identity Provider group via `spec.idpGroup`; reconciler will sync membership from the IDP (future implementation)
- **Organization tracking**: The reconciler tracks `status.previousOrganizationRefs` to detect when organizations are removed from the spec and cleans up teams from those organizations while preserving them in remaining organizations.
- **Team slug management**: GitHub assigns a team slug which may differ from the team name; this slug is stored in `status.slug` and used for all subsequent lookups.
- **Organization roles**: Teams can be assigned custom organization roles via `spec.organizationRoles` (e.g., `["all_repo_write"]`); if not specified, defaults to `["all_repo_write"]`.
- **No webhook**: Unlike Organization and Repository CRDs, Team has no validation webhook currently.

## CodeSecurityConfiguration CRD Specifics
- **Configuration-only CRD**: `CodeSecurityConfiguration` is a standalone CRD with no dedicated controller. It serves as a reusable configuration template referenced by Organizations.
- **Organization reference pattern**: Organizations reference configurations via `spec.codeSecurityConfigurations[]` with fields:
  - `name`: Name of the `CodeSecurityConfiguration` CRD
  - `attachmentScope`: Optional scope (`all`, `all_without_configurations`, `public`, `private_or_internal`, `selected`) determining which repositories the configuration applies to
- **Reconciliation by Organization controller**: The Organization reconciler reads referenced `CodeSecurityConfiguration` CRDs and creates/updates/deletes corresponding GitHub code security configurations; see [internal/reconciler/orgrec/rec_code_security_configurations.go](internal/reconciler/orgrec/rec_code_security_configurations.go).
- **Bypass reviewer resolution**: Code security configurations support `secretScanningDelegatedBypassOptions.reviewers[]` with `reviewerName` and `reviewerType` (TEAM or ROLE). The reconciler resolves reviewer names to IDs by querying GitHub teams or organization roles; see `resolveBypassReviewerNames()`.
- **Default for new repos**: Configurations can be set as default for new repositories via `spec.defaultForNewRepos` (`all`, `private_and_internal`, `public`); the reconciler manages this via a separate GitHub API endpoint.
- **Comprehensive security settings**: Configurations cover all GitHub code security features including advanced security, dependency graph, Dependabot, code scanning, secret scanning, push protection, validity checks, and more; see [api/v1alpha1/codesecurityconfiguration_types.go](api/v1alpha1/codesecurityconfiguration_types.go).
- **Diff detection**: The mapper implements `CodeSecurityConfigurationsDiffer()` to compare K8s desired state with GitHub current state and only apply updates when necessary; see [internal/mapper/github_code_security_configuration_mapper.go](internal/mapper/github_code_security_configuration_mapper.go).

## Observability & Ops
- Metrics server is configurable/secured; Prometheus and Grafana resources exist under [config/prometheus](config/prometheus) and [grafana](grafana).
- Helm chart and RBAC templates in [chart](chart) and [config/rbac](config/rbac); validate webhook/network/certmanager templates in [chart/templates](chart/templates) and [config/certmanager](config/certmanager).

Questions or gaps? Tell me which sections need more detail (e.g., secret schema, chart values), and I’ll refine this doc.
