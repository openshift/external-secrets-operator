# E2E Test Cases: external-secrets-operator (Network Policy Feature)

## Operator Information
- **Repository**: github.com/openshift/external-secrets-operator
- **Framework**: controller-runtime
- **API Group**: operator.openshift.io/v1alpha1
- **Managed CRDs**: ExternalSecretsConfig, ExternalSecretsManager
- **Operator Namespace**: external-secrets-operator
- **Operand Namespace**: external-secrets
- **Changes Analyzed**: git diff origin/ai-staging-release-1.1...HEAD

## Prerequisites
- OpenShift cluster with admin access
- `oc` CLI installed and authenticated
- Network policy support enabled in the cluster (OVN-Kubernetes or OpenShift SDN)

## Installation
The operator is installed via OLM. Assuming it is already installed in the `external-secrets-operator` namespace.

## Changes Summary
1. **API Integration Tests**: Added 19 test cases for `networkPolicies` field validation in `ExternalSecretsConfig`
2. **Controller Enhancement**: Added runtime validation warning when `BitwardenSDKServer` network policies are configured but the Bitwarden plugin is not enabled
3. **Unit Tests**: Added 6 test cases for `validateExternalSecretsConfig`

## Test Cases

### TC-1: Verify default-deny network policies are created on operand deployment
- **Test**: After creating ExternalSecretsConfig, verify the deny-all NetworkPolicy is created in the operand namespace
- **Steps**:
  1. Create ExternalSecretsConfig CR
  2. Wait for operand pods to be ready
  3. Verify `deny-all-traffic` NetworkPolicy exists in `external-secrets` namespace
- **Expected**: `deny-all-traffic` NetworkPolicy exists with `podSelector: {}` and policy types `Ingress` and `Egress`

### TC-2: Verify static allow network policies are created for all components
- **Test**: Verify that component-specific allow policies are created
- **Steps**:
  1. List all NetworkPolicies in `external-secrets` namespace
  2. Verify each expected static policy exists
- **Expected**: The following NetworkPolicies exist:
  - `deny-all-traffic`
  - `allow-api-server-egress-for-main-controller-traffic` (always)
  - `allow-api-server-egress-for-webhook` (always)
  - `allow-api-server-egress-for-cert-controller` (when cert-manager is NOT enabled)
  - `allow-to-dns` (always)

### TC-3: Verify custom network policy is created from ExternalSecretsConfig spec
- **Test**: Configure a custom network policy via the API and verify it's created
- **Steps**:
  1. Update ExternalSecretsConfig with a custom network policy for CoreController
  2. Wait for reconciliation
  3. Verify the custom NetworkPolicy exists in the operand namespace
- **Expected**: Custom NetworkPolicy exists with correct pod selector (`app.kubernetes.io/name: external-secrets`) and egress rules

### TC-4: Verify custom network policy egress rules are updated on spec change
- **Test**: Update egress rules in an existing custom network policy and verify the change is applied
- **Steps**:
  1. Create ExternalSecretsConfig with a custom network policy
  2. Update the egress rules
  3. Verify the NetworkPolicy is updated in the cluster
- **Expected**: NetworkPolicy spec reflects the updated egress rules

### TC-5: Verify operator namespace has its own network policy
- **Test**: Verify that the operator namespace has a network policy for the operator pod
- **Steps**:
  1. List NetworkPolicies in `external-secrets-operator` namespace
  2. Verify `allow-network-traffic` policy exists
- **Expected**: NetworkPolicy exists allowing API server egress (port 6443) and metrics ingress (ports 8443, 8080)

### TC-6: Verify operand pods can reach the API server through network policies
- **Test**: Verify that core controller pod can communicate with the API server
- **Steps**:
  1. Exec into core controller pod
  2. Attempt to reach the API server on port 6443
- **Expected**: Connection succeeds

### TC-7: Verify webhook is accessible through network policies
- **Test**: Verify the webhook admission endpoint is functional
- **Steps**:
  1. Create a SecretStore or ExternalSecret resource (triggers webhook)
  2. Verify the admission webhook responds
- **Expected**: Webhook validation succeeds (resource is created or properly rejected)

### TC-8: Verify BitwardenSDKServer network policy warning event
- **Test**: Configure a BitwardenSDKServer network policy without enabling the Bitwarden plugin
- **Steps**:
  1. Create ExternalSecretsConfig with a BitwardenSDKServer network policy but without enabling Bitwarden
  2. Check events on the ExternalSecretsConfig resource
- **Expected**: A Warning event with reason `NetworkPolicyMisconfiguration` is recorded

### TC-9: Verify ExternalSecretsConfig Ready condition after network policy creation
- **Test**: Verify the ExternalSecretsConfig becomes Ready after all network policies are created
- **Steps**:
  1. Create ExternalSecretsConfig with custom network policies
  2. Wait for Ready condition
- **Expected**: ExternalSecretsConfig has `Ready=True` condition

## Verification
```bash
oc get networkpolicies -n external-secrets
oc get networkpolicies -n external-secrets-operator
oc get externalsecretsconfig cluster -o jsonpath='{.status.conditions}'
oc get events -n external-secrets-operator --field-selector reason=NetworkPolicyMisconfiguration
```

## Cleanup
```bash
oc delete externalsecretsconfig cluster --ignore-not-found
# Wait for operand namespace cleanup
oc get networkpolicies -n external-secrets
```
