# Image URL to use all building/pushing image targets
HUB ?= ghcr.io/matrixinfer-ai
TAG ?= latest
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
	$(CONTROLLER_GEN) crd paths="./pkg/apis/networking/..." output:crd:artifacts:config=charts/matrixinfer/charts/networking/crds
	$(CONTROLLER_GEN) crd paths="./pkg/apis/workload/..." output:crd:artifacts:config=charts/matrixinfer/charts/workload/crds
	$(CONTROLLER_GEN) crd paths="./pkg/apis/registry/..." output:crd:artifacts:config=charts/matrixinfer/charts/registry/crds

.PHONY: gen-docs
gen-docs: crd-ref-docs ## Generate CRD reference documentation
	mkdir -p docs/matrixinfer/docs/api
	$(CRD_REF_DOCS) \
		--source-path=./pkg/apis \
		--config=docs/matrixinfer/crd-ref-docs-config.yaml \
		--output-path=docs/matrixinfer/docs/reference/crd \
		--renderer=markdown \
		--output-mode=group

.PHONY: generate
generate: controller-gen gen-crd gen-docs ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
	go mod tidy
	./hack/update-codegen.sh

.PHONY: gen-check
gen-check: generate
	git diff --exit-code

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: generate fmt vet envtest ## Run tests. Exclude e2e, client-go.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /e2e | grep -v /client-go) -coverprofile cover.out

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
build: generate fmt vet
	go build -o bin/infer-controller cmd/infer-controller/main.go
	go build -o bin/infer-gateway cmd/infer-gateway/main.go
	go build -o bin/model-controller cmd/model-controller/main.go
	go build -o bin/autoscaler cmd/autoscaler/main.go
	go build -o bin/registry-webhook cmd/registry-webhook/main.go
	go build -o bin/infer-webhook cmd/modelinfer-webhook/main.go
	go build -o bin/infer-gateway-webhook cmd/infer-gateway-webhook/main.go

IMG_MODELINFER ?= ${HUB}/infer-controller:${TAG}
IMG_MODELCONTROLLER ?= ${HUB}/model-controller:${TAG}
IMG_AUTOSCALER ?= ${HUB}/autoscaler:${TAG}
IMG_GATEWAY ?= ${HUB}/infer-gateway:${TAG}
IMG_REGISTRY_WEBHOOK ?= ${HUB}/registry-webhook:${TAG}
IMG_MODELINFER_WEBHOOK ?= ${HUB}/modelinfer-webhook:${TAG}
IMG_INFER_GATEWAY_WEBHOOK ?= ${HUB}/infer-gateway-webhook:${TAG}

.PHONY: docker-build-gateway
docker-build-gateway: generate
	$(CONTAINER_TOOL) build -t ${IMG_GATEWAY} -f docker/Dockerfile.gateway .

.PHONY: docker-build-modelinfer
docker-build-modelinfer: generate 
	$(CONTAINER_TOOL) build -t ${IMG_MODELINFER} -f docker/Dockerfile.modelinfer .

.PHONY: docker-build-modelcontroller
docker-build-modelcontroller: generate
	$(CONTAINER_TOOL) build -t ${IMG_MODELCONTROLLER} -f docker/Dockerfile.modelcontroller .

.PHONY: docker-build-autoscaler
docker-build-autoscaler: generate
	$(CONTAINER_TOOL) build -t ${IMG_AUTOSCALER} -f docker/Dockerfile.autoscaler .

.PHONY: docker-build-registry-webhook
docker-build-registry-webhook: generate
	$(CONTAINER_TOOL) build -t ${IMG_REGISTRY_WEBHOOK} -f docker/Dockerfile.registry.webhook .

.PHONY: docker-build-modelinfer-webhook
docker-build-modelinfer-webhook: generate
	$(CONTAINER_TOOL) build -t ${IMG_MODELINFER_WEBHOOK} -f docker/Dockerfile.modelinfer.webhook .

.PHONY: docker-build-infer-gateway-webhook
docker-build-infer-gateway-webhook: generate
	$(CONTAINER_TOOL) build -t ${IMG_INFER_GATEWAY_WEBHOOK} -f docker/Dockerfile.infergateway.webhook .

