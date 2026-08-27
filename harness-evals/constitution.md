<!-- Companion artifact: repo-assessment.md (target files, reusable assets, risks) -->
# External Secrets Operator Constitution

**AgentRoutingMode:** PROVIDED
<!-- PROVIDED — AGENTS.md exists at repo root -->

**Version**: 1.0.1 | **Ratified**: 2026-07-01 | **Last Amended**: 2026-08-27

## Core Principles

### I. Upstream Operand Separation — Do Not Fork Upstream Logic

The operator deploys and manages upstream **external-secrets** and the optional **bitwarden-sdk-server** plugin via **embedded manifests in `bindata/external-secrets/`**. The operator **never** reimplements upstream secret-sync logic (provider authentication, ExternalSecret reconciliation, generator behavior, Bitwarden SDK protocol). Operator packages reconcile operator CRs and deploy/configure operand workloads only.

**Evidence:** `bindata/external-secrets/resources/` — operand YAML from upstream helm; `pkg/controller/external_secrets/` installs deployments/RBAC/webhooks but contains zero provider-specific secret-fetch logic. `README.md` states the operator uses upstream helm charts.

### II. Operand Workloads — Core, Conditional TLS Helper, Plugin

| Workload | Always? | Controlled by | Image env |
|----------|---------|---------------|-----------|
| **external-secrets** (core controller) | Yes | `ExternalSecretsConfig` | `RELATED_IMAGE_EXTERNAL_SECRETS` |
| **external-secrets-webhook** | Yes | `ExternalSecretsConfig` | `RELATED_IMAGE_EXTERNAL_SECRETS` |
| **external-secrets-cert-controller** | No — when cert-manager is **disabled** | `ExternalSecretsConfig.spec.controllerConfig.certProvider` | `RELATED_IMAGE_EXTERNAL_SECRETS` |
| **bitwarden-sdk-server** (provider plugin) | No — when `plugins.bitwardenSecretManagerProvider.mode == Enabled` | `ExternalSecretsConfig.spec.plugins` | `RELATED_IMAGE_BITWARDEN_SDK_SERVER` |

New plugin workloads MUST follow the bitwarden pattern: API under `spec.plugins`, bindata deployment asset, conditional entry in `createOrApplyDeployments`, dedicated network policy, TLS via `certProvider` or `secretRef`.

**Evidence:** `pkg/controller/external_secrets/deployments.go` — conditional deployment table; `pkg/controller/external_secrets/constants.go` — `bitwardenDeploymentAssetName`, image env var names; `api/v1alpha1/external_secrets_config_types.go` — `BitwardenSecretManagerProvider`, CEL rules for TLS prerequisites. See also §VII (webhook TLS mutual exclusion).


### III. Controller-Runtime Only — Single Manager, Three Reconcilers

All controllers use **`sigs.k8s.io/controller-runtime`** on **one** shared manager. Register reconcilers in `pkg/operator/setup_manager.go` only. Do not introduce library-go, informer factories, or separate managers.

**Evidence:** `pkg/operator/setup_manager.go` — wires `external_secrets_manager`, `external_secrets`, and optional `crd_annotator`; `cmd/external-secrets-operator/main.go` — single `ctrl.Manager`; `go.mod` — `sigs.k8s.io/controller-runtime` (version from `go.mod`), no `openshift/library-go`.

### IV. Create-or-Update Reconciliation — Not SSA

Operand resources are reconciled via **create-or-update with deep equality** (`createWithFallback`, `common.HasObjectChanged`, `UpdateWithRetry`). Co-managed resources (Secret, ConfigMap) use `patchResourceMetadata` for metadata-only JSON Patch. Do **not** introduce Server-Side Apply (`client.Apply`) or convert operand reconcilers to SSA-first patterns.

**Evidence:** `pkg/controller/external_secrets/install_external_secrets.go`, `pkg/controller/common/utils.go` — `HasObjectChanged()`; `pkg/controller/external_secrets/utils.go` — `patchResourceMetadata()` (JSON Patch).

### V. Singleton CR Convention — Name `cluster`, One Per Kind

Operator CRs `ExternalSecretsConfig` and `ExternalSecretsManager` are **cluster-scoped singletons named `cluster`**. The operator auto-creates default `ExternalSecretsManager` named `cluster`. CEL validation enforces singleton naming.

**Evidence:** `pkg/controller/common/constants.go` — `ExternalSecretsConfigObjectName` and `ExternalSecretsManagerObjectName` = `"cluster"`; `pkg/operator/setup_manager.go` — `CreateDefaultESMResource()`; `README.md` — auto-creates `externalsecretsmanagers.operator.openshift.io` named `cluster`.

### VI. Feature Flags on ExternalSecretsManager — Not OpenShift FeatureGate API

