#!/bin/bash
set -euo pipefail
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$SCRIPT_DIR"
NEW_EXTERNAL_SECRETS_VERSION="${NEW_EXTERNAL_SECRETS_VERSION:-}"
NEW_BUNDLE_VERSION="${NEW_BUNDLE_VERSION:-}"
OLD_BUNDLE_VERSION="${OLD_BUNDLE_VERSION:-}"
OLD_EXTERNAL_SECRETS_VERSION="${OLD_EXTERNAL_SECRETS_VERSION:-}"
TARGET_BRANCH="${TARGET_BRANCH:-master}"

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

usage() {
    cat << EOF
Usage: $0 [OPTIONS]
Automates the external-secrets-operator rebase process.
Environment Variables:
  NEW_EXTERNAL_SECRETS_VERSION  New external-secrets version (e.g., v0.20.1, v0.20.4)
  NEW_BUNDLE_VERSION           New bundle version (e.g., 1.1.0)
  OLD_BUNDLE_VERSION           Old bundle version to replace (optional - auto-detected)
  OLD_EXTERNAL_SECRETS_VERSION Old external-secrets version (optional - auto-detected)
  TARGET_BRANCH                Target git branch for PR (default: master)
Options:
  -h, --help                 Show this help message
  -d, --dry-run             Show what would be done without making changes
  -s, --step STEP           Run only specific step (1-5)
  --skip-commit             Skip git commits (useful for testing)
Steps:
  1. Bump deps with upstream external-secrets
  2. Update Makefile: VERSION, EXTERNAL_SECRETS_VERSION
  3. Update operand manifests (external-secrets helm charts)
  4. Update CSV: OLM bundle name, version, replaces, skipRange
  5. Generate bundle and bindata
EOF
}

check_prerequisites() {
    log_info "Checking prerequisites..."
    if ! git rev-parse --git-dir > /dev/null 2>&1; then log_error "Not in a git repository"; exit 1; fi
    local required_tools=("go" "make" "sed" "grep")
    for tool in "${required_tools[@]}"; do
        if ! command -v "$tool" &> /dev/null; then log_error "$tool is not installed"; exit 1; fi
    done
    if [[ -z "$NEW_EXTERNAL_SECRETS_VERSION" ]]; then log_error "NEW_EXTERNAL_SECRETS_VERSION is not set"; log_info "Example: export NEW_EXTERNAL_SECRETS_VERSION=v0.20.1"; exit 1; fi
    if [[ -z "$NEW_BUNDLE_VERSION" ]]; then log_error "NEW_BUNDLE_VERSION is not set"; log_info "Example: export NEW_BUNDLE_VERSION=1.1.0"; exit 1; fi
    if [[ ! "$NEW_EXTERNAL_SECRETS_VERSION" =~ ^v ]]; then NEW_EXTERNAL_SECRETS_VERSION="v${NEW_EXTERNAL_SECRETS_VERSION}"; log_warning "Added 'v' prefix to external-secrets version: $NEW_EXTERNAL_SECRETS_VERSION"; fi
    log_success "Prerequisites check passed"
}

detect_current_versions() {
    log_info "Auto-detecting current versions..."
    if [[ -z "$OLD_BUNDLE_VERSION" ]]; then OLD_BUNDLE_VERSION=$(grep "^VERSION" Makefile | head -1 | cut -d'=' -f2 | tr -d ' ?'); log_info "Auto-detected OLD_BUNDLE_VERSION: $OLD_BUNDLE_VERSION"; fi
    if [[ -z "$OLD_EXTERNAL_SECRETS_VERSION" ]]; then OLD_EXTERNAL_SECRETS_VERSION=$(grep "^EXTERNAL_SECRETS_VERSION" Makefile | cut -d'=' -f2 | tr -d ' ?'); log_info "Auto-detected OLD_EXTERNAL_SECRETS_VERSION: $OLD_EXTERNAL_SECRETS_VERSION"; fi
    if [[ -z "$OLD_BUNDLE_VERSION" || -z "$OLD_EXTERNAL_SECRETS_VERSION" ]]; then log_error "Failed to auto-detect current versions"; exit 1; fi
    log_success "Version detection completed"
    log_info "OLD_BUNDLE_VERSION: $OLD_BUNDLE_VERSION"
    log_info "OLD_EXTERNAL_SECRETS_VERSION: $OLD_EXTERNAL_SECRETS_VERSION"
    log_info "NEW_BUNDLE_VERSION: $NEW_BUNDLE_VERSION"
    log_info "NEW_EXTERNAL_SECRETS_VERSION: $NEW_EXTERNAL_SECRETS_VERSION"
}

backup_current_state() {
    log_info "Creating backup of current state..."
    local backup_branch="backup-$(date +%Y%m%d-%H%M%S)"
    git branch "$backup_branch"
    log_success "Created backup branch: $backup_branch"
}

