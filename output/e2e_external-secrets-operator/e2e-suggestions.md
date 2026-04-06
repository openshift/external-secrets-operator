# E2E Suggestions: external-secrets-operator (EP-1898)

## Detected Operator Structure
- **Framework**: controller-runtime (kubebuilder/operator-sdk)
- **Managed CRDs**: ExternalSecretsConfig, ExternalSecretsManager
- **E2E Pattern**: Ginkgo (package `e2e`, uses `kubernetes.Clientset`, `dynamic.DynamicClient`, `utils.DynamicResourceLoader`)
- **Operator Namespace**: `external-secrets-operator`
- **Operand Namespace**: `external-secrets`

## Changes Detected in Diff (`ai-staging-release-1.0...HEAD`)

### API Type Changes
1. New `annotations` field on `ControllerConfig` — custom annotations for operand Deployments
2. New `componentConfig` field on `ControllerConfig` — per-component configuration overrides
3. New `ComponentConfig` type with `deploymentConfigs` and `overrideEnv`
4. New `DeploymentConfig` type with `revisionHistoryLimit`
5. New `KVPair` and `Annotation` types
6. New `ComponentName` enum values: `Webhook`, `CertController`
7. CEL validation rules for reserved annotation prefixes and env var prefixes
8. CEL validation for `componentName` uniqueness

### Controller Changes
1. New `applyAnnotations()` function — propagates annotations to Deployment and Pod template
2. New `applyComponentConfig()` function — applies revisionHistoryLimit and overrideEnv
3. New `applyOverrideEnv()` function — merges env vars into containers
4. New `getComponentNameForAsset()` function — maps asset names to ComponentName
5. Updated `getDeploymentObject()` — calls new functions during deployment reconciliation
6. Updated `getPodSelectorForComponent()` — supports Webhook and CertController

## Highly Recommended E2E Scenarios

| Scenario | Priority | Test Code Reference |
|----------|----------|---------------------|
| Custom annotations applied to all Deployments (TC-01) | **Critical** | `e2e_test.go` — "Custom Annotations" Context |
| Reserved annotation prefix rejection (TC-02) | **Critical** | `e2e_test.go` — "Reserved Annotation Prefix Rejection" Context |
| revisionHistoryLimit applied per component (TC-03) | **Critical** | `e2e_test.go` — "ComponentConfig revisionHistoryLimit" Context |
| overrideEnv applied to correct container (TC-04) | **Critical** | `e2e_test.go` — "ComponentConfig overrideEnv" Context |
| Reserved env var prefix rejection (TC-05) | **Critical** | `e2e_test.go` — "Reserved Env Var Prefix Rejection" Context |
| Multiple components with different configs (TC-06) | **High** | `e2e_test.go` — "ComponentConfig revisionHistoryLimit" Context |
| Duplicate componentName rejection (TC-07) | **High** | `e2e_test.go` — "Duplicate ComponentName Rejection" Context |
| NetworkPolicy with Webhook pod selector (TC-08) | **High** | `e2e_test.go` — "Network Policy with New Component Names" Context |
| NetworkPolicy with CertController pod selector (TC-09) | **High** | `e2e_test.go` — "Network Policy with New Component Names" Context |
| Annotation update after initial creation (TC-10) | **Medium** | `e2e_test.go` — "Custom Annotations" Context |
| Combined config reconciliation (TC-11) | **Medium** | `e2e_test.go` — "Reconciliation with Combined Config" Context |

## Optional / Nice-to-Have Scenarios

| Scenario | Rationale |
|----------|-----------|
| Annotation removal and cleanup | Verify annotations are removed from Deployments when cleared from CR |
| componentConfig removal | Verify revisionHistoryLimit reverts to default when componentConfig is removed |
| overrideEnv with ValueFrom (fieldRef, secretKeyRef) | The `corev1.EnvVar` type supports these but tests only cover plain values |
| Reconciliation after operator pod restart | Verify the operator re-applies config after a restart |
| Concurrent updates to multiple fields | Test rapid successive updates don't cause race conditions |
| Edge case: 50 annotations (MaxItems limit) | Verify the max items boundary is enforced |
| Edge case: revisionHistoryLimit=1 (Minimum) | Test the minimum allowed value |
| BitwardenSDKServer componentConfig | Test componentConfig targeting BitwardenSDKServer (requires plugin enabled) |

## Gaps / Hard-to-Test Scenarios

1. **Annotation conflict resolution**: The logic says "user-specified annotations take precedence over defaults in case of conflicts." Testing this requires knowing what default annotations the operator sets, which is an internal implementation detail.
2. **overrideEnv merge semantics**: Testing that an existing env var is overridden (not duplicated) requires knowledge of the default env vars set by the operator's asset templates.
3. **Pod rollout verification**: After annotation or env var changes, the Deployment should trigger a rollout. Verifying the rollout completes with new pods requires waiting for pod recreation, which adds test flakiness risk.
4. **CRD validation boundary testing**: Testing exact error messages from CEL rules requires knowledge of the API server's error formatting, which can vary between Kubernetes versions.

## Recommendations

1. **Start with TC-01 through TC-07** — these cover the core functionality and validation
2. **Add TC-08 and TC-09** if your cluster has NetworkPolicy enforcement enabled
3. **TC-11 (combined config)** is valuable as an integration smoke test
4. **Copy the generated `e2e_test.go`** into `test/e2e/component_config_e2e_test.go` and adjust as needed
5. **The test uses `dynamic.Client`** for CRD operations and `kubernetes.Clientset` for built-in resources, matching the existing test pattern
