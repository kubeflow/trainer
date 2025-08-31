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

# This shell is used to setup Kind cluster for Kubeflow Trainer e2e tests.

set -o errexit
set -o nounset
set -o pipefail
set -x

# Find all clusters with prefix "nvkind"
CLUSTERS=$(kind get clusters | grep '^nvkind' || true)

if [[ -z "${CLUSTERS}" ]]; then
  echo "No nvkind clusters found. Nothing to delete."
  exit 0
fi

for CLUSTER_NAME in ${CLUSTERS}; do
  echo "Deleting Kind cluster: ${CLUSTER_NAME}"
  if kind delete cluster --name "${CLUSTER_NAME}"; then
    echo "Successfully deleted ${CLUSTER_NAME}"
  else
    echo "Warning: Failed to delete ${CLUSTER_NAME}, continuing..."
  fi
done
