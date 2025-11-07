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

# Run this script from the root location: `make generate`

set -o errexit
set -o nounset

# Source container runtime utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../scripts/container-runtime.sh"

# Setup container runtime
setup_container_runtime

# TODO (andreyvelich): Read this data from the global VERSION file.
API_VERSION="2.1.0"
API_OUTPUT_PATH="api/python_api"
PKG_ROOT="${API_OUTPUT_PATH}/kubeflow_trainer_api"

OPENAPI_GENERATOR_VERSION="v7.13.0"
TRAINER_ROOT="$(pwd)"
SWAGGER_CODEGEN_CONF="hack/python-api/swagger_config.json"
SWAGGER_CODEGEN_FILE="api/openapi-spec/swagger.json"

echo "Generating Python API models for Kubeflow Trainer V2 using ${CONTAINER_RUNTIME}..."
# We need to add user to allow container override existing files.
${CONTAINER_RUNTIME} run --user "$(id -u)":"$(id -g)" --rm \
  -v "${TRAINER_ROOT}:/local" docker.io/openapitools/openapi-generator-cli:${OPENAPI_GENERATOR_VERSION} generate \
  -g python \
  -i "local/${SWAGGER_CODEGEN_FILE}" \
  -c "local/${SWAGGER_CODEGEN_CONF}" \
  -o "local/${API_OUTPUT_PATH}" \
  -p=packageVersion="${API_VERSION}" \
  --global-property models,modelTests=false,modelDocs=false,supportingFiles=__init__.py

echo "Removing unused files for the Python API"
git clean -f ${API_OUTPUT_PATH}/.openapi-generator
git clean -f ${API_OUTPUT_PATH}/.github
git clean -f ${API_OUTPUT_PATH}/test
git clean -f ${PKG_ROOT}/api

# Revert manually created files.
git checkout ${PKG_ROOT}/__init__.py

# Manually modify the SDK version in the __init__.py file.
if [[ $(uname) == "Darwin" ]]; then
  sed -i '' -e "s/__version__.*/__version__ = \"${API_VERSION}\"/" ${PKG_ROOT}/__init__.py
else
  sed -i -e "s/__version__.*/__version__ = \"${API_VERSION}\"/" ${PKG_ROOT}/__init__.py
fi
