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
REPO_ROOT="${1:-$(cd "${SCRIPT_DIR}/.." && pwd)}"

GENERATED_CRD_DIR="${REPO_ROOT}/manifests/base/crds"
KUSTOMIZATION_FILE="${GENERATED_CRD_DIR}/kustomization.yaml"
HELM_CRD_DIR="${REPO_ROOT}/charts/kubeflow-trainer/templates/crd"
HELM_CRD_TEST_FILE="${REPO_ROOT}/charts/kubeflow-trainer/tests/crd/crd_test.yaml"

require_directory() {
  local directory=$1
  if [[ ! -d "${directory}" ]]; then
    echo "CRD packaging verification failed: directory not found: ${directory}" >&2
    exit 1
  fi
}

require_file() {
  local file=$1
  if [[ ! -f "${file}" ]]; then
    echo "CRD packaging verification failed: file not found: ${file}" >&2
    exit 1
  fi
}

list_generated_crds() {
  find "${GENERATED_CRD_DIR}" -maxdepth 1 -type f \
    -name 'trainer.kubeflow.org_*.yaml' -exec basename {} \; | LC_ALL=C sort -u
}

list_kustomize_resources() {
  awk '
    /^resources:[[:space:]]*$/ { in_resources = 1; next }
    /^[^[:space:]]/ { in_resources = 0 }
    in_resources && $1 == "-" && $2 ~ /^trainer\.kubeflow\.org_[[:alnum:]_.-]+\.yaml$/ {
      print $2
    }
  ' "${KUSTOMIZATION_FILE}" | LC_ALL=C sort -u
}

list_helm_crds() {
  find "${HELM_CRD_DIR}" -maxdepth 1 -type f \
    -name 'trainer.kubeflow.org_*.yaml' -exec basename {} \; | LC_ALL=C sort -u
}

list_helm_test_suite_templates() {
  awk '
    /^templates:[[:space:]]*$/ { in_templates = 1; next }
    /^[^[:space:]]/ { in_templates = 0 }
    in_templates && $1 == "-" && $2 ~ /^crd\/trainer\.kubeflow\.org_[[:alnum:]_.-]+\.yaml$/ {
      sub(/^crd\//, "", $2)
      print $2
    }
  ' "${HELM_CRD_TEST_FILE}" | LC_ALL=C sort -u
}

list_helm_test_cases() {
  awk '
    /^tests:[[:space:]]*$/ { in_tests = 1; next }
    /^[^[:space:]]/ { in_tests = 0 }
    in_tests && $1 == "template:" && $2 ~ /^crd\/trainer\.kubeflow\.org_[[:alnum:]_.-]+\.yaml$/ {
      sub(/^crd\//, "", $2)
      print $2
    }
  ' "${HELM_CRD_TEST_FILE}" | LC_ALL=C sort -u
}

print_entries() {
  local prefix=$1
  local entries=$2

  while IFS= read -r entry; do
    [[ -n "${entry}" ]] && printf '  - %s: %s\n' "${prefix}" "${entry}" >&2
  done <<<"${entries}"
}

compare_inventory() {
  local inventory_name=$1
  local expected_file=$2
  local actual_file=$3
  local missing_entries
  local stale_entries

  missing_entries="$(comm -23 "${expected_file}" "${actual_file}")"
  stale_entries="$(comm -13 "${expected_file}" "${actual_file}")"

  if [[ -z "${missing_entries}" && -z "${stale_entries}" ]]; then
    return 0
  fi

  echo "${inventory_name}:" >&2
  print_entries "missing" "${missing_entries}"
  print_entries "stale" "${stale_entries}"
  return 1
}

require_directory "${GENERATED_CRD_DIR}"
require_directory "${HELM_CRD_DIR}"
require_file "${KUSTOMIZATION_FILE}"
require_file "${HELM_CRD_TEST_FILE}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

list_generated_crds >"${TMP_DIR}/generated"
list_kustomize_resources >"${TMP_DIR}/kustomize"
list_helm_crds >"${TMP_DIR}/helm"
list_helm_test_suite_templates >"${TMP_DIR}/helm-test-suite"
list_helm_test_cases >"${TMP_DIR}/helm-test-cases"

if [[ ! -s "${TMP_DIR}/generated" ]]; then
  echo "CRD packaging verification failed: no generated Trainer CRDs were found" >&2
  exit 1
fi

verification_failed=false

compare_inventory \
  "Kustomize CRD resources" "${TMP_DIR}/generated" "${TMP_DIR}/kustomize" || verification_failed=true
compare_inventory \
  "Helm CRD templates" "${TMP_DIR}/generated" "${TMP_DIR}/helm" || verification_failed=true
compare_inventory \
  "Helm CRD test suite templates" "${TMP_DIR}/generated" "${TMP_DIR}/helm-test-suite" || verification_failed=true
compare_inventory \
  "Helm CRD test cases" "${TMP_DIR}/generated" "${TMP_DIR}/helm-test-cases" || verification_failed=true

if [[ "${verification_failed}" == true ]]; then
  echo "CRD packaging verification failed" >&2
  exit 1
fi

echo "CRD packaging verification passed"
