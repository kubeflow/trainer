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

# Configure variables.
KIND=${KIND:-kind}
K8S_VERSION=${K8S_VERSION:-1.32.0}
GPU_OPERATOR_VERSION="v25.3.2"
KIND_NODE_VERSION=kindest/node:v${K8S_VERSION}
GPU_CLUSTER_NAME="kind-gpu"

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
echo "Install NVIDIA GPU Operator"
kubectl create ns gpu-operator
kubectl label --overwrite ns gpu-operator pod-security.kubernetes.io/enforce=privileged

# Helm home dirs for non-root user
export HELM_CONFIG_HOME="$HOME/.config/helm"
export HELM_CACHE_HOME="$HOME/.cache/helm"
export HELM_DATA_HOME="$HOME/.local/share/helm"

mkdir -p "$HELM_CONFIG_HOME" "$HELM_CACHE_HOME" "$HELM_DATA_HOME"

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
