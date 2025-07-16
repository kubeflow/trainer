#!/bin/bash

# This shell is used to auto generate some useful tools for k8s, such as clientset, lister, informer and so on.
# We don't use this tool to generate deepcopy because kubebuilder (controller-tools) has covered that part.

set -o errexit
set -o nounset
set -o pipefail

CURRENT_DIR=$(dirname "${BASH_SOURCE[0]}")
TRAINING_OPERATOR_ROOT=$(realpath "${CURRENT_DIR}/..")
TRAINING_OPERATOR_PKG="github.com/kubeflow/training-operator"

cd "$CURRENT_DIR/.."

# Get the code-generator binary.
CODEGEN_PKG=$(go list -m -mod=readonly -f "{{.Dir}}" k8s.io/code-generator)
source "${CODEGEN_PKG}/kube_codegen.sh"
echo ">> Using ${CODEGEN_PKG}"

# Generating deepcopy and defaults.
echo "Generating deepcopy and defaults for kubeflow.org/v1"
kube::codegen::gen_helpers \
  --boilerplate "${TRAINING_OPERATOR_ROOT}/hack/boilerplate/boilerplate.go.txt" \
  "${TRAINING_OPERATOR_ROOT}/pkg/apis"

# Generate clients for Training Operator V1
echo "Generating clients for kubeflow.org/v1"
kube::codegen::gen_client \
  --boilerplate "${TRAINING_OPERATOR_ROOT}/hack/boilerplate/boilerplate.go.txt" \
  --output-dir "${TRAINING_OPERATOR_ROOT}/pkg/client" \
  --output-pkg "${TRAINING_OPERATOR_PKG}/pkg/client" \
  --with-watch \
  --with-applyconfig \
  "${TRAINING_OPERATOR_ROOT}/pkg/apis"

# Get the kube-openapi binary.
OPENAPI_PKG=$(go list -m -mod=readonly -f "{{.Dir}}" k8s.io/kube-openapi)
echo ">> Using ${OPENAPI_PKG}"

echo "Generating OpenAPI specification for kubeflow.org/v1"
go run ${OPENAPI_PKG}/cmd/openapi-gen \
  --go-header-file "${TRAINING_OPERATOR_ROOT}/hack/boilerplate/boilerplate.go.txt" \
  --output-pkg "${TRAINING_OPERATOR_PKG}/pkg/apis/kubeflow.org/v1" \
  --output-dir "${TRAINING_OPERATOR_ROOT}/pkg/apis/kubeflow.org/v1" \
  --output-file "zz_generated.openapi.go" \
  --report-filename "${TRAINING_OPERATOR_ROOT}/hack/violation_exception_v1.list" \
  "${TRAINING_OPERATOR_ROOT}/pkg/apis/kubeflow.org/v1"
