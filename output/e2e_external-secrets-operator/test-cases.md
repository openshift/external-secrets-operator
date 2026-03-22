# E2E Test Cases: external-secrets-operator

## Operator Information
- **Repository**: github.com/openshift/external-secrets-operator
- **Framework**: controller-runtime
- **API Group**: operator.openshift.io/v1alpha1
- **Managed CRDs**: ExternalSecretsConfig, ExternalSecretsManager
- **Operator Namespace**: external-secrets-operator
- **Operand Namespace**: external-secrets
- **Changes Analyzed**: git diff origin/ai-staging-release-1.0...HEAD (EP-1898: Component Configuration Overrides)

## Prerequisites
- OpenShift cluster with admin access
- `oc` CLI installed and authenticated
- External Secrets Operator installed (OLM or manual)
- Operator pod running in `external-secrets-operator` namespace

## Changes Summary
The diff introduces three new features to ExternalSecretsConfig:
1. **Global annotations** (`controllerConfig.annotations`) — applied to all component Deployments and Pod templates
2. **Component configs** (`controllerConfig.componentConfigs`) — per-component deployment overrides
3. **Override env vars** (`controllerConfig.componentConfigs[].overrideEnv`) — custom environment variables per component

## Test Cases

### TC-01: Global Annotations Applied to All Components
- **Test**: Verify that annotations specified in `controllerConfig.annotations` are applied to all operand Deployment and Pod template metadata
- **Steps**:
  1. Create/update ExternalSecretsConfig with `controllerConfig.annotations`
  2. Wait for reconciliation
  3. Check Deployment metadata annotations for each component
  4. Check Pod template metadata annotations for each component
- **Expected**: All operand Deployments and Pod templates contain the custom annotation

### TC-02: Reserved Annotation Prefixes Rejected
- **Test**: Verify that annotations with reserved prefixes are rejected by API validation
- **Steps**:
  1. Attempt to create ExternalSecretsConfig with annotation key `kubernetes.io/custom`
  2. Attempt with `app.kubernetes.io/custom`
  3. Attempt with `openshift.io/custom`
  4. Attempt with `k8s.io/custom`
- **Expected**: API server rejects each attempt with validation error

### TC-03: RevisionHistoryLimit Applied to Specific Component
- **Test**: Verify that `deploymentConfig.revisionHistoryLimit` is applied to the correct component deployment
- **Steps**:
  1. Create/update ExternalSecretsConfig with componentConfigs for ExternalSecretsCoreController with revisionHistoryLimit=5
  2. Wait for reconciliation
  3. Check the controller Deployment spec.revisionHistoryLimit
- **Expected**: Deployment has revisionHistoryLimit set to 5

### TC-04: RevisionHistoryLimit Minimum Validation
- **Test**: Verify that revisionHistoryLimit below 1 is rejected
- **Steps**:
  1. Attempt to set revisionHistoryLimit=0
- **Expected**: API server rejects with validation error

### TC-05: Override Env Vars Applied to Component Container
- **Test**: Verify that custom environment variables are merged into the component container spec
- **Steps**:
  1. Create/update ExternalSecretsConfig with overrideEnv containing GOMAXPROCS=4 for Controller
  2. Wait for reconciliation
  3. Check the controller Deployment container env vars
- **Expected**: GOMAXPROCS=4 is present in the container env vars

### TC-06: Reserved Env Var Prefixes Rejected
- **Test**: Verify that environment variables with reserved prefixes are rejected
- **Steps**:
  1. Attempt to set overrideEnv with name HOSTNAME
  2. Attempt with KUBERNETES_SERVICE_HOST
  3. Attempt with EXTERNAL_SECRETS_LOGLEVEL
- **Expected**: API server rejects each with validation error

### TC-07: Duplicate ComponentName Rejected
- **Test**: Verify that duplicate componentName entries in componentConfigs are rejected
- **Steps**:
  1. Attempt to create componentConfigs with two entries both having componentName=ExternalSecretsCoreController
- **Expected**: API server rejects with uniqueness validation error

### TC-08: Multiple Component Configs
- **Test**: Verify that multiple components can be configured simultaneously
- **Steps**:
  1. Create ExternalSecretsConfig with componentConfigs for Controller (revisionHistoryLimit=5) and Webhook (revisionHistoryLimit=3)
  2. Wait for reconciliation
  3. Check both Deployment specs
- **Expected**: Each Deployment has its respective revisionHistoryLimit

### TC-09: Combined Annotations and Component Configs
- **Test**: Verify annotations and componentConfigs work together
- **Steps**:
  1. Create ExternalSecretsConfig with both annotations and componentConfigs
  2. Wait for reconciliation
  3. Verify annotations on all Deployments
  4. Verify revisionHistoryLimit on targeted Deployment
  5. Verify overrideEnv on targeted container
- **Expected**: All configurations applied correctly

### TC-10: Update Annotations After Creation
- **Test**: Verify that updating annotations triggers re-reconciliation
- **Steps**:
  1. Create ExternalSecretsConfig with one annotation
  2. Wait for reconciliation
  3. Update to add a second annotation
  4. Wait for re-reconciliation
  5. Verify both annotations present
- **Expected**: Both annotations appear on Deployments after update

### TC-11: Remove ComponentConfigs
- **Test**: Verify that removing componentConfigs restores defaults
- **Steps**:
  1. Create ExternalSecretsConfig with revisionHistoryLimit=10
  2. Wait for reconciliation, verify it's set
  3. Remove the componentConfigs
  4. Wait for re-reconciliation
  5. Verify revisionHistoryLimit is back to default
- **Expected**: Default revisionHistoryLimit restored

## Verification
```bash
oc get externalsecretsconfigs cluster -o yaml
oc get deployments -n external-secrets -o yaml
oc get pods -n external-secrets
oc logs -n external-secrets-operator -l app.kubernetes.io/name=external-secrets-operator --tail=50
```

## Cleanup
```bash
oc patch externalsecretsconfigs cluster --type=merge -p '{"spec":{"controllerConfig":{"annotations":null,"componentConfigs":null}}}'
```
