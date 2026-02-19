# E2E Execution Steps: external-secrets-operator

## Prerequisites

```bash
which oc
oc version
oc whoami
oc get nodes
oc get clusterversion
oc get packagemanifests | grep external-secrets
```

## Environment Variables

```bash
export OPERATOR_NAMESPACE="external-secrets-operator"
export OPERAND_NAMESPACE="external-secrets"
export ESC_NAME="cluster"
```

## Step 1: Verify Operator Installation

```bash
# Verify operator is running
oc get pods -n ${OPERATOR_NAMESPACE} -l app.kubernetes.io/name=external-secrets-operator
oc wait --for=condition=Available deployment/external-secrets-operator-controller-manager \
  -n ${OPERATOR_NAMESPACE} --timeout=120s

# Verify ExternalSecretsConfig exists
oc get externalsecretsconfig ${ESC_NAME} -o yaml
oc wait --for=condition=Ready externalsecretsconfig/${ESC_NAME} --timeout=120s
```

## Step 2: Verify Static NetworkPolicies

```bash
# List all network policies in operand namespace
oc get networkpolicies -n ${OPERAND_NAMESPACE}

# Verify deny-all policy exists
oc get networkpolicy deny-all-traffic -n ${OPERAND_NAMESPACE} -o yaml

# Verify DNS allow policy exists and covers all components
oc get networkpolicy allow-to-dns -n ${OPERAND_NAMESPACE} -o yaml

# Verify main controller allow policy
oc get networkpolicy allow-api-server-egress-for-main-controller -n ${OPERAND_NAMESPACE} -o yaml

# Verify webhook allow policy
oc get networkpolicy allow-api-server-and-webhook-traffic -n ${OPERAND_NAMESPACE} -o yaml
```

## Step 3: Test Custom NetworkPolicy Creation

```bash
# Patch ExternalSecretsConfig to add a custom network policy
oc patch externalsecretsconfig ${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "networkPolicies": [
        {
          "name": "allow-core-egress",
          "componentName": "ExternalSecretsCoreController",
          "egress": [
            {
              "ports": [
                {
                  "protocol": "TCP",
                  "port": 6443
                }
              ]
            }
          ]
        }
      ]
    }
  }
}'

# Wait for reconciliation
sleep 10
oc wait --for=condition=Ready externalsecretsconfig/${ESC_NAME} --timeout=60s

# Verify custom NetworkPolicy was created
oc get networkpolicy allow-core-egress -n ${OPERAND_NAMESPACE} -o yaml
```

## Step 4: Test DNS Subdomain Name Validation

```bash
# Test invalid name with uppercase (should fail)
oc patch externalsecretsconfig ${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "networkPolicies": [
        {
          "name": "Allow-Egress",
          "componentName": "ExternalSecretsCoreController",
          "egress": [{"ports": [{"protocol": "TCP", "port": 6443}]}]
        }
      ]
    }
  }
}' 2>&1 | grep -i "invalid"

# Test invalid name with underscore (should fail)
oc patch externalsecretsconfig ${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "networkPolicies": [
        {
          "name": "allow_egress",
          "componentName": "ExternalSecretsCoreController",
          "egress": [{"ports": [{"protocol": "TCP", "port": 6443}]}]
        }
      ]
    }
  }
}' 2>&1 | grep -i "invalid"

# Test invalid name starting with hyphen (should fail)
oc patch externalsecretsconfig ${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "networkPolicies": [
        {
          "name": "-allow-egress",
          "componentName": "ExternalSecretsCoreController",
          "egress": [{"ports": [{"protocol": "TCP", "port": 6443}]}]
        }
      ]
    }
  }
}' 2>&1 | grep -i "invalid"
```

## Step 5: Test Empty Egress (Deny-All)

```bash
# Patch with empty egress for deny-all behavior
oc patch externalsecretsconfig ${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "networkPolicies": [
        {
          "name": "deny-all-core-egress",
          "componentName": "ExternalSecretsCoreController",
          "egress": []
        }
      ]
    }
  }
}'

sleep 10
oc get networkpolicy deny-all-core-egress -n ${OPERAND_NAMESPACE} -o yaml
```

## Step 6: Test Egress Update (Mutable)

```bash
# Update the egress rules to add a new port
oc patch externalsecretsconfig ${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "networkPolicies": [
        {
          "name": "allow-core-egress",
          "componentName": "ExternalSecretsCoreController",
          "egress": [
            {"ports": [{"protocol": "TCP", "port": 6443}]},
            {"ports": [{"protocol": "TCP", "port": 443}]}
          ]
        }
      ]
    }
  }
}'

sleep 10
oc get networkpolicy allow-core-egress -n ${OPERAND_NAMESPACE} -o jsonpath='{.spec.egress}' | python3 -m json.tool
```

## Step 7: Test Invalid ComponentName

```bash
# Should fail with Unsupported value
oc patch externalsecretsconfig ${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "networkPolicies": [
        {
          "name": "test-policy",
          "componentName": "InvalidComponent",
          "egress": [{"ports": [{"protocol": "TCP", "port": 6443}]}]
        }
      ]
    }
  }
}' 2>&1 | grep -i "unsupported"
```

## Step 8: Cleanup Custom NetworkPolicies

```bash
# Remove custom network policies by setting empty list
oc patch externalsecretsconfig ${ESC_NAME} --type=merge -p '{
  "spec": {
    "controllerConfig": {
      "networkPolicies": []
    }
  }
}'

sleep 10

# Verify custom policies were cleaned up
oc get networkpolicies -n ${OPERAND_NAMESPACE} -l operator.openshift.io/custom-network-policy=true
```
