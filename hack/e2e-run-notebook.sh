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

# State for background log streaming.
POD_LOG_SEEN_DIR="$(mktemp -d)"
POD_LOG_STREAM_PID=""

# Follow a single pod's logs until its stream ends, retrying while the
# container is still starting up. Writes to stdout so the logs appear in the
# CI job output.
follow_pod_logs() {
    { set +x; } 2>/dev/null
    local pod="$1"
    echo "----- streaming logs ${pod} -----"
    # kubectl logs -f exits non-zero if the container is not ready yet; retry
    # until it attaches and streams to completion, or the pod disappears.
    while kubectl get "${pod}" &> /dev/null; do
        if kubectl logs -f "${pod}" --all-containers --prefix --tail=-1; then
            return
        fi
        sleep 2
    done
}

# Continuously discover training pods and stream their logs as soon as they
# appear. A post-run "kubectl logs" cannot see pods that JobSet garbage-collects
# immediately on failure (e.g. a failed MPI launcher for the DeepSpeed and MLX
# runtimes); streaming live captures their tracebacks before deletion.
#
# JobSet labels every training pod with jobset-name. MPI runtimes create
# "launcher" and "node" jobs, while other runtimes create only "node", so this
# follows all of them.
stream_pod_logs() {
    { set +x; } 2>/dev/null
    while true; do
        for pod in $(kubectl get pods -l jobset.sigs.k8s.io/jobset-name -o name 2>/dev/null); do
            local marker="${POD_LOG_SEEN_DIR}/${pod//\//_}"
            if [ ! -e "${marker}" ]; then
                touch "${marker}"
                follow_pod_logs "${pod}" &
            fi
        done
        sleep 2
    done
}

start_log_streaming() {
    if command -v kubectl &> /dev/null && kubectl cluster-info &> /dev/null; then
        stream_pod_logs &
        POD_LOG_STREAM_PID=$!
    fi
}

stop_log_streaming() {
    if [ -n "${POD_LOG_STREAM_PID}" ]; then
        # Stop the discovery loop; per-pod followers self-terminate when their
        # pod's log stream ends.
        kill "${POD_LOG_STREAM_PID}" 2>/dev/null || true
        wait "${POD_LOG_STREAM_PID}" 2>/dev/null || true
        POD_LOG_STREAM_PID=""
    fi
}

print_results() {
    # Only run kubectl commands if we're testing Kubernetes notebooks
    if command -v kubectl &> /dev/null && kubectl cluster-info &> /dev/null; then
        # Stop live streaming; per-pod logs have already been printed above.
        stop_log_streaming

        # Always show TrainJob status.
        kubectl describe trainjob
        kubectl logs -n kubeflow-system -l app.kubernetes.io/name=trainer

        # Describe pods that still exist for extra failure context. Failed pods
        # may already be garbage-collected, but their logs were captured live by
        # the background streamer above.
        if kubectl get pods -l jobset.sigs.k8s.io/jobset-name --no-headers 2>/dev/null | grep -q .; then
            echo "Surviving training pods:"
            kubectl get pods
            for pod in $(kubectl get pods -l jobset.sigs.k8s.io/jobset-name -o name); do
                echo "----- describe ${pod} -----"
                kubectl describe "${pod}" || true
            done
        else
            echo "No training pods found (local backend used - training runs outside Kubernetes)"
        fi

        kubectl wait trainjob --for=condition=Complete --all --timeout 30s
    else
        echo "Skipping kubectl commands (not a Kubernetes test)"
    fi
}

# Start streaming pod logs before the notebook runs so failed pods are captured
# before JobSet can garbage-collect them.
start_log_streaming

(papermill "${NOTEBOOK_INPUT}" "${NOTEBOOK_OUTPUT}" ${PAPERMILL_PARAMS} --execution-timeout "${PAPERMILL_TIMEOUT}" && print_results) ||
    (print_results && exit 1)
