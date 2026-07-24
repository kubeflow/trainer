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

# Claude Code PostToolUse hook: format the file the agent just edited.
# The hook receives the tool payload as JSON on stdin; the edited file
# path is at .tool_input.file_path.
#
# python3 is used for JSON parsing because it is already a prerequisite
# of this repo's dev workflow (make verify-boilerplate, pre-commit).
# If it is unavailable, the hook silently does nothing.

set -euo pipefail

command -v python3 >/dev/null 2>&1 || exit 0

file_path=$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("tool_input",{}).get("file_path",""))') || exit 0

case "$file_path" in
  *.go) gofmt -w "$file_path" ;;
  *.py) pre-commit run --files "$file_path" >/dev/null 2>&1 || true ;;
esac
