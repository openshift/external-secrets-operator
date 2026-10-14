# ADR-0003: Network Policy Naming Scheme and Migration

**Status**: Accepted
**Date**: 2025-09-12
**Deciders**: ESO team
**Component**: External Secrets Operator

## Context

Operator v1.0.0-v1.1.x created network policies with unprefixed names. As user-defined network policies were introduced, a clear naming convention was needed to distinguish operator-managed from user-managed policies. Upgrading clusters need to migrate from unprefixed to prefixed names without disrupting connectivity.

## Decision

Adopt a two-prefix naming scheme:
- `eso-sys-*` for operator-managed static policies (deny-all, API server egress, DNS, proxy)
- `eso-user-*` for user-defined policies from `spec.controllerConfig.networkPolicies`

Implement a one-time migration cleanup of unprefixed policies, tracked via the `skipNPCleanupAnnotation` on the ESC CR.

## Rationale

1. **Clear ownership** — prefix immediately identifies whether a policy is operator-managed or user-defined
2. **Safe migration** — one-time cleanup prevents orphaned unprefixed policies while the annotation prevents repeated cleanup attempts
3. **Future extensibility** — user policies can be added/modified via CR spec without naming collisions

## Consequences

### Positive

- Users can identify policy ownership at a glance
- No naming collisions between operator and user policies
- Migration is idempotent (runs once per cluster)

### Negative

- Migration code and `skipNPCleanupAnnotation` must be maintained until v1.5.0
- Network policy entries in the CR spec cannot be removed once added (CEL immutability constraint). The controller recreates a missing `eso-user-*` NetworkPolicy from the CR, so deleting the Kubernetes object alone does not revoke access.
- **Supported removal path today**: tighten or empty the policy's egress rules in place (same `name` + `componentName`), or leave the entry unused. Removing a list entry requires an API/CEL change via an [enhancement proposal](https://github.com/openshift/enhancements/tree/master/enhancements/external-secrets-operator) before implementation.

### Neutral

- User policy names are limited to 243 characters (`+kubebuilder:validation:MaxLength:=243`) so the `eso-user-` prefix fits within Kubernetes' 253-character name limit

## References

- `pkg/controller/external_secrets/networkpolicy.go` — policy creation and migration
- `pkg/controller/external_secrets/constants.go:133-138` — cleanup annotation and TODO
- Enhancement: [external-secrets-network-policy.md](https://github.com/openshift/enhancements/blob/master/enhancements/external-secrets-operator/external-secrets-network-policy.md)
