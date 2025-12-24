#!/bin/bash

# Cleanup E2E test credentials
# Usage: ./cleanup-credentials.sh [provider]

PROVIDER="${1:-all}"

echo "=========================================="
echo "E2E Credentials Cleanup"
echo "=========================================="
echo

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl not found"
    exit 1
fi

cleanup_secret() {
    local secret_name="$1"
    echo -n "Deleting $secret_name... "
    if kubectl delete secret "$secret_name" -n kube-system 2>/dev/null; then
        echo "✓ Deleted"
    else
        echo "✗ Not found"
    fi
}

case "$PROVIDER" in
    aws)
        cleanup_secret "aws-creds"
        ;;
    gcp)
        cleanup_secret "gcp-creds"
        ;;
    all)
        cleanup_secret "aws-creds"
        cleanup_secret "gcp-creds"
        ;;
    *)
        echo "Usage: ./cleanup-credentials.sh [provider]"
        echo
        echo "Providers:"
        echo "  aws    - Remove AWS credentials"
        echo "  gcp    - Remove GCP credentials"
        echo "  all    - Remove all credentials (default)"
        exit 1
        ;;
esac

echo
echo "=========================================="
echo "Cleanup complete!"
echo "=========================================="
