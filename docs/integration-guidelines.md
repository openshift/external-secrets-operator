# Integration Guidelines

## Architecture Overview

This operator does NOT embed or fork the upstream external-secrets project. It manages the upstream operand as a set of static YAML manifests (bindata) that are decoded at runtime, mutated with operator-controlled labels/annotations/env/images, and applied to the cluster. There is no Helm chart integration; resource creation is fully imperative via controller-runtime `Create`/`Update`.

## CRD Hierarchy and Singleton Pattern

- **ExternalSecretsConfig** (`externalsecretsconfigs.operator.openshift.io`): primary CR, singleton named `cluster`. Controls operand installation, cert-manager toggle, Bitwarden plugin, proxy, network policies, and per-component overrides.
- **ExternalSecretsManager** (`externalsecretsmanagers.operator.openshift.io`): global config CR, also singleton named `cluster`. Provides lower-priority defaults (labels, resources, proxy, tolerations, affinity, nodeSelector, logLevel) that ExternalSecretsConfig overrides. Also provides optional `features` (e.g., `UnsafeAllowGenericTargets`) that apply across managed deployments.
- Precedence chain for shared fields: `ExternalSecretsConfig > ExternalSecretsManager > OLM environment variables` (proxy only).

## Static Manifest (Bindata) Pattern

All operand Kubernetes resources live as YAML in `bindata/external-secrets/` and `bindata/external-secrets/resources/`. They are compiled into Go via `pkg/operator/assets/bindata.go` (generated with `go-bindata`). At reconcile time:
1. Decode bytes with typed decoders in `pkg/controller/common/utils.go` (e.g., `DecodeDeploymentObjBytes`, `DecodeServiceObjBytes`).
2. Mutate the decoded object: set namespace, apply labels/annotations via `common.ApplyResourceMetadata`, update container images/args, inject proxy env vars, mount CA bundles.
3. Check existence with `r.Exists()`, compare with `common.HasObjectChanged()`, then `Create` or `UpdateWithRetry`.

When adding a new resource: add the YAML to `bindata/`, add a constant in `constants.go` mapping the asset path, regenerate bindata, and follow the existing `createOrApply*` pattern.

## Reconciliation Order

Resources are created in strict dependency order within `reconcileExternalSecretsDeployment`:
Namespace -> NetworkPolicies -> ServiceAccounts -> Certificates -> Secrets -> TrustedCA ConfigMap -> RBAC -> Services -> Deployments -> ValidatingWebhooks -> CR annotation tracking.

Never reorder. CR annotation tracking (managed-annotations) is patched last to ensure obsolete annotations are removed from resources before tracking is updated.

## Conditional Resource Creation

Many resources are conditionally created based on CR spec. The pattern uses a slice of `{assetName, condition}` structs:
- **cert-controller deployment/service**: created when cert-manager is NOT enabled (`!isCertManagerConfigEnabled(esc)`)
- **bitwarden deployment/service/service-account/network-policy**: created when Bitwarden IS enabled (`isBitwardenConfigEnabled(esc)`)
- **bitwarden certificate**: created only when Bitwarden IS enabled AND no `secretRef` is provided AND cert-manager is enabled
- **webhook TLS secret**: created only when cert-manager is NOT enabled (cert-manager manages the secret otherwise, named `external-secrets-webhook-cm` to avoid collision)

## cert-manager Integration

- cert-manager is an optional dependency detected at startup via CRD discovery (`isCRDInstalled` checks `cert-manager.io/v1` `certificates` resource).
- When detected, the Certificate CRD informer is registered with the manager cache. When not detected, the cache omits it entirely to prevent startup failures.
- The `CertManagerConfig` in ExternalSecretsConfig spec requires an `issuerRef` (Issuer or ClusterIssuer). The operator validates the issuer exists via `assertIssuerRefExists` using the uncached client before creating Certificate resources.
- When cert-manager is enabled with `injectAnnotations: "true"`, the `crd-annotator` controller patches all ESO CRDs with `cert-manager.io/inject-ca-from: external-secrets/external-secrets-webhook`.
- Webhook ValidatingWebhookConfigurations get the `cert-manager.io/inject-ca-from` annotation via `withCertManagerAnnotation()`, tracked in managed-annotations for proper lifecycle.

## OpenShift Platform Integrations

