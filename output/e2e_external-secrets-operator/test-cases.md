# E2E Test Cases: external-secrets-operator

## Operator Information
- **Repository**: github.com/openshift/external-secrets-operator
- **Framework**: controller-runtime
- **API Group**: operator.openshift.io/v1alpha1
- **Managed CRDs**: ExternalSecretsConfig, ExternalSecretsManager
- **Operator Namespace**: external-secrets-operator
- **Operand Namespace**: external-secrets
- **Changes Analyzed**: git diff release-1.0...HEAD (EP-1898: Component Configuration Overrides)

## Prerequisites
- OpenShift cluster with admin access
- `oc` CLI installed and authenticated
- Operator deployed via OLM or manual install

## Changes Summary
The diff introduces:
1. **API Types**: New fields in `ControllerConfig`: `annotations` (global) and `componentConfigs` (per-component overrides)
2. **New Types**: `ComponentConfig`, `DeploymentConfig`, `KVPair`, `Annotation`
3. **New ComponentName values**: `Webhook`, `CertController`
4. **Controller Logic**: `component_config.go` — applies annotations, revisionHistoryLimit, and overrideEnv to deployments
5. **Wiring**: Modified `deployments.go` to call `applyAnnotations()` and `applyComponentConfig()`

## Test Cases

### TC-1: Global Annotations Applied to All Deployments
- **Test**: Verify that annotations specified in `controllerConfig.annotations` are applied to all operand Deployment metadata and Pod template metadata.
- **Steps**:
  1. Create ExternalSecretsConfig with `controllerConfig.annotations` containing a custom annotation
  2. Wait for reconciliation to complete (Ready condition = True)
  3. Verify annotation exists on each operand Deployment's metadata
  4. Verify annotation exists on each operand Deployment's Pod template metadata
- **Expected**: Custom annotation appears on both Deployment `.metadata.annotations` and `.spec.template.metadata.annotations` for all components

### TC-2: Per-Component revisionHistoryLimit
- **Test**: Verify that `revisionHistoryLimit` from componentConfig is applied to the correct Deployment.
- **Steps**:
  1. Create ExternalSecretsConfig with componentConfig for ExternalSecretsCoreController setting revisionHistoryLimit=5
  2. Wait for reconciliation
  3. Check the external-secrets Deployment's `.spec.revisionHistoryLimit`
- **Expected**: The controller Deployment has revisionHistoryLimit=5

### TC-3: Per-Component overrideEnv
- **Test**: Verify that custom environment variables from `overrideEnv` are merged into the component container.
- **Steps**:
  1. Create ExternalSecretsConfig with componentConfig for ExternalSecretsCoreController with overrideEnv `GOMAXPROCS=4`
  2. Wait for reconciliation
  3. Check the external-secrets Deployment container spec for the env var
- **Expected**: The GOMAXPROCS env var appears in the controller container's env list

### TC-4: Combined Annotations and ComponentConfig
- **Test**: Verify that both global annotations and per-component configs work together.
- **Steps**:
  1. Create ExternalSecretsConfig with both annotations and componentConfig
  2. Wait for reconciliation
  3. Verify annotations on all Deployments
  4. Verify revisionHistoryLimit on the specified component
  5. Verify overrideEnv on the specified component
- **Expected**: All configurations are applied correctly and simultaneously

### TC-5: Update Annotations After Initial Creation
- **Test**: Verify that updating annotations triggers re-reconciliation and updates Deployments.
- **Steps**:
  1. Create ExternalSecretsConfig with initial annotations
  2. Wait for Ready condition
  3. Update the annotations with new values
  4. Wait for Ready condition again
  5. Verify new annotation values on Deployments
- **Expected**: Updated annotations are reflected on Deployments

### TC-6: Update revisionHistoryLimit
- **Test**: Verify that changing revisionHistoryLimit triggers a Deployment update.
- **Steps**:
  1. Create ExternalSecretsConfig with revisionHistoryLimit=5
  2. Wait for reconciliation
  3. Update to revisionHistoryLimit=10
  4. Wait for reconciliation
  5. Verify Deployment has revisionHistoryLimit=10
- **Expected**: Deployment spec is updated with new value

### TC-7: Multiple Component Configs
- **Test**: Verify that different components can have independent configurations.
- **Steps**:
  1. Create ExternalSecretsConfig with componentConfigs for both Controller and Webhook
  2. Wait for reconciliation
  3. Verify each component's Deployment has its own revisionHistoryLimit
- **Expected**: Each component Deployment reflects its specific configuration

## Verification
```bash
oc get externalsecretsconfig cluster -o yaml
oc get deployments -n external-secrets -o yaml
oc get pods -n external-secrets
```

## Cleanup
```bash
oc delete externalsecretsconfig cluster
```
