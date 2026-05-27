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

# This shell is used to prepare a release commit for X.Y.Z version.
# It updates VERSION, Helm chart version, changelog, and runs make generate.
#
# Manifest image tags and configMapGenerator version are NOT updated here.
# Those are pinned on the release branch by the release workflow (release.yaml)
# so that the master branch keeps "latest"/"dev" values.

set -o errexit
set -o nounset
set -o pipefail

if [ -z "${1:-}" ]; then
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

MAJOR_VERSION="${NEW_VERSION%%.*}"
MINOR_VERSION="${NEW_VERSION#*.}"
MINOR_VERSION="${MINOR_VERSION%%.*}"

REPO_ROOT="$(dirname "$0")/.."
VERSION_FILE="$REPO_ROOT/VERSION"
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

CHANGELOG_DIR="$REPO_ROOT/CHANGELOG"
CHANGELOG_PATH="$CHANGELOG_DIR/CHANGELOG-${MAJOR_VERSION}.${MINOR_VERSION}.md"
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

# Abort if git-cliff produced empty output.
if [ ! -s "$TEMP_FILE" ]; then
  echo "git-cliff produced empty changelog" >&2
  rm -f "$TEMP_FILE"
  exit 1
fi

mkdir -p "$CHANGELOG_DIR"

if [ -f "$CHANGELOG_PATH" ]; then
  # Prepend new section to existing changelog using portable approach.
  TMP_COMBINED=$(mktemp)
  cat "$TEMP_FILE" "$CHANGELOG_PATH" > "$TMP_COMBINED"
  mv "$TMP_COMBINED" "$CHANGELOG_PATH"
else
  { echo "# Changelog"; echo ""; cat "$TEMP_FILE"; } > "$CHANGELOG_PATH"
fi
rm -f "$TEMP_FILE"
echo "Changelog generated at $CHANGELOG_PATH"

echo "Running make generate"
make -C "$REPO_ROOT" generate
echo "Completed make generate"

git add "$VERSION_FILE" "$CHART_DIR" "$PYTHON_API_VERSION_FILE" "$CHANGELOG_PATH"
# Also stage any files modified by make generate (CRDs, OpenAPI specs, Python API models).
git add -u
git commit -s -m "Release $TAG"

echo -e "\nRelease commit for $TAG created successfully."
echo "Next steps:"
echo "  1. Push your branch and open a PR to 'master'"
echo "  2. Once merged, GitHub Actions will create the release branch, tag, and release"
