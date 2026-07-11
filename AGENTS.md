# Crush Development Guide

## Project Overview

Crush is terminal AI coding assistant built in Go by [Charm](https://charm.land). Connects to LLMs, gives them tools to read, write, execute code. Supports providers (Anthropic, OpenAI, Gemini, Bedrock, Copilot, Hyper, MiniMax, Vercel, etc.), integrates LSPs for code intelligence, extensibility via MCP servers + agent skills.

Module path: `github.com/charmbracelet/crush`.

## Architecture

```
main.go                            CLI entry point (cobra via internal/cmd)
internal/
  app/app.go                       Top-level wiring: DB, config, agents, LSP, MCP, events
  cmd/                             CLI commands (root, run, login, models, stats, sessions)
  config/
    config.go                      Config struct, context file paths, agent definitions
    load.go                        crush.json loading and validation
    provider.go                    Provider configuration and model resolution
  agent/
    agent.go                       SessionAgent: runs LLM conversations per session
    coordinator.go                 Coordinator: manages named agents ("coder", "task")
    hooked_tool.go                 Decorator that runs PreToolUse hooks before tool execution
    prompts.go                     Loads Go-template system prompts
    templates/                     System prompt templates (coder.md.tpl, task.md.tpl, etc.)
    tools/                         All built-in tools (bash, edit, view, grep, glob, etc.)
      mcp/                         MCP client integration
  hooks/                           Hook engine: runs user shell commands on hook events
    hooks.go                       Decision types, aggregation logic, event constants
    runner.go                      Parallel hook execution, timeout, dedup
    input.go                       Stdin payload builder, env vars, stdout parsing (Crush + Claude Code compat)
  session/session.go               Session CRUD backed by SQLite
  message/                         Message model and content types
  db/                              SQLite via sqlc, with migrations
    sql/                           Raw SQL queries (consumed by sqlc)
    migrations/                    Schema migrations
  lsp/                             LSP client manager, auto-discovery, on-demand startup
  ui/                              Bubble Tea v2 TUI (see internal/ui/AGENTS.md)
  permission/                      Tool permission checking and allow-lists
  skills/                          Skill file discovery and loading
  shell/                           Bash command execution with background job support
  terminal/                        Terminal multiplexer (tmux/screen) detection
  plugins/                         Claude Code plugin installation and discovery
  event/                           Telemetry (PostHog)
  pubsub/                          Internal pub/sub for cross-component messaging
  filetracker/                     Tracks files touched per session
  history/                         Prompt history
```

### Key Dependency Roles

- **`charm.land/fantasy`**: LLM provider abstraction. Handles protocol differences between Anthropic, OpenAI, Gemini, etc. Used in `internal/app` + `internal/agent`.
- **`charm.land/bubbletea/v2`**: TUI framework.
- **`charm.land/lipgloss/v2`**: Terminal styling.
- **`charm.land/glamour/v2`**: Markdown rendering in terminal.
- **`charm.land/catwalk`**: Snapshot/golden-file testing for TUI components.
- **`sqlc`**: Generates Go code from SQL queries in `internal/db/sql/`.

### Key Patterns

- **Config is Service**: accessed via `config.Service`, not global state.
- **Tools self-documenting**: each tool has `.go` implementation + `.md` description in `internal/agent/tools/`.
- **System prompts are Go templates**: `internal/agent/templates/*.md.tpl` with runtime data injected.
- **Context files**: Crush reads AGENTS.md, CRUSH.md, CLAUDE.md, GEMINI.md (and `.local` variants) from working directory for project instructions.
- **Persistence**: SQLite + sqlc. Queries in `internal/db/sql/`, generated code in `internal/db/`. Migrations in `internal/db/migrations/`.
- **Pub/sub**: `internal/pubsub` for decoupled communication between agent, UI, services.
- **Hooks**: User-defined shell commands in `crush.json` fire before tool execution. Engine (`internal/hooks/`) independent of fantasy + agent — takes inputs, runs commands, returns decisions. `hookedTool` decorator in `internal/agent/hooked_tool.go` wraps tools at coordinator level. Hooks run before permission checks. See `HOOKS.md`.
- **CGO disabled**: builds with `CGO_ENABLED=0` + `GOEXPERIMENT=greenteagc`.

## Terminal Multiplexer Detection

`internal/terminal` detects if Crush runs inside tmux, screen, or zellij. Two strategies:

1. **Environment variables** (fast path): checks `$TMUX`, `$STY`, `$ZELLIJ`.
2. **Process tree traversal** (fallback): walks `/proc/{pid}/status` + `/proc/{pid}/comm` upward from current PID to find tmux/screen/zellij ancestor. Works even when sudo/su clears env vars.

When tmux detected via `/proc`, package reads original `$TMUX` from ancestor process envs and may query tmux socket for window name (`tmux -S <socket> display-message -p '#{window_name}'`).

Result displayed in compact header + sidebar as `tmux:session@window` (e.g. `tmux:0@main`). Asterisk suffix = detection from `/proc` traversal rather than env vars.

## Clipboard and Multiplexer Integration

Crush copies text to system clipboard via multiple strategies:

1. **OSC 52 via Bubble Tea's `SetClipboard`** — sends `\e]52;c;<base64>\a` escape sequence. Works in terminals supporting OSC 52 (iTerm2, Kitty, Alacritty, GNOME Terminal, etc.) + inside tmux when `set-clipboard` is `on`.

2. **Multiplexer-specific workarounds** — each mux drops OSC 52 differently; `CopyToClipboard` in `internal/ui/common/common.go` applies tailored workaround based on `terminal.DetectMux()`:

   | Mux | Env var | Strategy | Why |
   |-----|---------|----------|-----|
   | **tmux** | `$TMUX` | `tmux load-buffer -` | tmux drops incoming OSC 52 when `set-clipboard=external` (default). `load-buffer` sets tmux's internal buffer, forwards to outer terminal via own OSC 52. |
   | **screen** | `$STY` | DCS passthrough to stdout | Screen doesn't recognize OSC 52 natively, forwards DCS passthrough (`ESC P ... ESC \\`) to outer terminal by default — no config needed. |
   | **zellij** | `$ZELLIJ` | OSC 52 to `/dev/tty` | Zellij intercepts OSC 52 from stdout not from terminal device. Writing to `/dev/tty` bypasses interception. |

3. **Native clipboard fallback** — when display server available (X11 or Wayland), `golang.design/x/clipboard` provides native access. Headless envs (no `DISPLAY` or `WAYLAND_DISPLAY`) use only OSC 52 paths.

### Why not DCS passthrough for tmux?

Earlier version used DCS passthrough. Requires `allow-passthrough on` in tmux config, **off** by default (tmux 3.3+). Default setting silently strips DCS passthrough. `tmux load-buffer` works regardless of `allow-passthrough`, no config changes needed.

Note: GNU screen's DCS passthrough works by default (no equivalent of `allow-passthrough`), so DCS approach correct for screen.

## Claude Code Plugin Support

`internal/plugins` provides installation + discovery for Claude Code plugins. Plugins are git repos with `.claude-plugin/plugin.json` manifest + optional `skills/` directory with `SKILL.md` files.

**Installation**: `crush plugin install <owner/repo>` clones to `~/.config/crush/plugins/<name>`. Plugin's `skills/` auto-added to discovery path during config loading (`internal/config/load.go`), no manual config needed.

**Listing**: `crush plugin list` shows installed plugins with metadata from `.claude-plugin/plugin.json`.

**Auto-discovery**: `plugins.SkillsDirs()` scans plugins dir at startup, returns all existing `skills/` subdirectories. Merged into `config.Options.SkillsPaths` alongside builtin skill dirs (`GlobalSkillsDirs()` + `ProjectSkillsDir()`).

### Plugin Slash Commands

Plugin skills auto-become slash commands. Each skill in plugin's `skills/` gets `/` command:

- **Namespaced**: `/plugin-name:skill-name` — e.g. `/caveman:caveman-review`
- **Bare shorthand**: `/skill-name` — e.g. `/caveman-review` (when unambiguous)

Plugins auto-namespaced by installation dir name under `~/.config/crush/plugins/`. Non-plugin skills (built-in, user, project) opt into slash commands via `user-invocable: true` in `SKILL.md` YAML frontmatter.

Slash command resolution uses two-pronged approach:
1. Direct workspace skill catalog lookup (always available, primary path)
2. Async-loaded custom commands cache (fallback)

Commands dialog (`/` on empty input) shows plugin skills grouped under `── plugin-name ──` separator headers, sorted alphabetically within each group. Non-plugin skills appear without group header.

Namespace detection in `internal/skills/catalog.go:pluginNamespace()` (matches `.../crush/plugins/<name>/skills` structure), slash command matching in `internal/ui/model/ui.go:handleSkillSlashCommand()`, command palette grouping in `internal/ui/dialog/commands.go:setCommandItems()` with `GroupHeaderItem` separators in `internal/ui/dialog/commands_item.go`.

## Build/Test/Lint Commands

- **Build**: `go build .` or `go run .`
- **Test**: `task test` or `go test ./...` (run single test: `go test ./internal/llm/prompt -run TestGetContextFromPaths`)
- **Update Golden Files**: `go test ./... -update` (regenerates `.golden` on output change)
  - Update specific package: `go test ./internal/tui/components/core -update`
- **Lint**: `task lint:fix`
- **Format**: `task fmt` (`gofumpt -w .`)
- **Modernize**: `task modernize` (code simplifications)
- **Dev**: `task dev` (runs with profiling)

## Code Style Guidelines

- **Imports**: `goimports`, group stdlib, external, internal.
- **Formatting**: gofumpt (stricter than gofmt), enabled in golangci-lint.
- **Naming**: Standard Go — PascalCase exported, camelCase unexported.
- **Types**: Prefer explicit types, use type aliases for clarity (e.g. `type AgentName string`).
- **Error handling**: Return errors explicitly, use `fmt.Errorf` for wrapping.
- **Context**: Always pass `context.Context` as first parameter.
- **Interfaces**: Define in consuming packages, keep small + focused.
- **Structs**: Use struct embedding for composition, group related fields.
- **Constants**: Typed constants with iota for enums, group in const blocks.
- **Testing**: testify's `require` package, parallel tests with `t.Parallel()`, `t.SetEnv()`. Always use `t.Tempdir()` for temp dirs — no need to remove.
- **JSON tags**: snake_case.
- **File permissions**: Octal notation (`0o755`, `0o644`).
- **Log messages**: Start with capital letter (e.g. "Failed to save session"). Enforced by `task lint:log`.
- **Comments**: End in periods unless at end of line.

## Testing with Mock Providers

When writing tests with provider configs, use mock providers to avoid API calls:

```go
func TestYourFunction(t *testing.T) {
    // Enable mock providers for testing
    originalUseMock := config.UseMockProviders
    config.UseMockProviders = true
    defer func() {
        config.UseMockProviders = originalUseMock
        config.ResetProviders()
    }()

    // Reset providers to ensure fresh mock data
    config.ResetProviders()

    // Your test code here - providers will now return mock data
    providers := config.Providers()
    // ... test logic
}
```

## Formatting

- ALWAYS format any Go code.
  - First, try `gofumpt -w .`.
  - If `gofumpt` not available, use `goimports`.
  - If `goimports` not available, use `gofmt`.
  - Or use `task fmt` to run `gofumpt -w .` on entire project, if `gofumpt` on `PATH`.

## Comments

- Comments on own lines: start with capital letters, end with periods. Wrap at 78 columns.

## Committing

- ALWAYS use semantic commits (`fix:`, `feat:`, `chore:`, `refactor:`, `docs:`, `sec:`, etc).
- Keep commits to one line (excluding attribution). Multi-line only when truly needed.

## Working on the TUI (UI)

Before working on TUI, read `internal/ui/AGENTS.md`.

## Styling System

Styling system in `internal/ui/styles/`, three layers:

- **`quickstyle.go`**: Stable base theme builder. `quickStyle(opts)` constructs `Styles` struct from `quickStyleOpts` — palette of design tokens (primary, secondary, fgBase, bgBase, success, error, etc.). Must be fully token-driven: never hardcode specific `charmtone.*` colors (except Chroma syntax highlighting, pending tokenization). Lets any theme reuse base without inheriting Charmtone-specific colors.
- **`themes.go`**: Concrete themes. Each theme (e.g. `CharmtonePantera`) calls `quickStyle` with its palette, then applies theme-specific overrides.
- **`styles.go`**: `Styles` struct + documentation — shape of what `quickStyle` produces.

**Adding theme-specific overrides**: When style needs color not fitting token model (e.g. bang prompt uses Salt/Hazy/Larple), keep `quickStyle` on closest semantic token, override only differing colors in theme function:

```go
func CharmtonePantera() Styles {
	s := quickStyle(quickStyleOpts{ /* palette */ })

	// Override only the colors that differ from the token defaults.
	s.Editor.PromptBangIconFocused = s.Editor.PromptBangIconFocused.
		Foreground(charmtone.Salt).
		Background(charmtone.Hazy)

	return s
}
```

**Adding new theme**: Add function in `themes.go` returning `quickStyle` result with `quickStyleOpts` palette (plus needed overrides), wire into `ThemeForProvider`.
