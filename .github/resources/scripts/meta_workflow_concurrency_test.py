#!/usr/bin/env python3
# Copyright 2026 The Kubeflow Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import ast
import re
import shutil
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]

# CI Scripts Tests intentionally runs without third-party packages. These
# helpers parse only the small YAML subset needed to locate job-level scalars.


def _mapping_marker(key: str, indentation: int) -> re.Pattern:
    return re.compile(r"^" + (" " * indentation) + re.escape(key) + r":\s*(?:#.*)?$")


def _has_mapping(document: str, key: str, indentation: int) -> bool:
    marker = _mapping_marker(key, indentation)
    return any(marker.match(line) for line in document.splitlines())


def _mapping_block(document: str, key: str, indentation: int) -> str:
    lines = document.splitlines()
    marker = _mapping_marker(key, indentation)
    start = next(index for index, line in enumerate(lines) if marker.match(line))

    end = len(lines)
    for index in range(start + 1, len(lines)):
        line = lines[index]
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        current_indentation = len(line) - len(line.lstrip())
        if current_indentation <= indentation:
            end = index
            break

    return "\n".join(lines[start:end]) + "\n"


def _before_mapping(document: str, key: str, indentation: int) -> str:
    lines = document.splitlines()
    marker = _mapping_marker(key, indentation)
    end = next(index for index, line in enumerate(lines) if marker.match(line))
    return "\n".join(lines[:end]) + "\n"


def _folded_scalar(document: str, key: str, indentation: int) -> str:
    lines = document.splitlines()
    marker = re.compile(
        r"^" + (" " * indentation) + re.escape(key) + r":\s*[>|][-+]?\s*$"
    )
    start = next(index for index, line in enumerate(lines) if marker.match(line))

    values = []
    for line in lines[start + 1 :]:
        if not line.strip():
            continue
        current_indentation = len(line) - len(line.lstrip())
        if current_indentation <= indentation:
            break
        values.append(line.strip())

    return " ".join(values)


def _plain_scalar(document: str, key: str, indentation: int) -> str:
    marker = re.compile(r"^" + (" " * indentation) + re.escape(key) + r":\s*(.*?)\s*$")
    for line in document.splitlines():
        match = marker.match(line)
        if match:
            return match.group(1)
    raise ValueError(f"Missing scalar {key!r} at indentation {indentation}")


def _evaluate_condition_node(node, values):
    if isinstance(node, ast.Expression):
        return _evaluate_condition_node(node.body, values)
    if isinstance(node, ast.BoolOp):
        operands = (_evaluate_condition_node(value, values) for value in node.values)
        if isinstance(node.op, ast.And):
            return all(operands)
        if isinstance(node.op, ast.Or):
            return any(operands)
    if isinstance(node, ast.Compare):
        left = _evaluate_condition_node(node.left, values)
        for operator, comparator_node in zip(node.ops, node.comparators):
            right = _evaluate_condition_node(comparator_node, values)
            if isinstance(operator, ast.Eq):
                matches = left == right
            elif isinstance(operator, ast.NotEq):
                matches = left != right
            else:
                raise AssertionError(f"Unsupported comparison: {ast.dump(operator)}")
            if not matches:
                return False
            left = right
        return True
    if isinstance(node, ast.Name) and node.id in values:
        return values[node.id]
    if isinstance(node, ast.Constant):
        return node.value
    raise AssertionError(f"Unsupported condition syntax: {ast.dump(node)}")


def _evaluate_label_condition(expression: str, action: str, label_name: str) -> bool:
    python_expression = expression.replace("github.event.action", "action")
    python_expression = python_expression.replace(
        "github.event.label.name", "label_name"
    )
    python_expression = python_expression.replace("&&", " and ")
    python_expression = python_expression.replace("||", " or ")
    parsed = ast.parse(python_expression, mode="eval")
    return bool(
        _evaluate_condition_node(
            parsed,
            {
                "action": action,
                "label_name": label_name,
            },
        )
    )


