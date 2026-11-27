#!/usr/bin/env bash

set -o nounset
set -o pipefail
set -o errexit

# Script to run govulncheck and filter known vulnerabilities.
# Usage: ./hack/govulncheck.sh <govulncheck_binary> <output_dir>
#
# Arguments:
#   govulncheck_binary: Path to the govulncheck binary
#   output_dir: Directory to store results
#
# Environment variables:
#   ARTIFACT_DIR: If set and directory exists, results are copied there for CI artifact collection

# Known vulnerabilities to ignore (in vendored packages, not operator code).
# Each vulnerability ID has been reviewed and deemed acceptable.
#
## Below vulnerabilities are in the kubernetes package, which impacts the server and not the operator, which is the client.
# - https://pkg.go.dev/vuln/GO-2025-3521 - Kubernetes GitRepo Volume Inadvertent Local Repository Access in k8s.io/kubernetes
# - https://pkg.go.dev/vuln/GO-2025-3547 - Kubernetes kube-apiserver Vulnerable to Race Condition in k8s.io/kubernetes
#
## Below vulnerabilities are fixed only in the Go 1.26.x stdlib (net/http, crypto/tls,
## crypto/x509, net/textproto) or require golang.org/x/net@v0.60.0+, which itself requires
## Go 1.26. This operator currently builds with Go 1.25 (see go.mod and Dockerfile); these
## remain until the toolchain is bumped to 1.26.
# - https://pkg.go.dev/vuln/GO-2026-6603 - net/http: HTTP/2 server memory exhaustion due to Trailer headers
# - https://pkg.go.dev/vuln/GO-2026-6605 - net/http: HTTP/1 client connection desynchronization after CONNECT rejection
# - https://pkg.go.dev/vuln/GO-2026-6607 - crypto/tls: reject malformed ECH outer extension references
# - https://pkg.go.dev/vuln/GO-2026-6608 - net/textproto, mime/multipart: memory limit bypass when parsing MIME headers
# - https://pkg.go.dev/vuln/GO-2026-6610 - net/http/internal/http2: transport accepts multiple Content-Length headers
# - https://pkg.go.dev/vuln/GO-2026-6611 - net/http: excessive CPU consumption from repeated initial window changes
# - https://pkg.go.dev/vuln/GO-2026-6612 - net/http: double flow control refund on HTTP/2 server streams
# - https://pkg.go.dev/vuln/GO-2026-6613 - net/http: HTTP/1 server connection desynchronization after 2xx CONNECT response
# - https://pkg.go.dev/vuln/GO-2026-6617 - net/http: HTTP/2 server crash due to HPACK encoder race
# - https://pkg.go.dev/vuln/GO-2026-6809 - crypto/x509: mitigation for security concerns with a specific Certificate Authority
# - https://pkg.go.dev/vuln/GO-2026-6859 - net/http: HTTP/1 server crash due to MaxBytesReader header race
#
## Below vulnerability is in cert-manager's ACME/DNS01 solver controller. This operator only
## imports cert-manager's API types (for scheme registration and List/Get/Watch of its CRs) and
## never runs cert-manager's own controllers, so the vulnerable code path is not exercised here.
# - https://pkg.go.dev/vuln/GO-2026-6889 - cert-manager: Direct ACME Challenge resources can bypass Issuer DNS01 solver policy
KNOWN_VULNS_PATTERN="GO-2025-3521|GO-2025-3547|GO-2026-6603|GO-2026-6605|GO-2026-6607|GO-2026-6608|GO-2026-6610|GO-2026-6611|GO-2026-6612|GO-2026-6613|GO-2026-6617|GO-2026-6809|GO-2026-6859|GO-2026-6889"

GOVULNCHECK_BIN="${1:-}"
OUTPUT_DIR="${2:-}"

if [[ -z "${GOVULNCHECK_BIN}" ]] || [[ -z "${OUTPUT_DIR}" ]]; then
    echo "Usage: $0 <govulncheck_binary> <output_dir>"
    exit 1
fi

RESULTS_FILE="${OUTPUT_DIR}/govulncheck.results"

echo "Running govulncheck vulnerability scan..."
mkdir -p "${OUTPUT_DIR}"

# Run govulncheck and capture output (don't fail on vulnerabilities found)
"${GOVULNCHECK_BIN}" ./... > "${RESULTS_FILE}" 2>&1 || true

# Copy results to ARTIFACT_DIR if in CI environment
if [[ -n "${ARTIFACT_DIR:-}" ]] && [[ -d "${ARTIFACT_DIR}" ]]; then
    cp "${RESULTS_FILE}" "${ARTIFACT_DIR}/"
    echo "Results copied to ${ARTIFACT_DIR}/govulncheck.results"
fi

# Verify govulncheck actually ran successfully
if ! grep -q "pkg.go.dev" "${RESULTS_FILE}"; then
    echo ""
    echo "-- ERROR -- govulncheck may have failed to run"
    echo "Please review ${RESULTS_FILE} for details"
    echo ""
    cat "${RESULTS_FILE}"
    exit 1
fi

echo "Filtering known vulnerabilities and counting new ones..."

# Find new vulnerabilities (not in known list)
if [[ -n "${KNOWN_VULNS_PATTERN}" ]]; then
    new_vulns=$(grep "pkg.go.dev" "${RESULTS_FILE}" | grep -Ev "${KNOWN_VULNS_PATTERN}" || true)
else
    new_vulns=$(grep "pkg.go.dev" "${RESULTS_FILE}" || true)
fi

if [[ -n "${new_vulns}" ]]; then
    reported=$(echo "${new_vulns}" | wc -l)
    echo ""
    echo "-- ERROR -- ${reported} new vulnerabilities reported:"
    echo "${new_vulns}"
    echo ""
    echo "Please review ${RESULTS_FILE} for details"
    echo "To ignore these vulnerabilities, add them to KNOWN_VULNS_PATTERN with valid justification"
    echo ""
    exit 1
else
    echo "✓ Vulnerability scan passed - no new issues found"
fi
