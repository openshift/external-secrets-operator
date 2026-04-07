# E2E Execution Steps: external-secrets-operator

## Prerequisites

```bash
which oc
oc version
oc whoami
oc get nodes
oc get clusterversion
```

## Step 1: Verify Operator is Running

```bash
oc get pods -n external-secrets-operator
oc wait --for=condition=Ready pod -l app.kubernetes.io/name=external-secrets-operator -n external-secrets-operator --timeout=120s
```

## Step 2: Create ExternalSecretsConfig with Component Overrides

```bash
cat <<EOF | oc apply -f -
apiVersion: operator.openshift.io/v1alpha1
kind: ExternalSecretsConfig
metadata:
  name: cluster
spec:
  controllerConfig:
    annotations:
      - key: "example.com/e2e-test"
        value: "component-config-test"
    componentConfig:
      - componentName: ExternalSecretsCoreController
        deploymentConfigs:
          revisionHistoryLimit: 5
        overrideEnv:
          - name: GOMAXPROCS
            value: "4"
      - componentName: Webhook
        deploymentConfigs:
          revisionHistoryLimit: 3
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
EOF
```

## Step 3: Wait for Reconciliation

```bash
oc wait externalsecretsconfig cluster --for=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'=True --timeout=120s
```

## Step 4: Verify Annotations on Deployments

```bash
# Check controller deployment annotations
oc get deployment -n external-secrets -l app=external-secrets -o jsonpath='{.items[0].metadata.annotations.example\.com/e2e-test}'
# Expected: component-config-test

# Check pod template annotations
oc get deployment -n external-secrets -l app=external-secrets -o jsonpath='{.items[0].spec.template.metadata.annotations.example\.com/e2e-test}'
# Expected: component-config-test

# Check webhook deployment annotations
oc get deployment external-secrets-webhook -n external-secrets -o jsonpath='{.metadata.annotations.example\.com/e2e-test}'
# Expected: component-config-test
```

## Step 5: Verify revisionHistoryLimit

```bash
# Check controller revisionHistoryLimit
oc get deployment -n external-secrets -l app.kubernetes.io/name=external-secrets -o jsonpath='{.items[0].spec.revisionHistoryLimit}'
# Expected: 5

# Check webhook revisionHistoryLimit
oc get deployment external-secrets-webhook -n external-secrets -o jsonpath='{.spec.revisionHistoryLimit}'
# Expected: 3
```

## Step 6: Verify overrideEnv

```bash
# Check GOMAXPROCS env var on controller
oc get deployment -n external-secrets -l app.kubernetes.io/name=external-secrets -o jsonpath='{.items[0].spec.template.spec.containers[0].env[?(@.name=="GOMAXPROCS")].value}'
# Expected: 4
```

## Step 7: Update Configuration

```bash
oc patch externalsecretsconfig cluster --type=merge -p '
{
  "spec": {
    "controllerConfig": {
      "componentConfig": [
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

# Wait for re-reconciliation
sleep 10
oc wait externalsecretsconfig cluster --for=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'=True --timeout=120s

# Verify updated value
oc get deployment -n external-secrets -l app.kubernetes.io/name=external-secrets -o jsonpath='{.items[0].spec.revisionHistoryLimit}'
# Expected: 10
```

## Step 8: Cleanup

```bash
oc delete externalsecretsconfig cluster
```
