# GitHub Copilot Code Review Instructions

## Review Philosophy

- Only comment when you have HIGH CONFIDENCE (>80%) that an issue exists
- Be concise: one sentence per comment when possible
- Focus on actionable feedback, not observations
- When reviewing text, only comment on clarity issues if the text is genuinely confusing or could lead to errors.
  "Could be clearer" is not the same as "is confusing" - stay silent unless HIGH confidence it will cause problems

## Priority Areas (Review These)

### Security & Safety

- Unsafe code blocks without justification
- Command injection risks (shell commands, user input)
- Path traversal vulnerabilities
- Credential exposure or hardcoded secrets
- Missing input validation on external data
- Improper error handling that could leak sensitive info

### Correctness Issues

- Logic errors that could cause panics or incorrect behavior
- Resource leaks (files, connections, memory)
- Off-by-one errors or boundary conditions
- Optional types that don't need to be optional
- Booleans that should default to false but are set as optional
- Overly defensive code that adds unnecessary checks
- Unnecessary comments that just restate what the code already shows (remove them)

### Architecture & Patterns

- Code that violates existing patterns in the codebase
- Missing error handling
- Code that is not following [Effective Go](https://go.dev/doc/effective_go), [Python PEP8](https://peps.python.org/pep-0008/), or [Rust API guidelines](https://rust-lang.github.io/api-guidelines/)
- Violating [Kubernetes API guidelines](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api_changes.md)

## Project-Specific Context

- See [AGENTS.md](../AGENTS.md) in the root directory for project guidelines and architecture decisions.

## CI Pipeline Context

**Important**: You review PRs immediately, before CI completes. Do not flag issues that CI will catch.

### What Our CI Checks

- `.github/workflows/test-go.yaml` - Code generation, linting, and tests for Go source code
- `.github/workflows/test-python.yaml` - unit and integration tests for Python source code
- `.github/workflows/test-rust.yaml` - unit and integration tests for Rust source code
- `.github/workflows/test-e2e.yaml` - e2e tests
- `.github/workflows/build-and-push-images.yaml` - build and push container images

## Skip These (IMPORTANT)

Do not comment on:

- **Auto generated code** - CI handles this (make generate)
- **Style/formatting** - CI handles this (gofmt, black, prettier)
- **Test failures** - CI handles this (full test suite)
- **Missing dependencies** - CI handles this

## Response Format

When you identify an issue:

1. **State the problem** (1 sentence)
2. **Why it matters** (1 sentence, only if not obvious)
3. **Suggested fix** (code snippet or specific action)

## When to Stay Silent

If you're uncertain whether something is an issue, don't comment. False positives create noise and reduce trust in the review process.