Runtime feature toggles are defined on `ExternalSecretsManager.Spec.Features[]` with typed `FeatureName` values. Check via `common.IsFeatureEnabled()`. Do not add OpenShift cluster FeatureGate discovery or `pkg/features/` patterns from other operators.

**Evidence:** `api/v1alpha1/external_secrets_manager_types.go` — `Feature` slice; `pkg/controller/common/utils.go` — `IsFeatureEnabled()`; `pkg/controller/external_secrets/constants.go` — `featureContainerArgs` map.

### VII. Webhook TLS — cert-manager or In-Tree cert-controller (Mutually Exclusive)

Webhook TLS uses either cert-manager `Certificate` CRs (`certProvider.certManager.mode == Enabled`) **or** the in-tree `external-secrets-cert-controller` deployment — never both. cert-controller deployment is skipped when cert-manager path is active.

**Evidence:** `pkg/controller/external_secrets/deployments.go` — `certControllerDeploymentAssetName` condition `!isCertManagerConfigEnabled(esc)`; `pkg/controller/external_secrets/certificate.go`; `bindata/external-secrets/resources/certificate_external-secrets-webhook.yml` vs `bindata/external-secrets/resources/secret_external-secrets-webhook.yml`.

### VIII. Bindata / Manifest Regeneration — Never Hand-Edit, Always `make update`

Operand manifests under `bindata/` and generated code (`zz_generated.deepcopy.go`, `pkg/operator/assets/bindata.go`, CRD YAML under `config/crd/bases/`) are **generated artifacts**. Changes require `make update-operand-manifests` (helm pipeline) and/or `make generate && make manifests && make update-bindata`. CI verification (`make verify`) fails if outputs are stale.

**Evidence:** `hack/update-external-secrets-manifests.sh`; `Makefile` — `EXTERNAL_SECRETS_VERSION`, `update`, `verify`, `verify-bindata`, `verify-generated`; `pkg/operator/assets/bindata.go` — generated.

### IX. Verification-First Development — `make verify && make lint && make test`

All changes MUST pass:

- **`make verify`** — required pre-submit CI gate (fmt, vet, deps, bindata, generated files, govulncheck, markdownlint, git diff; `check-git-diff` runs `make update` for manifests/generate/bindata freshness)
- **`make test`** — unit + API integration tests (`test-unit`, `test-apis`)
- **`make lint`** — golangci-lint + kube-api-linter

E2E (`make test-e2e`, build tag `e2e`) requires a live cluster.

**Evidence:** `Makefile` — `verify`, `test`, `test-unit`, `test-apis`, `test-e2e`, `lint` targets; `.golangci.yml` — linter configuration with kube-api-linter plugin.

### X. RBAC Least Privilege — Explicit Operator and Operand Manifests

Operator RBAC is in `config/rbac/`. Operand RBAC is embedded in `bindata/external-secrets/resources/` and applied by `pkg/controller/external_secrets/rbacs.go`. New permissions MUST be explicit ClusterRole rules in bindata or operator RBAC — not broad cluster-admin grants.

**Evidence:** `config/rbac/role.yaml`; `bindata/external-secrets/resources/` — per-component RBAC YAML; `pkg/controller/external_secrets/rbacs.go`.

### XI. OLM Bundle and Related Images

The operator ships via OLM (`bundle/`, `config/manifests/`). Operand version is pinned in `Makefile` (`EXTERNAL_SECRETS_VERSION`). Images:
- `RELATED_IMAGE_EXTERNAL_SECRETS` / `OPERAND_EXTERNAL_SECRETS_IMAGE_VERSION` — core operand
- `RELATED_IMAGE_BITWARDEN_SDK_SERVER` / `BITWARDEN_SDK_SERVER_IMAGE_VERSION` — Bitwarden plugin

Missing image env vars cause irrecoverable errors.

**Evidence:** `Makefile` — `IMG_VERSION`, `EXTERNAL_SECRETS_VERSION`, `bundle` target; `bundle/manifests/openshift-external-secrets-operator.clusterserviceversion.yaml`; `pkg/controller/external_secrets/constants.go`.

### XII. Namespace Conventions

Operator runs in `external-secrets-operator` namespace. Operand runs in `external-secrets` namespace (`OperandDefaultNamespace`). Both are established conventions in controller constants and README.

**Evidence:** `README.md` — "operator runs in `external-secrets-operator` namespace"; `pkg/controller/external_secrets/constants.go` — `OperandDefaultNamespace`.

## Additional Constraints

