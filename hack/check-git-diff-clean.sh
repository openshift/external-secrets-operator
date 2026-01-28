#!/usr/bin/env bash

set -o nounset
set -o pipefail
set -o errexit

# Ignore any expected automated changes like the timestamp update in clusterserviceversion file.
ignore_expected_changes() {
	csv_filename="bundle/manifests/external-secrets-operator.clusterserviceversion.yaml"
	if [[ -f "${csv_filename}" ]]; then
		diff=$(git diff --no-ext-diff --unified=0 "${csv_filename}" 2>/dev/null | grep -E "^\+" | grep -Ev "createdAt|clusterserviceversion" || true)
		if [[ -z "${diff}" ]]; then
			git checkout "${csv_filename}" 2>/dev/null || true
		fi
	fi
}

##############################################
###############  MAIN  #######################
##############################################

# Update the git index
git update-index -q --ignore-submodules --refresh

# Ignore expected automated changes
ignore_expected_changes

# Check for any changes including untracked files (for CI environments)
changes=$(git status --porcelain)

if [[ -n "${changes}" ]]; then
	echo -e "\n-- ERROR -- There are uncommitted or untracked changes. Please commit or remove them.\n"
	echo "Changed files:"
	git status --short
	exit 1
fi

exit 0
