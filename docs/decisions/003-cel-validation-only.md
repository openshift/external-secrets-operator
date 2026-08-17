# ADR-003: Use CEL Validation Instead of Admission Webhooks

## Status

Accepted

## Context

The operator's own CRDs (`ExternalSecretsConfig`, `ExternalSecretsManager`) need validation to ensure:
- Fields meet length/range constraints
- Cross-field dependencies are satisfied
- Immutability rules are enforced (e.g., cert-manager mode cannot change once set)
- Singleton pattern is enforced (only one CR named "cluster" allowed)

Kubernetes provides two primary validation mechanisms:
1. **CEL (Common Expression Language)** via `+kubebuilder:validation:XValidation` markers
2. **Admission webhooks** via kubebuilder-generated ValidatingWebhookConfiguration

We needed to decide which approach to use for the operator's own API validation.

## Decision

We use **CEL-based CRD validation exclusively** for the operator's own CRDs. No admission webhooks are implemented for `ExternalSecretsConfig` or `ExternalSecretsManager`.

All validation rules are expressed as `+kubebuilder:validation:XValidation` markers in `api/v1alpha1/external_secrets_config_types.go` and `external_secrets_manager_types.go`.

## Rationale

**Why CEL?**

1. **No additional infrastructure**
   - CEL validation runs in the API server, no webhook endpoint needed
   - No need to manage webhook certificates, service, or deployment
   - One less point of failure (webhook unavailable → admission blocked)

2. **Simpler deployment**
   - No webhook configuration to reconcile
   - No network policies for webhook traffic (beyond operand webhooks)
   - Fewer RBAC rules (no webhook configuration management for operator CRDs)

3. **Better performance**
   - CEL evaluates in-process in the API server (nanoseconds)
   - Webhooks add network round-trip latency (milliseconds)
   - No webhook pod startup time or scaling concerns

4. **Declarative validation**
   - Validation rules live alongside field definitions in Go types
   - `make manifests` generates CRD YAML with validation embedded
   - No separate webhook implementation code to maintain

5. **API server guarantees**
   - CEL validation is atomic with admission
   - No race conditions between webhook calls and API writes
   - Consistent behavior across all API server replicas

6. **Testing simplicity**
   - API tests use envtest with real API server + CEL support
   - Declarative test cases in `.testsuite.yaml` files
   - No need to mock webhook server in tests

**Why not webhooks?**

Webhooks add operational complexity we don't need:
- Webhook certificate rotation (even with cert-manager)
- Webhook failure modes (timeout, network issues, pod crashes)
- Version skew between webhook and API server
- Conversion webhooks not needed (single version `v1alpha1`)

**CEL limitations we accept:**
- Cannot call external systems (fine - our validation is self-contained)
- Cannot mutate objects (fine - we use defaults via `+kubebuilder:default`, not defaulting webhooks)
- Requires Kubernetes >= 1.25 (acceptable - OpenShift 4.x supports this)

## Consequences

### Positive

- **Operational simplicity**: No webhook infrastructure to manage
- **Faster admission**: No network round-trip for validation
- **Declarative**: Validation rules defined in Go type markers
- **Reliable**: No webhook failure modes (timeout, pod crash, network partition)
- **Easier testing**: envtest provides real CEL validation

### Negative

- **Kubernetes version dependency**: Requires API server >= 1.25 for CEL
- **CEL learning curve**: Team must learn CEL syntax (mitigated by good examples)
- **Limited expressiveness**: Cannot validate against cluster state (e.g., "issuerRef must exist") - these checks happen in controller code instead

### Mitigations

- **Minimum Kubernetes version**: Documented in `go.mod` and enforced by CI envtest version
- **CEL examples**: Comprehensive validation rules in types serve as reference
- **Controller-level validation**: Complex checks (issuerRef existence) in reconciler with clear error messages
- **API test coverage**: `.testsuite.yaml` files cover all validation rules

## Implementation Notes

### CEL Patterns Used

**Singleton enforcement:**
```go
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="..."
```

**Immutability:**
```go
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="..."
```

**Cross-field dependencies:**
```go
// +kubebuilder:validation:XValidation:rule="!has(self.certManager) || has(self.certManager.issuerRef)",message="..."
```

**List key immutability:**
```go
// +kubebuilder:validation:XValidation:rule="oldSelf.all(op, self.exists(p, p.name == op.name))",message="..."
```

**Domain blocklists:**
```go
// +kubebuilder:validation:XValidation:rule="self.all(key, !key.matches('^(kubernetes\\.io|k8s\\.io|openshift\\.io|cert-manager\\.io)/'))",message="..."
```

### Validation Responsibilities Split

| Validation Type | Where It Happens |
|---|---|
| Field constraints (length, range, enum) | CEL (`+kubebuilder:validation:*`) |
| Cross-field logic | CEL (`XValidation`) |
| Immutability | CEL (`self == oldSelf`) |
| Singleton pattern | CEL (`self.metadata.name == 'cluster'`) |
| External resource existence | Controller (`assertIssuerRefExists`, `assertSecretRefExists`) |
| Configuration validity | Controller (`ReconcileError` with `UserConfigurationError`) |

### Test Coverage

API validation tests are in `api/v1alpha1/tests/<crd>/` as `.testsuite.yaml` files:
- Valid creation
- Invalid creation (CEL rejection)
- Immutability on update
- Boundary values

Run via `make test-apis` (Ginkgo + envtest with real API server).

## Alternatives Considered

1. **Admission webhooks** - Rejected due to operational complexity
2. **No validation** (rely on controller error handling) - Rejected due to poor user experience
3. **OpenAPI schema validation only** (no CEL) - Rejected due to inability to express cross-field rules
4. **Hybrid (CEL + webhooks)** - Rejected as overengineered; CEL covers our needs

## References

- CEL specification: https://github.com/google/cel-spec
- Kubebuilder CEL docs: https://book.kubebuilder.io/reference/markers/crd-validation.html
- Type definitions: `api/v1alpha1/external_secrets_config_types.go`
- API tests: `api/v1alpha1/tests/`
- Test framework: `test/apis/generator.go`
- Envtest setup: `test/apis/suite_test.go`

## Note on Operand Webhooks

This decision applies ONLY to the operator's own CRDs. The operator **does** manage ValidatingWebhookConfigurations for the upstream operand (external-secrets), but those are bindata resources reconciled like any other operand resource. That webhook validates user-created `ExternalSecret`, `SecretStore`, etc. resources, not the operator's configuration CRs.
