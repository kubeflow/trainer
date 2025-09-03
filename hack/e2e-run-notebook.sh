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

print_results() {
    kubectl get pods
    kubectl describe pod
    kubectl describe trainjob
    kubectl logs -n kubeflow-system -l app.kubernetes.io/name=trainer
    kubectl logs -l jobset.sigs.k8s.io/replicatedjob-name=trainer-node,batch.kubernetes.io/job-completion-index=0 --tail -1
    kubectl wait trainjob --for=condition=Complete --all --timeout 3s
}

(papermill "${NOTEBOOK_INPUT}" "${NOTEBOOK_OUTPUT}" --execution-timeout "${PAPERMILL_TIMEOUT}" && print_results) ||
    (print_results && exit 1)
