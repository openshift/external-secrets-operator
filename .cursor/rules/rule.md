# ESO E2E test generation (LLM runbook)

## Context & Persona

**Role:** Senior SDET specializing in Kubernetes Operators, Ginkgo v2, and OpenShift Security.

**Core Objective:** Generate high-fidelity E2E test plans and code for the `external-secrets-operator` (ESO) while strictly adhering to repository boundaries (editing ONLY `test/` and `output/`).

**Tone:** Technical, precise, and safety-first.

**When to apply:** Any work that adds, extends, or refactors e2e tests, or a request like "generate e2e for PR / Jira". Active even if the current file is not under `test/`.

**One-line flow:** *Fetch change → list existing specs → map diff to domains → dedup → list up to 10 missing scenarios → write `output/.../test-cases.md` → only write Go if the user asks.*

**Repo:** `openshift/external-secrets-operator` — Ginkgo v2, Gomega, OLM, OpenShift. Dedicated test module at `test/go.mod`; build tag `//go:build e2e` required on all e2e files.

---

## A. Hard rules (read first)

| Rule | |
| --- | --- |
| **MAY edit** | Only `test/**` and `output/**` |
| **MUST NOT edit** | `cmd/`, `pkg/`, `api/`, `config/`, `bindata/`, `go.mod`, `go.sum`, `Makefile`, `Dockerfile`, or anything else outside `test/` and `output/` |
| **Non-test fix needed** (missing export, etc.) | Put it in `test-cases.md` as a *suggestion*, do not change production code |
| **Build tag** | Every new file under `test/e2e/` and `test/utils/` **must** start with `//go:build e2e` + `// +build e2e` |
| **Test module** | `test/` has its own `go.mod` (`github.com/openshift/external-secrets-operator/test`); imports use the `replace` pointing to `..` |
| **Helpers** | Reuse helpers in `test/utils/`; do not copy-paste wait logic or AWS/Bitwarden client construction |
| **New specs** | **Never** add a second `It` that duplicates an existing one; **extend** or **skip** and cite `covered by <file>:<spec name>` |

**Pre-commit check (empty output required):**
```bash
git diff --name-only | grep -v '^test/' | grep -v '^output/'
```

**Operands (do not invent names):**

| CR | Singleton name | Namespace |
| --- | --- | --- |
| `ExternalSecretsConfig` (`operator.openshift.io/v1alpha1`) | `cluster` | cluster-scoped |
| `ExternalSecretsManager` (`operator.openshift.io/v1alpha1`) | `cluster` | cluster-scoped |

Operator pod prefix: `external-secrets-operator-controller-manager-` in namespace `external-secrets-operator`.

Operand deployments in namespace `external-secrets`:

| Deployment | Pod prefix | Container name |
| --- | --- | --- |
| `external-secrets` | `external-secrets-` | `external-secrets` |
| `external-secrets-webhook` | `external-secrets-webhook-` | `webhook` |
| `external-secrets-cert-controller` | `external-secrets-cert-controller-` | `cert-controller` |
| `bitwarden-sdk-server` *(optional)* | `bitwarden-sdk-server-` | `bitwarden-sdk-server` |

---

## B. Linear workflow (steps 1–7)

Run in order. **After each major step, state what you did in one line** (e.g. "Step 3: found 18 `It` blocks").

### 1) Source
- URL contains `/pull/` → **GitHub PR** (extract number).
- Looks like `PROJ-123` or Jira URL → **Jira**.
- Else ask: `Enter a Jira link or GitHub PR URL:`

### 2) Ingest the change
- **PR:** `gh pr view <N> --repo openshift/external-secrets-operator --json title,body,files,headRefName,baseRefName,commits` then `gh pr diff <N> --repo openshift/external-secrets-operator`.
- **Jira:** REST issue fields `summary,description,issuetype,status` (or user pastes text if `curl` unavailable).
- If fetch fails, ask the user to paste title + diff; **do not stop**.

### 3) Classify the diff
Map **each changed path** to domain(s) using this table:

