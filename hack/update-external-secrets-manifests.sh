#!/bin/bash

set -e

EXTERNAL_SECRETS_VERSION=${1:?"missing external-secrets version. Please specify a version from https://github.com/external-secrets/external-secrets/releases"}
MANIFESTS_PATH=./_output/manifests

mkdir -p ${MANIFESTS_PATH}

echo "---- Downloading external-secrets manifests ${EXTERNAL_SECRETS_VERSION} ----"

bin/helm repo add external-secrets https://charts.external-secrets.io --force-update
# render templates with certManager enabled to fetch cert-manager specific manifests.
bin/helm template external-secrets external-secrets/external-secrets -n external-secrets \
	--version "${EXTERNAL_SECRETS_VERSION}" \
	--set webhook.certManager.enabled=true \
	--set bitwarden-sdk-server.enabled=true \
	--set metrics.service.enabled=true \
	--set webhook.metrics.service.enabled=true \
	--set certController.metrics.service.enabled=false \
	> ${MANIFESTS_PATH}/manifests.yaml
# render templates with certManager disabled to fetch cert-controller specific manifests.
bin/helm template external-secrets external-secrets/external-secrets -n external-secrets \
	--version "${EXTERNAL_SECRETS_VERSION}" \
	--set webhook.certManager.enabled=false \
	--set bitwarden-sdk-server.enabled=true \
	--set metrics.service.enabled=true \
	--set webhook.metrics.service.enabled=true \
	--set certController.metrics.service.enabled=true \
	>> ${MANIFESTS_PATH}/manifests.yaml

echo "---- Patching manifest ----"

# remove non-essential fields from each resource manifests.
./bin/yq e 'del(.metadata.labels."helm.sh/chart")' -i ${MANIFESTS_PATH}/manifests.yaml
./bin/yq e 'del(.spec.template.metadata.labels."helm.sh/chart")' -i ${MANIFESTS_PATH}/manifests.yaml

# update all occurences of app.kubernetes.io/managed-by label value.
./bin/yq e \
  '(.. | select(has("app.kubernetes.io/managed-by"))."app.kubernetes.io/managed-by") |= "external-secrets-operator"' \
   -i ${MANIFESTS_PATH}/manifests.yaml

# add custom label to all CRDs
./bin/yq e 'select(.kind == "CustomResourceDefinition").metadata.labels."app" = "external-secrets"' -i ${MANIFESTS_PATH}/manifests.yaml

# regenerate all bindata
rm -rf bindata/external-secrets/resources
rm -f config/crd/bases/customresourcedefinition_*

# split into individual manifest files
./bin/yq '... comments=""' -s '"_output/manifests/" + .kind + "_" + .metadata.name + ".yml" | downcase' ${MANIFESTS_PATH}/manifests.yaml

# customize manifests
./bin/yq -i '.spec.template.spec.containers[] |= select (.name == "external-secrets") |= .args += ["--enable-leader-election=false", "--enable-cluster-store-reconciler=false", "--enable-cluster-external-secret-reconciler=false", "--enable-push-secret-reconciler=false"]' ${MANIFESTS_PATH}/deployment_external-secrets.yml

# remobe non-essential manifests
rm ${MANIFESTS_PATH}/customresourcedefinition_fakes.generators.external-secrets.io.yml

# Move resource manifests to appropriate location
mkdir -p bindata/external-secrets/resources

mv ${MANIFESTS_PATH}/customresourcedefinition_* config/crd/bases/
mv ${MANIFESTS_PATH}/*.yml bindata/external-secrets/resources

# Clean up
rm -r ${MANIFESTS_PATH}

