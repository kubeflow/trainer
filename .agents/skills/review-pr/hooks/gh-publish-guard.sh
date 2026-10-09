#!/usr/bin/env bash
# PreToolUse guard for review-pr skill.
#
# Forces a confirmation prompt before any Bash command that PUBLISHES to GitHub
# (posting PR/issue reviews or comments, merging, creating/editing PRs or
# releases, or writing via `gh api`). Read-only commands such as `gh pr view`,
# `gh pr diff`, `gh pr list`, and GET `gh api` calls pass straight through.
#
# This exists because the review-pr skill instructs the model to show a draft
# for human review before posting -- but a skill instruction can be skipped.
# This hook is enforced by the harness, so it cannot be.
set -euo pipefail

payload="$(cat)"

# Pull the Bash command string out of the hook payload.
if command -v jq >/dev/null 2>&1; then
  cmd="$(printf '%s' "$payload" | jq -r '.tool_input.command // ""')"
else
  cmd="$(printf '%s' "$payload" | python3 -c 'import sys, json; print(json.load(sys.stdin).get("tool_input", {}).get("command", ""))')"
fi

# Commands that write to GitHub. `gh api` is only flagged when it uses a
# write HTTP method, since it defaults to GET (read-only).
write_re='gh[[:space:]]+(pr|issue)[[:space:]]+(review|comment|merge|create|edit|close|reopen|ready)'
write_re+='|gh[[:space:]]+release[[:space:]]+(create|edit|delete)'
write_re+='|gh[[:space:]]+api[[:space:]].*(--method[[:space:]]+(POST|PATCH|PUT|DELETE)|-X[[:space:]]+(POST|PATCH|PUT|DELETE))'

if printf '%s' "$cmd" | grep -Eiq "$write_re"; then
  cat <<'JSON'
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "ask",
    "permissionDecisionReason": "This command publishes to GitHub. Confirm the draft review/comment was shown and approved before posting."
  }
}
JSON
fi

# Non-matching commands: emit nothing and exit 0 -> normal permission flow.
exit 0
