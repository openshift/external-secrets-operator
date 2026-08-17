# Contributing to External Secrets Operator for Red Hat OpenShift

Thank you for your interest in contributing! This guide covers the PR process, review expectations, coding conventions, and testing requirements.

## Getting Started

1. Fork the repository and clone your fork.
2. Create a feature branch from `main`.
3. Set up your environment (Go version matching `go.mod`, `kubectl` v1.32.1+, access to a Kubernetes cluster for E2E tests).

## Development Workflow

### Build and Verify

```sh
make build            # Full build: codegen + manifests + fmt + vet + compile
make build-operator   # Fast rebuild (binary only, no codegen)
```

After **any** code change, regenerate and verify before committing:

```sh
make update && make verify
```

This regenerates all code, manifests, bindata, and bundle artifacts, then runs verification checks including `check-git-diff`. CI will reject PRs with stale generated files.

### Linting

```sh
make lint             # Run golangci-lint with all configured linters
make lint-fix         # Auto-fix linting issues
```

### Testing

Run the full local test suite (no cluster required):

```sh
make test             # Unit + API integration tests
```

Individual test targets:

| Target | Description |
|---|---|
| `make test-unit` | Unit tests (excludes `test/e2e`, `test/apis`, `test/utils`) |
| `make test-apis` | API validation tests via envtest |
| `make test-e2e` | End-to-end tests against a live cluster |

For E2E tests with label filters:

```sh
make test-e2e E2E_GINKGO_LABEL_FILTER="<label-filter>"
```

See [test/e2e/README.md](test/e2e/README.md) for E2E prerequisites and suite-specific instructions.

### Pre-Submission Checklist

Before opening a PR, always run:

```sh
make lint
make test
make update && make verify
```

## Coding Conventions

### Import Order

Imports must follow the order enforced by `gci` in `.golangci.yml`:

1. Standard library
2. Third-party packages
3. `github.com/openshift/external-secrets-operator` (project-local)
4. Blank imports, dot imports, aliases, local module

### File Headers

All `.go` files must include the Apache 2.0 license header from [`hack/boilerplate.go.txt`](hack/boilerplate.go.txt).

### Naming Conventions

- **Go packages**: Controller packages use `snake_case` (`external_secrets`, `crd_annotator`).
- **Controller names**: kebab-case in strings (`"external-secrets-controller"`, `"crd-annotator"`).
- **Finalizers**: `<crd-plural>.<api-group>/<controller-name>`.
- **Asset name constants**: camelCase with pattern `<resourceKind>_<resourceName>AssetName`.
- **Bindata YAML files**: `<kind-lowercase>_<resource-name>.yml` in `bindata/external-secrets/resources/`.

### Linter Rules

The repo uses golangci-lint v2 (see [`.golangci.yml`](.golangci.yml)). Key rules to be aware of:

- **depguard** blocks `math/rand` (use `math/rand/v2`), `github.com/sirupsen/logrus`, and `github.com/pkg/errors` (use `errors`/`fmt`).
- **golines** enforces a max line length of 200 characters.
- **gofmt** rewrites `interface{}` to `any`.

### Error Handling

- Wrap API call errors with `common.FromClientError`.
- Use `common.NewIrrecoverableError` for config validation failures.
- Never return both `RequeueAfter` and a non-nil error from `Reconcile`.

### Generated Files

Never edit these files by hand — always use `make update`:

- `zz_generated.deepcopy.go`
- `config/crd/bases/*.yaml`
- `pkg/operator/assets/bindata.go`
- `config/rbac/role.yaml`
- `docs/api_reference.md`

## Testing Requirements

### Unit Tests

- Add unit tests for new reconciliation logic using table-driven tests and `FakeCtrlClient`.
- Unit tests use counterfeiter-generated fakes from `pkg/controller/client/fakes/`.
- Regenerate fakes after changing the `CtrlClient` interface: `go generate ./pkg/controller/client/...`

### API Tests

- Add test cases in `api/v1alpha1/tests/<crd>/` for any new CRD field or validation rule.
- API tests run via envtest (Ginkgo).

### E2E Tests

- Add E2E test cases with appropriate Ginkgo labels for platform-specific tests.
- E2E tests require a live cluster with the operator deployed and operands stable.

## Pull Request Process

### Creating a PR

1. Keep diffs small and focused. One logical change per PR.
2. Write a clear, descriptive PR title.
3. Reference the relevant Jira ticket in your commit messages (e.g., `OCPBUGS-12345: description`).
4. Describe your changes and the motivation behind them in the PR body.

### What to Include

- **Code changes** with corresponding tests (unit, API, or E2E as appropriate).
- **Generated file updates** — run `make update` and commit the results. CI checks for staleness via `check-git-diff`.
- **Documentation updates** — update `README.md` or files under `docs/` when behavior changes are user-visible.

### Review Process

- PRs are reviewed and approved by maintainers listed in the [OWNERS](OWNERS) file.
- All CI checks must pass before merge.
- Reviewers will check for adherence to the architectural patterns documented in [AGENTS.md](AGENTS.md), including reconciler structure, resource reconciliation flow, and proper use of the `CtrlClient` interface.

### CI Expectations

CI runs the following checks (among others):

- `make verify` — vet, fmt, dependency verification, bindata verification, generated file verification, govulncheck, and `check-git-diff`.
- `make lint` — full golangci-lint suite.
- `make test` — unit and API integration tests.

Ensure all of these pass locally before pushing.

## Go Workspace

The repo uses `go.work` with four modules (`.`, `./cmd/external-secrets-operator`, `./test`, `./tools`). Key implications:

- Do **not** use `-mod=vendor` for `go test` or `go fmt` — the Makefile handles `GOFLAGS` clearing.
- To add a dependency: `make update-dep PKG=pkg@version`, then `make update-vendor`.
- All build-time tools are vendored and built from source. Do not install tools globally.

## Container Builds

The default container tool is `podman`. Override with:

```sh
CONTAINER_TOOL=docker make docker-build
```

## Additional Resources

- [AGENTS.md](AGENTS.md) — Architecture, controller details, and common pitfalls
- [docs/testing-guidelines.md](docs/testing-guidelines.md) — Detailed testing framework and patterns
- [docs/api-contracts-guidelines.md](docs/api-contracts-guidelines.md) — CRD types, kubebuilder markers, CEL validation
- [docs/error-handling-guidelines.md](docs/error-handling-guidelines.md) — ReconcileError types, retry logic, status conditions
- [docs/security-guidelines.md](docs/security-guidelines.md) — Container security, RBAC, TLS, network policies

## License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).
