# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
go build -o gitsecret ./cmd/gitsecret   # Build binary
./gitsecret                               # Run (interactive TUI, no CLI flags)
go test -v ./...                          # Run all tests
go test -v ./internal/config/             # Run config tests only
```

Production build with version injection:
```bash
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X main.version=$VERSION" -o dist/gitsecret-linux-amd64 ./cmd/gitsecret
```

## Architecture

Go 1.23 single-binary CLI tool with an interactive TUI for scanning, analyzing, and cleaning secrets from Git history.

### Module Layout

- **`cmd/gitsecret/main.go`** — Entry point. Initializes logging and calls `tui.Run()`.
- **`internal/tui/`** — Terminal UI built on the Charm ecosystem (Bubbletea, Huh, Lipgloss). Follows Elm architecture (Model/Update/View).
  - `tui.go` — Main `Model` struct with 14 view states, `Update()` message dispatch, `View()` routing.
  - `views.go` — Per-view render functions and input handlers.
  - `forms.go` — Huh form definitions for scan/analyze/clean workflows.
  - `styles.go` — Lipgloss color constants and style definitions.
- **`internal/scanner/`** — Scans git history using `git log -S` (pickaxe). Supports Full (aggregated JSON), Stream (JSONL for large repos), and Fast modes. Executes git commands via `os/exec`.
- **`internal/analyzer/`** — Processes scan results (JSON/JSONL), computes statistics (top authors, top files, type breakdown), exports CSV.
- **`internal/cleaner/`** — Rewrites git history to replace secrets with `***REMOVED***`. Supports three backends: git-filter-repo (recommended), BFG, and git-filter-branch.
- **`internal/config/`** — Loads and merges pattern configuration from multiple sources (local file → project config → home config → built-in defaults). Defines extraction regex patterns, keyword groups, ignored values/files, and settings.

### Key Data Flow

1. **Scan**: User configures scan via TUI form → `scanner` runs `git log -S` per keyword → extracts key-value pairs using configurable regex patterns → outputs JSON or JSONL.
2. **Analyze**: Reads scan output file → aggregates statistics → displays in TUI or exports CSV.
3. **Clean**: Reads scan results → uses selected cleaning tool to rewrite git history → replaces secret values in matching files.

### Configuration Resolution Order

1. Custom path (via Ctrl+E in scan form)
2. `./patterns.json`
3. `./config/patterns.json`
4. `~/.config/git-secret-scanner/patterns.json`
5. Built-in defaults in `internal/config/config.go`

The `config/patterns.default.json` file defines the default pattern schema. User-provided `patterns.json` is gitignored.

### TUI State Machine

The TUI uses a `viewState` enum with 14 states (e.g., `viewMenu`, `viewScanForm`, `viewScanProgress`, `viewResults`, `viewCleanConfirm`). Navigation flows: Menu → Form → Progress → Results → Analysis/Clean. Esc goes back, Enter confirms.

## CI

GitHub Actions (`.github/workflows/ci.yml`) runs `go test` and `go build` on push/PR to main. Release builds trigger on version tags and produce cross-platform binaries (linux/macOS/windows, amd64/arm64).
