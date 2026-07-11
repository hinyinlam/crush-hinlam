# Crush Development Guide

## Project Overview

Crush is a terminal-based AI coding assistant built in Go by
[Charm](https://charm.land). It connects to LLMs and gives them tools to read,
write, and execute code. It supports multiple providers (Anthropic, OpenAI,
Gemini, Bedrock, Copilot, Hyper, MiniMax, Vercel, and more), integrates with
LSPs for code intelligence, and supports extensibility via MCP servers and
agent skills.

The module path is `github.com/charmbracelet/crush`.

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

- **`charm.land/fantasy`**: LLM provider abstraction layer. Handles protocol
  differences between Anthropic, OpenAI, Gemini, etc. Used in `internal/app`
  and `internal/agent`.
- **`charm.land/bubbletea/v2`**: TUI framework powering the interactive UI.
- **`charm.land/lipgloss/v2`**: Terminal styling.
- **`charm.land/glamour/v2`**: Markdown rendering in the terminal.
- **`charm.land/catwalk`**: Snapshot/golden-file testing for TUI components.
- **`sqlc`**: Generates Go code from SQL queries in `internal/db/sql/`.

### Key Patterns

- **Config is a Service**: accessed via `config.Service`, not global state.
- **Tools are self-documenting**: each tool has a `.go` implementation and a
  `.md` description file in `internal/agent/tools/`.
- **System prompts are Go templates**: `internal/agent/templates/*.md.tpl`
  with runtime data injected.
- **Context files**: Crush reads AGENTS.md, CRUSH.md, CLAUDE.md, GEMINI.md
  (and `.local` variants) from the working directory for project-specific
  instructions.
- **Persistence**: SQLite + sqlc. All queries live in `internal/db/sql/`,
  generated code in `internal/db/`. Migrations in `internal/db/migrations/`.
- **Pub/sub**: `internal/pubsub` for decoupled communication between agent,
  UI, and services.
- **Hooks**: User-defined shell commands in `crush.json` that fire before
  tool execution. The engine (`internal/hooks/`) is independent of fantasy
  and agent — it takes inputs, runs commands, returns decisions. The
  `hookedTool` decorator in `internal/agent/hooked_tool.go` wraps tools at
  the coordinator level. Hooks run before permission checks. See
  `HOOKS.md` for the user-facing protocol.
- **CGO disabled**: builds with `CGO_ENABLED=0` and
  `GOEXPERIMENT=greenteagc`.

## Terminal Multiplexer Detection

The `internal/terminal` package detects whether Crush is running inside a
terminal multiplexer (tmux, screen, or zellij). Detection uses two
strategies:

1. **Environment variables** (fast path): checks `$TMUX`, `$STY`, and
   `$ZELLIJ`.
2. **Process tree traversal** (fallback): walks `/proc/{pid}/status` and
   `/proc/{pid}/comm` upward from the current PID to find a `tmux`,
   `screen`, or `zellij` ancestor. This works even when sudo/su clears
   environment variables.

When tmux is detected via `/proc` fallback, the package also reads the
original `$TMUX` value from ancestor process environments and may query
the tmux socket for the current window name (`tmux -S <socket>
display-message -p '#{window_name}'`).

The detection result is displayed in the compact header and sidebar as
`tmux:session@window` (e.g. `tmux:0@main`). An asterisk suffix indicates
the detection came from `/proc` traversal rather than environment
variables.

## Clipboard and Multiplexer Integration

Crush copies text to the system clipboard using multiple strategies,
depending on the environment:

1. **OSC 52 via Bubble Tea's `SetClipboard`** — sends a plain
   `\e]52;c;<base64>\a` escape sequence. This works in terminals that
   support OSC 52 directly (iTerm2, Kitty, Alacritty, GNOME Terminal,
   etc.) and inside tmux when `set-clipboard` is `on`.

2. **Multiplexer-specific workarounds** — each mux intercepts or drops
   OSC 52 from stdout differently, so `CopyToClipboard` in
   `internal/ui/common/common.go` applies a tailored workaround based on
   `terminal.DetectMux()`:

   | Mux | Env var | Strategy | Why |
   |-----|---------|----------|-----|
   | **tmux** | `$TMUX` | `tmux load-buffer -` | tmux drops incoming OSC 52 when `set-clipboard=external` (default). `load-buffer` sets tmux's internal buffer, which it then forwards to the outer terminal via its own OSC 52. |
   | **screen** | `$STY` | DCS passthrough to stdout | Screen doesn't recognize OSC 52 natively, but forwards DCS passthrough (`ESC P ... ESC \\`) to the outer terminal by default — no config needed. |
   | **zellij** | `$ZELLIJ` | OSC 52 to `/dev/tty` | Zellij intercepts OSC 52 from stdout but not from writes to the terminal device. Writing to `/dev/tty` bypasses the interception. |

3. **Native clipboard fallback** — when a display server is available
   (X11 or Wayland), the `golang.design/x/clipboard` package provides
   native clipboard access. In headless environments (no `DISPLAY` or
   `WAYLAND_DISPLAY`), only the OSC 52 paths are used.

### Why not DCS passthrough for tmux?

An earlier version used DCS passthrough for tmux. This requires
`allow-passthrough on` in the tmux config, which is **off** by default
(tmux 3.3+). With the default setting, tmux silently strips DCS
passthrough sequences. The `tmux load-buffer` approach works regardless
of `allow-passthrough` and does not require any tmux configuration
changes.

Note: GNU screen's DCS passthrough works by default (no equivalent of
`allow-passthrough` exists), so the DCS approach is correct for screen.

