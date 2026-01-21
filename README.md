# Git Secret Scanner & Cleaner

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/Charm-TUI-FF69B4?style=for-the-badge" alt="Charm">
  <img src="https://img.shields.io/badge/License-MIT-green?style=for-the-badge" alt="License">
</p>

Beautiful terminal UI to scan and clean passwords/secrets from Git history.

Built with [Charm](https://charm.sh) tools:
- **[Bubbletea](https://github.com/charmbracelet/bubbletea)** - TUI framework
- **[Huh](https://github.com/charmbracelet/huh)** - Interactive forms
- **[Lipgloss](https://github.com/charmbracelet/lipgloss)** - Styling
- **[Bubbles](https://github.com/charmbracelet/bubbles)** - Components (spinner, progress)
- **[Log](https://github.com/charmbracelet/log)** - Logging

## Features

- **Interactive TUI** - Beautiful terminal interface, no CLI flags to remember
- **Optimized scanning** - Uses `git log -S` (pickaxe) for fast searching
- **3 scan modes** - Full, Fast, Stream for different repo sizes
- **3 scan sources** - Current files, Git history, or both
- **Configurable patterns** - Multiple regex formats for key-value extraction
- **Smart analysis** - See who changes secrets, frequency, CSV export
- **Safe cleaning** - Supports git-filter-repo, BFG, and filter-branch
- **Memory efficient** - Streaming mode for multi-GB repositories

## Screenshot

```
   _____ _ _     _____                     _
  / ____(_) |   / ____|                   | |
 | |  __ _| |_ | (___   ___  ___ _ __ ___ | |_
 | | |_ | | __|\___ \ / _ \/ __| '__/ _ \| __|
 | |__| | | |_ ____) |  __/ (__| | |  __/| |_
  \_____|_|\__|_____/ \___|\___|_|  \___| \__|
                                Scanner & Cleaner

  ▸ Scan Repository
    Search for passwords and secrets in git history

    Analyze Results
    View statistics, authors, and frequency of changes

    Clean History
    Remove secrets from git history (rewrite commits)

    Check Tools
    Verify and install cleaning tools (git-filter-repo, BFG)

    Quit
    Exit the application

  ↑/↓: navigate • enter: select • esc: quit
```

## Installation

### From Source

```bash
# Clone
git clone https://github.com/Drilmo/git-secret-scanner.git
cd git-secret-scanner

# Build
go build -o gitsecret ./cmd/gitsecret

# Run
./gitsecret
```

### Install git-filter-repo (recommended for cleaning)

```bash
# macOS (Homebrew)
brew install git-filter-repo

# Ubuntu/Debian
sudo apt install git-filter-repo

# pip
pip install git-filter-repo
```

## Usage

Simply run:

```bash
./gitsecret
```

Navigate with arrow keys, select with Enter, go back with Esc.

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate menus |
| `Enter` | Select / Confirm |
| `Esc` | Go back / Cancel |
| `Ctrl+E` | Open configuration (in Scan form) |
| `Ctrl+C` | Quit |

### Workflow

1. **Scan** - Search your repository for secrets
2. **Analyze** - Review what was found (frequency, authors, export CSV)
3. **Clean** - Remove secrets from git history and/or current files

## Scan Modes

| Mode | Description | Memory | Speed | Output |
|------|-------------|--------|-------|--------|
| **Full** | All history, aggregated results | Moderate | Medium | `.json` |
| **Stream** | Writes directly to file (JSONL) | Minimal | Medium | `.jsonl` |
| **Fast** | Quick check, same as Full | Low | Fast | `.json` |

## Scan Sources

| Source | Description |
|--------|-------------|
| **Both** | Current files (HEAD + untracked) + Git history |
| **Current** | Only current files (HEAD + untracked files) |
| **History** | Only Git history (all branches) |

## Configuration

The scanner looks for configuration in this order:

1. Form input (custom path via Ctrl+E)
2. `./patterns.json`
3. `./config/patterns.json`
4. `~/.config/git-secret-scanner/patterns.json`
5. Built-in defaults

### Example patterns.json

```json
{
  "extractionPatterns": [
    {
      "name": "key_equals_value",
      "pattern": "^\\s*([a-zA-Z_][\\w.$/-]*)\\s*=\\s*(.+)$",
      "valueGroup": 2,
      "description": "Standard key=value format"
    },
    {
      "name": "yaml_colon",
      "pattern": "^\\s*([a-zA-Z_][\\w._-]*)\\s*:\\s+['\"]?([^'\"\\n=]+)['\"]?\\s*$",
      "valueGroup": 2,
      "description": "YAML key: value format"
    },
    {
      "name": "json_quoted",
      "pattern": "\"([a-zA-Z_][\\w._]*)\"\\s*:\\s*\"([^\"]+)\"",
      "valueGroup": 2,
      "description": "JSON \"key\": \"value\" format"
    },
    {
      "name": "export_env",
      "pattern": "^\\s*export\\s+([A-Z_][A-Z0-9_]*)\\s*=\\s*['\"]?([^'\"\\n]+)['\"]?",
      "valueGroup": 2,
      "description": "Shell export KEY=value format"
    }
  ],
  "keywords": [
    {
      "name": "password",
      "patterns": ["password", "passwd", "pwd", "pass"],
      "description": "Passwords"
    },
    {
      "name": "secret",
      "patterns": ["secret", "client_secret", "api_secret"],
      "description": "Application secrets"
    },
    {
      "name": "api_key",
      "patterns": ["api_key", "apikey", "api-key"],
      "description": "API keys"
    },
    {
      "name": "token",
      "patterns": ["token", "access_token", "auth_token"],
      "description": "Authentication tokens"
    }
  ],
  "ignoredValues": ["<empty>", "null", "PLACEHOLDER", "example", "TODO"],
  "ignoredFiles": ["*.md", "*.go", "*.js", "node_modules/**", ".git/**"],
  "excludeBinaryExtensions": [".jar", ".zip", ".png", ".jpg", ".exe"],
  "settings": {
    "minSecretLength": 3,
    "maxSecretLength": 500,
    "caseSensitive": false
  }
}
```

### Extraction Patterns

The scanner supports multiple value formats via configurable regex patterns:

| Format | Example | Default Pattern |
|--------|---------|-----------------|
| Key=Value | `password=secret123` | `key_equals_value` |
| YAML | `password: secret123` | `yaml_colon` |
| JSON | `"password": "secret123"` | `json_quoted` |
| Export | `export PASSWORD=secret123` | `export_env` |

Each pattern specifies:
- `name`: Identifier for the pattern
- `pattern`: Regex with capture groups
- `valueGroup`: Which group (1-indexed) contains the secret value
- `description`: Human-readable description

## Analysis & Export

After scanning, use **Analyze Results** to:

- View global statistics (entries, unique secrets, unique values)
- See top authors (who commits secrets most)
- See top files (most impacted files)
- See secret types breakdown
- **Export to CSV** for spreadsheet analysis

The CSV export includes:
- File, Key, Type
- Change count, Total occurrences
- Authors, First seen, Last seen
- Days active, Masked values

## Cleaning Tools

| Tool | Installation | Performance |
|------|--------------|-------------|
| **git-filter-repo** | `pip install git-filter-repo` | Recommended |
| **BFG** | `brew install bfg` | Fast |
| **filter-branch** | Built-in | Slow |

### What the Cleaning Does

**Preserved:**
- All commits
- Commit messages
- Authors and emails
- Dates
- Branches and tags
- File structure

**Modified:**
- File contents: secrets replaced with `***REMOVED***`
- Commit SHA hashes (technical consequence)

### Cleaning Modes

The cleaner auto-detects from scan results:

| Mode | Description |
|------|-------------|
| **current** | Only current files (no history rewrite) |
| **history** | Only git history (rewrite commits) |
| **both** | Current files + git history |

### Example

```diff
# Before
db.password=Sup3rS3cr3t!2024
api.secret=sk_live_a1b2c3d4e5f6g7h8i9j0

# After
db.password=***REMOVED***
api.secret=***REMOVED***
```

## Post-Cleanup Steps

**Important**: After cleaning git history, all collaborators must:

```bash
# Delete old clone
rm -rf old-repo

# Re-clone
git clone <url>
```

**Rotate all secrets** - Even if removed from history, they were exposed:
- Database passwords
- API keys
- OAuth tokens
- Private keys

## Project Structure

```
.
├── cmd/gitsecret/          # Main entry point
├── internal/
│   ├── tui/                # Terminal UI (bubbletea, huh, lipgloss)
│   │   ├── tui.go          # Main TUI logic and navigation
│   │   ├── views.go        # View rendering and handlers
│   │   ├── forms.go        # Form definitions
│   │   └── styles.go       # Lipgloss styles
│   ├── scanner/            # Git history scanning
│   ├── analyzer/           # Results analysis & CSV export
│   ├── cleaner/            # History cleaning
│   └── config/             # Configuration handling
├── config/                 # Default patterns
├── patterns.json           # User configuration
├── go.mod
└── README.md
```

## License

MIT
