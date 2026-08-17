# ADR-004: Singleton Cluster-Scoped CRs Named "cluster"

## Status

Accepted

## Context

The operator provides two CRDs for configuration:
- `ExternalSecretsConfig`: User-facing configuration for operand installation
- `ExternalSecretsManager`: Global operator-level configuration and status aggregation

We needed to decide:
1. Should these be namespace-scoped or cluster-scoped?
2. Should we allow multiple instances or enforce a singleton?
3. What should the singleton instance be named?
4. How do we enforce the singleton constraint?

## Decision

Both `ExternalSecretsConfig` and `ExternalSecretsManager` are:
- **Cluster-scoped** (not namespaced)
- **Singletons** enforced via CEL validation
- **Named "cluster"** as the only valid instance name

The singleton constraint is enforced at admission time via CEL:
```go
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="..."
```

Constants in `pkg/controller/common/constants.go`:
```go
const (
    ExternalSecretsConfigObjectName   = "cluster"
    ExternalSecretsManagerObjectName  = "cluster"
)
```

## Rationale

### Why Cluster-Scoped?

1. **Global scope of operand**: The external-secrets operand runs in a single namespace (`external-secrets`) but manages cluster-scoped resources (ClusterRoles, ClusterRoleBindings, ClusterSecretStores, ValidatingWebhookConfigurations)

2. **Consistency with OpenShift patterns**: Many OpenShift operators use cluster-scoped singleton CRs for global configuration (e.g., `Image.config.openshift.io`, `Ingress.config.openshift.io`, `Proxy.config.openshift.io`)

3. **Single source of truth**: One cluster should have one external-secrets installation with one configuration

4. **Simpler RBAC**: No namespace-level permission complexity; cluster-admin or dedicated ClusterRole grants access

### Why Singleton?

1. **Prevents conflicts**: Multiple configs would create ambiguity about which should apply

2. **Simplifies reconciliation**: Controller always knows which CR to watch (no need for leader election or config merging)

3. **Clear ownership**: One CR, one operator instance managing it

4. **Security boundary**: Prevents privilege escalation via conflicting configurations

5. **Status clarity**: One status object for the entire cluster's external-secrets state

### Why Named "cluster"?

1. **OpenShift convention**: Matches naming pattern of OpenShift config CRs:
   - `Image.config.openshift.io/cluster`
   - `Ingress.config.openshift.io/cluster`
   - `Proxy.config.openshift.io/cluster`
   - `DNS.config.openshift.io/cluster`

2. **Semantic clarity**: The name "cluster" signals scope (cluster-wide configuration)

3. **Hard to mistype**: Simple, memorable name

4. **Avoids magic strings**: Could have used "default", "main", "global", but "cluster" aligns with ecosystem

### Why Enforce via CEL?

1. **Admission-time rejection**: Invalid names fail immediately at creation, not during reconciliation

2. **Clear error message**: CEL can provide specific validation error to user

3. **No controller code needed**: Validation happens before controller sees the object

4. **Immutable by design**: Cannot rename the object (renaming would fail CEL validation)

## Consequences

### Positive

- **Clear semantics**: One configuration per cluster, named predictably
- **Security**: No ambiguity about which config applies
- **Consistent with platform**: Follows OpenShift config CR patterns
- **Simple controller logic**: Controllers hardcode the reconcile key to `"cluster"`
- **Prevents user errors**: Cannot create multiple conflicting configs

### Negative

- **No multi-tenancy**: Cannot have different configurations per namespace (acceptable - not a use case)
- **Must delete/recreate to rename**: Cannot rename (acceptable - renaming a singleton makes no sense)
- **Cluster-admin required**: Creating these CRs requires cluster-scoped permissions (acceptable - this is platform-level config)

### Mitigations

- **Default creation**: `ExternalSecretsManager` is auto-created by operator on install, so users typically only interact with `ExternalSecretsConfig`
- **Clear documentation**: AGENTS.md, ARCHITECTURE.md, and API docs all explain the singleton pattern
- **Helpful error messages**: CEL validation provides clear message if wrong name used

## Implementation Notes

### Controller Reconciliation Pattern

All controllers hardcode the reconcile key:
```go
func (r *Reconciler) mapToExternalSecretsConfig(ctx context.Context, obj client.Object) []reconcile.Request {
    return []reconcile.Request{
        {NamespacedName: types.NamespacedName{Name: common.ExternalSecretsConfigObjectName}},
    }
}
```

### Default Resource Creation

The `ExternalSecretsManager` CR is auto-created in `pkg/operator/setup_manager.go:CreateDefaultESMResource`:
```go
esm := &operatorv1alpha1.ExternalSecretsManager{
    ObjectMeta: metav1.ObjectMeta{
        Name: common.ExternalSecretsManagerObjectName,
    },
    Spec: operatorv1alpha1.ExternalSecretsManagerSpec{
        ManagementState: operatorv1alpha1.Managed,
    },
}
```

### CEL Validation

Both CRDs include this validation marker:
```go
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="metadata.name must be 'cluster'"
```

Enforced at:
- Creation (cannot create with wrong name)
- Update (cannot rename to wrong name)
- Validation is server-side (API server enforces before controller sees it)

### Testing

API tests in `api/v1alpha1/tests/<crd>/invalid-name.testsuite.yaml` verify:
```yaml
tests:
  onCreate:
    - name: "reject non-cluster name"
      initial: |
        apiVersion: operator.openshift.io/v1alpha1
        kind: ExternalSecretsConfig
        metadata:
          name: wrong-name
        spec:
          managementState: Managed
      expectedError: "metadata.name must be 'cluster'"
```

## Alternatives Considered

1. **Namespace-scoped CRs** - Rejected; operand is cluster-scoped
2. **Multiple instances with leader election** - Overengineered; no multi-tenancy requirement
3. **Name it "default"** - Less semantic; "cluster" is clearer about scope
4. **No name enforcement** - Rejected; allows user errors and conflicts
5. **Enforce in controller code** - Rejected; CEL enforcement is cleaner and earlier

## Future Considerations

If multi-tenancy is ever required (different configs per namespace), we could:
- Introduce namespace-scoped `ExternalSecretsNamespaceConfig` CRD
- Keep cluster-scoped `ExternalSecretsConfig` for global defaults
- Namespace configs override cluster config for their scope

This would be a new CRD, not a change to the existing singleton pattern.

## References

- CEL validation: `api/v1alpha1/external_secrets_config_types.go`
- Constants: `pkg/controller/common/constants.go`
- Default creation: `pkg/operator/setup_manager.go:CreateDefaultESMResource`
- OpenShift config CRs: https://docs.openshift.com/container-platform/latest/rest_api/config_apis/
- API tests: `api/v1alpha1/tests/externalsecretsconfig.operator.openshift.io/`
