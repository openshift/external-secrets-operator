# CLAUDE.md

@AGENTS.md

## Build and Test Commands

When working in this repository, use these commands:

### Before Committing
```bash
make update && make verify
```
This regenerates all code, manifests, and bindata, then runs verification checks. CI will reject PRs with stale generated files.

### Development Workflow
```bash
make build          # Full build with codegen
make build-operator # Fast rebuild (binary only, no codegen)
make test           # Run unit + API tests (no cluster needed)
make lint           # Run golangci-lint
make lint-fix       # Auto-fix linting issues
```

### Testing
```bash
make test-unit      # Unit tests only
make test-apis      # API validation tests (envtest)
make test-e2e       # E2E tests (requires cluster)
```

### Dependency Management
```bash
make update-vendor                    # Sync vendor across all workspace modules
make update-dep PKG=package@version   # Update a dependency in all modules
```

## Claude Code Preferences

- Always run `make update && make verify` after code changes that affect:
  - CRD definitions (`api/v1alpha1/`)
  - Kubebuilder markers (`+kubebuilder:*`)
  - Bindata YAML (`bindata/`)
  - Generated code triggers

- Use `make lint-fix` to automatically fix formatting and linting issues before suggesting manual fixes

- The repository uses a Go workspace (`go.work`) with 4 modules. Never manually edit `GOFLAGS` or use `-mod=vendor` in test/fmt commands — the Makefile handles this

- Container builds default to `podman`. Override with `CONTAINER_TOOL=docker` if needed

- All build-time tools are vendored. Do not suggest installing tools globally
