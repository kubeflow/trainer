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

# This shell is used to setup GPU enabled Kind cluster for Kubeflow Trainer e2e tests.

set -o errexit
set -o nounset
set -o pipefail
set -x

# Source container runtime utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/scripts/container-runtime.sh"
source "${SCRIPT_DIR}/scripts/load-image-to-kind.sh"

# Setup container runtime
setup_container_runtime "sudo"

# Configure variables.
KIND=${KIND:-kind}
K8S_VERSION=${K8S_VERSION:-1.32.0}
GPU_OPERATOR_VERSION="v25.3.2"
KIND_NODE_VERSION=kindest/node:v${K8S_VERSION}
GPU_CLUSTER_NAME="kind-gpu"

# sudo for nvkind and docker commands
alias docker="sudo docker"
alias kubectl="sudo kubectl"
alias kind="sudo kind"
alias helm="sudo helm"
alias nvkind="sudo nvkind"

echo $(lsb_release -a)

export TOOLKIT_VERSION=1.17.0-1

sudo apt-get update
sudo apt-get install -y --allow-downgrades \
    nvidia-container-toolkit=${TOOLKIT_VERSION} \
    nvidia-container-toolkit-base=${TOOLKIT_VERSION} \
    libnvidia-container-tools=${TOOLKIT_VERSION} \
    libnvidia-container1=${TOOLKIT_VERSION}

sudo nvidia-ctk config --set accept-nvidia-visible-devices-as-volume-mounts=true --in-place
sudo nvidia-ctk runtime configure --runtime=docker --set-as-default
sudo systemctl restart docker

# Create a Kind cluster with GPU support.
NVKIND_BIN="/root/go/bin/nvkind"
sudo "$NVKIND_BIN" cluster create --name "${GPU_CLUSTER_NAME}" --image "${KIND_NODE_VERSION}"
sudo "$NVKIND_BIN" cluster print-gpus

# Install gpu-operator to make sure we can run GPU workloads.
echo "Install NVIDIA GPU Operator"
kubectl create ns gpu-operator
kubectl label --overwrite ns gpu-operator pod-security.kubernetes.io/enforce=privileged

helm repo add nvidia https://helm.ngc.nvidia.com/nvidia && helm repo update

helm install gpu-operator nvidia/gpu-operator \
  -n gpu-operator \
  --version="${GPU_OPERATOR_VERSION}" \
  --set driver.enabled=false \
  --set toolkit.enabled=false \
  --wait

# Validation steps for GPU operator installation
kubectl get ns gpu-operator
kubectl get ns gpu-operator --show-labels | grep pod-security.kubernetes.io/enforce=privileged
helm list -n gpu-operator
kubectl get pods -n gpu-operator -o name | while read pod; do
  kubectl wait --for=condition=Ready --timeout=300s "$pod" -n gpu-operator || echo "$pod failed to become Ready"
done
kubectl get pods -n gpu-operator
kubectl get nodes -o=custom-columns=NAME:.metadata.name,GPU:.status.allocatable.nvidia\.com/gpu
