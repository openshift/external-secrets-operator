# AGENTS.md

## Docs Index

Detailed domain-specific guidelines are in these files -- read them before working in the corresponding area:

- [docs/security-guidelines.md](docs/security-guidelines.md) -- Container security, RBAC, TLS, network policies, webhook security
- [docs/performance-guidelines.md](docs/performance-guidelines.md) -- Cache architecture, watch predicates, reconciliation patterns, drift detection
- [docs/error-handling-guidelines.md](docs/error-handling-guidelines.md) -- ReconcileError types, retry logic, status conditions, logging
- [docs/api-contracts-guidelines.md](docs/api-contracts-guidelines.md) -- CRD types, kubebuilder markers, CEL validation, code generation
- [docs/testing-guidelines.md](docs/testing-guidelines.md) -- Unit/API/E2E test tiers, frameworks, test helpers, CI integration
- [docs/integration-guidelines.md](docs/integration-guidelines.md) -- Bindata pattern, cert-manager, OpenShift platform, Bitwarden plugin

## Project Overview

This is a Kubernetes operator (built with kubebuilder/controller-runtime) that deploys and manages the upstream [external-secrets](https://github.com/openshift/external-secrets) application on OpenShift clusters. The operator does not embed upstream code -- it manages upstream resources as static YAML manifests (bindata) applied imperatively.

Three controllers run in a single binary:

| Controller | Package | Watches | Purpose |
|---|---|---|---|
| `external-secrets-controller` | `pkg/controller/external_secrets/` | `ExternalSecretsConfig` CR + `ExternalSecretsManager` (spec) + managed resources (Deployments, RBAC, Services, Secrets metadata, ConfigMaps, NetworkPolicies, Webhooks) + conditionally `cert-manager.io/Certificate` | Installs/reconciles operand deployments, RBAC, services, webhooks, network policies |
| `external-secrets-manager` | `pkg/controller/external_secrets_manager/` | `ExternalSecretsManager` CR + `ExternalSecretsConfig` status | Aggregates controller statuses into a global status CR |
| `crd-annotator` | `pkg/controller/crd_annotator/` | ESO CRDs (metadata, label-filtered) + `ExternalSecretsConfig` | Adds cert-manager CA injection annotations to operand CRDs (conditional; only registered when cert-manager is installed) |

## Project Structure

```
api/v1alpha1/           CRD type definitions, conditions, shared types, deepcopy
api/v1alpha1/tests/     Declarative YAML test suites for CRD validation
bindata/                Static operand YAML manifests (compiled into Go via go-bindata)
bundle/                 OLM bundle manifests
cmd/external-secrets-operator/  Operator entrypoint (main.go, separate Go module)
docs/                   Guideline docs (security, performance, error handling, etc.)
config/                 Kustomize manifests (CRDs, RBAC, manager, samples, bundle)
hack/                   Shell scripts (codegen, verification, CI helpers)
images/ci/              CI Dockerfiles (coverage-instrumented builds)
pkg/controller/         Controller implementations
  client/               CtrlClient interface + counterfeiter fakes
  common/               Shared utilities (errors, constants, decode helpers, drift detection)
  commontest/           Shared test fixtures (TestExternalSecretsConfig, TestExternalSecretsManager)
  crd_annotator/        CRD annotation controller
  external_secrets/     Main operand reconciliation controller
  external_secrets_manager/  Status aggregation controller
pkg/operator/           Manager setup, controller registration
  assets/               Generated bindata (bindata.go)
pkg/version/            Build-time version info (ldflags)
test/apis/              API integration tests (Ginkgo + envtest)
test/e2e/               End-to-end tests (Ginkgo + live cluster)
test/utils/             E2E test helpers
tools/                  Go module for build-time tool dependencies
vendor/                 Workspace-level vendoring (go work vendor)
```

## Go Workspace and Module Layout

The repo uses `go.work` with four modules: `.`, `./cmd/external-secrets-operator`, `./test`, `./tools`. This means:

- `GOFLAGS` is cleared for test and fmt targets to avoid `-mod=vendor` conflicting with `go.work`.
- When adding a dependency, use `make update-dep PKG=pkg@version` to update across all modules, then `make update-vendor`.
- Vendoring is workspace-level (`go work vendor`), not per-module.
- All build-time tools (controller-gen, golangci-lint, ginkgo, etc.) are vendored and built from source via `go build -mod=vendor`.

## Build System (Key Makefile Targets)

| Target | What it does |
|---|---|
| `make build` | Full build: generate + manifests + fmt + vet + compile binary |
| `make build-operator` | Compile binary only (no codegen, fastest iteration) |
| `make test` | Run unit + API integration tests (no cluster needed) |
| `make test-unit` | Unit tests only |
| `make test-apis` | API validation tests via envtest |
| `make test-e2e` | E2E tests against a live cluster |
| `make lint` | Run golangci-lint with all configured linters |
| `make lint-fix` | Run golangci-lint with auto-fix |
| `make update` | Full regeneration: codegen + manifests + operand manifests + bindata + bundle + docs |
| `make verify` | Run vet + fmt + verify-deps + verify-bindata + verify-bindata-assets + verify-generated + govulncheck + check-git-diff |
| `make update-vendor` | Update vendor directory across all workspace modules |
| `make update-dep PKG=x@v` | Update a single dependency across all modules |
| `make manifests` | Regenerate CRD YAML, RBAC, webhook configs from kubebuilder markers |
| `make generate` | Regenerate DeepCopy methods |
| `make docs` | Regenerate API reference docs |
| `make clean` | Remove bin/, _output/, cover.out, dist/ |

After any code change, run `make update && make verify` to ensure all generated files are consistent. CI runs `check-git-diff` which fails if generated files are stale.

## Code Style and Formatting

### Import Order

Imports must follow the order enforced by `gci` in `.golangci.yml`:

1. Standard library
2. Third-party packages
3. `github.com/openshift/external-secrets-operator` (project-local)
4. Blank imports, dot imports, aliases, local module

### Linting

The repo uses golangci-lint v2 with a comprehensive set of linters (see `.golangci.yml`). Key rules:

- **depguard** blocks `math/rand` (use `math/rand/v2`), `github.com/sirupsen/logrus`, and `github.com/pkg/errors` (use `errors`/`fmt`).
- **kubeapilinter** runs only on `api/v1alpha1/*` files. Use `//nolint:kubeapilinter` with a comment for intentional deviations.
- **golines** max line length is 200 characters.
- **gofmt** rewrites `interface{}` to `any` and `a[b:len(a)]` to `a[b:]`.
- Generated files are excluded in `lax` mode.
- Test files have relaxed rules for `gocyclo`, `errcheck`, `gosec`, `forcetypeassert`, and others.

### File Headers

All `.go` files must include the Apache 2.0 license header from `hack/boilerplate.go.txt` (year 2025).

### FIPS Build

Production builds use `hack/go-fips.sh` which enables `GOEXPERIMENT=strictfipsruntime` and build tags `strictfipsruntime,openssl` when the Go compiler supports it. Local dev builds without FIPS work but cannot be used in CI/production.

## Naming Conventions

### Go Packages

Controller packages use `snake_case`: `external_secrets`, `external_secrets_manager`, `crd_annotator`. This matches kubebuilder conventions.

### Constants

- Controller names: `"external-secrets-controller"`, `"external-secrets-manager"`, `"crd-annotator"` (kebab-case in strings).
- Finalizer format: `<crd-plural>.<api-group>/<controller-name>` (e.g., `externalsecretsconfigs.operator.openshift.io/external-secrets-controller`).
- Asset name constants: `<resourceKind>_<resourceName>AssetName` in camelCase (e.g., `controllerDeploymentAssetName`).
- Environment variable constants: all-caps with suffix `EnvVarName` (e.g., `externalSecretsImageEnvVarName`).

### Bindata YAML Files

Files in `bindata/external-secrets/resources/` follow the pattern: `<kind-lowercase>_<resource-name>.yml` (e.g., `deployment_external-secrets.yml`, `clusterrole_external-secrets-controller.yml`). Network policies in `bindata/external-secrets/` use `.yaml` extension.

### CRD Object Names

Both `ExternalSecretsConfig` and `ExternalSecretsManager` are singletons named `"cluster"` (enforced by CEL). Constants `ExternalSecretsConfigObjectName` and `ExternalSecretsManagerObjectName` in `pkg/controller/common/constants.go` hold this value.

## Architectural Patterns

### Reconciler Structure

Every controller follows the same pattern:

1. A `Reconciler` struct embedding `operatorclient.CtrlClient`.
2. A `New(ctx, mgr)` constructor that builds the reconciler and client(s).
3. A `SetupWithManager(mgr)` method that wires watches, predicates, and map functions.
4. A `Reconcile(ctx, req)` method that fetches the primary CR and delegates to `processReconcileRequest`.
5. An `updateStatus(ctx, obj)` method using `retry.RetryOnConflict`.

### CtrlClient Interface

All controllers interact with Kubernetes through `pkg/controller/client.CtrlClient`, not the raw `controller-runtime client.Client`. This interface adds `UpdateWithRetry`, `StatusUpdate`, and `Exists` methods. Unit tests use counterfeiter-generated fakes (`pkg/controller/client/fakes/`).

To regenerate fakes after changing the interface: `go generate ./pkg/controller/client/...`

### Resource Reconciliation Pattern

For each resource type (deployments, services, RBAC, etc.), the same flow is used:

1. Decode static YAML from bindata (`common.Decode*ObjBytes(assets.MustAsset(...))`).
2. Mutate the decoded object (set namespace, labels, annotations, images, env vars, security context).
3. Check existence with `r.Exists()`.
4. If new: `r.Create()`. If existing: compare with `common.HasObjectChanged()`, then `r.UpdateWithRetry()` if drifted.
5. Record Kubernetes events for create/update operations.
6. Wrap errors with `common.FromClientError()`.

### Conditional Resources

Resources that depend on CR configuration use a slice of `{assetName string, condition bool}` structs. Only assets with `condition: true` are applied. Follow this pattern when adding new conditionally-created resources.

## Common Pitfalls

1. **Never return both `RequeueAfter` and a non-nil error** from `Reconcile`. Return one or the other.
2. **Never edit generated files by hand**: `zz_generated.deepcopy.go`, `config/crd/bases/*.yaml`, `pkg/operator/assets/bindata.go`, `config/rbac/role.yaml`, `docs/api_reference.md`. Always use `make update`.
3. **Always run `make update && make verify`** after any code change. CI will reject PRs with stale generated files.
4. **Use the cached client for managed resources** (those with `app: external-secrets` label). Use the uncached client only for external resources like cert-manager Issuers or user-provided Secrets.
5. **Add new watched resources to both `controllerManagedResources` and `buildCacheObjectList()`** in `pkg/controller/external_secrets/controller.go`.
6. **Add new resource types to `HasObjectChanged`** in `pkg/controller/common/utils.go` with field-level comparison, not full `DeepEqual`.
7. **Decode helpers panic on failure** -- this is intentional for build-time-constant assets. Do not add error handling around `Decode*ObjBytes` calls.
8. **RBAC markers** (`+kubebuilder:rbac`) go in `pkg/controller/external_secrets/controller.go` for the operator's own permissions. Operand RBAC is in static YAML under `bindata/`.
9. **The `go.work` workspace** means you cannot use `-mod=vendor` for `go test` or `go fmt`. The Makefile already clears `GOFLAGS` for affected targets.
10. **Container tool defaults to `podman`** (`CONTAINER_TOOL ?= podman`), not Docker. Override with `CONTAINER_TOOL=docker` if needed.

## PR and Contribution Expectations

- Run `make lint` and `make test` locally before submitting.
- Run `make verify` to ensure generated files are in sync.
- Add unit tests for new reconciliation logic using table-driven tests and `FakeCtrlClient`.
- Add API test cases in `api/v1alpha1/tests/<crd-api-group-domain>/` (e.g., `externalsecretsconfig.operator.openshift.io/`) for any new CRD field or validation rule.
- Add E2E test cases with appropriate Ginkgo labels for platform-specific tests.
- Follow existing error wrapping patterns: `common.FromClientError` for API calls, `common.NewIrrecoverableError` for config validation failures, `common.NewUserConfigurationError` for invalid user-provided configuration.
- Commit messages should reference the relevant Jira ticket (e.g., `OCPBUGS-12345: description`).
- PR reviewers/approvers are listed in `OWNERS`.

## Environment Variables

The operator reads these at runtime (typically set by OLM/CSV):

| Variable | Purpose |
|---|---|
| `RELATED_IMAGE_EXTERNAL_SECRETS` | Container image for the external-secrets operand |
| `RELATED_IMAGE_BITWARDEN_SDK_SERVER` | Container image for the Bitwarden SDK server |
| `OPERAND_EXTERNAL_SECRETS_IMAGE_VERSION` | Version label for operand resources |
| `BITWARDEN_SDK_SERVER_IMAGE_VERSION` | Version label for Bitwarden resources |
| `OPERATOR_IMAGE_VERSION` | Operator version string |
| `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY` | Proxy fallback from OLM environment |

For local development, set at minimum `RELATED_IMAGE_EXTERNAL_SECRETS` via `t.Setenv()` in tests or shell export for `make run`.
