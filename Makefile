# Project path.
PROJECT_ROOT := $(shell git rev-parse --show-toplevel 2>/dev/null || pwd)

# Warn when an undefined variable is referenced, helping catch typos and missing definitions.
MAKEFLAGS += --warn-undefined-variables

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c

# Ensure cache and config directories are writable (needed for CI environments where
# HOME may be unset or pointing to a non-writable directory like /).
export XDG_CACHE_HOME ?= $(PROJECT_ROOT)/_output/.cache
export XDG_CONFIG_HOME ?= $(PROJECT_ROOT)/_output/.config

# IMG_VERSION defines the images version for the operator, bundle and catalog (must be valid semver: Major.Minor.Patch).
# To re-generate any image for another specific version without changing the standard setup, you can:
# - use the IMG_VERSION as arg of the specific image build and push targets (e.g make IMG_VERSION=1.1.0 bundle-build bundle-push)
# - use environment variables to overwrite this value (e.g export IMG_VERSION=1.1.0)
IMG_VERSION ?= 1.1.0

# Validate IMG_VERSION is valid semver (Major.Minor.Patch), fallback to default if not.
ifneq ($(shell echo '$(IMG_VERSION)' | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$$' && echo valid),valid)
$(error IMG_VERSION '$(IMG_VERSION)' is not valid semver (expected: Major.Minor.Patch))
endif

# EXTERNAL_SECRETS_VERSION defines the external-secrets release version to fetch helm charts.
EXTERNAL_SECRETS_VERSION ?= v0.20.4

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
BUNDLE_CHANNELS ?=
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
BUNDLE_DEFAULT_CHANNEL ?=
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
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(IMG_VERSION)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(IMG_VERSION) $(BUNDLE_METADATA_OPTS)

# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
	BUNDLE_GEN_FLAGS += --use-image-digests
endif

# IMG is the image URL used for building/pushing image targets.
# Default tag is 'latest' to avoid unnecessary changes in checked-in manifests.
# Override with a specific version when building release images (e.g., IMG=openshift.io/external-secrets-operator:v1.1.0).
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

# GO_PACKAGE is the Go module path (used for ldflags to embed version info).
GO_PACKAGE ?= github.com/openshift/external-secrets-operator

# Version information for ldflags injection.
SOURCE_GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null)
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Extract major/minor from IMG_VERSION (e.g., 1.1.0 -> major=1, minor=1)
IMG_VERSION_MAJOR = $(word 1,$(subst ., ,$(IMG_VERSION)))
IMG_VERSION_MINOR = $(word 2,$(subst ., ,$(IMG_VERSION)))

GOBUILD_VERSION_ARGS = -ldflags " \
	-X $(GO_PACKAGE)/pkg/version.commitFromGit=$(SOURCE_GIT_COMMIT) \
	-X $(GO_PACKAGE)/pkg/version.versionFromGit=v$(IMG_VERSION) \
	-X $(GO_PACKAGE)/pkg/version.majorFromGit=$(IMG_VERSION_MAJOR) \
	-X $(GO_PACKAGE)/pkg/version.minorFromGit=$(IMG_VERSION_MINOR) \
	-X $(GO_PACKAGE)/pkg/version.buildDate=$(BUILD_DATE) \
	"

# Location to install dependencies to.
LOCALBIN ?= $(PROJECT_ROOT)/bin

# Location to store temp outputs.
OUTPUTS_PATH ?= $(PROJECT_ROOT)/_output

# Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
YQ = $(LOCALBIN)/yq
HELM ?= $(LOCALBIN)/helm
OPM = $(LOCALBIN)/opm
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
REFERENCE_DOC_GENERATOR ?= $(LOCALBIN)/crd-ref-docs
GOVULNCHECK ?= $(LOCALBIN)/govulncheck
GINKGO ?= $(LOCALBIN)/ginkgo
KUBE_API_LINT = $(LOCALBIN)/kube-api-linter.so

# Tool Versions
# Set the Operator SDK version to use. By default, what is installed on the system is used.
# This is useful for CI or a project to utilize a specific version of the operator-sdk toolkit.
OPERATOR_SDK_VERSION ?= v1.39.0
YQ_VERSION = v4.50.1
HELM_VERSION ?= v3.17.3

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
manifests: $(CONTROLLER_GEN) ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: $(CONTROLLER_GEN) ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Targets that need Go workspace mode (CI sets GOFLAGS=-mod=vendor which conflicts with go.work)
fmt vet test test-unit test-e2e run update-vendor update-dep: GOFLAGS=

.PHONY: fmt
fmt: ## Run go fmt against code.
	@echo "Running go formatter..."
	@go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	@echo "Running go vet..."
	@go vet ./...

.PHONY: test
test: $(ENVTEST) manifests generate fmt vet test-apis test-unit ## Run all tests.

.PHONY: test-unit
test-unit: vet ## Run unit tests.
	@echo "Running go unit tests..."
	go test $$(go list ./... | grep -vE 'test/(e2e|apis|utils)') -coverprofile cover.out

# E2E_TIMEOUT is the timeout for e2e tests.
E2E_TIMEOUT ?= 1h
# E2E_GINKGO_LABEL_FILTER is ginkgo label query for selecting tests. See
# https://onsi.github.io/ginkgo/#spec-labels. The default is to run tests on the AWS platform.
E2E_GINKGO_LABEL_FILTER ?= "Platform: isSubsetOf {AWS}"
.PHONY: test-e2e
test-e2e: ## Run e2e tests against a cluster.
	@echo "Running go e2e tests..."
	@go test -C $(PROJECT_ROOT)/test \
		-timeout $(E2E_TIMEOUT) \
		-count 1 -v -p 1 \
		-tags e2e ./e2e \
		-ginkgo.v \
		-ginkgo.show-node-events \
		-ginkgo.label-filter=$(E2E_GINKGO_LABEL_FILTER)

.PHONY: test-apis
test-apis: $(ENVTEST) $(GINKGO) ## Run API integration tests.
	@echo "Running API unit tests..."
	@KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" ./hack/test-apis.sh

.PHONY: lint
lint: $(GOLANGCI_LINT) $(KUBE_API_LINT) ## Run golangci-lint linter.
	@echo "Running go linter..."
	@$(GOLANGCI_LINT) run --verbose --config .golangci.yml

.PHONY: lint-fix
lint-fix: $(GOLANGCI_LINT) ## Run golangci-lint linter and perform fixes.
	@echo "Running go linter with auto-fix..."
	@$(GOLANGCI_LINT) run --verbose --fix --config .golangci.yml

##@ Build

.PHONY: build-operator
build-operator: ## Build operator binary, no additional checks or code generation.
	@echo "Building operator..."
	@GOFLAGS="-mod=vendor" source hack/go-fips.sh && \
	go build $(GOBUILD_VERSION_ARGS) -o $(LOCALBIN)/external-secrets-operator cmd/external-secrets-operator/main.go

.PHONY: build
build: manifests generate fmt vet build-operator ## Build manager binary.

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	@echo "Starting operator in local env..."
	@go run ./cmd/external-secrets-operator/main.go --v=5 --metrics-secure=false

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: image-build
image-build: ## Build operator image.
	@echo "Building operator container image ${IMG}..."
	@$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: image-push
image-push: ## Push operator image.
	@echo "Pushing operator container to image ${IMG}..."
	@$(CONTAINER_TOOL) push ${IMG}

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
build-installer: $(KUSTOMIZE) manifests generate ## Generate a consolidated YAML with CRDs and deployment.
	@echo "Generating a consolidated yaml with CRDs and resource manifests..."
	@mkdir -p dist
	@cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	@$(KUSTOMIZE) build config/default > dist/install.yaml

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: $(KUSTOMIZE) manifests ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	@echo "Installing the CRDs..."
	@$(KUSTOMIZE) build config/crd | $(KUBECTL) apply --server-side -f -

.PHONY: uninstall
uninstall: $(KUSTOMIZE) ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	@echo "Uninstalling the CRDs..."
	@$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: $(KUSTOMIZE) manifests ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	@echo "Installing the CRDs and the operator..."
	@cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	@$(KUSTOMIZE) build config/default | $(KUBECTL) apply --server-side -f -

.PHONY: undeploy
undeploy: $(KUSTOMIZE) ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	@echo "Uninstalling the CRDs and the operator..."
	@$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

##@ Dependencies

$(LOCALBIN):
	@echo "Creating $(LOCALBIN) directory..."
	@mkdir -p $(LOCALBIN)

$(OUTPUTS_PATH):
	@echo "Creating $(OUTPUTS_PATH) directory..."
	@mkdir -p $(OUTPUTS_PATH)

$(KUSTOMIZE): $(LOCALBIN) ## Build kustomize from vendor.
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5)

