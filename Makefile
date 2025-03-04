# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
REPO := github.com/kubeflow/trainer
TRAINER_CHART_DIR := $(PROJECT_DIR)/charts/kubeflow-trainer

# Location to install tool binaries
LOCALBIN ?= $(PROJECT_DIR)/bin

# Tool versions
CONTROLLER_GEN_VERSION ?= v0.17.2
ENVTEST_VERSION ?= release-0.20
ENVTEST_K8S_VERSION ?= 1.32
HELM_VERSION ?= v3.15.3
HELM_UNITTEST_VERSION ?= 0.5.1
HELM_CHART_TESTING_VERSION ?= v3.12.0
HELM_DOCS_VERSION ?= v1.14.2
YQ_VERSION ?= v4.45.1

# Tool binaries
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
HELM ?= $(LOCALBIN)/helm
HELM_DOCS ?= $(LOCALBIN)/helm-docs
YQ ?= $(LOCALBIN)/yq

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

# Download external CRDs for Go integration testings.
EXTERNAL_CRDS_DIR ?= $(PROJECT_DIR)/manifests/external-crds

JOBSET_ROOT = $(shell go list -m -mod=readonly -f "{{.Dir}}" sigs.k8s.io/jobset)
.PHONY: jobset-operator-crd
jobset-operator-crd: ## Copy the CRDs from the JobSet repository to the manifests/external-crds directory.
	mkdir -p $(EXTERNAL_CRDS_DIR)/jobset-operator/
	cp -f $(JOBSET_ROOT)/config/components/crd/bases/* $(EXTERNAL_CRDS_DIR)/jobset-operator/

SCHEDULER_PLUGINS_ROOT = $(shell go list -m -f "{{.Dir}}" sigs.k8s.io/scheduler-plugins)
.PHONY: scheduler-plugins-crd
scheduler-plugins-crd: ## Copy the CRDs from the Scheduler Plugins repository to the manifests/external-crds directory.
	mkdir -p $(EXTERNAL_CRDS_DIR)/scheduler-plugins/
	cp -f $(SCHEDULER_PLUGINS_ROOT)/manifests/coscheduling/* $(EXTERNAL_CRDS_DIR)/scheduler-plugins

# Instructions for code generation.
.PHONY: manifests
manifests: controller-gen ## Generate manifests.
	# Skip outputing the RBAC and webhook manifests as we will sync them from the manifests templated by the Helm chart.
	$(CONTROLLER_GEN) "crd:generateEmbeddedObjectMeta=true" rbac:roleName=kubeflow-trainer-controller-manager webhook \
		paths="./pkg/apis/trainer/v1alpha1/...;./pkg/controller/...;./pkg/runtime/...;./pkg/webhooks/...;./pkg/util/cert/..." \
		output:crd:artifacts:config=manifests/base/crds \
		output:rbac:artifacts:config=manifests/base/rbac \
		output:rbac:none \
		output:webhook:artifacts:config=manifests/base/webhook \
		output:webhook:none

.PHONY: generate
generate: go-mod-download manifests sync-manifests ## Generate APIs and SDK.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate/boilerplate.go.txt" paths="./pkg/apis/..."
	hack/update-codegen.sh
	hack/python-sdk/gen-sdk.sh

.PHONY: go-mod-download
go-mod-download: ## Run go mod download to download modules.
	go mod download

# Instructions for code formatting.
.PHONY: fmt
fmt: ## Run go fmt against the code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against the code.
	go vet ./...

GOLANGCI_LINT=$(shell which golangci-lint)
.PHONY: golangci-lint
golangci-lint: ## Run golangci-lint to verify Go files.
ifeq ($(GOLANGCI_LINT),)
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.61.0
	$(info golangci-lint has been installed)
endif
	golangci-lint run --timeout 5m --go 1.23 ./...

# Instructions to run tests.
.PHONY: test
test: ## Run Go unit test.
	go test $(shell go list ./... | grep -v '/test/') -coverprofile cover.out

.PHONY: test-integration
test-integration: envtest jobset-operator-crd scheduler-plugins-crd ## Run Go integration test.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./test/... -coverprofile cover.out

test-python: ## Run Python unit test.
	export PYTHONPATH=$(PROJECT_DIR)
	pip install pytest
	pip install -r ./cmd/initializer/dataset/requirements.txt
	pip install ./sdk

	pytest ./pkg/initializer/dataset
	pytest ./pkg/initializer/model
	pytest ./pkg/initializer/utils

test-python-integration: ## Run Python integration test.
	export PYTHONPATH=$(PROJECT_DIR)
	pip install pytest
	pip install -r ./cmd/initializer/dataset/requirements.txt

	pytest ./test/integration/initializer

##@ Helm

.PHONY: sync-manifests
sync-manifests: helm yq ## Sync Kustomize manifests from manifests templated by Helm chart.
	HELM=$(HELM) RELEASE_NAME=$(RELEASE_NAME) RELEASE_NAMESPACE=$(RELEASE_NAMESPACE) YQ=$(YQ) hack/sync-manifests.sh

.PHONY: helm-unittest
helm-unittest: helm-unittest-plugin ## Run Helm chart unittests.
	$(HELM) unittest $(TRAINER_CHART_DIR) --strict --file "tests/**/*_test.yaml"

.PHONY: helm-lint
helm-lint: ## Run Helm chart lint test.
	$(CONTAINER_TOOL) run --rm --workdir /workspace --volume "$$(pwd):/workspace" quay.io/helmpack/chart-testing:$(HELM_CHART_TESTING_VERSION) ct lint --target-branch master --validate-maintainers=false

.PHONY: helm-docs
helm-docs: helm-docs-plugin ## Generates markdown documentation for helm charts from requirements and values files.
	$(HELM_DOCS) --sort-values-order=file

##@ Dependencies

$(LOCALBIN): ## Create the bin directory if it does not exist.
	mkdir -p $(LOCALBIN)

.PHONY: envtest
envtest: $(ENVTEST) ## Download the setup-envtest binary if necessary.
$(ENVTEST): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download the controller-gen binary if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

.PHONY: helm
helm: $(HELM) ## Download helm locally if necessary.
$(HELM): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install helm.sh/helm/v3/cmd/helm@$(HELM_VERSION)

.PHONY: helm-unittest-plugin
helm-unittest-plugin: helm ## Download helm unittest plugin locally if necessary.
	if [ -z "$(shell $(HELM) plugin list | grep unittest)" ]; then \
		echo "Installing helm unittest plugin"; \
		$(HELM) plugin install https://github.com/helm-unittest/helm-unittest.git --version $(HELM_UNITTEST_VERSION); \
	fi

.PHONY: helm-docs-plugin
helm-docs-plugin: $(HELM_DOCS) ## Download helm-docs plugin locally if necessary.
$(HELM_DOCS): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/norwoodj/helm-docs/cmd/helm-docs@$(HELM_DOCS_VERSION)

.PHONY: yq
yq: $(YQ) ## Download yq locally if necessary.
$(YQ): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/mikefarah/yq/v4@$(YQ_VERSION)
