# ADR-002: Three-Controller Architecture

## Status

Accepted

## Context

The External Secrets Operator needs to:
1. Install and manage the upstream external-secrets operand (Deployments, RBAC, Services, etc.)
2. Provide a global status view for operators/admins
3. Conditionally integrate with cert-manager when available

We needed to decide whether to implement this as:
- A single monolithic controller
- Multiple specialized controllers
- A controller with multiple reconcilers
- An event-driven architecture

## Decision

We implement **three separate controllers** running in a single binary:

| Controller | Package | Primary Watch | Purpose |
|---|---|---|---|
| `external-secrets-controller` | `pkg/controller/external_secrets/` | `ExternalSecretsConfig` CR | Installs/reconciles operand deployments, RBAC, services, webhooks, network policies |
| `external-secrets-manager` | `pkg/controller/external_secrets_manager/` | `ExternalSecretsManager` CR | Aggregates controller statuses into a global status CR |
| `crd-annotator` | `pkg/controller/crd_annotator/` | ESO CRDs (metadata only) | Adds cert-manager CA injection annotations to operand CRDs (conditional) |

All three controllers are registered in `pkg/operator/setup_manager.go` with a single `controller-runtime` manager.

## Rationale

**Why separate controllers instead of a monolith?**

1. **Separation of concerns**
   - Each controller has a distinct purpose and lifecycle
   - `crd-annotator` is conditionally registered only when cert-manager is installed
   - Status aggregation (`external-secrets-manager`) is independent of resource reconciliation

2. **Different watch patterns**
   - `external-secrets-controller`: Watches `ExternalSecretsConfig` + managed resources (labeled `app=external-secrets`)
   - `external-secrets-manager`: Watches `ExternalSecretsManager` + `ExternalSecretsConfig` status
   - `crd-annotator`: Watches ESO CRDs (metadata only, label-filtered) + `ExternalSecretsConfig`

3. **Independent cache configurations**
   - Main controller uses manager-level label-filtered cache
   - `crd-annotator` builds its own custom cache because it watches a disjoint resource set

4. **Testability**
   - Each controller can be tested independently
   - Fakes for `CtrlClient` interface work across all controllers
   - Unit tests don't need to mock the entire system

5. **Conditional registration**
   - `crd-annotator` is only registered when cert-manager CRD exists
   - Avoids startup failures when cert-manager is not installed
   - Clean conditional logic in `setup_manager.go`

**Why not event-driven or pub/sub?**
- Kubernetes watch-based reconciliation is simpler and more reliable
- No need for message queues or complex event routing
- Controller-runtime provides battle-tested reconciliation primitives

**Why not multiple binaries?**
- Single binary simplifies deployment and versioning
- All controllers share the same manager and cache (where appropriate)
- Easier to coordinate status updates and configuration changes

## Consequences

### Positive

- **Clear boundaries**: Each controller has well-defined responsibilities
- **Conditional features**: `crd-annotator` can be disabled cleanly when cert-manager is absent
- **Independent evolution**: Controllers can change independently within their interface contracts
- **Testable**: Each controller has focused unit tests with minimal mocking
- **Debuggable**: Controller-specific logs and metrics make troubleshooting easier

### Negative

- **Coordination complexity**: Must ensure controllers don't conflict (e.g., status update races)
- **More code**: Three controller packages instead of one (acceptable trade-off)
- **Watch overhead**: Three sets of watches on some resources (mitigated by predicates)

### Mitigations

- **Status coordination**: `external-secrets-manager` aggregates status from `ExternalSecretsConfig`, avoiding races
- **Watch predicates**: Each controller uses predicates to filter events (`GenerationChangedPredicate`, `LabelChangedPredicate`, etc.)
- **Shared interfaces**: `CtrlClient` interface used by all controllers, fakes shared across tests
- **Single manager**: All controllers share the same cache and rate limiter

## Implementation Notes

### Reconciliation Order
Resources are created in strict dependency order within the main controller:
```
Namespace → NetworkPolicies → ServiceAccounts → Certificates → Secrets 
→ TrustedCA ConfigMap → RBAC → Services → Deployments 
→ ValidatingWebhooks → CR annotation tracking
```

### Status Flow
1. `external-secrets-controller` updates `ExternalSecretsConfig.status.conditions` (Ready, Degraded)
2. `external-secrets-manager` watches `ExternalSecretsConfig.status` changes
3. `external-secrets-manager` aggregates into `ExternalSecretsManager.status.controllerStatuses`
4. `crd-annotator` updates `ExternalSecretsConfig.status` with `UpdateAnnotation` condition

### Conditional Registration Pattern
```go
if isCRDInstalled(ctx, mgr, certmanagerv1.SchemeGroupVersion.WithResource("certificates")) {
    if err := (&crd_annotator.Reconciler{}).SetupWithManager(mgr); err != nil {
        return err
    }
}
```

## Alternatives Considered

1. **Single monolithic controller** - Rejected due to mixing concerns and complex conditional logic
2. **Separate binaries** - Rejected due to deployment complexity and coordination overhead
3. **Plugin architecture** - Overengineered for current requirements
4. **Kubernetes aggregated API server** - Too heavyweight for operator use case

## References

- Controller registration: `pkg/operator/setup_manager.go`
- Main controller: `pkg/controller/external_secrets/controller.go`
- Status aggregator: `pkg/controller/external_secrets_manager/controller.go`
- CRD annotator: `pkg/controller/crd_annotator/controller.go`
- Cache builder: `pkg/controller/external_secrets/controller.go:NewCacheBuilder`
