# Enhancement Proposals & Design Documents

Catalog of design documentation for the External Secrets Operator.

## OpenShift Enhancement Proposals

| Title | Status | Location | Tracking |
|-------|--------|----------|----------|
| External Secrets Operator for Red Hat OpenShift | Implemented | [enhancements/external-secrets-operator/external-secrets-operator.md](https://github.com/openshift/enhancements/blob/master/enhancements/external-secrets-operator/external-secrets-operator.md) | OCPSTRAT-1539, OCPSTRAT-1637, ESO-2, ESO-13, ESO-155 |
| Network Policies for external-secrets Operator and Operands | Implemented | [enhancements/external-secrets-operator/external-secrets-network-policy.md](https://github.com/openshift/enhancements/blob/master/enhancements/external-secrets-operator/external-secrets-network-policy.md) | ESO-165, ESO-70, ESO-418 |
| Component Configuration for external-secrets Operator | Implemented | [enhancements/external-secrets-operator/external-secrets-component-config.md](https://github.com/openshift/enhancements/blob/master/enhancements/external-secrets-operator/external-secrets-component-config.md) | OCPSTRAT-2419, RFE-7842, RFE-8685, ESO-266, ESO-417, ESO-533 |

## Local Design Documents

| Title | Location | Description |
|-------|----------|-------------|
| Dual Cache Fix | [docs/anti-patterns/DUAL_CACHE_FIX.md](../../../docs/anti-patterns/DUAL_CACHE_FIX.md) | Anti-pattern documentation for cache issues |

## Notes

- Enhancement proposals are the source of truth for feature design decisions
- Enhancement proposals are cross-component feature designs; component-specific ADRs are in [`../decisions/`](../decisions/)
- For the latest proposal status, check the [openshift/enhancements](https://github.com/openshift/enhancements/tree/master/enhancements/external-secrets-operator) repository
- **ESO-533 (`controllerConfig.replicas`)**: Documented in-repo at [`../domain/external-secrets-config.md`](../domain/external-secrets-config.md). A follow-up PR to [external-secrets-component-config.md](https://github.com/openshift/enhancements/blob/master/enhancements/external-secrets-operator/external-secrets-component-config.md) in the openshift/enhancements repository is required to align the external enhancement proposal with the shipped API field (purpose, default of 1, valid range ≥1, and HA/leader-election behavior).
