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
# Check the operator is installed
oc get csv -n external-secrets-operator | grep external-secrets

# Wait for operator pod to be ready
oc wait --for=condition=Ready pods -l app.kubernetes.io/name=external-secrets-operator -n external-secrets-operator --timeout=120s

# Verify operator pod
oc get pods -n external-secrets-operator
```

## Step 2: Create ExternalSecretsConfig with Default Settings

```bash
# Create minimal ExternalSecretsConfig
cat <<'EOF' | oc apply -f -
apiVersion: operator.openshift.io/v1alpha1
kind: ExternalSecretsConfig
metadata:
  name: cluster
spec: {}
EOF

# Wait for Ready condition
oc wait --for=condition=Ready externalsecretsconfig/cluster --timeout=120s

# Verify operand pods are running
oc get pods -n external-secrets
```

## Step 3: Test Custom Annotations

```bash
# Apply ExternalSecretsConfig with custom annotations
cat <<'EOF' | oc apply -f -
apiVersion: operator.openshift.io/v1alpha1
kind: ExternalSecretsConfig
metadata:
  name: cluster
spec:
  controllerConfig:
    annotations:
      - key: "example.com/team"
        value: "platform-engineering"
      - key: "example.com/environment"
        value: "staging"
EOF

# Wait for reconciliation
sleep 10

# Verify annotations on controller deployment
oc get deployment external-secrets -n external-secrets -o jsonpath='{.metadata.annotations.example\.com/team}'
# Expected: platform-engineering

# Verify annotations on webhook deployment
oc get deployment external-secrets-webhook -n external-secrets -o jsonpath='{.metadata.annotations.example\.com/team}'
# Expected: platform-engineering

# Verify annotations on pod template
oc get deployment external-secrets -n external-secrets -o jsonpath='{.spec.template.metadata.annotations.example\.com/team}'
# Expected: platform-engineering
```

## Step 4: Test Reserved Annotation Prefix Rejection

```bash
# Try to set annotation with reserved kubernetes.io/ prefix (should fail)
cat <<'EOF' | oc apply -f - 2>&1 | grep -i "reserved"
apiVersion: operator.openshift.io/v1alpha1
kind: ExternalSecretsConfig
metadata:
  name: cluster
spec:
  controllerConfig:
    annotations:
      - key: "kubernetes.io/test"
        value: "should-fail"
EOF
# Expected: Error containing "annotations with reserved prefixes"
```

## Step 5: Test Component Configs - RevisionHistoryLimit

```bash
# Apply per-component configuration
cat <<'EOF' | oc apply -f -
apiVersion: operator.openshift.io/v1alpha1
kind: ExternalSecretsConfig
metadata:
  name: cluster
spec:
  controllerConfig:
    componentConfigs:
      - componentName: ExternalSecretsCoreController
        deploymentConfig:
          revisionHistoryLimit: 5
      - componentName: Webhook
        deploymentConfig:
          revisionHistoryLimit: 3
EOF

# Wait for reconciliation
sleep 10

# Check revisionHistoryLimit on controller deployment
oc get deployment external-secrets -n external-secrets -o jsonpath='{.spec.revisionHistoryLimit}'
# Expected: 5

# Check revisionHistoryLimit on webhook deployment
oc get deployment external-secrets-webhook -n external-secrets -o jsonpath='{.spec.revisionHistoryLimit}'
# Expected: 3
```

## Step 6: Test Component Configs - Override Environment Variables

```bash
# Apply overrideEnv configuration
cat <<'EOF' | oc apply -f -
apiVersion: operator.openshift.io/v1alpha1
kind: ExternalSecretsConfig
metadata:
  name: cluster
spec:
  controllerConfig:
    componentConfigs:
      - componentName: ExternalSecretsCoreController
        overrideEnv:
          - name: GOMAXPROCS
            value: "4"
EOF

# Wait for reconciliation
sleep 15

# Verify env var in controller deployment
oc get deployment external-secrets -n external-secrets -o jsonpath='{.spec.template.spec.containers[0].env}' | grep -o 'GOMAXPROCS'
# Expected: GOMAXPROCS found
```

## Step 7: Test Reserved Env Var Prefix Rejection

```bash
# Try to set env var with reserved HOSTNAME prefix (should fail)
cat <<'EOF' | oc apply -f - 2>&1 | grep -i "reserved"
apiVersion: operator.openshift.io/v1alpha1
kind: ExternalSecretsConfig
metadata:
  name: cluster
spec:
  controllerConfig:
    componentConfigs:
      - componentName: ExternalSecretsCoreController
        overrideEnv:
          - name: HOSTNAME
            value: "should-fail"
EOF
# Expected: Error containing "reserved prefixes"
```

## Step 8: Cleanup

```bash
# Reset ExternalSecretsConfig to default
cat <<'EOF' | oc apply -f -
apiVersion: operator.openshift.io/v1alpha1
kind: ExternalSecretsConfig
metadata:
  name: cluster
spec: {}
EOF

# Or delete the CR
oc delete externalsecretsconfig cluster
```
