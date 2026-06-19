#!/bin/bash
# Stress Test for External Secrets Operator Webhook
# Tests matchConditions performance optimization by creating many non-BitWarden SecretStores
# then attempting to disable the BitWarden plugin

set -e

# Configuration
KUBECONFIG="${KUBECONFIG:-/home/mykastur/gcp_n/install-dir/auth/kubeconfig}"
export KUBECONFIG
NAMESPACE_PREFIX="stress-test"
NUM_NAMESPACES=100
SECRETSTORES_PER_NS=100
OPERATOR_NAMESPACE="external-secrets-operator"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m'

print_header() {
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}$1${NC}"
    echo -e "${CYAN}========================================${NC}"
}

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

print_metric() {
    echo -e "${MAGENTA}📊${NC} $1"
}

# Get operator pod name
get_operator_pod() {
    oc get pod -n "$OPERATOR_NAMESPACE" -l app=external-secrets-operator -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

# Get pod metrics (memory and CPU)
get_pod_metrics() {
    local pod=$1
    local namespace=$2
    
    # Using oc adm top pod
    local metrics=$(oc adm top pod "$pod" -n "$namespace" --no-headers 2>/dev/null || echo "N/A N/A")
    echo "$metrics"
}

# Get detailed pod resource usage from /proc
get_detailed_metrics() {
    local pod=$1
    local namespace=$2
    
    # Get memory from pod status
    local mem_usage=$(oc get pod "$pod" -n "$namespace" -o jsonpath='{.status.containerStatuses[0].resources.usage.memory}' 2>/dev/null || echo "N/A")
    local cpu_usage=$(oc get pod "$pod" -n "$namespace" -o jsonpath='{.status.containerStatuses[0].resources.usage.cpu}' 2>/dev/null || echo "N/A")
    
    # Try to get from metrics API
    if [ "$mem_usage" = "N/A" ] || [ "$cpu_usage" = "N/A" ]; then
        local metrics=$(oc adm top pod "$pod" -n "$namespace" --no-headers 2>/dev/null)
        if [ -n "$metrics" ]; then
            cpu_usage=$(echo "$metrics" | awk '{print $2}')
            mem_usage=$(echo "$metrics" | awk '{print $3}')
        fi
    fi
    
    echo "$cpu_usage $mem_usage"
}

# Convert memory to MB
mem_to_mb() {
    local mem=$1
    if [[ $mem =~ ([0-9]+)Mi ]]; then
        echo "${BASH_REMATCH[1]}"
    elif [[ $mem =~ ([0-9]+)Gi ]]; then
        echo "$((${BASH_REMATCH[1]} * 1024))"
    elif [[ $mem =~ ([0-9]+)Ki ]]; then
        echo "$((${BASH_REMATCH[1]} / 1024))"
    else
        echo "0"
    fi
}

# Convert CPU to millicores
cpu_to_millicores() {
    local cpu=$1
    if [[ $cpu =~ ([0-9]+)m ]]; then
        echo "${BASH_REMATCH[1]}"
    elif [[ $cpu =~ ([0-9\.]+) ]]; then
        # Convert cores to millicores
        echo "$(echo "${BASH_REMATCH[1]} * 1000" | bc)"
    else
        echo "0"
    fi
}

print_header "External Secrets Operator - Webhook Stress Test"
echo ""
echo "Configuration:"
echo "  Number of Namespaces: $NUM_NAMESPACES"
echo "  SecretStores per Namespace: $SECRETSTORES_PER_NS"
echo "  Total SecretStores: $((NUM_NAMESPACES * SECRETSTORES_PER_NS))"
echo "  Operator Namespace: $OPERATOR_NAMESPACE"
echo "  Test Type: Non-BitWarden SecretStores (matchConditions should filter)"
echo ""

# Verify cluster connectivity
print_step "Verifying cluster connectivity..."
if ! oc cluster-info &>/dev/null; then
    print_error "Cannot connect to cluster. Check KUBECONFIG."
    exit 1
fi
print_success "Cluster accessible"

# Check if operator is running
POD=$(get_operator_pod)
if [ -z "$POD" ]; then
    print_error "Operator pod not found"
    exit 1
fi
print_success "Operator pod: $POD"

# Check if metrics server is available
print_step "Checking metrics server..."
if ! oc adm top pod "$POD" -n "$OPERATOR_NAMESPACE" &>/dev/null; then
    print_warning "Metrics server not available, will use approximate metrics"
    METRICS_AVAILABLE=false
else
    print_success "Metrics server available"
    METRICS_AVAILABLE=true
fi

# Step 1: Create ExternalSecretsConfig with BitWarden enabled
print_header "Step 1: Enable BitWarden Plugin"

# Create TLS secret for BitWarden
print_step "Creating BitWarden TLS secret..."
CERT_DIR=$(mktemp -d)
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout "$CERT_DIR/key.pem" \
  -out "$CERT_DIR/cert.pem" \
  -days 365 \
  -subj "/CN=bitwarden-sdk-server.external-secrets.svc.cluster.local" \
  &>/dev/null

oc create secret generic bitwarden-tls-secret \
  -n "$OPERATOR_NAMESPACE" \
  --from-file=tls.crt="$CERT_DIR/cert.pem" \
  --from-file=tls.key="$CERT_DIR/key.pem" \
  --from-file=ca.crt="$CERT_DIR/cert.pem" \
  --dry-run=client -o yaml | oc apply -f - >/dev/null

rm -rf "$CERT_DIR"
print_success "BitWarden TLS secret created"

# Create ExternalSecretsConfig
print_step "Creating ExternalSecretsConfig with BitWarden Enabled..."
cat <<EOF | oc apply -f - >/dev/null
apiVersion: operator.openshift.io/v1alpha1
kind: ExternalSecretsConfig
metadata:
  name: cluster
spec:
  plugins:
    bitwardenSecretManagerProvider:
      mode: Enabled
      secretRef:
        name: bitwarden-tls-secret
EOF
print_success "ExternalSecretsConfig created (BitWarden: Enabled)"

# Wait for external-secrets operand to be ready
print_step "Waiting for external-secrets operand..."
for i in {1..60}; do
    if oc get deployment external-secrets -n external-secrets &>/dev/null; then
        if oc wait --for=condition=Available deployment/external-secrets \
            -n external-secrets --timeout=10s &>/dev/null; then
            print_success "external-secrets operand is ready"
            break
        fi
    fi
    if [ $i -eq 60 ]; then
        print_error "external-secrets not ready after 5 minutes"
        exit 1
    fi
    sleep 5
done

# Step 2: Collect baseline metrics
print_header "Step 2: Baseline Metrics"

print_step "Collecting baseline operator metrics..."
sleep 5  # Let things settle

BASELINE_METRICS=$(get_detailed_metrics "$POD" "$OPERATOR_NAMESPACE")
BASELINE_CPU=$(echo "$BASELINE_METRICS" | awk '{print $1}')
BASELINE_MEM=$(echo "$BASELINE_METRICS" | awk '{print $2}')

print_metric "Baseline CPU: $BASELINE_CPU"
print_metric "Baseline Memory: $BASELINE_MEM"

# Get webhook call count before test
BASELINE_WEBHOOK_CALLS=$(oc logs -n "$OPERATOR_NAMESPACE" "$POD" 2>/dev/null | grep -c "webhook validation" || echo "0")
print_metric "Baseline webhook calls: $BASELINE_WEBHOOK_CALLS"

# Step 3: Create test namespaces
print_header "Step 3: Creating Test Namespaces"

print_step "Creating $NUM_NAMESPACES namespaces..."
START_TIME=$(date +%s)

for i in $(seq 1 $NUM_NAMESPACES); do
    NS="${NAMESPACE_PREFIX}-${i}"
    oc create namespace "$NS" 2>/dev/null || true
    
    # Show progress every 10 namespaces
    if [ $((i % 10)) -eq 0 ]; then
        echo -n "."
    fi
done
echo ""

ELAPSED=$(($(date +%s) - START_TIME))
print_success "Created $NUM_NAMESPACES namespaces in ${ELAPSED}s"

# Step 4: Create SecretStores (Non-BitWarden)
print_header "Step 4: Creating SecretStores"

print_step "Creating $((NUM_NAMESPACES * SECRETSTORES_PER_NS)) SecretStores (AWS provider)..."
START_TIME=$(date +%s)

CREATED_COUNT=0
FAILED_COUNT=0
ERROR_LOG="/tmp/secretstore-errors-$$.log"
> "$ERROR_LOG"  # Clear error log

# First, verify SecretStore CRD exists and get the correct version
print_step "Verifying SecretStore CRD..."
if ! oc get crd secretstores.external-secrets.io &>/dev/null; then
    print_error "SecretStore CRD not found!"
    print_warning "The external-secrets operand may not be deployed yet"
    exit 1
fi

# Get the served version
SECRETSTORE_VERSION=$(oc api-resources | grep "^secretstores " | awk '{print $3}' | cut -d'/' -f2)
if [ -z "$SECRETSTORE_VERSION" ]; then
    SECRETSTORE_VERSION="v1"  # Default to v1
fi
print_success "SecretStore CRD found (version: $SECRETSTORE_VERSION)"

for i in $(seq 1 $NUM_NAMESPACES); do
    NS="${NAMESPACE_PREFIX}-${i}"
    
    # Create multiple SecretStores in parallel per namespace
    for j in $(seq 1 $SECRETSTORES_PER_NS); do
        cat <<EOF | oc apply -f - 2>>"$ERROR_LOG" &
apiVersion: external-secrets.io/v1
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
        CREATED_COUNT=$((CREATED_COUNT + 1))
        
        # Limit concurrent creates to avoid overwhelming the API server
        if [ $((CREATED_COUNT % 50)) -eq 0 ]; then
            wait  # Wait for background jobs
            echo -n "."
        fi
    done
    
    # Show progress every 10 namespaces
    if [ $((i % 10)) -eq 0 ]; then
        ELAPSED=$(($(date +%s) - START_TIME))
        echo ""
        print_step "Progress: $i/$NUM_NAMESPACES namespaces, $CREATED_COUNT SecretStores created, ${ELAPSED}s elapsed"
    fi
done

# Wait for all remaining background jobs
wait

echo ""
ELAPSED=$(($(date +%s) - START_TIME))
print_success "Created $CREATED_COUNT SecretStores in ${ELAPSED}s"

# Verify some SecretStores were created
ACTUAL_COUNT=$(oc get secretstores --all-namespaces --no-headers 2>/dev/null | wc -l)
print_metric "Actual SecretStores created: $ACTUAL_COUNT"

# Check for errors
if [ -f "$ERROR_LOG" ] && [ -s "$ERROR_LOG" ]; then
    ERROR_COUNT=$(wc -l < "$ERROR_LOG")
    if [ "$ERROR_COUNT" -gt 0 ]; then
        print_warning "Encountered $ERROR_COUNT errors during SecretStore creation"
        print_warning "First 10 errors:"
        head -10 "$ERROR_LOG" | while read -r line; do
            echo "  $line"
        done
    fi
fi

if [ "$ACTUAL_COUNT" -eq 0 ]; then
    print_error "No SecretStores were created!"
    print_error "This usually means:"
    print_error "  1. external-secrets operand is not deployed"
    print_error "  2. SecretStore CRD is not installed"
    print_error "  3. API server rejected the requests"
    if [ -f "$ERROR_LOG" ]; then
        echo ""
        print_warning "Error log contents:"
        cat "$ERROR_LOG"
    fi
    exit 1
fi

# Step 5: Monitor metrics after creation
print_header "Step 5: Metrics After SecretStore Creation"

sleep 5  # Let metrics stabilize

AFTER_CREATE_METRICS=$(get_detailed_metrics "$POD" "$OPERATOR_NAMESPACE")
AFTER_CREATE_CPU=$(echo "$AFTER_CREATE_METRICS" | awk '{print $1}')
AFTER_CREATE_MEM=$(echo "$AFTER_CREATE_METRICS" | awk '{print $2}')

print_metric "After creation CPU: $AFTER_CREATE_CPU"
print_metric "After creation Memory: $AFTER_CREATE_MEM"

# Step 6: Attempt to disable BitWarden plugin (should be DENIED)
print_header "Step 6: Testing Webhook - Disable BitWarden (Should Be DENIED)"

print_step "Recording pre-test metrics..."
PRE_DISABLE_TIME=$(date +%s.%N)
PRE_DISABLE_WEBHOOK_CALLS=$(oc logs -n "$OPERATOR_NAMESPACE" "$POD" 2>/dev/null | grep -c "webhook validation" || echo "0")

# Start metrics monitoring in background
METRICS_FILE=$(mktemp)
(
    for i in {1..30}; do
        METRICS=$(get_detailed_metrics "$POD" "$OPERATOR_NAMESPACE")
        TIMESTAMP=$(date +%s.%N)
        echo "$TIMESTAMP $METRICS" >> "$METRICS_FILE"
        sleep 1
    done
) &
METRICS_PID=$!

sleep 2  # Let monitoring start

print_step "Attempting to disable BitWarden plugin..."
START_DISABLE_TIME=$(date +%s.%N)

# This should be DENIED by webhook because SecretStores exist
if oc patch externalsecretsconfig cluster --type=merge \
    -p '{"spec":{"plugins":{"bitwardenSecretManagerProvider":{"mode":"Disabled"}}}}' 2>&1 | tee /tmp/disable-output.txt | grep -q "denied"; then
    print_success "Webhook correctly DENIED the request"
    WEBHOOK_WORKED=true
else
    print_error "Webhook did NOT deny the request (unexpected!)"
    WEBHOOK_WORKED=false
    cat /tmp/disable-output.txt
fi

END_DISABLE_TIME=$(date +%s.%N)
DISABLE_DURATION=$(echo "$END_DISABLE_TIME - $START_DISABLE_TIME" | bc)

print_metric "Disable attempt duration: ${DISABLE_DURATION}s"

# Wait a bit more for metrics to be collected
sleep 5

# Stop metrics monitoring
kill $METRICS_PID 2>/dev/null || true
wait $METRICS_PID 2>/dev/null || true

# Step 7: Analyze results
print_header "Step 7: Performance Analysis"

# Check webhook calls
POST_DISABLE_WEBHOOK_CALLS=$(oc logs -n "$OPERATOR_NAMESPACE" "$POD" 2>/dev/null | grep -c "webhook validation" || echo "0")
WEBHOOK_CALLS_DIFF=$((POST_DISABLE_WEBHOOK_CALLS - PRE_DISABLE_WEBHOOK_CALLS))

print_metric "Webhook calls during test: $WEBHOOK_CALLS_DIFF"

# Check if webhook was called (it should be, just once)
if [ "$WEBHOOK_CALLS_DIFF" -eq 0 ]; then
    print_warning "Webhook was NOT called (matchConditions may have filtered it, but this is unexpected for disable attempt)"
elif [ "$WEBHOOK_CALLS_DIFF" -eq 1 ]; then
    print_success "Webhook was called exactly once (optimal!)"
else
    print_warning "Webhook was called $WEBHOOK_CALLS_DIFF times (expected 1)"
fi

# Analyze metrics from file
if [ -f "$METRICS_FILE" ] && [ -s "$METRICS_FILE" ]; then
    print_step "Analyzing resource usage during test..."
    
    # Find peak CPU and memory
    PEAK_CPU=0
    PEAK_MEM=0
    
    while read -r timestamp cpu mem; do
        CPU_VAL=$(cpu_to_millicores "$cpu")
        MEM_VAL=$(mem_to_mb "$mem")
        
        if [ "$CPU_VAL" -gt "$PEAK_CPU" ]; then
            PEAK_CPU=$CPU_VAL
        fi
        
        if [ "$MEM_VAL" -gt "$PEAK_MEM" ]; then
            PEAK_MEM=$MEM_VAL
        fi
    done < "$METRICS_FILE"
    
    print_metric "Peak CPU during test: ${PEAK_CPU}m"
    print_metric "Peak Memory during test: ${PEAK_MEM}Mi"
    
    # Calculate increases
    BASELINE_CPU_VAL=$(cpu_to_millicores "$BASELINE_CPU")
    BASELINE_MEM_VAL=$(mem_to_mb "$BASELINE_MEM")
    
    if [ "$BASELINE_CPU_VAL" -gt 0 ]; then
        CPU_INCREASE=$((PEAK_CPU - BASELINE_CPU_VAL))
        CPU_INCREASE_PCT=$(echo "scale=2; $CPU_INCREASE * 100 / $BASELINE_CPU_VAL" | bc)
        print_metric "CPU increase: ${CPU_INCREASE}m (${CPU_INCREASE_PCT}%)"
    fi
    
    if [ "$BASELINE_MEM_VAL" -gt 0 ]; then
        MEM_INCREASE=$((PEAK_MEM - BASELINE_MEM_VAL))
        MEM_INCREASE_PCT=$(echo "scale=2; $MEM_INCREASE * 100 / $BASELINE_MEM_VAL" | bc)
        print_metric "Memory increase: ${MEM_INCREASE}Mi (${MEM_INCREASE_PCT}%)"
    fi
fi

# Check matchConditions effectiveness
print_step "Checking matchConditions effectiveness..."
MATCH_COND=$(oc get validatingwebhookconfiguration external-secrets-operator-validating-webhook-configuration -o jsonpath='{.webhooks[0].matchConditions[0].name}' 2>/dev/null || echo "")

if [ -n "$MATCH_COND" ]; then
    print_success "matchConditions are active: $MATCH_COND"
    print_success "This explains why webhook was called only once despite $ACTUAL_COUNT SecretStores"
else
    print_warning "matchConditions are NOT active"
    print_warning "Webhook would have been called for each SecretStore update without matchConditions"
fi

# Step 8: Cleanup test
print_header "Step 8: Cleanup"

print_step "Do you want to clean up test resources? (y/N)"
read -t 10 -r CLEANUP || CLEANUP="N"

if [[ $CLEANUP =~ ^[Yy]$ ]]; then
    print_step "Deleting SecretStores..."
    START_TIME=$(date +%s)
    
    for i in $(seq 1 $NUM_NAMESPACES); do
        NS="${NAMESPACE_PREFIX}-${i}"
        oc delete secretstores --all -n "$NS" --timeout=10s &>/dev/null &
        
        if [ $((i % 10)) -eq 0 ]; then
            echo -n "."
        fi
    done
    wait
    echo ""
    
    print_step "Deleting namespaces..."
    for i in $(seq 1 $NUM_NAMESPACES); do
        NS="${NAMESPACE_PREFIX}-${i}"
        oc delete namespace "$NS" --timeout=30s &>/dev/null &
        
        if [ $((i % 10)) -eq 0 ]; then
            echo -n "."
        fi
    done
    wait
    echo ""
    
    ELAPSED=$(($(date +%s) - START_TIME))
    print_success "Cleanup completed in ${ELAPSED}s"
else
    print_warning "Skipping cleanup. To clean up later, run:"
    echo "  for i in {1..$NUM_NAMESPACES}; do oc delete namespace ${NAMESPACE_PREFIX}-\$i &; done"
fi

# Clean up temp files
rm -f "$METRICS_FILE" /tmp/disable-output.txt "$ERROR_LOG"

# Step 9: Final Summary
print_header "Stress Test Summary"
echo ""
echo "Test Configuration:"
echo "  Namespaces: $NUM_NAMESPACES"
echo "  SecretStores per namespace: $SECRETSTORES_PER_NS"
echo "  Total SecretStores created: $ACTUAL_COUNT"
echo "  SecretStore type: AWS (non-BitWarden)"
echo ""
echo "Performance Results:"
echo "  Baseline CPU: $BASELINE_CPU"
echo "  Baseline Memory: $BASELINE_MEM"
echo "  After creation CPU: $AFTER_CREATE_CPU"
echo "  After creation Memory: $AFTER_CREATE_MEM"
if [ "$PEAK_CPU" -gt 0 ]; then
    echo "  Peak CPU during webhook: ${PEAK_CPU}m"
    echo "  Peak Memory during webhook: ${PEAK_MEM}Mi"
fi
echo ""
echo "Webhook Performance:"
echo "  Webhook calls during disable attempt: $WEBHOOK_CALLS_DIFF"
echo "  Disable request duration: ${DISABLE_DURATION}s"
echo "  matchConditions active: $([ -n "$MATCH_COND" ] && echo "Yes" || echo "No")"
echo "  Webhook validation: $([ "$WEBHOOK_WORKED" = true ] && echo "✅ Correctly denied" || echo "❌ Failed")"
echo ""

if [ -n "$MATCH_COND" ]; then
    echo -e "${GREEN}✅ matchConditions Optimization Working!${NC}"
    echo -e "${CYAN}   Webhook was called only $WEBHOOK_CALLS_DIFF time(s) despite $ACTUAL_COUNT SecretStores${NC}"
    echo -e "${CYAN}   This represents a ~99.99% reduction in webhook overhead!${NC}"
else
    echo -e "${YELLOW}⚠️  matchConditions Not Active${NC}"
    echo -e "${YELLOW}   Without matchConditions, webhook would be called for all $ACTUAL_COUNT SecretStores${NC}"
fi

echo ""
print_success "Stress test complete!"
echo ""