## Claude Code Plugin Support

The `internal/plugins` package provides installation and discovery for
Claude Code plugins. Plugins are git repositories containing a
`.claude-plugin/plugin.json` manifest and optionally a `skills/` directory
with `SKILL.md` files.

**Installation**: `crush plugin install <owner/repo>` clones the repository
to `~/.config/crush/plugins/<name>`. The plugin's `skills/` directory is
automatically added to the skills discovery path during config loading
(`internal/config/load.go`), so installed plugin skills are available
without manual configuration.

**Listing**: `crush plugin list` shows all installed plugins with metadata
parsed from their `.claude-plugin/plugin.json` manifests.

**Auto-discovery**: `plugins.SkillsDirs()` scans the plugins directory at
startup and returns all `skills/` subdirectories that exist. These are
merged into `config.Options.SkillsPaths` alongside the built-in skill
directories (`GlobalSkillsDirs()` and `ProjectSkillsDir()`).

### Plugin Slash Commands

Plugin skills automatically become slash commands. Each skill defined in a
plugin's `skills/` directory gets a `/` command:

- **Namespaced**: `/plugin-name:skill-name` — e.g. `/caveman:caveman-review`
- **Bare shorthand**: `/skill-name` — e.g. `/caveman-review`
  (shorthand when unambiguous)

Plugins are auto-namespaced by their installation directory name under
`~/.config/crush/plugins/`. Skills from non-plugin sources (built-in,
user, project) can opt into slash commands by setting
`user-invocable: true` in their `SKILL.md` YAML frontmatter.

Slash command resolution uses a two-pronged approach:
1. Direct workspace skill catalog lookup (always available, primary path)
2. Async-loaded custom commands cache (fallback)

The commands dialog (`/` on empty input) shows plugin skills grouped
under `── plugin-name ──` separator headers, sorted alphabetically within
each group. Non-plugin skills appear without a group header.

The namespace detection is in `internal/skills/catalog.go:pluginNamespace()`
(matches `.../crush/plugins/<name>/skills` structure), slash command
matching in `internal/ui/model/ui.go:handleSkillSlashCommand()`, and
command palette grouping in
`internal/ui/dialog/commands.go:setCommandItems()` with
`GroupHeaderItem` separators defined in
`internal/ui/dialog/commands_item.go`.

## Build/Test/Lint Commands

- **Build**: `go build .` or `go run .`
- **Test**: `task test` or `go test ./...` (run single test:
  `go test ./internal/llm/prompt -run TestGetContextFromPaths`)
