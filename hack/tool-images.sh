#!/usr/bin/env bash

# Assert an image reference is digest-pinned (@sha256:...).
# Usage: require_image_digest <image_ref> [name]
require_image_digest()
{
	local image="$1"
	local name="${2:-image}"

	if [[ ! "${image}" =~ @sha256:[a-fA-F0-9]{64}$ ]]; then
		echo "[$(date)] -- ERROR -- ${name} must be pinned with @sha256:: ${image}" >&2
		return 1
	fi
	return 0
}
