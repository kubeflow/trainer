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
        # Always show TrainJob status
        kubectl describe trainjob
        kubectl logs -n kubeflow-system -l app.kubernetes.io/name=trainer

        # Collect pod logs BEFORE waiting on completion. A failed TrainJob
        # never satisfies --for=condition=Complete, so the wait below would
        # block for its full timeout, during which JobSet garbage-collects the
        # failed pods and their logs are lost. Dumping logs first captures the
        # launcher/node tracebacks that explain the failure.
        #
        # JobSet labels every training pod with jobset-name. MPI runtimes
        # (DeepSpeed, MLX) create "launcher" and "node" jobs, while other
        # runtimes create only "node", so dump logs from all of them to help
        # debug failures.
        if kubectl get pods -l jobset.sigs.k8s.io/jobset-name --no-headers 2>/dev/null | grep -q .; then
            echo "Found training pods - showing pod details and logs"
            kubectl get pods
            for pod in $(kubectl get pods -l jobset.sigs.k8s.io/jobset-name -o name); do
                echo "----- describe ${pod} -----"
                kubectl describe "${pod}" || true
                echo "----- logs ${pod} -----"
                kubectl logs "${pod}" --all-containers --prefix --tail=-1 || true
            done
        else
            echo "No training pods found (local backend used - training runs outside Kubernetes)"
        fi

        kubectl wait trainjob --for=condition=Complete --all --timeout 30s
    else
        echo "Skipping kubectl commands (not a Kubernetes test)"
    fi
}

(papermill "${NOTEBOOK_INPUT}" "${NOTEBOOK_OUTPUT}" ${PAPERMILL_PARAMS} --execution-timeout "${PAPERMILL_TIMEOUT}" && print_results) ||
    (print_results && exit 1)
