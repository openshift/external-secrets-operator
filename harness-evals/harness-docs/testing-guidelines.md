# Testing Guidelines

## Make Targets

| Target | What it runs | Notes |
|---|---|---|
| `make test` | manifests + generate + fmt + vet + `test-apis` + `test-unit` | All non-e2e tests |
| `make test-unit` | Go tests excluding E2E, API, and utility packages | Standard Go tests |
| `make test-apis` | `hack/test-apis.sh` via Ginkgo + envtest | CEL/CRD validation |
| `make test-e2e` | `go test -C test -tags e2e ./e2e` (via Makefile) | Requires live cluster; prefer `make test-e2e` |

## 1. Unit Tests (pkg/controller/)

### Counterfeiter Fakes

Uses counterfeiter for `CtrlClient` interface in `pkg/controller/client/client.go`. Fakes in `pkg/controller/client/fakes/`. Regenerate with:

```bash
go generate ./pkg/controller/client/...
```

Never edit `fakes/fake_ctrl_client.go` by hand.

### Shared Test Utilities

- `pkg/controller/commontest/utils.go` — `TestExternalSecretsConfig()`, `TestExternalSecretsManager()`, `ErrTestClient` (test error variable for client failure scenarios). Use these instead of ad-hoc CR fixtures.
- `pkg/controller/external_secrets/test_utils.go` — `testReconciler(t)`, `testDeployment(name string)`, `testResourceMetadata(esc)`, typed helpers for each resource type.

### Writing a Unit Test

1. Use **stdlib `testing`**, not Ginkgo.
2. Use **table-driven tests** with `t.Run(tt.name, ...)`.
3. Call `t.Parallel()` on the outer function and each subtest.
4. Create reconciler with `testReconciler(t)`, wire `&fakes.FakeCtrlClient{}`.
5. Use `t.Setenv()` for environment variables, never `os.Setenv`.
6. Assert with `t.Fatalf` / `t.Errorf`, not testify (testify is used only in E2E `test/utils/`).

### Capturing Created/Updated Objects

```go
var capturedDeployment *appsv1.Deployment
mock.CreateCalls(func(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
    if dep, ok := obj.(*appsv1.Deployment); ok {
        capturedDeployment = dep.DeepCopy()
    }
    return nil
})
```

## 2. API Integration Tests (test/apis/)

### Data-Driven Test Suites

CRD validation tests use `.testsuite.yaml` files under `api/<version>/tests/<crdname>.<group>/` (e.g., `externalsecretsconfig.operator.openshift.io`). The generator (`test/apis/generator.go`) auto-discovers files and generates Ginkgo `DescribeTable` entries. No Go code changes needed to add test cases.

### envtest Details

- CRDs from `config/crd/bases/`
- Each `Describe` installs/uninstalls its CRD per `Ordered` group
- Requires Kube API >= 1.25 for CEL validation
- Run with `make test-apis` (sets `KUBEBUILDER_ASSETS` automatically)

## 3. E2E Tests (test/e2e/)

### Build Tag

All E2E files require `//go:build e2e`. `make test-e2e` passes `-tags e2e`.

### Ginkgo v2 Label System

Every `Context`/`Describe` must carry `Label()` decorators:

| Dimension | Values |
|---|---|
| `Platform:` | `AWS`, `GCP`, `Generic` |
| `Provider:` | `AWS`, `Vault`, `Bitwarden` |
| `Feature:` | `Proxy`, `Upgrade`, `OverrideEnv`, `NetworkPolicy`, `CustomAnnotations`, `CustomLabels`, `TrustedCABundle`, `RevisionHistoryLimit`, `UnsafeAllowGenericTargets`, `TLSAdherence` |
| `TechPreview` | Presence label for TechPreviewNoUpgrade / feature-gated suites (e.g. APIServer TLS adherence) |
| `Skipped:` | `Disconnected` |

Default filter: `Platform: isSubsetOf {AWS,Generic} && !(Feature: containsAny {Proxy, Upgrade}) && !(Provider: containsAny Bitwarden) && !TechPreview`

Override: `make test-e2e E2E_GINKGO_LABEL_FILTER='Provider:Vault'`

TLS adherence (APIServer `tlsAdherence` / `tlsSecurityProfile`, openssl probes): `make test-e2e E2E_GINKGO_LABEL_FILTER='TechPreview && Feature:TLSAdherence'`. Requires a TechPreview cluster with the `TLSAdherence` feature gate, plus `oc` and `openssl` in PATH.

> **Note:** If/when the e2e suite is restructured, update this label taxonomy and the default `E2E_GINKGO_LABEL_FILTER` to match the new suite layout.

### Embedded Testdata

Test manifests in `test/e2e/testdata/` embedded with `//go:embed testdata/*`. Use `testassets.ReadFile` to load. For template substitution use `utils.ReplacePatternInAsset("${PLACEHOLDER}", value)`.

### Condition Utilities

- `utils.VerifyPodsReadyByPrefix(ctx, clientset, ns, prefixes)` — poll for pods to reach Ready
- `utils.VerifyOperandPodsReady(ctx, clientset, ns, esc)` — verify all expected operand pods
- `utils.WaitForESOResourceReady(ctx, dynamicClient, gvr, ns, name, timeout)` — poll for `Ready=True`
- `utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, name, timeout)` — checks both `Ready=True` and `Degraded=False`

### Artifact Dumping

`AfterEach` calls `utils.DumpE2EArtifacts(...)` on failure. Artifacts written to `$ARTIFACT_DIR/e2e-artifacts/failure-<timestamp>/` include pod logs, events, and ESO custom resources.

### Test Structure Conventions

- Use `Ordered` on blocks with `BeforeAll`/`AfterAll`
- Use `BeforeAll` for expensive setup (credential checks, CR creation)
- Clean up with `defer loader.DeleteFromFile(...)` immediately after creation
- Use `retry.RetryOnConflict` when updating shared CRs
- Use `Eventually(...).Should(Succeed())` with explicit timeout and poll interval
- Use `Consistently(...)` to verify something does NOT happen

## 4. What to Test vs. What Not to Test

### Do Test

- Reconciler logic for each resource type
- Error classification (irrecoverable vs. retryable vs. user-configuration)
- Status condition generation and requeue behavior
- User-configurable fields (affinity, tolerations, env overrides, etc.)
- CRD validation rules (CEL) via `.testsuite.yaml`
- Feature flag propagation

### Do Not Test

- Counterfeiter-generated fake code
- Upstream controller-runtime / client-go behavior
- Static asset YAML content (test the Go code that processes it)

## 5. Naming Conventions

| Category | Convention |
|---|---|
| Unit test files | `<source_file>_test.go` in same package |
| Unit test functions | `TestFunctionName` with descriptive subtests |
| Shared test helpers | `test_utils.go`, `commontest/` for cross-package |
| E2E test files | `<feature>_test.go` with `//go:build e2e` |
| E2E helpers | `helpers_test.go` for internal, `test/utils/*.go` for shared |
| API test suites | `.testsuite.yaml` under `api/<version>/tests/<crdname>.<group>/` (singular CRD kind) |
