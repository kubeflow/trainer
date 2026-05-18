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
  - Files with years 2024-2025 are accepted (the year is stripped during
    comparison against the year-less template).
  - Files with year 2026 are rejected unless allowlisted in
    .boilerplateignore under [year-2026] (these were merged before this
    enforcement landed).
  - Files with any other year (2027+, or earlier) are rejected.
  - New files must use the year-less template directly.

Reference: https://github.com/kubernetes/steering/issues/299
"""

import argparse
import glob
import os
import re
import subprocess
import sys
from typing import Dict, List, Optional, Set, Tuple

FIRST_YEAR = 2024
FINAL_YEAR = 2025

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

# Path patterns skipped during repo-wide scans (not when explicit
# filenames are passed on the command line).
SCAN_SKIP_PATTERNS = (
    "hack/boilerplate/test/",
    "api/python_api/",
)

IGNORE_FILE = ".boilerplateignore"


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


def parse_ignore_file(path: str) -> Dict[str, Set[str]]:
    """Parse a .boilerplateignore file into named sections."""
    sections: Dict[str, Set[str]] = {}
    current: Optional[Set[str]] = None
    if not os.path.isfile(path):
        return sections
    with open(path, "r", encoding="utf-8") as f:
        for raw in f:
            line = raw.strip()
            if not line or line.startswith("#"):
                continue
            if line.startswith("[") and line.endswith("]"):
                current = sections.setdefault(line[1:-1].strip(), set())
                continue
            if current is None:
                continue
            current.add(line)
    return sections


def load_templates(boilerplate_dir: str) -> Dict[str, List[str]]:
    """Load boilerplate templates as {stem: [line, ...]}."""
    templates: Dict[str, List[str]] = {}
    for path in glob.glob(os.path.join(boilerplate_dir, "boilerplate.*.txt")):
        stem = os.path.basename(path).replace("boilerplate.", "").replace(".txt", "")
        with open(path, "r", encoding="utf-8") as f:
            templates[stem] = f.read().splitlines()
    return templates


def get_regexes() -> Dict[str, re.Pattern]:
    return {
        "any_year": re.compile(r"Copyright (\d{4}) "),
        "go_build_constraints": re.compile(
            r"^(//(go:build| \+build).*\n)+\n", re.MULTILINE
        ),
        "shebang": re.compile(r"^(#!.*\n)\n*"),
        "generated": re.compile(r"DO NOT EDIT", re.MULTILINE),
    }


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
        skip_patterns: Tuple[str, ...] = ()
    else:
        candidates = list_git_files(rootdir)
        skip_patterns = SCAN_SKIP_PATTERNS

    def keep(path: str) -> bool:
        if template_stem_for(path) is None:
            return False
        normalized = path.replace(os.sep, "/")
        return not any(p in normalized for p in skip_patterns)

    return sorted(f for f in candidates if keep(f))


def normalize_year(
    line: str,
    regexes: Dict[str, re.Pattern],
    allow_2026: bool,
) -> Tuple[Optional[str], Optional[str]]:
    """Validate and strip the copyright year on a line."""
    match = regexes["any_year"].search(line)
    if not match:
        return line, None
    year = int(match.group(1))
    if year < FIRST_YEAR:
        return (
            None,
            f"Year {year} predates the project (earliest allowed: {FIRST_YEAR})",
        )
    if year > FINAL_YEAR and not (year == 2026 and allow_2026):
        return (
            None,
            f"Year {year} is not allowed in the copyright header (must be omitted)",
        )
    return re.sub(r"Copyright \d{4} ", "Copyright ", line), None


def file_passes(
    filename: str,
    templates: Dict[str, List[str]],
    regexes: Dict[str, re.Pattern],
    rootdir: str,
    ignores: Optional[Dict[str, Set[str]]] = None,
) -> Tuple[bool, Optional[str]]:
    """Check that filename starts with the expected boilerplate header."""
    ignores = ignores or {}
    relpath = os.path.relpath(filename, rootdir).replace(os.sep, "/")

    if relpath in ignores.get("kubernetes-authors", set()):
        return True, None
    if relpath in ignores.get("line-comment", set()):
        return True, None

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
        and regexes["generated"].search(data)
        and GENERATED_GO_TEMPLATE in templates
    ):
        stem = GENERATED_GO_TEMPLATE

    ref = templates[stem]

    if stem in ("go", GENERATED_GO_TEMPLATE):
        data = regexes["go_build_constraints"].sub("", data)
    if stem == "sh":
        data = regexes["shebang"].sub("", data)

    lines = data.splitlines()
    if len(lines) < len(ref):
        return False, "File is shorter than the expected boilerplate header"

    allow_2026 = relpath in ignores.get("year-2026", set())
    normalized: List[str] = []
    for line in lines[: len(ref)]:
        new_line, err = normalize_year(line, regexes, allow_2026)
        if err:
            return False, err
        normalized.append(new_line)

    if normalized != ref:
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

    regexes = get_regexes()
    ignores = parse_ignore_file(os.path.join(args.boilerplate_dir, IGNORE_FILE))
    no_header = ignores.get("no-header", set())

    failed: List[Tuple[str, Optional[str]]] = []
    for filepath in collect_files(rootdir, args.filenames or None):
        relpath = os.path.relpath(filepath, rootdir).replace(os.sep, "/")
        if relpath in no_header:
            continue
        passes, error = file_passes(filepath, templates, regexes, rootdir, ignores)
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
