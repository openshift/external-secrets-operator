# External Secrets Operator E2E Tests

This directory contains end-to-end tests for the External Secrets Operator.

## Overview

The e2e test suite validates the External Secrets Operator's functionality with AWS secret providers:

- **AWS** (13 test cases) - Secrets Manager and Parameter Store

**Total: 13 comprehensive test cases**

## Test Structure

```
test/e2e/
├── e2e_suite_test.go           # Test suite setup (existing)
├── e2e_test.go                 # Original AWS test (existing)
├── e2e_aws_test.go             # Extended AWS tests (NEW)
└── testdata/                   # YAML test data files
    └── aws_*.yaml              # AWS test configurations

test/utils/
├── conditions.go               # Test utilities (extended with AWS helpers)
├── dynamic_resources.go        # Resource loading utilities (existing)
└── ...
```

## Prerequisites

### 1. Kubernetes Cluster

You need a running Kubernetes cluster with:
- External Secrets Operator installed
- Operator pods running in `external-secrets-operator` namespace
- Operand pods running in `external-secrets` namespace

### 2. Provider Credentials

All provider credentials must be stored as Kubernetes secrets in the `kube-system` namespace:

#### AWS Credentials

```bash
kubectl create secret generic aws-creds \
  --from-literal=aws_access_key_id=<YOUR_ACCESS_KEY_ID> \
  --from-literal=aws_secret_access_key=<YOUR_SECRET_ACCESS_KEY> \
  -n kube-system
```

**Required IAM Permissions:**
- `secretsmanager:CreateSecret`
- `secretsmanager:GetSecretValue`
- `secretsmanager:UpdateSecret`
- `secretsmanager:DeleteSecret`
- `ssm:PutParameter`
- `ssm:GetParameter`
- `ssm:DeleteParameter`

## Running the Tests

### Run All E2E Tests

```bash
make test-e2e
```

### Run Tests by Provider

```bash
# Run AWS tests
make test-e2e E2E_GINKGO_LABEL_FILTER="Platform:AWS"
```

### Run Without Make

```bash
# Set the region (default: ap-south-1)
export E2E_AWS_REGION=us-east-1

# Run tests directly
go test -v -tags=e2e ./test/e2e/... -ginkgo.label-filter="Platform:AWS"
```

## Test Coverage

### AWS Tests (e2e_aws_test.go)

#### Basic Operations (3 tests)
1. **Namespace-scoped SecretStore** - Tests non-cluster scoped SecretStore
2. **Multiple data keys** - Fetches multiple keys from a single AWS secret
3. **Binary data handling** - Tests base64-encoded binary data

#### Advanced Features (4 tests)
4. **Secret rotation** - Tests automatic refresh with 30s interval
5. **Template transformation** - Tests Go template engine
6. **dataFrom** - Fetches entire secret without explicit mapping
7. **JSON path extraction** - Extracts nested values from complex JSON

#### AWS Parameter Store (1 test)
8. **SSM Parameter Store basic** - Tests Systems Manager Parameter Store integration

#### Error Scenarios (2 tests)
9. **Non-existent secret** - Verifies error handling for missing secrets
10. **Invalid credentials** - Tests SecretStore with bad credentials

**Total: 11 new AWS tests** (plus 1 existing test = 12 total)

## Test Pattern

All tests follow this pattern:

```go
It("should do something", func() {
    // 1. Create remote AWS secret

    // 2. Create SecretStore/ClusterSecretStore

    // 3. Wait for SecretStore to become Ready

    // 4. Create ExternalSecret/PushSecret

    // 5. Wait for resource to become Ready

    // 6. Verify K8s secret contains expected data

    // 7. Cleanup (via defer)
})
```

## Troubleshooting

### Tests Skip with "credentials not found"

Make sure credentials are created in the `kube-system` namespace:
- `aws-creds`

Check credentials:
```bash
kubectl get secret aws-creds -n kube-system
```

### "ClusterSecretStore not becoming Ready"

Check operator logs:
```bash
# Operator logs
kubectl logs -n external-secrets-operator -l app.kubernetes.io/name=external-secrets-operator

# Operand logs
kubectl logs -n external-secrets -l app.kubernetes.io/name=external-secrets
```

### "Secret not created in K8s"

Check ExternalSecret status:
```bash
kubectl describe externalsecret <name> -n external-secrets
```

Look for error conditions in the status.

### Tests taking too long

- AWS API calls can be slow
- Default timeout is 2 minutes per resource
- Ensure good network connectivity to AWS

## Cleanup

Tests automatically cleanup resources using defer blocks. If tests are interrupted, you may need to manually cleanup:

### AWS
```bash
# List secrets
aws secretsmanager list-secrets --query "SecretList[?starts_with(Name, 'eso-e2e-')]"

# Delete secret
aws secretsmanager delete-secret --secret-id <name> --force-delete-without-recovery
```

## CI/CD Integration

Example GitHub Actions workflow:

```yaml
name: E2E Tests

on: [push, pull_request]

jobs:
  e2e-aws:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Create kind cluster
        run: |
          kind create cluster
          kubectl cluster-info

      - name: Install External Secrets Operator
        run: |
          # Install operator

      - name: Create AWS credentials
        run: |
          kubectl create secret generic aws-creds \
            --from-literal=aws_access_key_id=${{ secrets.AWS_ACCESS_KEY_ID }} \
            --from-literal=aws_secret_access_key=${{ secrets.AWS_SECRET_ACCESS_KEY }} \
            -n kube-system

      - name: Run AWS E2E tests
        run: make test-e2e E2E_GINKGO_LABEL_FILTER="Platform:AWS"
```

## Contributing

When adding new test cases:

1. Follow the existing pattern in test files
2. Use descriptive test names
3. Add appropriate labels (e.g., `Label("Platform:AWS")`)
4. Create test data YAML files in `testdata/`
5. Use pattern replacement for dynamic values
6. Always cleanup resources in defer blocks
7. Use `Eventually()` for async assertions
8. Test both success and error scenarios

## Resources

- [External Secrets Operator Documentation](https://external-secrets.io/)
- [Ginkgo Testing Framework](https://onsi.github.io/ginkgo/)
- [Gomega Matcher Library](https://onsi.github.io/gomega/)
