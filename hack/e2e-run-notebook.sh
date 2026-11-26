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

# This shell is used to run Jupyter Notebook with Papermill.

set -o errexit
set -o nounset
set -o pipefail
set -x

if [ -z "${NOTEBOOK_INPUT}" ]; then
    echo "NOTEBOOK_INPUT env variable must be set to run this script."
    exit 1
fi

if [ -z "${NOTEBOOK_OUTPUT}" ]; then
    echo "NOTEBOOK_OUTPUT env variable must be set to run this script."
    exit 1
fi

if [ -z "${PAPERMILL_TIMEOUT}" ]; then
    echo "PAPERMILL_TIMEOUT env variable must be set to run this script."
    exit 1
fi

# PAPERMILL_PARAMS should contain full papermill parameter flags.
# Example: "-p num_cpu 3 -p gpu 1"
PAPERMILL_PARAMS="${PAPERMILL_PARAMS:-}"

print_results() {
    # Only run kubectl commands if we're testing Kubernetes notebooks
    if command -v kubectl &> /dev/null && kubectl cluster-info &> /dev/null; then
        echo "=== TrainJob Status ==="
        kubectl describe trainjob || true

        echo "=== JobSet Status & Events ==="
        kubectl describe jobset || true

        echo "=== Controller Manager Logs ==="
        kubectl logs -n kubeflow-system -l app.kubernetes.io/name=trainer --tail 100 || true

        echo "=== Cluster Pods Summary ==="
        kubectl get pods -o wide || true

        # Discover all training pods (MPI launcher & workers, PyTorch nodes, etc.)
        local training_pods
        training_pods=$(kubectl get pods -l jobset.sigs.k8s.io/jobset-name -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || true)
        if [ -z "${training_pods}" ]; then
            training_pods=$(kubectl get pods -l jobset.sigs.k8s.io/replicatedjob-name -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || true)
        fi

        if [ -n "${training_pods}" ]; then
            echo "Found training pods: ${training_pods}"
            for pod in ${training_pods}; do
                local role
                role=$(kubectl get pod "${pod}" -o jsonpath='{.metadata.labels.jobset\.sigs\.k8s\.io/replicatedjob-name}' 2>/dev/null || echo "unknown")
                echo "=================================================="
                echo "=== Pod: ${pod} (Role: ${role}) ==="
                echo "=================================================="
                echo "--- Pod Description & Events ---"
                kubectl describe pod "${pod}" || true
                echo "--- Pod Current Logs ---"
                kubectl logs "${pod}" --all-containers=true --prefix=true --tail=-1 || true
                echo "--- Pod Previous Logs (if restarted) ---"
                kubectl logs "${pod}" --all-containers=true --prefix=true --previous=true --tail=-1 2>/dev/null || true
            done
        else
            echo "No training pods found (local backend used - training runs outside Kubernetes)"
        fi

        kubectl wait trainjob --for=condition=Complete --all --timeout 30s || true
    else
        echo "Skipping kubectl commands (not a Kubernetes test)"
    fi
}

(papermill "${NOTEBOOK_INPUT}" "${NOTEBOOK_OUTPUT}" ${PAPERMILL_PARAMS} --execution-timeout "${PAPERMILL_TIMEOUT}" && print_results) ||
    (print_results && exit 1)
