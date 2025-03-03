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

# Run this script from the root location: `make generate`

set -o errexit
set -o nounset

# TODO (andreyvelich): Read this data from the global VERSION file.
SDK_VERSION="0.1.0"
SDK_OUTPUT_PATH="sdk"

OPENAPI_GENERATOR_VERSION="v7.11.0"
TRAINER_ROOT="$(pwd)"
SWAGGER_CODEGEN_CONF="hack/python-sdk/swagger_config.json"
SWAGGER_CODEGEN_FILE="api/openapi-spec/swagger.json"

# We need to add user to allow container override existing files.
echo "Generating Python SDK for Kubeflow Trainer V2 ..."
docker run --rm \
  -v "${TRAINER_ROOT}:/local" docker.io/openapitools/openapi-generator-cli:${OPENAPI_GENERATOR_VERSION} generate \
  -g python \
  -i "local/${SWAGGER_CODEGEN_FILE}" \
  -c "local/${SWAGGER_CODEGEN_CONF}" \
  -o "local/${SDK_OUTPUT_PATH}" \
  -p=packageVersion="${SDK_VERSION}" \
  --global-property models,modelTests=false
# --global-property apiTests=false,modelTests=false,supportingFiles=README.md,models

# sleep 4

echo "Removing unused files for the Python SDK"
rm -rf ${SDK_OUTPUT_PATH}/.openapi-generator
# rm -rf ${SDK_OUTPUT_PATH}/.github
# rm -rf ${SDK_OUTPUT_PATH}/.gitignore
# rm -rf ${SDK_OUTPUT_PATH}/.gitlab-ci.yml
# rm -rf ${SDK_OUTPUT_PATH}/git_push.sh
# rm -rf ${SDK_OUTPUT_PATH}/.openapi-generator-ignore
# rm -rf ${SDK_OUTPUT_PATH}/.travis.yml
# rm -rf ${SDK_OUTPUT_PATH}/requirements.txt
# rm -rf ${SDK_OUTPUT_PATH}/setup.cfg
# rm -rf ${SDK_OUTPUT_PATH}/setup.py
# rm -rf ${SDK_OUTPUT_PATH}/test-requirements.txt
# rm -rf ${SDK_OUTPUT_PATH}/tox.ini
# rm -rf ${SDK_OUTPUT_PATH}/kubeflow/trainer/py.typed

# Revert manually created files.
# git checkout ${SDK_OUTPUT_PATH}/README.md
# git checkout ${SDK_OUTPUT_PATH}/pyproject.toml
# git checkout ${SDK_OUTPUT_PATH}/kubeflow/trainer/__init__.py

# # Manually modify the SDK version in the __init__.py file.
# if [[ $(uname) == "Darwin" ]]; then
#   sed -i '' -e "s/__version__.*/__version__ = \"${SDK_VERSION}\"/" ${SDK_OUTPUT_PATH}/kubeflow/trainer/__init__.py
# else
#   sed -i -e "s/__version__.*/__version__ = \"${SDK_VERSION}\"/" ${SDK_OUTPUT_PATH}/kubeflow/trainer/__init__.py
# fi

# Kubeflow models must have Kubernetes models to perform serialization.
# cat <<EOF >>${SDK_OUTPUT_PATH}/kubeflow/trainer/models/__init__.py
# # Import Kubernetes and JobSet models for the serialization.
# from kubernetes.client import *
# from jobset.models import *
# EOF
