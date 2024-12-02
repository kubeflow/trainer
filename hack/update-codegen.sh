#!/bin/bash

# This shell is used to auto generate some useful tools for k8s, such as clientset, lister, informer and so on.
# We don't use this tool to generate deepcopy because kubebuilder (controller-tools) has covered that part.

set -o errexit
set -o nounset
set -o pipefail

GO_CMD=${1:-go}
CURRENT_DIR=$(dirname "${BASH_SOURCE[0]}")
TRAINING_OPERATOR_ROOT=$(realpath "${CURRENT_DIR}/..")
TRAINING_OPERATOR_PKG="github.com/kubeflow/training-operator"
CODEGEN_PKG=$(go list -m -mod=readonly -f "{{.Dir}}" k8s.io/code-generator)

cd "$CURRENT_DIR/.."

# shellcheck source=/dev/null
source "${CODEGEN_PKG}/kube_codegen.sh"

# Generating conversion and defaults functions
kube::codegen::gen_helpers \
  --boilerplate "${TRAINING_OPERATOR_ROOT}/hack/boilerplate/boilerplate.go.txt" \
  "${TRAINING_OPERATOR_ROOT}/pkg/apis"

# Generating OpenAPI for Kueue API extensions for v1
kube::codegen::gen_openapi \
  --boilerplate "${TRAINING_OPERATOR_ROOT}/hack/boilerplate/boilerplate.go.txt" \
  --output-pkg "${TRAINING_OPERATOR_PKG}/pkg/apis/kubeflow.org/v1" \
  --output-dir "${TRAINING_OPERATOR_ROOT}/pkg/apis/kubeflow.org/v1" \
  --report-filename "${TRAINING_OPERATOR_ROOT}/hack/violation_exception_v1.list" \
  --update-report \
  "${TRAINING_OPERATOR_ROOT}/pkg/apis/kubeflow.org/v1"

# Generating OpenAPI for Kueue API extensions for v2alpha1
kube::codegen::gen_openapi \
  --boilerplate "${TRAINING_OPERATOR_ROOT}/hack/boilerplate/boilerplate.go.txt" \
  --output-pkg "${TRAINING_OPERATOR_PKG}/pkg/apis/kubeflow.org/v2alpha1" \
  --output-dir "${TRAINING_OPERATOR_ROOT}/pkg/apis/kubeflow.org/v2alpha1" \
  --report-filename "${TRAINING_OPERATOR_ROOT}/hack/violation_exception_v2alpha1.list" \
  --update-report \
  "${TRAINING_OPERATOR_ROOT}/pkg/apis/kubeflow.org/v2alpha1"

kube::codegen::gen_client \
  --boilerplate "${TRAINING_OPERATOR_ROOT}/hack/boilerplate/boilerplate.go.txt" \
  --output-dir "${TRAINING_OPERATOR_ROOT}/pkg/client" \
  --output-pkg "${TRAINING_OPERATOR_PKG}/pkg/client" \
  --with-watch \
  --with-applyconfig \
  "${TRAINING_OPERATOR_ROOT}/pkg/apis"
