# Security Guidelines

Security conventions enforced by the external-secrets-operator codebase. These rules govern contributions to the operator controller, CRD types, bindata manifests, and container images.

## 1. CRD Singleton and CEL Validation

Both `ExternalSecretsConfig` and `ExternalSecretsManager` are cluster-scoped singletons. The CRD enforces `metadata.name == 'cluster'` via a CEL `XValidation` rule on the type itself (`api/v1alpha1/external_secrets_config_types.go`, `api/v1alpha1/external_secrets_manager_types.go`).

New API fields must include CEL validation rules for:
- **Immutability**: Fields that cannot change after creation use `rule="self == oldSelf"`. Applied to `certManager.mode`, `issuerRef`, and `injectAnnotations`. Network policy `name` and `componentName` use a list-level CEL rule: `oldSelf.all(op, self.exists(p, p.name == op.name && p.componentName == op.componentName))`.
- **Cross-field dependencies**: The spec-level CEL rule on `ExternalSecretsConfigSpec` ensures that when Bitwarden is enabled, either `secretRef` or `certManager` is configured.
- **Bounded cardinality**: Labels, annotations, tolerations, componentConfigs, networkPolicies, and overrideEnv all enforce `MaxItems`/`MaxProperties` (typically 20 or 50).

All CEL rules must have corresponding test cases in `api/v1alpha1/tests/` test suite YAML files.

## 2. Annotation Domain Restrictions

User-supplied annotations in `controllerConfig.annotations` are blocked from reserved Kubernetes ecosystem domains via CEL rules:
- `kubernetes.io/` and subdomains (`*.kubernetes.io/`)
- `openshift.io/` and subdomains
- `k8s.io/` and subdomains
- `cert-manager.io/` (exact domain only, not subdomains)

The regex validates key format (alphanumeric start/end, optional DNS prefix), prefix length (max 253), and name part length (max 63). New annotation restrictions must be added as CEL rules on the `annotations` map field, not in controller-side Go code.

## 3. Label Domain Restrictions

Labels are restricted at the controller level via `disallowedLabelMatcher` regex in `install_external_secrets.go`. Matching **user-supplied** / ESM global labels are **silently skipped** (no CEL on `controllerConfig.labels`, to avoid upgrade breakage for already-stored CRs). This is intentionally broader than the historical identity-only denylist and is aligned with the reserved domains blocked for annotations via CEL:

```regex
^([^/]*\.)?(kubernetes\.io|k8s\.io|openshift\.io)/|^cert-manager\.io/|^external-secrets\.io/|^servicebinding\.io/controller$|^app$
```

That covers:
- Policy-significant domains and subdomains: `kubernetes.io` / `k8s.io` / `openshift.io` (e.g. `pod-security.kubernetes.io/*`, `security.openshift.io/*`) and `cert-manager.io`
- Operand identity keys: `external-secrets.io/*`, exact `app`, and `servicebinding.io/controller`

The operator still sets `app: external-secrets` (and other defaults) via `controllerDefaultResourceLabels` in `constants.go` so `app`, `app.kubernetes.io/version`, `app.kubernetes.io/managed-by`, and `app.kubernetes.io/part-of` remain operator-controlled. Namespace PSA labels (`pod-security.kubernetes.io/{enforce,audit,warn}=restricted`) are applied separately after user label merge and are never taken from CR input. Behavior depends on OpenShift's `security.openshift.io/scc.podSecurityLabelSync` (the operator never sets this key):

- **absent or `true`** (sync may run / forced): **fill-if-absent** only — existing PSA keys (OpenShift sync, admin, GitOps) are left unchanged; missing keys get `restricted`.
- **`false`** (sync off): **overwrite** PSA mode labels to `restricted`.

`*-version` labels are left alone. GitOps/admin owners that must keep non-operator PSA values should leave sync absent/`true` and set the PSA keys they care about; do not set `podSecurityLabelSync=false` unless operator ownership of those labels is intended.

## 4. Environment Variable Reservation

The `overrideEnv` field on `ComponentConfig` uses a CEL rule to reject:
- **Prefix reservations**: Names starting with `KUBERNETES_` or `EXTERNAL_SECRETS_`
- **Exact name reservations**: `HOSTNAME`, `SSL_CERT_DIR`, `SSL_CERT_FILE`

`SSL_CERT_DIR` is managed by the operator when `trustedCABundle` is configured. Proxy env vars (`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY` and lowercase variants) are managed by the proxy reconciliation logic and must not be added to the reservation list since they are set/removed programmatically.

