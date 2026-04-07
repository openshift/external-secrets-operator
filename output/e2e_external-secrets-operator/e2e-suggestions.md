# E2E Test Suggestions: external-secrets-operator (EP-1898)

## Detected Operator Structure
- **Framework**: controller-runtime (kubebuilder/operator-sdk)
- **Managed CRDs**: ExternalSecretsConfig (cluster-scoped singleton), ExternalSecretsManager
- **E2E Pattern**: Ginkgo v2, package `e2e`, uses `kubernetes.Clientset` and `dynamic.DynamicClient`
- **Operator Namespace**: `external-secrets-operator`
- **Operand Namespace**: `external-secrets`
- **Existing helpers**: `utils.VerifyPodsReadyByPrefix`, `utils.WaitForESOResourceReady`, `utils.DynamicResourceLoader`

## Changes Detected in Diff

| File | Category | Changes |
|------|----------|---------|
| `api/v1alpha1/external_secrets_config_types.go` | API Types | New fields: `annotations`, `componentConfigs` in ControllerConfig; new types: ComponentConfig, DeploymentConfig, KVPair, Annotation; new enum values: Webhook, CertController |
| `api/v1alpha1/zz_generated.deepcopy.go` | Generated | DeepCopy for new types |
| `config/crd/bases/...externalsecretsconfigs.yaml` | CRD | Updated OpenAPI schema with new fields |
| `pkg/controller/external_secrets/component_config.go` | Controller | New file: applyAnnotations, applyComponentConfig, applyOverrideEnv, mergeEnvVars helpers |
| `pkg/controller/external_secrets/deployments.go` | Controller | Wired applyAnnotations and applyComponentConfig into getDeploymentObject |
| `api/v1alpha1/tests/.../externalsecretsconfig.testsuite.yaml` | Integration Tests | New test cases for validation of annotations, componentConfig, overrideEnv |

## Highly Recommended E2E Scenarios

### 1. Global Annotations (Priority: HIGH)
- **Why**: New API field + new controller logic. Must verify end-to-end that annotations flow from CR to Deployment and Pod template metadata.
- **Risk if untested**: Annotations could be silently dropped or only applied to Deployment but not Pod template.

### 2. Per-Component revisionHistoryLimit (Priority: HIGH)
- **Why**: New field that modifies Deployment spec directly. Must verify the correct Deployment is targeted based on componentName.
- **Risk if untested**: Wrong Deployment could get the limit, or limit could be ignored.

### 3. Per-Component overrideEnv (Priority: HIGH)
- **Why**: Merges user env vars into running containers. Must verify merge logic works (no duplicate, user takes precedence).
- **Risk if untested**: Env vars could clobber critical defaults or not appear in the container.

### 4. Update Scenarios (Priority: MEDIUM)
- **Why**: Verify that changing componentConfig triggers re-reconciliation and Deployment update.
- **Risk if untested**: Updates could be ignored if change detection doesn't work.

### 5. Combined Annotations + ComponentConfig (Priority: MEDIUM)
- **Why**: Verify both features work together without interference.
- **Risk if untested**: One feature could overwrite or interfere with the other.

## Optional/Nice-to-Have Scenarios

### 6. Reserved Prefix Filtering (Priority: LOW)
- **Why**: CEL validation should reject reserved prefixes at API level. Controller also filters them.
- **Note**: Already covered by integration tests in testsuite.yaml. E2E would be redundant.

### 7. Remove Configuration (Priority: LOW)
- **Why**: Verify that removing annotations/componentConfig reverts Deployment to defaults.
- **Note**: Complex to test reliably since defaults may vary.

### 8. Multiple ComponentConfigs for All Four Components (Priority: LOW)
- **Why**: Verify all four component types (Controller, Webhook, CertController, BitwardenSDKServer) can be configured.
- **Note**: BitwardenSDKServer requires plugin to be enabled, adding test complexity.

## Gaps
- **BitwardenSDKServer component**: Testing this requires the Bitwarden plugin to be enabled, which needs additional setup (TLS certs or cert-manager). Not included in generated tests.
- **Reserved prefix enforcement at API level**: Already covered by integration test suite (CEL validation). No need for e2e duplication.
- **Annotation removal/cleanup**: When annotations are removed from the CR, the operator would need to detect and remove them from Deployments. This behavior depends on reconciliation logic and may need manual verification.
