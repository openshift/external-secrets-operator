# ADR-001: Use Bindata Pattern Instead of Helm Charts

## Status

Accepted

## Context

The External Secrets Operator needs to manage the deployment and lifecycle of the upstream [external-secrets](https://github.com/external-secrets/external-secrets) application on OpenShift clusters. Several approaches were considered for how to package and deploy the operand resources (Deployments, Services, RBAC, etc.):

1. **Helm charts**: Use upstream Helm charts with values overrides
2. **Embedded Go code**: Define all Kubernetes resources as Go structs in code
3. **Static YAML manifests (bindata)**: Store YAML templates, compile into binary, decode and mutate at runtime
4. **CRDs only**: Define a CRD that generates resources dynamically

Each approach has different trade-offs around:
- Control over resource configuration
- Ability to track upstream changes
- Type safety and validation
- Operational complexity
- Drift detection capability

## Decision

We will use **static YAML manifests compiled via go-bindata** (option 3). All operand Kubernetes resources are stored as YAML files in `bindata/external-secrets/` and `bindata/external-secrets/resources/`. These are:

1. Compiled into Go binary via `go-bindata` → `pkg/operator/assets/bindata.go`
2. Decoded at reconcile time using typed decoders (`DecodeDeploymentObjBytes`, etc.)
3. Mutated with operator-controlled configuration (images, env vars, labels, security context)
4. Applied imperatively to the cluster via `Create`/`Update`

## Rationale

**Why not Helm?**
- Helm adds operational complexity (Tiller/3-way merge, release tracking)
- Difficult to implement precise drift detection with Helm-managed resources
- Values overrides don't provide complete control over all resource fields
- Helm templating language limits type safety
- OLM (OpenShift's operator lifecycle manager) doesn't require Helm

**Why not embedded Go code?**
- Harder to review resource definitions (Go structs vs YAML)
- Difficult to track upstream changes (must manually translate YAML → Go)
- Loses the benefits of YAML as the canonical format for Kubernetes resources
- More verbose for large resource definitions

**Why not pure CRD generation?**
- Upstream resources have many fields that would need to be exposed as CRD spec
- Would create a non-standard API that differs from upstream
- Harder to track upstream changes when they add new resource types

**Why bindata works for us:**
- ✅ Full control over every field in every resource
- ✅ YAML manifests are easy to review and compare against upstream
- ✅ Type-safe decoding catches schema mismatches at reconcile time
- ✅ Precise drift detection via field-level comparison (`HasObjectChanged`)
- ✅ No external dependencies (Helm, Kustomize) at runtime
- ✅ OLM-compatible pattern used by other OpenShift operators
- ✅ Clear separation between template (bindata YAML) and runtime configuration (mutation)

## Consequences

### Positive

- **Predictable behavior**: Resources are defined explicitly, no hidden templating logic
- **Strong drift detection**: We can detect and revert manual changes to managed resources
- **Build-time validation**: Decode failures panic, indicating build-time bugs
- **Upstream tracking**: Easy to diff bindata YAML against upstream releases
- **Type safety**: Typed decoders ensure schema compatibility

### Negative

- **Manual YAML maintenance**: Must manually update bindata when upstream changes
- **Binary size**: Compiled YAML increases binary size (acceptable trade-off)
- **Regeneration step**: Requires `make update-bindata` after YAML changes (enforced by CI)
- **No templating**: Cannot use Helm helpers or Kustomize overlays (we mutate in Go instead)

### Mitigations

- CI enforces that `bindata.go` is regenerated via `make verify` → `check-git-diff`
- Decode helpers panic on failure to catch manifest corruption early
- `HasObjectChanged` provides type-specific field comparison for precise drift detection
- Asset name constants in `constants.go` catch typos at compile time

## Alternatives Considered

1. **Helm with custom controller** - Rejected due to operational complexity and drift detection limitations
2. **Kustomize** - Rejected for similar reasons to Helm (no runtime templating, harder drift detection)
3. **Hybrid (CRD + bindata)** - Considered but adds complexity without clear benefit

## References

- Bindata implementation: `pkg/operator/assets/bindata.go`
- Decode helpers: `pkg/controller/common/utils.go`
- Resource reconciliation pattern: `pkg/controller/external_secrets/install_external_secrets.go`
- Drift detection: `pkg/controller/common/utils.go:HasObjectChanged`
- Upstream project: https://github.com/external-secrets/external-secrets
