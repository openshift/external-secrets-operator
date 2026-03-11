# Test directory

This directory holds the operator’s tests and shared test utilities.

## Layout

| Path | Purpose |
|------|---------|
| **e2e/** | End-to-end tests run against a live cluster. See [e2e/README.md](e2e/README.md) for suites, labels, and prerequisites. |
| **apis/** | API integration tests (Ginkgo) run with envtest (no real cluster). See [apis/README.md](apis/README.md). |
| **utils/** | Shared helpers for e2e (and related) tests; built only with the `e2e` build tag. |

## Make targets (from repo root)

| Command | What it runs |
|---------|----------------|
| `make test-unit` | Unit tests for all packages except `test/e2e`, `test/apis`, `test/utils`. |
| `make test-apis` | API tests in `test/apis` (envtest + Ginkgo). |
| `make test` | `make test-apis` and `make test-unit` (no cluster). |
| `make test-e2e` | E2e tests in `test/e2e`; requires a cluster and optional label filter. |

See the root [README.md](../README.md) Testing section and [e2e/README.md](e2e/README.md) for more detail.