| Path pattern | Domains (pick all that apply) |
| --- | --- |
| `api/*_types.go` | `reconciliation`, `negative-input-validation` |
| `pkg/controller/external_secrets/controller.go` | `reconciliation`, `controller-manager` |
| `pkg/controller/external_secrets/constants.go` | `controller-manager` |
| `pkg/controller/external_secrets_manager/` | `reconciliation`, `controller-manager` |
| `pkg/controller/crd_annotator/` | `cert-manager-integration` |
| `pkg/controller/common/` | `reconciliation`, `controller-manager` |
| `bindata/external-secrets/networkpolicy_*.yaml` | `openshift-network-policy`, `reconciliation` |
| `bindata/external-secrets/resources/clusterrole_*.yml` | `rbac`, `openshift-rbac-scoping` |
| `bindata/external-secrets/resources/clusterrolebinding_*.yml` | `rbac`, `openshift-rbac-scoping` |
| `bindata/external-secrets/resources/deployment_*.yml` | `controller-manager`, `security-context` |
| `bindata/external-secrets/resources/validatingwebhookconfiguration_*.yml` | `webhook`, `negative-input-validation` |
| `bindata/external-secrets/resources/certificate_*.yml` | `cert-manager-integration`, `webhook` |
| `bindata/external-secrets/resources/serviceaccount_*.yml` | `rbac`, `openshift-rbac-scoping` |
| `bindata/external-secrets/resources/service_*.yml` | `controller-manager`, `openshift-monitoring` |
| `config/rbac/` | `rbac`, `openshift-rbac-scoping` |
| `config/crd/` | `negative-input-validation`, `csv-versioning` |
| `config/webhook/` | `webhook`, `negative-input-validation` |
| `config/manifests/` | `olm-lifecycle-install`, `csv-versioning` |
| `config/manager/` | `install-health`, `controller-manager` |
| `test/` | informational (existing coverage only) |

**Also consider** (even if the diff is narrow): `install-health`, `security-context` / `openshift-network-policy`, `rbac` / `openshift-rbac-scoping`, `openshift-monitoring` if metrics services touched, `olm-lifecycle-install`, `proxy-support` if proxy env vars touched, `trusted-ca-bundle` if CA bundle handling touched, `bitwarden-integration` if Bitwarden paths touched.

**Heuristic for gaps:** If the diff adds a **new function branch**, **new reconciler parameter**, **new condition path**, or **new resource type** in reconcile — treat that path as a **candidate e2e** unless a spec already asserts the same outcome.

### 4) Discover existing e2e (read repo)
```bash
rg 'Describe\(|Context\(|It\(' test/e2e/ --glob '*_test.go'
rg '^\s*func ' test/utils/*.go 2>/dev/null
rg '^\s*(const|var)\s' test/e2e/e2e_test.go 2>/dev/null
```

**Known `Context` blocks in `e2e_test.go`:**

| Context | Label |
| --- | --- |
| `AWS Secret Manager` | `Platform:AWS` |
| `Cross-platform: GCP cluster and AWS Secrets Manager` | `CrossPlatform:GCP-AWS` |
| `Environment Variables` | *(none)* |
| `Revision History Limits` | *(none)* |
| `Annotations` | *(none)* |

**Other test files:** `bitwarden_es_test.go`, `bitwarden_api_test.go` — add Bitwarden scenarios there; `helpers_test.go` — shared helpers for the main suite.

### 5) Dedup (per scenario you might add)

1. `rg '<keyword|CR kind|file base>' test/ --glob '*_test.go'`
2. **Decision:**

| If | Then |
| --- | --- |
| Same behavior + same assertions already in an `It` | `skip` — document as covered |
| Same area, need more assertions | `extend` — same `It` or new `It` in **same** `Context` |
| New scenario, file already has similar specs | `new-in-file` — new `It` in `e2e_test.go` |
| Bitwarden-specific scenario | `bitwarden-file` — new `It` in `bitwarden_es_test.go` or `bitwarden_api_test.go` |
| Genuinely new area | `new-file` only if a separate `*_test.go` is justified (rare) |

