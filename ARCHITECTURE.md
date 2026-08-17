# Architecture

This document describes the high-level architecture of the External Secrets Operator for Red Hat OpenShift.
If you want to familiarize yourself with the codebase, you are in the right place!

For detailed guidelines on specific areas, see the files listed in `AGENTS.md`.
For contribution process, see `CONTRIBUTING.md`.

## Bird's Eye View

On the highest level, this is a Kubernetes operator that installs and manages the upstream [external-secrets](https://github.com/external-secrets/external-secrets) application on OpenShift clusters. The upstream project provides the actual secret-syncing logic (ExternalSecret, SecretStore, etc.). This operator does **not** embed or fork that code. Instead, it manages upstream resources as a set of static YAML manifests that are compiled into the binary, decoded at runtime, mutated with operator-controlled configuration, and applied imperatively to the cluster.

```
                  ┌─────────────────────────────────────────────┐
                  │        ExternalSecretsConfig CR              │
                  │   (singleton "cluster", user-facing API)     │
                  └──────────────────┬──────────────────────────┘
                                     │
              ┌──────────────────────┼──────────────────────┐
              │                      │                      │
    ┌─────────▼──────────┐  ┌───────▼────────┐  ┌──────────▼─────────┐
    │ external-secrets   │  │ external-      │  │ crd-annotator      │
    │ controller         │  │ secrets-manager│  │ controller         │
    │                    │  │                │  │                    │
    │ Decodes bindata,   │  │ Aggregates     │  │ Patches ESO CRDs  │
    │ mutates, creates   │  │ controller     │  │ with cert-manager  │
    │ operand resources  │  │ statuses into  │  │ CA injection       │
    │ (Deployments,      │  │ a global       │  │ annotations        │
    │  RBAC, Services,   │  │ status CR      │  │                    │
    │  NetworkPolicies,  │  │                │  │ (conditional: only │
    │  Webhooks, etc.)   │  │                │  │  when cert-manager │
    │                    │  │                │  │  is installed)     │
    └────────────────────┘  └────────────────┘  └────────────────────┘
              │
              ▼
    ┌────────────────────┐
    │ Upstream operand   │
    │ (external-secrets  │
    │  deployments in    │
    │  "external-secrets"│
    │  namespace)        │
    └────────────────────┘
```

Two singleton CRs drive the system:

- **ExternalSecretsConfig** (name: `cluster`): the primary user-facing CR. Controls operand installation, cert-manager toggle, Bitwarden plugin, proxy, network policies, per-component overrides, and custom trusted CA bundles.
- **ExternalSecretsManager** (name: `cluster`): global operator-level config, auto-created at install. Provides lower-priority defaults and optional features (e.g., `UnsafeAllowGenericTargets`).

The precedence chain for shared fields is: `ExternalSecretsConfig > ExternalSecretsManager > OLM environment variables` (proxy only).

## Code Map

This section describes important directories and data structures.
Pay attention to the **Architecture Invariant** sections.

### `api/v1alpha1/`

CRD type definitions. This is where `ExternalSecretsConfig` and `ExternalSecretsManager` structs live, along with shared types (`CommonConfigs`, `ProxyConfig`, `Mode`, etc.), condition constants, and deepcopy code.

Validation is entirely CEL-based via `+kubebuilder:validation:XValidation` markers on the types. There are no admission webhooks for the operator's own CRDs.

`api/v1alpha1/tests/` contains declarative YAML test suites (`.testsuite.yaml` files) organized by CRD API group domain. These are automatically picked up by the test generator.

**Architecture Invariant:** both CRDs are cluster-scoped singletons enforced by CEL (`self.metadata.name == 'cluster'`). There is exactly one instance of each, ever.

**Architecture Invariant:** several fields on `CertManagerConfig` (`mode`, `issuerRef`, `injectAnnotations`) are immutable once set, enforced by CEL `self == oldSelf` rules. This prevents configuration drift that would leave the cluster in an inconsistent state.

### `bindata/`

Static YAML manifests for the upstream operand. These are the Deployments, Services, RBAC, NetworkPolicies, Certificates, and ValidatingWebhookConfigurations that the operator creates in the cluster.

Files in `bindata/external-secrets/resources/` follow the pattern `<kind-lowercase>_<resource-name>.yml`. Network policies in `bindata/external-secrets/` use `.yaml`.

These files are compiled into Go via go-bindata and live as `pkg/operator/assets/bindata.go` at build time.

**Architecture Invariant:** bindata manifests are templates, not final resources. The controller decodes them, mutates them (namespace, labels, annotations, images, env vars, security context), and then creates or updates them. Never deploy bindata YAML directly to a cluster.

**Architecture Invariant:** do not hand-edit `bindata.go`. Run `make update` to regenerate it from the YAML sources.

### `cmd/external-secrets-operator/`

The operator binary entrypoint. This is a separate Go module. It sets up the controller-runtime manager, configures metrics/webhook servers (with HTTP/2 disabled by default), wires the cache builder, and starts the controllers.

**Architecture Invariant:** HTTP/2 is disabled for metrics and webhook servers to mitigate known HTTP/2 vulnerabilities. The `--enable-http2` flag defaults to `false`.

### `pkg/controller/`

The heart of the operator. Three controller packages plus shared infrastructure:

#### `pkg/controller/external_secrets/`

The main reconciliation controller. Watches `ExternalSecretsConfig`, `ExternalSecretsManager`, and all managed operand resources. The reconciliation flow in `install_external_secrets.go` creates resources in strict dependency order:

```
Namespace → NetworkPolicies → ServiceAccounts → Certificates → Secrets
→ TrustedCA ConfigMap → RBAC → Services → Deployments
→ ValidatingWebhooks → CR annotation tracking
```

Each resource follows the same pattern: decode from bindata → mutate → check existence → create or update if drifted → record events.

`controller.go` is the entry point. It defines `controllerManagedResources` (the set of resource types the controller owns), `NewCacheBuilder` (manager-level label-filtered cache), and `SetupWithManager` (watch/predicate wiring).

`constants.go` contains all asset path constants, label keys, env var names, and network policy prefixes.

**Architecture Invariant:** the reconciliation order must not change. CR annotation tracking is patched last to ensure obsolete annotations are removed before tracking is updated.

**Architecture Invariant:** never return both `RequeueAfter` and a non-nil error from `Reconcile`. Return one or the other.

#### `pkg/controller/external_secrets_manager/`

Status aggregation controller. Watches `ExternalSecretsManager` and `ExternalSecretsConfig` status. Copies per-controller conditions into the manager's `controllerStatuses` list. Also handles auto-creation of the default `ExternalSecretsManager` CR at startup.

#### `pkg/controller/crd_annotator/`

Conditionally registered (only when cert-manager is installed). Watches ESO CRD metadata and `ExternalSecretsConfig`. When cert-manager is enabled with `injectAnnotations: "true"`, patches all ESO CRDs with the `cert-manager.io/inject-ca-from` annotation.

Builds its own custom cache (`BuildCustomClient`) because it watches a disjoint resource set from the main controller.

#### `pkg/controller/client/`

The `CtrlClient` interface. All controllers interact with Kubernetes through this interface, not the raw controller-runtime `client.Client`. It adds `UpdateWithRetry`, `StatusUpdate`, and `Exists` methods. Unit tests use counterfeiter-generated fakes in `client/fakes/`.

**Architecture Invariant:** all resource updates go through `UpdateWithRetry`, which wraps `retry.RetryOnConflict` with a read-modify-write pattern. This prevents silent data loss from stale resource versions.

#### `pkg/controller/common/`

Shared utilities used by all controllers:

- `errors.go` — `ReconcileError` type with three reasons: `IrrecoverableError` (no retry), `RetryRequiredError` (30s requeue), `UserConfigurationError` (no retry, watches drive recovery). `FromClientError` auto-classifies Kubernetes API errors.
- `constants.go` — singleton object names, annotation keys, version strings.
- `utils.go` — `HasObjectChanged` (type-specific drift detection), `Decode*ObjBytes` (bindata decoders that panic on failure), `ApplyResourceMetadata`, finalizer helpers, the `Now` resettable-once type.

**Architecture Invariant:** `Decode*ObjBytes` helpers panic on failure. This is intentional — they are called only with compile-time-known static assets, so a panic indicates a build-time bug, not a runtime error.

#### `pkg/controller/commontest/`

Shared test fixtures: `TestExternalSecretsConfig()`, `TestExternalSecretsManager()`, standard test constants and a sentinel error.

### `pkg/operator/`

Manager setup and controller registration. `setup_manager.go` wires all three controllers and handles the default ESM resource creation. `assets/bindata.go` is the generated bindata.

### `config/`

Kustomize manifests for CRDs, RBAC, manager deployment, samples, and OLM bundle inputs. CRDs in `config/crd/bases/` and RBAC in `config/rbac/role.yaml` are generated — do not hand-edit them.

### `test/`

A separate Go module (`./test` in `go.work`) containing:

- `test/apis/` — API integration tests using Ginkgo + envtest. A generator in `generator.go` auto-creates Ginkgo specs from the YAML test suites in `api/v1alpha1/tests/`.
- `test/e2e/` — End-to-end tests using Ginkgo against a live cluster. Filtered by Ginkgo labels (`Platform:AWS`, `Provider:Bitwarden`, etc.).
- `test/utils/` — Shared E2E helpers (dynamic resource loading, pod readiness polling, artifact dumping).

### `hack/`

Shell scripts for codegen, verification, and CI. Notable scripts:
- `go-fips.sh` — enables FIPS build tags for production.
- `test-apis.sh` — runs API tests with Ginkgo flags.
- `e2e-coverage.sh` — manages coverage-instrumented E2E builds.

### `tools/`

A separate Go module for build-time tool dependencies (controller-gen, golangci-lint, ginkgo, etc.). All tools are vendored and built from source.

## Cross-Cutting Concerns

### Two Clients

The operator maintains two Kubernetes clients:

- **Cached client** (`r.CtrlClient`): reads from the manager cache, which is configured with label selectors (`app=external-secrets`) via `NewCacheBuilder`. Used for all managed operand resources.
- **Uncached client** (`r.UncachedClient`): reads directly from the API server. Used only for objects not tracked by the cache — cert-manager Issuers, user-provided Secrets (Bitwarden `secretRef`), and one-time bootstrap operations.

**Architecture Invariant:** never use the uncached client for objects in `controllerManagedResources`. The cache is designed for those.

### Drift Detection

The operator continuously reconciles all managed resources back to desired state. `HasObjectChanged` in `pkg/controller/common/utils.go` uses type-specific field comparison (not full `DeepEqual`) to detect drift:

- Deployments: containers, init containers, volumes, affinity, tolerations, env vars, security context, revision history limit, etc.
- RBAC: Rules (Roles) or RoleRef+Subjects (Bindings)
- NetworkPolicies: PodSelector, PolicyTypes, Ingress, Egress
- Webhooks: individual webhook entries by name
- Certificates: full Spec via DeepEqual

Annotations are compared using managed-key tracking, which only checks keys the operator manages, ignoring annotations set by external controllers. This prevents infinite reconcile loops.

**Architecture Invariant:** if someone manually modifies a managed ClusterRole, Deployment, or NetworkPolicy, the operator will detect the change and revert it. This is a critical security property.

### Error Classification

Reconciliation errors flow through three channels:

| Reason | Requeue? | Status |
|---|---|---|
| `IrrecoverableError` | No | Degraded=True |
| `UserConfigurationError` | No (except NotFound → 30s) | Degraded=True |
| `RetryRequiredError` | Yes (30s) | Ready=False/Progressing |

`FromClientError` auto-classifies Kubernetes API errors: `Unauthorized`, `Forbidden`, `Invalid`, and `BadRequest` become irrecoverable; everything else becomes retry-required.

### Code Generation

Several artifacts are generated and must be committed:

- `zz_generated.deepcopy.go` — from `make generate`
- `config/crd/bases/*.yaml` — from `make manifests`
- `pkg/operator/assets/bindata.go` — from `make update-bindata`
- `config/rbac/role.yaml` — from `make manifests`
- `docs/api_reference.md` — from `make docs`

`make update` runs the full pipeline. `make verify` checks that generated files are fresh via `check-git-diff`. CI will reject PRs with stale generated files.

**Architecture Invariant:** never edit generated files by hand. Always use `make update`.

### Container Security

All operand containers are hardened programmatically via `updateContainerSecurityContext()`:

- `AllowPrivilegeEscalation: false`
- `Capabilities.Drop: ["ALL"]`
- `ReadOnlyRootFilesystem: true`
- `RunAsNonRoot: true`
- `SeccompProfile.Type: RuntimeDefault`

All Dockerfiles run as `USER 65534:65534` (nobody). The reconciler drift-detects container security context changes and reverts them.

### Network Policy Architecture

The operator deploys a **deny-all** base NetworkPolicy, then layers specific allow-policies (prefixed `eso-sys-`) for API server egress, webhook traffic, DNS, and conditionally cert-controller and Bitwarden. User-defined policies (prefixed `eso-user-`) are restricted to egress only.

### Go Workspace

The repo uses `go.work` with four modules: `.`, `./cmd/external-secrets-operator`, `./test`, `./tools`. This means:

- `GOFLAGS` is cleared for test and fmt targets to avoid `-mod=vendor` conflicting with `go.work`.
- Dependencies are updated across all modules via `make update-dep PKG=pkg@version`.
- Vendoring is workspace-level (`go work vendor`).
- All build-time tools are vendored and built from source.

### Relationship to Upstream

This operator manages — but does not contain — the upstream [external-secrets](https://github.com/external-secrets/external-secrets) project. The upstream project defines the CRDs that end users interact with (`ExternalSecret`, `SecretStore`, `ClusterSecretStore`, `PushSecret`, etc.) and the controllers that sync secrets from external providers. This operator's job is to deploy, configure, and lifecycle-manage those upstream components on OpenShift, providing an opinionated, security-hardened, and OLM-integrated installation.

The upstream operand version is controlled by the `RELATED_IMAGE_EXTERNAL_SECRETS` environment variable, typically set by OLM/CSV. The operator has no compile-time dependency on the upstream Go code.
