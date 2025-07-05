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

# Setup a kind cluster for local development with a local registry.

set -o errexit
set -o nounset
set -o pipefail
set -x

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=hack/scripts/container-runtime.sh
source "${SCRIPT_DIR}/container-runtime.sh"

setup_container_runtime

REGISTRY_NAME="${REGISTRY_NAME:-kind-registry}"
REGISTRY_PORT="${REGISTRY_PORT:-5001}"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-trainer-dev}"
KIND_CONFIG="${KIND_CONFIG:-kind-config.yaml}"
KF_HOST_PORT_80="${KF_HOST_PORT_80:-8080}"
KF_HOST_PORT_443="${KF_HOST_PORT_443:-8443}"
INGRESS_NGINX_VERSION="${INGRESS_NGINX_VERSION:=controller-v1.9.6}"

running="$("${CONTAINER_RUNTIME}" inspect -f '{{.State.Running}}' "${REGISTRY_NAME}" 2>/dev/null || true)"
if [[ "${running}" != "true" ]]; then
  "${CONTAINER_RUNTIME}" run -d --restart=always -p "${REGISTRY_PORT}:${REGISTRY_PORT}" --name "${REGISTRY_NAME}" registry:2
fi

# kind requires this env var if using podman on mac
if [[ "${CONTAINER_RUNTIME}" == "podman" ]]; then
  export KIND_EXPERIMENTAL_PROVIDER=podman
fi

echo "Building Config"
cat <<CONFIG > "${KIND_CONFIG}"
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${REGISTRY_PORT}"]
    endpoint = ["http://${REGISTRY_NAME}:${REGISTRY_PORT}"]
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 80
    hostPort: ${KF_HOST_PORT_80}
    protocol: TCP
  - containerPort: 443
    hostPort: ${KF_HOST_PORT_443}
    protocol: TCP
  kubeadmConfigPatches:
    - |
      kind: InitConfiguration
      nodeRegistration:
        kubeletExtraArgs:
          node-labels: "ingress-ready=true"
CONFIG


echo "Creating cluster"
kind create cluster --config "${KIND_CONFIG}" --name "${KIND_CLUSTER_NAME}"

"${CONTAINER_RUNTIME}" network connect kind "${REGISTRY_NAME}" 2>/dev/null || true

echo "Configuring Registry"
cat <<EOF_CM | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${REGISTRY_PORT}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF_CM


echo "Deploying Ingress controller into KinD cluster"
curl https://raw.githubusercontent.com/kubernetes/ingress-nginx/"${INGRESS_NGINX_VERSION}"/deploy/static/provider/kind/deploy.yaml | sed "s/--publish-status-address=localhost/--report-node-internal-ip-address\\n        - --status-update-interval=10/g" | kubectl apply -f -
kubectl annotate ingressclass nginx "ingressclass.kubernetes.io/is-default-class=true"
kubectl patch deploy --type json --patch '[{"op":"add","path": "/spec/template/spec/containers/0/args/-","value":"--enable-ssl-passthrough"}]' ingress-nginx-controller -n ingress-nginx
kubectl -n ingress-nginx wait --timeout=300s --for=condition=Available deployments --all