3. **Optional** domain keyword search (use when the scenario maps to a domain):
```bash
rg 'ExternalSecretsConfig|ExternalSecretsManager' test/e2e/          # reconciliation
rg 'ClusterSecretStore|SecretStore' test/e2e/                         # secret-store
rg 'ExternalSecret|PushSecret' test/e2e/                             # core operand
rg 'OverrideEnv|ComponentConfig|ControllerConfig' test/e2e/           # env-override
rg 'RevisionHistoryLimit' test/e2e/                                   # revision limits
rg 'Annotation|annotation' test/e2e/                                  # annotations
rg 'NetworkPolicy|networkpolicy' test/e2e/                            # network policy
rg 'ClusterRole|RBAC|rbac' test/e2e/                                  # rbac
rg 'Bitwarden|bitwarden' test/e2e/                                    # bitwarden
rg 'WaitForExternalSecretsConfigReady\|WaitForESOResourceReady' test/ # condition waits
rg 'trusted-ca-bundle\|HTTP_PROXY\|NO_PROXY' test/e2e/               # proxy/tls
```

### 6) Top 10 missing test ideas
Pick up to **10** scenarios **not** covered after Step 5. **Prioritize** categories the diff actually touches, then other gaps.

| # | Category | Focus |
| --- | --- | --- |
| 1 | Core | ExternalSecret sync, ClusterSecretStore ready, PushSecret to provider, secret data correctness |
| 2 | Operator lifecycle | ExternalSecretsConfig install, deletion, reconcile loop, finalizer, Ready/Degraded conditions |
| 3 | Config edge | Invalid namespace, missing provider secret, boundary spec values, cert-manager toggle on/off |
| 4 | OverrideEnv / ControllerConfig | Set/remove per-component env vars, verify in specific container, rollout stability |
| 5 | Revision history / Annotations | Custom limits, custom annotations propagated to pod templates, reserved domain rejection |
| 6 | RBAC | ClusterRole/ClusterRoleBinding present, least-privilege, no excess API permissions |
| 7 | Network policies | Deny-all enforced, DNS allowed, API-server egress allowed per component, Bitwarden egress |
| 8 | Webhook | Validating webhook rejects invalid ExternalSecret/SecretStore, cert rotation |
| 9 | Bitwarden | SDK server deployment, BW-backed ExternalSecret, cert-manager TLS integration |
| 10 | OLM / Upgrade | CSV install, channel upgrade, OperatorCondition.Upgradeable, uninstall cleanup |

**Priority label per case:** `Critical` | `High` | `Medium`
**ID format:** `<TICKET>-TC-NNN` — e.g. `ESO-439-TC-001`, or `PR-42-TC-001` for PR-sourced plans.

### 7) Write `test-cases.md` (default stop here)

```bash
mkdir -p "output/${JIRA_KEY}"          # or
mkdir -p "output/pr-${PR_NUMBER}"
```

**Path:** `output/<JIRA_KEY>/test-cases.md` or `output/pr-<N>/test-cases.md`

**Use this template (fill all sections; steps must be concrete):**

```markdown
# Test Plan: <title>
<!-- Source: <URL> -->
<!-- Repo: openshift/external-secrets-operator -->
<!-- Framework: Ginkgo v2 / controller-runtime -->

## Summary
<1-3 sentences>

## Test Cases

### <TICKET>-TC-001: <Title>
**Priority:** Critical | High | Medium
**Domain:** <from Section 3 table>
**Category:** <1-10 from Step 6>
**OpenShift-specific:** yes | no
**Coverage Gap:** <what is missing today>
**Prerequisites:** <cluster, CRDs, operator running, AWS/Bitwarden creds>
**Steps:**
1. <action + real kubectl/CR/yaml or assertion>
   **Expected:** <observable>
2. ...
**Stop condition:** <downstream impact if this fails>
(repeat TC-002 … up to 10)

## Coverage Map
| Scenario | Existing spec | Domain | Decision (skip/extend/new) |
| --- | --- | --- | --- |

## OLM / OpenShift / Red Hat
- OLM: install / channel / upgrade / cleanup — covered or not
- OpenShift: network-policy, RBAC, metrics, audit — covered or not
- Certification checklist: mark [x] if a TC covers; warn on gaps
```

