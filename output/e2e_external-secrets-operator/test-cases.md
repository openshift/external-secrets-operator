# E2E Test Cases: external-secrets-operator

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
- External Secrets Operator installed via OLM or deployment

## Changes Summary
The diff introduces NetworkPolicy API improvements to `ExternalSecretsConfig`:
1. DNS subdomain pattern validation on `networkPolicies[].name` field
2. Fixed `egress` JSON tag (removed `omitempty` for required field consistency)
3. Improved godoc for `ComponentName` constants and `egress` field
4. CRD schema updated with new `pattern` validation

## Test Cases

### TC-1: Create ExternalSecretsConfig with Custom NetworkPolicy for CoreController
- **Test**: Verify that creating an ExternalSecretsConfig with a custom network policy targeting ExternalSecretsCoreController creates a corresponding NetworkPolicy resource.
- **Steps**:
  1. Create ExternalSecretsConfig with `spec.controllerConfig.networkPolicies` containing a policy for `ExternalSecretsCoreController`
  2. Wait for Ready condition
  3. Verify NetworkPolicy exists in operand namespace with correct podSelector (`app.kubernetes.io/name: external-secrets`)
  4. Verify egress rules match the configured rules
- **Expected**: NetworkPolicy created with correct podSelector and egress rules

### TC-2: Create ExternalSecretsConfig with Custom NetworkPolicy for BitwardenSDKServer
- **Test**: Verify that a custom network policy targeting BitwardenSDKServer creates a NetworkPolicy with the correct pod selector.
- **Steps**:
  1. Create ExternalSecretsConfig with a network policy for `BitwardenSDKServer`
  2. Verify NetworkPolicy exists with podSelector `app.kubernetes.io/name: bitwarden-sdk-server`
- **Expected**: NetworkPolicy created targeting bitwarden-sdk-server pods

### TC-3: Validate NetworkPolicy Name DNS Subdomain Pattern
- **Test**: Verify that the `name` field in networkPolicies enforces DNS subdomain naming (lowercase alphanumeric and hyphens, no leading/trailing hyphens).
- **Steps**:
  1. Attempt to create ExternalSecretsConfig with `networkPolicies[].name` = "Allow-Egress" (uppercase)
  2. Attempt with name = "allow_egress" (underscore)
  3. Attempt with name = "-allow-egress" (leading hyphen)
  4. Attempt with name = "allow-egress-" (trailing hyphen)
- **Expected**: All four should be rejected with validation error

### TC-4: Valid DNS Subdomain Names Accepted
- **Test**: Verify that valid DNS subdomain names are accepted for networkPolicies[].name.
- **Steps**:
  1. Create ExternalSecretsConfig with name = "allow-core-egress"
  2. Create with name = "a1b2c3"
  3. Create with name = "x"
- **Expected**: All valid names accepted without error

### TC-5: NetworkPolicy with Empty Egress (Deny-All)
- **Test**: Verify that a NetworkPolicy with an empty egress list creates a deny-all egress policy.
- **Steps**:
  1. Create ExternalSecretsConfig with `egress: []`
  2. Verify NetworkPolicy created with empty egress rules
- **Expected**: NetworkPolicy exists with policyTypes: [Egress] and no egress rules (deny-all)

### TC-6: Update Egress Rules in Existing NetworkPolicy
- **Test**: Verify that updating egress rules in an existing networkPolicy is allowed (name and componentName are immutable, but egress is mutable).
- **Steps**:
  1. Create ExternalSecretsConfig with a network policy allowing egress on port 6443
  2. Update the same policy to also allow egress on port 443
  3. Verify the NetworkPolicy resource is updated
- **Expected**: NetworkPolicy updated with new egress rules

### TC-7: Immutability of Name and ComponentName
- **Test**: Verify that name and componentName fields in networkPolicies are immutable once set.
- **Steps**:
  1. Create ExternalSecretsConfig with a network policy
  2. Attempt to change the name or componentName
- **Expected**: Update rejected with immutability error

### TC-8: Multiple NetworkPolicies
- **Test**: Verify that multiple custom network policies can be configured simultaneously.
- **Steps**:
  1. Create ExternalSecretsConfig with two network policies (one for CoreController, one for BitwardenSDKServer)
  2. Verify both NetworkPolicy resources exist
- **Expected**: Both NetworkPolicies created with correct selectors

### TC-9: Static NetworkPolicies Exist
- **Test**: Verify that the operator creates static deny-all, DNS, and component-specific network policies.
- **Steps**:
  1. Deploy ExternalSecretsConfig with default spec
  2. List NetworkPolicies in operand namespace
  3. Verify deny-all, allow-dns, allow-main-controller, allow-webhook policies exist
- **Expected**: Static policies present, deny-all selects all pods

### TC-10: Invalid ComponentName Rejected
- **Test**: Verify that an invalid componentName value is rejected at admission.
- **Steps**:
  1. Attempt to create ExternalSecretsConfig with componentName = "InvalidComponent"
- **Expected**: Rejected with "Unsupported value" error

## Verification
```bash
oc get externalsecretsconfig cluster -o yaml
oc get networkpolicies -n external-secrets
oc describe networkpolicies -n external-secrets
oc get pods -n external-secrets-operator
oc logs -n external-secrets-operator -l app.kubernetes.io/name=external-secrets-operator --tail=50
```

## Cleanup
```bash
oc delete externalsecretsconfig cluster --ignore-not-found
oc delete namespace external-secrets --ignore-not-found
```
