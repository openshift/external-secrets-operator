# E2E Test Cases: external-secrets-operator

## Operator Information
- **Repository**: github.com/openshift/external-secrets-operator
- **Framework**: controller-runtime
- **API Group**: operator.openshift.io
- **Managed CRDs**: ExternalSecretsConfig, ExternalSecretsManager
- **Operator Namespace**: external-secrets-operator
- **Operand Namespace**: external-secrets
- **Changes Analyzed**: git diff upstream/ai-staging-release-1.0...HEAD

## Prerequisites
- OpenShift cluster with admin access
- `oc` CLI installed and authenticated
- External Secrets Operator installed via OLM

## Installation
The operator is installed via OLM. The ExternalSecretsConfig CR (`cluster`) triggers the operand installation.

```bash
oc apply -f config/samples/operator_v1alpha1_externalsecretsconfig.yaml
oc wait --for=condition=Ready externalsecretsconfig/cluster --timeout=120s
```

## Test Cases

### TC-1: Custom Annotations Applied to Deployments
**Objective**: Verify that custom annotations from `controllerConfig.annotations` are applied to all operand deployments and pod templates.

**Steps**:
1. Create ExternalSecretsConfig with custom annotations
2. Wait for reconciliation
3. Verify annotations on core controller deployment
4. Verify annotations on webhook deployment
5. Verify annotations on cert-controller deployment
6. Verify annotations on pod templates of each deployment

**Expected**: All deployment objects and their pod templates should have the custom annotations.

### TC-2: Reserved Annotation Prefixes Are Rejected
**Objective**: Verify CRD validation rejects annotations with reserved prefixes.

**Steps**:
1. Attempt to create ExternalSecretsConfig with annotation key `kubernetes.io/test`
2. Attempt with `app.kubernetes.io/test`
3. Attempt with `openshift.io/test`
4. Attempt with `k8s.io/test`

**Expected**: All creation attempts should be rejected by CRD validation.

### TC-3: ComponentConfig RevisionHistoryLimit Applied
**Objective**: Verify that `componentConfigs.deploymentConfigs.revisionHistoryLimit` is applied to target deployment.

**Steps**:
1. Create ExternalSecretsConfig with componentConfigs for ExternalSecretsCoreController with revisionHistoryLimit=5
2. Wait for reconciliation
3. Verify core controller deployment has revisionHistoryLimit=5
4. Verify other deployments have default revisionHistoryLimit

**Expected**: Only the targeted deployment has revisionHistoryLimit overridden.

### TC-4: ComponentConfig OverrideEnv Applied
**Objective**: Verify that `componentConfigs.overrideEnv` adds environment variables to the target container.

**Steps**:
1. Create ExternalSecretsConfig with componentConfigs for ExternalSecretsCoreController with overrideEnv GOMAXPROCS=4
2. Wait for reconciliation
3. Verify core controller container has GOMAXPROCS=4 env var

**Expected**: The custom env var should appear in the target container spec.

### TC-5: Reserved Env Var Prefixes Are Rejected
**Objective**: Verify CRD validation rejects env vars with reserved name prefixes.

**Steps**:
1. Attempt to create ExternalSecretsConfig with overrideEnv name `HOSTNAME`
2. Attempt with `KUBERNETES_SERVICE_HOST`
3. Attempt with `EXTERNAL_SECRETS_CONFIG`

**Expected**: All creation attempts should be rejected by CRD validation.

### TC-6: Multiple ComponentConfigs for Different Components
**Objective**: Verify that multiple componentConfig entries targeting different components work correctly.

**Steps**:
1. Create ExternalSecretsConfig with componentConfigs for both ExternalSecretsCoreController (revisionHistoryLimit=10) and Webhook (revisionHistoryLimit=3)
2. Wait for reconciliation
3. Verify each deployment has its respective revisionHistoryLimit

**Expected**: Each targeted deployment should have its own revisionHistoryLimit value.

### TC-7: ComponentConfig Update Triggers Reconciliation
**Objective**: Verify that updating componentConfigs triggers reconciliation and applies changes.

**Steps**:
1. Create ExternalSecretsConfig with revisionHistoryLimit=5 for ExternalSecretsCoreController
2. Wait for reconciliation
3. Update revisionHistoryLimit to 10
4. Wait for re-reconciliation
5. Verify deployment has revisionHistoryLimit=10

**Expected**: The deployment should be updated with the new revisionHistoryLimit value.

### TC-8: Invalid ComponentName Rejected
**Objective**: Verify CRD validation rejects invalid componentName values.

**Steps**:
1. Attempt to create ExternalSecretsConfig with componentName="InvalidComponent"

**Expected**: Creation should be rejected by enum validation.

### TC-9: MaxItems Constraint on ComponentConfigs
**Objective**: Verify that no more than 4 componentConfigs entries are allowed.

**Steps**:
1. Attempt to create ExternalSecretsConfig with 5 componentConfigs entries

**Expected**: Creation should be rejected with maxItems validation error.

### TC-10: RevisionHistoryLimit Minimum Constraint
**Objective**: Verify that revisionHistoryLimit must be >= 1.

**Steps**:
1. Attempt to create ExternalSecretsConfig with revisionHistoryLimit=0

**Expected**: Creation should be rejected with minimum validation error.

## Verification
```bash
oc get externalsecretsconfig cluster -o yaml
oc get deployments -n external-secrets -o yaml
oc get pods -n external-secrets
oc logs deployment/external-secrets-operator-controller-manager -n external-secrets-operator
```

## Cleanup
```bash
oc delete externalsecretsconfig cluster
```
