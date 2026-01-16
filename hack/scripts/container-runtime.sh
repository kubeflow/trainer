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

# Container runtime detection and setup utilities

# Detect available container runtime (Docker or Podman)
detect_container_runtime() {
  if command -v docker &> /dev/null; then
    echo "docker"
  elif command -v podman &> /dev/null; then
    echo "podman"
  else
    echo "Error: Neither docker nor podman found" >&2
    exit 1
  fi
}

# Setup container runtime variable
# Uses provided CONTAINER_RUNTIME environment variable or auto-detects
setup_container_runtime() {
  if [[ -z "${CONTAINER_RUNTIME:-}" ]]; then
    CONTAINER_RUNTIME=$(detect_container_runtime)
  fi

  if [[ "${1:-}" == "sudo" ]]; then
    CONTAINER_RUNTIME="sudo ${CONTAINER_RUNTIME}"
  fi
  export CONTAINER_RUNTIME
}
