#!/bin/bash

# validate-rbac-resourcenames.sh
# This script validates that the resourceNames in kubebuilder RBAC annotations
# match the actual resource names defined in the assets.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# extract resourceNames from kubebuilder annotations
extract_kubebuilder_resourcenames() {
    local resource_type="$1"
    grep -E "^\s*//\s*\+kubebuilder:rbac:.*resources=${resource_type}.*resourceNames=" \
        "${PROJECT_ROOT}/pkg/controller/external_secrets/controller.go" | \
        sed -E 's/.*resourceNames=([^,]*).*/\1/' | \
        tr ';' '\n' | sort -u
}

# extract actual resource names from assets
extract_asset_names() {
    local pattern="$1"
    find "${PROJECT_ROOT}/bindata/external-secrets/resources" "${PROJECT_ROOT}/bindata/external-secrets" -name "*.yml" -exec grep -l "kind: ${pattern}" {} \; 2>/dev/null | \
        xargs grep -h "^  name:" | \
        awk '{print $2}' | sort -u
}

# extract CRD names from assets
extract_crd_names() {
    # Get CRDs from config/crd/bases
    local crd_names=""
    if [[ -d "${PROJECT_ROOT}/config/crd/bases" ]]; then
        crd_names=$(find "${PROJECT_ROOT}/config/crd/bases" -name "*.yml" -o -name "*.yaml" 2>/dev/null | \
            xargs grep -l "kind: CustomResourceDefinition" 2>/dev/null | \
            xargs grep -h "^  name:" 2>/dev/null | \
            awk '{print $2}')
    fi
    echo "${crd_names}" | grep -v "^$" | sort -u
}

# compare kubebuilder resourceNames with actual resources
compare_resources() {
    local resource_display_name="$1"
    local kubebuilder_resources="$2"
    local actual_resources="$3"
    
    echo "Kubebuilder resourceNames:"
    echo "${kubebuilder_resources}" | sed 's/^/  - /'
    echo
    echo "Actual ${resource_display_name} names:"
    echo "${actual_resources}" | sed 's/^/  - /'
    echo
    
    # Compare - find missing and extra resources
    local missing_in_kb
    missing_in_kb=$(comm -23 <(echo "${actual_resources}") <(echo "${kubebuilder_resources}"))
    
    local extra_in_kb
    extra_in_kb=$(comm -13 <(echo "${actual_resources}") <(echo "${kubebuilder_resources}"))
    
    local has_errors=false
    
    if [[ -n "${missing_in_kb}" ]]; then
        echo "Missing in kubebuilder annotations:"
        echo "${missing_in_kb}" | sed 's/^/    /'
        has_errors=true
    fi
    
    if [[ -n "${extra_in_kb}" ]]; then
        echo "Extra in kubebuilder annotations (might be outdated):"
        echo "${extra_in_kb}" | sed 's/^/    /'
    fi
    
    if [[ "$has_errors" == "false" ]]; then
        echo "${resource_display_name} validation passed"
        return 0
    else
        return 1
    fi
}

# Generic validation function
validate_resource_type() {
    local resource_display_name="$1"
    local kubebuilder_resource_type="$2"
    local asset_kind="$3"
    local extract_func="$4"
    
    echo "Validating ${resource_display_name}..."
    
    # Extract from kubebuilder annotations
    local kb_resources
    kb_resources=$(extract_kubebuilder_resourcenames "${kubebuilder_resource_type}" || echo "")
    
    # Extract from assets using the specified function
    local actual_resources
    if [[ "$extract_func" == "extract_crd_names" ]]; then
        actual_resources=$(extract_crd_names)
    else
        actual_resources=$(extract_asset_names "${asset_kind}")
    fi
    
    # Compare and report
    compare_resources "${resource_display_name}" "${kb_resources}" "${actual_resources}"
}

validate_deployments() {
    validate_resource_type "Deployments" "deployments" "Deployment" "extract_asset_names"
}

validate_webhooks() {
    validate_resource_type "ValidatingWebhookConfigurations" "validatingwebhookconfigurations" "ValidatingWebhookConfiguration" "extract_asset_names"
}

validate_crds() {
    validate_resource_type "CustomResourceDefinitions" "customresourcedefinitions" "" "extract_crd_names"
}

validate_roles() {
    validate_resource_type "Roles" "roles" "Role" "extract_asset_names"
}

validate_rolebindings() {
    validate_resource_type "RoleBindings" "rolebindings" "RoleBinding" "extract_asset_names"
}

main() {
    local exit_code=0
    
    echo "Validating RBAC resourceNames consistency for external-secrets-operator"
    echo "=================================================================================="
    
    validate_deployments || exit_code=1
    echo
    validate_webhooks || exit_code=1
    echo
    validate_crds || exit_code=1
    echo
    validate_roles || exit_code=1
    echo
    validate_rolebindings || exit_code=1
    echo
    
    if [[ $exit_code -eq 0 ]]; then
        echo "All RBAC resourceNames validations passed!"
        echo "The kubebuilder annotations are consistent with the actual resources."
    else
        echo "RBAC validation failed!"
        echo "Please update the kubebuilder annotations in pkg/controller/external_secrets/controller.go"
    fi
    
    return $exit_code
}

main "$@" 
