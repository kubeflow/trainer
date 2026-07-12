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

Two checks are applied:

  1. All files: the header must match the boilerplate template once any
     copyright year is normalized out, so a year-bearing header and a
     year-less header both validate against the single year-less template.
  2. New files (added relative to the base branch): the header must
     additionally be year-less; a hardcoded copyright year is rejected.

New-file detection compares against the base branch (--base-ref, default
$TARGET_BRANCH or master), resolved as origin/<ref> then <ref>. If the base
cannot be resolved, or files cannot be listed, the run fails rather than
passing vacuously.

Reference: https://github.com/kubernetes/steering/issues/299
"""

import argparse
import glob
import os
import re
import subprocess
import sys
from typing import Dict, List, Optional, Set, Tuple

# Maps file extension (or special basename) to the boilerplate template
# stem on disk (boilerplate.<stem>.txt). The "sh" stem is reused for every
# hash-comment language (bash, py, Dockerfile, yaml, yml). "helm" (also used
# for gotmpl) maps to boilerplate.helm.txt for Go-template comments, and
# go / rs have their own templates.
EXTENSION_TEMPLATES = {
    "go": "go",
    "helm": "helm",
    "gotmpl": "helm",
    "rs": "rs",
    "sh": "sh",
    "bash": "sh",
    "py": "sh",
    "Dockerfile": "sh",
    "yaml": "sh",
    "yml": "sh",
}

# Template used for code-generated Go files (DO NOT EDIT). Loaded from
# disk but only activated when a Go file is detected as generated.
GENERATED_GO_TEMPLATE = "generatego"

# Regex patterns
_RE_ANY_YEAR = re.compile(r"Copyright \d{4}")
_RE_YEAR_STRIP = re.compile(r"(Copyright )\d{4}(?:-\d{4})? ")
_RE_GO_BUILD_CONSTRAINTS = re.compile(r"^(//(go:build| \+build).*\n)+\n", re.MULTILINE)
_RE_SHEBANG = re.compile(r"^(#!.*\n)\n*")
_RE_GENERATED = re.compile(r"DO NOT EDIT", re.MULTILINE)
_RE_HELM_TEMPLATE = re.compile(r"(?:^|/)charts/[^/]+/templates/.*\.(?:ya?ml|tpl)$")


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


def base_tree_files(base_ref: str, rootdir: str) -> Optional[Set[str]]:
    """Repo-relative paths at the merge-base of the base branch and HEAD.

    Tries origin/<base_ref> then <base_ref>. Returns None if neither
    resolves, so the caller can fail rather than skip the new-file check.
    """
    for ref in (f"origin/{base_ref}", base_ref):
        try:
            base = subprocess.run(
                ["git", "-C", rootdir, "merge-base", ref, "HEAD"],
                capture_output=True,
                text=True,
                check=True,
            ).stdout.strip()
            tree = subprocess.run(
                ["git", "-C", rootdir, "ls-tree", "-r", "--name-only", base],
                capture_output=True,
                text=True,
                check=True,
            )
        except (subprocess.CalledProcessError, FileNotFoundError):
            continue
        return {line for line in tree.stdout.splitlines() if line}
    return None


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
    normalized = filename.replace(os.sep, "/")
    if _RE_HELM_TEMPLATE.search(normalized):
        return "helm"
    basename = os.path.basename(filename)
    if basename == "Dockerfile" or basename.startswith("Dockerfile."):
        return EXTENSION_TEMPLATES.get("Dockerfile")
    ext = os.path.splitext(filename)[1].lstrip(".").lower()
    return EXTENSION_TEMPLATES.get(ext)


def list_git_files(rootdir: str) -> Optional[List[str]]:
    """List files git considers part of the working tree.

    Includes tracked files plus untracked-not-ignored files, so newly
    added files are checked but build artifacts and __pycache__ are not.
    Returns None if git fails, so the caller can fail rather than treat a
    broken listing as "no files".
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
    except (subprocess.CalledProcessError, FileNotFoundError) as e:
        print(f"ERROR: 'git ls-files' failed: {e}", file=sys.stderr)
        return None
    return [os.path.join(rootdir, line) for line in result.stdout.splitlines() if line]


def collect_files(rootdir: str, filenames: Optional[List[str]]) -> Optional[List[str]]:
    """Return the files to check, or None if the git listing failed."""
    if filenames:
        candidates: List[str] = []
        for f in filenames:
            if os.path.isfile(f):
                candidates.append(f)
            elif os.path.isdir(f):
                for root, _, names in os.walk(f):
                    for name in names:
                        candidates.append(os.path.join(root, name))
        return sorted(candidates)

    files = list_git_files(rootdir)
    if files is None:
        return None
    return sorted(files)


def file_passes(
    filename: str,
    templates: Dict[str, List[str]],
    new_file: bool = False,
) -> Tuple[bool, Optional[str]]:
    """Verify filename's boilerplate header.

    Check 1 (all files): the header must equal the template after any
      copyright year is normalized out.
    Check 2 (new files): the header must be year-less.
    """
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

    if len(lines) < len(ref):
        return False, "File is shorter than the expected boilerplate header"

    # Check 1: the header must match the template once the copyright year is
    # normalized out, so year-bearing and year-less headers both validate.
    normalized = [_RE_YEAR_STRIP.sub(r"\1", line) for line in header_lines]
    if normalized != ref:
        return False, "Header does not match the boilerplate template"

    # Check 2: files added on this branch must not hardcode a copyright year.
    if new_file and any(_RE_ANY_YEAR.search(line) for line in header_lines):
        return (
            False,
            "New file must use the year-less header "
            "(remove the hardcoded copyright year)",
        )

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
    parser.add_argument(
        "--base-ref",
        default=os.environ.get("TARGET_BRANCH", "master"),
        help=(
            "Base branch for new-file detection (default: $TARGET_BRANCH or "
            "master). Resolved as origin/<ref> then <ref>."
        ),
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

    files = collect_files(rootdir, args.filenames or None)
    if files is None:
        print("ERROR: could not list repository files via git.", file=sys.stderr)
        return 1

    base = base_tree_files(args.base_ref, rootdir)
    if base is None:
        print(
            "ERROR: could not resolve a base ref for new-file detection "
            f"(tried origin/{args.base_ref}, {args.base_ref}). Ensure the base "
            "branch is fetched (actions/checkout fetch-depth: 0) or pass "
            "--base-ref.",
            file=sys.stderr,
        )
        return 1

    failed: List[Tuple[str, Optional[str]]] = []
    for filepath in files:
        relpath = os.path.relpath(filepath, rootdir).replace(os.sep, "/")
        new_file = relpath not in base
        passes, error = file_passes(filepath, templates, new_file=new_file)
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
