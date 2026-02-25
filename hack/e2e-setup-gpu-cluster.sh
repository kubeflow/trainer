#!/usr/bin/env bash

# Copyright 2025 The Kubeflow Authors.
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

# This shell is used to setup Kind cluster for Kubeflow Trainer e2e tests.

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
KIND=${KIND:-kind}
K8S_VERSION=${K8S_VERSION:-1.32.0}
GPU_OPERATOR_VERSION="v25.3.2"
KIND_NODE_VERSION=kindest/node:v${K8S_VERSION}
GPU_CLUSTER_NAME="kind-gpu"
NAMESPACE="kubeflow-system"
TIMEOUT="5m"

# Kubeflow Trainer images.
CONTROLLER_MANAGER_CI_IMAGE_NAME="ghcr.io/kubeflow/trainer/trainer-controller-manager"
CI_IMAGE_TAG="test"
CONTROLLER_MANAGER_CI_IMAGE="${CONTROLLER_MANAGER_CI_IMAGE_NAME}:${CI_IMAGE_TAG}"
echo "Build Kubeflow Trainer images"
${CONTAINER_RUNTIME} build . -f cmd/trainer-controller-manager/Dockerfile -t ${CONTROLLER_MANAGER_CI_IMAGE}

# Kubeflow Trainer initializer images.
DATASET_INITIALIZER_CI_IMAGE_NAME="ghcr.io/kubeflow/trainer/dataset-initializer"
DATASET_INITIALIZER_CI_IMAGE="${DATASET_INITIALIZER_CI_IMAGE_NAME}:${CI_IMAGE_TAG}"
${CONTAINER_RUNTIME} build . -f cmd/initializers/dataset/Dockerfile -t ${DATASET_INITIALIZER_CI_IMAGE}

MODEL_INITIALIZER_CI_IMAGE_NAME="ghcr.io/kubeflow/trainer/model-initializer"
MODEL_INITIALIZER_CI_IMAGE="${MODEL_INITIALIZER_CI_IMAGE_NAME}:${CI_IMAGE_TAG}"
${CONTAINER_RUNTIME} build . -f cmd/initializers/model/Dockerfile -t ${MODEL_INITIALIZER_CI_IMAGE}

TRAINER_CI_IMAGE_NAME="ghcr.io/kubeflow/trainer/torchtune-trainer"
TRAINER_CI_IMAGE="${TRAINER_CI_IMAGE_NAME}:${CI_IMAGE_TAG}"
${CONTAINER_RUNTIME} build . -f cmd/trainers/torchtune/Dockerfile -t ${TRAINER_CI_IMAGE}

# Configure NVIDIA runtime.
sudo nvidia-ctk config --set accept-nvidia-visible-devices-as-volume-mounts=true --in-place
sudo nvidia-ctk runtime configure --runtime=docker --set-as-default
sudo systemctl restart docker

# Create a Kind cluster with GPU support.
NVKIND_BIN="/root/go/bin/nvkind"
sudo "$NVKIND_BIN" cluster create --name "${GPU_CLUSTER_NAME}" --image "${KIND_NODE_VERSION}"
sudo "$NVKIND_BIN" cluster print-gpus

# Make kubeconfig available to non-root user
mkdir -p "$HOME/.kube"
sudo cp /root/.kube/config "$HOME/.kube/config"
sudo chown "$(id -u):$(id -g)" "$HOME/.kube/config"
export KUBECONFIG="$HOME/.kube/config"

# Install gpu-operator to make sure we can run GPU workloads.
echo "Installing NVIDIA GPU Operator"
kubectl create ns gpu-operator
kubectl label --overwrite ns gpu-operator pod-security.kubernetes.io/enforce=privileged

# Helm home dirs for non-root user
export HELM_CONFIG_HOME="$HOME/.config/helm"
export HELM_CACHE_HOME="$HOME/.cache/helm"
export HELM_DATA_HOME="$HOME/.local/share/helm"

mkdir -p "$HELM_CONFIG_HOME" "$HELM_CACHE_HOME" "$HELM_DATA_HOME"

helm repo add nvidia https://helm.ngc.nvidia.com/nvidia && helm repo update

helm install --wait --generate-name \
  -n gpu-operator --create-namespace \
  nvidia/gpu-operator \
  --version="${GPU_OPERATOR_VERSION}" \
  --set driver.enabled=false

# Validation steps for GPU operator installation
kubectl get ns gpu-operator
kubectl get ns gpu-operator --show-labels | grep pod-security.kubernetes.io/enforce=privileged
helm list -n gpu-operator
kubectl get pods -n gpu-operator -o name | while read pod; do
  kubectl wait --for=condition=Ready --timeout=180s "$pod" -n gpu-operator || echo "$pod failed to become Ready"
