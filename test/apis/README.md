# API integration tests

API tests exercise the operator’s APIs (CRDs, webhooks, controllers) using **envtest** — a local control plane (no real cluster required). They are implemented with [Ginkgo](https://onsi.github.io/ginkgo/).

## Running

From the repository root:

```bash
make test-apis
```

This installs envtest binaries if needed, sets `KUBEBUILDER_ASSETS`, and runs Ginkgo in `test/apis` with a 30-minute timeout. In OpenShift CI, when `OPENSHIFT_CI=true` and `ARTIFACT_DIR` is set, JUnit output and coverage are written to `ARTIFACT_DIR`.

## Requirements

- `make test-apis` pulls the correct Kubernetes version via `envtest`; ensure `make test-apis` (or `make test`) has been run at least once so envtest assets are present.

## Relation to other tests

- **Unit tests** (`make test-unit`): Pure Go tests excluding `test/e2e`, `test/apis`, and `test/utils`.
- **E2e tests** (`make test-e2e`): Run against a real cluster; see [e2e/README.md](e2e/README.md).

`make test` runs both unit and API tests (no cluster).
