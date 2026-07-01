#!/usr/bin/env bash

# Copyright The Kubeflow Authors.
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

# Verifies that the tool-specific AI agent config (.claude/, .cursor/) is in
# sync with the vendor-neutral ai/ directory.

set -euo pipefail

./hack/sync-agents-config.sh > /dev/null

bad_files=$(git status --porcelain -- .claude .cursor)

if [[ -n ${bad_files} ]]; then
    echo "!!! AI agent config is out of sync with ai/:"
    echo "${bad_files}"
    echo "Try running 'make sync-agents-config'"
    exit 1
fi

echo "AI agent config is in sync with ai/."