step1_bump_deps() {
    log_info "Step 1: Bumping deps with upstream external-secrets@$NEW_EXTERNAL_SECRETS_VERSION"
    if [[ "$DRY_RUN" == "true" ]]; then
        log_warning "[DRY RUN] Would execute:"
        echo "  go get github.com/external-secrets/external-secrets@$NEW_EXTERNAL_SECRETS_VERSION"
        echo "  go mod edit -replace github.com/external-secrets/external-secrets=github.com/openshift/external-secrets@$NEW_EXTERNAL_SECRETS_VERSION"
        echo "  go mod tidy && go mod vendor"
        return 0
    fi
    log_info "Running: go get github.com/external-secrets/external-secrets@$NEW_EXTERNAL_SECRETS_VERSION"
    go get "github.com/external-secrets/external-secrets@$NEW_EXTERNAL_SECRETS_VERSION"
    log_info "Running: go mod edit -replace github.com/external-secrets/external-secrets=github.com/openshift/external-secrets@$NEW_EXTERNAL_SECRETS_VERSION"
    go mod edit -replace "github.com/external-secrets/external-secrets=github.com/openshift/external-secrets@$NEW_EXTERNAL_SECRETS_VERSION"
    log_info "Running: go mod tidy && go mod vendor"
    go mod tidy
    go mod vendor
    if [[ "$SKIP_COMMIT" != "true" ]]; then
        git add go.mod go.sum vendor/
        git commit -m "Bump deps with upstream external-secrets@$NEW_EXTERNAL_SECRETS_VERSION"
        log_success "Step 1 committed"
    fi
    log_success "Step 1 completed"
}

step2_update_makefile() {
    log_info "Step 2: Update Makefile: VERSION, EXTERNAL_SECRETS_VERSION"
    if [[ "$DRY_RUN" == "true" ]]; then
        log_warning "[DRY RUN] Would update Makefile:"
        echo "  VERSION: $OLD_BUNDLE_VERSION -> $NEW_BUNDLE_VERSION"
        echo "  EXTERNAL_SECRETS_VERSION: $OLD_EXTERNAL_SECRETS_VERSION -> $NEW_EXTERNAL_SECRETS_VERSION"
        return 0
    fi
    log_info "Updating VERSION: $OLD_BUNDLE_VERSION -> $NEW_BUNDLE_VERSION"
    sed -i "s/^VERSION ?= $OLD_BUNDLE_VERSION/VERSION ?= $NEW_BUNDLE_VERSION/" Makefile
    log_info "Updating EXTERNAL_SECRETS_VERSION: $OLD_EXTERNAL_SECRETS_VERSION -> $NEW_EXTERNAL_SECRETS_VERSION"
    sed -i "s|^EXTERNAL_SECRETS_VERSION ?= $OLD_EXTERNAL_SECRETS_VERSION|EXTERNAL_SECRETS_VERSION ?= $NEW_EXTERNAL_SECRETS_VERSION|" Makefile
    if [[ "$SKIP_COMMIT" != "true" ]]; then
        git add Makefile
        git commit -m "Update Makefile: VERSION, EXTERNAL_SECRETS_VERSION"
        log_success "Step 2 committed"
    fi
    log_success "Step 2 completed"
}

step3_update_operand_manifests() {
    log_info "Step 3: Update operand manifests from external-secrets helm charts"
    if [[ "$DRY_RUN" == "true" ]]; then
        log_warning "[DRY RUN] Would execute:"
        echo "  make update-operand-manifests"
        echo "  This fetches external-secrets helm charts for version $NEW_EXTERNAL_SECRETS_VERSION"
        return 0
    fi
    log_info "Running: make update-operand-manifests"
    make update-operand-manifests
    if [[ "$SKIP_COMMIT" != "true" ]]; then
        git add bindata/
        git commit -m "Update operand manifests for external-secrets@$NEW_EXTERNAL_SECRETS_VERSION"
        log_success "Step 3 committed"
    fi
    log_success "Step 3 completed"
}

