# E2E Test Suggestions: external-secrets-operator

## Detected Operator Structure
- **Framework**: controller-runtime (kubebuilder v4 / operator-sdk)
- **Managed CRDs**: ExternalSecretsConfig (cluster-scoped singleton), ExternalSecretsManager
- **E2E Pattern**: Ginkgo v2 with dynamic client, build tag `e2e`
- **Operator Namespace**: `external-secrets-operator`
- **Operand Namespace**: `external-secrets`

## Changes Detected in Diff

| File | Change | Impact |
|------|--------|--------|
| `api/v1alpha1/external_secrets_config_types.go` | Added DNS pattern validation on `name` field; fixed `egress` JSON tag | API validation changes |
| `config/crd/bases/operator.openshift.io_externalsecretsconfigs.yaml` | CRD schema updated with `pattern` field | Admission validation |
| `api/v1alpha1/tests/.../externalsecretsconfig.testsuite.yaml` | Added test cases for DNS pattern, empty egress, egress updates | Integration test coverage |

## Highly Recommended E2E Scenarios

### 1. Custom NetworkPolicy Lifecycle (Critical)
**Reason**: The NetworkPolicy feature is the core of EP #1834. End-to-end validation that the operator correctly creates, updates, and deletes Kubernetes NetworkPolicy resources based on the API spec is essential.

**Tests**:
- Create ExternalSecretsConfig with custom networkPolicies -> verify NetworkPolicy resources exist
- Update egress rules -> verify NetworkPolicy updated
- Remove policies from spec -> verify stale policies deleted

### 2. DNS Name Pattern Validation (High)
**Reason**: New pattern validation was added (`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`). E2E tests verify the CRD admission webhook correctly rejects invalid names on a real cluster.

**Tests**:
- Uppercase names rejected
- Underscore names rejected
- Leading/trailing hyphen names rejected
- Valid names accepted

### 3. Static NetworkPolicy Verification (High)
**Reason**: The operator creates baseline deny-all and allow policies for security. Verifying these exist ensures the security posture is correct.

**Tests**:
- deny-all-traffic policy exists and matches all pods
- allow-to-dns policy exists
- Component-specific allow policies exist

### 4. Empty Egress Deny-All Behavior (Medium)
**Reason**: The `egress` field was changed to remove `omitempty`, enabling explicit empty-list semantics for deny-all egress per component.

**Tests**:
- Create policy with `egress: []` -> verify NetworkPolicy has Egress policyType but no egress rules

## Optional/Nice-to-Have Scenarios

### 5. Component Pod Selector Mapping (Low)
**Reason**: Verify that `ExternalSecretsCoreController` maps to `app.kubernetes.io/name: external-secrets` and `BitwardenSDKServer` maps to `bitwarden-sdk-server`.

### 6. Multiple NetworkPolicies (Low)
**Reason**: Verify the operator handles multiple custom policies simultaneously.

### 7. Connectivity Verification (Low, requires workload)
**Reason**: Actually test that network policies block/allow traffic as expected. This requires deploying test pods and attempting connections, which is complex to automate.

## Gaps / Hard to Test Automatically

1. **Bitwarden SDK Server scenarios**: Require Bitwarden plugin to be enabled, which needs additional infrastructure
2. **Cert-manager conditional policies**: Require cert-manager to be installed/uninstalled, which is disruptive
3. **Actual network connectivity testing**: Requires deploying test pods to verify that traffic is blocked/allowed per policy rules
4. **DNS resolution verification**: Hard to test that the DNS policy actually allows name resolution without deploying workloads
