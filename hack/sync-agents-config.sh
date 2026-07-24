#!/usr/bin/env bash

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

# Syncs AI agent config from the vendor-neutral ai/ directory (single source
# of truth) to tool-specific directories:
#   ai/skills/<name>/SKILL.md -> .claude/skills/<name>/SKILL.md
#                             -> .cursor/skills/<name>/SKILL.md
#   ai/rules/<name>.md        -> .claude/rules/<name>.md
# Never edit the synced copies by hand; edit ai/ and re-run this script.

set -euo pipefail

AGENTS_DIR="ai"
AGENTS_SKILLS_DIR="$AGENTS_DIR/skills"
AGENTS_RULES_DIR="$AGENTS_DIR/rules"

CLAUDE_CONFIG_DIR=".claude"
CLAUDE_SKILLS_DIR="$CLAUDE_CONFIG_DIR/skills"
CLAUDE_RULES_DIR="$CLAUDE_CONFIG_DIR/rules"

CURSOR_CONFIG_DIR=".cursor"
CURSOR_SKILLS_DIR="$CURSOR_CONFIG_DIR/skills"

sync_skills() {
  rm -rf "$CLAUDE_SKILLS_DIR" "$CURSOR_SKILLS_DIR"

  local found=0
  for skill_dir in "$AGENTS_SKILLS_DIR"/*/; do
    [ -d "$skill_dir" ] || continue
    local name
    name=$(basename "$skill_dir")
    local skill_file="$skill_dir/SKILL.md"
    [ -f "$skill_file" ] || continue
    found=1

    mkdir -p "$CLAUDE_SKILLS_DIR/$name"
    cp "$skill_file" "$CLAUDE_SKILLS_DIR/$name/SKILL.md"
    echo "  claude: $CLAUDE_SKILLS_DIR/$name/SKILL.md"

    mkdir -p "$CURSOR_SKILLS_DIR/$name"
    cp "$skill_file" "$CURSOR_SKILLS_DIR/$name/SKILL.md"
    echo "  cursor: $CURSOR_SKILLS_DIR/$name/SKILL.md"
  done

  if [ "$found" -eq 0 ]; then
    echo "  (no skills found in $AGENTS_SKILLS_DIR)"
  fi
}

sync_rules() {
  rm -rf "$CLAUDE_RULES_DIR"

  local found=0
  for rule_file in "$AGENTS_RULES_DIR"/*.md; do
    [ -f "$rule_file" ] || continue
    found=1

    mkdir -p "$CLAUDE_RULES_DIR"
    cp "$rule_file" "$CLAUDE_RULES_DIR/$(basename "$rule_file")"
    echo "  claude: $CLAUDE_RULES_DIR/$(basename "$rule_file")"
  done

  if [ "$found" -eq 0 ]; then
    echo "  (no rules found in $AGENTS_RULES_DIR)"
  fi
}

echo "Syncing AI config from $AGENTS_DIR/ ..."
echo ""
echo "Skills:"
sync_skills
echo ""
echo "Rules:"
sync_rules
echo ""
echo "Done."
