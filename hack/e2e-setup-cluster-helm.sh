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

# This shell is used to setup Kind cluster and deploy Kubeflow Trainer with Helm for e2e tests.

set -o errexit
set -o nounset
set -o pipefail
set -x

# Configure variables.
KIND=${KIND:-./bin/kind}
K8S_VERSION=${K8S_VERSION:-1.32.0}
KIND_NODE_VERSION=kindest/node:v${K8S_VERSION}
NAMESPACE="kubeflow-system"
TIMEOUT="5m"

# Check if we should build image or use existing
BUILD_IMAGE=${BUILD_IMAGE:-true}
CONTROLLER_MANAGER_CI_IMAGE_NAME="ghcr.io/kubeflow/trainer/trainer-controller-manager"
CONTROLLER_MANAGER_CI_IMAGE_TAG="test"
CONTROLLER_MANAGER_CI_IMAGE="${CONTROLLER_MANAGER_CI_IMAGE_NAME}:${CONTROLLER_MANAGER_CI_IMAGE_TAG}"

if [ "${BUILD_IMAGE}" = "true" ]; then
  echo "Build Kubeflow Trainer images"
  docker build . -f cmd/trainer-controller-manager/Dockerfile -t ${CONTROLLER_MANAGER_CI_IMAGE}
fi

echo "Create Kind cluster and load Kubeflow Trainer images"
${KIND} create cluster --image "${KIND_NODE_VERSION}"

if [ "${BUILD_IMAGE}" = "true" ]; then
  ${KIND} load docker-image ${CONTROLLER_MANAGER_CI_IMAGE}
fi

echo "Deploy Kubeflow Trainer with Helm"
helm upgrade --install kubeflow-trainer ./charts/kubeflow-trainer \
  --namespace ${NAMESPACE} \
  --create-namespace \
  --set image.registry=ghcr.io \
  --set image.repository=kubeflow/trainer/trainer-controller-manager \
  --set image.tag=${CONTROLLER_MANAGER_CI_IMAGE_TAG} \
  --set image.pullPolicy=IfNotPresent \
  --set jobset.install=true \
  --wait \
  --timeout ${TIMEOUT}

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

echo "Deploy Kubeflow Trainer runtimes"
kubectl apply --server-side -k manifests/overlays/runtimes || (
  kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=trainer &&
    print_cluster_info &&
    exit 1
)

# TODO (andreyvelich): We should build runtime images before adding them.
TORCH_RUNTIME_IMAGE=pytorch/pytorch:2.7.1-cuda12.8-cudnn9-runtime
DEEPSPEED_RUNTIME_IMAGE=ghcr.io/kubeflow/trainer/deepspeed-runtime:latest

docker pull ${TORCH_RUNTIME_IMAGE}
docker pull ${DEEPSPEED_RUNTIME_IMAGE}
${KIND} load docker-image ${TORCH_RUNTIME_IMAGE} ${DEEPSPEED_RUNTIME_IMAGE}

print_cluster_info
