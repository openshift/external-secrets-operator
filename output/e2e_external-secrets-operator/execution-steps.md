# E2E Execution Steps: external-secrets-operator

## Prerequisites

```bash
which oc
oc version
oc whoami
oc get nodes
oc get clusterversion
oc get packagemanifests -n openshift-marketplace | grep external-secrets
```

## Environment Variables

```bash
export OPERATOR_NAMESPACE="external-secrets-operator"
export OPERAND_NAMESPACE="external-secrets"
export ESC_NAME="cluster"
```

## Step 1: Install Operator

```bash
# Verify the operator is installed and running
oc get pods -n ${OPERATOR_NAMESPACE}
oc wait --for=condition=Ready pods -l app.kubernetes.io/name=external-secrets-operator -n ${OPERATOR_NAMESPACE} --timeout=120s
```

## Step 2: Deploy ExternalSecretsConfig CR

```bash
oc apply -f config/samples/operator_v1alpha1_externalsecretsconfig.yaml
oc wait --for=condition=Ready externalsecretsconfigs/${ESC_NAME} --timeout=120s
```

## Step 3: Verify Operand Pods

```bash
# Wait for all operand pods to be ready
oc get pods -n ${OPERAND_NAMESPACE}
oc wait --for=condition=Ready pods -l app.kubernetes.io/name=external-secrets -n ${OPERAND_NAMESPACE} --timeout=120s
oc wait --for=condition=Ready pods -l app.kubernetes.io/name=external-secrets-webhook -n ${OPERAND_NAMESPACE} --timeout=120s
oc wait --for=condition=Ready pods -l app.kubernetes.io/name=external-secrets-cert-controller -n ${OPERAND_NAMESPACE} --timeout=120s
```

## Step 4: Diff-Specific Tests

### Test 4.1: Global Annotations — Apply Custom Annotations

```bash
# Patch ExternalSecretsConfig with global annotations
oc patch externalsecretsconfigs/${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "annotations": [
        {"key": "example.com/team", "value": "platform"},
        {"key": "example.com/env", "value": "e2e-test"}
      ]
    }
  }
}'

# Wait for reconciliation (controller processes the change)
sleep 10

# Verify annotations on controller Deployment metadata
oc get deployment -n ${OPERAND_NAMESPACE} external-secrets -o jsonpath='{.metadata.annotations.example\.com/team}'
# Expected: platform

oc get deployment -n ${OPERAND_NAMESPACE} external-secrets -o jsonpath='{.metadata.annotations.example\.com/env}'
# Expected: e2e-test

# Verify annotations on controller Pod template
oc get deployment -n ${OPERAND_NAMESPACE} external-secrets -o jsonpath='{.spec.template.metadata.annotations.example\.com/team}'
# Expected: platform

# Verify annotations on webhook Deployment
oc get deployment -n ${OPERAND_NAMESPACE} external-secrets-webhook -o jsonpath='{.metadata.annotations.example\.com/team}'
# Expected: platform

# Verify annotations on cert-controller Deployment
oc get deployment -n ${OPERAND_NAMESPACE} external-secrets-cert-controller -o jsonpath='{.metadata.annotations.example\.com/team}'
# Expected: platform
```

### Test 4.2: Global Annotations — Reserved Prefix Rejection

```bash
# Try to set annotation with kubernetes.io/ prefix — should be rejected by API
oc patch externalsecretsconfigs/${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "annotations": [
        {"key": "kubernetes.io/test", "value": "blocked"}
      ]
    }
  }
}'
# Expected: admission error mentioning reserved prefix

# Try with app.kubernetes.io/ prefix
oc patch externalsecretsconfigs/${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "annotations": [
        {"key": "app.kubernetes.io/test", "value": "blocked"}
      ]
    }
  }
}'
# Expected: admission error

# Try with openshift.io/ prefix
oc patch externalsecretsconfigs/${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "annotations": [
        {"key": "openshift.io/test", "value": "blocked"}
      ]
    }
  }
}'
# Expected: admission error

# Try with k8s.io/ prefix
oc patch externalsecretsconfigs/${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "annotations": [
        {"key": "k8s.io/test", "value": "blocked"}
      ]
    }
  }
}'
# Expected: admission error
```

### Test 4.3: Global Annotations — Update and Removal

```bash
# Update annotation value
oc patch externalsecretsconfigs/${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "annotations": [
        {"key": "example.com/team", "value": "updated-team"}
      ]
    }
  }
}'

sleep 10

# Verify updated value
oc get deployment -n ${OPERAND_NAMESPACE} external-secrets -o jsonpath='{.metadata.annotations.example\.com/team}'
# Expected: updated-team

# Remove annotations
oc patch externalsecretsconfigs/${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "annotations": []
    }
  }
}'
```

### Test 4.4: ComponentConfig — RevisionHistoryLimit for Controller

```bash
# Set revisionHistoryLimit for controller
oc patch externalsecretsconfigs/${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "componentConfig": [
        {
          "componentName": "ExternalSecretsCoreController",
          "deploymentConfigs": {
            "revisionHistoryLimit": 5
          }
        }
      ]
    }
  }
}'

sleep 10

# Verify revisionHistoryLimit on controller Deployment
oc get deployment -n ${OPERAND_NAMESPACE} external-secrets -o jsonpath='{.spec.revisionHistoryLimit}'
# Expected: 5
```

