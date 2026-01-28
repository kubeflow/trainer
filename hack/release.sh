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

# This shell is used to generate a release for X.Y.Z version.

set -o errexit
set -o nounset
set -o pipefail

if [ -z "$1" ]; then
  echo "Usage: $0 <version>"
  echo "You must follow this format: X.Y.Z or X.Y.Z-rc.N"
  exit 1
fi

NEW_VERSION=$(echo "$1" | tr -d '\n' | tr -d ' ')

if [[ ! "$NEW_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-rc\.[0-9]+)?$ ]]; then
  echo "Version format is invalid. Use: X.Y.Z or X.Y.Z-rc.N"
  exit 1
fi

TAG="v$NEW_VERSION"
export TAG

REPO_ROOT="$(dirname "$0")/.."
VERSION_FILE="$REPO_ROOT/VERSION"
MANIFESTS_DIR="$REPO_ROOT/manifests"
CHART_DIR="$REPO_ROOT/charts/kubeflow-trainer"
CHART_FILE="$CHART_DIR/Chart.yaml"
PYTHON_API_VERSION_FILE="$REPO_ROOT/api/python_api/kubeflow_trainer_api/__init__.py"


# Identify branch and ensure it's up to date if it tracks a remote
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
UPSTREAM=$(git rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>/dev/null || true)

# Fetch refs and verify tag absence
git fetch --tags
git fetch origin master
if [ -n "$UPSTREAM" ]; then
  git pull --ff-only
fi
if git tag --list | grep -q "^${TAG}$"; then
  echo "Tag: ${TAG} already exists. Release can't be published."
  exit 1
fi

# Ensure current branch contains origin/master
if ! git merge-base --is-ancestor origin/master HEAD; then
  echo "Current branch ${CURRENT_BRANCH} is not up to date with origin/master. Please rebase or merge origin/master first."
  exit 1
fi

echo -e "\nCreating a new release commit on branch ${CURRENT_BRANCH}. Tag to be created: ${TAG}\n"

echo -n "v$NEW_VERSION" > "$VERSION_FILE"
echo "Updated VERSION file to $NEW_VERSION"

# Update image tags in manifests
find "$MANIFESTS_DIR" -type f -name '*.yaml' -exec sed -i "s/newTag: .*/newTag: $TAG/" {} +
echo "Updated image tags in manifests to $TAG"

echo "Pinning ghcr.io image references in manifests to $TAG"
CHANGED_FILES=$(grep -REl "ghcr\.io/kubeflow/trainer/[A-Za-z0-9._/-]+:latest" "$MANIFESTS_DIR" || true)
if [ -n "$CHANGED_FILES" ]; then
  while IFS= read -r f; do
    sed -i -E "s|(ghcr\.io/kubeflow/trainer/[A-Za-z0-9._/-]+):latest|\\1:${TAG}|g" "$f"
    echo "  Updated ${f#$MANIFESTS_DIR/}"
  done <<< "$CHANGED_FILES"
else
  echo "  No ghcr.io references pinned to :latest found."
fi

if [ ! -f "$CHART_FILE" ]; then
  echo "Helm chart file not found: $CHART_FILE"
  exit 1
fi

python3 - "$CHART_FILE" "$NEW_VERSION" <<'PYTHON'
import pathlib
import re
import sys

chart_path = pathlib.Path(sys.argv[1])
new_version = sys.argv[2]
data = chart_path.read_text()
pattern = re.compile(r"^version:\s*.+$", re.MULTILINE)

if not pattern.search(data):
  print("Unable to locate version field in chart file.")
  sys.exit(1)

chart_path.write_text(pattern.sub(f"version: {new_version}", data, count=1))
PYTHON
echo "Updated Helm chart version to $NEW_VERSION"


echo "Running make generate"
make -C "$REPO_ROOT" generate
echo "Completed make generate"

git add "$VERSION_FILE" "$MANIFESTS_DIR" "$CHART_DIR" "$PYTHON_API_VERSION_FILE"
git commit -s -m "Release $TAG"

echo -e "\nRelease $NEW_VERSION is ready. Commit created locally on branch ${CURRENT_BRANCH}."
echo "Open a PR with this commit; pushing is intentionally not done by the script."