- **Update Golden Files**: `go test ./... -update` (regenerates `.golden`
  files when test output changes)
  - Update specific package:
    `go test ./internal/tui/components/core -update` (in this case,
    we're updating "core")
- **Lint**: `task lint:fix`
- **Format**: `task fmt` (`gofumpt -w .`)
- **Modernize**: `task modernize` (runs `modernize` which makes code
  simplifications)
- **Dev**: `task dev` (runs with profiling enabled)

## Code Style Guidelines

- **Imports**: Use `goimports` formatting, group stdlib, external, internal
  packages.
- **Formatting**: Use gofumpt (stricter than gofmt), enabled in
  golangci-lint.
- **Naming**: Standard Go conventions — PascalCase for exported, camelCase
  for unexported.
- **Types**: Prefer explicit types, use type aliases for clarity (e.g.,
  `type AgentName string`).
- **Error handling**: Return errors explicitly, use `fmt.Errorf` for
  wrapping.
- **Context**: Always pass `context.Context` as first parameter for
  operations.
- **Interfaces**: Define interfaces in consuming packages, keep them small
  and focused.
- **Structs**: Use struct embedding for composition, group related fields.
- **Constants**: Use typed constants with iota for enums, group in const
  blocks.
- **Testing**: Use testify's `require` package, parallel tests with
  `t.Parallel()`, `t.SetEnv()` to set environment variables. Always use
  `t.Tempdir()` when in need of a temporary directory. This directory does
  not need to be removed.
- **JSON tags**: Use snake_case for JSON field names.
- **File permissions**: Use octal notation (0o755, 0o644) for file
  permissions.
- **Log messages**: Log messages must start with a capital letter (e.g.,
  "Failed to save session" not "failed to save session").
  - This is enforced by `task lint:log` which runs as part of `task lint`.
- **Comments**: End comments in periods unless comments are at the end of the
  line.

## Testing with Mock Providers

When writing tests that involve provider configurations, use the mock
providers to avoid API calls:

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

- ALWAYS format any Go code you write.
  - First, try `gofumpt -w .`.
  - If `gofumpt` is not available, use `goimports`.
  - If `goimports` is not available, use `gofmt`.
  - You can also use `task fmt` to run `gofumpt -w .` on the entire project,
    as long as `gofumpt` is on the `PATH`.

## Comments

- Comments that live on their own lines should start with capital letters and
  end with periods. Wrap comments at 78 columns.

## Committing

- ALWAYS use semantic commits (`fix:`, `feat:`, `chore:`, `refactor:`,
  `docs:`, `sec:`, etc).
- Try to keep commits to one line, not including your attribution. Only use
  multi-line commits when additional context is truly necessary.

## Working on the TUI (UI)

Anytime you need to work on the TUI, read `internal/ui/AGENTS.md` before
starting work.

## Styling System

The styling system lives in `internal/ui/styles/` and is organized into
three layers:

- **`quickstyle.go`**: The stable base theme builder. `quickStyle(opts)`
  constructs a `Styles` struct from `quickStyleOpts` — a palette of
  design tokens (primary, secondary, fgBase, bgBase, success, error, etc.).
  `quickStyle` must be fully token-driven: never hardcode specific
  `charmtone.*` colors here (except Chroma syntax highlighting, which is
  pending tokenization). This lets any theme reuse the base without
  inheriting Charmtone-specific colors.
- **`themes.go`**: Defines concrete themes. Each theme function (e.g.
  `CharmtonePantera`) calls `quickStyle` with its palette, then applies
  theme-specific overrides as needed.
- **`styles.go`**: Defines the `Styles` struct and its documentation —
  the shape of what `quickStyle` produces.

**Adding theme-specific overrides**: When a style genuinely needs a
color that doesn't fit the token model (e.g. the bang prompt uses
Salt/Hazy/Larple), keep `quickStyle` on the closest semantic token and
override only the differing colors in the theme function:

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

**Adding a new theme**: Add a function in `themes.go` that returns the
result of `quickStyle` with a `quickStyleOpts` palette (plus any needed
overrides), then wire it into `ThemeForProvider`.
