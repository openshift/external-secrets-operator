# Integration Guidelines

Rules and conventions for integrating the external-secrets-operator with OpenShift and Kubernetes subsystems.

## 1. cert-manager Integration

### Startup Detection

The operator probes for `certificates.cert-manager.io/v1` at startup using the discovery API (`isCRDInstalled`). The result is cached for the process lifetime in `optionalResourcesList`. No dynamic re-detection after startup.

Rules:
- Never add a `Watches()` call for Certificate without guarding with `optionalResourcesList[certificateCRDGKV]`.
- If cert-manager is enabled in ESC spec but the CRD was not detected at startup, return `NewIrrecoverableError`.
- If cert-manager is installed **after** the operator has already started, restart the operator deployment so discovery runs again (the cached negative result does not self-heal).

### CRD Annotator Controller

When cert-manager is installed **and** `injectAnnotations: "true"` is set, the `crd-annotator` controller patches `cert-manager.io/inject-ca-from` on CRDs labeled `external-secrets.io/component=controller`. Only registered when `IsCertManagerInstalled()` returns true.

### Certificate Resources

- Webhook Certificate secret name is `external-secrets-webhook-cm` (avoids collision with cert-controller secret)
- Bitwarden Certificate only created when plugin enabled
- `IssuerRef` validated at reconcile time via uncached client
- DNSNames rewritten to match the operand namespace

### Cert-Controller Exclusion

When cert-manager is enabled, the in-tree cert-controller Deployment and its metrics Service are not created.

## 2. OpenShift CNO / Trusted CA Bundle

### Proxy CA Bundle ConfigMap

When proxy is configured, the operator creates ConfigMap `external-secrets-trusted-ca-bundle` with label `config.openshift.io/inject-trusted-cabundle: "true"`. CNO injects the cluster CA bundle. The operator **never writes Data** — use `patchResourceMetadata` for metadata-only updates.

### User trustedCABundle

Rules:
- Mounted only on the core controller as volume `user-ca-bundle` at `/etc/pki/tls/user-certs`
- Sets `SSL_CERT_DIR=/etc/pki/tls/user-certs:/etc/pki/tls/certs:/etc/ssl/certs`
- Skipped when ConfigMap has CNO inject label and proxy is enabled
- PEM validated: must contain CA certificates, no private keys, no trailing non-PEM data
- ConfigMap labeled with `WatchedResourceLabelKey` for change detection

## 3. OLM Integration

### RELATED_IMAGE Convention

Operand images read from env vars at runtime:
- `RELATED_IMAGE_EXTERNAL_SECRETS` — external-secrets image
- `RELATED_IMAGE_BITWARDEN_SDK_SERVER` — bitwarden image

Set in `config/manager/manager.yaml` and CSV deployment spec. OLM uses the `RELATED_IMAGE_` prefix for disconnected/mirrored registries. Missing env var returns `IrrecoverableError`.

### Bundle Structure

```text
bundle/
  manifests/     # CSV, CRDs, console quickstarts, YAML samples
  metadata/      # annotations.yaml (package, channel, layout)
```

CSV declares `installModes: AllNamespaces` only.

## 4. Proxy Configuration (Layered Resolution)

Precedence (first non-empty wins per field):
1. `ExternalSecretsConfig.spec.appConfig.proxy` (ESC)
2. `ExternalSecretsManager.spec.globalConfig.proxy` (ESM)
3. OLM environment variables (`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`)

Both uppercase and lowercase env vars set on all containers. URLs validated for scheme, host, port range 1-65535.

## 5. OpenShift Console Integration

- `ConsoleQuickStart` in `config/console/` with guided tasks for SecretStore/ExternalSecret creation
- Two `ConsoleYAMLSample` resources for ExternalSecret and Vault SecretStore

## 6. Metrics / Monitoring Integration

Operator metrics use OpenShift service-CA for TLS:
- Service annotation `service.beta.openshift.io/serving-cert-secret-name: metrics-serving-cert`
- Mounted at `/etc/metrics-certs` with `--metrics-cert-dir=/etc/metrics-certs --metrics-secure=true`

Operand metrics Services (port 8080) are plain ClusterIP without TLS.

## 7. Multi-Architecture Support

Operator Deployment declares `nodeAffinity` for `amd64`, `arm64`, `ppc64le`, `s390x` on `linux`. When adding arch support, update both `config/manager/manager.yaml` and the CSV deployment spec.

Operand deployments (from bindata) do not carry node affinity by default; users configure via `spec.appConfig.affinity` or `spec.appConfig.nodeSelector`.

## 8. Webhook Integration

Two `ValidatingWebhookConfiguration` resources: `externalsecret-validate` and `secretstore-validate`. When cert-manager `injectAnnotations` is enabled, `cert-manager.io/inject-ca-from` annotation is added. Webhook volume switches from cert-controller secret to `external-secrets-webhook-cm`.

## 9. Controller-to-Controller Communication (ESM <-> ESC)

### Data Flow

- **ESC reads ESM**: fetches ESM for `globalConfig` (labels, proxy, resources, etc.) and `features` as defaults. ESC-level config takes precedence.
- **ESM watches ESC status**: copies ESC conditions into `esm.status.controllerStatuses[]`
- **ESC watches ESM spec**: `GenerationChangedPredicate`, enqueues ESC singleton on ESM spec changes

### Feature Flag Propagation

ESM `features` mapped to container args via `featureContainerArgs`. Applied only if feature is enabled **and** the deployment declares support via `updateOptionalFeatures`.

### Default ESM Creation

Auto-created at startup by `CreateDefaultESMResource` with standard labels and empty spec.

## 10. Bindata / Asset Management

Static manifests in `bindata/operands/external-secrets/` and `bindata/operator/networkpolicies/`, compiled into `pkg/operator/assets/bindata.go`.

Rules:
- Decode with typed helpers (`DecodeDeploymentObjBytes`, etc.)
- Always call `updateNamespace(obj, esc)` after decoding
- Always call `ApplyResourceMetadata(obj, resourceMetadata)` for labels/annotations
- Use `createWithFallback` for fully-owned resources; `createWithMetadataFallback` for co-managed
- Network policy prefixes: `eso-sys-` (operator), `eso-user-` (user)

## 11. Resource Labeling Conventions

| Label/Annotation | Purpose |
|---|---|
| `app=external-secrets` | Cache filter for managed operand resources |
| `externalsecretsconfig.operator.openshift.io/watching=true` | Watch trigger for user-referenced resources |
| `config.openshift.io/inject-trusted-cabundle=true` | CNO CA bundle injection |
| `external-secrets.io/component=controller` | CRD annotator target |
| `cert-manager.io/inject-ca-from` | cert-manager CA injection |

Disallowed user labels: `^app.kubernetes.io/`, `^external-secrets.io/`, `^rbac.authorization.k8s.io/`, `^servicebinding.io/controller$`, `^app$`
