# Test Plan: OverrideEnv – per-component environment variable injection
<!-- Source: https://github.com/openshift/external-secrets-operator/pull/106 -->
<!-- Repo: openshift/external-secrets-operator -->
<!-- Framework: Ginkgo v2 / controller-runtime -->

## Summary

PR #106 introduces `OverrideEnv` in `ComponentConfig`, allowing users to inject or
override environment variables in individual operand component containers via
`ExternalSecretsConfig.Spec.ControllerConfig.ComponentConfigs[].OverrideEnv`.
User-defined values take precedence over operator-managed values. The diff adds
`applyUserDeploymentConfigs`, `mergeEnvVars`, and `getComponentNameFromAsset` to
`pkg/controller/external_secrets/deployments.go`, and new container name constants
to `constants.go`.

## Diff classification

| Path | Domains |
| --- | --- |
| `pkg/controller/external_secrets/constants.go` | `controller-manager` |
| `pkg/controller/external_secrets/deployments.go` | `reconciliation`, `controller-manager` |

## Coverage Map

| Scenario | Existing spec | Domain | Decision |
| --- | --- | --- | --- |
| Set env vars for all three components, verify in target container | `Context("Environment Variables") / It("should set custom…")` | `controller-manager` | `skip` – covered |
| Clear ComponentConfigs, verify vars gone | `Context("Environment Variables") / It("should remove custom…")` | `controller-manager` | `skip` – covered |
| Env vars for one component must not appear in sibling containers | *not covered* | `controller-manager`, `reconciliation` | `new` → TC-001 |
| Add a second env var; first must be preserved | *not covered* | `reconciliation` | `new` → TC-002 |
| Remove one of several env vars; remaining must stay | *not covered* | `reconciliation` | `new` → TC-003 |
| Change env var value; deployment must reflect new value | *not covered* | `reconciliation` | `new` → TC-004 |
| Reserved prefix (`KUBERNETES_`, `HOSTNAME`, `EXTERNAL_SECRETS_`) rejected at API | *not covered* | `negative-input-validation` | `new` → TC-005 |
| Updating one component must not trigger rollout in other components | *not covered* | `controller-manager` | `new` → TC-006 |

## Test Cases

### PR-106-TC-001: Container isolation – OverrideEnv applies only to the targeted component

**Priority:** Critical
**Domain:** `reconciliation`, `controller-manager`
**Category:** Core reconciler behaviour / per-container targeting
**OpenShift-specific:** no
**Coverage Gap:** `mergeEnvVars` + `getComponentNameFromAsset` are supposed to target only
the primary container of the named component; no test today verifies that env vars
DON'T appear in sibling deployments.

**Prerequisites:** Operator running in `external-secrets-operator` ns; `ExternalSecretsConfig/cluster` exists and is Ready.

**Steps:**
1. Add a unique env var (e.g. `ESO_ISOLATION_TEST_<rand>`) to `ComponentConfigs` for
   `ExternalSecretsCoreController` only.
   **Expected:** `runtimeClient.Update` succeeds.
2. Wait for `external-secrets` pod prefix to be Ready.
   **Expected:** Pod reaches Running + ContainersReady.
3. Get deployment `external-secrets`; find container `external-secrets`.
   **Expected:** Env var IS present with the correct value.
4. Get deployment `external-secrets-webhook`; find container `webhook`.
   **Expected:** Env var is NOT present.
5. Get deployment `external-secrets-cert-controller`; find container `cert-controller`.
   **Expected:** Env var is NOT present.

**Stop condition:** Env var leaking into a sibling container means `getComponentNameFromAsset` / `applyUserDeploymentConfigs` is broken and every OverrideEnv set is potentially polluting other components.

---

### PR-106-TC-002: Incremental add – existing env vars preserved when a new one is added

**Priority:** High
**Domain:** `reconciliation`, `controller-manager`
**Category:** Env var lifecycle
**OpenShift-specific:** no
**Coverage Gap:** No test today verifies that a second `OverrideEnv` entry does not evict the first one.

**Prerequisites:** Same as TC-001.

**Steps:**
1. Set one env var (`ESO_INC_A_<rand>`) for `ExternalSecretsCoreController`.
   **Expected:** Update succeeds; value appears in `external-secrets` container.
2. Update the same `ComponentConfig` to add a second env var (`ESO_INC_B_<rand>`) while
   keeping the first.
   **Expected:** Update succeeds.
3. Wait for pod readiness. Get deployment; inspect container `external-secrets`.
   **Expected:** Both `ESO_INC_A_*` and `ESO_INC_B_*` are present with correct values.

