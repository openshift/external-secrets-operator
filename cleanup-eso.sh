#!/bin/bash
# Cleanup script for External Secrets Operator
# Removes all ESO-related resources from the cluster

set -e

# Configuration
KUBECONFIG="${KUBECONFIG:-/home/mykastur/gcp_n/install-dir/auth/kubeconfig}"
export KUBECONFIG

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_step() {
    echo -e "${BLUE}==>${NC} $1"
}

print_success() {
    echo -e "${GREEN}✅${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠️${NC} $1"
}

echo "=========================================="
echo "External Secrets Operator - Cleanup"
echo "=========================================="
echo "Cluster: $(oc cluster-info | head -1 | cut -d' ' -f6)"
echo ""

print_warning "This will delete ALL External Secrets Operator resources!"
echo "Press Ctrl+C within 5 seconds to cancel..."
sleep 5

# Step 1: Delete ExternalSecrets (managed secrets)
print_step "Step 1: Deleting ExternalSecrets..."
oc delete externalsecrets --all --all-namespaces --timeout=30s 2>/dev/null && print_success "ExternalSecrets deleted" || print_warning "No ExternalSecrets found or already deleted"

# Step 2: Delete PushSecrets
print_step "Step 2: Deleting PushSecrets..."
oc delete pushsecrets --all --all-namespaces --timeout=30s 2>/dev/null && print_success "PushSecrets deleted" || print_warning "No PushSecrets found"

# Step 3: Delete ClusterExternalSecrets
print_step "Step 3: Deleting ClusterExternalSecrets..."
oc delete clusterexternalsecrets --all --timeout=30s 2>/dev/null && print_success "ClusterExternalSecrets deleted" || print_warning "No ClusterExternalSecrets found"

# Step 4: Delete SecretStores (namespaced)
print_step "Step 4: Deleting SecretStores..."
oc delete secretstores --all --all-namespaces --timeout=30s 2>/dev/null && print_success "SecretStores deleted" || print_warning "No SecretStores found"

# Step 5: Delete ClusterSecretStores
print_step "Step 5: Deleting ClusterSecretStores..."
oc delete clustersecretstores --all --timeout=30s 2>/dev/null && print_success "ClusterSecretStores deleted" || print_warning "No ClusterSecretStores found"

# Step 6: Delete Generators
print_step "Step 6: Deleting Generator resources..."
for generator in acraccesstokens ecrauthorizationtokens fakes gcraccesstokens githubaccesstokens passwords sshkeys stssessiontokens uuids vaultdynamicsecrets webhooks grafanas mfas quayaccesstokens; do
    oc delete $generator --all --all-namespaces --timeout=10s 2>/dev/null || true
done
print_success "Generator resources deleted"

# Step 7: Delete ClusterGenerators
print_step "Step 7: Deleting ClusterGenerators..."
oc delete clustergenerators --all --timeout=30s 2>/dev/null && print_success "ClusterGenerators deleted" || print_warning "No ClusterGenerators found"

# Step 8: Delete GeneratorStates
print_step "Step 8: Deleting GeneratorStates..."
oc delete generatorstates --all --all-namespaces --timeout=30s 2>/dev/null && print_success "GeneratorStates deleted" || print_warning "No GeneratorStates found"

# Step 9: Delete ExternalSecretsConfig
print_step "Step 9: Deleting ExternalSecretsConfig..."
oc delete externalsecretsconfig --all --timeout=30s 2>/dev/null && print_success "ExternalSecretsConfig deleted" || print_warning "No ExternalSecretsConfig found"

# Step 9a: Force remove finalizers if stuck
if oc get externalsecretsconfig 2>/dev/null | grep -v NAME | grep -q .; then
    print_warning "ExternalSecretsConfig still exists, removing finalizers..."
    for esc in $(oc get externalsecretsconfig -o name 2>/dev/null); do
        oc patch $esc --type json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' 2>/dev/null || true
    done
    sleep 2
fi

# Step 10: Delete ExternalSecretsManager
print_step "Step 10: Deleting ExternalSecretsManager..."
oc delete externalsecretsmanager --all --timeout=30s 2>/dev/null && print_success "ExternalSecretsManager deleted" || print_warning "No ExternalSecretsManager found"

# Step 10a: Force remove finalizers if stuck
if oc get externalsecretsmanager 2>/dev/null | grep -v NAME | grep -q .; then
    print_warning "ExternalSecretsManager still exists, removing finalizers..."
    for esm in $(oc get externalsecretsmanager -o name 2>/dev/null); do
        oc patch $esm --type json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' 2>/dev/null || true
    done
    sleep 2
fi

# Step 11: Delete operator deployment using kustomize
print_step "Step 11: Deleting operator deployment..."
cd "$(dirname "$0")"
if [ -f "config/default/kustomization.yaml" ]; then
    bin/kustomize build config/default | oc delete --ignore-not-found=true -f - 2>/dev/null && print_success "Operator deployment deleted" || print_warning "Some resources not found"
else
    print_warning "kustomization.yaml not found, skipping"
fi

