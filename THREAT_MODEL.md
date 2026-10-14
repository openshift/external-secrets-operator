# Threat Model: External Secrets Operator for Red Hat OpenShift

This document provides a structured security analysis of the External Secrets Operator (ESO)
to help both human reviewers and AI coding agents scope security-relevant changes, understand
existing mitigations, and avoid re-litigating accepted risks. See
[`SECURITY.md`](SECURITY.md) for the vulnerability disclosure process and
[`harness-evals/harness-docs/security-guidelines.md`](harness-evals/harness-docs/security-guidelines.md)
for the enforceable coding-level security rules this model is derived from.

## 1. System context

ESO manages the lifecycle of the upstream [external-secrets](https://github.com/external-secrets/external-secrets)
project on OpenShift. It is **not** a fork — it deploys and configures the upstream operand via
static YAML manifests embedded as bindata, built on controller-runtime (no library-go, no
operator-sdk Go libraries).

- **Operator namespace**: `external-secrets-operator` (OLM-managed deployment)
- **Operand namespace**: `external-secrets` (operator-created, hosts the upstream components)
- **Configuration surface**: two cluster-scoped singleton CRs, both named `cluster` —
  `ExternalSecretsConfig` (operand behavior) and `ExternalSecretsManager` (global/aggregated
  status and feature gates)
- **Reconciliation model**: standard `Get` → `Update` with `RetryOnConflict` — explicitly **not**
  Server-Side Apply (see [`docs/decisions/adr-0002-update-with-retry-over-ssa.md`](docs/decisions/adr-0002-update-with-retry-over-ssa.md))
- **Core trust assumption**: authors of the two singleton CRs are cluster-admin-privileged
  (CRUD on cluster-scoped CRDs is RBAC-gated like any other cluster resource). This model treats
  CR authors the same way any CRD-based operator does — it does not attempt to defend against a
  cluster-admin who is deliberately malicious, only against cluster-admin *misconfiguration* and
  against non-admin actors who interact with narrower surfaces (webhook, generated resources).

## 2. Assets

| Asset | Description | Sensitivity |
|---|---|---|
| Operator service account credentials | Backing identity for the `manager-role` `ClusterRole` (see §4, T0) | Critical |
| Webhook TLS secret (`external-secrets-webhook` / `external-secrets-webhook-cm`) | Private key material for the admission webhook | High |
| User-referenced provider secrets (e.g. Bitwarden `secretRef`, Vault tokens) | External secret-store credentials referenced by `ExternalSecretsConfig`/`ClusterSecretStore` | High |
| Synced application `Secret` objects | Secrets fetched from external stores and written into cluster namespaces by the operand | High |
| Trusted CA bundle `ConfigMap` | PEM-encoded CA certs for proxy/TLS trust | Medium |
| `ExternalSecretsConfig` / `ExternalSecretsManager` specs | Cluster configuration, including `issuerRef`, cert mode, plugin toggles | Medium |
| RBAC objects (`ClusterRole`, `ClusterRoleBinding`, `Role`, `RoleBinding`) | Both the operator's own role and the roles it provisions for the operand | High |
| Operand/operator container images | Supply-chain integrity of running code | Medium |

## 3. Entry points & trust boundaries

| Entry point | Description | Trust boundary | Reachable assets |
|---|---|---|---|
| `ExternalSecretsConfig` / `ExternalSecretsManager` spec fields | Cluster-admin input, extensively CEL-validated at the API server | cluster-admin | CRD specs, cert config, feature gates |
| `controllerConfig.annotations` / `.labels` / `overrideEnv` | User-supplied, filtered via CEL regex (annotations) and controller-side regex (labels, env names) | cluster-admin | Reserved k8s/OpenShift/cert-manager annotation and env namespaces |
| `trustedCABundle` `ConfigMap` reference | User-supplied, strict PEM validation (rejects private keys, non-CA leaf certs, trailing garbage) | cluster-admin | TLS trust chain for proxy/webhook |
| Validating webhook (admission) | Serves over TLS (server cert only — no client-cert verification; `caBundle` injection via cert-manager or the built-in cert-controller lets the API server trust the webhook's cert, not the reverse); validates upstream `ExternalSecret`/`ClusterSecretStore`/etc. CRs | in-cluster, any namespace that can create those CRs | Upstream-managed secret-sync CRs |
| NetworkPolicy ingress/egress boundary | Deny-all-first model; only narrow, named allow rules | namespace (`external-secrets`) | Operand pod network reachability |
| Operand container runtime | Hardened security context on every container (non-root, read-only root FS, all capabilities dropped, `RuntimeDefault` seccomp) | pod | Operand process/filesystem |
| Operator's own Kubernetes API access | The identity described in §4, T0 | cluster-wide | Everything the `manager-role` `ClusterRole` can touch |

## 4. Threats

| ID | Threat | Actor | Impact | Status |
|---|---|---|---|---|
| T0 | **Operator pod/SA compromise.** The operator's `ClusterRole` (`config/rbac/role.yaml`) is cluster-wide and grants `create` on `serviceaccounts/token` (any SA, any namespace), full CRUD+watch on `secrets` (any namespace), and `create`/`delete`/`patch`/`update` on `clusterroles`/`clusterrolebindings`/`roles`/`rolebindings` (notably, **not** `escalate` or `bind`, so Kubernetes' built-in RBAC guards block it from directly granting itself new permissions via RBAC *object* manipulation beyond what `manager-role` already holds). `serviceaccounts/token` creation is a **separate, unconstrained path**, though: `escalate`/`bind` only gate RBAC *object* changes, not token minting — a minted token authenticates as its target SA, so if any other SA in the cluster holds broader permissions than `manager-role` (e.g. a cluster-admin or namespace-admin SA), the attacker inherits those broader permissions directly, with no RBAC guard in between. A compromised operator pod (container escape, supply-chain compromise of the operator image, leaked SA token) could therefore: impersonate any service account cluster-wide — potentially escalating beyond its own permission set via a more-privileged target SA; read/exfiltrate any `Secret` cluster-wide; delete or tamper with existing RBAC objects (availability impact); and bind other identities — including a new, attacker-created `ServiceAccount` — to existing roles whose permissions `manager-role` already fully covers (a persistence/replication risk bounded by `manager-role`'s own scope, distinct from the unbounded token-impersonation path above). | Attacker with code-execution in the operator pod | Critical | **Accepted architectural risk.** These verbs are required for the operator to provision operand RBAC and the webhook TLS secret across namespaces. No compensating detective control exists today (see §6, §8). |
| T1 | Reserved annotation/label/env-var injection overriding operator-managed values (`kubernetes.io/`, `openshift.io/`, `k8s.io/`, `cert-manager.io/`, `app.kubernetes.io/`, `KUBERNETES_*`, `EXTERNAL_SECRETS_*`, etc.) | cluster-admin (misconfiguration, not malice) | Medium | Mitigated — CEL rules on annotations, controller-side regex on labels/env names |
| T2 | Malicious or malformed CA material via `trustedCABundle`, e.g. embedding a private key or a non-CA leaf cert to intercept/weaken TLS trust | cluster-admin (misconfiguration) | High | Mitigated — strict PEM validation in `trusted_ca_bundle.go` rejects private keys and non-CA certs |
| T3 | TLS secret clash/reuse between the built-in cert-controller and cert-manager integration | configuration error during cert-manager mode transition | Medium | Mitigated — `certManager.mode`/`issuerRef`/`injectAnnotations` are immutable (CEL `self == oldSelf`); distinct secret names (`external-secrets-webhook` vs. `-cm`) prevent clash |
| T4 | Supply-chain: untrusted or unpinned operand/operator image substituted at deploy time | cluster/registry compromise | High | Mitigated — images resolved exclusively from OLM-injected `RELATED_IMAGE_*` env vars; missing var is an `IrrecoverableError`; no hardcoded image refs in Go code |
| T5 | Lateral movement from a compromised operand pod to other cluster workloads | in-cluster attacker who has compromised an operand container | High | Mitigated — deny-all-first `NetworkPolicy` model (`eso-sys-deny-all-traffic`) with narrow, explicit allow rules |
| T6 | HTTP/2 protocol-level DoS (e.g. rapid-reset class vulnerabilities) against the metrics/webhook servers | network attacker | Medium | Mitigated — HTTP/2 disabled by default (`--enable-http2=false`) on both servers in `cmd/external-secrets-operator/main.go` |
| T7 | Reconciliation drift / external tampering of operator-managed resources (labels stripped, spec fields hand-edited) | in-cluster actor with write access to managed resources | Medium | Partially mitigated — `HasObjectChanged`/`ObjectMetadataModified` detect drift and `createWithFallback` restores desired state, but this is reactive (corrected on next reconcile), not preventive |
| T8 | Privilege escalation via overly broad **operand** RBAC (as opposed to the operator's own role in T0) | compromised operand pod | Medium | Mitigated — operand `ClusterRole` is generated with least-privilege verbs per resource (e.g. no `delete` on `deployments`/`namespaces`) |

## 5. Deprioritized

| Threat | Reason |
|---|---|
| Malicious cluster-admin deliberately misusing the two singleton CRs | Out of scope — identical trust model to any CRD-based operator; cluster-admin compromise is a cluster-wide concern, not specific to this operator |
| Vulnerabilities in upstream external-secrets operand code | Tracked upstream; this repo embeds pinned upstream manifests/images and does not fork or patch operand logic |
| Startup-only cert-manager CRD detection (no dynamic runtime watch) | Known limitation (`controller.go:349`, tracked as a TODO); an availability/correctness gap if cert-manager is installed after operator startup, not a security exposure — the webhook still falls back to the built-in cert-controller path |

## 6. Open questions

- There is no detective control today for anomalous use of the operator's own high-privilege
  verbs (T0): `serviceaccounts/token` creation, cross-namespace `Secret` access, or
  self-service RBAC object creation by the operator's identity. Should audit-log-based alerting
  or an admission-time anomaly check be added?
- Is a narrower RBAC shape possible for T0 (e.g. scoping `secrets`/`serviceaccounts/token` verbs
  by `resourceNames` or namespace) without breaking the current cross-namespace operand
  provisioning model? This needs input from whoever designed the current cross-namespace
  provisioning flow.
- What is the plan for operand resource cleanup when the operator itself is uninstalled
  pre-GA? (`controller.go:584`, tracked as a TODO — currently out of scope of this threat model
  since it's a lifecycle/cleanup gap rather than an active threat.)
- Is the pre-1.2.0 NetworkPolicy migration-cleanup code (`skipNPCleanupAnnotation`,
  `constants.go:133-138`) still needed, or safe to remove now?

## 7. Provenance

- Installation is OLM-managed; operand/operator images are resolved exclusively via
  `RELATED_IMAGE_*` environment variables (disconnected/mirror-registry safe, no hardcoded refs).
- Operand manifests are sourced from a pinned upstream external-secrets Helm chart
  (currently v2.5.0) via `hack/update-external-secrets-manifests.sh`, embedded as bindata
  (`pkg/operator/assets/bindata.go`) — not hand-patched. See
  [`docs/decisions/adr-0001-bindata-over-helm.md`](docs/decisions/adr-0001-bindata-over-helm.md).
- Dependencies are Go modules, vendored (`vendor/` committed deliberately); `govulncheck` runs
  as part of `make verify`.
- Secret-leak scanning via the `gitleaks` pre-commit hook (`.pre-commit-config.yaml`).
- Container images run as non-root (`65534:65534` / `nobody`), hardened per §3's "Operand
  container runtime" entry.

## 8. Recommended mitigations

| Mitigation | Threat ID(s) | Effort |
|---|---|---|
| Add audit-log-based alerting for `serviceaccounts/token` creation and cross-namespace `Secret` reads/writes by the operator's service account | T0 | M |
| Evaluate whether cluster-wide verbs on `secrets` and `serviceaccounts/token` can be scoped more narrowly (by namespace or `resourceNames`) without breaking cross-namespace operand provisioning | T0 | L |
| Add a dynamic (not just startup) watch for the cert-manager CRD so mid-lifecycle installation is detected without an operator restart | availability, related to T3's trust assumptions | M |
| Periodically re-sync the reserved annotation/label/env-var domain lists against upstream Kubernetes/OpenShift ecosystem additions | T1 | S |
| Revisit `HasObjectChanged`/drift-detection cadence if T7 is judged to need faster-than-next-reconcile correction | T7 | S |
