#!/usr/bin/env bash

# Ignore any expected automated changes like the timestamp update in clusterserviceversion file.
ignore_expected_changes() {
	csv_filename="bundle/manifests/external-secrets-operator.clusterserviceversion.yaml"
	diff=$(git diff --no-ext-diff --unified=0 ${csv_filename} | egrep "^\+" | grep -Ev "createdAt|clusterserviceversion")
	if [[ -z ${diff} ]]; then
		git checkout ${csv_filename}
	fi
}

##############################################
###############  MAIN  #######################
##############################################

# update the git index
git update-index -q --ignore-submodules --refresh

ignore_expected_changes

# git add all files, so that even untracked files are counted.
git add . && git diff-index --cached --ignore-submodules --name-status --exit-code HEAD
if [[ $? -ne 0 ]]; then
	echo -e "\n-- ERROR -- There are uncommitted changes after running verify target. Please commit the changes.\n"
	exit 1
fi

exit 0