# Step 12: Delete namespace
print_step "Step 12: Deleting external-secrets namespace..."
oc delete namespace external-secrets --timeout=60s 2>/dev/null && print_success "external-secrets namespace deleted" || print_warning "Namespace not found or already deleted"

# Step 13: Delete operator namespace
print_step "Step 13: Deleting external-secrets-operator namespace..."
oc delete namespace external-secrets-operator --timeout=60s 2>/dev/null && print_success "external-secrets-operator namespace deleted" || print_warning "Namespace not found or already deleted"

# Step 14: Delete webhook configurations
print_step "Step 14: Deleting webhook configurations..."
oc delete validatingwebhookconfiguration -l app.kubernetes.io/name=external-secrets-operator --timeout=10s 2>/dev/null && print_success "Webhook configurations deleted" || print_warning "No webhook configurations found"
oc delete validatingwebhookconfiguration validating-webhook-configuration --timeout=10s 2>/dev/null || true
oc delete validatingwebhookconfiguration external-secrets-operator-validating-webhook-configuration --timeout=10s 2>/dev/null || true
oc delete validatingwebhookconfiguration eso-bitwarden-webhook --timeout=10s 2>/dev/null || true
oc delete validatingwebhookconfiguration eso-webhook-test --timeout=10s 2>/dev/null || true

# Step 15: Delete CRDs
print_step "Step 15: Deleting CRDs..."
oc delete crd \
    externalsecrets.external-secrets.io \
    clustersecretstores.external-secrets.io \
    secretstores.external-secrets.io \
    clusterexternalsecrets.external-secrets.io \
    pushsecrets.external-secrets.io \
    clusterpushsecrets.external-secrets.io \
    acraccesstokens.generators.external-secrets.io \
    ecrauthorizationtokens.generators.external-secrets.io \
    fakes.generators.external-secrets.io \
    gcraccesstokens.generators.external-secrets.io \
    githubaccesstokens.generators.external-secrets.io \
    passwords.generators.external-secrets.io \
    sshkeys.generators.external-secrets.io \
    stssessiontokens.generators.external-secrets.io \
    uuids.generators.external-secrets.io \
    vaultdynamicsecrets.generators.external-secrets.io \
    webhooks.generators.external-secrets.io \
    grafanas.generators.external-secrets.io \
    mfas.generators.external-secrets.io \
    quayaccesstokens.generators.external-secrets.io \
    clustergenerators.generators.external-secrets.io \
    generatorstates.generators.external-secrets.io \
    externalsecretsconfigs.operator.openshift.io \
    externalsecretsmanagers.operator.openshift.io \
    --timeout=30s 2>/dev/null && print_success "CRDs deleted" || print_warning "Some CRDs not found"

# Step 15a: Force remove CRD finalizers if stuck
print_step "Checking for stuck CRDs..."
STUCK_CRDS=$(oc get crd -o json 2>/dev/null | jq -r '.items[] | select(.metadata.deletionTimestamp != null and (.metadata.name | contains("external-secrets") or contains("operator.openshift.io"))) | .metadata.name' 2>/dev/null)
if [ -n "$STUCK_CRDS" ]; then
    print_warning "Found CRDs stuck in terminating state, removing finalizers..."
    for crd in $STUCK_CRDS; do
        echo "  Patching CRD: $crd"
        oc patch crd $crd --type json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' 2>/dev/null || true
    done
    sleep 5
    print_success "Finalizers removed from stuck CRDs"
fi

# Step 16: Delete ClusterRoles and ClusterRoleBindings
print_step "Step 16: Deleting ClusterRoles and ClusterRoleBindings..."
oc delete clusterrole -l app.kubernetes.io/name=external-secrets-operator --timeout=10s 2>/dev/null && print_success "ClusterRoles deleted" || print_warning "No ClusterRoles found"
oc delete clusterrolebinding -l app.kubernetes.io/name=external-secrets-operator --timeout=10s 2>/dev/null && print_success "ClusterRoleBindings deleted" || print_warning "No ClusterRoleBindings found"

# Also delete by specific names
oc delete clusterrole external-secrets-operator-manager-role external-secrets-operator-metrics-auth-role external-secrets-operator-metrics-reader 2>/dev/null || true
oc delete clusterrolebinding external-secrets-operator-manager-rolebinding external-secrets-operator-metrics-auth-rolebinding 2>/dev/null || true

# Step 17: Verify cleanup
print_step "Step 17: Verifying cleanup..."
echo ""
echo "Remaining resources check:"
echo "- SecretStores: $(oc get secretstores --all-namespaces 2>/dev/null | wc -l)"
echo "- ClusterSecretStores: $(oc get clustersecretstores 2>/dev/null | wc -l)"
echo "- ExternalSecrets: $(oc get externalsecrets --all-namespaces 2>/dev/null | wc -l)"
echo "- Webhook Configs: $(oc get validatingwebhookconfiguration -l app.kubernetes.io/name=external-secrets-operator 2>/dev/null | wc -l)"
echo "- Operator Pods: $(oc get pods -n external-secrets-operator 2>/dev/null | wc -l)"

echo ""
echo "=========================================="
print_success "Cleanup Complete!"
echo "=========================================="
echo ""
echo "All External Secrets Operator resources have been removed from the cluster."
echo ""