- **Go version**: Match `go.mod`. — **Evidence:** `go.mod`
- **Workspace**: Multi-module `go.work` (root, `cmd/external-secrets-operator`, `test`, `tools`). Vendor via `make update-vendor`. — **Evidence:** `go.work`, `vendor/`
- **Import ordering**: Local prefix `github.com/openshift/external-secrets-operator`. — **Evidence:** `.golangci.yml` `local-prefixes`
- **FIPS**: Production builds use `hack/go-fips.sh` (`GOEXPERIMENT=strictfipsruntime`, `CGO_ENABLED=1`). — **Evidence:** `Makefile` `build-operator`, `hack/go-fips.sh`
- **Container image**: Operator image from `Dockerfile` / `images/ci/`; operand images from `RELATED_IMAGE_*` env vars. — **Evidence:** `Dockerfile`, CSV relatedImages
- **Test frameworks**: Standard `testing` + counterfeiter fakes in `pkg/`; Ginkgo v2 + envtest in `test/apis/`; Ginkgo + live cluster in `test/e2e/` (tag `e2e`). — **Evidence:** `pkg/controller/client/fakes/`, `test/apis/`, `test/e2e/`
- **CI system**: Prow via `openshift/release`; in-repo verify via `make verify`. — **Evidence:** `README.md` contributing section
- **Optional cert-manager**: Webhook TLS may use cert-manager `Certificate` CRs when cert-manager is installed; `crd_annotator` is conditional. Never assume cert-manager is present. — **Evidence:** `pkg/operator/setup_manager.go`, `pkg/controller/crd_annotator/`

## Development Workflow

| Activity | Requirement | Evidence |
|----------|-------------|----------|
| Local unit tests | `make test-unit` or full `make test` | `Makefile` |
| API validation tests | `make test-apis` after CRD/testsuite changes | `hack/test-apis.sh`, `test/apis/` |
| Full verify | `make verify` | `Makefile` `verify` target |
| Lint | `make lint` | `Makefile`, `.golangci.yml` |
| Codegen refresh | `make generate && make manifests` after API edits | `Makefile` |
| Operand bump | `make update-operand-manifests && make update-bindata` | `hack/update-external-secrets-manifests.sh` |
| E2E tests | `make test-e2e` (cluster required); filter via `E2E_GINKGO_LABEL_FILTER` | `Makefile` `test-e2e` |
| Bundle generation | `make bundle` after CSV/CRD changes | `Makefile` `bundle` |
| PR pre-merge | `make test && make verify && make lint`; commit generated outputs | `AGENTS.md` verification matrix |

## Agent Routing

| Agent ID | Scope | When to route |
|----------|-------|---------------|
| **API_Agent** | `api/v1alpha1/`, testsuite YAML | CRD types, validation, markers |
| **OperatorController_Agent** | `pkg/controller/external_secrets/` (incl. `pkg/controller/external_secrets/trusted_ca_bundle.go`), `pkg/controller/external_secrets_manager/`, `pkg/controller/crd_annotator/`, `pkg/operator/setup_manager.go` | Core operand reconciliation, wiring, user trusted CA |
| **ManifestsBindata_Agent** | `bindata/`, `hack/update-external-secrets-manifests.sh`, operand CRDs | Operand manifest refresh, version pins |
| **BitwardenPlugin_Agent** | `Plugins.BitwardenSecretManagerProvider`, bitwarden bindata assets | Bitwarden SDK plugin workload |
| **WebhookTLS_Agent** | `pkg/controller/external_secrets/certificate.go`, webhook deployments under `bindata/external-secrets/resources/` | Webhook TLS paths (cert-manager Certificate vs in-tree cert-controller) |
| **RBACSecurity_Agent** | `config/rbac/`, `pkg/controller/external_secrets/rbacs.go`, `pkg/controller/external_secrets/networkpolicy.go` | RBAC and network policy |
| **OLMRelease_Agent** | `bundle/`, `config/manifests/` | CSV, relatedImages |
| **Testing_Agent** | `test/e2e/`, `test/apis/` | Test authoring |
| **Docs_Agent** | `README.md`, `docs/`, `harness-evals/harness-docs/` | User-facing and agentic docs |

Full routing detail: `AGENTS.md` (repo root).

## Governance

- This constitution supersedes ad-hoc conventions for downstream Planning, Task Creation, and Code Generation agents.
- **Amendments:** require documented evidence of repo change; bump Version and Last Amended date.
- **Conflicts:** if spec contradicts constitution, escalate in plan.md §8 — do not silently override. Forking upstream external-secrets logic into the operator is a constitution violation.
- **Companion docs:**
  - **AGENTS.md** takes precedence for agent routing, controller map, Make targets, and test patterns.
  - **README.md** takes precedence for human-facing install and contributing procedures.
  - **This constitution** takes precedence for architectural principles and non-negotiable guardrails.
- **Complexity:** new patterns must justify deviation from existing repo conventions. Adding a second controller framework or SSA-first operand reconciliation requires constitution amendment.