done
kubectl get pods -n gpu-operator
kubectl get nodes -o=custom-columns=NAME:.metadata.name,GPU:'.status.allocatable.nvidia\.com/gpu'

# Load Kubeflow Trainer images
echo "Load Kubeflow Trainer images"
load_image_to_kind "${CONTROLLER_MANAGER_CI_IMAGE}" "${GPU_CLUSTER_NAME}"

echo "Load Kubeflow Trainer initializers images"
load_image_to_kind "${DATASET_INITIALIZER_CI_IMAGE}" "${GPU_CLUSTER_NAME}"
load_image_to_kind "${MODEL_INITIALIZER_CI_IMAGE}" "${GPU_CLUSTER_NAME}"
load_image_to_kind "${TRAINER_CI_IMAGE}" "${GPU_CLUSTER_NAME}"

# Deploy Kubeflow Trainer control plane
echo "Deploy Kubeflow Trainer control plane"
E2E_MANIFESTS_DIR="artifacts/e2e/manifests"
mkdir -p "${E2E_MANIFESTS_DIR}"
cat <<EOF >"${E2E_MANIFESTS_DIR}/kustomization.yaml"
  apiVersion: kustomize.config.k8s.io/v1beta1
  kind: Kustomization
  resources:
  - ../../../manifests/overlays/manager
  images:
  - name: "${CONTROLLER_MANAGER_CI_IMAGE_NAME}"
    newTag: "${CI_IMAGE_TAG}"
EOF

kubectl apply --server-side -k "${E2E_MANIFESTS_DIR}"

# We should wait until Deployment is in Ready status.
echo "Wait for Kubeflow Trainer to be ready"
(kubectl wait deploy/kubeflow-trainer-controller-manager --for=condition=available -n ${NAMESPACE} --timeout ${TIMEOUT} &&
  kubectl wait pods --for=condition=ready -n ${NAMESPACE} --timeout ${TIMEOUT} --all) ||
  (
    echo "Failed to wait until Kubeflow Trainer is ready" &&
      kubectl get pods -n ${NAMESPACE} &&
      kubectl describe pods -n ${NAMESPACE} &&
      exit 1
  )

print_cluster_info() {
  kubectl version
  kubectl cluster-info
  kubectl get nodes
  kubectl get pods -n ${NAMESPACE}
  kubectl describe pod -n ${NAMESPACE}
}

# Deploy Kubeflow Trainer Runtimes
echo "Deploy Kubeflow Trainer Runtimes"
E2E_RUNTIMES_DIR="artifacts/e2e/runtimes"
mkdir -p "${E2E_RUNTIMES_DIR}"
cat <<EOF >"${E2E_RUNTIMES_DIR}/kustomization.yaml"
  apiVersion: kustomize.config.k8s.io/v1beta1
  kind: Kustomization
  resources:
  - ../../../manifests/overlays/runtimes
  images:
  - name: "${DATASET_INITIALIZER_CI_IMAGE_NAME}"
    newTag: "${CI_IMAGE_TAG}"
  - name: "${MODEL_INITIALIZER_CI_IMAGE_NAME}"
    newTag: "${CI_IMAGE_TAG}"
  - name: "${TRAINER_CI_IMAGE_NAME}"
    newTag: "${CI_IMAGE_TAG}"
EOF

# TODO (andreyvelich): Currently, we print manager logs due to flaky test.
echo "Deploy Kubeflow Trainer runtimes"
kubectl apply --server-side -k "${E2E_RUNTIMES_DIR}" || (
  kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=trainer &&
    print_cluster_info &&
    exit 1
)

# hotfix: patch CRDs to run on GPU nodes (Check #3067)
echo "Patch CRDs to run on GPU nodes"
kubectl get clustertrainingruntimes -o json | jq '
  .items[].spec.template.spec.replicatedJobs[].template.spec.template.spec.runtimeClassName = "nvidia"
' | kubectl apply -f -

# hotfix(jaiakash) - skip pre-load due to kind failure
# # TODO (andreyvelich): Discuss how we want to pre-load runtime images to the Kind cluster.
# TORCH_RUNTIME_IMAGE=pytorch/pytorch:2.9.1-cuda12.8-cudnn9-runtime
# ${CONTAINER_RUNTIME} pull ${TORCH_RUNTIME_IMAGE}
# load_image_to_kind ${TORCH_RUNTIME_IMAGE} ${GPU_CLUSTER_NAME}

# # Pre-pull NVIDIA JAX image for JAX runtime.
# JAX_RUNTIME_IMAGE=nvcr.io/nvidia/jax:25.10-py3
# ${CONTAINER_RUNTIME} pull ${JAX_RUNTIME_IMAGE}
# load_image_to_kind ${JAX_RUNTIME_IMAGE} ${GPU_CLUSTER_NAME}

print_cluster_info