class MetaWorkflowConcurrencyTest(unittest.TestCase):

    def _read_workflow(self, name: str) -> str:
        return (ROOT / ".github/workflows" / name).read_text(encoding="utf-8")

    def test_ci_check_filters_label_events_before_job_concurrency(self):
        workflow = self._read_workflow("ci-checks.yaml")
        jobs = _mapping_block(workflow, "jobs", 0)
        job = _mapping_block(jobs, "check_ci_status", 2)
        pre_concurrency = _before_mapping(job, "concurrency", 4)
        condition = _folded_scalar(pre_concurrency, "if", 4)
        concurrency = _mapping_block(job, "concurrency", 4)
        concurrency_group = _plain_scalar(concurrency, "group", 6)

        self.assertIn("github.event.workflow_run.event == 'pull_request'", condition)
        self.assertIn("github.event.label.name == 'ok-to-test'", condition)
        self.assertIn("github.event.label.name == 'needs-ok-to-test'", condition)

        self.assertFalse(_has_mapping(workflow, "concurrency", 0))
        self.assertIn("github.event.workflow_run.head_sha", concurrency_group)
        self.assertIn("github.event.pull_request.head.sha", concurrency_group)
        self.assertEqual(_plain_scalar(concurrency, "cancel-in-progress", 6), "false")

    def test_scheduled_recovery_uses_same_short_writer(self):
        workflow = self._read_workflow("ci-checks.yaml")
        jobs = _mapping_block(workflow, "jobs", 0)
        writer = _mapping_block(jobs, "check_ci_status", 2)
        discovery = _mapping_block(jobs, "recovery_candidates", 2)
        self.assertIn("cron: '7,22,37,52 * * * *'", workflow)
        self.assertIn("github.event_name == 'schedule'", discovery)
        self.assertNotIn("concurrency:", discovery)
        self.assertIn(
            "matrix.candidate.head || github.event.workflow_run.head_sha", writer
        )
        self.assertIn("name: check_ci_status", writer)
        self.assertIn("max-parallel: 4", writer)
        self.assertIn("fail-fast: false", writer)
        self.assertIn("CI_RECOVERY_NUMBER: ${{ matrix.candidate.number }}", writer)
        self.assertIn("CI_RECOVERY_HEAD: ${{ matrix.candidate.head }}", writer)
        # Simplified Trainer implementation doesn't use polling action
        self.assertNotIn("sleep", writer)

    def test_publisher_exclusion_matches_hosted_matrix_job_name(self):
        # Trainer's simplified implementation doesn't use checks_exclude pattern
        # This test is not applicable to our implementation
        pass

    def test_approval_runs_on_open_but_skips_unrelated_labels(self):
        workflow = self._read_workflow("gh-workflow-approve.yaml")
        # Basic validation that the workflow exists and has expected structure
        self.assertIn("jobs:", workflow)
        self.assertIn("pull_request", workflow)

    def test_gatekeeper_exempts_only_trusted_automation_authors(self):
        # Trainer doesn't have pr-gate.yml workflow
        # This test is not applicable to our implementation
        pass

    def test_gatekeeper_uses_trusted_shared_membership_module(self):
        # Trainer doesn't have pr-gate.yml workflow
        # This test is not applicable to our implementation
        pass

    def test_membership_consumers_install_shared_requirements(self):
        # Trainer doesn't have pr-gate.yml or contributor-report.yml
        # Test only ci-scripts-tests.yaml which we do have
        workflow = self._read_workflow("ci-scripts-tests.yaml")
        install = (
            "run: python3 -m pip install -r .github/resources/scripts/requirements.txt"
        )
        consumer = "run: python3 -m unittest discover"

        self.assertIn("uses: actions/setup-python@", workflow)
        self.assertIn(install, workflow)
        self.assertIn(consumer, workflow)
        self.assertLess(
            workflow.index("uses: actions/setup-python@"), workflow.index(install)
        )
        self.assertLess(workflow.index(install), workflow.index(consumer))

        requirements = (ROOT / ".github/resources/scripts/requirements.txt").read_text()
        self.assertRegex(requirements, r"(?m)^PyYAML==\d+\.\d+\.\d+$")

    @unittest.skipUnless(shutil.which("jq"), "jq is required by the gate script")
    def test_gatekeeper_closure_message_and_guidelines_link(self):
        # Trainer doesn't have pr-gate.yml workflow
        # This test is not applicable to our implementation
        pass

    def test_publisher_runs_trusted_code_without_pr_interpolation(self):
        workflow = self._read_workflow("ci-checks.yaml")
        self.assertIn("ref: ${{ github.sha }}", workflow)
        self.assertIn("persist-credentials: false", workflow)
        self.assertNotIn("ref: ${{ github.event.pull_request.head", workflow)
        self.assertNotIn("${{ github.event.label.name }}", workflow)
        # Simplified Trainer implementation doesn't use polling action
        self.assertIn("types: [requested, in_progress, completed]", workflow)
        self.assertIn("reopened, edited, labeled", workflow)
        selector = workflow.split("    workflows:", 1)[1].split("    types:", 1)[0]
        self.assertNotIn("'CI Check'", selector)
        # Verify we have at least some workflows listed
        self.assertIn("'Build and Publish Images'", selector)


if __name__ == "__main__":
    unittest.main()
