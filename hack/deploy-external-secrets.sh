#!/usr/bin/env bash
#
# Apply an ExternalSecretsConfig CR and wait
# until the external-secrets operand — including bitwarden-sdk-server — is Available.
#
# Used by TLS scanner / PQC readiness CI jobs (deploy-operand step) and for
# local verification against a cluster where the operator is already installed
# via OLM.
#
# Prerequisites:
#   - oc and openssl in PATH, logged into a cluster with the external-secrets-operator installed
set -o errexit
set -o nounset
set -o pipefail

OPERATOR_NAMESPACE="${OPERATOR_NAMESPACE:-external-secrets-operator}"
OPERAND_NAMESPACE="external-secrets"
OPERATOR_DEPLOYMENT="${OPERATOR_DEPLOYMENT:-external-secrets-operator-controller-manager}"
POLL_ATTEMPTS="${POLL_ATTEMPTS:-60}"
POLL_SLEEP_SECONDS="${POLL_SLEEP_SECONDS:-5}"
BITWARDEN_TLS_SECRET_NAME="${BITWARDEN_TLS_SECRET_NAME:-bitwarden-tls-certs}"

# Deployments the operator creates:
# core controller, webhook, in-tree cert-controller (skipped when cert-manager is
# enabled), and bitwarden-sdk-server (enabled via spec.plugins).
OPERAND_DEPLOYMENTS=(external-secrets external-secrets-webhook external-secrets-cert-controller bitwarden-sdk-server)

# Waits for the operator namespace, deployment creation, and availability.
wait_for_operator() {
  echo "Waiting for the external-secrets operator deployment..."
  oc wait --for=create "namespace/${OPERATOR_NAMESPACE}" --timeout=2m
  oc wait --for=create -n "${OPERATOR_NAMESPACE}" "deployment/${OPERATOR_DEPLOYMENT}" --timeout=2m
  oc wait --for=condition=Available -n "${OPERATOR_NAMESPACE}" "deployment/${OPERATOR_DEPLOYMENT}" --timeout=2m
}

# Generates a self-signed CA + server certificate for bitwarden-sdk-server and
# creates the TLS secret referenced by ExternalSecretsConfig secretRef.
create_bitwarden_tls_secret() {
  echo "Generating self-signed TLS materials for bitwarden-sdk-server..."
  local tmpdir
  tmpdir=$(mktemp -d)
  trap 'rm -rf "${tmpdir}"' RETURN

  local service_name="bitwarden-sdk-server"
  local san="DNS:${service_name},DNS:${service_name}.${OPERAND_NAMESPACE},DNS:${service_name}.${OPERAND_NAMESPACE}.svc,DNS:${service_name}.${OPERAND_NAMESPACE}.svc.cluster.local,IP:127.0.0.1"

  openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes \
    -keyout "${tmpdir}/ca.key" -out "${tmpdir}/ca.crt" \
    -days 365 -subj "/CN=bitwarden-sdk-server-ca" 2>/dev/null

  openssl req -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes \
    -keyout "${tmpdir}/tls.key" -out "${tmpdir}/tls.csr" \
    -subj "/CN=${service_name}" \
    -addext "subjectAltName=${san}" 2>/dev/null

  openssl x509 -req -in "${tmpdir}/tls.csr" \
    -CA "${tmpdir}/ca.crt" -CAkey "${tmpdir}/ca.key" -CAcreateserial \
    -out "${tmpdir}/tls.crt" -days 365 \
    -extfile <(printf "subjectAltName=%s" "${san}") 2>/dev/null

  oc delete secret "${BITWARDEN_TLS_SECRET_NAME}" -n "${OPERAND_NAMESPACE}" --ignore-not-found
  oc create secret generic "${BITWARDEN_TLS_SECRET_NAME}" -n "${OPERAND_NAMESPACE}" \
    --from-file=tls.crt="${tmpdir}/tls.crt" \
    --from-file=tls.key="${tmpdir}/tls.key" \
    --from-file=ca.crt="${tmpdir}/ca.crt"

  echo "Created secret ${OPERAND_NAMESPACE}/${BITWARDEN_TLS_SECRET_NAME}"
}

# Deletes any leftover ExternalSecretsConfig, then creates a fresh one with
# the Bitwarden plugin enabled and pointing at the TLS secret.
apply_external_secrets_config_cr() {
  echo "Creating ExternalSecretsConfig CR with Bitwarden plugin enabled..."
  oc delete externalsecretsconfigs.operator.openshift.io cluster --ignore-not-found
  oc create -f - <<EOF
apiVersion: operator.openshift.io/v1alpha1
kind: ExternalSecretsConfig
metadata:
  name: cluster
spec:
  plugins:
    bitwardenSecretManagerProvider:
      mode: Enabled
      secretRef:
        name: ${BITWARDEN_TLS_SECRET_NAME}
EOF
}

# Polls the ExternalSecretsConfig Ready condition, then waits for operand
# deployments and pods.
wait_for_external_secrets() {
  echo "Waiting for ExternalSecretsConfig Ready and external-secrets deployments..."
  local found=false ready
  for _ in $(seq 1 "${POLL_ATTEMPTS}"); do
    ready=$(oc get externalsecretsconfigs.operator.openshift.io cluster \
      -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || true)
    if [[ "${ready}" == "True" ]]; then
      echo "ExternalSecretsConfig CR is Ready"
      found=true
      break
    fi
    sleep "${POLL_SLEEP_SECONDS}"
  done
  if [[ "${found}" != "true" ]]; then
    echo "Timed out waiting for ExternalSecretsConfig Ready=True"
    echo "Ready condition: status=$(oc get externalsecretsconfigs.operator.openshift.io cluster \
      -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}{" reason="}{.status.conditions[?(@.type=="Ready")].reason}' 2>/dev/null || true)"
    exit 1
  fi

  oc wait --for=create "namespace/${OPERAND_NAMESPACE}" --timeout=2m
  local deploy
  for deploy in "${OPERAND_DEPLOYMENTS[@]}"; do
    oc wait --for=create -n "${OPERAND_NAMESPACE}" "deployment/${deploy}" --timeout=2m
  done
  for deploy in "${OPERAND_DEPLOYMENTS[@]}"; do
    oc wait --for=condition=Available -n "${OPERAND_NAMESPACE}" "deployment/${deploy}" --timeout=2m
  done
  for deploy in "${OPERAND_DEPLOYMENTS[@]}"; do
    oc wait --for=condition=Ready -n "${OPERAND_NAMESPACE}" pod \
      -l "app.kubernetes.io/name=${deploy}" --timeout=2m
  done
}

wait_for_operator
apply_external_secrets_config_cr
# The operator creates the operand namespace as part of ESC reconciliation;
# wait for it before creating the TLS secret that bitwarden-sdk-server needs.
echo "Waiting for operand namespace to be created..."
oc wait --for=create "namespace/${OPERAND_NAMESPACE}" --timeout=2m
create_bitwarden_tls_secret
wait_for_external_secrets
echo "external-secrets operand (including bitwarden-sdk-server) is Available"
