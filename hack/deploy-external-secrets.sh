#!/usr/bin/env bash
#
# Apply a minimal (empty spec) ExternalSecretsConfig CR and wait until the
# external-secrets operand is Available.
#
# Used by TLS scanner / PQC readiness CI jobs (deploy-operand step) and for
# local verification against a cluster where the operator is already installed
# via OLM.
#
# Prerequisites:
#   - oc in PATH, logged into a cluster with the external-secrets-operator installed
set -o errexit
set -o nounset
set -o pipefail

OPERATOR_NAMESPACE="${OPERATOR_NAMESPACE:-external-secrets-operator}"
OPERAND_NAMESPACE="${OPERAND_NAMESPACE:-external-secrets}"
OPERATOR_DEPLOYMENT="${OPERATOR_DEPLOYMENT:-external-secrets-operator-controller-manager}"
POLL_ATTEMPTS="${POLL_ATTEMPTS:-60}"
POLL_SLEEP_SECONDS="${POLL_SLEEP_SECONDS:-5}"

# Deployments the operator creates for an empty ExternalSecretsConfig spec:
# core controller, webhook and the in-tree cert-controller (the latter is only
# skipped when spec.controllerConfig.certProvider.certManager is enabled).
OPERAND_DEPLOYMENTS=(external-secrets external-secrets-webhook external-secrets-cert-controller)

wait_for_operator() {
  echo "Waiting for the external-secrets operator deployment..."
  oc wait --for=create "namespace/${OPERATOR_NAMESPACE}" --timeout=5m
  oc wait --for=create -n "${OPERATOR_NAMESPACE}" "deployment/${OPERATOR_DEPLOYMENT}" --timeout=5m
  oc wait --for=condition=Available -n "${OPERATOR_NAMESPACE}" "deployment/${OPERATOR_DEPLOYMENT}" --timeout=5m
}

apply_external_secrets_config_cr() {
  echo "Creating ExternalSecretsConfig CR (minimal, empty spec)..."
  # Apply (not create) so a CR left behind by a previous run, or one created by
  # the e2e suite, is reconciled to this minimal shape instead of failing.
  oc apply -f - <<EOF
apiVersion: operator.openshift.io/v1alpha1
kind: ExternalSecretsConfig
metadata:
  name: cluster
spec: {}
EOF
}

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
    oc get externalsecretsconfigs.operator.openshift.io cluster -o yaml || true
    exit 1
  fi

  # The operand namespace is created by the operator during reconcile.
  oc wait --for=create "namespace/${OPERAND_NAMESPACE}" --timeout=5m
  local deploy
  for deploy in "${OPERAND_DEPLOYMENTS[@]}"; do
    oc wait --for=create -n "${OPERAND_NAMESPACE}" "deployment/${deploy}" --timeout=5m
  done
  for deploy in "${OPERAND_DEPLOYMENTS[@]}"; do
    oc wait --for=condition=Available -n "${OPERAND_NAMESPACE}" "deployment/${deploy}" --timeout=5m
  done
  # Deployment Available can stay True while an older pod is still terminating,
  # so wait for the operand pods themselves to report Ready.
  for deploy in "${OPERAND_DEPLOYMENTS[@]}"; do
    oc wait --for=condition=Ready -n "${OPERAND_NAMESPACE}" pod \
      -l "app.kubernetes.io/name=${deploy}" --timeout=5m
  done
}

wait_for_operator
apply_external_secrets_config_cr
wait_for_external_secrets
echo "external-secrets operand is Available"