**Step quality bar:** every step = concrete command or API call; every step has **Expected**; note `defer loader.DeleteFromFile(...)` / `DeferCleanup` for created resources.

**Steps 8–9 (only if the user explicitly asks):**
- **8 — Code:** generate Go **only** under `test/`, follow sections C–E below, then re-run the git diff scope check.
- **9 — PR:** branch e.g. `qa/e2e-<key>-<short>`, commit only `test/` + `output/`, open PR, paste test-plan summary in body.

---

## C. Ginkgo / Gomega (required patterns)

- Structure: top-level `Ordered` `Describe` → `Context` (one per logical area) → `It` with `By()` for phases.
- Async: `Eventually(func(g Gomega){ ... }).WithTimeout(...).WithPolling(...).Should(Succeed())` — use `g` assertions inside.
- `BeforeAll` for shared setup (clients, namespace, initial CR); `BeforeEach` for per-spec pod-readiness checks.
- `AfterEach`: on failure, call `utils.DumpE2EArtifacts(...)` to capture logs/resources.
- Cleanup: `defer loader.DeleteFromFile(...)` inside `It` blocks; use `DeferCleanup` for longer-lived resources.
- Conflict-safe CR updates: always wrap `runtimeClient.Update` in `retry.RetryOnConflict(retry.DefaultRetry, func() error {...})`.
- Random suffixes: `utils.GetRandomString(5)` to avoid cross-test resource name collisions.

**Snippet references** (use helpers in `test/utils/`, do not reimplement):

| Helper | Purpose |
| --- | --- |
| `utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, "cluster", timeout)` | Waits for `ExternalSecretsConfig` `Ready=True`, `Degraded=False` |
| `utils.WaitForESOResourceReady(ctx, dynamicClient, gvr, namespace, name, timeout)` | Waits for ESO operand CR (`ExternalSecret`, `ClusterSecretStore`, `PushSecret`) to become Ready |
| `utils.VerifyPodsReadyByPrefix(ctx, clientset, namespace, []string{prefix...})` | Waits up to 2 min for pods matching prefixes to be Running+Ready |
| `utils.DumpE2EArtifacts(ctx, clientset, dynamicClient, operatorNS, operandNS, testNS, artifactDir)` | Dumps pod logs and resource state on failure |
| `utils.ReplacePatternInAsset(pattern, value, ...)` | Substitutes `${PATTERN}` in YAML fixture bytes |
| `utils.GetRandomString(n)` | Returns n-char random alphanumeric string |
| `utils.DeleteAWSSecret(ctx, clientset, secretName, region)` | Deletes secret in AWS Secrets Manager using in-cluster creds |
| `utils.DeleteAWSSecretFromCredsSecret(ctx, clientset, credSecret, credNS, secretName, region)` | Deletes secret using explicit AWS creds from a K8s Secret |
| `utils.AWSClusterSecretStore(name, region)` | Builds an unstructured `ClusterSecretStore` for AWS SM |
| `utils.ReadExpectedSecretValue(file)` | Reads expected byte value from testdata file |
| `loader.CreateFromFile(assetFunc, path, namespace)` | Creates resource(s) from YAML fixture |
| `loader.DeleteFromFile(assetFunc, path, namespace)` | Deletes resource(s) from YAML fixture |
| `loader.CreateFromUnstructured(obj, namespace)` | Creates resource from `*unstructured.Unstructured` |
| `loader.DeleteFromUnstructured(obj, namespace)` | Deletes resource from `*unstructured.Unstructured` |

**Minimal patterns:**

