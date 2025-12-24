#!/bin/bash

# Run E2E tests with various options
# Usage: ./run-tests.sh [provider] [options]

set -e

PROVIDER="${1:-all}"
VERBOSE="${2:-}"

echo "=========================================="
echo "External Secrets Operator E2E Tests"
echo "=========================================="
echo

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Verify we're in the right directory
if [ ! -f "go.mod" ]; then
    echo "Error: Must be run from repository root"
    exit 1
fi

# Function to run tests
run_tests() {
    local label="$1"
    local name="$2"

    echo -e "${GREEN}Running $name tests...${NC}"
    echo

    if [ "$VERBOSE" = "-v" ] || [ "$VERBOSE" = "--verbose" ]; then
        make test-e2e E2E_GINKGO_LABEL_FILTER="$label" || return 1
    else
        make test-e2e E2E_GINKGO_LABEL_FILTER="$label" 2>&1 | grep -E "Ran|PASS|FAIL|SUCCESS|•" || return 1
    fi

    echo
}

# Main execution
case "$PROVIDER" in
    aws)
        echo "Provider: AWS"
        echo "Expected: ~12 tests"
        echo
        run_tests "Platform:AWS" "AWS"
        ;;

    gcp)
        echo "Provider: GCP"
        echo "Expected: 4 tests"
        echo
        run_tests "Platform:GCP" "GCP"
        ;;

    all)
        echo "Provider: All"
        echo "Expected: ~16 tests total"
        echo

        echo "1/2: Running AWS tests..."
        run_tests "Platform:AWS" "AWS"

        echo "2/2: Running GCP tests..."
        run_tests "Platform:GCP" "GCP"
        ;;

    *)
        echo "Usage: ./run-tests.sh [provider] [options]"
        echo
        echo "Providers:"
        echo "  aws    - Run AWS tests only (~12 tests)"
        echo "  gcp    - Run GCP tests only (4 tests)"
        echo "  all    - Run all tests (default)"
        echo
        echo "Options:"
        echo "  -v, --verbose  - Show verbose output"
        echo
        echo "Examples:"
        echo "  ./run-tests.sh aws"
        echo "  ./run-tests.sh all -v"
        exit 1
        ;;
esac

echo "=========================================="
echo -e "${GREEN}Test run complete!${NC}"
echo "=========================================="
