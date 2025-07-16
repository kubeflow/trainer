#!/usr/bin/env bash

# Copyright 2021 The Kubeflow Authors.
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

set -o errexit
set -o nounset
set -o pipefail

repo_root="$(dirname "${BASH_SOURCE}")/../.."

SWAGGER_JAR_URL="https://repo1.maven.org/maven2/org/openapitools/openapi-generator-cli/4.3.1/openapi-generator-cli-4.3.1.jar"
SWAGGER_CODEGEN_JAR="${repo_root}/hack/python-sdk/openapi-generator-cli.jar"
SWAGGER_CODEGEN_CONF="${repo_root}/hack/python-sdk/swagger_config.json"
SDK_OUTPUT_PATH="${repo_root}/sdk/python"
VERSION=1.9.3
SWAGGER_CODEGEN_FILE="${repo_root}/hack/python-sdk/swagger.json"

if [ -z "${GOPATH:-}" ]; then
  export GOPATH=$(go env GOPATH)
fi

echo "Generating OpenAPI specification ..."
echo "./hack/update-codegen.sh already help us generate openapi specs ..."

if [[ ! -f "$SWAGGER_CODEGEN_JAR" ]]; then
  echo "Downloading the swagger-codegen JAR package ..."
  wget -O "${SWAGGER_CODEGEN_JAR}" ${SWAGGER_JAR_URL}
fi

echo "Generating swagger file ..."
go run "${repo_root}"/hack/swagger/main.go ${VERSION} >"${SWAGGER_CODEGEN_FILE}"

echo "Removing previously generated files ..."
rm -rf "${SDK_OUTPUT_PATH}"/docs/KubeflowOrgV1*.md "${SDK_OUTPUT_PATH}"/kubeflow/training/models "${SDK_OUTPUT_PATH}"/kubeflow/training/*.py "${SDK_OUTPUT_PATH}"/test/test_*.py
echo "Generating Python SDK for Training Operator ..."
java -jar "${SWAGGER_CODEGEN_JAR}" generate -i "${repo_root}"/hack/python-sdk/swagger.json -g python --global-property apiTests=false,modelTests=false -o "${SDK_OUTPUT_PATH}" -c "${SWAGGER_CODEGEN_CONF}"

echo "Kubeflow Training Operator Python SDK is generated successfully to folder ${SDK_OUTPUT_PATH}/."

echo "Running post-generation script ..."
"${repo_root}"/hack/python-sdk/post_gen.py
