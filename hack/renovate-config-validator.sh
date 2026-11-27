#!/usr/bin/env bash

# shellcheck disable=SC1091 # sourced file path is runtime-resolved
source "$(dirname "${BASH_SOURCE[0]}")/tool-images.sh"

export RENOVATE_IMAGE="ghcr.io/renovatebot/renovate:44.237.3@sha256:2f8e7e452e276cf21fa644a6aee470f720f3351b26e423625113fed06d3a65ef"

validate_config()
{
	require_image_digest "${RENOVATE_IMAGE}" "RENOVATE_IMAGE"
	if ! podman run -e "LOG_LEVEL=debug" --rm -v "./renovate.json:/tmp/validate/renovate.json:Z" -w /tmp/validate \
		"${RENOVATE_IMAGE}" \
		renovate-config-validator --strict /tmp/validate/renovate.json; then
		exit 1
	fi
}

##############################################
###############  MAIN   ######################
##############################################

validate_config

exit 0
