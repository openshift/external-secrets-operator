# E2E Test Suggestions: external-secrets-operator

## Operator Structure

| Property | Value |
|---|---|
| **Framework** | controller-runtime |
| **API Group** | operator.openshift.io/v1alpha1 |
| **Managed CRDs** | ExternalSecretsConfig, ExternalSecretsManager |
| **Operator Namespace** | external-secrets-operator |
| **Operand Namespace** | external-secrets |
| **E2E Pattern** | Ginkgo v2 with dynamic client, Ordered specs |

## Changes Detected in Diff (release-1.0...HEAD)

| File | Category | Change Summary |
|---|---|---|
| `api/v1alpha1/external_secrets_config_types.go` | API Types | Added `Annotations`, `ComponentConfigs` to `ControllerConfig`; new types `ComponentConfig`, `DeploymentConfig`, `KVPair`, `Annotation`; new `ComponentName` constants `Webhook`, `CertController` |
| `api/v1alpha1/zz_generated.deepcopy.go` | Generated | DeepCopy functions for new types |
| `config/crd/bases/...externalsecretsconfigs...yaml` | CRD | Updated CRD schema with new fields, CEL validation rules |
| `pkg/controller/external_secrets/component_config.go` | Controller | New reconciliation logic: `applyGlobalAnnotations`, `applyComponentDeploymentConfig`, `applyComponentOverrideEnv`, `applyComponentConfig` |
| `pkg/controller/external_secrets/component_config_test.go` | Unit Tests | Unit tests for all new controller functions |
| `pkg/controller/external_secrets/deployments.go` | Controller | Integration of `applyComponentConfig` into `getDeploymentObject` |
| `api/v1alpha1/tests/.../externalsecretsconfig.testsuite.yaml` | Integration Tests | YAML test suite for API validation |

## Highly Recommended E2E Scenarios

### 1. Global Annotations — Apply to All Components ⭐
**Priority**: High
**Why**: Core new feature. Validates that annotations flow from CR → all Deployment metadata and Pod templates.
**Test**: TC-01 in test-cases.md

### 2. Reserved Annotation Prefix Rejection ⭐
**Priority**: High
**Why**: CEL validation rule. Validates API-level admission rejection for `kubernetes.io/`, `app.kubernetes.io/`, `openshift.io/`, `k8s.io/` prefixes.
**Test**: TC-02 in test-cases.md

### 3. RevisionHistoryLimit — Controller Component ⭐
**Priority**: High
**Why**: Core new feature. Validates per-component deployment configuration override.
**Test**: TC-04 in test-cases.md

### 4. OverrideEnv — Controller Component ⭐
**Priority**: High
**Why**: Core new feature. Validates custom env vars are applied to specific containers.
**Test**: TC-06 in test-cases.md

### 5. Reserved Env Var Prefix Rejection ⭐
**Priority**: High
**Why**: CEL validation rule. Validates API rejects `HOSTNAME`, `KUBERNETES_*`, `EXTERNAL_SECRETS_*` env var overrides.
**Test**: TC-07 in test-cases.md

### 6. Invalid ComponentName Rejection ⭐
**Priority**: High
**Why**: Enum validation. Validates API rejects unknown component names.
**Test**: TC-09 in test-cases.md

### 7. Duplicate ComponentName Rejection ⭐
**Priority**: High
**Why**: CEL uniqueness validation. Validates API rejects duplicate componentName entries.
**Test**: TC-10 in test-cases.md

### 8. Combined Annotations + Component Configs ⭐
**Priority**: High
**Why**: Integration test. Validates all new features work together without interference.
**Test**: TC-11 in test-cases.md

## Optional / Nice-to-Have Scenarios

### 9. Annotation Update and Removal
**Priority**: Medium
**Why**: Lifecycle test. Validates annotations can be added, updated, and removed cleanly.
**Test**: TC-03 in test-cases.md

### 10. RevisionHistoryLimit for Webhook
**Priority**: Medium
**Why**: Validates the same feature works across different components.
**Test**: TC-05 in test-cases.md

### 11. Multiple Components Configured Simultaneously
**Priority**: Medium
**Why**: Validates independent component configuration.
**Test**: TC-08 in test-cases.md

### 12. Reconciliation Recovery — Config Drift
**Priority**: Medium
**Why**: Validates operator restores desired state after manual changes. Important for production resilience but slower to test.
**Test**: TC-12 in test-cases.md

## Gaps / Hard-to-Test Scenarios

| Scenario | Difficulty | Reason |
|---|---|---|
| BitwardenSDKServer component config | High | Requires Bitwarden plugin to be enabled and configured, needs additional infrastructure |
| Annotation removal side effects | Medium | Verifying that previously-set annotations are fully cleaned up requires tracking across reconciliation cycles |
| Race conditions during rapid config updates | High | Requires concurrent config patches which are hard to reproduce deterministically |
| Controller restart resilience | Medium | Requires killing the operator pod and verifying state after recovery |

## Recommendations

1. **Start with the 8 highly recommended scenarios** — they cover all new API fields and validation rules.
2. **Add reconciliation recovery (TC-12) if time permits** — it validates operator robustness.
3. **Skip BitwardenSDKServer tests** unless the Bitwarden plugin infrastructure is available in the CI environment.
4. **Use the generated `e2e_test.go`** as a starting point — copy it into `test/e2e/` and adjust as needed.
5. **Label tests** with `Label("ComponentConfig")` so they can be run independently from existing AWS-specific tests.
