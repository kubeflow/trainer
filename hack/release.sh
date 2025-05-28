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

RELEASE_BRANCH="release-$NEW_VERSION"
TAG="v$NEW_VERSION"

VERSION_FILE="$(dirname "$0")/../VERSION"
MANIFESTS_DIR="$(dirname "$0")/../manifests"

# Fetch and check if tag already exists
git fetch --tags
git fetch origin "$RELEASE_BRANCH" || true
if git tag --list | grep -q "^${TAG}$"; then
  echo "Tag: ${TAG} already exists. Release can't be published."
  exit 1
fi

echo -e "\nCreating a new release. Branch: ${RELEASE_BRANCH}, Tag: ${TAG}\n"

# Check if branch exists, else create it
if git branch -r | grep -q "origin/${RELEASE_BRANCH}"; then
  echo "Branch: ${RELEASE_BRANCH} already exists. Switching to the branch."
  git checkout "$RELEASE_BRANCH"
else
  echo "Branch: ${RELEASE_BRANCH} does not exist. Creating a new release branch."
  git checkout master
  git pull origin master
  git checkout -b "$RELEASE_BRANCH"
fi

echo "$NEW_VERSION" > "$VERSION_FILE"
echo "Updated VERSION file to $NEW_VERSION"

# Update image tags in manifests
find "$MANIFESTS_DIR" -type f -name '*.yaml' -exec sed -i "s/newTag: .*/newTag: $TAG/" {} +
echo "Updated image tags in manifests to $TAG"

git add "$VERSION_FILE" "$MANIFESTS_DIR"
git commit -s -m "Release $TAG"

echo -e "\nRelease $NEW_VERSION is ready."
read -rp "Do you want to push the branch and commit to origin? [y|n] "
if [ "$REPLY" != "y" ]; then
  echo "Push aborted."
  exit 1
fi

git push origin "$RELEASE_BRANCH"

echo -e "\nRelease $TAG has been published on branch $RELEASE_BRANCH."