**Stop condition:** Loss of an earlier env var on incremental updates means the merge logic in `mergeEnvVars` has a regression.

---

### PR-106-TC-003: Partial remove – deleting one env var leaves the others intact

**Priority:** High
**Domain:** `reconciliation`, `controller-manager`
**Category:** Env var lifecycle
**OpenShift-specific:** no
**Coverage Gap:** No test today verifies partial removal.

**Prerequisites:** Same as TC-001.

**Steps:**
1. Set two env vars (`ESO_REM_X_<rand>`, `ESO_REM_Y_<rand>`) for `ExternalSecretsCoreController`.
   **Expected:** Both appear in `external-secrets` container.
2. Remove `ESO_REM_X_*` from `OverrideEnv`, keeping `ESO_REM_Y_*`.
   **Expected:** Update succeeds; deployment rolls out.
3. Inspect container `external-secrets`.
   **Expected:** `ESO_REM_Y_*` is present; `ESO_REM_X_*` is absent.

**Stop condition:** Stale env vars that can't be removed are a correctness bug in the reconciler.

---

### PR-106-TC-004: Value update – changing an OverrideEnv value propagates to the deployment

**Priority:** High
**Domain:** `reconciliation`, `controller-manager`
**Category:** Env var lifecycle / update propagation
**OpenShift-specific:** no
**Coverage Gap:** No test today changes an existing env var's value.

**Prerequisites:** Same as TC-001.

**Steps:**
1. Set `ESO_UPDATE_TEST_<rand>=initial-value` for `Webhook`.
   **Expected:** Value appears in `webhook` container.
2. Change the value to `updated-value`.
   **Expected:** Update accepted; deployment rolls out.
3. Inspect `webhook` container.
   **Expected:** Env var has `updated-value`; `initial-value` is no longer present.

**Stop condition:** Stale values after update mean the override merge is not idempotent on value changes.

---

### PR-106-TC-005: Reserved prefix rejection – KUBERNETES_, HOSTNAME, EXTERNAL_SECRETS_ blocked at API level

**Priority:** High
**Domain:** `negative-input-validation`
**Category:** API validation / CRD CEL rules
**OpenShift-specific:** no
**Coverage Gap:** The CRD carries a CEL validation rule rejecting reserved prefixes; no e2e verifies this gate.

**Prerequisites:** CRD installed with up-to-date CEL rules.

**Steps:**
1. Attempt to set `KUBERNETES_SERVICE_HOST=10.0.0.1` as an OverrideEnv for `ExternalSecretsCoreController`.
   **Expected:** API returns HTTP 422 (Invalid); error message contains "reserved".
2. Attempt to set `HOSTNAME=test-host`.
   **Expected:** API returns HTTP 422; error message contains "reserved".
3. Attempt to set `EXTERNAL_SECRETS_CUSTOM=val`.
   **Expected:** API returns HTTP 422; error message contains "reserved".

**Stop condition:** If reserved env vars can be injected, operators may accidentally override internal Kubernetes / operator env vars, causing cluster-level or operator stability issues.

---

### PR-106-TC-006: Single component scope – updating one component does not roll out unrelated components

**Priority:** Medium
**Domain:** `controller-manager`, `reconciliation`
**Category:** Reconciler scope / deployment stability
**OpenShift-specific:** no
**Coverage Gap:** No test verifies that the reconciler does not needlessly update sibling deployments when only one `ComponentConfig` entry changes.

**Prerequisites:** All operand pods Ready.

**Steps:**
1. Record the current `Generation` of `external-secrets-webhook` and `external-secrets-cert-controller` deployments.
2. Add an env var only for `ExternalSecretsCoreController`.
   **Expected:** Update succeeds; `external-secrets` deployment rolls out.
3. Wait for `external-secrets` pods to be Ready.
4. Re-read `external-secrets-webhook` and `external-secrets-cert-controller` deployments.
   **Expected:** Their `Generation` is unchanged (no spec diff, no rollout triggered).

**Stop condition:** Spurious rollouts for unaffected components increase churn and latency during operator reconciliation.

---

## OLM / OpenShift / Red Hat

- **OLM:** Not in scope for this PR / test plan (install path unchanged).
- **OpenShift:** All tests run on a real OCP cluster. No SCC or privileged behaviour changed by this PR.
- **Certification checklist:**
  - [x] Negative input validation covered (TC-005)
  - [ ] OLM install – not in scope
  - [ ] Network policy – not touched by this PR
  - [ ] RBAC – not touched by this PR
  - [ ] SecurityContext – not touched by this PR
