# External Secrets Operator — Development Guide

> **Generic Development Practices**: See [Platform Development Practices](https://github.com/openshift/enhancements/tree/master/ai-docs) for Go standards, controller-runtime patterns, and CI/CD workflows.

This guide covers **ESO-specific** development practices.

## Quick Start

### Prerequisites

- Go 1.26.0+ (from go.mod)
- Access to OpenShift cluster with `KUBECONFIG` set
- Podman (default container tool; Docker supported via `CONTAINER_TOOL=docker`)
- `yq` (installed via `make ensure-yq`)

### Build

```bash
make build-operator          # Build operator binary
make build                   # Build all binaries
make image-build             # Build container image (podman)
```

**Binary output**: `bin/external-secrets-operator`
**Version ldflags**: commitFromGit, versionFromGit, majorFromGit, minorFromGit, buildDate injected at build time.

## Key Makefile Targets

| Target | Purpose |
|--------|---------|
| `make build-operator` | Build operator binary |
| `make test` | Run all non-e2e tests (manifests + generate + fmt + vet + test-apis + test-unit) |
| `make test-unit` | Run unit tests only (excludes e2e, apis, utils) |
| `make test-apis` | Run envtest-based API validation tests |
| `make manifests` | Generate CRDs and RBAC via controller-gen |
| `make generate` | Generate deepcopy methods |
| `make update-operand-manifests` | Re-render upstream Helm → bindata manifests |
| `make bundle` | Generate OLM bundle |
| `make verify` | Run all verification checks |
| `make lint` | Run golangci-lint |
| `make fmt` | Format Go code |
| `make vet` | Run go vet |
| `make govulncheck` | Run Go vulnerability scanner |
| `make verify-deps` | Verify dependency integrity |

## Common Tasks

### 1. Bump Upstream External-Secrets Version

```bash
# 1. Update EXTERNAL_SECRETS_VERSION in Makefile
# 2. Re-render manifests from upstream Helm charts
make update-operand-manifests

# 3. Regenerate bindata
make update-bindata

# 4. Update IMG_VERSION in Makefile if needed
# 5. Review generated diffs in bindata/ and config/crd/bases/
# 6. Run tests
make test verify
```

The `hack/update-external-secrets-manifests.sh` script:
- Downloads upstream Helm charts
- Renders templates (cert-manager enabled + disabled variants)
- Strips Helm labels, relabels `managed-by`
- Customizes core deployment (disables leader election, cluster-store/push-secret reconcilers)
- Refreshes helm-rendered files under `bindata/external-secrets/operand/` (preserves `namespace_external-secrets.yml` and `certificate_bitwarden-tls-certs.yml`)
- Network policies live under `bindata/external-secrets/networkpolicies/` and are not touched by the script

### 2. Add a New Managed Resource Type

1. Add bindata YAML to `bindata/external-secrets/operand/`
2. Run `make update-bindata` so `pkg/operator/assets/bindata.go` picks up the new asset
3. Add asset name constant in `pkg/controller/external_secrets/constants.go`
4. Add `Decode*ObjBytes` function in `pkg/controller/common/utils.go`
5. Add creation/update logic in a new file under `pkg/controller/external_secrets/`
6. Add to `controllerManagedResources` in `controller.go` if the resource should be cleaned on delete
7. Add the type to `buildCacheObjectList()` when it should be watched via the label-filtered cache
8. Add the resource type to `HasObjectChanged` type-switch in `common/utils.go`
9. Wire into the installation order in `install_external_secrets.go`
10. Add unit tests
11. Run `make verify` before opening the PR

### 3. Add a New Feature Toggle

1. Add constant to `FeatureName` enum in `api/v1alpha1/meta.go`
2. Add kubebuilder enum validation marker
3. Map feature to container arg in `featureContainerArgs` in `constants.go`
4. If feature affects only specific deployments, add to that deployment's `supportedFeatures` slice
5. Run `make manifests generate` to regenerate CRDs and deepcopy

### 4. Modify CRD API Types

```bash
# 1. Edit types in api/v1alpha1/
# 2. Regenerate
make manifests generate

# 3. If CEL validation added, add test cases in test/apis/
make test-apis

# 4. Verify no git diff in generated files
make verify
```

### 5. Update RBAC Permissions

```bash
# 1. Edit RBAC markers on controller methods (//+kubebuilder:rbac:...)
# 2. Regenerate
make manifests

# 3. Review config/rbac/role.yaml changes
```

## Go Workspace

This repo uses Go workspaces (`go.work`) with 4 modules:
- `.` (root)
- `cmd/external-secrets-operator`
- `test`
- `tools`

`GOFLAGS` is overridden in the Makefile for workspace mode.

## Key Environment Variables

| Variable | Purpose | Set By |
|----------|---------|--------|
| `RELATED_IMAGE_EXTERNAL_SECRETS` | Operand image | `config/manager/manager.yaml`, OLM |
| `RELATED_IMAGE_BITWARDEN_SDK_SERVER` | Bitwarden image | `config/manager/manager.yaml`, OLM |
| `OPERAND_EXTERNAL_SECRETS_IMAGE_VERSION` | Version tracking | Makefile |
| `BITWARDEN_SDK_SERVER_IMAGE_VERSION` | Version tracking | Makefile |
| `OPERATOR_IMAGE_VERSION` | Operator version | `config/manager/manager.yaml` |

## Common Mistakes

1. **DO NOT hand-edit `pkg/operator/assets/bindata.go`** — it is generated by `make update-bindata`
2. **DO NOT hand-edit `pkg/controller/client/fakes/fake_ctrl_client.go`** — it is generated by counterfeiter
3. **DO NOT introduce Server-Side Apply** — the codebase uses `UpdateWithRetry`; SSA would conflict with cert-manager/CNO field ownership on co-managed resources
4. **DO NOT skip `make verify` before PRs** — it catches generated file drift
5. **DO NOT assume all deployments exist** — cert-controller is conditional (only when cert-manager disabled), bitwarden is conditional (only when enabled)
6. **DO NOT use reserved annotation domains** — `kubernetes.io/`, `openshift.io/`, `cert-manager.io/`, `k8s.io/` are blocked by CEL validation
7. **DO NOT modify cert-manager config fields after creation** — `mode`, `injectAnnotations`, `issuerRef` are immutable via CEL
8. **DO NOT return both `RequeueAfter` and a non-nil error** from `Reconcile` — return one or the other
9. **DO NOT forget `buildCacheObjectList()`** when adding watched/managed resource types — also update `controllerManagedResources` and `HasObjectChanged`
10. **DO NOT wrap `Decode*ObjBytes` in error handling** — those helpers panic on failure by design for build-time-constant assets
11. **DO NOT put operand RBAC in `+kubebuilder:rbac` markers** — operator permissions go in controller Go markers; operand RBAC is static YAML under `bindata/`

## FIPS

`make build-operator` / `make build` source `hack/go-fips.sh`, which enables `GOEXPERIMENT=strictfipsruntime` and build tags `strictfipsruntime,openssl` when the Go compiler supports it.

Container image builds (`make image-build` / `Dockerfile`) currently run `CGO_ENABLED=0 go build` **without** the FIPS script or FIPS tags. Treat the root `Dockerfile` path as a non-FIPS local/dev image build unless/until it is wired through the FIPS build path (CI operand images under `images/ci/` use a separate FIPS-oriented Dockerfile).

## See Also

- [Testing Guide](./ESO_TESTING.md)
- [Architecture](./architecture/components.md)
- [Platform Development Practices](https://github.com/openshift/enhancements/tree/master/ai-docs)

## SME Review Recommended

- Detailed steps for adding a new operand component end-to-end (deployment + service + RBAC + network policy + bindata)
- CI/CD pipeline specifics beyond `.ci-operator.yaml`
- Release process for operator version bumps