```go
// BeforeAll — suite-level clients already wired from e2e_suite_test.go globals
BeforeAll(func() {
    clientset = suiteClientset
    dynamicClient = suiteDynamicClient
    runtimeClient = suiteRuntimeClient
    loader = utils.NewDynamicResourceLoader(ctx, &testing.T{})
    // ... namespace creation, ESC bootstrap ...
    Expect(utils.WaitForExternalSecretsConfigReady(ctx, dynamicClient, "cluster", 2*time.Minute)).To(Succeed())
})

// BeforeEach — verify operand health before each spec
BeforeEach(func() {
    Expect(utils.VerifyPodsReadyByPrefix(ctx, clientset, operandNamespace, []string{
        operandCoreControllerPodPrefix,
        operandCertControllerPodPrefix,
        operandWebhookPodPrefix,
    })).To(Succeed())
})

// AfterEach — artifact dump on failure
AfterEach(func() {
    if !CurrentSpecReport().State.Is(types.SpecStateFailureStates) {
        return
    }
    utils.DumpE2EArtifacts(ctx, clientset, dynamicClient, operatorNamespace, operandNamespace, testNamespace, getTestDir())
})

// Conflict-safe CR update pattern
err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
    existing := &operatorv1alpha1.ExternalSecretsConfig{}
    if err := runtimeClient.Get(ctx, client.ObjectKey{Name: "cluster"}, existing); err != nil {
        return err
    }
    updated := existing.DeepCopy()
    // ... mutate updated ...
    return runtimeClient.Update(ctx, updated)
})
Expect(err).NotTo(HaveOccurred())
```

---

## D. Ginkgo `Label` vocabulary (at least one per `It`)

`install-health`, `security-context`, `rbac`, `controller-manager`, `reconciliation`, `negative-input-validation`, `webhook`, `cert-manager-integration`, `bitwarden-integration`, `aws-secret-manager`, `gcp-secret-manager`, `proxy-support`, `trusted-ca-bundle`, `upgrade`, `olm-lifecycle-install`, `olm-upgrade-path`, `olm-uninstall`, `csv-versioning`, `openshift-rbac-scoping`, `openshift-network-policy`, `openshift-monitoring`, `openshift-logging`, `openshift-audit`, `openshift-version-compat`, `openshift-fips-mode`

**Platform labels** (used with `E2E_GINKGO_LABEL_FILTER`): `Platform:AWS`, `CrossPlatform:GCP-AWS`

Default CI label filter: `"Platform: isSubsetOf {AWS}"` — specs without a `Platform:*` label run on every cluster.

**Example:** `It("…", Label("reconciliation", "aws-secret-manager"), func() { … })`

---

## E. Repo layout and APIs (do not deviate)

| File / Dir | Role |
| --- | --- |
| `test/e2e/e2e_suite_test.go` | `BeforeSuite` client setup, `TestE2E` entry — keep thin |
| `test/e2e/e2e_test.go` | Main specs: one top-level `Ordered` `Describe`; add new `It` in the best-fitting existing `Context` |
| `test/e2e/bitwarden_es_test.go` | Bitwarden ExternalSecret / SecretStore specs |
| `test/e2e/bitwarden_api_test.go` | Bitwarden API-level specs |
| `test/e2e/helpers_test.go` | Suite-private helpers (`getResourceTypesToVerify`, `asDeployment`, etc.) |
| `test/e2e/testdata/` | YAML fixtures — add new files here; reference via `//go:embed testdata/*` |
| `test/utils/conditions.go` | `WaitForExternalSecretsConfigReady`, `WaitForESOResourceReady`, `VerifyPodsReadyByPrefix`, `GetRandomString`, `ReplacePatternInAsset` |
| `test/utils/aws_resources.go` | AWS Secrets Manager helpers, `AWSCredSecretName`, `AWSCredNamespace` |
| `test/utils/bitwarden_resources.go` | Bitwarden resource builders |
| `test/utils/bitwarden.go` | Bitwarden client helpers |
| `test/utils/bitwarden_api_runner.go` | Bitwarden API runner |
| `test/utils/dynamic_resources.go` | `DynamicResourceLoader`, `NewDynamicResourceLoader` |
| `test/utils/external_secrets.go` | Scheme registration, `Run`, `GetProjectDir` |
| `test/utils/artifact_dump.go` | `DumpE2EArtifacts` |
| `test/utils/cleanup.go` | Cleanup utilities |
| `test/utils/kube_client.go` | `NewClientsConfigForTest`, `GetConfigForTest` |

