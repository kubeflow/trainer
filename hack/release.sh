#!/usr/bin/env bash

# Copyright 2026 The Kubeflow Authors.
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

# This shell is used to prepare a release commit for X.Y.Z version.

set -o errexit
set -o nounset
set -o pipefail

if [ "$#" -eq 0 ]; then
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

REPO_ROOT="$(dirname "$0")/.."
VERSION_FILE="$REPO_ROOT/VERSION"
MANIFESTS_DIR="$REPO_ROOT/manifests"
CHART_DIR="$REPO_ROOT/charts/kubeflow-trainer"
CHART_FILE="$CHART_DIR/Chart.yaml"
PYTHON_API_VERSION_FILE="$REPO_ROOT/api/python_api/kubeflow_trainer_api/__init__.py"

# Verify tag doesn't already exist
git fetch --tags
if git tag --list | grep -q "^${TAG}$"; then
  echo "Tag: ${TAG} already exists. Release can't be published."
  exit 1
fi

echo -e "\nPreparing release commit for ${TAG}\n"

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

# Update configmap version in manager overlay
MANAGER_KUSTOMIZE="$MANIFESTS_DIR/overlays/manager/kustomization.yaml"
if [ -f "$MANAGER_KUSTOMIZE" ]; then
  sed -i "s/kubeflow_trainer_version=.*/kubeflow_trainer_version=$TAG/" "$MANAGER_KUSTOMIZE"
  echo "Updated configmap version in manager overlay to $TAG"
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

# Update Python API version
sed -i "s/__version__ = \".*\"/__version__ = \"$NEW_VERSION\"/" "$PYTHON_API_VERSION_FILE"
echo "Updated Python API version to $NEW_VERSION"

CHANGELOG_PATH="$REPO_ROOT/CHANGELOG.md"
echo "Generating changelog for $TAG"
ABSOLUTE_REPO_ROOT="$(cd "$REPO_ROOT" && pwd)"
if [ -z "${GITHUB_TOKEN:-}" ]; then
  echo "WARNING: GITHUB_TOKEN not set. Set it to avoid GitHub API rate limits."
  echo "Export GITHUB_TOKEN before running this script: export GITHUB_TOKEN=your_token"
fi

# Generate and prepend new changelog section
TEMP_FILE=$(mktemp)
docker run --rm -u "$(id -u):$(id -g)" -v "$ABSOLUTE_REPO_ROOT:/app" \
  -e "GITHUB_TOKEN=${GITHUB_TOKEN:-}" -w /app \
  "ghcr.io/orhun/git-cliff/git-cliff:latest" --unreleased --tag "$TAG" -o - > "$TEMP_FILE"

if [ -f "$CHANGELOG_PATH" ]; then
  sed -i "1 r $TEMP_FILE" "$CHANGELOG_PATH"
else
  { echo "# Changelog"; cat "$TEMP_FILE"; } > "$CHANGELOG_PATH"
fi
rm "$TEMP_FILE"
echo "Changelog generated at $CHANGELOG_PATH"

echo "Running make generate"
make -C "$REPO_ROOT" generate
echo "Completed make generate"

git add "$VERSION_FILE" "$MANIFESTS_DIR" "$CHART_DIR" "$REPO_ROOT/api" "$REPO_ROOT/pkg/apis" "$REPO_ROOT/pkg/client" "$CHANGELOG_PATH"
git commit -s -m "chore(release): Release $TAG"

echo -e "\nRelease commit for $TAG created successfully."
echo "Next steps:"
echo "  1. Push your branch to your fork"
echo "  2. Open a PR against master (add the 'area/release' label to skip PR title check)"
echo "  3. Once merged, GitHub Actions will create the release branch and tag"
