#!/bin/bash
# Populate or cleanup SecretStores in stress-test namespaces
# Usage: 
#   ./populate-test-secretstores.sh          # Create SecretStores
#   ./populate-test-secretstores.sh cleanup  # Delete SecretStores

set -e

KUBECONFIG="${KUBECONFIG:-/home/mykastur/gcp_n/install-dir/auth/kubeconfig}"
export KUBECONFIG
NAMESPACE_PREFIX="${NAMESPACE_PREFIX:-stress-test}"
SECRETSTORES_PER_NS="${SECRETSTORES_PER_NS:-100}"

# Parse arguments
MODE="${1:-populate}"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

print_step() {
    echo -e "${BLUE}==>${NC} $1"
}

print_success() {
    echo -e "${GREEN}✅${NC} $1"
}

print_error() {
    echo -e "${RED}❌${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠️${NC} $1"
}

print_header() {
    echo -e "${CYAN}==========================================${NC}"
    echo -e "${CYAN}$1${NC}"
    echo -e "${CYAN}==========================================${NC}"
}

# Show help
if [ "$MODE" = "--help" ] || [ "$MODE" = "-h" ]; then
    echo "Usage: $0 [MODE]"
    echo ""
    echo "Modes:"
    echo "  populate (default)  Create SecretStores in test namespaces"
    echo "  cleanup            Delete all SecretStores from test namespaces"
    echo "  --help, -h         Show this help"
    echo ""
    echo "Environment Variables:"
    echo "  NAMESPACE_PREFIX        Namespace prefix (default: stress-test)"
    echo "  SECRETSTORES_PER_NS     SecretStores per namespace (default: 100)"
    echo ""
    echo "Examples:"
    echo "  $0                      # Create SecretStores"
    echo "  $0 cleanup              # Delete SecretStores"
    echo "  NAMESPACE_PREFIX=quick-test $0 cleanup"
    echo ""
    exit 0
fi

# Validate mode
if [ "$MODE" != "populate" ] && [ "$MODE" != "cleanup" ]; then
    print_error "Invalid mode: $MODE"
    echo "Use: $0 [populate|cleanup|--help]"
    exit 1
fi

# Find existing stress-test namespaces
NAMESPACES=$(oc get ns | grep "^${NAMESPACE_PREFIX}-" | awk '{print $1}' | sort)
NUM_NS=$(echo "$NAMESPACES" | wc -l)

if [ -z "$NAMESPACES" ] || [ "$NUM_NS" -eq 0 ]; then
    print_error "No ${NAMESPACE_PREFIX}-* namespaces found"
    exit 1
fi

# Cleanup mode
if [ "$MODE" = "cleanup" ]; then
    print_header "Cleanup SecretStores from Test Namespaces"
    echo ""
    echo "Found $NUM_NS namespaces matching ${NAMESPACE_PREFIX}-*"
    echo "Will delete all SecretStores from these namespaces"
    echo ""
    
    # Count existing SecretStores
    print_step "Counting existing SecretStores..."
    BEFORE_COUNT=$(oc get secretstores --all-namespaces --no-headers 2>/dev/null | grep "^${NAMESPACE_PREFIX}-" | wc -l)
    echo "  Found $BEFORE_COUNT SecretStores in test namespaces"
    
    if [ "$BEFORE_COUNT" -eq 0 ]; then
        print_warning "No SecretStores found in test namespaces"
        exit 0
    fi
    
    echo ""
    print_warning "This will delete $BEFORE_COUNT SecretStores!"
    echo "Press Ctrl+C within 5 seconds to cancel..."
    sleep 5
    
    print_step "Deleting SecretStores..."
    START_TIME=$(date +%s)
    
    DELETED=0
    for NS in $NAMESPACES; do
        # Delete all SecretStores in this namespace
        oc delete secretstores --all -n "$NS" --timeout=30s &>/dev/null &
        
        # Count how many we deleted
        NS_COUNT=$(oc get secretstores -n "$NS" --no-headers 2>/dev/null | wc -l)
        DELETED=$((DELETED + NS_COUNT))
        
        # Limit concurrent deletes
        if [ $((DELETED % 50)) -eq 0 ]; then
            wait
            echo -n "."
        fi
        
        # Progress every 10 namespaces
        NUM_PROCESSED=$(echo "$NAMESPACES" | grep -n "^${NS}$" | cut -d':' -f1)
        if [ $((NUM_PROCESSED % 10)) -eq 0 ]; then
            ELAPSED=$(($(date +%s) - START_TIME))
            PCT=$((NUM_PROCESSED * 100 / NUM_NS))
            echo ""
            print_step "Progress: $NUM_PROCESSED/$NUM_NS namespaces processed (${PCT}%), ${ELAPSED}s elapsed"
        fi
    done
    
    # Wait for all deletions
    wait
    
    echo ""
    ELAPSED=$(($(date +%s) - START_TIME))
    print_success "Deletion commands completed in ${ELAPSED}s"
    
    # Wait for resources to be fully deleted
    print_step "Waiting for resources to be fully deleted..."
    sleep 5
    
    # Verify cleanup
    AFTER_COUNT=$(oc get secretstores --all-namespaces --no-headers 2>/dev/null | grep "^${NAMESPACE_PREFIX}-" | wc -l)
    TOTAL_COUNT=$(oc get secretstores --all-namespaces --no-headers 2>/dev/null | wc -l)
    
    echo ""
    print_success "Cleanup complete!"
    print_success "SecretStores in test namespaces: $BEFORE_COUNT → $AFTER_COUNT"
    print_success "Total SecretStores in cluster: $TOTAL_COUNT"
    
    if [ "$AFTER_COUNT" -gt 0 ]; then
        echo ""
        print_warning "$AFTER_COUNT SecretStores still exist (may be stuck deleting)"
        echo "To force cleanup, run:"
        echo "  for ns in \$(oc get ns | grep '^${NAMESPACE_PREFIX}-' | awk '{print \$1}'); do"
        echo "    oc delete secretstores --all -n \$ns --grace-period=0 --force"
        echo "  done"
    fi
    
    echo ""
    exit 0
