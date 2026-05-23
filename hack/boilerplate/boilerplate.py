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

"""Verify boilerplate copyright headers in source files.

Policy:
  - Files that already contain a copyright year are accepted.
  - Files without a year must match the year-less boilerplate template.

Reference: https://github.com/kubernetes/steering/issues/299
"""

import argparse
import glob
import os
import re
import subprocess
import sys
from typing import Dict, List, Optional, Tuple

# Maps file extension (or special basename) to the boilerplate template
# stem on disk (boilerplate.<stem>.txt). One template is reused across
# all hash-comment languages.
EXTENSION_TEMPLATES = {
    "go": "go",
    "sh": "sh",
    "bash": "sh",
    "py": "sh",
    "Dockerfile": "sh",
}

# Template used for code-generated Go files (DO NOT EDIT). Loaded from
# disk but only activated when a Go file is detected as generated.
GENERATED_GO_TEMPLATE = "generatego"

# Regex patterns
_RE_ANY_YEAR = re.compile(r"Copyright \d{4} ")
_RE_GO_BUILD_CONSTRAINTS = re.compile(r"^(//(go:build| \+build).*\n)+\n", re.MULTILINE)
_RE_SHEBANG = re.compile(r"^(#!.*\n)\n*")
_RE_GENERATED = re.compile(r"DO NOT EDIT", re.MULTILINE)


def find_root_dir() -> str:
    """Resolve the repo root via git, falling back to the cwd."""
    try:
        result = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            capture_output=True,
            text=True,
            check=True,
        )
        return result.stdout.strip()
    except (subprocess.CalledProcessError, FileNotFoundError):
        return os.getcwd()


def load_templates(boilerplate_dir: str) -> Dict[str, List[str]]:
    """Load boilerplate templates as {stem: [line, ...]}."""
    templates: Dict[str, List[str]] = {}
    for path in glob.glob(os.path.join(boilerplate_dir, "boilerplate.*.txt")):
        stem = os.path.basename(path).replace("boilerplate.", "").replace(".txt", "")
        with open(path, "r", encoding="utf-8") as f:
            templates[stem] = f.read().splitlines()
    return templates


def template_stem_for(filename: str) -> Optional[str]:
    """Return the template stem for filename, or None to skip."""
    basename = os.path.basename(filename)
    if basename == "Dockerfile" or basename.startswith("Dockerfile."):
        return EXTENSION_TEMPLATES.get("Dockerfile")
    ext = os.path.splitext(filename)[1].lstrip(".").lower()
    return EXTENSION_TEMPLATES.get(ext)


def list_git_files(rootdir: str) -> List[str]:
    """List files git considers part of the working tree.

    Includes tracked files plus untracked-not-ignored files, so newly
    added files are checked but build artifacts and __pycache__ are not.
    """
    try:
        result = subprocess.run(
            [
                "git",
                "-C",
                rootdir,
                "ls-files",
                "--cached",
                "--others",
                "--exclude-standard",
            ],
            capture_output=True,
            text=True,
            check=True,
        )
    except (subprocess.CalledProcessError, FileNotFoundError):
        return []
    return [os.path.join(rootdir, line) for line in result.stdout.splitlines() if line]


def collect_files(rootdir: str, filenames: Optional[List[str]]) -> List[str]:
    """Return the files to check."""
    if filenames:
        candidates: List[str] = []
        for f in filenames:
            if os.path.isfile(f):
                candidates.append(f)
            elif os.path.isdir(f):
                for root, _, names in os.walk(f):
                    for name in names:
                        candidates.append(os.path.join(root, name))
    else:
        candidates = list_git_files(rootdir)

    return sorted(candidates)


def file_passes(
    filename: str,
    templates: Dict[str, List[str]],
) -> Tuple[bool, Optional[str]]:
    """Check that filename starts with the expected boilerplate header."""
    stem = template_stem_for(filename)
    if stem is None or stem not in templates:
        return True, None

    try:
        with open(filename, "r", encoding="utf-8") as f:
            data = f.read()
    except (IOError, UnicodeDecodeError) as e:
        return False, f"Error reading file: {e}"

    if (
        stem == "go"
        and _RE_GENERATED.search(data)
        and GENERATED_GO_TEMPLATE in templates
    ):
        stem = GENERATED_GO_TEMPLATE

    ref = templates[stem]

    if stem in ("go", GENERATED_GO_TEMPLATE):
        data = _RE_GO_BUILD_CONSTRAINTS.sub("", data)
    if stem == "sh":
        data = _RE_SHEBANG.sub("", data)

    lines = data.splitlines()
    header_lines = lines[: len(ref)]

    # Policy: any file whose header already contains "Copyright YYYY"
    # is accepted as-is. Those headers were reviewed when the file
    # landed; re-validating them on every CI run adds little. Trade-off:
    # a copy-paste with a different holder (e.g. "Copyright 2025 ACME")
    # would slip through. New files must use the year-less template.
    for line in header_lines:
        if _RE_ANY_YEAR.search(line):
            return True, None

    if len(lines) < len(ref):
        return False, "File is shorter than the expected boilerplate header"

    if header_lines != ref:
        return False, "Header does not match the boilerplate template"

    return True, None


def print_remediation(failed: List[Tuple[str, Optional[str]]]) -> None:
    print("", file=sys.stderr)
    print(
        "Boilerplate header verification failed for the following files:",
        file=sys.stderr,
    )
    print("", file=sys.stderr)
    for path, err in failed:
        if err:
            print(f"  {path}: {err}", file=sys.stderr)
        else:
            print(f"  {path}", file=sys.stderr)
    print("", file=sys.stderr)
    print("For new files, use the copyright header WITHOUT a year:", file=sys.stderr)
    print("  Copyright The Kubeflow Authors.", file=sys.stderr)
    print("", file=sys.stderr)
    print("See hack/boilerplate/ for the templates.", file=sys.stderr)
    print(
        "Reference: https://github.com/kubernetes/steering/issues/299", file=sys.stderr
    )


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Verify boilerplate copyright headers")
    parser.add_argument(
        "filenames",
        nargs="*",
        help="Specific files to check (default: all git-tracked files)",
    )
    parser.add_argument(
        "--boilerplate-dir",
        default=os.path.dirname(os.path.abspath(__file__)),
        help="Directory containing boilerplate template files",
    )
    parser.add_argument(
        "--rootdir",
        default=None,
        help="Root directory to scan (default: auto-detect via git)",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    rootdir = args.rootdir or find_root_dir()
    os.chdir(rootdir)

    templates = load_templates(args.boilerplate_dir)
    if not templates:
        print(
            f"ERROR: no boilerplate templates found in {args.boilerplate_dir}",
            file=sys.stderr,
        )
        return 1

    failed: List[Tuple[str, Optional[str]]] = []
    for filepath in collect_files(rootdir, args.filenames or None):
        relpath = os.path.relpath(filepath, rootdir).replace(os.sep, "/")
        passes, error = file_passes(filepath, templates)
        if not passes:
            failed.append((relpath, error))
            print(relpath)  # stdout stays parseable

    if failed:
        print_remediation(failed)
        return 1

    print("Boilerplate header verification passed.", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
