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

# Function to load container image into KinD cluster
load_image_to_kind() {
  local image_name="$1"
  local cluster_name="${2:-}"
  local use_sudo="${3:-}"
  local cluster_arg=""

  if [[ -n "${cluster_name}" ]]; then
    cluster_arg="--name ${cluster_name}"
  fi

  echo "Loading image ${image_name} into KinD cluster${cluster_name:+ ${cluster_name}}"
  
  local kind_cmd="${KIND}"
  if [[ "${use_sudo}" == "sudo" ]]; then
    kind_cmd="sudo ${KIND}"
  fi

  if [[ "${CONTAINER_RUNTIME}" == *"docker"* ]]; then
    ${kind_cmd} load docker-image "${image_name}" ${cluster_arg}
  else
    ${CONTAINER_RUNTIME} save "${image_name}" -o /dev/stdout | ${kind_cmd} load image-archive /dev/stdin ${cluster_arg}
  fi
}
