# E2E Test Suggestions: external-secrets-operator

## Detected Operator Structure
- **Framework**: controller-runtime (Ginkgo e2e tests)
- **Managed CRDs**: ExternalSecretsConfig, ExternalSecretsManager
- **E2E Pattern**: Ginkgo with `dynamic.Interface`, `kubernetes.Clientset`, `utils.VerifyPodsReadyByPrefix`, `utils.WaitForESOResourceReady`
- **Operator Namespace**: `external-secrets-operator`
- **Operand Namespace**: `external-secrets`

## Changes Detected in Diff

### API Types (`api/v1alpha1/external_secrets_config_types.go`)
- Added `annotations` field to `ControllerConfig` (type `[]Annotation`, list-map by key)
- Added `componentConfigs` field to `ControllerConfig` (type `[]ComponentConfig`, list-map by componentName)
- Added `ComponentConfig` struct with `componentName`, `deploymentConfigs`, `overrideEnv`
- Added `DeploymentConfig` struct with `revisionHistoryLimit`
- Added `Annotation` struct with `key`, `value`
- Extended `ComponentName` enum with `Webhook`, `CertController`
- CEL validation for reserved annotation prefixes and env var prefixes

### Controller (`pkg/controller/external_secrets/component_config.go`)
- `applyAnnotationsToDeployment` — applies annotations to deployment and pod template
- `applyComponentConfigToDeployment` — applies per-component overrides
- `applyOverrideEnvToContainer` — merges env vars into target container
- Integration into `getDeploymentObject` pipeline

### CRD Manifest (`config/crd/bases/operator.openshift.io_externalsecretsconfigs.yaml`)
- Updated schema with new fields and validation rules

## Highly Recommended Scenarios

| # | Scenario | Reason | Priority |
|---|----------|--------|----------|
| 1 | Custom annotations applied to deployments | Core feature: verifies annotation propagation | **High** |
| 2 | Reserved annotation prefix rejection | Security: prevents platform annotation override | **High** |
| 3 | RevisionHistoryLimit applied per-component | Core feature: verifies deployment config override | **High** |
| 4 | OverrideEnv applied to container | Core feature: verifies env var injection | **High** |
| 5 | Reserved env var prefix rejection | Security: prevents reserved env var override | **High** |
| 6 | Multiple componentConfigs | Correctness: verifies independent component targeting | **Medium** |
| 7 | Update triggers re-reconciliation | Operational: verifies spec changes are picked up | **Medium** |

## Optional/Nice-to-Have Scenarios

| # | Scenario | Reason |
|---|----------|--------|
| 8 | All four components configured | Coverage: verifies BitwardenSDKServer and CertController |
| 9 | Annotation with empty value | Edge case: verifies empty string handling |
| 10 | Remove annotations on update | Lifecycle: verifies annotations are removed when cleared |
| 11 | OverrideEnv with valueFrom (fieldRef) | Edge case: verifies complex env var sources work |

## Gaps / Limitations

- **BitwardenSDKServer and CertController**: Testing these components requires specific configurations (bitwarden plugin enabled, cert-manager not configured). These may need conditional test setup.
- **Reserved prefix enforcement at controller level**: The CEL validation in the CRD catches reserved prefixes, but the controller also has a `disallowedAnnotationMatcher`. E2E tests primarily test the CRD-level validation since invalid configs never reach the controller.
- **Env var merge semantics**: Testing that existing env vars are overridden (not duplicated) requires knowing the default env vars in the deployment template. This is tightly coupled to the static manifest assets.
