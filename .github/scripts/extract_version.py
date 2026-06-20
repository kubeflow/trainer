#!/usr/bin/env python3

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
"""
Extract package version from uv tree output.

This script parses uv tree output to extract the full version
(including pre-release, post-release, dev versions) of a package.

Usage:
    uv tree --package <package> | python extract_version.py <package>

Exit codes:
    0 - version found and printed to stdout
    1 - version not found or error
"""

import re
import sys


def main():
    if len(sys.argv) != 2:
        print(
            "Usage: uv tree --package <pkg> | extract_version.py <pkg>", file=sys.stderr
        )
        sys.exit(1)

    package_name = sys.argv[1]

    # Read from stdin (piped from uv tree)
    tree_output = sys.stdin.read()

    # Look for pattern: "package_name vX.Y.Z"
    # Using non-greedy match to get version until whitespace
    pattern = rf"{re.escape(package_name)}\s+v([^\s]+)"
    match = re.search(pattern, tree_output)

    if match:
        version = match.group(1)
        print(version)
        sys.exit(0)
    else:
        print(f"Error: Could not find version for {package_name}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