### Test 4.5: ComponentConfig — RevisionHistoryLimit for Webhook

```bash
# Set revisionHistoryLimit for webhook
oc patch externalsecretsconfigs/${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "componentConfig": [
        {
          "componentName": "Webhook",
          "deploymentConfigs": {
            "revisionHistoryLimit": 3
          }
        }
      ]
    }
  }
}'

sleep 10

# Verify on webhook Deployment
oc get deployment -n ${OPERAND_NAMESPACE} external-secrets-webhook -o jsonpath='{.spec.revisionHistoryLimit}'
# Expected: 3
```

### Test 4.6: ComponentConfig — OverrideEnv for Controller

```bash
# Set custom environment variable for controller
oc patch externalsecretsconfigs/${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "componentConfig": [
        {
          "componentName": "ExternalSecretsCoreController",
          "overrideEnv": [
            {"name": "GOMAXPROCS", "value": "4"}
          ]
        }
      ]
    }
  }
}'

sleep 15

# Verify env var on controller container
oc get deployment -n ${OPERAND_NAMESPACE} external-secrets -o jsonpath='{.spec.template.spec.containers[?(@.name=="external-secrets")].env[?(@.name=="GOMAXPROCS")].value}'
# Expected: 4
```

### Test 4.7: ComponentConfig — Reserved Env Var Rejection

```bash
# Try HOSTNAME — should be rejected
oc patch externalsecretsconfigs/${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "componentConfig": [
        {
          "componentName": "ExternalSecretsCoreController",
          "overrideEnv": [
            {"name": "HOSTNAME", "value": "custom"}
          ]
        }
      ]
    }
  }
}'
# Expected: admission error about reserved prefix

# Try KUBERNETES_SERVICE_HOST
oc patch externalsecretsconfigs/${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "componentConfig": [
        {
          "componentName": "ExternalSecretsCoreController",
          "overrideEnv": [
            {"name": "KUBERNETES_SERVICE_HOST", "value": "custom"}
          ]
        }
      ]
    }
  }
}'
# Expected: admission error about reserved prefix

# Try EXTERNAL_SECRETS_CONFIG
oc patch externalsecretsconfigs/${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "componentConfig": [
        {
          "componentName": "ExternalSecretsCoreController",
          "overrideEnv": [
            {"name": "EXTERNAL_SECRETS_CONFIG", "value": "custom"}
          ]
        }
      ]
    }
  }
}'
# Expected: admission error about reserved prefix
```

### Test 4.8: ComponentConfig — Multiple Components

```bash
# Configure multiple components simultaneously
oc patch externalsecretsconfigs/${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "componentConfig": [
        {
          "componentName": "ExternalSecretsCoreController",
          "deploymentConfigs": {
            "revisionHistoryLimit": 10
          }
        },
        {
          "componentName": "Webhook",
          "deploymentConfigs": {
            "revisionHistoryLimit": 3
          }
        }
      ]
    }
  }
}'

sleep 10

# Verify each component
oc get deployment -n ${OPERAND_NAMESPACE} external-secrets -o jsonpath='{.spec.revisionHistoryLimit}'
# Expected: 10

oc get deployment -n ${OPERAND_NAMESPACE} external-secrets-webhook -o jsonpath='{.spec.revisionHistoryLimit}'
# Expected: 3
```

### Test 4.9: ComponentConfig — Invalid Component Name

```bash
# Try invalid component name — should be rejected
oc patch externalsecretsconfigs/${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "componentConfig": [
        {
          "componentName": "InvalidComponent"
        }
      ]
    }
  }
}'
# Expected: admission error about enum validation
```

### Test 4.10: ComponentConfig — Duplicate Component Names

```bash
# Try duplicate component names — should be rejected
oc patch externalsecretsconfigs/${ESC_NAME} --type=json -p '[
  {"op": "replace", "path": "/spec/controllerConfig/componentConfig", "value": [
    {"componentName": "ExternalSecretsCoreController", "deploymentConfigs": {"revisionHistoryLimit": 5}},
    {"componentName": "ExternalSecretsCoreController", "deploymentConfigs": {"revisionHistoryLimit": 10}}
  ]}
]'
# Expected: admission error about componentName uniqueness
```

### Test 4.11: Reconciliation Recovery — Config Drift

```bash
# Set revisionHistoryLimit to 5
oc patch externalsecretsconfigs/${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "componentConfig": [
        {
          "componentName": "ExternalSecretsCoreController",
          "deploymentConfigs": {
            "revisionHistoryLimit": 5
          }
        }
      ]
    }
  }
}'

sleep 10

# Manually modify the deployment (drift)
oc patch deployment -n ${OPERAND_NAMESPACE} external-secrets --type=merge -p '{"spec":{"revisionHistoryLimit":1}}'

# Wait for operator to reconcile
sleep 15

# Verify restored to desired state
oc get deployment -n ${OPERAND_NAMESPACE} external-secrets -o jsonpath='{.spec.revisionHistoryLimit}'
# Expected: 5
```

## Step 5: Cleanup

```bash
# Remove all custom configuration
oc patch externalsecretsconfigs/${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "annotations": [],
      "componentConfig": []
    }
  }
}'

# Wait for reconciliation
sleep 10

# Verify cleanup
oc get deployments -n ${OPERAND_NAMESPACE} -o yaml
```
