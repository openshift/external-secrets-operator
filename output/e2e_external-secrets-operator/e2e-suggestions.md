# E2E Test Suggestions: external-secrets-operator (EP-1898)

## Detected Operator Structure
- **Framework**: controller-runtime (kubebuilder/operator-sdk)
- **Managed CRDs**: ExternalSecretsConfig, ExternalSecretsManager
- **E2E Pattern**: Ginkgo v2 with `Ordered` Describe blocks
- **Operator Namespace**: `external-secrets-operator`
- **Operand Namespace**: `external-secrets`
- **Install**: OLM-based (CSV in config/manifests)

## Changes Detected in Diff

| File | Category | Changes |
|------|----------|---------|
| `api/v1alpha1/external_secrets_config_types.go` | API Types | New types: ComponentConfig, DeploymentConfig, KVPair, Annotation. New fields: ControllerConfig.Annotations, ControllerConfig.ComponentConfigs. New enum values: Webhook, CertController |
| `pkg/controller/external_secrets/component_config.go` | Controller | New reconciliation logic: applyAnnotations, applyComponentConfig, applyOverrideEnv, mergeEnvVars |
| `pkg/controller/external_secrets/deployments.go` | Controller | Integration of component config into getDeploymentObject |
| `api/v1alpha1/zz_generated.deepcopy.go` | Generated | Auto-generated deepcopy functions for new types |
| `config/crd/bases/...externalsecretsconfigs.yaml` | CRD | Schema updates for new fields |
| `api/v1alpha1/tests/.../externalsecretsconfig.testsuite.yaml` | Integration Tests | New test cases for annotations, componentConfigs, overrideEnv |

## Highly Recommended E2E Scenarios

### 1. Annotations Applied to All Components (Priority: HIGH)
- **Why**: Core feature from EP-1898, affects all operand deployments
- **Test**: Set annotation, verify it appears on all Deployment and Pod template metadata
- **Risk if not tested**: Annotations may not propagate to all components

### 2. RevisionHistoryLimit Applied Correctly (Priority: HIGH)
- **Why**: Directly modifies Deployment spec, affects rollback capability
- **Test**: Set revisionHistoryLimit per-component, verify on target Deployment
- **Risk if not tested**: Incorrect deployment spec may prevent rollbacks

### 3. Override Env Vars Merged (Priority: HIGH)
- **Why**: Modifies container environment, could affect component behavior
- **Test**: Set custom env vars, verify they appear in container spec
- **Risk if not tested**: Env vars may not be injected or may override reserved vars

### 4. Reserved Prefix Rejection - Annotations (Priority: MEDIUM)
- **Why**: CEL validation rule at API level, but also controller-level filtering
- **Test**: Attempt to set annotation with `kubernetes.io/` prefix
- **Risk if not tested**: Reserved annotations could interfere with platform

### 5. Reserved Prefix Rejection - Env Vars (Priority: MEDIUM)
- **Why**: CEL validation at API level prevents reserved env var overrides
- **Test**: Attempt to set env var with `KUBERNETES_` prefix
- **Risk if not tested**: Critical env vars could be overridden

### 6. Multiple Component Configs (Priority: MEDIUM)
- **Why**: Tests list semantics and per-component isolation
- **Test**: Configure Controller and Webhook with different values
- **Risk if not tested**: Config may bleed across components

## Optional / Nice-to-Have Scenarios

### 7. Configuration Removal Restores Defaults (Priority: LOW)
- **Why**: Verifies idempotent reconciliation after config removal
- **Test**: Add config, verify, remove, verify restoration

### 8. Annotation Update Re-reconciliation (Priority: LOW)
- **Why**: Verifies spec-change detection triggers reconciliation
- **Test**: Modify annotation value, verify updated on Deployments

### 9. Stress Test: All 4 Components Configured (Priority: LOW)
- **Why**: Tests maximum componentConfigs (4) with all component types
- **Test**: Configure all 4 components simultaneously

## Gaps / Hard to Test Automatically

1. **Bitwarden component**: Requires Bitwarden plugin enabled with TLS certs, making it hard to test in standard e2e environments
2. **CertController component**: Only present when cert-manager is NOT installed, so testing depends on cluster configuration
3. **Annotation merge precedence**: User annotations should override operator defaults, but testing requires knowing the exact default annotations
4. **Rolling update behavior**: Verifying that annotation/env changes trigger proper rolling updates requires watching ReplicaSets over time