fi

# Populate mode
print_header "Populate Test Namespaces with SecretStores"
echo ""
echo "Found $NUM_NS namespaces matching ${NAMESPACE_PREFIX}-*"
echo "Will create $SECRETSTORES_PER_NS SecretStores in each"
echo "Total: $((NUM_NS * SECRETSTORES_PER_NS)) SecretStores"
echo ""

# Verify SecretStore CRD exists
print_step "Verifying SecretStore CRD..."
if ! oc get crd secretstores.external-secrets.io &>/dev/null; then
    print_error "SecretStore CRD not found!"
    exit 1
fi
print_success "SecretStore CRD found"

# Get the correct API version
SECRETSTORE_VERSION=$(oc api-resources | grep "^secretstores " | awk '{print $3}' | cut -d'/' -f2)
if [ -z "$SECRETSTORE_VERSION" ]; then
    SECRETSTORE_VERSION="v1"
fi
print_success "Using API version: $SECRETSTORE_VERSION"

print_step "Creating SecretStores..."
START_TIME=$(date +%s)

CREATED=0
TOTAL=$((NUM_NS * SECRETSTORES_PER_NS))

for NS in $NAMESPACES; do
    for j in $(seq 1 $SECRETSTORES_PER_NS); do
        cat <<EOF | oc apply -f - &>/dev/null &
apiVersion: external-secrets.io/${SECRETSTORE_VERSION}
kind: SecretStore
metadata:
  name: aws-store-${j}
  namespace: ${NS}
spec:
  provider:
    aws:
      service: SecretsManager
      region: us-east-1
      auth:
        secretRef:
          accessKeyIDSecretRef:
            name: aws-secret
            key: access-key
          secretAccessKeySecretRef:
            name: aws-secret
            key: secret-key
EOF
        CREATED=$((CREATED + 1))
        
        # Limit concurrent creates
        if [ $((CREATED % 50)) -eq 0 ]; then
            wait
            echo -n "."
        fi
    done
    
    # Progress every 10 namespaces
    if [ $(((CREATED / SECRETSTORES_PER_NS) % 10)) -eq 0 ]; then
        ELAPSED=$(($(date +%s) - START_TIME))
        PCT=$((CREATED * 100 / TOTAL))
        echo ""
        print_step "Progress: $CREATED/$TOTAL SecretStores (${PCT}%), ${ELAPSED}s elapsed"
    fi
done

# Wait for all background jobs
wait

echo ""
ELAPSED=$(($(date +%s) - START_TIME))
print_success "Created $CREATED SecretStores in ${ELAPSED}s"

# Verify
sleep 3
ACTUAL_COUNT=$(oc get secretstores --all-namespaces --no-headers 2>/dev/null | wc -l)
print_success "Verified: $ACTUAL_COUNT SecretStores exist in cluster"

echo ""
echo "Done! You can now continue with the stress test steps:"
echo "  1. Check webhook status: ./analyze-webhook-performance.sh"
echo "  2. Test disable attempt (should be denied):"
echo "     oc patch externalsecretsconfig cluster --type=merge \\"
echo "       -p '{\"spec\":{\"plugins\":{\"bitwardenSecretManagerProvider\":{\"mode\":\"Disabled\"}}}}'"
echo ""
echo "To cleanup later, run:"
echo "  $0 cleanup"
echo ""

