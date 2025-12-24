#!/bin/bash

# Verify E2E test setup
# Usage: ./verify-setup.sh

echo "=========================================="
echo "E2E Test Setup Verification"
echo "=========================================="
echo

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Track overall status
READY=true

echo "Checking prerequisites..."
echo

# 1. Check kubectl
echo -n "1. kubectl command: "
if command -v kubectl &> /dev/null; then
    echo -e "${GREEN}✓ Found${NC}"
else
    echo -e "${RED}✗ Not found${NC}"
    READY=false
fi

# 2. Check cluster connection
echo -n "2. Kubernetes cluster: "
if kubectl cluster-info &> /dev/null; then
    echo -e "${GREEN}✓ Connected${NC}"
    kubectl version --short 2>/dev/null | grep "Server Version" || true
else
    echo -e "${RED}✗ Not connected${NC}"
    READY=false
fi

echo

# 3. Check namespaces
echo "3. Required namespaces:"
for ns in external-secrets-operator external-secrets kube-system; do
    echo -n "   - $ns: "
    if kubectl get namespace $ns &> /dev/null; then
        echo -e "${GREEN}✓ Exists${NC}"
    else
        echo -e "${RED}✗ Not found${NC}"
        READY=false
    fi
done

echo

# 4. Check External Secrets Operator pods
echo "4. External Secrets Operator:"
echo -n "   - Operator pods: "
OPERATOR_PODS=$(kubectl get pods -n external-secrets-operator 2>/dev/null | grep -v NAME | wc -l | tr -d ' ')
if [ "$OPERATOR_PODS" -gt 0 ]; then
    echo -e "${GREEN}✓ Running ($OPERATOR_PODS pods)${NC}"
    kubectl get pods -n external-secrets-operator 2>/dev/null | grep -E "Running|NAME" || true
else
    echo -e "${RED}✗ No pods found${NC}"
    READY=false
fi

echo
echo -n "   - Operand pods: "
OPERAND_PODS=$(kubectl get pods -n external-secrets 2>/dev/null | grep -v NAME | wc -l | tr -d ' ')
if [ "$OPERAND_PODS" -gt 0 ]; then
    echo -e "${GREEN}✓ Running ($OPERAND_PODS pods)${NC}"
    kubectl get pods -n external-secrets 2>/dev/null | grep -E "Running|NAME" || true
else
    echo -e "${YELLOW}⚠ No pods found${NC}"
    echo "   (This may be normal if operand is not yet deployed)"
fi

echo

# 5. Check credentials
echo "5. Provider credentials (kube-system namespace):"

echo -n "   - aws-creds: "
if kubectl get secret aws-creds -n kube-system &> /dev/null; then
    echo -e "${GREEN}✓ Found${NC}"
    kubectl get secret aws-creds -n kube-system -o jsonpath='{.data}' | jq 'keys' 2>/dev/null || echo "     (Cannot display keys)"
else
    echo -e "${YELLOW}✗ Not found${NC}"
    echo "     Run: ./setup-aws-credentials.sh"
fi

echo -n "   - gcp-creds: "
if kubectl get secret gcp-creds -n kube-system &> /dev/null; then
    echo -e "${GREEN}✓ Found${NC}"
    kubectl get secret gcp-creds -n kube-system -o jsonpath='{.data}' | jq 'keys' 2>/dev/null || echo "     (Cannot display keys)"
else
    echo -e "${YELLOW}✗ Not found${NC}"
    echo "     Run: ./setup-gcp-credentials.sh"
fi

echo

# 6. Check environment variables
echo "6. Environment variables:"

echo -n "   - E2E_AWS_REGION: "
if [ -n "$E2E_AWS_REGION" ]; then
    echo -e "${GREEN}✓ Set ($E2E_AWS_REGION)${NC}"
else
    echo -e "${YELLOW}Not set (will use default: ap-south-1)${NC}"
fi

echo -n "   - E2E_GCP_PROJECT_ID: "
if [ -n "$E2E_GCP_PROJECT_ID" ]; then
    echo -e "${GREEN}✓ Set ($E2E_GCP_PROJECT_ID)${NC}"
else
    echo -e "${YELLOW}Not set (will extract from service account JSON)${NC}"
fi

echo
echo "=========================================="

# Final verdict
if [ "$READY" = true ]; then
    echo -e "${GREEN}✓ System is ready for E2E tests!${NC}"
    echo
    echo "You can run tests with:"
    echo "  make test-e2e E2E_GINKGO_LABEL_FILTER=\"Platform:AWS\""
    echo "  make test-e2e E2E_GINKGO_LABEL_FILTER=\"Platform:GCP\""
else
    echo -e "${RED}✗ System is NOT ready for E2E tests${NC}"
    echo
    echo "Please fix the issues above before running tests."
fi

echo "=========================================="
echo