$(CONTROLLER_GEN): $(LOCALBIN) ## Build controller-gen from vendor.
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen)

$(ENVTEST): $(LOCALBIN) ## Build setup-envtest locally from vendor.
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest)

$(GOLANGCI_LINT): $(LOCALBIN) ## Build golangci-lint locally from vendor.
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint)

$(REFERENCE_DOC_GENERATOR): $(LOCALBIN) ## Build crd-ref-docs locally from vendor.
	$(call go-install-tool,$(REFERENCE_DOC_GENERATOR),github.com/elastic/crd-ref-docs)

$(KUBE_API_LINT): $(LOCALBIN) ## Build kube-api-linter plugin locally from vendor.
	@echo "Building kube-api-linter plugin library..."
	@go build -mod=vendor -buildmode=plugin -o $(KUBE_API_LINT) sigs.k8s.io/kube-api-linter/pkg/plugin

$(GOVULNCHECK): $(LOCALBIN) ## Build govulncheck locally from vendor.
	$(call go-install-tool,$(GOVULNCHECK),golang.org/x/vuln/cmd/govulncheck)

$(GINKGO): $(LOCALBIN) ## Build ginkgo locally from vendor.
	$(call go-install-tool,$(GINKGO),github.com/onsi/ginkgo/v2/ginkgo)

