# E2E Execution Steps: external-secrets-operator

## Prerequisites

```bash
which oc
oc version
oc whoami
oc get nodes
oc get clusterversion
```

## Step 1: Verify Operator Installation

```bash
# Verify operator pod is running
oc get pods -n external-secrets-operator -l app.kubernetes.io/name=external-secrets-operator
oc wait --for=condition=Ready pod -n external-secrets-operator -l app.kubernetes.io/name=external-secrets-operator --timeout=120s

# Verify ExternalSecretsConfig CRD exists
oc get crd externalsecretsconfigs.operator.openshift.io
```

## Step 2: Verify Operand is Running

```bash
# Verify operand pods are running
oc get pods -n external-secrets
oc wait --for=condition=Ready pod -n external-secrets -l app=external-secrets --timeout=120s

# Check ExternalSecretsConfig status
oc get externalsecretsconfigs cluster -o jsonpath='{.status.conditions}' | jq .
```

## Step 3: Test Global Annotations

```bash
# Apply custom annotation
oc patch externalsecretsconfigs cluster --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "annotations": [
        {"key": "example.com/custom-annotation", "value": "e2e-test-value"}
      ]
    }
  }
}'

# Wait for reconciliation
sleep 10

# Verify annotation on controller deployment metadata
oc get deployment -n external-secrets external-secrets -o jsonpath='{.metadata.annotations.example\.com/custom-annotation}'
# Expected: e2e-test-value

# Verify annotation on pod template
oc get deployment -n external-secrets external-secrets -o jsonpath='{.spec.template.metadata.annotations.example\.com/custom-annotation}'
# Expected: e2e-test-value

# Verify annotation on webhook deployment
oc get deployment -n external-secrets external-secrets-webhook -o jsonpath='{.metadata.annotations.example\.com/custom-annotation}'
# Expected: e2e-test-value
```

## Step 4: Test Reserved Annotation Prefix Rejection

```bash
# Attempt to apply reserved annotation (should fail)
oc patch externalsecretsconfigs cluster --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "annotations": [
        {"key": "kubernetes.io/reserved", "value": "should-fail"}
      ]
    }
  }
}' 2>&1 | grep -q "annotations with reserved prefixes"
echo "Reserved prefix correctly rejected: $?"
```

## Step 5: Test Component Config - RevisionHistoryLimit

```bash
# Apply revisionHistoryLimit for controller
oc patch externalsecretsconfigs cluster --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "componentConfigs": [
        {
          "componentName": "ExternalSecretsCoreController",
          "deploymentConfig": {
            "revisionHistoryLimit": 5
          }
        }
      ]
    }
  }
}'

# Wait for reconciliation
sleep 10

# Verify revisionHistoryLimit
LIMIT=$(oc get deployment -n external-secrets external-secrets -o jsonpath='{.spec.revisionHistoryLimit}')
echo "RevisionHistoryLimit: $LIMIT"
# Expected: 5
```

## Step 6: Test Override Env Vars

```bash
# Apply custom env var
oc patch externalsecretsconfigs cluster --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "componentConfigs": [
        {
          "componentName": "ExternalSecretsCoreController",
          "deploymentConfig": {
            "revisionHistoryLimit": 5
          },
          "overrideEnv": [
            {"name": "GOMAXPROCS", "value": "4"}
          ]
        }
      ]
    }
  }
}'

# Wait for reconciliation
sleep 10

# Verify env var in container spec
oc get deployment -n external-secrets external-secrets -o jsonpath='{.spec.template.spec.containers[0].env}' | jq '.[] | select(.name == "GOMAXPROCS")'
# Expected: {"name": "GOMAXPROCS", "value": "4"}
```

## Step 7: Test Reserved Env Var Prefix Rejection

```bash
# Attempt to apply reserved env var (should fail)
oc patch externalsecretsconfigs cluster --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "componentConfigs": [
        {
          "componentName": "ExternalSecretsCoreController",
          "overrideEnv": [
            {"name": "KUBERNETES_SERVICE_HOST", "value": "should-fail"}
          ]
        }
      ]
    }
  }
}' 2>&1 | grep -q "environment variable names with reserved prefixes"
echo "Reserved env var prefix correctly rejected: $?"
```

## Step 8: Test Multiple Component Configs

```bash
# Apply configs for multiple components
oc patch externalsecretsconfigs cluster --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "componentConfigs": [
        {
          "componentName": "ExternalSecretsCoreController",
          "deploymentConfig": {"revisionHistoryLimit": 10}
        },
        {
          "componentName": "Webhook",
          "deploymentConfig": {"revisionHistoryLimit": 3}
        }
      ]
    }
  }
}'

# Wait for reconciliation
sleep 10

# Verify both deployments
CTRL_LIMIT=$(oc get deployment -n external-secrets external-secrets -o jsonpath='{.spec.revisionHistoryLimit}')
WH_LIMIT=$(oc get deployment -n external-secrets external-secrets-webhook -o jsonpath='{.spec.revisionHistoryLimit}')
echo "Controller RevisionHistoryLimit: $CTRL_LIMIT (expected: 10)"
echo "Webhook RevisionHistoryLimit: $WH_LIMIT (expected: 3)"
```

## Step 9: Cleanup

```bash
# Remove all component config and annotation overrides
oc patch externalsecretsconfigs cluster --type=json -p '[
  {"op": "remove", "path": "/spec/controllerConfig/annotations"},
  {"op": "remove", "path": "/spec/controllerConfig/componentConfigs"}
]'

# Verify reconciliation restores defaults
sleep 10
oc get deployment -n external-secrets external-secrets -o jsonpath='{.spec.revisionHistoryLimit}'
oc get deployment -n external-secrets external-secrets -o jsonpath='{.metadata.annotations}' | jq 'keys'
```
