# ADR-0002: UpdateWithRetry Over Server-Side Apply

**Status**: Accepted
**Date**: 2025-05-15
**Deciders**: ESO team
**Component**: External Secrets Operator

## Context

The operator must create and update Kubernetes resources for the operand deployment. Two primary patterns exist: Server-Side Apply (SSA) and traditional Update with conflict retry. Some resources (Secret, ConfigMap) have fields managed by external controllers (cert-manager, CNO), requiring careful handling.

## Decision

Use `UpdateWithRetry` (Get → set ResourceVersion → Update) as the primary update pattern. Use `patchResourceMetadata` (JSON Patch) for metadata-only updates on co-managed resources. Use MergePatch for CR annotation tracking.

## Rationale

1. **Simplicity** — `UpdateWithRetry` is straightforward and well-understood within the team
2. **Co-managed resources** — Secrets (Data managed by cert-controller/cert-manager) and ConfigMaps (Data managed by CNO) require metadata-only patches to avoid overwriting externally managed fields
3. **Field-level change detection** — Custom `HasObjectChanged` / `deploymentSpecModified` functions provide precise drift detection without SSA's field ownership complexity

## Consequences

### Positive

- No field ownership conflicts with external controllers
- `ObjectMetadataModified` avoids unnecessary updates to Secrets and ConfigMaps whose Data is externally managed
- Precise change detection via type-specific comparison functions

### Negative

- Must maintain type-switch comparison functions (`HasObjectChanged`) for each resource type
- `createWithFallback` needed to handle AlreadyExists from label-filtered cache misses

### Neutral

- `RetryOnConflict` handles concurrent modification gracefully

## Alternatives Considered

### Server-Side Apply

**Description**: Use SSA with field managers for all resource updates.
**Rejected because**: Would conflict with cert-manager and CNO field ownership on Secrets and ConfigMaps. SSA's "apply configurations" would require generating typed apply configs for every resource type.

## References

- `pkg/controller/client/client.go` — `UpdateWithRetry` implementation
- `pkg/controller/common/utils.go` — `HasObjectChanged`, `ObjectMetadataModified`, `deploymentSpecModified`
- `pkg/controller/external_secrets/secret.go` — `createWithMetadataFallback` pattern
