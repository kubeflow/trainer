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

K8S_VERSION ?= 1.32.0

PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))

# Tool Binaries
LOCALBIN ?= $(PROJECT_DIR)/bin

GINKGO ?= $(LOCALBIN)/ginkgo
ENVTEST ?= $(LOCALBIN)/setup-envtest
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
KIND ?= $(LOCALBIN)/kind

# Instructions to download tools for development.

.PHONY: ginkgo
ginkgo: ## Download the ginkgo binary if required.
	GOBIN=$(LOCALBIN) go install github.com/onsi/ginkgo/v2/ginkgo@$(shell go list -m -f '{{.Version}}' github.com/onsi/ginkgo/v2)

.PHONY: envtest
envtest: ## Download the setup-envtest binary if required.
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@release-0.20

.PHONY: controller-gen
controller-gen: ## Download the controller-gen binary if required.
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.17.2

.PHONY: kind
kind: ## Download Kind binary if required.
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/kind@$(shell go list -m -f '{{.Version}}' sigs.k8s.io/kind)

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
	$(CONTROLLER_GEN) "crd:generateEmbeddedObjectMeta=true" rbac:roleName=kubeflow-trainer-controller-manager webhook \
		paths="./pkg/apis/trainer/v1alpha1/...;./pkg/controller/...;./pkg/runtime/...;./pkg/webhooks/...;./pkg/util/cert/..." \
		output:crd:artifacts:config=manifests/base/crds \
		output:rbac:artifacts:config=manifests/base/rbac \
		output:webhook:artifacts:config=manifests/base/webhook

.PHONY: generate
generate: go-mod-download manifests ## Generate APIs and SDK.
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
test-integration: ginkgo envtest jobset-operator-crd scheduler-plugins-crd ## Run Go integration test.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(K8S_VERSION) -p path)" $(GINKGO) -v ./test/integration/... -coverprofile cover.out

.PHONY: test-python
test-python: ## Run Python unit test.
	export PYTHONPATH=$(PROJECT_DIR)
	pip install pytest
	pip install -r ./cmd/initializer/dataset/requirements.txt
	pip install ./sdk

	pytest ./pkg/initializer/dataset
	pytest ./pkg/initializer/model
	pytest ./pkg/initializer/utils

.PHONY: test-python-integration
test-python-integration: ## Run Python integration test.
	export PYTHONPATH=$(PROJECT_DIR)
	pip install pytest
	pip install -r ./cmd/initializer/dataset/requirements.txt

	pytest ./test/integration/initializer

.PHONY: test-e2e-setup-cluster
test-e2e-setup-cluster: kind ## Setup Kind cluster for e2e test.
	KIND=$(KIND) K8S_VERSION=$(K8S_VERSION) ./hack/e2e-setup-cluster.sh

.PHONY: test-e2e
test-e2e: ginkgo ## Run Go e2e test.
	$(GINKGO) -v ./test/e2e/...

# Input and output location for Notebooks executed with Papermill.
NOTEBOOK_INPUT=$(PROJECT_DIR)/examples/pytorch/image-classification/mnist.ipynb
NOTEBOOK_OUTPUT=$(PROJECT_DIR)/artifacts/notebooks/trainer_output.ipynb
PAPERMILL_TIMEOUT=900
.PHONY: test-e2e-notebook
test-e2e-notebook: ## Run Jupyter Notebook with Papermill.
	NOTEBOOK_INPUT=$(NOTEBOOK_INPUT) NOTEBOOK_OUTPUT=$(NOTEBOOK_OUTPUT) PAPERMILL_TIMEOUT=$(PAPERMILL_TIMEOUT) ./hack/e2e-run-notebook.sh
