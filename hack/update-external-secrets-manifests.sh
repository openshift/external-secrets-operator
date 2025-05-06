#!/bin/bash

set -e

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"
source "$(dirname "${BASH_SOURCE}")/lib/yq.sh"

EXTERNAL_SECRETS_VERSION=${1:?"missing external-secrets version. Please specify a version from https://github.com/external-secrets/external-secrets/releases"}

mkdir -p ./_output

echo "---- Downloading external-secrets manifests ${EXTERNAL_SECRETS_VERSION} ----"

bin/helm repo add external-secrets https://charts.external-secrets.io --force-update
bin/helm template external-secrets external-secrets/external-secrets -n external-secrets --version "${EXTERNAL_SECRETS_VERSION}" --set webhook.certManager.enabled=true --set bitwarden-sdk-server.enabled=true > _output/manifests.yaml

echo "---- Patching manifest ----"

#remove nonessential fields from each resource manifests.
./bin/yq e 'del(.metadata.labels."helm.sh/chart")' -i _output/manifests.yaml
./bin/yq e 'del(.spec.template.metadata.labels."helm.sh/chart")' -i _output/manifests.yaml

# update all occurences of app.kubernetes.io/managed-by label value.
./bin/yq e \
  '(.. | select(has("app.kubernetes.io/managed-by"))."app.kubernetes.io/managed-by") |= "external-secrets-operator"' \
   -i _output/manifests.yaml

# regenerate all bindata
rm -rf bindata/external-secrets
rm -f config/crd/bases/customresourcedefinition_*

# split into individual manifest files
yq '... comments=""' -s '.kind + "_" + .metadata.name | downcase' _output/manifests.yaml

#customize manifests
yq -i '.spec.template.spec.containers[] |= select (.name == "external-secrets") |= .args += ["--enable-leader-election=false", "--enable-cluster-store-reconciler=false", "--enable-cluster-external-secret-reconciler=false", "--enable-push-secret-reconciler=false"]' deployment_external-secrets.yml

# Move CRDs to appropriate location
#mkdir -p bindata/external-secrets/crds
mkdir -p bindata/external-secrets/resources

mv customresourcedefinition_* config/crd/bases/
mv *.yml bindata/external-secrets/resources

# Clean up
rm _output/manifests.yaml

