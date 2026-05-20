#!/usr/bin/env python3
"""Update a direct dependency's version constraint in a requirements.txt or pyproject.toml.

Uses packaging.requirements.Requirement for exact name matching (no prefix collision)
and specifier intersection (preserves extras and upper bounds).

Usage:
    python3 update-source-dep.py <package> <fix_version> <source_file>

Prints "direct" if the package was found and updated, "transitive" otherwise.
"""

import re
import sys
import tomllib

from packaging.requirements import Requirement
from packaging.specifiers import SpecifierSet


def normalize(name):
    return re.sub(r"[-_.]+", "-", name).lower()


def update_requirements_txt(target, fix_ver, source_file):
    with open(source_file, "r") as f:
        lines = f.readlines()
    found = False
    new_lines = []
    for line in lines:
        stripped = line.strip()
        if stripped and not stripped.startswith("#"):
            try:
                req = Requirement(stripped)
                if normalize(req.name) == target:
                    found = True
                    req.specifier = req.specifier & SpecifierSet(f">={fix_ver}")
                    new_lines.append(str(req) + "\n")
                    continue
            except Exception:
                pass
        new_lines.append(line)
    if found:
        with open(source_file, "w") as f:
            f.writelines(new_lines)
    return found


def update_pyproject_toml(target, fix_ver, source_file):
    with open(source_file, "rb") as f:
        deps = tomllib.load(f).get("project", {}).get("dependencies", [])
    found = False
    with open(source_file, "r") as f:
        content = f.read()
    for dep in deps:
        req = Requirement(dep)
        if normalize(req.name) == target:
            found = True
            new_req = Requirement(dep)
            new_req.specifier = req.specifier & SpecifierSet(f">={fix_ver}")
            content = content.replace(f'"{dep}"', f'"{new_req}"')
    if found:
        with open(source_file, "w") as f:
            f.write(content)
    return found


def main():
    pkg_name, fix_ver, source_file = sys.argv[1], sys.argv[2], sys.argv[3]
    target = normalize(pkg_name)

    if source_file.endswith(".toml"):
        found = update_pyproject_toml(target, fix_ver, source_file)
    else:
        found = update_requirements_txt(target, fix_ver, source_file)

    print("direct" if found else "transitive")


if __name__ == "__main__":
    main()
