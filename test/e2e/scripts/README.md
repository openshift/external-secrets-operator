# E2E Test Setup Scripts

This directory contains helper scripts to simplify E2E test setup and execution.

## Prerequisites

- `kubectl` installed and configured
- Access to a Kubernetes cluster
- External Secrets Operator installed on the cluster
- `jq` installed (for GCP script)

## Scripts Overview

### 1. Setup Credentials

**`setup-aws-credentials.sh`**
- Sets up AWS credentials for E2E tests
- Creates `aws-creds` secret in `kube-system` namespace
- Prompts for Access Key ID and Secret Access Key

**`setup-gcp-credentials.sh`**
- Sets up GCP credentials for E2E tests
- Creates `gcp-creds` secret in `kube-system` namespace
- Requires path to service account JSON file

### 2. Verification

**`verify-setup.sh`**
- Checks all prerequisites for running E2E tests
- Verifies:
  - kubectl is installed
  - Cluster is accessible
  - Required namespaces exist
  - Operator pods are running
  - Credentials are configured
  - Environment variables are set

### 3. Running Tests

**`run-tests.sh`**
- Simplified test execution
- Supports filtering by provider
- Options for verbose output

### 4. Cleanup

**`cleanup-credentials.sh`**
- Removes test credentials from the cluster
- Can cleanup specific provider or all credentials

## Quick Start

### First Time Setup

```bash
# Navigate to scripts directory
cd test/e2e/scripts

# Make scripts executable
chmod +x *.sh

# Setup credentials for providers you want to test
./setup-aws-credentials.sh

# Verify everything is configured correctly
./verify-setup.sh

# Run tests
./run-tests.sh all
```

### Running Tests for Specific Providers

```bash
# Run AWS tests only
./run-tests.sh aws

# Run all tests with verbose output
./run-tests.sh all -v
```

### Cleanup After Testing

```bash
# Remove all credentials
./cleanup-credentials.sh all

# Remove AWS credentials
./cleanup-credentials.sh aws
```

## Detailed Usage

### AWS Setup

```bash
./setup-aws-credentials.sh
```

You'll be prompted for:
- AWS Access Key ID
- AWS Secret Access Key
- AWS Region (optional, defaults to ap-south-1)

**IAM Permissions Required:**
- `secretsmanager:CreateSecret`
- `secretsmanager:GetSecretValue`
- `secretsmanager:UpdateSecret`
- `secretsmanager:DeleteSecret`
- `ssm:PutParameter`
- `ssm:GetParameter`
- `ssm:DeleteParameter`

### Verification

```bash
./verify-setup.sh
```

This script performs comprehensive checks:

✅ **Pass criteria:**
- All components are properly configured
- Ready to run tests

⚠️ **Warning criteria:**
- Optional components missing (e.g., specific provider credentials)
- Tests for that provider will be skipped

❌ **Fail criteria:**
- Critical components missing (kubectl, cluster connection, operator)
- Cannot run any tests

### Running Tests

```bash
# Basic usage
./run-tests.sh [provider] [options]

# Examples
./run-tests.sh aws              # AWS tests only
./run-tests.sh all              # All tests
./run-tests.sh all -v           # All tests with verbose output
```

**Expected Test Counts:**
- AWS: ~13 tests
- Total: ~13 tests

### Cleanup

```bash
# Remove all provider credentials
./cleanup-credentials.sh all

# Remove AWS credentials
./cleanup-credentials.sh aws
```

## Environment Variables

Set these before running tests:

**AWS:**
```bash
export E2E_AWS_REGION=us-east-1  # Optional, defaults to ap-south-1
```

## Troubleshooting

### kubectl not found
Install kubectl: https://kubernetes.io/docs/tasks/tools/

### Cannot connect to cluster
```bash
# Check kubeconfig
kubectl config view

# Check cluster connection
kubectl cluster-info

# If using kind/minikube, ensure cluster is running
kind get clusters
minikube status
```

### Operator not installed
Install External Secrets Operator on your cluster before running tests.

### Credentials not working
1. Verify secrets are in `kube-system` namespace
2. Check secret data keys match expected names
3. Verify provider permissions are correctly configured

### Tests are skipped
- Check if provider credentials are set up
- Run `./verify-setup.sh` to see what's missing

### Tests fail
1. Check operator logs:
   ```bash
   kubectl logs -n external-secrets-operator -l app.kubernetes.io/name=external-secrets-operator
   ```
2. Check test namespace logs
3. See full test output with verbose flag: `./run-tests.sh all -v`

## Security Notes

- Credentials are stored as Kubernetes secrets in the cluster
- Scripts will overwrite existing credentials without warning
- Use `cleanup-credentials.sh` to remove credentials when done
- Never commit credentials to version control
- Consider using temporary/test-only credentials with minimal permissions

## CI/CD Integration

These scripts can be used in CI/CD pipelines:

```yaml
# Example GitHub Actions
- name: Setup AWS Credentials
  run: |
    echo "$AWS_ACCESS_KEY_ID" | ./test/e2e/scripts/setup-aws-credentials.sh
  env:
    AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
    AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}

- name: Run E2E Tests
  run: ./test/e2e/scripts/run-tests.sh aws -v
```

## Contributing

When adding new providers:
1. Create `setup-[provider]-credentials.sh` script
2. Update `verify-setup.sh` to check for new credentials
3. Update `run-tests.sh` to support the new provider
4. Update `cleanup-credentials.sh` to handle new provider
5. Update this README with usage instructions
