# E2E Execution Steps: external-secrets-operator (Network Policy Feature)

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
# Verify the operator is installed and running
oc get pods -n external-secrets-operator
oc wait --for=condition=Ready pod -l app=external-secrets-operator -n external-secrets-operator --timeout=120s
```

## Step 2: Create ExternalSecretsConfig

```bash
# Create the ExternalSecretsConfig CR with custom network policies
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
          - {}
EOF

# Wait for the ExternalSecretsConfig to become Ready
oc wait --for=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'=True \
  externalsecretsconfig/cluster --timeout=180s
```

## Step 3: Verify Operand Pods

```bash
# Wait for operand pods to be running
oc wait --for=condition=Ready pod -l app.kubernetes.io/name=external-secrets \
  -n external-secrets --timeout=120s
oc wait --for=condition=Ready pod -l app.kubernetes.io/name=external-secrets-webhook \
  -n external-secrets --timeout=120s
oc wait --for=condition=Ready pod -l app.kubernetes.io/name=external-secrets-cert-controller \
  -n external-secrets --timeout=120s

oc get pods -n external-secrets
```

## Step 4: Verify Static Network Policies

```bash
# Verify all expected static network policies exist
echo "=== Network Policies in operand namespace ==="
oc get networkpolicies -n external-secrets

# Verify deny-all policy
oc get networkpolicy deny-all-traffic -n external-secrets -o yaml

# Verify main controller traffic policy
oc get networkpolicy allow-api-server-egress-for-main-controller-traffic -n external-secrets -o yaml

# Verify webhook traffic policy
oc get networkpolicy allow-api-server-egress-for-webhook -n external-secrets -o yaml

# Verify DNS traffic policy
oc get networkpolicy allow-to-dns -n external-secrets -o yaml

# Verify cert-controller traffic policy (should exist when cert-manager is NOT enabled)
oc get networkpolicy allow-api-server-egress-for-cert-controller -n external-secrets -o yaml

echo "=== Network Policies in operator namespace ==="
oc get networkpolicies -n external-secrets-operator
```

## Step 5: Verify Custom Network Policy

```bash
# Verify the custom network policy was created
oc get networkpolicy allow-external-secrets-egress -n external-secrets -o yaml

# Verify it has the correct pod selector
oc get networkpolicy allow-external-secrets-egress -n external-secrets \
  -o jsonpath='{.spec.podSelector.matchLabels}'
# Expected: {"app.kubernetes.io/name":"external-secrets"}
```

## Step 6: Test Custom Network Policy Update

```bash
# Update the custom network policy egress rules
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
          - ports:
              - protocol: TCP
                port: 443
          - ports:
              - protocol: TCP
                port: 8443
EOF

# Wait for reconciliation
sleep 10

# Verify the network policy was updated
oc get networkpolicy allow-external-secrets-egress -n external-secrets -o yaml
```

## Step 7: Test BitwardenSDKServer Misconfiguration Warning

```bash
# Add a BitwardenSDKServer network policy without enabling the plugin
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
          - {}
      - name: allow-bitwarden-egress
        componentName: BitwardenSDKServer
        egress:
          - ports:
              - protocol: TCP
                port: 6443
EOF

# Check for warning event
sleep 10
oc get events -n "" --field-selector involvedObject.name=cluster,reason=NetworkPolicyMisconfiguration
```

## Step 8: Verify Webhook Accessibility

```bash
# Create a test namespace for validating webhook function
oc create namespace external-secrets-e2e-test || true

# Test webhook by creating a SecretStore (webhook validates this)
cat <<EOF | oc apply -f - 2>&1 || true
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: test-webhook-e2e
  namespace: external-secrets-e2e-test
spec:
  provider:
    fake:
      data: []
EOF

# If the webhook responds (success or validation error), it proves network connectivity
echo "Webhook is accessible if the above command got a response (even if validation failed)"

# Cleanup
oc delete secretstore test-webhook-e2e -n external-secrets-e2e-test --ignore-not-found
oc delete namespace external-secrets-e2e-test --ignore-not-found
```

## Step 9: Verify ExternalSecretsConfig Status

```bash
# Check the ExternalSecretsConfig status conditions
oc get externalsecretsconfig cluster -o jsonpath='{.status.conditions}' | python3 -m json.tool

# Verify Ready=True
READY=$(oc get externalsecretsconfig cluster -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')
if [ "$READY" = "True" ]; then
  echo "PASS: ExternalSecretsConfig is Ready"
else
  echo "FAIL: ExternalSecretsConfig is not Ready"
fi
```

## Step 10: Cleanup

```bash
# Reset to minimal config
cat <<EOF | oc apply -f -
apiVersion: operator.openshift.io/v1alpha1
kind: ExternalSecretsConfig
metadata:
  name: cluster
spec: {}
EOF
```
