# `.cursor/rules/rule.md` — ESO E2E Test Generation Runbook

## What is this file?

`rule.md` is a **Cursor AI runbook** — a set of instructions that tells the AI assistant *exactly* how to behave whenever you ask it to generate, extend, or review e2e tests for the `external-secrets-operator` repository.

Think of it as a senior SDET's brain written down: it knows the repo layout, the right helpers to call, what already exists, and what still needs testing.

---

## What does each section do?

### Context & Persona
Sets the AI's role and tone before it does anything. Tells it to act as a Senior SDET specialising in Kubernetes Operators and OpenShift Security, and to be precise and safety-first. Also defines the **one-line flow** the AI follows every time:
> Fetch change → list existing specs → map diff to domains → dedup → write test-cases.md → only write Go if you ask for it.

---

### Section A — Hard Rules
The most important section. Defines **what the AI is allowed to touch**:

| Allowed | Forbidden |
|---|---|
| `test/**` | `pkg/`, `api/`, `cmd/`, `config/`, `bindata/` |
| `output/**` | `go.mod`, `Makefile`, `Dockerfile` |

Also enforces:
- Every new test file **must** have `//go:build e2e` at the top
- Never copy-paste wait logic — reuse `test/utils/` helpers
- Never add a duplicate `It` block — extend or skip instead
- Run a **pre-commit scope check** (`git diff --name-only | grep -v '^test/'`) before finishing — output must be empty

It also documents the exact names of every operand deployment, pod prefix, and container name so the AI never invents wrong resource names.

---

### Section B — Linear Workflow (Steps 1–7)
A **7-step process** the AI follows every time you give it a PR or Jira ticket:

| Step | What happens |
|---|---|
| 1. Source | Detects if input is a GitHub PR URL or Jira ticket |
| 2. Ingest | Fetches the diff with `gh pr diff` or Jira REST API |
| 3. Classify | Maps every changed file path to a test domain (e.g. `rbac`, `webhook`, `reconciliation`) |
| 4. Discover | Reads all existing `It` blocks so it knows what's already covered |
| 5. Dedup | Decides for each scenario: skip / extend existing / add new |
| 6. Top 10 | Picks up to 10 missing scenarios, ranked by priority |
| 7. Write plan | Produces `output/pr-<N>/test-cases.md` with full steps and expected outcomes |

Steps 8–9 (write Go code, open PR) only happen **if you explicitly ask**.

---

### Section C — Ginkgo / Gomega Patterns
Enforces consistent test structure across the codebase:
- How to structure `Describe → Context → It` with `By()` steps
- How to write async assertions with `Eventually`
- How to clean up resources with `DeferCleanup`
- How to safely update the `ExternalSecretsConfig` CR without conflicts (`retry.RetryOnConflict`)
- A reference table of every shared helper in `test/utils/` with its purpose

---

### Section D — Label Vocabulary
A fixed list of Ginkgo labels every `It` block must carry at least one of (e.g. `reconciliation`, `rbac`, `webhook`, `negative-input-validation`). Also defines the platform labels (`Platform:AWS`, `CrossPlatform:GCP-AWS`) that CI uses to filter which tests run on which cluster.

---

### Section E — Repo Layout and APIs
A precise map of every test file and utility file, plus the exact string values of all constants (namespaces, pod prefixes, API group names). Prevents the AI from hardcoding strings that already exist as constants in the repo.

---

### Section F — OLM / OpenShift / Red Hat Checklist
OpenShift-specific requirements the AI checks before declaring a test plan complete:
- OLM install path (Subscription → InstallPlan → CSV)
- Network policy validation (deny-all + per-component egress)
- SecurityContext requirements (`runAsNonRoot`, `capabilities.drop: ALL`, etc.)
- Proxy and trusted-CA-bundle handling
- Cert-manager integration annotation
- A certification checklist used to warn when coverage gaps exist

---

### Section G — Code Generation Phases
When you explicitly ask for Go code (Step 8), the AI follows 4 phases:
1. Re-run discovery to list every `It` it might touch
2. Dedup again to avoid parallel duplicates
3. Route shared logic to `test/utils/`, one-offs to the test file
4. Scope check — `git diff` must only show `test/` and `output/`

---

### Section H — Style
Final style guardrails: idiomatic Go, small functions, `By()` as chapter markers, comments explain *why* not *what*, random suffixes on resource names to prevent collision between parallel runs.

---

## How to use it

Just give the AI a GitHub PR URL or a Jira ticket number:

```
generate e2e for https://github.com/openshift/external-secrets-operator/pull/123
```

or

```
generate e2e for ESO-456
```

The AI will automatically follow the 7-step workflow in `rule.md`, produce a `output/pr-123/test-cases.md` plan, and ask before writing any Go code.

---

## File locations

```
.cursor/rules/
└── rule.md          ← this runbook (active for every AI session in this repo)

output/
└── pr-106/
    └── test-cases.md   ← example plan generated for PR #106

test/e2e/
└── override_env_test.go  ← example Go tests generated from PR #106 plan
```
