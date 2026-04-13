# E2E Test Cases: external-secrets-operator

## Operator Information
- **Repository**: github.com/openshift/external-secrets-operator
- **Framework**: controller-runtime
- **API Group**: operator.openshift.io/v1alpha1
- **Managed CRDs**: ExternalSecretsConfig, ExternalSecretsManager
- **Operator Namespace**: external-secrets-operator
- **Operand Namespace**: external-secrets
- **Changes Analyzed**: git diff release-1.0...HEAD

## Prerequisites
- OpenShift cluster with admin access
- `oc` CLI installed and authenticated
- Operator installed (via OLM or manual deployment)

## Installation
The operator is installed via OLM. The `ExternalSecretsConfig` CR named `cluster` triggers operand deployment.

```bash
oc apply -f config/samples/operator_v1alpha1_externalsecretsconfig.yaml
oc wait --for=condition=Ready externalsecretsconfigs/cluster --timeout=120s
```

## Test Cases

### TC-01: Global Annotations — Apply Custom Annotations to All Components
- **Test**: Create ExternalSecretsConfig with global annotations and verify they appear on all operand Deployments and Pod templates
- **Steps**:
  1. Patch ExternalSecretsConfig with `spec.controllerConfig.annotations`
  2. Wait for reconciliation
  3. Verify annotations on each Deployment (controller, webhook, cert-controller)
  4. Verify annotations on Pod templates of each Deployment
- **Expected**: Custom annotations appear on both Deployment metadata and Pod template metadata for all components

### TC-02: Global Annotations — Reserved Prefix Rejection
- **Test**: Attempt to set annotations with reserved prefixes via the API and verify they are rejected
- **Steps**:
  1. Try to create/update ExternalSecretsConfig with annotation key `kubernetes.io/test`
  2. Repeat for `app.kubernetes.io/`, `openshift.io/`, `k8s.io/` prefixes
- **Expected**: API server rejects the request with validation error

### TC-03: Global Annotations — Update and Removal
- **Test**: Add annotations, update their values, then remove them
- **Steps**:
  1. Add annotation `example.com/test: value1`
  2. Verify it appears on Deployments
  3. Update to `example.com/test: value2`
  4. Verify updated value
  5. Remove the annotation
  6. Verify it is no longer present
- **Expected**: Annotations are correctly added, updated, and removed through reconciliation

### TC-04: Component Config — RevisionHistoryLimit for Controller
- **Test**: Set revisionHistoryLimit for ExternalSecretsCoreController and verify it is applied
- **Steps**:
  1. Patch ExternalSecretsConfig with componentConfig for ExternalSecretsCoreController with revisionHistoryLimit: 5
  2. Wait for reconciliation
  3. Check the controller Deployment's spec.revisionHistoryLimit
- **Expected**: Deployment spec.revisionHistoryLimit equals 5

### TC-05: Component Config — RevisionHistoryLimit for Webhook
- **Test**: Set revisionHistoryLimit for Webhook component
- **Steps**:
  1. Patch with componentConfig for Webhook with revisionHistoryLimit: 3
  2. Verify on webhook Deployment
- **Expected**: Webhook Deployment spec.revisionHistoryLimit equals 3

### TC-06: Component Config — OverrideEnv for Controller
- **Test**: Set custom environment variables for the controller component
- **Steps**:
  1. Patch with componentConfig for ExternalSecretsCoreController with overrideEnv: [{name: GOMAXPROCS, value: "4"}]
  2. Wait for reconciliation and pod restart
  3. Check env vars on the external-secrets container in the controller Deployment
- **Expected**: GOMAXPROCS=4 appears in the container's env vars

### TC-07: Component Config — Reserved Env Var Prefix Rejection
- **Test**: Attempt to set environment variables with reserved prefixes
- **Steps**:
  1. Try to set overrideEnv with name HOSTNAME
  2. Try KUBERNETES_SERVICE_HOST
  3. Try EXTERNAL_SECRETS_CONFIG
- **Expected**: API server rejects with CEL validation error

### TC-08: Component Config — Multiple Components
- **Test**: Configure multiple components simultaneously
- **Steps**:
  1. Set componentConfig for ExternalSecretsCoreController (revisionHistoryLimit: 10) and Webhook (revisionHistoryLimit: 3)
  2. Wait for reconciliation
  3. Verify each Deployment has the correct revisionHistoryLimit
- **Expected**: Each component has its specific configuration applied independently

### TC-09: Component Config — Invalid Component Name
- **Test**: Attempt to use an invalid componentName enum value
- **Steps**:
  1. Try to patch with componentName: InvalidComponent
- **Expected**: API server rejects with enum validation error

### TC-10: Component Config — Duplicate Component Names
- **Test**: Attempt to configure the same component twice
- **Steps**:
  1. Try to patch with two entries for ExternalSecretsCoreController
- **Expected**: API server rejects with uniqueness validation error

### TC-11: Combined Configuration — Annotations + Component Configs
- **Test**: Apply both annotations and component configs together
- **Steps**:
  1. Patch with annotations and componentConfig for controller with revisionHistoryLimit and overrideEnv
  2. Verify annotations on all Deployments
  3. Verify revisionHistoryLimit on controller Deployment
  4. Verify overrideEnv on controller container
- **Expected**: All configurations are applied correctly together

### TC-12: Reconciliation Recovery — Config Drift
- **Test**: Manually modify a Deployment and verify the operator restores it
- **Steps**:
  1. Configure revisionHistoryLimit: 5 via ExternalSecretsConfig
  2. Manually edit the Deployment to change revisionHistoryLimit to 1
  3. Wait for reconciliation
  4. Verify revisionHistoryLimit is restored to 5
- **Expected**: Operator reconciles the Deployment back to the desired state

## Verification
```bash
oc get externalsecretsconfigs cluster -o yaml
oc get deployments -n external-secrets -o yaml
oc get pods -n external-secrets
oc logs -n external-secrets-operator -l app.kubernetes.io/name=external-secrets-operator --tail=100
```

## Cleanup
```bash
oc patch externalsecretsconfigs cluster --type=merge -p '{"spec":{"controllerConfig":{"annotations":null,"componentConfig":null}}}'
```