- **Trusted CA bundle**: when proxy is configured, a ConfigMap `external-secrets-trusted-ca-bundle` is created with label `config.openshift.io/inject-trusted-cabundle: "true"`. OpenShift's Cluster Network Operator (CNO) injects cluster CA certs into it. The ConfigMap data is owned by CNO; the operator only manages labels/annotations. Mounted at `/etc/pki/tls/certs` in all containers.
- **Proxy**: both uppercase and lowercase variants (`HTTP_PROXY`/`http_proxy`, etc.) are set on all containers and init containers. Proxy is removed cleanly when config is cleared.
- **Network policies**: default deny-all is always applied. Static allow policies (prefixed `eso-sys-`) for API server egress, webhook, DNS, and optionally cert-controller/bitwarden are applied from bindata. An automatic proxy egress NetworkPolicy (`eso-sys-allow-proxy-egress`) is created when proxy URLs are set and `networkPolicyProvisioning` is `Managed`. Users add custom egress-only policies via `controllerConfig.networkPolicies` (prefixed `eso-user-`) with component targeting.
- **Security context**: all containers get hardened `SecurityContext` (non-root, read-only root FS, drop ALL capabilities, seccomp RuntimeDefault).
- **Console capability annotation**: ConsoleYAMLSample resources require `capability.openshift.io/name: Console` annotation.

## Bitwarden Plugin Integration

- Bitwarden is the only currently supported plugin. Enabled via `spec.plugins.bitwardenSecretManagerProvider.mode: Enabled`.
- Requires either a `secretRef` (pre-existing TLS secret with keys `tls.crt`, `tls.key`, `ca.crt`) or cert-manager configuration for automated TLS.
- The bitwarden-sdk-server runs as a separate deployment in the operand namespace. Its image comes from env var `RELATED_IMAGE_BITWARDEN_SDK_SERVER`.
- Network policies must explicitly allow egress from the core controller to bitwarden-sdk-server on port 9998.
- ClusterSecretStore for Bitwarden requires: `bitwardenServerSDKURL`, `caBundle` (base64 CA cert), `organizationID`, `projectID`, and auth `secretRef.credentials` pointing to an access token secret.

## Image Management

Container images in static YAML manifests (`bindata/`) have placeholder values that are replaced at runtime. The reconciler reads images from environment variables set on the operator pod (typically by OLM/CSV):
- `RELATED_IMAGE_EXTERNAL_SECRETS`: external-secrets operand image
- `RELATED_IMAGE_BITWARDEN_SDK_SERVER`: bitwarden-sdk-server image
- `OPERAND_EXTERNAL_SECRETS_IMAGE_VERSION`: version label for operand resources
- `BITWARDEN_SDK_SERVER_IMAGE_VERSION`: version label for bitwarden resources
- `OPERATOR_IMAGE_VERSION`: operator version

The reconciler returns an irrecoverable error if `RELATED_IMAGE_EXTERNAL_SECRETS` or `RELATED_IMAGE_BITWARDEN_SDK_SERVER` is empty. The version env vars (`OPERAND_EXTERNAL_SECRETS_IMAGE_VERSION`, `BITWARDEN_SDK_SERVER_IMAGE_VERSION`, `OPERATOR_IMAGE_VERSION`) may be empty without causing errors -- version labels will simply be blank.

## Client Architecture

- **Cached client** (`r.CtrlClient`): reads from manager cache filtered by `app=external-secrets` label selector. Used for all managed resources.
- **Uncached client** (`r.UncachedClient`): reads directly from API server. Used for objects NOT managed by the controller (issuer validation, secret ref existence checks).
- **UpdateWithRetry**: all mutations use `UpdateWithRetry` which fetches latest resourceVersion before update, wrapped in `retry.RetryOnConflict`.

## Label and Annotation Conventions

- All managed resources carry `app: external-secrets` label (cache filter key).
- Default labels: `app`, `app.kubernetes.io/version`, `app.kubernetes.io/managed-by: external-secrets-operator`, `app.kubernetes.io/part-of: external-secrets-operator`.
- Disallowed label prefixes (rejected silently): `app.kubernetes.io/`, `external-secrets.io/`, `rbac.authorization.k8s.io/`, `servicebinding.io/controller`, `app`.
- Disallowed annotation domain prefixes (rejected by CRD CEL validation): `kubernetes.io/`, `k8s.io/`, `openshift.io/`, `cert-manager.io/`.
- Managed annotation keys are tracked in a base64-encoded JSON array on the CR annotation `externalsecretsconfig.operator.openshift.io/managed-annotations`.

## Adding a New Provider Plugin

Follow the Bitwarden pattern:
1. Add new `Mode`-gated fields to `PluginsConfig` in `api/v1alpha1/external_secrets_config_types.go`.
2. Add static YAML manifests (deployment, service, service account, network policy) to `bindata/external-secrets/resources/`.
3. Add asset name constants to `constants.go`.
4. Add conditional entries to `createOrApplyDeployments`, `createOrApplyServices`, `createOrApplyServiceAccounts`, `createOrApplyNetworkPolicies` using the `{assetName, condition}` struct pattern.
5. Add image env var constants and wire them into `getDeploymentObject`.
6. Register the new component name in the `ComponentName` enum for network policy targeting.
7. Add the container name mapping in `getComponentNameFromAsset`.
8. Add e2e tests with appropriate Ginkgo labels and credential secret conventions.
