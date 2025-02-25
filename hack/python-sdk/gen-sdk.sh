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

SWAGGER_JAR_URL="https://repo1.maven.org/maven2/org/openapitools/openapi-generator-cli/4.3.1/openapi-generator-cli-4.3.1.jar"
SWAGGER_CODEGEN_JAR="hack/python-sdk/openapi-generator-cli.jar"
SWAGGER_CODEGEN_CONF="hack/python-sdk/swagger_config.json"
SWAGGER_CODEGEN_FILE="api/openapi-spec/swagger.json"

if [[ ! -f "$SWAGGER_CODEGEN_JAR" ]]; then
  echo "Downloading the openapi-generator-cli JAR package to generate SDK"
  wget -O "${SWAGGER_CODEGEN_JAR}" ${SWAGGER_JAR_URL}
fi

echo "Generating Python SDK for Kubeflow Trainer V2 ..."
java -jar "${SWAGGER_CODEGEN_JAR}" generate -i "${SWAGGER_CODEGEN_FILE}" -g python \
  -o "${SDK_OUTPUT_PATH}" \
  -c "${SWAGGER_CODEGEN_CONF}" \
  -p=packageVersion="${SDK_VERSION}" \
  --global-property apiTests=false,modelTests=false # TODO (andreyvelich): Discuss if we should use these test files.

echo "Removing unused files for the Python SDK"
git clean -f ${SDK_OUTPUT_PATH}/.openapi-generator
git clean -f ${SDK_OUTPUT_PATH}/.gitignore
git clean -f ${SDK_OUTPUT_PATH}/.gitlab-ci.yml
git clean -f ${SDK_OUTPUT_PATH}/git_push.sh
git clean -f ${SDK_OUTPUT_PATH}/.openapi-generator-ignore
git clean -f ${SDK_OUTPUT_PATH}/.travis.yml
git clean -f ${SDK_OUTPUT_PATH}/requirements.txt
git clean -f ${SDK_OUTPUT_PATH}/setup.cfg
git clean -f ${SDK_OUTPUT_PATH}/setup.py
git clean -f ${SDK_OUTPUT_PATH}/test-requirements.txt
git clean -f ${SDK_OUTPUT_PATH}/tox.ini

# Revert the README since it is manually created.
git checkout ${SDK_OUTPUT_PATH}/README.md
git checkout ${SDK_OUTPUT_PATH}/kubeflow/trainer/__init__.py

# Manually modify the SDK version in the __init__.py file.
if [[ $(uname) == "Darwin" ]]; then
  sed -i '' -e "s/__version__.*/__version__ = \"${SDK_VERSION}\"/" ${SDK_OUTPUT_PATH}/kubeflow/trainer/__init__.py
else
  sed -i -e "s/__version__.*/__version__ = \"${SDK_VERSION}\"/" ${SDK_OUTPUT_PATH}/kubeflow/trainer/__init__.py
fi

# Kubeflow models must have Kubernetes models to perform serialization.
printf "\n# Import JobSet models for the serialization. It imports the Kubernetes models.\n" >>${SDK_OUTPUT_PATH}/kubeflow/trainer/models/__init__.py
printf "from jobset.models import *\n" >>${SDK_OUTPUT_PATH}/kubeflow/trainer/models/__init__.py
