# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 1.0.0

# EXTERNAL_SECRETS_VERSION defines the external-secrets release version to fetch helm charts.
EXTERNAL_SECRETS_VERSION ?= v0.19.0

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# operator.openshift.io/external-secrets-operator-bundle:$VERSION and operator.openshift.io/external-secrets-operator-catalog:$VERSION.
IMAGE_TAG_BASE ?= operator.openshift.io/external-secrets-operator

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)

# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
	BUNDLE_GEN_FLAGS += --use-image-digests
endif

# Set the Operator SDK version to use. By default, what is installed on the system is used.
# This is useful for CI or a project to utilize a specific version of the operator-sdk toolkit.
OPERATOR_SDK_VERSION ?= v1.39.0
# Image URL to use all building/pushing image targets
IMG ?= openshift.io/external-secrets-operator:latest
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.32.0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= podman

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

COMMIT ?= $(shell git rev-parse HEAD)
SHORTCOMMIT ?= $(shell git rev-parse --short HEAD)
GOBUILD_VERSION_ARGS = -ldflags "-X $(PACKAGE)/pkg/version.SHORTCOMMIT=$(SHORTCOMMIT) -X $(PACKAGE)/pkg/version.COMMIT=$(COMMIT)"

# Include the library makefiles
include $(addprefix ./vendor/github.com/openshift/build-machinery-go/make/, \
    targets/openshift/bindata.mk \
    targets/openshift/yq.mk \
)

# generate bindata targets
$(call add-bindata,assets,./bindata/...,bindata,assets,pkg/operator/assets/bindata.go)

.PHONY: all
all: build verify

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet envtest test-apis test-unit ## Run tests.

.PHONY: test-unit
test-unit: vet ## Run unit tests.
	go test $$(go list ./... | grep -vE 'test/[e2e|apis|utils]') -coverprofile cover.out

update-operand-manifests: helm yq
	hack/update-external-secrets-manifests.sh $(EXTERNAL_SECRETS_VERSION)
.PHONY: update-operand-manifests

# Utilize Kind or modify the e2e tests to load the image locally, enabling compatibility with other vendors.
E2E_TIMEOUT ?= 1h
# E2E_GINKGO_LABEL_FILTER is ginkgo label query for selecting tests. See
# https://onsi.github.io/ginkgo/#spec-labels. The default is to run tests on the AWS platform.
E2E_GINKGO_LABEL_FILTER ?= "Platform: isSubsetOf {AWS}"
.PHONY: test-e2e  # Run the e2e tests against a Kind k8s instance that is spun up.
test-e2e:
	go test \
	-timeout $(E2E_TIMEOUT) \
	-count 1 \
	-v \
	-p 1 \
	-tags e2e \
	./test/e2e \
	-ginkgo.v \
	-ginkgo.show-node-events \
	-ginkgo.label-filter=$(E2E_GINKGO_LABEL_FILTER)

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

##@ Build

build-operator: ## Build operator binary, no additional checks or code generation
	@GOFLAGS="-mod=vendor" source hack/go-fips.sh && \
	go build $(GOBUILD_VERSION_ARGS) -o $(LOCALBIN)/external-secrets-operator cmd/external-secrets-operator/main.go

.PHONY: build
build: manifests generate fmt vet build-operator ## Build manager binary.

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/external-secrets-operator/main.go --v=5 --metrics-secure=false

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: image-build
image-build: ## Build operator image.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: image-push
image-push: ## Push operator image.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the operator image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- docker buildx create --name external-secrets-operator-builder
	docker buildx use external-secrets-operator-builder
	- docker buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- docker buildx rm external-secrets-operator-builder
	rm Dockerfile.cross

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default > dist/install.yaml

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply --server-side -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply --server-side -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Location to store temp outputs
OUTPUTS_PATH ?= $(shell pwd)/_output
$(OUTPUTS_PATH):
	mkdir -p $(OUTPUTS_PATH)

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
YQ = $(LOCALBIN)/yq
HELM ?= $(LOCALBIN)/helm
REFERENCE_DOC_GENERATOR ?= $(LOCALBIN)/crd-ref-docs
GOVULNCHECK ?= $(LOCALBIN)/govulncheck
GINKGO ?= $(LOCALBIN)/ginkgo

## Tool Versions
YQ_VERSION = v4.45.2
HELM_VERSION ?= v3.17.3

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen)

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest)

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint)

.PHONY: crd-ref-docs
crd-ref-docs: $(LOCALBIN) ## Download crd-ref-docs locally if necessary.
	$(call go-install-tool,$(REFERENCE_DOC_GENERATOR),github.com/elastic/crd-ref-docs)

.PHONY: govulncheck
govulncheck: $(LOCALBIN) ## Download govulncheck locally if necessary.
	$(call go-install-tool,$(GOVULNCHECK),golang.org/x/vuln/cmd/govulncheck)

.PHONY: ginkgo
ginkgo: $(LOCALBIN) ## Download ginkgo locally if necessary.
	$(call go-install-tool,$(GINKGO),github.com/onsi/ginkgo/v2/ginkgo)

