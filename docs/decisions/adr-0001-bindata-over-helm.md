# ADR-0001: Bindata-Embedded Manifests Over Direct Helm Usage

**Status**: Accepted
**Date**: 2025-05-15
**Deciders**: ESO team
**Component**: External Secrets Operator

## Context

The operator needs to deploy the upstream external-secrets project. The upstream project distributes its manifests via Helm charts. The operator could either use Helm at runtime or pre-render and embed the manifests.

## Decision

Pre-render upstream Helm charts at build time via `hack/update-external-secrets-manifests.sh`, embed them as Go bindata using `openshift/build-machinery-go`, and decode them at runtime with `runtime.Decode`.

## Rationale

1. **No runtime Helm dependency** — the operator binary is self-contained with no need for Helm libraries or tiller
2. **Deterministic deployments** — manifests are version-pinned and reviewed in PRs
3. **OpenShift customization** — the rendering step strips Helm labels, relabels managed-by, disables leader election and cluster-store/push-secret reconcilers
4. **Consistency with OpenShift patterns** — other OpenShift operators (MCO, cluster-authentication-operator) use bindata embedding

## Consequences

### Positive

- Operator image contains all manifests — works in disconnected environments
- Changes to operand manifests are visible in code review
- No runtime dependency on Helm chart repositories

### Negative

- Upstream version bump requires running the update script and reviewing generated diffs
- Customizations to the Helm rendering must be maintained in `hack/update-external-secrets-manifests.sh`

### Neutral

- Generated `pkg/operator/assets/bindata.go` must never be hand-edited

## Alternatives Considered

### Direct Helm Library Usage

**Description**: Use the Helm Go SDK to render charts at runtime.
**Rejected because**: Adds a large dependency, makes deployments non-deterministic, and complicates disconnected environment support.

### Kustomize Overlays

**Description**: Use kustomize to customize upstream manifests.
**Rejected because**: Upstream distributes via Helm, not kustomize bases. Would require maintaining a separate kustomize layer on top of rendered Helm output without clear benefit over bindata.

## References

- `hack/update-external-secrets-manifests.sh` — manifest rendering pipeline
- `pkg/operator/assets/bindata.go` — generated bindata (DO NOT EDIT)
- [openshift/build-machinery-go](https://github.com/openshift/build-machinery-go) — bindata embedding tool