step4_update_csv() {
    log_info "Step 4: Update CSV: OLM bundle name, version, replaces, skipRange"
    if [[ "$DRY_RUN" == "true" ]]; then
        log_warning "[DRY RUN] Would update CSV files:"
        echo "  version: $OLD_BUNDLE_VERSION -> $NEW_BUNDLE_VERSION"
        echo "  name: external-secrets-operator.v$OLD_BUNDLE_VERSION -> external-secrets-operator.v$NEW_BUNDLE_VERSION"
        echo "  replaces: external-secrets-operator.v[previous] -> external-secrets-operator.v$OLD_BUNDLE_VERSION"
        echo "  skipRange: <$OLD_BUNDLE_VERSION -> <$NEW_BUNDLE_VERSION"
        return 0
    fi
    local csv_file="config/manifests/bases/external-secrets-operator.clusterserviceversion.yaml"
    if [[ -f "$csv_file" ]]; then
        log_info "Updating $csv_file"
        sed -i "s/version: $OLD_BUNDLE_VERSION/version: $NEW_BUNDLE_VERSION/" "$csv_file"
        sed -i "s/name: external-secrets-operator\.v$OLD_BUNDLE_VERSION/name: external-secrets-operator.v$NEW_BUNDLE_VERSION/" "$csv_file"
        if grep -q "replaces:" "$csv_file"; then
            sed -i "s/replaces: external-secrets-operator\.v[0-9]\+\.[0-9]\+\.[0-9]\+/replaces: external-secrets-operator.v$OLD_BUNDLE_VERSION/" "$csv_file"
        else
            if [[ "$OLD_BUNDLE_VERSION" != "0.0.0" ]]; then
                sed -i "/name: external-secrets-operator\.v$NEW_BUNDLE_VERSION/a\  replaces: external-secrets-operator.v$OLD_BUNDLE_VERSION" "$csv_file"
            fi
        fi
        sed -i "s/olm\.skipRange: <[0-9]\+\.[0-9]\+\.[0-9]\+/olm.skipRange: <$NEW_BUNDLE_VERSION/" "$csv_file"
    fi
    if [[ "$SKIP_COMMIT" != "true" ]]; then
        git add "$csv_file"
        git commit -m "Update CSV: version, name, replaces, skipRange for v$NEW_BUNDLE_VERSION"
        log_success "Step 4 committed"
    fi
    log_success "Step 4 completed"
}

step5_generate_bundle() {
    log_info "Step 5: Generate bundle manifests and update bindata"
    if [[ "$DRY_RUN" == "true" ]]; then
        log_warning "[DRY RUN] Would execute:"
        echo "  make manifests"
        echo "  make bundle"
        echo "  make update-bindata"
        return 0
    fi
    log_info "Running: make manifests"
    make manifests
    log_info "Running: make bundle"
    make bundle
    log_info "Running: make update-bindata"
    make update-bindata
    if [[ "$SKIP_COMMIT" != "true" ]] && [[ -n "$(git status --porcelain)" ]]; then
        git add .
        git commit -m "Generate bundle manifests and update bindata for v$NEW_BUNDLE_VERSION"
        log_success "Step 5 committed"
    else
        log_info "No changes to commit in Step 5"
    fi
    log_success "Step 5 completed"
}

run_all_steps() {
    log_info "Running all rebase steps..."
    step1_bump_deps
    step2_update_makefile
    step3_update_operand_manifests
    step4_update_csv
    step5_generate_bundle
    log_success "All steps completed successfully!"
    log_info "Summary of changes:"
    log_info "  - Bumped external-secrets from $OLD_EXTERNAL_SECRETS_VERSION to $NEW_EXTERNAL_SECRETS_VERSION"
    log_info "  - Updated bundle version from $OLD_BUNDLE_VERSION to $NEW_BUNDLE_VERSION"
    log_info "  - Updated operand manifests (helm charts)"
    log_info "  - Updated CSV metadata and skipRange"
    log_info "  - Generated bundle manifests and bindata"
    log_info ""
    log_info "Next steps:"
    log_info "  1. Review the changes: git diff"
    log_info "  2. Run tests: make test"
    log_info "  3. Create PR targeting '$TARGET_BRANCH' branch"
}

main() {
    local DRY_RUN=false
    local SKIP_COMMIT=false
    local SPECIFIC_STEP=""
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help) usage; exit 0 ;;
            -d|--dry-run) DRY_RUN=true; shift ;;
            -s|--step) SPECIFIC_STEP="$2"; shift 2 ;;
            --skip-commit) SKIP_COMMIT=true; shift ;;
            *) log_error "Unknown option: $1"; usage; exit 1 ;;
        esac
    done
    export DRY_RUN SKIP_COMMIT
    log_info "Starting external-secrets-operator rebase automation"
    check_prerequisites
    detect_current_versions
    if [[ "$DRY_RUN" != "true" ]]; then
        backup_current_state
    fi
    if [[ -n "$SPECIFIC_STEP" ]]; then
        case "$SPECIFIC_STEP" in
            1) step1_bump_deps ;;
            2) step2_update_makefile ;;
            3) step3_update_operand_manifests ;;
            4) step4_update_csv ;;
            5) step5_generate_bundle ;;
            *) log_error "Invalid step: $SPECIFIC_STEP. Must be 1-5"; exit 1 ;;
        esac
    else
        run_all_steps
    fi
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "Dry run completed. No changes were made."
    else
        log_success "Rebase automation completed successfully!"
    fi
}

main "$@"