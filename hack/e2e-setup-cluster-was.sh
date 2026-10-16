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

# This shell is used to setup Kind cluster for Kubeflow Trainer Workload-Aware Scheduling e2e tests.

set -o errexit
set -o nounset
set -o pipefail
set -x

# Source container runtime utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/scripts/container-runtime.sh"
source "${SCRIPT_DIR}/scripts/load-image-to-kind.sh"

# Setup container runtime
setup_container_runtime

# Configure variables.
KIND=${KIND:-./bin/kind}
KIND_NODE_VERSION=kindest/node:v${K8S_VERSION}
NAMESPACE="kubeflow-system"

# Tag used for all locally-built CI images.
CI_IMAGE_TAG="test"
CLUSTER_NAME="kind"

print_cluster_info() {
  kubectl version
  kubectl cluster-info
  kubectl get nodes
  kubectl get pods -n ${NAMESPACE}
  kubectl describe pod -n ${NAMESPACE}
}

# ==========================================
# 1. Create Cluster
# ==========================================
echo "Create Kind cluster with Workload-Aware Scheduling feature gates"
${KIND} create cluster --name "${CLUSTER_NAME}" --image "${KIND_NODE_VERSION}" --config "${SCRIPT_DIR}/kind-config-was.yaml"

# ==========================================
# 2. Build and Load Images
# ==========================================
CONTROLLER_MANAGER_CI_IMAGE_NAME="ghcr.io/kubeflow/trainer/trainer-controller-manager"
CONTROLLER_MANAGER_CI_IMAGE="${CONTROLLER_MANAGER_CI_IMAGE_NAME}:${CI_IMAGE_TAG}"
echo "Build Kubeflow Trainer controller manager image"
${CONTAINER_RUNTIME} build . -f cmd/trainer-controller-manager/Dockerfile -t ${CONTROLLER_MANAGER_CI_IMAGE}

TORCH_DISTRIBUTED_IMAGE="pytorch/pytorch:2.13.0-cuda13.0-cudnn9-runtime"
echo "Pull torch-distributed runtime image"
${CONTAINER_RUNTIME} pull ${TORCH_DISTRIBUTED_IMAGE}

echo "Load Kubeflow Trainer and torch-distributed runtime images into Kind"
load_image_to_kind "${CONTROLLER_MANAGER_CI_IMAGE}" "${CLUSTER_NAME}"
load_image_to_kind "${TORCH_DISTRIBUTED_IMAGE}" "${CLUSTER_NAME}"

# ==========================================
# 3. Deploy Control Plane & Runtimes
# ==========================================
echo "Installing Kubeflow Trainer via Helm"

# Build Helm dependencies
helm dependency build charts/kubeflow-trainer

helm install trainer charts/kubeflow-trainer \
  --namespace ${NAMESPACE} \
  --create-namespace \
  --set runtimes.torchDistributed.enabled=true \
  --set image.tag=${CI_IMAGE_TAG} \
  --set manager.config.featureGates.TrainJobWorkloadAwareScheduling=true \
  --wait

print_cluster_info
