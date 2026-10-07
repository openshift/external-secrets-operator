# Contributing to External Secrets Operator

Thank you for your interest in contributing to the External Secrets Operator for Red Hat OpenShift. This guide covers the contribution workflow. For architecture details, see [README.md](README.md). For detailed domain rules, see the guideline files linked below.

## Development Environment Setup

### Prerequisites

- Go 1.26+
- podman (or docker -- set `CONTAINER_TOOL=docker` to override)
- kubectl v1.32.1+ or oc
- Access to a Kubernetes v1.32.1+ or OpenShift cluster (required only for E2E tests)

### Building

```sh
make build           # Full build: codegen, fmt, vet, compile
make build-operator  # Compile binary only (skip codegen/checks)
```

All build tooling (controller-gen, golangci-lint, envtest, etc.) is vendored and built on demand -- no manual tool installation required.

### Repository Layout

This project uses Go workspaces (`go.work`). Targets that need workspace mode (`fmt`, `vet`, `test`, `test-unit`, `test-e2e`, `run`, `update-vendor`, `update-dep`) automatically unset `GOFLAGS` to avoid conflicts with `-mod=vendor`. Run `make help` for all available targets.

### Deterministic Enforcement (Pre-Commit Hooks)

This repo ships a [`.pre-commit-config.yaml`](.pre-commit-config.yaml) with hooks for trailing whitespace, end-of-file newlines, YAML validity, merge-conflict markers, secret detection ([gitleaks](https://github.com/gitleaks/gitleaks)), and commit message format. Set it up once per clone:

```sh
pip install pre-commit
pre-commit install                        # pre-commit stage (file hygiene)
pre-commit install --hook-type commit-msg # commit message format
```

Run manually against all files:

```sh
pre-commit run --all-files
```

Note: the gitleaks hook always scans staged changes only (`gitleaks git --staged`) regardless of `--all-files` -- it doesn't accept file paths, so this command won't catch secrets outside what's staged. For a full-tree secret scan, run gitleaks directly:

```sh
gitleaks dir .
```

Claude Code sessions operating in this repo also get deterministic enforcement via [`.claude/settings.json`](.claude/settings.json): a `PostToolUse` hook runs `gofmt -w` on edited `.go` files, and a `PreToolUse` hook blocks obviously destructive Bash commands (`rm -rf`, `git push --force`, `git reset --hard`, etc.) before they execute. These hooks are specific to Claude Code and don't apply to other AI coding agents or tools. See `hack/claude-hooks/` for the hook scripts.

## Code Style and Conventions

The project enforces style through `golangci-lint` (configured in `.golangci.yml`) and `go vet`. A few key conventions:

- **Import ordering**: standard, third-party, project-local (`github.com/openshift/external-secrets-operator`). Enforced by the `gci` formatter in `.golangci.yml`.
- **String constants**: All string constants (asset names, label keys, env var names) belong in `constants.go`. Do not scatter literals.
- **Update strategy**: All resource updates use `UpdateWithRetry` (Get, set ResourceVersion, Update). Do not use Server-Side Apply. See [AGENTS.md](AGENTS.md) for details.
- **Generated files**: Never hand-edit `bindata.go`, `fake_ctrl_client.go`, `zz_generated.deepcopy.go`, or CRD YAML. Regenerate with `make manifests generate update-bindata`.

For domain-specific rules, see:

| Guideline | When to Read |
|-----------|-------------|
| [Security](harness-evals/harness-docs/security-guidelines.md) | Touching RBAC, CEL validation, container specs, network policies, TLS |
| [API Contracts](harness-evals/harness-docs/api-contracts-guidelines.md) | Modifying CRD fields, adding CEL rules, writing `.testsuite.yaml` |
| [Testing](harness-evals/harness-docs/testing-guidelines.md) | Writing unit tests, API integration tests, or E2E tests |
| [Error Handling](harness-evals/harness-docs/error-handling-guidelines.md) | Adding error paths, status conditions, or requeue logic |
| [Performance](harness-evals/harness-docs/performance-guidelines.md) | Working with caches, predicates, or reconciler concurrency |
| [Integration](harness-evals/harness-docs/integration-guidelines.md) | Touching cert-manager, OLM, proxy, or webhook integration |

## Submitting Changes

### Branch Naming

Use the Jira ticket ID as a prefix (e.g., `eso-142`, `oape-481-fix-predicates`). The Jira project can be any valid project (ESO, OAPE, etc.).

### Commit Messages

Follow the pattern used in the repository -- a scope is always required:

```text
<JIRA-ID>: <scope>: Short imperative description of the change
```

Plain colons, no parens: OpenShift CI's jira-lifecycle-plugin maps PRs to their Jira ticket via a
literal `<TICKET>:`/`NO-JIRA:` prefix, so the ticket must be followed directly by a colon rather
than a parenthesized scope.

Example: `ESO-142: controller: add proxy egress network policy`

The Jira project can be any valid project (ESO, OAPE, etc.). For changes without a Jira ticket, use `NO-JIRA: <scope>: <imperative description>` (e.g., `NO-JIRA: build: fix make verify`, `NO-JIRA: docs: update owners list`). Avoid generic messages like "fix bug" or "update code".

Commit messages are checked by the `conventional-pre-commit` and `commit-msg-jira-prefix` hooks (see [Deterministic Enforcement](#deterministic-enforcement-pre-commit-hooks) below); the latter hard-enforces the full `<TICKET-or-NO-JIRA>: <scope>: ...` shape, including the required scope segment.

### Pull Request Process

1. Fork the repository and create a branch from `main`.
2. Make your changes. Run all checks (see next section).
3. Submit a Pull Request describing what changed and why.
4. Address review feedback. Push additional commits rather than force-pushing.
5. A maintainer will merge once CI passes and the review is approved.

## Pre-Submission Checks

Run these before every PR. `make verify` is the single gate CI enforces.

```sh
make verify         # Runs vet, fmt, deps, bindata, generated files, govulncheck, markdownlint, git diff
make test           # All non-E2E tests (unit + API integration)
make lint           # golangci-lint
make lint-markdown  # markdownlint-cli2 (also part of make verify)
```

If you changed CRD types, run the full regeneration first:

```sh
make manifests generate update-bindata
```

If `make verify` reports a git diff, it means generated files are out of date. Regenerate and commit the results.

## API and Design Changes

Significant API or behavioral changes need design review before implementation.

- For user-visible API changes, new configuration surfaces, or cross-cutting behavior, open (or update) an enhancement proposal under [`openshift/enhancements/enhancements/external-secrets-operator/`](https://github.com/openshift/enhancements/tree/master/enhancements/external-secrets-operator).
- Discuss the design with maintainers (Jira + enhancement PR) and get agreement before landing CRD/API changes in this repository.
- Small additive fields that follow an existing, approved pattern may not need a full enhancement — ask maintainers if unsure.
- Component-local implementation decisions that do not warrant a cross-repo enhancement can be captured as ADRs in [`docs/decisions/`](docs/decisions/). See the catalog in [`harness-evals/harness-docs/references/enhancements.md`](harness-evals/harness-docs/references/enhancements.md).

## Adding New Features

### New CRD Fields

1. Confirm design review / enhancement status (see [API and Design Changes](#api-and-design-changes)).
2. Edit types in `api/v1alpha1/`.
3. Add CEL validation rules as needed (see [API Contracts](harness-evals/harness-docs/api-contracts-guidelines.md)).
4. Run `make manifests generate`.
5. Add `.testsuite.yaml` test cases for new CEL rules.
6. Update controller logic and add unit tests.

### New Managed Resources (Operand Manifests)

1. Add the manifest to `bindata/`.
2. Register it in `controllerManagedResources` and the `HasObjectChanged` type-switch.
3. Add it to the ordered install sequence.
4. Run `make update-bindata`.
5. See `harness-evals/harness-docs/ESO_DEVELOPMENT.md` section 2 for the full checklist.

### New Controllers

Follow the existing controller structure under `pkg/controller/`. Each controller should have its own package with dedicated unit tests. See the existing controllers (`external_secrets`, `external_secrets_manager`, `crd_annotator`) as reference.

## Testing Expectations

Follow [harness-evals/harness-docs/testing-guidelines.md](harness-evals/harness-docs/testing-guidelines.md). In short: stdlib `testing` for unit tests (table-driven, `t.Parallel()`), envtest + Ginkgo for API tests (`make test-apis`), and labeled Ginkgo E2E against a live cluster (`make test-e2e`; see [test/e2e/README.md](test/e2e/README.md)).

## Review Process

- All PRs require at least one maintainer approval.
- CI must pass (`make verify` is the primary gate).
- Reviewers will check for adherence to the domain guidelines linked above.
- For API changes, expect additional scrutiny on backward compatibility, CEL validation, and whether an [enhancement proposal](#api-and-design-changes) was completed.
- Generated file drift is a common rejection reason -- always run `make verify` locally.

## For AI Agents

Start with [AGENTS.md](AGENTS.md). Claude Code–specific notes are in [CLAUDE.md](CLAUDE.md).

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
