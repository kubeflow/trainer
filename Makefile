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

# Setting SED allows macOS users to install GNU sed (gsed) and use it instead
# of the default BSD sed, which the in-place edits in this Makefile require.
ifeq ($(shell command -v gsed 2>/dev/null),)
    SED ?= $(shell command -v sed)
else
    SED ?= $(shell command -v gsed)
endif
ifeq ($(shell ${SED} --version 2>&1 | grep -q GNU; echo $$?),1)
    $(error !!! GNU sed is required. If on OS X, use 'brew install gnu-sed'.)
endif

PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
REPO := github.com/kubeflow/trainer
TRAINER_CHART_DIR := $(PROJECT_DIR)/charts/kubeflow-trainer
# Year-less copyright header prepended to generated manifests (controller-gen
# emits none). Single source of truth shared with the boilerplate verifier.
BOILERPLATE_HEADER := $(PROJECT_DIR)/hack/boilerplate/boilerplate.sh.txt
HELM_BOILERPLATE_HEADER := $(PROJECT_DIR)/hack/boilerplate/boilerplate.helm.txt
# Location to install tool binaries
LOCALBIN ?= $(PROJECT_DIR)/bin

# Tool versions
K8S_VERSION ?= 1.37.0
GINKGO_VERSION ?= $(shell go list -m -f '{{.Version}}' github.com/onsi/ginkgo/v2)
ENVTEST_VERSION ?= release-0.22
CONTROLLER_GEN_VERSION ?= v0.21.0
KIND_VERSION ?= $(shell go list -m -f '{{.Version}}' sigs.k8s.io/kind)
HELM_VERSION ?= v3.18.6
HELM_UNITTEST_VERSION ?= 1.1.1
HELM_CHART_TESTING_VERSION ?= v3.12.0
HELM_DOCS_VERSION ?= v1.14.2
YQ_VERSION ?= v4.45.1
KUBE_LINTER_VERSION ?= v0.7.1
VOLCANO_VERSION ?= v1.13.1

# Container runtime (docker or podman)
CONTAINER_RUNTIME ?=

# Tool binaries
GINKGO ?= $(LOCALBIN)/ginkgo
ENVTEST ?= $(LOCALBIN)/setup-envtest
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
KIND ?= $(LOCALBIN)/kind
HELM ?= $(LOCALBIN)/helm
HELM_DOCS ?= $(LOCALBIN)/helm-docs
YQ ?= $(LOCALBIN)/yq
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
GOLANGCI_LINT_KAL ?= $(LOCALBIN)/golangci-lint-kube-api-linter
LINT_PKG ?= ./...
KUBE_LINTER ?= $(LOCALBIN)/kube-linter

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

# Instructions to download tools for development.

.PHONY: ginkgo
ginkgo: ## Download the ginkgo binary if required.
	GOBIN=$(LOCALBIN) go install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)

.PHONY: envtest
envtest: ## Download the setup-envtest binary if required.
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION)

.PHONY: controller-gen
controller-gen: ## Download the controller-gen binary if required.
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

.PHONY: kind
kind: ## Download Kind binary if required.
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/kind@$(KIND_VERSION)

.PHONY: helm
helm: ## Download helm locally if required.
	GOBIN=$(LOCALBIN) go install helm.sh/helm/v3/cmd/helm@$(HELM_VERSION)

.PHONY: golangci-lint-install
golangci-lint-install: ## Download golangci-lint locally if required.
	@GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.1

.PHONY: golangci-lint-kal
golangci-lint-kal: ## Build golangci-lint-kal from custom configuration.
	cd $(PROJECT_DIR)/hack; $(GOLANGCI_LINT) custom; mv bin/golangci-lint-kube-api-linter $(LOCALBIN)/

.PHONY: helm-unittest-plugin
helm-unittest-plugin: helm ## Download helm unittest plugin locally if required.
	if [ -z "$(shell $(HELM) plugin list | grep unittest)" ]; then \
		echo "Installing helm unittest plugin"; \
		$(HELM) plugin install https://github.com/helm-unittest/helm-unittest.git --version $(HELM_UNITTEST_VERSION); \
	fi

.PHONY: helm-docs-plugin
helm-docs-plugin: ## Download helm-docs plugin locally if required.
	GOBIN=$(LOCALBIN) go install github.com/norwoodj/helm-docs/cmd/helm-docs@$(HELM_DOCS_VERSION)

