# Image URL to use all building/pushing image targets
IMG_GATEWAY ?= ghcr.io/matrixinfer-ai/infer-gateway:latest
IMG_MODELINFER ?= ghcr.io/matrixinfer-ai/infer-controller:latest
IMG_MODEL_CONTROLLER ?= ghcr.io/matrixinfer-ai/model-controller:latest
IMG_REGISTRY_WEBHOOK ?= ghcr.io/matrixinfer-ai/registry-webhook:latest
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.31.0
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))


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
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

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

.PHONY: gen-crd
gen-crd: controller-gen 
	$(CONTROLLER_GEN) crd paths="./pkg/apis/networking/..." output:crd:artifacts:config=manifests/crd/infer-gateway
	$(CONTROLLER_GEN) crd paths="./pkg/apis/workload/..." output:crd:artifacts:config=manifests/crd/infer-controller
	$(CONTROLLER_GEN) crd paths="./pkg/apis/registry/..." output:crd:artifacts:config=manifests/crd/model-controller
.PHONY: generate
generate: controller-gen code-generator gen-crd ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
	./hack/update-codegen.sh

# Use same code-generator version as k8s.io/api
CODEGEN_VERSION := $(shell go list -m -f '{{.Version}}' k8s.io/api)
CODEGEN = $(shell pwd)/bin/code-generator
CODEGEN_ROOT = $(shell go env GOMODCACHE)/k8s.io/code-generator@$(CODEGEN_VERSION)
.PHONY: code-generator
code-generator:
	@GOBIN=$(PROJECT_DIR)/bin GO111MODULE=on go install k8s.io/code-generator/cmd/client-gen@$(CODEGEN_VERSION)
	cp -f $(CODEGEN_ROOT)/generate-groups.sh $(PROJECT_DIR)/bin/
	cp -f $(CODEGEN_ROOT)/generate-internal-groups.sh $(PROJECT_DIR)/bin/
	cp -f $(CODEGEN_ROOT)/kube_codegen.sh $(PROJECT_DIR)/bin/

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

# TODO(user): To use a different vendor for e2e tests, modify the setup under 'tests/e2e'.
# The default setup assumes Kind is pre-installed and builds/loads the Manager Docker image locally.
# Prometheus and CertManager are installed by default; skip with:
# - PROMETHEUS_INSTALL_SKIP=true
# - CERT_MANAGER_INSTALL_SKIP=true
.PHONY: test-e2e
test-e2e: generate fmt vet ## Run the e2e tests. Expected an isolated environment using Kind.
	@command -v kind >/dev/null 2>&1 || { \
		echo "Kind is not installed. Please install Kind manually."; \
		exit 1; \
	}
	@kind get clusters | grep -q 'kind' || { \
		echo "No Kind cluster is running. Please start a Kind cluster before running the e2e tests."; \
		exit 1; \
	}
	go test ./test/e2e/ -v -ginkgo.v

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

##@ Build

.PHONY: build
build: generate fmt vet ## Build ai-gateway binary.
	go build -o bin/infer-gateway cmd/infer-gateway/main.go
	go build -o bin/registry-webhook cmd/registry-webhook/main.go

.PHONY: build-model-controller
build-model-controller: generate fmt vet
	go build -o bin/model-controller cmd/model-controller/main.go

.PHONY: build-registry-webhook
build-registry-webhook: generate fmt vet
	go build -o bin/registry-webhook cmd/registry-webhook/main.go

# If you wish to build the ai-gateway image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build-gateway
docker-build-gateway: generate## Build docker image with the ai-gateway.
	$(CONTAINER_TOOL) build -t ${IMG_GATEWAY} -f Dockerfile-gateway .

.PHONY: docker-build-modelinfer
docker-build-modelinfer: generate## Build docker image with the ai-gateway.
	$(CONTAINER_TOOL) build -t ${IMG_MODELINFER} -f Dockerfile-modelinfer .

.PHONY: docker-build-model-controller
docker-build-model-controller: build-model-controller## Build docker image with the model controller.
	$(CONTAINER_TOOL) build -t ${IMG_MODEL_CONTROLLER} -f Dockerfile-model-controller .

.PHONY: docker-build-registry-webhook
docker-build-registry-webhook: build-registry-webhook## Build docker image with the model webhook.
	$(CONTAINER_TOOL) build -t ${IMG_REGISTRY_WEBHOOK} -f Dockerfile-registry-webhook .

.PHONY: docker-push-gateway
docker-push-gateway: ## Push docker image with the ai-gateway.
	$(CONTAINER_TOOL) push ${IMG_GATEWAY}

.PHONY: docker-push-modelinfer
docker-push-modelinfer: ## Push docker image with the ai-gateway.
	$(CONTAINER_TOOL) push ${IMG_MODELINFER}

.PHONY: docker-push-model-controller
docker-push-model-controller: ## Push docker image with the model controller.
	$(CONTAINER_TOOL) push ${IMG_MODEL_CONTROLLER}

.PHONY: docker-push-registry-webhook
docker-push-registry-webhook: ## Push docker image with the model webhook.
	$(CONTAINER_TOOL) push ${IMG_REGISTRY_WEBHOOK}
# PLATFORMS defines the target platforms for the ai-gateway image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the ai-gateway for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name aiengine-builder
	$(CONTAINER_TOOL) buildx use aiengine-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm aiengine-builder
	rm Dockerfile.cross

.PHONY: build-installer
build-installer: generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default > dist/install.yaml

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

CERT_MANAGER_VERSION ?= v1.18.0
.PHONY: install-cert-manager
install-cert-manager:
	$(KUBECTL) apply -f https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml

.PHONY: uninstall-cert-manager
uninstall-cert-manager:
	$(KUBECTL) delete -f https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml

.PHONY: install-crd
install-crd: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build manifests/crd | $(KUBECTL) apply --server-side -f -

.PHONY: uninstall-crd
uninstall-crd: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build manifests/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy-registry-webhook
deploy-registry-webhook: kustomize ## Deploy model webhook to the K8s cluster specified in ~/.kube/config.
	cd manifests/registry-webhook && $(KUSTOMIZE) edit set image registry-webhook=${IMG_REGISTRY_WEBHOOK}
	$(KUSTOMIZE) build manifests/registry-webhook | $(KUBECTL) apply -f -

.PHONY: undeploy-registry-webhook
undeploy-registry-webhook: kustomize ## Deploy model webhook to the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build manifests/registry-webhook | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint

## Tool Versions
KUSTOMIZE_VERSION ?= v5.5.0
CONTROLLER_TOOLS_VERSION ?= v0.17.2
ENVTEST_VERSION ?= release-0.19
GOLANGCI_LINT_VERSION ?= v1.64.8

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef
