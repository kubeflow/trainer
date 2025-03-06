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

# This shell is used to setup Kind cluster for Kubeflow Trainer e2e tests.

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

# Kubeflow Trainer images.
# TODO (andreyvelich): Support initializers images.
CONTROLLER_MANAGER_CI_IMAGE_NAME="kubeflow/trainer-controller-manager"
CONTROLLER_MANAGER_CI_IMAGE_TAG="test"
CONTROLLER_MANAGER_CI_IMAGE="${CONTROLLER_MANAGER_CI_IMAGE_NAME}:${CONTROLLER_MANAGER_CI_IMAGE_TAG}"
echo "Build Kubeflow Trainer images"
docker build . -f cmd/trainer-controller-manager/Dockerfile -t ${CONTROLLER_MANAGER_CI_IMAGE}

echo "Create Kind cluster and load Kubeflow Trainer images"
${KIND} create cluster --image "${KIND_NODE_VERSION}"
${KIND} load docker-image ${CONTROLLER_MANAGER_CI_IMAGE}

echo "Deploy Kubeflow Trainer control plane"
E2E_MANIFESTS_DIR="artifacts/e2e/manifests"
mkdir -p "${E2E_MANIFESTS_DIR}"
cat <<EOF > "${E2E_MANIFESTS_DIR}/kustomization.yaml"
  apiVersion: kustomize.config.k8s.io/v1beta1
  kind: Kustomization
  resources:
  - ../../../manifests/overlays/manager
  images:
  - name: "${CONTROLLER_MANAGER_CI_IMAGE_NAME}"
    newTag: "${CONTROLLER_MANAGER_CI_IMAGE_TAG}"
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

# TODO (andreyvelich): Currently, we print manager logs due to flaky test.
echo "Deploy Kubeflow Trainer runtimes"
kubectl apply --server-side -k manifests/overlays/runtimes || (
  kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=trainer &&
    print_cluster_info &&
    exit 1
)

# TODO (andreyvelich): Discuss how we want to pre-load runtime images to the Kind cluster.
TORCH_RUNTIME_IMAGE=pytorch/pytorch:2.5.0-cuda12.4-cudnn9-runtime
docker pull ${TORCH_RUNTIME_IMAGE}
${KIND} load docker-image ${TORCH_RUNTIME_IMAGE}

print_cluster_info
