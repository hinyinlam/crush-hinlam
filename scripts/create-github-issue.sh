#!/bin/bash
# Creates the GitHub issue for plugin support.
# Usage: bash scripts/create-github-issue.sh
# Requires: gh auth login (or GITHUB_TOKEN env var)

set -euo pipefail

ISSUE_TITLE="feat: Support Claude Code plugins (superpowers, caveman)"
ISSUE_BODY=$(cat <<'BODY'
## Goal

Add Claude Code plugin support to Crush so plugins like [superpowers](https://github.com/obra/superpowers) and [caveman](https://github.com/JuliusBrussee/caveman) can be installed and used.

## Status: COMPLETED

### Completed Work

- [x] Research Claude plugin format (`.claude-plugin/plugin.json` + `skills/SKILL.md`)
- [x] Implement `crush plugin install <repo>` CLI command
- [x] Implement `crush plugin list` CLI command
- [x] Auto-discovery of plugin skills in config loading
- [x] 16 unit/integration tests (all pass)
- [x] Install superpowers plugin (14 skills discovered)
- [x] Install caveman plugin (7 skills discovered)
- [x] Real `crush` CLI test (both installed binary and source-built)
- [x] Documentation in AGENTS.md
- [x] Git committed, pushed, clean worktree

### Usage

```bash
crush plugin install obra/superpowers
crush plugin install JuliusBrussee/caveman
crush plugin list
```

### Commits

- `01156370` feat: add Claude Code plugin support
- `36e5e8ad` docs: add plugin integration tests

### Acceptance Criteria

The acceptance criteria is met: `crush` can install the superpowers plugin.
BODY
)

# Check if issue already exists
EXISTING=$(gh issue list --repo hinyinlam/crush-hinlam --search "$ISSUE_TITLE" --state all --json number --jq '.[0].number' 2>/dev/null || echo "")
if [ -n "$EXISTING" ]; then
    echo "Issue already exists: #$EXISTING"
    exit 0
fi

# Create the issue
gh issue create --repo hinyinlam/crush-hinlam \
    --title "$ISSUE_TITLE" \
    --body "$ISSUE_BODY" \
    --label "enhancement"

echo ""
echo "Issue created successfully!"
echo "To add a progress comment:"
echo "  gh issue comment <number> --repo hinyinlam/crush-hinlam --body 'Progress update'"
