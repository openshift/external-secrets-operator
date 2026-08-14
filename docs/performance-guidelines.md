# Performance Guidelines

## Cache Architecture

### Manager-Level Label-Filtered Cache
The operator uses a single manager-level cache with per-object label selectors configured via `NewCacheBuilder()` in `pkg/controller/external_secrets/controller.go`. All controller-managed resources (Deployments, Services, RBAC, etc.) are filtered by `app=external-secrets` label. Own CRs (`ExternalSecretsConfig`, `ExternalSecretsManager`) are cached without label filters.

- When adding a new watched resource type, register it in both `controllerManagedResources` (line ~68) and `buildCacheObjectList()` with the appropriate label selector.
- Never create a second manager-level cache for the main controller. The `crd_annotator` controller is the only one that builds its own custom cache (`BuildCustomClient`), because it watches a disjoint resource set (`CustomResourceDefinition` with different label selectors).
- Set `ReaderFailOnMissingInformer: true` on custom caches (as `crd_annotator` does) to fail fast on missing informers rather than silently returning empty results.

### Uncached Client Usage
The operator maintains a separate uncached `CtrlClient` (`r.UncachedClient`) for objects not tracked by the cache, specifically cert-manager Issuers/ClusterIssuers and user-provided Secrets (Bitwarden secretRef). Use uncached reads only for:
- Validating existence of external resources (e.g., `assertIssuerRefExists`, `assertSecretRefExists`)
- One-time bootstrap operations (e.g., `CreateDefaultESMResource` in `setup_manager.go`)

Never use the uncached client for objects in `controllerManagedResources` -- the cache is designed for those.

### Conditional CRD Cache Registration
Before creating the cache, `NewCacheBuilder` checks whether the cert-manager CRD exists via `isCRDInstalled`. If absent, `certmanagerv1.Certificate` is excluded from the cache object list to prevent startup failure. When the CRD exists, an informer is explicitly registered via `mgr.GetCache().GetInformer()`. Follow this pattern for any future optional CRD dependencies.

## Watch and Predicate Patterns

### Predicate Strategy by Resource Type
The controller uses different predicate combinations per resource type to minimize reconciliation:

| Resource | Watch Method | Predicates | Rationale |
|---|---|---|---|
| ExternalSecretsConfig (primary) | `For()` | `GenerationChangedPredicate` | Ignore status-only updates |
| Deployments | `Watches()` | `Or(GenerationChangedPredicate, LabelChangedPredicate)` + `managedResources` | Skip status/replica updates; catch label changes |
| Secrets | `WatchesMetadata()` | `LabelChangedPredicate` | Avoid caching Secret data in memory |
| ConfigMaps | `Watches()` | `ResourceVersionChangedPredicate` + `managedOrWatchedResources` | Watch both operator-managed and user-referenced (trustedCABundle) ConfigMaps |
| Other managed resources | `Watches()` | `managedResources` (label filter) | Only watch operator-owned objects |
| ExternalSecretsManager | `Watches()` | `GenerationChangedPredicate` (no `managedResources`) | Skip status-only changes; ESM is not labeled `app=external-secrets` |
| Certificates (conditional) | `Watches()` | `managedResources` | Only when cert-manager CRD is installed |
| CRDs (crd_annotator) | `WatchesMetadata()` | `AnnotationChangedPredicate` + label filter | Only metadata matters |

When adding new watched resources, always apply at minimum the `managedResources` predicate to avoid reconciling unrelated objects. Use `WatchesMetadata()` when only labels/annotations matter -- it avoids fetching full object bodies.

### Map Function Convention
The `external-secrets-controller` maps events to a single reconcile key: `common.ExternalSecretsConfigObjectName` ("cluster"). The map function checks `hasManagedOrWatchLabel(objLabels)` which matches either the managed resource label (`app=external-secrets` via `ManagedResourceLabelKey`/`ManagedResourceLabelValue`) or the watch label (`externalsecretsconfig.operator.openshift.io/watching`), and returns an empty slice for non-matching objects. The `crd-annotator` uses a separate map function with its own label filter (`external-secrets.io/component=controller`). The `external-secrets-manager` controller uses `EnqueueRequestForObject` directly for its primary CR. Never enqueue multiple requests from a single event.

## Reconciliation Patterns

### Error Classification and Requeue Strategy
The operator classifies errors into three categories via `common.ReconcileError`:

