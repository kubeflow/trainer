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

# This bash script is used to run the example notebooks

set -o errexit
set -o nounset
set -o pipefail

NOTEBOOK_INPUT=""
NOTEBOOK_OUTPUT="-" # outputs to console
NAMESPACE="default"
TRAINING_PYTHON_SDK="./sdk/python"

usage() {
  echo "Usage: $0 -i <input_notebook> -o <output_notebook> [-p \"<param> <value>\"...] [-y <params.yaml>]"
  echo "Options:"
  echo "  -i  Input notebook (required)"
  echo "  -o  Output notebook (required)"
  echo "  -k  Kubeflow Training Operator Python SDK (optional)"
  echo "  -n  Kubernetes namespace used by tests (optional)"
  echo "  -h  Show this help message"
  echo "NOTE: papermill, jupyter and ipykernel are required Python dependencies to run Notebooks"
  exit 1
}

while getopts "i:o:p:k:n:r:d:h:" opt; do
  case "$opt" in
  i) NOTEBOOK_INPUT="$OPTARG" ;;      # -i for notebook input path
  o) NOTEBOOK_OUTPUT="$OPTARG" ;;     # -o for notebook output path
  k) TRAINING_PYTHON_SDK="$OPTARG" ;; # -k for training operator python sdk
  n) NAMESPACE="$OPTARG" ;;           # -n for kubernetes namespace used by tests
  h) usage ;;                         # -h for help (usage)
  *)
    usage
    exit 1
    ;;
  esac
done

if [ -z "$NOTEBOOK_INPUT" ]; then
  echo "Error: -i notebook input path is required."
  exit 1
fi

papermill_cmd="papermill $NOTEBOOK_INPUT $NOTEBOOK_OUTPUT -p training_python_sdk $TRAINING_PYTHON_SDK -p namespace $NAMESPACE"

if ! command -v papermill &>/dev/null; then
  echo "Error: papermill is not installed. Please install papermill to proceed."
  exit 1
fi

echo "Running command: $papermill_cmd"
$papermill_cmd

if [ $? -ne 0 ]; then
  echo "Error: papermill execution failed." >&2
  exit 1
fi

echo "Notebook execution completed successfully"