.PHONY: docker-push
docker-push: docker-build-gateway docker-build-modelinfer docker-build-modelcontroller docker-build-registry-webhook docker-build-modelinfer-webhook docker-build-autoscaler docker-build-infer-gateway-webhook ## Push all images to the registry.
	$(CONTAINER_TOOL) push ${IMG_GATEWAY}
	$(CONTAINER_TOOL) push ${IMG_MODELINFER}
	$(CONTAINER_TOOL) push ${IMG_MODELCONTROLLER}
	$(CONTAINER_TOOL) push ${IMG_AUTOSCALER}
	$(CONTAINER_TOOL) push ${IMG_REGISTRY_WEBHOOK}
	$(CONTAINER_TOOL) push ${IMG_MODELINFER_WEBHOOK}
	$(CONTAINER_TOOL) push ${IMG_INFER_GATEWAY_WEBHOOK}

# PLATFORMS defines the target platforms for the images be built to provide support to multiple
# architectures.
PLATFORMS ?= linux/arm64,linux/amd64

# Make sure Buildx is set up:
#   docker buildx create --name mybuilder --driver docker-container --use
#   docker buildx inspect --bootstrap

.PHONY: docker-buildx
docker-buildx: ## Build docker image for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	find docker -name "Dockerfile.*" -type f -exec sh -c 'sed "s/FROM \(golang:[^[:space:]]\+\) AS builder/FROM --platform=\$$BUILDPLATFORM \1 AS builder/g" "$$1" > "$$1.cross"' _ {} \;
	-$(CONTAINER_TOOL) buildx create --name matrixinfer-builder
	$(CONTAINER_TOOL) buildx use matrixinfer-builder
	-$(CONTAINER_TOOL) buildx build --platform ${PLATFORMS} --tag ${IMG_GATEWAY} -f docker/Dockerfile.gateway.cross .
	-$(CONTAINER_TOOL) buildx build --platform ${PLATFORMS} --tag ${IMG_MODELINFER} -f docker/Dockerfile.modelinfer.cross .
	-$(CONTAINER_TOOL) buildx build --platform ${PLATFORMS} --tag ${IMG_MODELCONTROLLER} -f docker/Dockerfile.modelcontroller.cross .
	-$(CONTAINER_TOOL) buildx build --platform ${PLATFORMS} --tag ${IMG_AUTOSCALER} -f docker/Dockerfile.autoscaler.cross .
	-$(CONTAINER_TOOL) buildx build --platform ${PLATFORMS} --tag ${IMG_REGISTRY_WEBHOOK} -f docker/Dockerfile.registry.webhook.cross .
	-$(CONTAINER_TOOL) buildx build --platform ${PLATFORMS} --tag ${IMG_MODELINFER_WEBHOOK} -f docker/Dockerfile.modelinfer.webhook.cross .
	-$(CONTAINER_TOOL) buildx build --platform ${PLATFORMS} --tag ${IMG_INFER_GATEWAY_WEBHOOK} -f docker/Dockerfile.infergateway.webhook.cross .
	-$(CONTAINER_TOOL) buildx rm matrixinfer-builder
	find docker -name "*.cross" -type f -delete

.PHONY: docker-buildx-push
docker-buildx-push: docker-buildx ## Build and push docker image for cross-platform support
	$(CONTAINER_TOOL) push ${IMG_GATEWAY}
	$(CONTAINER_TOOL) push ${IMG_MODELINFER}
	$(CONTAINER_TOOL) push ${IMG_MODELCONTROLLER}
	$(CONTAINER_TOOL) push ${IMG_AUTOSCALER}
	$(CONTAINER_TOOL) push ${IMG_REGISTRY_WEBHOOK}
	$(CONTAINER_TOOL) push ${IMG_MODELINFER_WEBHOOK}
	$(CONTAINER_TOOL) push ${IMG_INFER_GATEWAY_WEBHOOK}

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

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
CRD_REF_DOCS ?= $(LOCALBIN)/crd-ref-docs

## Tool Versions
KUSTOMIZE_VERSION ?= v5.5.0
CONTROLLER_TOOLS_VERSION ?= v0.17.2
ENVTEST_VERSION ?= release-0.19
GOLANGCI_LINT_VERSION ?= v1.64.8
CRD_REF_DOCS_VERSION ?= v0.2.0

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

.PHONY: crd-ref-docs
crd-ref-docs: $(CRD_REF_DOCS) ## Download crd-ref-docs locally if necessary.
$(CRD_REF_DOCS): $(LOCALBIN)
	$(call go-install-tool,$(CRD_REF_DOCS),github.com/elastic/crd-ref-docs,$(CRD_REF_DOCS_VERSION))

.PHONY: add-copyright
add-copyright:
	@echo "Adding copyright headers..."
	@hack/update-copyright.sh

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
