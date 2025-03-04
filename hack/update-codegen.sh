#!/usr/bin/env bash

# Copyright 2024 The Kubeflow Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This shell is used to auto generate some useful tools for k8s, such as clientset, lister, informer.

set -o errexit
set -o nounset
set -o pipefail

CURRENT_DIR=$(dirname "${BASH_SOURCE[0]}")
TRAINER_ROOT=$(realpath "${CURRENT_DIR}/..")
TRAINER_PKG="github.com/kubeflow/trainer"

cd "$CURRENT_DIR/.."

# Get the code-generator binary.
CODEGEN_PKG=$(go list -m -mod=readonly -f "{{.Dir}}" k8s.io/code-generator)
source "${CODEGEN_PKG}/kube_codegen.sh"
echo ">> Using ${CODEGEN_PKG}"

# Generating deepcopy and defaults.
echo "Generating deepcopy and defaults for Kubeflow Trainer"
kube::codegen::gen_helpers \
  --boilerplate "${TRAINER_ROOT}/hack/boilerplate/boilerplate.go.txt" \
  "${TRAINER_ROOT}/pkg/apis"

# Generate clients.
externals=(
  "sigs.k8s.io/jobset/api/jobset/v1alpha2.JobSetSpec:sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"
  "k8s.io/api/core/v1.EnvVar:k8s.io/client-go/applyconfigurations/core/v1"
  "k8s.io/api/core/v1.EnvFromSource:k8s.io/client-go/applyconfigurations/core/v1"
  "k8s.io/api/core/v1.ResourceRequirements:k8s.io/client-go/applyconfigurations/core/v1"
  "k8s.io/api/core/v1.Toleration:k8s.io/client-go/applyconfigurations/core/v1"
  "k8s.io/api/core/v1.Volume:k8s.io/client-go/applyconfigurations/core/v1"
  "k8s.io/api/core/v1.VolumeMount:k8s.io/client-go/applyconfigurations/core/v1"
  "k8s.io/api/autoscaling/v2.MetricSpec:k8s.io/client-go/applyconfigurations/autoscaling/v2"
)

apply_config_externals="${externals[0]}"
for external in "${externals[@]:1}"; do
  apply_config_externals="${apply_config_externals},${external}"
done

echo "Generating clients for Kubeflow Trainer"
kube::codegen::gen_client \
  --boilerplate "${TRAINER_ROOT}/hack/boilerplate/boilerplate.go.txt" \
  --output-dir "${TRAINER_ROOT}/pkg/client" \
  --output-pkg "${TRAINER_PKG}/pkg/client" \
  --with-watch \
  --with-applyconfig \
  --applyconfig-externals "${apply_config_externals}" \
  "${TRAINER_ROOT}/pkg/apis"

# Get the kube-openapi binary to generate OpenAPI spec.
OPENAPI_PKG=$(go list -m -mod=readonly -f "{{.Dir}}" k8s.io/kube-openapi)
echo ">> Using ${OPENAPI_PKG}"

echo "Generating OpenAPI specification for Kubeflow Trainer"
go run ${OPENAPI_PKG}/cmd/openapi-gen \
  --go-header-file "${TRAINER_ROOT}/hack/boilerplate/boilerplate.go.txt" \
  --output-pkg "${TRAINER_PKG}/pkg/apis/trainer/v1alpha1" \
  --output-dir "${TRAINER_ROOT}/pkg/apis/trainer/v1alpha1" \
  --output-file "zz_generated.openapi.go" \
  --report-filename "${TRAINER_ROOT}/hack/violation_exception_v1alpha1.list" \
  "${TRAINER_ROOT}/pkg/apis/trainer/v1alpha1"

# Generating OpenAPI Swagger for Kubeflow Trainer.
echo "Generate OpenAPI Swagger for Kubeflow Trainer"
go run hack/swagger/main.go >api/openapi-spec/swagger.json
