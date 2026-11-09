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
        kubectl wait trainjob --for=condition=Complete --all --timeout 30s

        # Only check pod logs if pods exist (not for local backends)
        if kubectl get pods -l jobset.sigs.k8s.io/replicatedjob-name=trainer-node --no-headers 2>/dev/null | grep -q .; then
            echo "Found training pods - showing pod details and logs"
            kubectl get pods
            kubectl describe pod
            kubectl logs -l jobset.sigs.k8s.io/replicatedjob-name=trainer-node,batch.kubernetes.io/job-completion-index=0 --tail -1
        else
            echo "No training pods found (local backend used - training runs outside Kubernetes)"
        fi
    else
        echo "Skipping kubectl commands (not a Kubernetes test)"
    fi
}

# Local container backends run training in Docker containers outside Kubernetes, so
# print_results shows nothing for them. Show their state and logs, so a notebook that
# hangs or fails leaves something to debug from. Best effort: it never fails the script.
print_local_containers() {
    if ! command -v docker &> /dev/null || ! docker info &> /dev/null; then
        return 0
    fi

    # Containers created by the SDK container backend carry this label.
    local filter="label=trainer.kubeflow.org/trainjob-name"
    local ids
    ids="$(docker ps -aq --filter "${filter}" || true)"
    if [ -z "${ids}" ]; then
        return 0
    fi

    echo "Found local backend containers - showing state and logs"
    docker ps -a --filter "${filter}" || true
    local id
    for id in ${ids}; do
        # A container that is still running may be hung, so show what it is doing.
        if [ "$(docker inspect --format '{{.State.Running}}' "${id}" || true)" = "true" ]; then
            echo "Processes in container ${id}:"
            docker top "${id}" || true
        fi
        echo "Last 200 log lines of container ${id}:"
        docker logs --tail 200 "${id}" 2>&1 || true
    done
    return 0
}

(papermill "${NOTEBOOK_INPUT}" "${NOTEBOOK_OUTPUT}" ${PAPERMILL_PARAMS} --execution-timeout "${PAPERMILL_TIMEOUT}" && print_results) ||
    (print_results && print_local_containers && exit 1)
