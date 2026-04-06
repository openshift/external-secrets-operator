# E2E Execution Steps: external-secrets-operator

## Prerequisites

```bash
which oc
oc version
oc whoami
oc get nodes
oc get clusterversion
```

## Step 1: Verify Operator is Installed

```bash
# Check operator pod is running
oc get pods -n external-secrets-operator
oc wait --for=condition=Available deployment/external-secrets-operator-controller-manager \
  -n external-secrets-operator --timeout=120s
```

## Step 2: Create ExternalSecretsConfig with Component Configuration

```bash
cat <<EOF | oc apply -f -
apiVersion: operator.openshift.io/v1alpha1
kind: ExternalSecretsConfig
metadata:
  name: cluster
spec:
  controllerConfig:
    annotations:
      - key: "example.com/managed-by"
        value: "e2e-test"
      - key: "example.com/version"
        value: "1.0"
    componentConfigs:
      - componentName: ExternalSecretsCoreController
        deploymentConfigs:
          revisionHistoryLimit: 5
        overrideEnv:
          - name: GOMAXPROCS
            value: "4"
      - componentName: Webhook
        deploymentConfigs:
          revisionHistoryLimit: 3
EOF
```

## Step 3: Wait for Reconciliation

```bash
oc wait --for=condition=Ready externalsecretsconfig/cluster --timeout=120s
```

## Step 4: Verify Operand Pods are Running

```bash
oc get pods -n external-secrets
oc wait --for=condition=Ready pod -l app=external-secrets -n external-secrets --timeout=120s
```

## Step 5: Verify Annotations on Deployments

```bash
# Core controller deployment
oc get deployment external-secrets -n external-secrets \
  -o jsonpath='{.metadata.annotations.example\.com/managed-by}'
# Expected: e2e-test

# Pod template annotations
oc get deployment external-secrets -n external-secrets \
  -o jsonpath='{.spec.template.metadata.annotations.example\.com/managed-by}'
# Expected: e2e-test

# Webhook deployment
oc get deployment external-secrets-webhook -n external-secrets \
  -o jsonpath='{.metadata.annotations.example\.com/managed-by}'
# Expected: e2e-test
```

## Step 6: Verify RevisionHistoryLimit

```bash
# Core controller: should be 5
oc get deployment external-secrets -n external-secrets \
  -o jsonpath='{.spec.revisionHistoryLimit}'
# Expected: 5

# Webhook: should be 3
oc get deployment external-secrets-webhook -n external-secrets \
  -o jsonpath='{.spec.revisionHistoryLimit}'
# Expected: 3
```

## Step 7: Verify OverrideEnv

```bash
# Core controller container should have GOMAXPROCS=4
oc get deployment external-secrets -n external-secrets \
  -o jsonpath='{.spec.template.spec.containers[?(@.name=="external-secrets")].env[?(@.name=="GOMAXPROCS")].value}'
# Expected: 4
```

## Step 8: Verify Validation - Reserved Annotation Prefix

```bash
# This should fail
cat <<EOF | oc apply -f - 2>&1
apiVersion: operator.openshift.io/v1alpha1
kind: ExternalSecretsConfig
metadata:
  name: cluster
spec:
  controllerConfig:
    annotations:
      - key: "kubernetes.io/reserved"
        value: "should-fail"
EOF
# Expected: Error about reserved prefixes
```

## Step 9: Verify Validation - Reserved Env Var Prefix

```bash
# This should fail
cat <<EOF | oc apply -f - 2>&1
apiVersion: operator.openshift.io/v1alpha1
kind: ExternalSecretsConfig
metadata:
  name: cluster
spec:
  controllerConfig:
    componentConfigs:
      - componentName: ExternalSecretsCoreController
        overrideEnv:
          - name: KUBERNETES_SERVICE_HOST
            value: "10.0.0.1"
EOF
# Expected: Error about reserved env var prefixes
```

## Step 10: Update ComponentConfig and Verify Re-reconciliation

```bash
# Update revisionHistoryLimit
oc patch externalsecretsconfig cluster --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "componentConfigs": [
        {
          "componentName": "ExternalSecretsCoreController",
          "deploymentConfigs": {
            "revisionHistoryLimit": 10
          }
        }
      ]
    }
  }
}'

# Wait for reconciliation
sleep 15
oc wait --for=condition=Ready externalsecretsconfig/cluster --timeout=60s

# Verify new value
oc get deployment external-secrets -n external-secrets \
  -o jsonpath='{.spec.revisionHistoryLimit}'
# Expected: 10
```

## Step 11: Cleanup

```bash
oc delete externalsecretsconfig cluster
```
