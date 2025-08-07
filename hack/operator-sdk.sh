#!/bin/sh

set -e

OUTPUT_PATH=${1:-./bin/operator-sdk}
VERSION=${2:-"v1.39.0"}

GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)
BIN="operator-sdk"
BIN_ARCH="${BIN}_${GOOS}_${GOARCH}"
OPERATOR_SDK_DL_URL="https://github.com/operator-framework/operator-sdk/releases/download/${VERSION}"

if [[ "$GOOS" != "linux" && "$GOOS" != "darwin" ]]; then
  echo "Unsupported OS $GOOS"
  exit 1
fi

if [[ "$GOARCH" != "amd64" && "$GOARCH" != "arm64" && "$GOARCH" != "ppc64le" && "$GOARCH" != "s390x" ]]; then
  echo "Unsupported architecture $GOARCH"
  exit 1
fi

command -v curl &> /dev/null || { echo "can't find curl command" && exit 1; }

TEMPDIR=$(mktemp -d)
BIN_PATH="${TEMPDIR}/${BIN_ARCH}"

echo "> downloading binary"
curl --location -o "${BIN_PATH}" "${OPERATOR_SDK_DL_URL}/operator-sdk_${GOOS}_${GOARCH}"

echo "> installing binary"
mv "${BIN_PATH}" "${OUTPUT_PATH}"
chmod +x "${OUTPUT_PATH}"
rm -rf "${TEMPDIR}"

echo "> operator-sdk binary available at ${OUTPUT_PATH}"
