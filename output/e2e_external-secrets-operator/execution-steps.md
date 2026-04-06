# E2E Execution Steps: external-secrets-operator

## Prerequisites

```bash
which oc
oc version
oc whoami
oc get nodes
oc get clusterversion
```

## Environment Variables

```bash
export OPERATOR_NAMESPACE="external-secrets-operator"
export OPERAND_NAMESPACE="external-secrets"
```

## Step 1: Verify Operator Installation

```bash
# Check operator pod is running
oc get pods -n ${OPERATOR_NAMESPACE} -l app.kubernetes.io/name=external-secrets-operator

# Wait for operator deployment to be available
oc wait deployment -n ${OPERATOR_NAMESPACE} external-secrets-operator-controller-manager \
  --for=condition=Available --timeout=120s
```

## Step 2: Create ExternalSecretsConfig CR

```bash
# Create ExternalSecretsConfig with minimal spec
cat <<EOF | oc apply -f -
apiVersion: operator.openshift.io/v1alpha1
kind: ExternalSecretsConfig
metadata:
  name: cluster
spec:
  controllerConfig:
    networkPolicies:
      - name: allow-external-secrets-egress
        componentName: ExternalSecretsCoreController
        egress:
          - to: []
            ports:
            - protocol: TCP
              port: 6443
            - protocol: TCP
              port: 443
            - protocol: TCP
              port: 5353
            - protocol: UDP
              port: 5353
      - name: allow-webhook-egress
        componentName: Webhook
        egress:
          - to: []
            ports:
            - protocol: TCP
              port: 6443
            - protocol: TCP
              port: 443
      - name: allow-cert-controller-egress
        componentName: CertController
        egress:
          - to: []
            ports:
            - protocol: TCP
              port: 6443
            - protocol: TCP
              port: 443
EOF

# Wait for operand pods to be ready
oc wait deployment -n ${OPERAND_NAMESPACE} external-secrets --for=condition=Available --timeout=120s
oc wait deployment -n ${OPERAND_NAMESPACE} external-secrets-webhook --for=condition=Available --timeout=120s
oc wait deployment -n ${OPERAND_NAMESPACE} external-secrets-cert-controller --for=condition=Available --timeout=120s
```

## Step 3: Test Custom Annotations (TC-01)

```bash
# Update ExternalSecretsConfig with custom annotations
oc patch externalsecretsconfig cluster --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "annotations": [
        {"key": "example.com/test-annotation", "value": "e2e-value"}
      ]
    }
  }
}'

# Wait for reconciliation
sleep 15

# Verify annotation on each operand deployment
for DEPLOY in external-secrets external-secrets-webhook external-secrets-cert-controller; do
  echo "Checking $DEPLOY..."
  oc get deployment -n ${OPERAND_NAMESPACE} ${DEPLOY} -o jsonpath='{.metadata.annotations.example\.com/test-annotation}'
  echo ""
  oc get deployment -n ${OPERAND_NAMESPACE} ${DEPLOY} -o jsonpath='{.spec.template.metadata.annotations.example\.com/test-annotation}'
  echo ""
done
```

## Step 4: Test Reserved Annotation Rejection (TC-02)

```bash
# Attempt to set a reserved annotation - should be rejected
oc patch externalsecretsconfig cluster --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "annotations": [
        {"key": "kubernetes.io/custom", "value": "forbidden"}
      ]
    }
  }
}' 2>&1 | grep -i "reserved"
```

## Step 5: Test ComponentConfig revisionHistoryLimit (TC-03)

```bash
# Apply revisionHistoryLimit to core controller
oc patch externalsecretsconfig cluster --type=merge -p '{
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

# Wait for reconciliation
sleep 15

# Verify revisionHistoryLimit
oc get deployment -n ${OPERAND_NAMESPACE} external-secrets -o jsonpath='{.spec.revisionHistoryLimit}'
echo ""
# Expected: 5
```

## Step 6: Test ComponentConfig overrideEnv (TC-04)

```bash
# Apply overrideEnv to Webhook
oc patch externalsecretsconfig cluster --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "componentConfig": [
        {
          "componentName": "ExternalSecretsCoreController",
          "deploymentConfigs": {
            "revisionHistoryLimit": 5
          }
        },
        {
          "componentName": "Webhook",
          "overrideEnv": [
            {"name": "GOMAXPROCS", "value": "4"}
          ]
        }
      ]
    }
  }
}'

# Wait for reconciliation
sleep 15

# Verify environment variable
oc get deployment -n ${OPERAND_NAMESPACE} external-secrets-webhook -o jsonpath='{.spec.template.spec.containers[0].env}' | python3 -m json.tool
# Expected: GOMAXPROCS=4 present in output
```

## Step 7: Test Reserved Env Var Rejection (TC-05)

```bash
# Attempt to set a reserved env var - should be rejected
oc patch externalsecretsconfig cluster --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "componentConfig": [
        {
          "componentName": "Webhook",
          "overrideEnv": [
            {"name": "KUBERNETES_SERVICE_HOST", "value": "10.0.0.1"}
          ]
        }
      ]
    }
  }
}' 2>&1 | grep -i "reserved"
```

## Step 8: Test Duplicate ComponentName Rejection (TC-07)

```bash
# Attempt to create duplicate componentName entries - should be rejected
oc patch externalsecretsconfig cluster --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "componentConfig": [
        {
          "componentName": "ExternalSecretsCoreController",
          "deploymentConfigs": {"revisionHistoryLimit": 5}
        },
        {
          "componentName": "ExternalSecretsCoreController",
          "deploymentConfigs": {"revisionHistoryLimit": 3}
        }
      ]
    }
  }
}' 2>&1 | grep -i "unique"
```

## Step 9: Test Network Policy with Webhook Component (TC-08)

```bash
# Check NetworkPolicy pod selector for Webhook
oc get networkpolicy -n ${OPERAND_NAMESPACE} -l app.kubernetes.io/name=external-secrets-webhook -o yaml
```

## Step 10: Test Network Policy with CertController Component (TC-09)

```bash
# Check NetworkPolicy pod selector for CertController
oc get networkpolicy -n ${OPERAND_NAMESPACE} -l app.kubernetes.io/name=external-secrets-cert-controller -o yaml
```

## Step 11: Cleanup

```bash
# Delete ExternalSecretsConfig CR
oc delete externalsecretsconfig cluster --timeout=60s

# Wait for operand resources to be cleaned up
oc wait deployment -n ${OPERAND_NAMESPACE} external-secrets --for=delete --timeout=120s 2>/dev/null || true
oc wait deployment -n ${OPERAND_NAMESPACE} external-secrets-webhook --for=delete --timeout=120s 2>/dev/null || true
oc wait deployment -n ${OPERAND_NAMESPACE} external-secrets-cert-controller --for=delete --timeout=120s 2>/dev/null || true
```
