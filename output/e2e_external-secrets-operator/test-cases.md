# E2E Test Cases: external-secrets-operator

## Operator Information
- **Repository**: github.com/openshift/external-secrets-operator
- **Framework**: controller-runtime
- **API Group**: operator.openshift.io/v1alpha1
- **Managed CRDs**: ExternalSecretsConfig, ExternalSecretsManager
- **Operator Namespace**: external-secrets-operator
- **Operand Namespace**: external-secrets
- **Changes Analyzed**: git diff origin/ai-staging-release-1.0...HEAD

## Prerequisites
- OpenShift cluster with admin access
- `oc` CLI installed and authenticated
- Operator installed via OLM or pre-deployed

## Changes Under Test (EP-1898)
1. **New API fields**: `annotations`, `componentConfig` (with `deploymentConfigs.revisionHistoryLimit` and `overrideEnv`) in `ControllerConfig`
2. **New types**: `ComponentConfig`, `DeploymentConfig`, `KVPair`, `Annotation`
3. **New ComponentName values**: `Webhook`, `CertController`
4. **Controller logic**: `applyAnnotations`, `applyComponentConfig`, `applyOverrideEnv` applied to operand Deployments
5. **Network policy**: `getPodSelectorForComponent` supports `Webhook` and `CertController`

## Test Cases

### TC-01: Custom Annotations Applied to Deployments
- **Test**: Verify custom annotations from `controllerConfig.annotations` are propagated to all operand Deployment and Pod template ObjectMeta
- **Steps**:
  1. Create ExternalSecretsConfig with `controllerConfig.annotations` containing `example.com/test-annotation: e2e-value`
  2. Wait for reconciliation to complete (Ready=True condition)
  3. Check each operand Deployment (`external-secrets`, `external-secrets-webhook`, `external-secrets-cert-controller`) for the annotation on both Deployment and Pod template
- **Expected**: All operand Deployments and Pod templates contain annotation `example.com/test-annotation: e2e-value`

### TC-02: Reserved Annotation Prefix Rejection
- **Test**: Verify CRD validation rejects annotations with reserved prefixes (`kubernetes.io/`, `app.kubernetes.io/`, `openshift.io/`, `k8s.io/`)
- **Steps**:
  1. Attempt to create/update ExternalSecretsConfig with annotation key `kubernetes.io/custom`
- **Expected**: API server rejects the request with validation error about reserved prefixes

### TC-03: ComponentConfig revisionHistoryLimit Applied
- **Test**: Verify `componentConfig.deploymentConfigs.revisionHistoryLimit` is applied to the targeted component's Deployment
- **Steps**:
  1. Create ExternalSecretsConfig with componentConfig targeting `ExternalSecretsCoreController` with `revisionHistoryLimit: 5`
  2. Wait for reconciliation
  3. Check the `external-secrets` Deployment's `spec.revisionHistoryLimit`
- **Expected**: Deployment has `revisionHistoryLimit: 5`

### TC-04: ComponentConfig overrideEnv Applied
- **Test**: Verify `componentConfig.overrideEnv` merges environment variables into the targeted container
- **Steps**:
  1. Create ExternalSecretsConfig with componentConfig targeting `Webhook` with `overrideEnv: [{name: GOMAXPROCS, value: "4"}]`
  2. Wait for reconciliation
  3. Check the `external-secrets-webhook` Deployment's container env vars
- **Expected**: Webhook container has `GOMAXPROCS=4` environment variable

### TC-05: Reserved Env Var Prefix Rejection
- **Test**: Verify CRD validation rejects env var names with reserved prefixes (`HOSTNAME`, `KUBERNETES_`, `EXTERNAL_SECRETS_`)
- **Steps**:
  1. Attempt to update ExternalSecretsConfig with `overrideEnv: [{name: KUBERNETES_SERVICE_HOST, value: "10.0.0.1"}]`
- **Expected**: API server rejects with validation error about reserved prefixes

### TC-06: Multiple ComponentConfigs for Different Components
- **Test**: Verify multiple componentConfig entries for different components are each applied correctly
- **Steps**:
  1. Create ExternalSecretsConfig with componentConfig entries for `ExternalSecretsCoreController` (revisionHistoryLimit: 10) and `Webhook` (revisionHistoryLimit: 3)
  2. Wait for reconciliation
  3. Check both Deployments
- **Expected**: Core controller Deployment has revisionHistoryLimit 10, Webhook Deployment has revisionHistoryLimit 3

### TC-07: Duplicate ComponentName Rejection
- **Test**: Verify CRD validation rejects duplicate componentName entries
- **Steps**:
  1. Attempt to create ExternalSecretsConfig with two componentConfig entries both targeting `ExternalSecretsCoreController`
- **Expected**: API server rejects with "componentName must be unique" error

### TC-08: Network Policy with Webhook Component
- **Test**: Verify network policies can target the `Webhook` component
- **Steps**:
  1. Create ExternalSecretsConfig with a networkPolicy targeting `componentName: Webhook`
  2. Wait for reconciliation
  3. Check NetworkPolicy pod selector matches `external-secrets-webhook` pods
- **Expected**: NetworkPolicy created with `app.kubernetes.io/name: external-secrets-webhook` pod selector

### TC-09: Network Policy with CertController Component
- **Test**: Verify network policies can target the `CertController` component
- **Steps**:
  1. Create ExternalSecretsConfig with a networkPolicy targeting `componentName: CertController`
  2. Wait for reconciliation
  3. Check NetworkPolicy pod selector
- **Expected**: NetworkPolicy created with `app.kubernetes.io/name: external-secrets-cert-controller` pod selector

### TC-10: Update Annotations After Initial Creation
- **Test**: Verify annotations can be updated after initial CR creation
- **Steps**:
  1. Create ExternalSecretsConfig with annotation `example.com/v1: initial`
  2. Wait for reconciliation
  3. Update to `example.com/v1: updated` and add `example.com/v2: new`
  4. Wait for reconciliation
- **Expected**: All Deployments reflect updated annotations

### TC-11: Reconciliation Completes with Combined Config
- **Test**: Verify successful reconciliation with annotations, componentConfig, labels, and networkPolicies all configured together
- **Steps**:
  1. Create ExternalSecretsConfig with all new fields configured simultaneously
  2. Wait for Ready=True condition
  3. Verify all configurations applied
- **Expected**: Operator reports Ready=True, all configurations applied correctly

## Cleanup
1. Delete ExternalSecretsConfig CR
2. Wait for operand resources to be cleaned up