- **IrrecoverableError**: Config validation failures, permission errors, bad requests. Returns `ctrl.Result{}` with no requeue. Examples: missing CRD, invalid cert-manager config, missing env var.
- **UserConfigurationError**: Invalid or incomplete user-provided configuration. Returns `ctrl.Result{}` with no requeue (Degraded=True); recovery is driven by watches. Exception: `IsUserConfigurationNotFound` requeues with 30s since watches won't fire for unmanaged objects.
- **RetryRequiredError**: Transient API failures, conflicts. Requeues with `DefaultRequeueTime` (30s). The helper `common.FromClientError()` auto-classifies based on API error type.

Convention: never return both a `RequeueAfter` result and an error simultaneously. Return the error alone and let controller-runtime handle backoff, or return `RequeueAfter` with nil error.

### Condition Update Optimization
Status conditions are updated only when they actually change. The pattern uses `apimeta.SetStatusCondition()` which returns a boolean, and the controller checks both `degradedChanged` and `readyChanged` before issuing a status update call. Follow this pattern to avoid unnecessary API writes.

### CR Annotation Patching
After all resources are reconciled, CR annotations (managed-annotations tracking + processed annotation) are updated via a single `MergePatch` rather than a full object update. This reduces conflict risk and avoids overwriting spec changes made concurrently.

## Retry and Conflict Handling

### UpdateWithRetry Pattern
Regular object updates (non-status) use `retry.RetryOnConflict(retry.DefaultRetry, ...)`. The `UpdateWithRetry` method on `CtrlClient` follows the read-modify-write pattern: it fetches the latest `ResourceVersion`, sets it on the object, then updates. Use `UpdateWithRetry` instead of bare `Update` for any object that might be modified concurrently.

### Status Update Pattern
Status subresource updates use `retry.RetryOnConflict(retry.DefaultRetry, ...)` directly. The pattern is: fetch latest, deep-copy desired status into it, then call `StatusUpdate`. This appears identically in all three controllers.

### Bootstrap Resource Creation
`CreateDefaultESMResource` uses `retry.OnError` with a custom `shouldRetryOnError` function that stops on permanent errors (AlreadyExists, Conflict, Invalid, BadRequest, Unauthorized, Forbidden, TooManyRequests) and retries on transient ones. Follow this pattern for one-time resource creation at startup.

## Concurrency Primitives

### Resettable sync.Once (`common.Now`)
The `Now` type in `pkg/controller/common/utils.go` extends `sync.Once` with a `Reset()` method using `sync.Mutex` + `atomic.Uint32` with double-checked locking. It is used in both the `external_secrets_manager` controller (to emit a warning event only once per error cycle) and the `external_secrets` controller (for one-shot validation warnings in trusted CA bundle handling, reset on successful reconciliation via `r.now.Reset()`). Use this type when you need one-shot behavior that can be re-armed.

No goroutines are spawned directly by controller code -- all concurrency is handled by the controller-runtime framework. Do not introduce raw goroutines in reconcile loops.

## Drift Detection

### HasObjectChanged Pattern
The `common.HasObjectChanged()` function uses type-specific field comparison (not full `reflect.DeepEqual` on the entire object) to detect drift. Each resource type has a dedicated comparison:
- Deployments: compares replicas, containers (including init containers, security context, resources, ports, probes), volumes, affinity, tolerations, node selector, env vars, service account, DNS policy, template labels, revision history limit
- RBAC: compares only Rules (Roles) or RoleRef+Subjects (Bindings)
- Services: compares Type, Ports, Selector
- Webhooks: compares individual webhook entries by name
- Certificates: compares full `Spec` via `DeepEqual`
- NetworkPolicies: compares PodSelector, PolicyTypes, Ingress, Egress
- ServiceAccounts: metadata-only drift detection (spec comparison always returns `false`)
- Secrets: uses `ObjectMetadataModified()` only (data co-managed by cert-controller)
- ConfigMaps: metadata-only patching pattern (similar to Secrets)

Annotations are compared using managed-key tracking (`annotationMapsModified`) which only checks keys the operator manages, ignoring annotations set by external controllers (e.g., `deployment.kubernetes.io/revision`). This prevents infinite reconcile loops.

When adding a new managed resource type, add a case to the `HasObjectChanged` switch and implement field-level comparison rather than using full `DeepEqual` on the entire spec.

## Asset Decoding
Static manifests are embedded via go-bindata (`pkg/operator/assets/bindata.go`) and decoded at reconcile time using typed `Decode*ObjBytes` helpers. Each decode call allocates a new object. These helpers `panic` on decode failure since the assets are build-time constants. Do not add error handling around these -- a panic here indicates a build or manifest corruption problem.

## E2E Test Polling
E2E tests use `wait.PollUntilContextTimeout` with 5-second intervals and 2-minute default timeouts. Bitwarden-related tests use 4-minute timeouts due to SDK server startup latency. Follow these conventions for new e2e tests rather than using arbitrary sleep durations.
