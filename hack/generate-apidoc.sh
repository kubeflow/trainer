#!/bin/bash

# Copyright 2017 The Kubernetes Authors.
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

# This shell is used to auto generate some useful tools for k8s, such as lister,
# informer, deepcopy, defaulter and so on.

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..

cd ${SCRIPT_ROOT}

CRD_REF_DOCS_PATH=$(go env GOPATH)/bin
CRD_REF_GEN_VERSION=v0.3.0
go install github.com/elastic/crd-ref-docs@${CRD_REF_GEN_VERSION}

${CRD_REF_DOCS_PATH}/crd-ref-docs --log-level DEBUG\
    --source-path ./pkg/apis/kubeflow.org/v1 \
		--config ./docs/api/autogen/config.yaml \
		--templates-dir ./docs/api/autogen/templates \
		--output-path ./docs/api/kubeflow.org_v1_generated.asciidoc \
		--max-depth 30

cd - > /dev/null
