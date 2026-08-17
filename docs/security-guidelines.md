# Security Guidelines

## Container Security Context (Hardened Defaults)

All operand containers are hardened programmatically via `updateContainerSecurityContext()` in `pkg/controller/external_secrets/deployments.go`. This function is called for every container (controller, webhook, cert-controller, bitwarden-sdk-server) during deployment reconciliation. The enforced settings are:

- `AllowPrivilegeEscalation: false`
- `Capabilities.Drop: ["ALL"]`
- `ReadOnlyRootFilesystem: true`
- `RunAsNonRoot: true`
- `RunAsUser: nil` (defers to pod-level or image default; static manifests use `1000`)
- `SeccompProfile.Type: RuntimeDefault`

When adding a new container or deployment, always call `updateContainerSecurityContext(&container)` on the container spec. Never set `RunAsUser` to `0` or add capabilities. The reconciler drift-detects security context changes and reverts them.

Static deployment manifests in `bindata/external-secrets/resources/` also set `hostNetwork: false` explicitly. Never change this.

## Container Image (Non-root)

All Dockerfiles (`Dockerfile`, `images/ci/Dockerfile`, `images/ci/operand.Dockerfile`) run as `USER 65534:65534` (nobody). Never change the USER to root or remove this line. The base image is `ubi9-minimal`.

## HTTP/2 Disabled by Default

In `cmd/external-secrets-operator/main.go`, HTTP/2 is disabled for both metrics and webhook servers to mitigate known HTTP/2 vulnerabilities. The `--enable-http2` flag defaults to `false`. The implementation sets `c.NextProtos = []string{"http/1.1"}` on the TLS config. Do not change this default.

## Metrics Endpoint Security

The metrics server uses `filters.WithAuthenticationAndAuthorization` as `FilterProvider`, enforcing authn/authz on the metrics endpoint. Secure metrics serving (HTTPS on `:8443`) is enabled by default (`--metrics-secure=true`). The OpenShift service CA (`/var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt`) is loaded for client verification. When modifying metrics configuration, preserve these protections.

## RBAC Architecture (Three Tiers)

The repo manages RBAC at three distinct levels. Changing one level does not affect the others.

**1. Operator's own ClusterRole** (`config/rbac/role.yaml`): Grants permissions the operator process needs. Generated from `+kubebuilder:rbac` markers in `pkg/controller/external_secrets/controller.go` and `pkg/controller/external_secrets_manager/controller.go`. When adding new API interactions, add markers to the appropriate controller file and run `make manifests`.

**2. Operand ClusterRoles** (static YAML in `bindata/external-secrets/resources/`):
- `clusterrole_external-secrets-controller.yml` -- main controller (secrets CRUD, ESO CRDs)
- `clusterrole_external-secrets-cert-controller.yml` -- cert-controller (webhook configs, secrets for TLS, CRDs)
- `clusterrole_external-secrets-view.yml` -- aggregated to `view`/`edit`/`admin` (read-only ESO resources)
- `clusterrole_external-secrets-edit.yml` -- aggregated to `edit`/`admin` (mutate ESO resources)
- `clusterrole_external-secrets-servicebindings.yml` -- service bindings integration (read-only ESO resources with servicebinding.io/controller label)

The cert-controller role uses `resourceNames` restrictions on webhook configurations (`secretstore-validate`, `externalsecret-validate`) for its update permissions. Follow this pattern of scoping when adding new permissions.

**3. Namespace-scoped roles**: `role_external-secrets-leaderelection.yml` restricts leader election to the operand namespace.

Key convention: The controller ClusterRole grants `secrets` verbs `get/list/watch/create/update/delete/patch`. This is inherently powerful. Never broaden this (e.g., never add `*` verbs or `*` resources).

## RBAC Label Protection

The regex `disallowedLabelMatcher` in `install_external_secrets.go` prevents users from overriding internal labels via `ControllerConfig.Labels`:
```
^app.kubernetes.io\/|^external-secrets.io\/|^rbac.authorization.k8s.io\/|^servicebinding.io\/controller$|^app$
```
When adding new internal labels, add them to this regex.

## Annotation Domain Restrictions

The CRD validation (CEL rules in `external_secrets_config_types.go`) blocks user annotations with reserved domain prefixes: `kubernetes.io/`, `openshift.io/`, `k8s.io/`, `cert-manager.io/`. These rules are enforced at admission time. Do not weaken them. When the operator adds its own annotations (e.g., `cert-manager.io/inject-ca-from`), it does so programmatically, bypassing user-facing validation.

## Environment Variable Reservation

The `ComponentConfig.OverrideEnv` field uses CEL validation to block reserved env var names/prefixes: `KUBERNETES_*`, `EXTERNAL_SECRETS_*`, `HOSTNAME`, `SSL_CERT_DIR`, `SSL_CERT_FILE`. When adding new operator-managed env vars that users should not override, add them to the CEL rule in the CRD type definition.

## TLS Certificate Management (Dual Mode)

