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

echo "Generating Python SDK for Kubeflow Trainer V2 ..."
# We need to add user to allow container override existing files.
docker run --user "$(id -u)":"$(id -g)" --rm \
  -v "${TRAINER_ROOT}:/local" docker.io/openapitools/openapi-generator-cli:${OPENAPI_GENERATOR_VERSION} generate \
  -g python \
  -i "local/${SWAGGER_CODEGEN_FILE}" \
  -c "local/${SWAGGER_CODEGEN_CONF}" \
  -o "local/${SDK_OUTPUT_PATH}" \
  -p=packageVersion="${SDK_VERSION}" \
  --global-property models,modelTests=false,modelDocs=false,supportingFiles=__init__.py

echo "Removing unused files for the Python SDK"
git clean -f ${SDK_OUTPUT_PATH}/.openapi-generator
git clean -f ${SDK_OUTPUT_PATH}/.github
git clean -f ${SDK_OUTPUT_PATH}/test

# Revert manually created files.
git checkout ${SDK_OUTPUT_PATH}/kubeflow/trainer/__init__.py

# Manually modify the SDK version in the __init__.py file.
if [[ $(uname) == "Darwin" ]]; then
  sed -i '' -e "s/__version__.*/__version__ = \"${SDK_VERSION}\"/" ${SDK_OUTPUT_PATH}/kubeflow/trainer/__init__.py
else
  sed -i -e "s/__version__.*/__version__ = \"${SDK_VERSION}\"/" ${SDK_OUTPUT_PATH}/kubeflow/trainer/__init__.py
fi

# The `model_config` property conflicts with Pydantic name.
# Therefore, we rename it to `model_config_crd`
TRAINJOB_SPEC_MODEL=${SDK_OUTPUT_PATH}/kubeflow/trainer/models/trainer_v1alpha1_train_job_spec.py
if [[ $(uname) == "Darwin" ]]; then
  sed -i '' -e "s/model_config/model_config_crd/" ${TRAINJOB_SPEC_MODEL}
  sed -i '' -e "s/model_config_crd = ConfigDict/model_config = ConfigDict/" ${TRAINJOB_SPEC_MODEL}
  sed -i '' -e "s/kubeflow.trainer.models.trainer_v1alpha1_model_config_crd/kubeflow.trainer.models.trainer_v1alpha1_model_config/" ${TRAINJOB_SPEC_MODEL}
else
  sed -i -e "s/model_config/model_config_crd/" ${TRAINJOB_SPEC_MODEL}
  sed -i -e "s/model_config_crd = ConfigDict/model_config = ConfigDict/" ${TRAINJOB_SPEC_MODEL}
  sed -i -e "s/kubeflow.trainer.models.trainer_v1alpha1_model_config_crd/kubeflow.trainer.models.trainer_v1alpha1_model_config/" ${TRAINJOB_SPEC_MODEL}
fi
