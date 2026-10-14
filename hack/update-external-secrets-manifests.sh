#!/usr/bin/env bash

set -o nounset
set -o pipefail
set -o errexit

EXTERNAL_SECRETS_VERSION=${1:?"missing external-secrets version. Please specify a version from https://github.com/external-secrets/external-secrets/releases"}
BITWARDEN_SDK_SERVER_VERSION=${2:?"missing bitwarden-sdk-server version. Please specify a version from https://github.com/external-secrets/bitwarden-sdk-server/releases"}
MANIFESTS_PATH=./_output/manifests
OPERAND_BINDATA=bindata/operands/external-secrets

mkdir -p ${MANIFESTS_PATH}

echo "---- Downloading external-secrets manifests ${EXTERNAL_SECRETS_VERSION} ----"

# The chart version doesn't have a "v" prefix, so strip it before using it in the helm command.
EXTERNAL_SECRETS_CHART_VERSION="${EXTERNAL_SECRETS_VERSION#v}"

bin/helm repo add external-secrets https://charts.external-secrets.io --force-update
# render templates with certManager enabled to fetch cert-manager specific manifests.
bin/helm template external-secrets external-secrets/external-secrets -n external-secrets \
	--version "${EXTERNAL_SECRETS_CHART_VERSION}" \
	--set webhook.certManager.enabled=true \
	--set metrics.service.enabled=true \
	--set webhook.metrics.service.enabled=true \
	--set certController.metrics.service.enabled=false \
	> ${MANIFESTS_PATH}/manifests.yaml
# render templates with certManager disabled to fetch cert-controller specific manifests.
bin/helm template external-secrets external-secrets/external-secrets -n external-secrets \
	--version "${EXTERNAL_SECRETS_CHART_VERSION}" \
	--set webhook.certManager.enabled=false \
	--set metrics.service.enabled=true \
	--set webhook.metrics.service.enabled=true \
	--set certController.metrics.service.enabled=true \
	>> ${MANIFESTS_PATH}/manifests.yaml

echo "---- Downloading bitwarden-sdk-server manifests ${BITWARDEN_SDK_SERVER_VERSION} ----"

# bitwarden-sdk-server publishes its own standalone chart as an OCI artifact, independently
# versioned from the external-secrets chart. Render it directly so the operand tracks its
# own upstream releases instead of the stale subchart pin.
bin/helm template external-secrets "oci://ghcr.io/external-secrets/charts/bitwarden-sdk-server" \
	--version "${BITWARDEN_SDK_SERVER_VERSION}" -n external-secrets \
	--set state.path=/state/.bitwarden-state | grep -Ev "^(Pulled|Digest):" \
	>> ${MANIFESTS_PATH}/manifests.yaml

echo "---- Patching external-secrets manifests ----"

# remove non-essential fields from each resource manifests.
./bin/yq e 'del(.metadata.labels."helm.sh/chart")' -i ${MANIFESTS_PATH}/manifests.yaml
./bin/yq e 'del(.spec.template.metadata.labels."helm.sh/chart")' -i ${MANIFESTS_PATH}/manifests.yaml

# update all occurences of app.kubernetes.io/managed-by label value.
./bin/yq e \
  '(.. | select(has("app.kubernetes.io/managed-by"))."app.kubernetes.io/managed-by") |= "external-secrets-operator"' \
   -i ${MANIFESTS_PATH}/manifests.yaml

# add custom label to all CRDs
./bin/yq e 'select(.kind == "CustomResourceDefinition").metadata.labels."app" = "external-secrets"' -i ${MANIFESTS_PATH}/manifests.yaml

# refresh helm-rendered operand bindata; preserve operator-owned manifests.
mkdir -p "${OPERAND_BINDATA}"
shopt -s extglob nullglob
(
	cd "${OPERAND_BINDATA}"
	for manifest in !(namespace_external-secrets.yml|certificate_bitwarden-tls-certs.yml); do
		rm -- "${manifest}"
	done
)
shopt -u extglob nullglob
rm -f config/crd/bases/customresourcedefinition_*

# split into individual manifest files
./bin/yq '... comments=""' -s '"_output/manifests/" + .kind + "_" + .metadata.name + ".yml" | downcase' ${MANIFESTS_PATH}/manifests.yaml

# customize manifests
./bin/yq -i '.spec.template.spec.containers[] |= select (.name == "external-secrets") |= .args += ["--enable-leader-election=false", "--enable-cluster-store-reconciler=false", "--enable-cluster-external-secret-reconciler=false", "--enable-push-secret-reconciler=false"]' ${MANIFESTS_PATH}/deployment_external-secrets.yml

# remobe non-essential manifests
rm ${MANIFESTS_PATH}/customresourcedefinition_fakes.generators.external-secrets.io.yml

# Move resource manifests to appropriate location
mv ${MANIFESTS_PATH}/customresourcedefinition_* config/crd/bases/
mv ${MANIFESTS_PATH}/*.yml "${OPERAND_BINDATA}"

# Clean up
rm -r ${MANIFESTS_PATH}
