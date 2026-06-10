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
Update override-dependencies in pyproject.toml.

This script manages the [tool.uv] override-dependencies section by:
1. Reading existing overrides
2. Adding or updating a package override
3. Rewriting the entire section in consistent multi-line format

Usage:
    python update_overrides.py <package> <target> <date> <advisory_url>

Example:
    python update_overrides.py requests "requests==2.31.0" "2025-01-15" "https://..."
"""

import re
import sys
from pathlib import Path


def main():
    if len(sys.argv) != 5:
        print(
            "Usage: update_overrides.py <package> <target> <date> <advisory_url>",
            file=sys.stderr,
        )
        sys.exit(1)

    package = sys.argv[1]
    target = sys.argv[2]
    current_date = sys.argv[3]
    advisory_url = sys.argv[4]

    pyproject_path = Path("pyproject.toml")
    if not pyproject_path.exists():
        print(f"Error: {pyproject_path} not found", file=sys.stderr)
        sys.exit(1)

    # Read current pyproject.toml
    content = pyproject_path.read_text()

    comment = f"# {target} - Added {current_date} for security fix - {advisory_url}"

    # Extract existing override-dependencies
    overrides = {}  # pkg_name -> (full_spec, comment)
    has_tool_uv = "[tool.uv]" in content
    has_overrides = "override-dependencies" in content

    if has_overrides:
        # Parse existing override-dependencies (handles both single-line and multi-line)
        in_override = False
        last_comment = None
        for line in content.split("\n"):
            if "override-dependencies" in line and "=" in line:
                in_override = True
                # Check if it's a single-line array: override-dependencies = ["pkg1", "pkg2"]
                single_line_match = re.search(
                    r"override-dependencies\s*=\s*\[(.*?)\]", line
                )
                if single_line_match:
                    # Parse all packages from single line
                    for pkg_match in re.finditer(
                        r'"([^"]+)"', single_line_match.group(1)
                    ):
                        spec = pkg_match.group(1)
                        pkg_name = (
                            spec.split("==")[0] if "==" in spec else spec.split("[")[0]
                        )
                        overrides[pkg_name] = (spec, None)
                    break  # Single-line format, done parsing
                continue
            if in_override:
                if line.strip() == "]":
                    break
                # Check for comment lines
                if line.strip().startswith("#"):
                    last_comment = line.strip()
                    continue
                # Extract package spec
                match = re.search(r'"([^"]+)"', line)
                if match:
                    spec = match.group(1)
                    pkg_name = (
                        spec.split("==")[0] if "==" in spec else spec.split("[")[0]
                    )
                    overrides[pkg_name] = (spec, last_comment)
                    last_comment = None

    # Add or update the current package
    overrides[package] = (target, comment)

    # Rebuild pyproject.toml
    if not has_tool_uv:
        # Add [tool.uv] section at the end
        content += "\n[tool.uv]\n"
        content += (
            "# Security overrides - Review periodically and remove "
            "if parent constraints allow natural upgrade\n"
        )
        has_tool_uv = True

    if has_overrides:
        # Remove old override-dependencies section (handles both single-line and multi-line)
        # Single-line: override-dependencies = ["pkg==1.0"]
        # Multi-line: override-dependencies = [\n    "pkg",\n]
        content = re.sub(
            r"^override-dependencies\s*=\s*\[.*?\]",
            "",
            content,
            flags=re.MULTILINE | re.DOTALL,
        )
        # Also remove any orphaned comments before override-dependencies
        content = re.sub(r"^# Security overrides.*?\n", "", content, flags=re.MULTILINE)

        # Re-add the header after [tool.uv] so insertion logic stays consistent
        content = re.sub(
            r"(\[tool\.uv\]\n)",
            r"\1# Security overrides - Review periodically and remove "
            r"if parent constraints allow natural upgrade\n",
            content,
        )

    # Find [tool.uv] and insert override-dependencies
    if not has_overrides and has_tool_uv:
        # Insert after [tool.uv]
        content = re.sub(
            r"(\[tool\.uv\]\n)",
            r"\1# Security overrides - Review periodically and remove "
            r"if parent constraints allow natural upgrade\n",
            content,
        )

    # Build override-dependencies array
    override_lines = ["override-dependencies = ["]
    for _pkg_name, (spec, pkg_comment) in sorted(overrides.items()):
        if pkg_comment:
            override_lines.append(f"    {pkg_comment}")
        override_lines.append(f'    "{spec}",')
    override_lines.append("]")
    override_block = "\n".join(override_lines)

    # Insert after [tool.uv] or the header comment
    if "# Security overrides" in content:
        content = re.sub(
            r"(# Security overrides.*?\n)", r"\1" + override_block + "\n", content
        )
    else:
        content = re.sub(r"(\[tool\.uv\]\n)", r"\1" + override_block + "\n", content)

    # Write back
    pyproject_path.write_text(content)
    print(f"Updated override-dependencies with {target}")


if __name__ == "__main__":
    main()
