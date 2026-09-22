# External Secrets Operator — Architecture

## Overview

The OpenShift External Secrets Operator (ESO) manages the lifecycle of the upstream [external-secrets](https://github.com/external-secrets/external-secrets) project on OpenShift. It is **not** a fork — it deploys and configures the upstream operand via static YAML manifests embedded as bindata.

**Framework**: controller-runtime v0.23.3 (no library-go, no operator-sdk Go libraries)
**Reconciliation**: Standard Update with `RetryOnConflict` — **NOT Server-Side Apply**
**Operand version**: external-secrets v2.5.0

## Repository Layout

```text
api/v1alpha1/                          # CRD types: ExternalSecretsConfig, ExternalSecretsManager
cmd/external-secrets-operator/         # Entrypoint: scheme registration, leader election, metrics
pkg/
├── controller/
│   ├── external_secrets/              # PRIMARY CONTROLLER — operand lifecycle
│   │   ├── controller.go             # Reconciler, watches, error classification
│   │   ├── constants.go              # All constants, asset names, label maps
│   │   ├── install_external_secrets.go # Ordered resource installation (11 steps)
│   │   ├── deployments.go            # Deployment creation, image resolution, container security
│   │   ├── certificate.go            # cert-manager Certificate resources
│   │   ├── configmap.go              # Trusted CA bundle ConfigMap
│   │   ├── networkpolicy.go          # Static (eso-sys-*) and user (eso-user-*) policies
│   │   ├── rbacs.go                  # ClusterRole, ClusterRoleBinding, Role, RoleBinding
│   │   ├── secret.go                 # Webhook TLS secret
│   │   └── validatingwebhook.go      # Webhook configurations
│   ├── common/                        # Shared utilities, error types, decode functions
│   ├── external_secrets_manager/      # ESM CONTROLLER — status aggregation
│   ├── crd_annotator/                 # CRD ANNOTATOR — cert-manager CA injection (conditional)
│   └── client/                        # CtrlClient interface with UpdateWithRetry, Exists
│       └── fakes/                     # Counterfeiter-generated test fakes
pkg/operator/
│   ├── setup_manager.go              # Controller registration, default ESM creation
│   └── assets/bindata.go             # Generated — DO NOT EDIT
pkg/version/                           # Build-time ldflags (commit, version, date)
bindata/external-secrets/
├── operand/                           # Helm-rendered + operator-preserved operand YAML
└── networkpolicies/                   # Operator-owned NetworkPolicy YAML
config/                                # CRDs, RBAC, manager deployment, samples, console
bundle/                                # OLM bundle (CRDs, metadata, console quickstarts)
hack/                                  # Build/update scripts
test/
├── e2e/                              # Ginkgo E2E tests (//go:build e2e)
├── apis/                             # envtest-based API/CEL validation tests
└── utils/                            # Test utilities (conditions, cleanup, artifact dump)
```

## Controllers

### 1. ExternalSecrets Controller (primary)

**File**: `pkg/controller/external_secrets/controller.go`
**Watches**: ExternalSecretsConfig (primary), all managed resources via label `app=external-secrets`, ExternalSecretsManager (spec changes)
**Singleton**: Always reconciles ExternalSecretsConfig named `cluster`

#### Reconciliation Flow

```text
Fetch ESC
  ├─ if deleting: cleanup managed resources → remove finalizer → return
  └─ else: ensure finalizer → Fetch ESM → processReconcileRequest
```

#### Resource Installation Order (`install_external_secrets.go`)

Resources are applied in strict dependency order:

1. Namespace (`external-secrets`)
2. Network Policies (static `eso-sys-*` + user `eso-user-*`)
3. Service Accounts
4. Certificates (when cert-manager enabled)
5. Secrets (webhook TLS)
6. Trusted CA Bundle ConfigMap (when proxy configured)
7. RBAC (ClusterRoles, ClusterRoleBindings, Roles, RoleBindings)
8. Services
9. Deployments (core, webhook, cert-controller, bitwarden — conditional)
10. Validating Webhooks
11. CR annotation tracking (MergePatch — only after all resources reconciled)

#### Error Classification (`common/errors.go`)

| Error Type | Effect | Requeue |
|-----------|--------|---------|
| `IrrecoverableError` | Degraded=True, Ready=False | No |
| `RetryRequiredError` | Degraded=False (Reason=Ready), Ready=False (Reason=Progressing) | Yes (30s) |
| `UserConfigurationError` | Degraded=True, Ready=False | Only if NotFound |

#### Cache Configuration

- Label-filtered cache scoped to `app=external-secrets`
- ConfigMaps cached by namespace (OperandDefaultNamespace only)
- Certificate informer conditionally registered (only when cert-manager CRD detected at startup via discovery API)
- `createWithFallback`: handles AlreadyExists from label-filtered cache misses

#### Two Clients

The ExternalSecrets reconciler maintains two Kubernetes clients:

| Client | Field | Use |
|--------|-------|-----|
| Cached | `r.CtrlClient` | Normal access to managed operand resources in the label-filtered cache (`NewCacheBuilder` / `buildCacheObjectList`) |
| Uncached | `r.UncachedClient` | Objects not tracked by the cache (cert-manager Issuers, user-provided Secrets such as Bitwarden `secretRef`); also cache-miss fallbacks via `createWithFallback` / `createWithMetadataFallback` |

**Invariant**: use `r.CtrlClient` for normal reads/writes of `controllerManagedResources`. Use `r.UncachedClient` for those same types only as a fallback when the label-filtered cache misses (for example, after an external actor strips the managed label and `Create` returns `AlreadyExists`).

### 2. ExternalSecretsManager Controller

**File**: `pkg/controller/external_secrets_manager/controller.go`
**Purpose**: Status aggregation — copies ESC conditions into ESM ControllerStatuses
**Auto-creation**: Creates default ESM CR at startup with retry

### 3. CRD Annotator Controller (conditional)

**File**: `pkg/controller/crd_annotator/controller.go`
**Purpose**: Adds `cert-manager.io/inject-ca-from` annotation to external-secrets CRDs
**Condition**: Only registered when cert-manager inject annotations enabled in ESC
**Cache**: Label-filtered by `external-secrets.io/component=controller`
**Update method**: MergePatch for CRD annotation updates

## Resource Management Patterns

### Update Strategy: NOT Server-Side Apply

There is **no SSA usage** in this codebase. Full-resource updates use `UpdateWithRetry` (fresh Get, set ResourceVersion, Update). Metadata-only and annotation updates use JSON Patch or MergePatch instead.

| Pattern | Used For | Code Reference |
|---------|----------|---------------|
| `UpdateWithRetry` | Full resource updates | `pkg/controller/client/client.go` |
| `createWithFallback` | Create with AlreadyExists handling | `pkg/controller/external_secrets/controller.go` |
| `patchResourceMetadata` | JSON Patch for metadata-only updates on co-managed resources (Secret, ConfigMap) | `pkg/controller/external_secrets/controller.go` |
| MergePatch | CR annotation tracking, CRD annotator | `install_external_secrets.go`, `crd_annotator/controller.go` |

### Change Detection

| Function | Purpose | Used For |
|----------|---------|----------|
| `HasObjectChanged` | Type-switch field-level comparison | Most resources |
| `ObjectMetadataModified` | Labels + managed annotation keys only | Secrets, ConfigMaps (Data managed externally) |
| `deploymentSpecModified` | Extensive field-by-field including order-insensitive env vars | Deployments |

## Image Resolution

Images are resolved from environment variables (OLM disconnected/mirror convention):

| Env Var | Purpose |
|---------|---------|
| `RELATED_IMAGE_EXTERNAL_SECRETS` | External-secrets operand image |
| `RELATED_IMAGE_BITWARDEN_SDK_SERVER` | Bitwarden SDK server image |
| `OPERAND_EXTERNAL_SECRETS_IMAGE_VERSION` | Version tracking |
| `BITWARDEN_SDK_SERVER_IMAGE_VERSION` | Version tracking |

Defaults are set in `config/manager/manager.yaml`. Missing image env var returns `IrrecoverableError` (no retry, no fallback).

## Bindata Pipeline

Operand manifests are sourced from upstream Helm charts, processed by `hack/update-external-secrets-manifests.sh`:

```text
upstream Helm chart → helm template (cert-manager enabled + disabled variants)
  → strip Helm labels → relabel managed-by → split into individual YAML files
  → bindata/external-secrets/operand/ (preserves operator-owned namespace + bitwarden cert)
  → bindata/external-secrets/networkpolicies/ (unchanged by update script)
  → openshift/build-machinery-go add-bindata → pkg/operator/assets/bindata.go (DO NOT EDIT)
```

Customizations applied during rendering: leader election disabled, cluster-store and push-secret reconcilers disabled in core deployment.

**Decoding**: `runtime.Decode` with codec factory via `Decode*ObjBytes` functions in `common/utils.go`.

## Conditional Components

| Component | Condition | Deployment |
|-----------|-----------|------------|
| Core controller | Always | `external-secrets` |
| Webhook | Always | `external-secrets-webhook` |
| Cert-controller | When cert-manager **disabled** | `external-secrets-cert-controller` |
| Bitwarden SDK server | When bitwarden plugin **enabled** | `bitwarden-sdk-server` |
| Proxy CA ConfigMap | When proxy configured | `external-secrets-trusted-ca-bundle` |
| Proxy NetworkPolicy | When proxy configured + Managed | `eso-sys-allow-proxy-egress` |
| CRD Annotator controller | When cert-manager inject annotations enabled | (runs in operator) |

## Network Policy Architecture

**Static policies** (operator-managed, `eso-sys-*` prefix):
- `eso-sys-deny-all-traffic` — default deny
- `eso-sys-allow-api-server-egress-for-main-controller` — core controller to API server
- `eso-sys-allow-api-server-egress-for-webhook` — webhook traffic (ingress + egress)
- `eso-sys-allow-to-dns` — DNS resolution
- `eso-sys-allow-proxy-egress` — proxy egress (conditional on proxy + Managed)

**User policies** (`eso-user-*` prefix): Built from `spec.controllerConfig.networkPolicies`. Only egress rules; operator auto-handles ingress.

**Migration**: Unprefixed policies from versions < 1.2.0 are cleaned up once, tracked via `skipNPCleanupAnnotation`. TODO: Remove in v1.5.0.

## Label Architecture

**Label priority** (lowest to highest): ESM GlobalConfig labels < ESC ControllerConfig labels < `controllerDefaultResourceLabels`

**Disallowed user labels** (regex): `^app.kubernetes.io/`, `^external-secrets.io/`, `^rbac.authorization.k8s.io/`, `^servicebinding.io/controller$`, `^app$`

**Key labels**:
- `app=external-secrets` — ManagedResourceLabelKey, used for cache filtering and secondary watches
- `app.kubernetes.io/managed-by=external-secrets-operator` — ownership marker
- `externalsecretsconfig.operator.openshift.io/watching=true` — marks user-provided referenced resources

## Managed Annotation Tracking

Annotations from ESC spec are tracked via base64-encoded JSON in the `ManagedAnnotationsKey` annotation on the CR. When annotations are removed from spec, they appear in `DeletedAnnotationKeys` and are removed from all child resources. MergePatch is used for CR annotation update.

## Proxy Configuration

Layered resolution (highest priority first): ESC `spec.appConfig.proxy` > ESM `spec.globalConfig.proxy` > OLM-injected env vars (`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`).

Both uppercase and lowercase proxy env vars are set on operand containers. Trusted CA bundle injected via CNO-labeled ConfigMap with `config.openshift.io/inject-trusted-cabundle=true`.

## Feature Gates

| Feature | CR | Effect |
|---------|--------|--------|
| `UnsafeAllowGenericTargets` | ESM `.spec.features` | Passes `--unsafe-allow-generic-targets=true` to core controller |
| cert-manager integration | ESC `.spec.controllerConfig.certProvider.certManager` | Detected at startup via discovery API; conditionally registers CRD annotator + certificate informer |
| Bitwarden plugin | ESC `.spec.plugins.bitwardenSecretManagerProvider` | Deploys bitwarden-sdk-server |

## Shared Utilities (`pkg/controller/common/`)

| Symbol | Purpose |
|--------|---------|
| `HasObjectChanged` | Type-switch field-level comparison for resource drift |
| `ObjectMetadataModified` | Metadata-only comparison (labels + managed annotations) |
| `deploymentSpecModified` | Deployment-specific field comparison (env order-insensitive) |
| `ReconcileError` / `FromClientError` | Error classification (Irrecoverable, RetryRequired, UserConfiguration) |
| `EvalMode` / `ParseBool` | Mode and bool evaluation helpers |
| `IsFeatureEnabled` | Check feature toggle state from ESM |
| `AddFinalizer` / `RemoveFinalizer` | Finalizer management |
| `RemoveObsoleteAnnotations` | Annotation cleanup |
| `AddManagedMetadataAnnotation` / `GetPreviouslyAppliedAnnotationKeys` | Annotation tracking |
| `Decode*ObjBytes` | Typed bindata decoders (one per resource type) |
| `DefaultRequeueTime` | 30 seconds |

## Container Security

All operand containers enforce:
- `AllowPrivilegeEscalation: false`
- `Capabilities: drop ALL`
- `ReadOnlyRootFilesystem: true`
- `RunAsNonRoot: true`
- `SeccompProfile: RuntimeDefault`

## OpenShift Integrations

- **Trusted CA Bundle**: CNO injects cluster CA via labeled ConfigMap
- **Proxy**: Resolves from OLM-injected env vars as fallback
- **Console**: QuickStart content in `config/console/`
- **Metrics**: Secure metrics with OpenShift service CA
- **Multi-arch**: NodeAffinity for amd64, arm64, ppc64le, s390x

## Anti-Patterns and TODOs

1. `controller.go:349` — cert-manager CRD detection is startup-only; no runtime watch. TODO: Add dynamic CRD watch
2. `controller.go:584` — TODO: For GA, handle cleanup of operand resources on operator removal
3. `configmap.go:25` — TODO: ConfigMap removal when proxy config is removed (deferred)
4. `constants.go:133-138` — TODO: Remove NP migration cleanup in v1.5.0
5. `common/validation_helpers_duplication.go` — Duplicated private k8s.io/kubernetes validation functions. TODO: Remove when upstream makes them public

## SME Review Recommended

- Recipes for adding a new operand component (deployment + service + RBAC + network policy wiring)
- Rationale behind startup-only cert-manager detection vs runtime watch
- Institutional knowledge around bindata update process when bumping upstream external-secrets version
