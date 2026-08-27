# External Secrets Operator — Agentic Documentation

**Component**: External Secrets Operator for Red Hat OpenShift
**Repository**: openshift/external-secrets-operator

> **Platform Patterns**: See [openshift/enhancements/ai-docs/](https://github.com/openshift/enhancements/tree/master/ai-docs) for operator patterns, testing practices, security guidelines, and cross-repo ADRs.

## What is ESO?

Manages the lifecycle of the upstream [external-secrets](https://github.com/external-secrets/external-secrets) project on OpenShift. Deploys and configures the operand via static YAML manifests embedded as bindata — it is not a fork.

**Key Principle**: The operator owns the operand deployment; users configure via two singleton CRs (`ExternalSecretsConfig`, `ExternalSecretsManager`), both named `cluster`.

## Core Components

| Component | Purpose | Location |
|-----------|---------|----------|
| ExternalSecrets Controller | Operand lifecycle (install, update, delete) | `pkg/controller/external_secrets/` |
| ESM Controller | Status aggregation, default ESM creation | `pkg/controller/external_secrets_manager/` |
| CRD Annotator | cert-manager CA injection on CRDs (conditional) | `pkg/controller/crd_annotator/` |

**Quick Start**: `oc get esc cluster -o yaml` | `oc get esm cluster -o yaml` | `oc get pods -n external-secrets`

## Critical Patterns

1. **NOT Server-Side Apply** — all updates use `UpdateWithRetry` (Get → set ResourceVersion → Update). Co-managed resources (Secret, ConfigMap) use `patchResourceMetadata` for metadata-only JSON Patch. Never introduce SSA.
2. **Bindata pipeline** — operand manifests are pre-rendered from upstream Helm charts at build time (`hack/update-external-secrets-manifests.sh`), embedded via `openshift/build-machinery-go`. `pkg/operator/assets/bindata.go` is generated — **never hand-edit**.
3. **Immutable cert-manager fields** — `mode`, `injectAnnotations`, `issuerRef` in CertManagerConfig are immutable via CEL `self == oldSelf`. Network policy entries (name+componentName) also cannot be removed once added.

## Domain Guidelines

Detailed rules for each domain are in `harness-evals/harness-docs/`. Read the relevant file before modifying that area.

| Guideline | Scope |
|-----------|-------|
| [Security](harness-evals/harness-docs/security-guidelines.md) | CEL validation, annotation/label restrictions, container hardening, RBAC, network policies, TLS |
| [Performance](harness-evals/harness-docs/performance-guidelines.md) | Label-filtered caches, change detection, event predicates, requeue strategy, concurrency |
| [Error Handling](harness-evals/harness-docs/error-handling-guidelines.md) | Error classification (Irrecoverable/Retry/UserConfig), status conditions, requeue matrix, events |
| [API Contracts](harness-evals/harness-docs/api-contracts-guidelines.md) | Singleton enforcement, field immutability, CEL rules, list map keys, `.testsuite.yaml` patterns |
| [Testing](harness-evals/harness-docs/testing-guidelines.md) | Unit tests, API integration tests (envtest), E2E with Ginkgo labels, make targets |
| [Integration](harness-evals/harness-docs/integration-guidelines.md) | cert-manager, OLM, proxy, CNO trusted CA, console, metrics, multi-arch, webhooks |

## Cross-Cutting Conventions

- **Generated files**: Never hand-edit `bindata.go`, `fake_ctrl_client.go`, `zz_generated.deepcopy.go`, or CRD YAML in `config/crd/bases/`. Regenerate with `make manifests generate update-bindata` or `go generate`.
- **Go style**: stdlib `testing` only (no Ginkgo for unit tests, no testify except E2E utils). Table-driven tests with `t.Run`. Call `t.Parallel()` on outer function and each subtest. Use `t.Setenv()` instead of `os.Setenv`.
- **Constants**: All string constants (asset names, label keys, env var names) live in `constants.go`. Do not scatter literals across source files.
- **New managed resources**: Must be added to `controllerManagedResources`, `buildCacheObjectList()`, `HasObjectChanged` type-switch, and the ordered install sequence. See `harness-evals/harness-docs/ESO_DEVELOPMENT.md` section 2.
- **Commit messages**: Always include the Jira ticket number and a clear imperative description. Format: `<JIRA-ID>: short description` (e.g., `ESO-142: add proxy egress network policy`). The Jira project can be any valid project (ESO, OAPE, etc.). If no Jira ticket exists, use a descriptive imperative summary. Never use generic messages like "fix bug" or "update code".
- **PR checklist**: Run `make verify` (vet, fmt, deps, bindata, generated files, govulncheck, markdownlint, git diff), `make test`, and `make lint` before submitting. `make verify` is the single gate that CI enforces.

## Common Pitfalls

1. **Never return both `RequeueAfter` and a non-nil error** from `Reconcile` — return one or the other.
2. **Use the cached client for managed resources** (`app=external-secrets`). Use `UncachedClient` only for objects outside the cache (cert-manager Issuers, user-provided Secrets).
3. **`Decode*ObjBytes` helpers panic on failure** — intentional for build-time-constant assets; do not wrap them in error handling.
4. **Operator RBAC markers** (`+kubebuilder:rbac`) go in controller Go files; operand RBAC lives in static YAML under `bindata/`.
5. More contributor pitfalls: `harness-evals/harness-docs/ESO_DEVELOPMENT.md` → Common Mistakes.

## Documentation Structure

```text
harness-evals/harness-docs/
├── *-guidelines.md            # Enforceable domain guidelines (security, testing, API, …)
├── domain/                    # ExternalSecretsConfig, ExternalSecretsManager API docs
├── architecture/              # Controller internals, resource management, bindata pipeline
│   └── components.md
├── decisions/                 # Component-specific ADRs (bindata, update strategy, NP naming)
├── exec-plans/                # Feature planning
├── references/
│   ├── ecosystem.md           # Links to Platform patterns
│   └── enhancements.md        # Enhancement proposals catalog
├── ESO_DEVELOPMENT.md         # Development workflows, build targets, common tasks
└── ESO_TESTING.md             # Test suites, patterns, E2E labels
```

**AI Agent Path**: `harness-evals/harness-docs/*-guidelines.md` (as needed) → `domain/` → `architecture/` → `decisions/` → `ESO_DEVELOPMENT.md`

## OpenSpec (planning / evals)

| Path | Role |
|------|------|
| `AGENTS.md` (this file) | Agent routing + architecture for `/opsx-*` |
| `harness-evals/harness-docs/` | Source docs for `/opsx-constitute` |
| `harness-evals/constitution.md` | Guardrails required before `plan.md` |
| `harness-evals/evals/` | Stage eval quality gates (`/opsx-continue`, `/opsx-apply`) |
| `eval-generation/` | Retrospective `/eval-loop` (fill `input/feature-bundle.yaml`) |

See `harness-evals/README.md`. Stage eval cases in `harness-evals/evals/` are populated and active when the OpenSpec workflow loads them; `/eval-loop` accumulates further cases from completed features.

## Namespaces & Image Resolution

| Namespace | Purpose |
|-----------|---------|
| `external-secrets-operator` | Operator deployment (OLM-managed) |
| `external-secrets` | Operand namespace (operator-created) |

| Env Var | Purpose |
|---------|---------|
| `RELATED_IMAGE_EXTERNAL_SECRETS` | Operand image (OLM disconnected convention) |
| `RELATED_IMAGE_BITWARDEN_SDK_SERVER` | Bitwarden image |

## Error Classification

| Type | Requeue | Example |
|------|---------|---------|
| `IrrecoverableError` | No | Missing RELATED_IMAGE_* env var |
| `RetryRequiredError` | 30s | Transient API server error |
| `UserConfigurationError` | Only NotFound | Invalid cert-manager issuer ref |

## Conditional Deployments

| Operand Component | Condition |
|-------------------|-----------|
| `external-secrets` (core) | Always |
| `external-secrets-webhook` | Always |
| `external-secrets-cert-controller` | cert-manager **disabled** |
| `bitwarden-sdk-server` | Bitwarden plugin **enabled** |

## Key References

- [Enhancement: ESO on OpenShift](https://github.com/openshift/enhancements/blob/master/enhancements/external-secrets-operator/external-secrets-operator.md)
- [Enhancement: Network Policies](https://github.com/openshift/enhancements/blob/master/enhancements/external-secrets-operator/external-secrets-network-policy.md)
- [Enhancement: Component Config](https://github.com/openshift/enhancements/blob/master/enhancements/external-secrets-operator/external-secrets-component-config.md)
- [Upstream external-secrets](https://github.com/external-secrets/external-secrets) | [OpenShift Docs](https://docs.openshift.com/)

---

**Platform Documentation**: openshift/enhancements/ai-docs/