.PHONY: yq
yq: # Download yq locally if required.
	GOBIN=$(LOCALBIN) go install github.com/mikefarah/yq/v4@$(YQ_VERSION)

.PHONY: kube-linter
kube-linter: ## Download kube-linter locally if required.
	GOBIN=$(LOCALBIN) go install golang.stackrox.io/kube-linter/cmd/kube-linter@$(KUBE_LINTER_VERSION)

.PHONY: uv
uv: ## Install uv if it is not already installed.
	@command -v uv > /dev/null 2>&1 || { \
		echo "Installing uv"; \
		curl -LsSf https://astral.sh/uv/install.sh | sh; \
		echo "uv has been installed."; \
	}

.PHONY: lint-manifests
lint-manifests: kube-linter ## Run kube-linter on manifests and helm charts.
	$(KUBE_LINTER) lint manifests/base --config .kube-linter.yaml
	$(KUBE_LINTER) lint charts/kubeflow-trainer --config .kube-linter.yaml

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

VOLCANO_CRD_URL = https://raw.githubusercontent.com/volcano-sh/volcano/$(VOLCANO_VERSION)/config/crd/volcano/bases/scheduling.volcano.sh_podgroups.yaml

.PHONY: volcano-crd
volcano-crd: ## Copy the CRDs from Volcano repository to the manifests/external-crds directory.
	mkdir -p $(EXTERNAL_CRDS_DIR)/volcano/
	curl -fsSL $(VOLCANO_CRD_URL) -o $(EXTERNAL_CRDS_DIR)/volcano/scheduling.volcano.sh_podgroups.yaml

# Instructions for code generation.
.PHONY: manifests
manifests: controller-gen ## Generate manifests.
	$(CONTROLLER_GEN) "crd:generateEmbeddedObjectMeta=true,maxDescLen=128" rbac:roleName=kubeflow-trainer-controller-manager webhook \
		paths="./pkg/apis/trainer/v1alpha1/...;./pkg/controller/...;./pkg/runtime/...;./pkg/webhooks/...;./pkg/util/cert/...;./pkg/metrics/..." \
		output:crd:artifacts:config=manifests/base/crds \
		output:rbac:artifacts:config=manifests/base/rbac \
		output:webhook:artifacts:config=manifests/base/webhook
	@# controller-gen emits no license header. Prepend the year-less
	@# boilerplate to each generated manifest. controller-gen rewrites these
	@# files in full on every run, so prepending here once is idempotent.
	@# Copy the header-free CRDs into the chart before adding the kustomize (#)
	@# header, so the chart templates can use the Helm-style license block.
	cp -f manifests/base/crds/trainer.kubeflow.org_*.yaml $(TRAINER_CHART_DIR)/templates/crd/
	@for f in manifests/base/crds/trainer.kubeflow.org_*.yaml \
			manifests/base/rbac/role.yaml \
			manifests/base/webhook/manifests.yaml; do \
		{ cat $(BOILERPLATE_HEADER); echo; cat "$$f"; } > "$$f.tmp" && mv "$$f.tmp" "$$f"; \
	done
	# Prepend the Helm license block and wrap the chart CRD templates so
	# installation can be toggled via `crds.enabled`.
	for f in $(TRAINER_CHART_DIR)/templates/crd/trainer.kubeflow.org_*.yaml; do \
		{ cat $(HELM_BOILERPLATE_HEADER); echo; echo '{{- if .Values.crds.enabled }}'; cat $$f; echo '{{- end }}'; } > $$f.tmp && mv $$f.tmp $$f; \
	done

.PHONY: generate
generate: go-mod-download manifests helm-docs ## Generate APIs.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate/boilerplate.go.txt" paths="./pkg/apis/..."
	hack/update-codegen.sh
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate/boilerplate.go.txt" paths="./pkg/apis/config/v1alpha1/..."
	CONTAINER_RUNTIME=$(CONTAINER_RUNTIME) hack/python-api/gen-api.sh

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

.PHONY: golangci-lint
golangci-lint: golangci-lint-install golangci-lint-kal ## Run golangci-lint to verify Go files.
	$(GOLANGCI_LINT) run --timeout 5m $(LINT_PKG)
	$(GOLANGCI_LINT_KAL) run -v --config $(PROJECT_DIR)/.golangci-kal.yml

.PHONY: verify-boilerplate
verify-boilerplate: ## Verify copyright boilerplate headers in source files.
	python3 hack/boilerplate/boilerplate.py --base-ref "$(TARGET_BRANCH)"

