#!/usr/bin/env bash

# Copyright The Kubeflow Authors.
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

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERIFIER="${SCRIPT_DIR}/verify-crd-packaging.sh"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

create_fixture() {
  local fixture_root=$1

  mkdir -p \
    "${fixture_root}/manifests/base/crds" \
    "${fixture_root}/charts/kubeflow-trainer/templates/crd" \
    "${fixture_root}/charts/kubeflow-trainer/tests/crd"

  touch \
    "${fixture_root}/manifests/base/crds/trainer.kubeflow.org_optimizationjobs.yaml" \
    "${fixture_root}/manifests/base/crds/trainer.kubeflow.org_trainjobs.yaml" \
    "${fixture_root}/charts/kubeflow-trainer/templates/crd/trainer.kubeflow.org_optimizationjobs.yaml" \
    "${fixture_root}/charts/kubeflow-trainer/templates/crd/trainer.kubeflow.org_trainjobs.yaml"

  cat >"${fixture_root}/manifests/base/crds/kustomization.yaml" <<'EOF'
resources:
  - trainer.kubeflow.org_optimizationjobs.yaml
  - trainer.kubeflow.org_trainjobs.yaml
EOF

  cat >"${fixture_root}/charts/kubeflow-trainer/tests/crd/crd_test.yaml" <<'EOF'
templates:
  - crd/trainer.kubeflow.org_optimizationjobs.yaml
  - crd/trainer.kubeflow.org_trainjobs.yaml
tests:
  - it: tests OptimizationJob
    template: crd/trainer.kubeflow.org_optimizationjobs.yaml
  - it: tests TrainJob
    template: crd/trainer.kubeflow.org_trainjobs.yaml
EOF
}

remove_matching_line() {
  local pattern=$1
  local file=$2

  awk -v pattern="${pattern}" 'index($0, pattern) == 0' "${file}" >"${file}.tmp"
  mv "${file}.tmp" "${file}"
}

expect_failure() {
  local test_name=$1
  local expected_message=$2
  local fixture_root=$3
  local output

  if output="$(${VERIFIER} "${fixture_root}" 2>&1)"; then
    echo "FAIL: ${test_name}: verifier unexpectedly passed" >&2
    exit 1
  fi

  if [[ "${output}" != *"${expected_message}"* ]]; then
    echo "FAIL: ${test_name}: expected output to contain: ${expected_message}" >&2
    echo "Actual output:" >&2
    echo "${output}" >&2
    exit 1
  fi
}

baseline="${TMP_DIR}/baseline"
create_fixture "${baseline}"
"${VERIFIER}" "${baseline}" >/dev/null

missing_kustomize="${TMP_DIR}/missing-kustomize"
create_fixture "${missing_kustomize}"
remove_matching_line "trainer.kubeflow.org_optimizationjobs.yaml" \
  "${missing_kustomize}/manifests/base/crds/kustomization.yaml"
expect_failure "missing Kustomize resource" \
  "missing: trainer.kubeflow.org_optimizationjobs.yaml" "${missing_kustomize}"

missing_helm="${TMP_DIR}/missing-helm"
create_fixture "${missing_helm}"
rm "${missing_helm}/charts/kubeflow-trainer/templates/crd/trainer.kubeflow.org_optimizationjobs.yaml"
expect_failure "missing Helm template" \
  "missing: trainer.kubeflow.org_optimizationjobs.yaml" "${missing_helm}"

missing_test_suite="${TMP_DIR}/missing-test-suite"
create_fixture "${missing_test_suite}"
remove_matching_line "  - crd/trainer.kubeflow.org_optimizationjobs.yaml" \
  "${missing_test_suite}/charts/kubeflow-trainer/tests/crd/crd_test.yaml"
expect_failure "missing Helm test suite template" \
  "Helm CRD test suite templates:" "${missing_test_suite}"

missing_test_case="${TMP_DIR}/missing-test-case"
create_fixture "${missing_test_case}"
remove_matching_line "template: crd/trainer.kubeflow.org_optimizationjobs.yaml" \
  "${missing_test_case}/charts/kubeflow-trainer/tests/crd/crd_test.yaml"
expect_failure "missing Helm test case" \
  "Helm CRD test cases:" "${missing_test_case}"

stale_kustomize="${TMP_DIR}/stale-kustomize"
create_fixture "${stale_kustomize}"
echo "  - trainer.kubeflow.org_stale.yaml" >> \
  "${stale_kustomize}/manifests/base/crds/kustomization.yaml"
expect_failure "stale Kustomize resource" \
  "stale: trainer.kubeflow.org_stale.yaml" "${stale_kustomize}"

stale_helm="${TMP_DIR}/stale-helm"
create_fixture "${stale_helm}"
touch "${stale_helm}/charts/kubeflow-trainer/templates/crd/trainer.kubeflow.org_stale.yaml"
expect_failure "stale Helm template" \
  "stale: trainer.kubeflow.org_stale.yaml" "${stale_helm}"

stale_test_suite="${TMP_DIR}/stale-test-suite"
create_fixture "${stale_test_suite}"
test_file="${stale_test_suite}/charts/kubeflow-trainer/tests/crd/crd_test.yaml"
awk '
  /^tests:/ { print "  - crd/trainer.kubeflow.org_stale.yaml" }
  { print }
' "${test_file}" >"${test_file}.tmp"
mv "${test_file}.tmp" "${test_file}"
expect_failure "stale Helm test suite template" \
  "stale: trainer.kubeflow.org_stale.yaml" "${stale_test_suite}"

stale_test_case="${TMP_DIR}/stale-test-case"
create_fixture "${stale_test_case}"
echo "    template: crd/trainer.kubeflow.org_stale.yaml" >> \
  "${stale_test_case}/charts/kubeflow-trainer/tests/crd/crd_test.yaml"
expect_failure "stale Helm test case" \
  "stale: trainer.kubeflow.org_stale.yaml" "${stale_test_case}"

echo "CRD packaging verifier tests passed"