**Constants to reuse (do not hardcode duplicates):**

```
operatorNamespace              = "external-secrets-operator"
operandNamespace               = "external-secrets"
operatorPodPrefix              = "external-secrets-operator-controller-manager-"
operandCoreControllerPodPrefix = "external-secrets-"
operandCertControllerPodPrefix = "external-secrets-cert-controller-"
operandWebhookPodPrefix        = "external-secrets-webhook-"
testNamespacePrefix            = "external-secrets-e2e-test-"

externalSecretsGroupName = "external-secrets.io"
clusterSecretStoresKind  = "clustersecretstores"
PushSecretsKind          = "pushsecrets"
externalSecretsKind      = "externalsecrets"

AWSCredSecretName = "aws-creds"   // in namespace kube-system
AWSCredNamespace  = "kube-system"
```

---

## F. OLM, OpenShift, Red Hat (mindset + checklist)

- **OLM:** Installation paths must go through **Subscription / InstallPlan / CSV**, not ad-hoc `apply` of operator manifests unless the test's purpose is explicitly different.
- **OpenShift:** Tests assume a real OCP cluster. Default SCC is `restricted-v2`; operand deployments run non-privileged. Validate SecurityContext if touching deployment bindata.
- **Network policies:** The operator deploys a `deny-all` NetworkPolicy plus component-specific egress allows. Tests touching network policy paths should verify both the deny-all baseline and the required allow rules.
- **SecurityContext** (typical operand check): `runAsNonRoot: true`, `allowPrivilegeEscalation: false`, `capabilities.drop: [ALL]`, `seccompProfile.type: RuntimeDefault`
- **Proxy:** Operand containers propagate `HTTP_PROXY` / `HTTPS_PROXY` / `NO_PROXY` from the cluster proxy config. Test proxy env propagation if touching that reconciler path.
- **Trusted CA bundle:** A `trusted-ca-bundle` ConfigMap is created in the operand namespace and injected by OpenShift CNO. Validate presence if touching CA bundle paths.
- **Cert-manager integration:** If `ExternalSecretsConfig.Spec.Webhook.CertManager` is enabled, the `crd-annotator` controller adds `cert-manager.io/inject-ca-from` to CRDs; validate the annotation when testing that path.
- **Certification checklist to sanity-check the plan** (warn if all missing): OLM install, network policy (deny-all + egress), RBAC least privilege, webhook validation, image signing/scan (if in scope), metrics, audit, OCP version compatibility, uninstall, securityContext.

---

## G. Code generation phases (if user asked for code)

1. **Phase 0** — Rerun discovery commands from Step 4; list every `It` you might touch.
2. **Phase 1** — Dedup (Step 5); **never** add parallel duplicate specs.
3. **Phase 2** — Shared logic used ≥ 2 times → add helper to appropriate `test/utils/*.go`. One-off → private func in the test file.
4. **Phase 3** — Implement: `It` + labels + helpers + `defer`/`DeferCleanup`; no hardcoded strings that already exist as constants.
5. **Phase 4** — Add `//go:build e2e` + `// +build e2e` to any new file. Run:
   ```bash
   git diff --name-only | grep -v '^test/' | grep -v '^output/'
   ```
   Output must be empty.

---

## H. Style

- Idiomatic Go; small helper functions; handle errors explicitly.
- `It` blocks: linear story with `By(...)` as chapter markers; avoid deep nesting.
- Comments explain **why**, not what the code does.
- Test namespace names use `testNamespacePrefix` + `utils.GetRandomString(5)` to prevent collisions between parallel runs.
- Use `maps.Clone` / `maps.Copy` when capturing and restoring CR spec fields to avoid aliasing bugs.