# go-install-tool will 'go install' any package with custom target and name of the binary.
# $1 - target path with name of binary
# $2 - vendor code path of the package
define go-install-tool
@{ \
bin_path=$(1); \
package=$(2); \
echo "Building $${package}..."; \
rm -f $(1) || true; \
go build -mod=vendor -o $${bin_path} $${package}; \
}
endef

$(OPERATOR_SDK): ## Download operator-sdk locally if necessary.
ifeq (,$(wildcard $(OPERATOR_SDK)))
ifeq (,$(shell which operator-sdk 2>/dev/null))
	@{ \
	mkdir -p $(dir $(OPERATOR_SDK)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPERATOR_SDK) https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$${OS}_$${ARCH} ;\
	chmod +x $(OPERATOR_SDK) ;\
	}
else
OPERATOR_SDK = $(shell which operator-sdk)
endif
endif

$(YQ): ensure-yq  ## Download yq locally if necessary.

$(HELM): ## Download helm locally if necessary.
ifeq (,$(wildcard $(HELM)))
	@{ \
	mkdir -p $(dir $(HELM)) ;\
	temp_dir=$(shell mktemp -d) && OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $${temp_dir}/helm.tar.gz https://get.helm.sh/helm-${HELM_VERSION}-$${OS}-$${ARCH}.tar.gz ;\
	tar -xf $${temp_dir}/helm.tar.gz -C $${temp_dir} ;\
	cp $${temp_dir}/$${OS}-$${ARCH}/helm $(HELM) ;\
	chmod +x $(HELM) ;\
	rm -r $${temp_dir} ;\
	}
endif

