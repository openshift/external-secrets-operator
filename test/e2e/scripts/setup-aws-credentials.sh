#!/bin/bash
set -e

# Setup AWS credentials for E2E tests
# Usage: ./setup-aws-credentials.sh

echo "=========================================="
echo "AWS Credentials Setup for E2E Tests"
echo "=========================================="
echo

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ Error: kubectl not found. Please install kubectl first."
    exit 1
fi

# Check if cluster is accessible
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ Error: Cannot connect to Kubernetes cluster."
    echo "Please ensure your kubeconfig is set correctly."
    exit 1
fi

echo "✓ Kubernetes cluster is accessible"
echo

# Prompt for AWS credentials
read -p "Enter AWS Access Key ID: " AWS_ACCESS_KEY_ID
read -sp "Enter AWS Secret Access Key: " AWS_SECRET_ACCESS_KEY
echo
echo

# Optional: AWS Region
read -p "Enter AWS Region [default: ap-south-1]: " AWS_REGION
AWS_REGION=${AWS_REGION:-ap-south-1}

# Verify credentials are not empty
if [ -z "$AWS_ACCESS_KEY_ID" ] || [ -z "$AWS_SECRET_ACCESS_KEY" ]; then
    echo "❌ Error: AWS credentials cannot be empty"
    exit 1
fi

echo "Creating aws-creds secret in kube-system namespace..."

# Delete existing secret if it exists
kubectl delete secret aws-creds -n kube-system 2>/dev/null || true

# Create the secret
kubectl create secret generic aws-creds \
    --from-literal=aws_access_key_id="$AWS_ACCESS_KEY_ID" \
    --from-literal=aws_secret_access_key="$AWS_SECRET_ACCESS_KEY" \
    -n kube-system

echo "✓ AWS credentials secret created successfully"
echo

# Set environment variable for region
echo "Note: Set the following environment variable before running tests:"
echo "  export E2E_AWS_REGION=$AWS_REGION"
echo

echo "=========================================="
echo "AWS Credentials Setup Complete!"
echo "=========================================="
echo
echo "You can now run AWS e2e tests with:"
echo "  make test-e2e E2E_GINKGO_LABEL_FILTER=\"Platform:AWS\""
echo
