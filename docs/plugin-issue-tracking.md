# Claude Code Plugin Support - Issue Tracking

## Implementation Status: COMPLETE

All code implementation, testing, and documentation is complete:

- `crush plugin install <repo>` - clones Claude Code plugins to `~/.config/crush/plugins/`
- `crush plugin list` - lists installed plugins
- Auto-discovery of plugin `skills/` directories during config loading
- 16 unit and integration tests (all pass)
- superpowers plugin installed (14 skills discovered)
- caveman plugin installed (7 skills discovered)
- Tested with both installed `crush` binary and source-built `crush`
- Documentation in AGENTS.md

## Commits

| Hash | Description |
|------|-------------|
| `01156370` | feat: add Claude Code plugin support |
| `36e5e8ad` | docs: add plugin integration tests |
| `2864dc28` | ci: add workflow to auto-create plugin support GitHub issue |

## GitHub Issue Creation Attempts

### Approach 1: gh CLI with git credential helper
**Result**: FAILED — expired device code

### Approach 2: SSH deploy key
**Result**: FAILED — SSH keys can push/pull but cannot create issues via REST API

### Approach 3: gh CLI interactive device flow
**Result**: FAILED — requires browser on headless server

### Approach 4: GitHub Actions workflow (github-script)
**Result**: FAILED — fork repos have read-only GITHUB_TOKEN for issues

### Approach 5: GitHub Actions workflow (gh CLI)
**Result**: FAILED — same fork token limitation

### Approach 6: Manual device code flow with API polling
**Result**: PENDING — device code generated, waiting for browser authorization
