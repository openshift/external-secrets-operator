# E2E Test Cases: external-secrets-operator

## Operator Information
- **Repository**: github.com/openshift/external-secrets-operator
- **Framework**: controller-runtime
- **API Group**: operator.openshift.io
- **Managed CRDs**: ExternalSecretsConfig, ExternalSecretsManager
- **Operator Namespace**: external-secrets-operator
- **Operand Namespace**: external-secrets
- **Changes Analyzed**: git diff ai-staging-release-1.0...HEAD

## Prerequisites
- OpenShift cluster with admin access
- `oc` CLI installed and authenticated
- External Secrets Operator installed via OLM

## Installation
The operator is installed via OLM in the `external-secrets-operator` namespace.

```bash
oc get csv -n external-secrets-operator | grep external-secrets
oc get pods -n external-secrets-operator
```

## CR Deployment
Create the ExternalSecretsConfig singleton:

```bash
oc apply -f config/samples/operator_v1alpha1_externalsecretsconfig.yaml
oc wait --for=condition=Ready externalsecretsconfig/cluster --timeout=120s
```

## Test Cases

### 1. Custom Annotations Propagation

**Test**: Verify that custom annotations from `controllerConfig.annotations` are propagated to all operand Deployments and Pod templates.

**Steps**:
1. Create ExternalSecretsConfig with custom annotations
2. Wait for reconciliation
3. Check that all operand Deployments have the specified annotations
4. Check that Pod templates within Deployments have the specified annotations

**Expected**:
- All operand Deployments (controller, webhook, cert-controller) have the custom annotations
- Pod templates within those Deployments also have the custom annotations
- Reserved prefix annotations (kubernetes.io/, openshift.io/) are rejected by CRD validation

### 2. Reserved Annotation Prefix Rejection

**Test**: Verify that annotations with reserved prefixes are rejected at CRD validation level.

**Steps**:
1. Attempt to create/update ExternalSecretsConfig with annotations using `kubernetes.io/` prefix
2. Attempt with `app.kubernetes.io/` prefix
3. Attempt with `openshift.io/` prefix
4. Attempt with `k8s.io/` prefix

**Expected**: All attempts are rejected with appropriate validation error messages.

### 3. Component-Specific RevisionHistoryLimit

**Test**: Verify that `componentConfigs[].deploymentConfig.revisionHistoryLimit` is applied per-component.

**Steps**:
1. Create ExternalSecretsConfig with componentConfigs setting revisionHistoryLimit for CoreController
2. Wait for reconciliation
3. Check the controller Deployment's spec.revisionHistoryLimit

**Expected**: The specified Deployment has the configured revisionHistoryLimit value.

### 4. Component-Specific Environment Variable Overrides

**Test**: Verify that `componentConfigs[].overrideEnv` merges custom env vars into the specified component's containers.

**Steps**:
1. Create ExternalSecretsConfig with overrideEnv for CoreController (e.g., GOMAXPROCS=4)
2. Wait for reconciliation
3. Check the controller Deployment's container env vars

**Expected**: The specified container has the custom env var merged with defaults.

### 5. Reserved Environment Variable Prefix Rejection

**Test**: Verify that environment variables with reserved prefixes are rejected.

**Steps**:
1. Attempt to set overrideEnv with name starting with HOSTNAME
2. Attempt with KUBERNETES_ prefix
3. Attempt with EXTERNAL_SECRETS_ prefix

**Expected**: All attempts are rejected by CRD validation.

### 6. Multiple Component Configs

**Test**: Verify that multiple components can be independently configured.

**Steps**:
1. Create ExternalSecretsConfig with componentConfigs for both CoreController and Webhook
2. Set different revisionHistoryLimit and overrideEnv for each
3. Wait for reconciliation
4. Verify each Deployment has its own configuration applied independently

**Expected**: Each component Deployment reflects its own specific configuration.

### 7. Annotation Update Propagation

**Test**: Verify that updating annotations triggers re-reconciliation and applies changes.

**Steps**:
1. Create ExternalSecretsConfig with initial annotations
2. Wait for reconciliation
3. Update annotations with new values
4. Wait for re-reconciliation
5. Verify new annotations are applied to Deployments

**Expected**: Updated annotations are propagated to all operand Deployments.

### 8. New ComponentName Enum Values for NetworkPolicies

**Test**: Verify that the new Webhook and CertController component names work for network policies.

**Steps**:
1. Create ExternalSecretsConfig with networkPolicies using componentName: Webhook
2. Create networkPolicy with componentName: CertController
3. Wait for reconciliation

**Expected**: NetworkPolicies are created for the new component types.

## Verification

```bash
# Check operator pod
oc get pods -n external-secrets-operator

# Check operand pods
oc get pods -n external-secrets

# Check ExternalSecretsConfig status
oc get externalsecretsconfig cluster -o yaml

# Check Deployment annotations
oc get deployment -n external-secrets -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.metadata.annotations}{"\n"}{end}'

# Check Pod template annotations
oc get deployment -n external-secrets -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.template.metadata.annotations}{"\n"}{end}'

# Check revisionHistoryLimit
oc get deployment -n external-secrets -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.revisionHistoryLimit}{"\n"}{end}'
```

## Cleanup

```bash
oc delete externalsecretsconfig cluster
```