Two certificate provisioning paths exist, controlled by `spec.controllerConfig.certProvider.certManager.mode`:

**Built-in cert-controller** (default): A separate deployment (`external-secrets-cert-controller`) manages TLS certificates for the webhook. The cert-controller has its own RBAC and creates/updates the `external-secrets-webhook` Secret. The webhook reads certs from `/tmp/certs` volume mount.

**cert-manager integration**: When enabled, `Certificate` resources are created from templates in `bindata/`. The `issuerRef` is validated to exist before creating the Certificate (via `assertIssuerRefExists`). The webhook secret name changes to `external-secrets-webhook-cm` to avoid clashing with the cert-controller secret. The `cert-manager.io/inject-ca-from` annotation is conditionally added to webhook configurations and ESO CRDs (via the `crd-annotator` controller) only when `injectAnnotations` is set to `"true"` (not merely when cert-manager mode is Enabled).

Key constraint: `certManager.mode`, `issuerRef`, and `injectAnnotations` are immutable once set (enforced via `XValidation:rule="self == oldSelf"`).

## Bitwarden TLS Requirements

When `BitwardenSecretManagerProvider` is enabled, TLS certificates are mandatory. Users must provide either:
- A `secretRef` pointing to a K8s Secret with keys `tls.crt`, `tls.key`, `ca.crt` (validated via `assertSecretRefExists` using an uncached client)
- cert-manager configuration to auto-generate certificates

This is enforced at the CRD level via CEL and in controller code. The bitwarden deployment mounts certs at `/certs` and probes use `scheme: HTTPS`.

## Network Policy Architecture (Default-Deny)

The operator deploys a **deny-all** base NetworkPolicy (`networkpolicy_deny-all.yaml`) that blocks all ingress and egress for all pods in the operand namespace. Specific allow-policies are then layered:

- `allow-api-server-egress-for-main-controller-traffic` -- egress to API server (port 6443)
- `allow-api-server-and-webhook-traffic` -- egress to API server + ingress on webhook port 10250 and metrics port 8080 (from monitoring namespace only)
- `allow-api-server-egress-for-cert-controller-traffic` -- only when cert-controller is active
- `allow-api-server-egress-for-bitwarden-sever` -- only when Bitwarden is enabled (note: file has typo "sever" instead of "server")
- `allow-dns` -- DNS egress

When adding new components or network requirements, follow this pattern: add a conditional static policy in `bindata/` and register it in `createOrApplyStaticNetworkPolicies()` with the appropriate condition.

User-defined network policies via `spec.controllerConfig.networkPolicies` are prefixed with `eso-user-` and restricted to `Egress` policy type only. The operator auto-selects pods via component label.

## Webhook Security Configuration

Validating webhooks use:
- `failurePolicy: Fail` (on the ExternalSecret webhook) -- rejecting requests if the webhook is unavailable
- `sideEffects: None`
- `timeoutSeconds: 5`
- Webhook listens on port `10250` (not the default 443)

The SecretStore webhook also explicitly sets `failurePolicy: Fail` on both SecretStore and ClusterSecretStore webhooks. Maintain `Fail` for security-critical validations.

## Singleton Pattern (Cluster-Scoped CRs)

Both `ExternalSecretsConfig` and `ExternalSecretsManager` are singletons enforced via CEL: `self.metadata.name == 'cluster'`. This prevents privilege escalation through multiple conflicting configurations. Never remove this validation.

## Trusted CA Bundle Handling

When proxy configuration is present, a ConfigMap labeled `config.openshift.io/inject-trusted-cabundle: "true"` is created. The Cluster Network Operator (CNO) populates it with cluster-wide CA certificates. The operator mounts this at `/etc/pki/tls/certs` (Go's default cert path) as a read-only volume. The operator never writes CA data directly -- it only manages the label and lets CNO handle the content.

## Sensitive Data in Tests

E2E tests reference AWS credentials via a well-known Secret (`aws-creds` in `kube-system`). Credential keys are `aws_access_key_id` and `aws_secret_access_key`. These are read from the cluster, never hardcoded. When adding e2e tests for new providers, follow this pattern: reference credentials from pre-existing cluster Secrets, never embed them in test code or YAML fixtures.

## Reconciler Drift Detection

The operator reconciles all managed resources back to desired state. RBAC rules, deployments (including container security context), webhook configurations, network policies, and certificates are compared field-by-field (via `HasObjectChanged` in `pkg/controller/common/utils.go`). If any resource is externally modified (e.g., someone manually adds permissions to a ClusterRole), the operator detects the drift and reverts it. This is a critical security property -- do not disable drift detection for security-sensitive resources.

Note: drift detection does not currently cover pod-level `SecurityContext`, `hostNetwork`, or webhook `failurePolicy`. Manual changes to those fields will not be automatically reverted.

## Error Classification for Security

`FromClientError` in `pkg/controller/common/errors.go` classifies `Unauthorized` and `Forbidden` API errors as `IrrecoverableError`, meaning the operator will not retry. This prevents retry storms against permission boundaries. Maintain this classification when adding new API interactions.
