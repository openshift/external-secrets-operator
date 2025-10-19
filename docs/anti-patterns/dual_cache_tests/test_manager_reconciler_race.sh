#!/bin/bash
# By changing the catalog-source, differen versions of the operator can be tested.
# Earlier versions prior to ESO-203 fix should have the race condition reported in 20-50 attempts, later versions should not.
attempt=1
while true; do
    echo "Attempt: $attempt"
    kubectl delete application external-secrets-app -n openshift-gitops --ignore-not-found=true 2>/dev/null || true
    kubectl patch externalsecretsconfigs.operator.openshift.io/cluster -p '{"metadata":{"finalizers":null}}' --type=merge 2>/dev/null || true
    kubectl delete externalsecretsconfigs.operator.openshift.io cluster 2>/dev/null || true
    kubectl delete Subscription -n external-secrets-operator external-secrets-operator 2>/dev/null || true
    kubectl delete operatorgroup -n external-secrets-operator external-secrets-operator-group 2>/dev/null || true
    kubectl delete -f https://github.com/mytreya-rh/external-secrets-gitops/blob/main/manifests/og.yaml --ignore-not-found=true 2>/dev/null || true
    kubectl patch externalsecretsmanagers.operator.openshift.io/cluster -p '{"metadata":{"finalizers":null}}' --type=merge 2>/dev/null || true
    kubectl delete externalsecretsmanagers.operator.openshift.io cluster 2>/dev/null || true
    kubectl delete csv external-secrets-operator.v0.1.0 -n external-secrets-operator --ignore-not-found=true 2>/dev/null || true
    kubectl delete namespace external-secrets-operator --ignore-not-found=true 2>/dev/null || true
    kubectl delete namespace external-secrets --ignore-not-found=true 2>/dev/null || true
    kubectl patch crd/externalsecrets.external-secrets.io -p '{"metadata":{"finalizers":null}}' --type=merge 2>/dev/null || true
    kubectl patch crd/externalsecretsmanagers.operator.openshift.io -p '{"metadata":{"finalizers":null}}' --type=merge 2>/dev/null || true
    kubectl patch crd/externalsecrets.operator.openshift.io -p '{"metadata":{"finalizers":null}}' --type=merge 2>/dev/null || true
    kubectl patch crd/clusterexternalsecrets.external-secrets.io -p '{"metadata":{"finalizers":null}}' --type=merge 2>/dev/null || true
    kubectl delete crd externalsecrets.external-secrets.io externalsecretsconfigs.operator.openshift.io clusterexternalsecrets.external-secrets.io externalsecretsmanagers.operator.openshift.io --ignore-not-found=true 2>/dev/null || true
    kubectl delete crd `kubectl get crd | grep external-secrets | awk '{print $1}'` --ignore-not-found=true 2>/dev/null || true
    kubectl apply -f app.yaml
    sleep 150
    kubectl get ns external-secrets
    if [ $? -eq 1 ]; then
        echo "Race condition reproduced"
        echo "Attempt: $attempt"
         break
    fi
    attempt=$((attempt+1))
done
