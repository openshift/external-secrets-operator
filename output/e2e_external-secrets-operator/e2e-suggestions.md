# E2E Test Suggestions: external-secrets-operator (Network Policy Feature)

## Detected Operator Structure
- **Framework**: controller-runtime (kubebuilder/operator-sdk)
- **Managed CRDs**: ExternalSecretsConfig, ExternalSecretsManager
- **E2E Pattern**: Ginkgo v2 with `Ordered` test suites, dynamic client, embed for testdata
- **Operator Namespace**: `external-secrets-operator`
- **Operand Namespace**: `external-secrets`
- **Install Mechanism**: OLM (bundle in `bundle/manifests/`)

## Changes Detected in Diff (origin/ai-staging-release-1.1...HEAD)

| File | Category | Changes |
|------|----------|---------|
| `api/v1alpha1/tests/.../externalsecretsconfig.testsuite.yaml` | API Tests | 19 new integration tests for `networkPolicies` field validation |
| `pkg/controller/external_secrets/utils.go` | Controller | Added runtime validation warning for BitwardenSDKServer misconfiguration |
| `pkg/controller/external_secrets/utils_test.go` | Unit Tests | 6 new unit tests for `validateExternalSecretsConfig` |

## Highly Recommended E2E Scenarios

### 1. Static Network Policy Verification (Priority: HIGH)
- **Why**: The core feature — verify that deny-all and component-specific allow policies are created
- **Tests**: TC-1, TC-2 in generated test code (Context: "Static Network Policies")

### 2. Custom Network Policy CRUD (Priority: HIGH)
- **Why**: Validates the user-facing API for custom network policies
- **Tests**: TC-3, TC-4 in generated test code (Context: "Custom Network Policies")

### 3. Webhook Accessibility (Priority: HIGH)
- **Why**: Network policies must not break webhook admission control, which is critical for operator functionality
- **Tests**: TC-7 in generated test code (Context: "Webhook Accessibility")

### 4. ExternalSecretsConfig Ready Condition (Priority: HIGH)
- **Why**: Ensures the operator reports correct status after network policy reconciliation
- **Tests**: TC-9 in generated test code (Context: "ExternalSecretsConfig Status")

### 5. BitwardenSDKServer Misconfiguration Warning (Priority: MEDIUM)
- **Why**: Directly tests the new controller enhancement added in this diff
- **Tests**: TC-8 in generated test code (Context: "Network Policy Misconfiguration Warning")

## Optional / Nice-to-Have Scenarios

### 6. Operator Namespace Network Policy (Priority: LOW)
- **Why**: The operator namespace policy is shipped via OLM bundle, not controller-managed
- **Notes**: May already exist from OLM installation

### 7. Network Connectivity Validation (Priority: LOW)
- **Why**: Validates actual network connectivity rather than just policy existence
- **Notes**: Requires exec into pods and network probing; may be flaky in CI

### 8. Bitwarden Network Policy with Plugin Enabled (Priority: LOW)
- **Why**: Requires Bitwarden plugin to be configured with valid TLS certs
- **Notes**: Complex setup; better suited for integration rather than e2e

## Gaps / Hard to Test Automatically

1. **Actual traffic blocking**: Verifying that unauthorized traffic is actually blocked requires creating a test pod and attempting network connections to operand pods, which is fragile in CI environments
2. **Metrics endpoint accessibility**: Verifying that Prometheus can scrape metrics through the network policies requires the monitoring stack to be fully operational
3. **Downgrade scenario**: The EP notes that orphaned policies on downgrade are a known issue; testing this requires version manipulation which is complex in e2e