# go-install-tool will 'go install' any package with custom target and name of the binary.
# $1 - target path with name of binary
# $2 - package url which can be installed
define go-install-tool
@{ \
set -e; \
package=$(2) ;\
echo "Installing $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) GOFLAGS="-mod=vendor" go install $${package} ;\
}
endef

.PHONY: operator-sdk
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
operator-sdk: ## Download operator-sdk locally if necessary.
ifeq (,$(wildcard $(OPERATOR_SDK)))
ifeq (, $(shell which operator-sdk 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPERATOR_SDK)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPERATOR_SDK) https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$${OS}_$${ARCH} ;\
	chmod +x $(OPERATOR_SDK) ;\
	}
else
OPERATOR_SDK = $(shell which operator-sdk)
endif
endif

.PHONY: yq
yq: $(LOCALBIN) ensure-yq  ## Download yq locally if necessary.

.PHONY: helm
helm: ## Download helm locally if necessary.
ifeq (,$(wildcard $(HELM)))
ifeq (, $(shell ls $(HELM) 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(HELM)) ;\
	temp_dir=$(shell mktemp -d) && OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $${temp_dir}/helm.tar.gz https://get.helm.sh/helm-${HELM_VERSION}-$${OS}-$${ARCH}.tar.gz ;\
	tar -xf $${temp_dir}/helm.tar.gz -C $${temp_dir} ;\
	cp $${temp_dir}/$${OS}-$${ARCH}/helm $(HELM) ;\
	chmod +x $(HELM) ;\
	rm -r $${temp_dir} ;\
	}
endif
endif

.PHONY: bundle
bundle: manifests kustomize operator-sdk ## Generate bundle manifests and metadata, then validate generated files.
	$(OPERATOR_SDK) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	$(CONTAINER_TOOL) build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(CONTAINER_TOOL) push $(BUNDLE_IMG)

.PHONY: opm
OPM = $(LOCALBIN)/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.23.0/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool $(CONTAINER_TOOL) --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(CONTAINER_TOOL) push $(CATALOG_IMG)

## verify the changes are working as expected.
.PHONY: verify
verify: vet fmt golangci-lint verify-bindata verify-bindata-assets verify-generated govulnscan

## update the relevant data based on new changes.
.PHONY: update
update: generate manifests update-operand-manifests update-bindata bundle docs

## checks for any uncommitted changes.
.PHONY: check-git-diff
check-git-diff: update
	./hack/check-git-diff-clean.sh

## generate internal docs.
.PHONY: docs
docs: crd-ref-docs
	$(REFERENCE_DOC_GENERATOR) --source-path=api/v1alpha1/ --renderer=markdown --config=hack/docs/config.yaml --output-path=docs/api_reference.md

## perform vulnerabilities scan using govulncheck.
.PHONY: govulnscan
# The ignored vulnerabilities are not in the operator code, but in the vendored packages.
# Each vulnerability ID corresponds to a specific issue that has been reviewed and deemed
# acceptable for the current vendored dependencies.
# - https://pkg.go.dev/vuln/GO-2025-3956
# - https://pkg.go.dev/vuln/GO-2025-3547
# - https://pkg.go.dev/vuln/GO-2025-3521
KNOWN_VULNERABILITIES=GO-2025-3956|GO-2025-3547|GO-2025-3521
govulnscan: govulncheck $(OUTPUTS_PATH)  ## Run govulncheck
	@echo "Running govulncheck vulnerability scan..."
	@$(GOVULNCHECK) ./... > $(OUTPUTS_PATH)/govulcheck.results 2>&1 || true
	@grep -q "pkg.go.dev" $(OUTPUTS_PATH)/govulcheck.results || { \
		echo "-- ERROR -- govulncheck may have failed to run; see $(OUTPUTS_PATH)/govulcheck.results"; exit 1; }
	@echo "Filtering known vulnerabilities and counting new ones..."
	$(eval reported_vulnerabilities = $(strip $(shell grep "pkg.go.dev" $(OUTPUTS_PATH)/govulcheck.results | grep -Ev "$(KNOWN_VULNERABILITIES)" | wc -l)))
	@echo "Found $(reported_vulnerabilities) new vulnerabilities (excluding known issues)"
	@(if [ $(reported_vulnerabilities) -ne 0 ]; then \
		echo ""; \
		echo "-- ERROR -- $(reported_vulnerabilities) new vulnerabilities reported"; \
		echo "Please review $(OUTPUTS_PATH)/govulcheck.results for details"; \
		echo ""; \
		exit 1; \
	else \
		echo "✓ Vulnerability scan passed - no new issues found"; \
	fi)

# Utilize controller-runtime provided envtest for API integration test
.PHONY: test-apis  ## Run only the api integration tests.
test-apis: envtest ginkgo
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" ./hack/test-apis.sh

.PHONY: clean
clean:
	rm -rf $(LOCALBIN) $(OUTPUTS_PATH) cover.out dist
