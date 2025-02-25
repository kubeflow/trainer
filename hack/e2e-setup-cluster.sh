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

# TODO (andreyvelich): Refactor this script for Kubeflow Trainer V2

set -o errexit
set -o nounset
set -o pipefail

echo "Kind load newly locally built image"
# use cluster name which is used in github actions kind create
kind load docker-image ${TRAINING_CI_IMAGE} --name ${KIND_CLUSTER}

echo "Update training operator manifest with newly built image"
cd manifests/overlays/standalone
kustomize edit set image kubeflow/training-operator=${TRAINING_CI_IMAGE}

echo "Installing training operator manifests"
kustomize build . | kubectl apply --server-side -f -

if [ "${GANG_SCHEDULER_NAME}" = "scheduler-plugins" ]; then
  SCHEDULER_PLUGINS_VERSION=$(go list -m -f "{{.Version}}" sigs.k8s.io/scheduler-plugins)
  git clone https://github.com/kubernetes-sigs/scheduler-plugins.git -b "${SCHEDULER_PLUGINS_VERSION}"

  echo "Installing Scheduler Plugins ${SCHEDULER_PLUGINS_VERSION}..."
  helm install scheduler-plugins scheduler-plugins/manifests/install/charts/as-a-second-scheduler/ --create-namespace \
    --namespace scheduler-plugins \
    --set controller.image="registry.k8s.io/scheduler-plugins/controller:${SCHEDULER_PLUGINS_VERSION}" \
    --set scheduler.image="registry.k8s.io/scheduler-plugins/kube-scheduler:${SCHEDULER_PLUGINS_VERSION}"

  echo "Configure gang-scheduling using scheduler-plugins to training-operator"
  kubectl patch -n kubeflow deployments training-operator --type='json' \
    -p='[{"op": "add", "path": "/spec/template/spec/containers/0/command/1", "value": "--gang-scheduler-name=scheduler-plugins"}]'
elif [ "${GANG_SCHEDULER_NAME}" = "volcano" ]; then
  VOLCANO_SCHEDULER_VERSION=$(go list -m -f "{{.Version}}" volcano.sh/apis)

  # patch scheduler first so that it is ready when scheduler-deployment installing finished
  echo "Configure gang-scheduling using volcano to training-operator"
  kubectl patch -n kubeflow deployments training-operator --type='json' \
    -p='[{"op": "add", "path": "/spec/template/spec/containers/0/command/1", "value": "--gang-scheduler-name=volcano"}]'

  echo "Installing volcano scheduler ${VOLCANO_SCHEDULER_VERSION}..."
  kubectl apply -f https://raw.githubusercontent.com/volcano-sh/volcano/${VOLCANO_SCHEDULER_VERSION}/installer/volcano-development.yaml
fi

TIMEOUT=30
until kubectl get pods -n kubeflow | grep training-operator | grep 1/1 || [[ $TIMEOUT -eq 1 ]]; do
  sleep 10
  TIMEOUT=$((TIMEOUT - 1))
done
if [ "${GANG_SCHEDULER_NAME}" = "scheduler-plugins" ]; then
  kubectl wait pods --for=condition=ready -n scheduler-plugins --timeout "${TIMEOUT}s" --all ||
    (
      kubectl get pods -n scheduler-plugins && kubectl describe pods -n scheduler-plugins
      exit 1
    )
fi

# wait for volcano up
if [ "${GANG_SCHEDULER_NAME}" = "volcano" ]; then
  kubectl rollout status deployment -n volcano-system volcano-admission --timeout "${TIMEOUT}s" &&
    kubectl rollout status deployment -n volcano-system volcano-scheduler --timeout "${TIMEOUT}s" &&
    kubectl rollout status deployment -n volcano-system volcano-controllers --timeout "${TIMEOUT}s" ||
    (
      kubectl get pods -n volcano-system && kubectl describe pods -n volcano-system
      exit 1
    )
fi

kubectl version
kubectl cluster-info
kubectl get nodes
kubectl get pods -n kubeflow
kubectl describe pods -n kubeflow
