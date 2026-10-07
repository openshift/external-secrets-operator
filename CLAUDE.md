@AGENTS.md

## Build & Test Commands

### Build

```bash
make build              # Build operator with manifests, generate, fmt, vet
make build-operator     # Build operator binary only (no checks)
make image-build        # Build container image
```

### Test

```bash
make test               # Run all non-e2e tests (test-apis + test-unit)
make test-unit          # Run unit tests only
make test-apis          # Run API integration tests (envtest + Ginkgo)
make test-e2e           # Run e2e tests (requires live cluster)
```

### Lint & Verify

```bash
make lint               # Run golangci-lint
make lint-fix           # Run golangci-lint with auto-fix
make lint-markdown      # Run markdownlint-cli2 on docs
make lint-markdown-fix  # Auto-fix markdownlint findings where possible
make verify             # Run vet, fmt, deps, bindata, generated files, govulncheck, markdownlint, git diff
make fmt                # Run go fmt
make vet                # Run go vet
```

### Code Generation

```bash
make manifests          # Generate CRDs and RBAC
make generate           # Generate deepcopy methods
make update-bindata     # Regenerate bindata.go from bindata/
make update             # Run generate, manifests, update-operand-manifests, update-bindata, bundle, docs
```

### Dependency Management

```bash
make update-vendor      # Update vendor directory for all workspace modules
make update-dep PKG=... # Update a dependency across all modules
make verify-deps        # Verify go.mod dependencies
```

## Claude Code Behavioral Preferences

### Commit Messages

Always include the Jira ticket number, a scope, and a clear imperative description. Format:

```text
<JIRA-ID>: <scope>: short description of the change
```

Plain colons, no parens — OpenShift CI's jira-lifecycle-plugin maps PRs to Jira via a literal `<TICKET>:`/`NO-JIRA:` prefix, which a parenthesized scope would break.

Example: `ESO-142: controller: add proxy egress network policy`. A scope is always required. The Jira project can be any valid project (ESO, OAPE, etc.). If no Jira ticket exists, ask the user for context, or use `NO-JIRA: <scope>: <imperative description>` (e.g., `NO-JIRA: docs: fix typo in README`). Enforced by the `commit-msg-jira-prefix` pygrep hook in `.pre-commit-config.yaml`. Never use generic messages like "fix bug", "update code", or "address review comments".

### Pre-Commit Workflow

Always run `make verify` before committing. This is the single CI gate that catches:
- Generated file drift (bindata, deepcopy, CRDs)
- Dependency inconsistencies
- Go formatting and vet issues
- Vulnerability scan failures

### Go Workspace Mode

This repository uses Go workspaces (`go.work`). Commands that need workspace mode (`fmt`, `vet`, `test`, `test-unit`, `test-e2e`, `run`, `update-vendor`, `update-dep`) automatically unset `GOFLAGS` to avoid conflicts with `-mod=vendor`.

### Generated Files — Never Hand-Edit

- `pkg/operator/assets/bindata.go` (regenerate with `make update-bindata`)
- `pkg/controller/client/fakes/fake_ctrl_client.go` (regenerate with `go generate ./pkg/controller/client/...`)
- `zz_generated.deepcopy.go` files (regenerate with `make generate`)
- CRD YAML in `config/crd/bases/` (regenerate with `make manifests`)

### Container Tool

Default is `podman`. Override with `CONTAINER_TOOL=docker` if needed.

### E2E Test Filtering

Default filter excludes Proxy, Upgrade, and Bitwarden tests. Override with:

```bash
make test-e2e E2E_GINKGO_LABEL_FILTER='Provider:Vault'
```

## File Exclusions

When reading or editing code, skip these auto-generated files:
- `pkg/operator/assets/bindata.go`
- `pkg/controller/client/fakes/`
- `**/zz_generated.deepcopy.go`
- `config/crd/bases/*.yaml`
