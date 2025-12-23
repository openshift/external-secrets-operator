#!/bin/bash
# Deployment script for webhook implementation
# This builds the operator image and deploys it to the cluster

set -e

# Configuration
export KUBECONFIG=/home/mykastur/gcp_n/install-dir/auth/kubeconfig
export IMG=${IMG:-quay.io/rh-ee-mykastur/eso:webhook-test}

echo "=========================================="
echo "Deploying External Secrets Operator Webhook"
echo "=========================================="
echo "Image: $IMG"
echo "Cluster: $(kubectl cluster-info | head -1)"
echo ""

# Step 1: Build image
echo "Step 1: Building operator image..."
make image-build IMG="$IMG"

# Step 2: Push image
echo ""
echo "Step 2: Pushing image to registry..."
echo "Note: You must be logged in to quay.io"
echo "Run: podman login quay.io"
make image-push IMG="$IMG"

# Step 3: Deploy
echo ""
echo "Step 3: Deploying to cluster..."
make deploy IMG="$IMG"

echo ""
echo "=========================================="
echo "Deployment complete!"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. Wait for pod to be ready:"
echo "     kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=external-secrets-operator -n external-secrets-operator --timeout=120s"
echo ""
echo "  2. Check webhook certificate (OpenShift service-ca):"
echo "     oc get secret webhook-server-cert -n external-secrets-operator"
echo ""
echo "  3. Verify CA bundle injected:"
echo "     oc get validatingwebhookconfiguration external-secrets-operator-validating-webhook -o jsonpath='{.webhooks[0].clientConfig.caBundle}' | base64 -d"
echo ""
echo "  4. Test webhook functionality:"
echo "     See TESTING_GUIDE.md for test scenarios"
echo ""