# Instructions to run tests.
.PHONY: test
test: ## Run Go unit test.
	go test $(shell go list ./... | grep -Ev '/(test|cmd|hack|pkg/apis|pkg/client|pkg/util/testing)') -coverprofile cover.out

.PHONY: test-integration
test-integration: ginkgo envtest jobset-operator-crd scheduler-plugins-crd volcano-crd ## Run Go integration test.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(K8S_VERSION) -p path)" $(GINKGO) -v ./test/integration/...

.PHONY: test-python
test-python: ## Run Python unit test.
	uv sync --locked --no-dev --directory ./cmd/initializers/dataset
	uv sync --locked --no-dev --directory ./cmd/initializers/model

	PYTHONPATH=$(PROJECT_DIR) uv run --with pytest --directory ./cmd/initializers/dataset pytest $(PROJECT_DIR)/pkg/initializers/dataset
	PYTHONPATH=$(PROJECT_DIR) uv run --with pytest --directory ./cmd/initializers/dataset pytest $(PROJECT_DIR)/pkg/initializers/model
	PYTHONPATH=$(PROJECT_DIR) uv run --with pytest --directory ./cmd/initializers/dataset pytest $(PROJECT_DIR)/pkg/initializers/utils

.PHONY: test-python-integration
test-python-integration: ## Run Python integration test.
	uv sync --locked --no-dev --directory ./cmd/initializers/dataset

	PYTHONPATH=$(PROJECT_DIR) uv run --with pytest --directory ./cmd/initializers/dataset pytest $(PROJECT_DIR)/test/integration/initializers

.PHONY: test-rust
test-rust: ## Run Rust unit test.
	cargo test --lib --bins --manifest-path ./pkg/data_cache/Cargo.toml

.PHONY: test-e2e-setup-cluster
test-e2e-setup-cluster: kind ## Setup Kind cluster for e2e test. (Set GPU_CLUSTER=gpu for GPU nodes)
	CLUSTER_TYPE=$(CLUSTER_TYPE) KIND=$(KIND) K8S_VERSION=$(K8S_VERSION) INSTALL_METHOD=$(INSTALL_METHOD) ./hack/e2e-setup-cluster.sh

.PHONY: test-e2e
test-e2e: ginkgo ## Run Go e2e test.
	$(GINKGO) -v ./test/e2e/...

# Input and output location for Notebooks executed with Papermill.
NOTEBOOK_INPUT=$(PROJECT_DIR)/examples/pytorch/image-classification/mnist.ipynb
NOTEBOOK_OUTPUT=$(PROJECT_DIR)/artifacts/notebooks/trainer_output.ipynb
PAPERMILL_PARAMS=
PAPERMILL_TIMEOUT=900
.PHONY: test-e2e-notebook
test-e2e-notebook: ## Run Jupyter Notebook with Papermill.
	NOTEBOOK_INPUT=$(NOTEBOOK_INPUT) NOTEBOOK_OUTPUT=$(NOTEBOOK_OUTPUT) PAPERMILL_PARAMS="$(PAPERMILL_PARAMS)" PAPERMILL_TIMEOUT=$(PAPERMILL_TIMEOUT) ./hack/e2e-run-notebook.sh

##@ Documentation

.PHONY: docs
docs: ## Build HTML documentation locally
	cd docs && $(MAKE) html

.PHONY: docs-linkcheck
docs-linkcheck: ## Check all links in documentation
	cd docs && $(MAKE) linkcheck

.PHONY: docs-clean
docs-clean: ## Remove documentation build artifacts
	cd docs && $(MAKE) clean

.PHONY: docs-serve
docs-serve: ## Build and serve documentation locally with live reload
	cd docs && $(MAKE) serve

##@ Helm

TARGET_BRANCH ?= master

.PHONY: helm-unittest
helm-unittest: helm-unittest-plugin ## Run Helm chart unittests.
	$(HELM) unittest $(TRAINER_CHART_DIR) --strict --file "tests/**/*_test.yaml"

.PHONY: helm-lint
helm-lint: ## Run Helm chart lint test.
	docker run --rm --workdir /workspace --user "$(shell id -u):$(shell id -g)" --volume "$$(pwd):/workspace" quay.io/helmpack/chart-testing:$(HELM_CHART_TESTING_VERSION) ct lint --target-branch $(TARGET_BRANCH) --validate-maintainers=false --check-version-increment=false

