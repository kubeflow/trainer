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

# This shell script is used to automatically syncing the Kustomize manifests from manifests templated by trainer Helm chart.
# Run 'make sync-manifests' from the root directory of this repository.

echo "Syncing Kustomize manifests from manifests templated by trainer Helm chart..."

set -o errexit
set -o nounset
set -o pipefail

# Helm chart release name and namespace.
TRAINER_CHART_DIR=charts/trainer
RELEASE_NAME=kubeflow-trainer
RELEASE_NAMESPACE=kubeflow-system

# Source directories
SRC_CRD_DIR=manifests/base/crds

# Destination directories
DST_CRD_DIR=charts/trainer/crds
DST_RBAC_DIR=manifests/base/rbac
DST_CONTROLLER_DIR=manifests/base/controller
DST_WEBHOOK_DIR=manifests/base/webhook
DST_RUNTIMES_DIR=manifests/base/runtimes

MANIFESTS_FILE=$(mktemp -t kubeflow-trainer-manifests.yaml)
FIND_EXCLUDE_ARGS="-not -name kustomization.yaml -not -name kustomization.yml"
YEAR=$(date +%Y)
LICENSE=$(cat hack/boilerplate/boilerplate.go.txt | sed 's|//|#|' | sed "s|YEAR|${YEAR}|g")
YQ_ARGS=".metadata.labels.\"app.kubernetes.io/managed-by\" = \"Kustomize\" | del(.metadata.labels.\"helm.sh/chart\") | ... comments=\"\""

setup() {
    # yq is required to parse yaml files.
    if ! command -v yq &>/dev/null; then
        echo "'yq' is not installed, please install it first. Ref: https://github.com/mikefarah/yq."
        exit 1
    fi

    # Create destination directory if it doesn't exist.
    mkdir -p ${DST_CRD_DIR} ${DST_RBAC_DIR} ${DST_CONTROLLER_DIR} ${DST_WEBHOOK_DIR} ${DST_RUNTIMES_DIR}/pretraining

    helm template ${RELEASE_NAME} ${TRAINER_CHART_DIR} --namespace ${RELEASE_NAMESPACE} > "${MANIFESTS_FILE}"
}

update_crds() {
    # Copy all CRD files to destination directory.
    # shellcheck disable=SC2086
    find ${SRC_CRD_DIR} -type f -name "*.yaml" ${FIND_EXCLUDE_ARGS} -exec cp {} ${DST_CRD_DIR} \;
}

update_rbac() {
    printf '#\n%s\n#\n\n' "${LICENSE}" > ${DST_RBAC_DIR}/serviceaccount.yaml
    yq -e "select(.kind == \"ServiceAccount\" and .metadata.name == \"kubeflow-trainer-controller\") | ${YQ_ARGS}" "${MANIFESTS_FILE}" >> ${DST_RBAC_DIR}/serviceaccount.yaml

    printf '#\n%s\n#\n\n' "${LICENSE}" > ${DST_RBAC_DIR}/clusterrole.yaml
    yq -e "select(.kind == \"ClusterRole\" and .metadata.name == \"kubeflow-trainer-controller\") | ${YQ_ARGS}" "${MANIFESTS_FILE}" >> ${DST_RBAC_DIR}/clusterrole.yaml

    printf '#\n%s\n#\n\n' "${LICENSE}" > ${DST_RBAC_DIR}/clusterrolebinding.yaml
    yq -e "select(.kind == \"ClusterRoleBinding\" and .metadata.name == \"kubeflow-trainer-controller\") | ${YQ_ARGS}" "${MANIFESTS_FILE}" >> ${DST_RBAC_DIR}/clusterrolebinding.yaml

    printf '#\n%s\n#\n\n' "${LICENSE}" > ${DST_RBAC_DIR}/role.yaml
    yq -e "select(.kind == \"Role\" and .metadata.name == \"kubeflow-trainer-controller\") | ${YQ_ARGS}" "${MANIFESTS_FILE}" >> ${DST_RBAC_DIR}/role.yaml

    printf '#\n%s\n#\n\n' "${LICENSE}" > ${DST_RBAC_DIR}/rolebinding.yaml
    yq -e "select(.kind == \"RoleBinding\" and .metadata.name == \"kubeflow-trainer-controller\") | ${YQ_ARGS}" "${MANIFESTS_FILE}" >> ${DST_RBAC_DIR}/rolebinding.yaml
}

update_controller() {
    printf '#\n%s\n#\n\n' "${LICENSE}" > ${DST_CONTROLLER_DIR}/deployment.yaml
    yq -e "select(.kind == \"Deployment\" and .metadata.name == \"kubeflow-trainer-controller\") | ${YQ_ARGS}" "${MANIFESTS_FILE}" >> ${DST_CONTROLLER_DIR}/deployment.yaml

    printf '#\n%s\n#\n\n' "${LICENSE}" > ${DST_CONTROLLER_DIR}/service.yaml
    yq -e "select(.kind == \"Service\" and .metadata.name == \"kubeflow-trainer-controller-service\") | ${YQ_ARGS}" "${MANIFESTS_FILE}" >> ${DST_CONTROLLER_DIR}/service.yaml
}

update_webhook() {
    printf '#\n%s\n#\n\n' "${LICENSE}" > ${DST_WEBHOOK_DIR}/secret.yaml
    yq -e "select(.kind == \"Secret\" and .metadata.name == \"kubeflow-trainer-webhook-cert\") | ${YQ_ARGS}" "${MANIFESTS_FILE}" >> ${DST_WEBHOOK_DIR}/secret.yaml

    printf '#\n%s\n#\n\n' "${LICENSE}" > ${DST_WEBHOOK_DIR}/validatingwebhookconfiguration.yaml
    yq -e "select(.kind == \"ValidatingWebhookConfiguration\" and .metadata.name == \"kubeflow-trainer-validating-webhook\" | ${YQ_ARGS})" "${MANIFESTS_FILE}" >> ${DST_WEBHOOK_DIR}/validatingwebhookconfiguration.yaml
}

update_runtimes() {
    printf '#\n%s\n#\n\n' "${LICENSE}" > ${DST_RUNTIMES_DIR}/pretraining/torch-distributed.yaml
    yq -e "select(.kind == \"ClusterTrainingRuntime\" and .metadata.name == \"torch-distributed\")" "${MANIFESTS_FILE}" > ${DST_RUNTIMES_DIR}/pretraining/torch-distributed.yaml
}

cleanup() {
    rm -f "${MANIFESTS_FILE}"
}

setup
update_crds
update_rbac
update_controller
update_webhook
update_runtimes
cleanup