.PHONY: bundle
bundle: $(KUSTOMIZE) $(OPERATOR_SDK) manifests ## Generate bundle manifests and metadata, then validate generated files.
	@echo "Generating the bundle manifests and metadata..."
	@$(OPERATOR_SDK) generate kustomize manifests -q
	@cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	@$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	@$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	@echo "Building bundle image $(BUNDLE_IMG)..."
	@$(CONTAINER_TOOL) build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	@echo "Pushing bundle image $(BUNDLE_IMG)..."
	@$(CONTAINER_TOOL) push $(BUNDLE_IMG)

$(OPM): ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
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
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(IMG_VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: $(OPM) ## Build a catalog image.
	@echo "Building catalog image $(CATALOG_IMG)..."
	@$(OPM) index add --container-tool $(CONTAINER_TOOL) --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	@echo "Pushing catalog image $(CATALOG_IMG)..."
	@$(CONTAINER_TOOL) push $(CATALOG_IMG)

##@ Verification

.PHONY: verify
verify: vet fmt verify-deps verify-bindata verify-bindata-assets verify-generated govulncheck check-git-diff ## Verify the changes are working as expected.

.PHONY: check-git-diff
check-git-diff: update ## Check for any uncommitted changes including untracked files.
	@echo "Checking for any uncommitted changes including untracked files..."
	@./hack/check-git-diff-clean.sh

.PHONY: govulncheck
govulncheck: $(GOVULNCHECK) $(OUTPUTS_PATH) ## Run govulncheck vulnerability scan.
	@./hack/govulncheck.sh $(GOVULNCHECK) $(OUTPUTS_PATH)

##@ Maintenance

.PHONY: update
update: generate manifests update-operand-manifests update-bindata bundle docs ## Update generated code, manifests, and documentation.

.PHONY: update-operand-manifests
update-operand-manifests: $(HELM) $(YQ) ## Update external-secrets operand manifests from upstream helm charts.
	@echo "Updating external-secrets operand manifests..."
	@hack/update-external-secrets-manifests.sh $(EXTERNAL_SECRETS_VERSION)

.PHONY: update-vendor
update-vendor: ## Update vendor directory for all modules in the workspace.
	@echo "Updating vendor directory for all modules..."
	@go mod tidy -C $(PROJECT_ROOT)/cmd/external-secrets-operator
	@go mod tidy -C $(PROJECT_ROOT)/test
	@go mod tidy -C $(PROJECT_ROOT)/tools
	@go mod tidy
	@go work sync
	@go work vendor

PKG ?=
.PHONY: update-dep
update-dep: ## Update a dependency across all modules. Usage: make update-dep PKG=k8s.io/api@v0.35.0
	@if [ -z "$(PKG)" ]; then echo "Usage: make update-dep PKG=package@version"; exit 1; fi
	@echo "Updating $(PKG) in main module..."
	@go get $(PKG)
	@echo "Updating $(PKG) in cmd module..."
	@-cd cmd/external-secrets-operator && go get $(PKG)
	@echo "Updating $(PKG) in tools module..."
	@-cd tools && go get $(PKG)
	@echo "Updating $(PKG) in test module..."
	@-cd test && go get $(PKG)
	@echo "Running update-vendor..."
	@$(MAKE) update-vendor

.PHONY: verify-deps
verify-deps: ## Verify go.mod dependencies are consistent.
	@echo "Verifying go.mod dependencies..."
	@hack/verify-deps.sh

.PHONY: docs
docs: $(REFERENCE_DOC_GENERATOR) ## Generate API reference documentation.
	@echo "Generating API doc..."
	@$(REFERENCE_DOC_GENERATOR) --source-path=api/v1alpha1/ --renderer=markdown --config=hack/docs/config.yaml --output-path=docs/api_reference.md

.PHONY: clean
clean: ## Clean up generated files and directories.
	@echo "Cleaning up make generated files...."
	@rm -rf $(LOCALBIN) $(OUTPUTS_PATH) cover.out dist