.PHONY: helm-docs
helm-docs: helm-docs-plugin ## Generates markdown documentation for helm charts from requirements and values files.
	$(HELM_DOCS) --sort-values-order=file

##@ Release

# Release version, including the leading "v" (vX.Y.Z or vX.Y.Z-rc.N).
VERSION ?=
GITHUB_TOKEN ?=

.PHONY: release
release: ## Create a release commit. Usage: make release VERSION=vX.Y.Z [GITHUB_TOKEN=<token>]
	@if [ -z "$(VERSION)" ] || ! echo "$(VERSION)" | grep -E -q '^v[0-9]+\.[0-9]+\.[0-9]+(-rc\.[0-9]+)?$$'; then \
		echo "Error: VERSION must be set in vX.Y.Z or vX.Y.Z-rc.N format. Usage: make release VERSION=vX.Y.Z[-rc.N]"; \
		exit 1; \
	fi
	@echo -n "$(VERSION)" > VERSION
	@echo "Updated VERSION to $(VERSION)"
	@CHART_VERSION=$$(echo "$(VERSION)" | $(SED) 's/^v//'); \
		$(SED) -i "s/^version: .*/version: $$CHART_VERSION/" charts/kubeflow-trainer/Chart.yaml; \
		echo "Updated Helm chart version to $$CHART_VERSION"
	@if echo "$(VERSION)" | grep -E -q '\-rc\.[0-9]+$$'; then \
		echo "Skipping changelog generation for RC release $(VERSION)"; \
	else \
		git fetch upstream --tags --prune; \
		MAJOR_MINOR=$$(echo "$(VERSION)" | $(SED) 's/^v//' | cut -d. -f1,2); \
		CHANGELOG_PATH="CHANGELOG/CHANGELOG-$$MAJOR_MINOR.md"; \
		RELEASE_BRANCH="release-$$MAJOR_MINOR"; \
		RELEASE_SHA=$$(git rev-parse --verify --quiet "refs/remotes/upstream/$$RELEASE_BRANCH" || true); \
		if [ -z "$$RELEASE_SHA" ]; then \
			if [ -f "$$CHANGELOG_PATH" ]; then \
				echo "Error: branch $$RELEASE_BRANCH not found on upstream, but $$CHANGELOG_PATH exists. Run: git fetch upstream $$RELEASE_BRANCH"; \
				exit 1; \
			fi; \
			RELEASE_SHA=$$(git rev-parse HEAD); \
			echo "Branch $$RELEASE_BRANCH does not exist yet (new release line $$MAJOR_MINOR, created by the release workflow); using HEAD"; \
		fi; \
		PATCH=$$(echo "$(VERSION)" | cut -d. -f3); \
		if [ "$$PATCH" -gt 0 ]; then \
			PREV_TAG="$$(echo "$(VERSION)" | cut -d. -f1,2).$$((PATCH - 1))"; \
		else \
			PREV_MINOR=$$(( $$(echo "$(VERSION)" | cut -d. -f2) - 1 )); \
			PREV_TAG=$$(git tag --list "$$(echo "$(VERSION)" | cut -d. -f1).$$PREV_MINOR.*" | grep -vE -- '(rc)' | sort -t. -k3,3nr | head -1 || true); \
		fi; \
		if [ -z "$$PREV_TAG" ]; then \
			echo "Error: cannot determine the previous release tag for $(VERSION)"; \
			exit 1; \
		fi; \
		echo "Generating changelog for $(VERSION) (range: $$PREV_TAG..$$RELEASE_SHA)"; \
		touch "$$CHANGELOG_PATH"; \
		docker run --rm -u $$(id -u):$$(id -g) -e HOME=/tmp -e GITHUB_TOKEN \
			-v $(PROJECT_DIR):/app -w /app ghcr.io/orhun/git-cliff/git-cliff:latest \
			"$$PREV_TAG..$$RELEASE_SHA" --tag $(VERSION) --prepend "$$CHANGELOG_PATH"; \
		echo "Changelog generated at $$CHANGELOG_PATH"; \
	fi
	@echo "Regenerating files for $(VERSION)"
	@$(MAKE) generate
	@echo ""
	@echo "Release commit for $(VERSION) is ready."
	@echo "Review the changelog changes, then commit with:"
	@echo "  git add -A && git commit -s -m 'Prepare Release $(VERSION)'"