## 5. Container Security Context

Every operand container gets a hardened security context applied in `updateContainerSecurityContext()` (`deployments.go`):

```go
AllowPrivilegeEscalation: false
Capabilities.Drop: ["ALL"]
ReadOnlyRootFilesystem: true
RunAsNonRoot: true
RunAsUser: nil          // defers to the image or PSA
SeccompProfile.Type: RuntimeDefault
```

This function is called for every deployment container. The bindata manifests also declare these settings, but the controller overwrites them on every reconcile to prevent drift. New deployments must call `updateContainerSecurityContext()` on each container.

## 6. Container Image Security

The Dockerfile runs as UID `65534:65534` (nobody). Operand images are resolved exclusively from `RELATED_IMAGE_*` environment variables (`RELATED_IMAGE_EXTERNAL_SECRETS`, `RELATED_IMAGE_BITWARDEN_SDK_SERVER`), which are set by OLM during installation. The controller treats a missing `RELATED_IMAGE_*` variable as an irrecoverable error. Never hardcode image references in Go code; always read from environment variables following the `RELATED_IMAGE_` convention.

## 7. HTTP/2 Disabled by Default

In `cmd/external-secrets-operator/main.go`, HTTP/2 is disabled for both metrics and webhook servers to mitigate known HTTP/2 vulnerabilities. The `--enable-http2` flag defaults to `false` and sets `c.NextProtos = []string{"http/1.1"}` on the TLS config. Do not change this default.

## 8. Network Policy Architecture

The operator enforces a deny-all-first network model for the operand namespace.

**Naming prefixes** (defined in `constants.go`):
- `eso-sys-` — operator-managed static policies from bindata manifests
- `eso-user-` — user-defined policies from `controllerConfig.networkPolicies`

**Static policies** (always applied):
- `eso-sys-deny-all-traffic` — blanket deny on all pods
- `eso-sys-allow-api-server-egress-for-main-controller` — egress to port 6443
- `eso-sys-allow-api-server-egress-for-webhook` — egress to 6443, ingress on 10250 and 8080
- `eso-sys-allow-to-dns` — egress to OpenShift DNS pods on ports 53/5353

**Conditional policies**:
- `eso-sys-allow-api-server-egress-for-cert-controller` — only when cert-manager disabled
- `eso-sys-allow-api-server-egress-for-bitwarden-server` — only when Bitwarden enabled
- `eso-sys-allow-proxy-egress` — when proxy configured and `networkPolicyProvisioning` is `Managed`

**User custom policies** only support `egress` rules and only target `ExternalSecretsCoreController` or `BitwardenSDKServer` components.

## 9. TLS Certificate Management

Two mutually exclusive certificate strategies:

**Built-in cert-controller** (default): Deployed as `external-secrets-cert-controller` when `certManager.mode` is not `Enabled`. TLS secret is `external-secrets-webhook`.

**cert-manager integration**: When `certManager.mode: Enabled`, the cert-controller is skipped. Webhook TLS secret becomes `external-secrets-webhook-cm` to avoid clash. The `mode`, `issuerRef`, and `injectAnnotations` fields are all immutable after creation.

## 10. Trusted CA Bundle Validation

The `trustedCABundle` ConfigMap undergoes strict PEM validation in `trusted_ca_bundle.go`:
- Must contain at least one PEM-encoded X.509 CA certificate
- Private key PEM blocks are rejected (RSA, EC, DSA, ENCRYPTED, generic)
- Leaf certificates (non-CA) are rejected
- Trailing non-PEM data is rejected
- When the ConfigMap carries the CNO inject label and proxy is enabled, the user CA mount is skipped

## 11. Proxy Configuration Security

Proxy settings are resolved by layering three sources (highest to lowest priority): ESC spec, ESM globalConfig, OLM environment variables. URL validation requires a valid scheme and host; explicit ports must be in TCP range 1-65535. Both uppercase and lowercase proxy env vars are set on all containers.

## 12. RBAC Least-Privilege

The operator's ClusterRole (`config/rbac/role.yaml`) is auto-generated from `+kubebuilder:rbac` markers and requests only needed verbs per resource. Key constraints:
- `serviceaccounts/token` only gets `create`
- Deployments do not get `delete`
- Namespaces do not get `delete`

## 13. Reconciliation Drift Protection

The controller uses label-filtered informer caches (`app=external-secrets`) and detects external modifications via `HasObjectChanged()`. `createWithFallback()` handles label-stripped resources by using an uncached client to restore desired state when `Create` returns `AlreadyExists`.
