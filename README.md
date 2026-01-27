# Git Secret Scanner & Cleaner

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/Python-3.8+-3776AB?style=for-the-badge&logo=python&logoColor=white" alt="Python">
  <img src="https://img.shields.io/badge/Charm-TUI-FF69B4?style=for-the-badge" alt="Charm">
  <img src="https://img.shields.io/badge/License-MIT-green?style=for-the-badge" alt="License">
</p>

Interactive terminal tool to scan, analyze, and clean passwords/secrets from Git history.

Available in **two versions**:

| Version | Requirements | Interface | Dependencies |
|---------|-------------|-----------|-------------|
| **Go** (original) | Go 1.21+ | Rich TUI (Bubbletea) | Charm ecosystem |
| **Python** (enterprise) | Python 3.8+ | Interactive CLI | **Zero** (standard library only) |

The Python version is designed for **enterprise environments** where Go or pre-built binaries are not available. See [ENTERPRISE.md](ENTERPRISE.md) for deployment details.

## Features

- **Interactive interface** — No CLI flags to remember
- **Optimized scanning** — Uses `git log -S` (pickaxe) for fast searching
- **3 scan modes** — Full, Fast, Stream for different repo sizes
- **3 scan sources** — Current files, Git history, or both
- **Configurable patterns** — Multiple regex formats for key-value extraction
- **Smart analysis** — See who changes secrets, frequency, CSV export
- **Safe cleaning** — Supports git-filter-repo, BFG, and filter-branch
- **Memory efficient** — Streaming mode for multi-GB repositories

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

### Go version (from source)

```bash
git clone https://github.com/Drilmo/git-secret-scanner.git
cd git-secret-scanner

# Build
go build -o gitsecret ./cmd/gitsecret

# Run
./gitsecret
```

### Python version (no build needed)

```bash
git clone https://github.com/Drilmo/git-secret-scanner.git
cd git-secret-scanner/python

# Run directly
python3 gitsecret.py

# Or as a module
python3 -m gitsecret
```

No `pip install`, no compilation, no internet access required. Python 3.8+ only.

### Install git-filter-repo (optional, recommended for cleaning)

`git-filter-repo` is an **external git tool** used by the cleaning feature. It is **not** a dependency of this project — the built-in `git-filter-branch` always works as a fallback.

```bash
# macOS (Homebrew)
brew install git-filter-repo

# Ubuntu/Debian
sudo apt install git-filter-repo

# pip (if available)
pip install git-filter-repo

# Manual: git-filter-repo is a single Python script
# https://github.com/newren/git-filter-repo
```

---

## Usage

### Go version

```bash
./gitsecret
```

Navigate with arrow keys, select with Enter, go back with Esc.

### Python version

```bash
python3 gitsecret.py
```

Navigate with numbered menus, type values at prompts.

---

## Main Menu

The main menu provides 5 options:

| # | Option | Description |
|---|--------|-------------|
| 1 | **Scan Repository** | Search for passwords and secrets in git history |
| 2 | **Analyze Results** | View statistics, authors, and frequency of changes |
| 3 | **Clean History** | Remove secrets from git history (rewrite commits) |
| 4 | **Check Tools** | Verify and install cleaning tools |
| 5 | **Quit** | Exit the application |

---

## 1. Scan Repository

Scans a git repository for secrets (passwords, tokens, API keys, etc.) using `git log -S` (pickaxe search).

### Scan Form Options

| Option | Default | Description |
|--------|---------|-------------|
| **Repository Path** | `.` | Path to the git repository to scan. Can be relative or absolute. |
| **Scan Mode** | `full` | How to perform the scan (see table below). |
| **Source** | `both` | What to scan (see table below). |
| **Branch** | `--all` | Git branch or ref to scan. Use `--all` for all branches, `main` for a single branch. |
| **Output File** | `secrets.json` | Where to save scan results. Extension determines format (`.json` or `.jsonl`). |
| **Configuration** | Built-in defaults | Pattern configuration to use. Press `Ctrl+E` (Go) or enter a path (Python) to change. |

### Scan Modes

| Mode | Description | Memory | Speed | Output Format |
|------|-------------|--------|-------|---------------|
| **Full** | Scans all history, aggregates and deduplicates results into a single structured JSON file. Best for most repositories. | Moderate | Medium | `.json` |
| **Stream** | Writes each finding immediately to file as JSONL (one JSON object per line). Best for very large repositories where full mode runs out of memory. | Minimal | Medium | `.jsonl` |
| **Fast** | Quick check of current files only, no history traversal. Useful for a rapid assessment. | Low | Fast | `.json` |

### Scan Sources

| Source | What is scanned | Use case |
|--------|----------------|----------|
| **Both** | Current files (HEAD + untracked) **and** full git history (all commits) | Most thorough, recommended default |
| **Current** | Only files currently in the working directory (HEAD + untracked) | Quick check of current state |
| **History** | Only git commit history (all branches, all commits) | Find secrets that were removed from current files but still exist in history |

### Scan Output

**JSON format** (`.json`) — Aggregated results:
```json
{
  "repository": ".",
  "branch": "--all",
  "secretsFound": 42,
  "totalValues": 156,
  "secrets": [
    {
      "file": "config/database.yml",
      "key": "password",
      "type": "password",
      "changeCount": 5,
      "totalOccurrences": 12,
      "authors": ["Alice", "Bob"],
      "history": [...]
    }
  ],
  "scanDate": "2024-01-15T10:30:00Z"
}
```

**JSONL format** (`.jsonl`) — One entry per line:
```json
{"file":"config/db.yml","key":"password","value":"secret123","maskedValue":"se******23","type":"password","commit":"abc1234","author":"Alice","date":"2024-01-15T10:30:00Z"}
```

### How Scanning Works

1. For each keyword group (password, secret, token, api_key, etc.), runs:
   ```
   git log --all -S<keyword> --pretty=format:COMMIT_START|%H|%an|%aI -p
   ```
2. Parses the diff output to find lines containing the keyword
3. Applies extraction patterns (regex) to extract key-value pairs
4. Filters out false positives (code patterns, URLs, common placeholders)
5. Deduplicates and aggregates results

---

## 2. Analyze Results

Loads scan results and computes statistics: who commits secrets, which files are most impacted, and how often secrets change.

### Analyze Form Options

| Option | Default | Description |
|--------|---------|-------------|
| **Input File** | `secrets.json` | Path to scan results file. Accepts `.json` (full scan) or `.jsonl` (stream scan). |
| **CSV Output File** | `secrets_analysis.csv` | Where to export the detailed CSV report for spreadsheet analysis. |

### Analysis Output

Displays:

- **Global statistics** — Total entries, unique secrets, unique values
- **Top 10 authors** — Who commits/modifies secrets most frequently, with bar chart
- **Top 10 files** — Files containing the most secrets
- **Type breakdown** — Distribution by secret type (password, token, api_key, etc.)
- **Detailed secrets** — Each secret with change count, authors, date range, masked values

### CSV Export

The CSV file uses semicolon (`;`) separator with UTF-8 BOM for Excel compatibility.

**Columns:**

| Column | Description |
|--------|-------------|
| `File` | File path containing the secret |
| `Key` | The key/variable name (e.g., `password`, `api_key`) |
| `Type` | Secret type category (e.g., `password`, `token`, `aws`) |
| `ChangeCount` | Number of different values this secret has had |
| `TotalOccurrences` | Total number of times this secret appears across commits |
| `Authors` | Comma-separated list of authors who touched this secret |
| `AuthorCount` | Number of distinct authors |
| `FirstSeen` | Date of earliest commit containing this secret |
| `LastSeen` | Date of most recent commit containing this secret |
| `DaysActive` | Number of days between first and last seen |
| `Values` | Pipe-separated masked values (e.g., `se****23 \| xK****jL`) |

---

## 3. Clean History

Removes secrets from git history and/or current files by replacing them with `***REMOVED***`.

### Clean Form Options

| Option | Default | Description |
|--------|---------|-------------|
| **Scan Results File** | `secrets.json` | JSON or JSONL file containing secrets to remove (output from Scan). |
| **Repository Path** | `.` | Path to the git repository to clean. |
| **History Tool** | `auto` | Tool to use for rewriting git history (see table below). |
| **Dry Run** | `Yes` | Simulate the operation without making changes. Always recommended first. |
| **Proceed** | `Cancel` | Final confirmation before starting. |

If **Dry Run = No**, a second confirmation prompt appears:

> **WARNING: This will rewrite git history!**
> This operation cannot be undone. Make sure you have a backup.
> All collaborators will need to re-clone the repository.

### Cleaning Tools

| Tool | Installation | Speed | Recommendation |
|------|-------------|-------|----------------|
| **git-filter-repo** | `brew install git-filter-repo` or `pip install git-filter-repo` | Fast | **Recommended**. Modern, safe, well-maintained. |
| **BFG Repo Cleaner** | `brew install bfg` | Fast | Good alternative. Requires Java. |
| **git-filter-branch** | Built-in (no install) | Slow | **Always available** as fallback. No external dependencies. |

When set to `auto`, the tool selects the best available: filter-repo > BFG > filter-branch.

### Cleaning Source (auto-detected)

The cleaner auto-detects the source from the scan results file:

| Source | What happens | When detected |
|--------|-------------|---------------|
| **current** | Replaces secrets in working directory files only. No history rewrite. | Scan was "current files only" |
| **history** | Rewrites git commit history. Current files unchanged. | Scan was "history only" |
| **both** | Replaces in current files **and** rewrites history. | Scan included both sources |

### What Cleaning Preserves and Modifies

**Preserved:**
- All commits, messages, authors, dates
- Branches and tags structure
- File structure and non-secret content

**Modified:**
- Secret values replaced with `***REMOVED***`
- Commit SHA hashes change (unavoidable consequence of history rewriting)

### Cleaning Example

```diff
# Before
db.password=Sup3rS3cr3t!2024
api.secret=sk_live_a1b2c3d4e5f6g7h8i9j0

# After
db.password=***REMOVED***
api.secret=***REMOVED***
```

### Dry Run Output

When dry run is enabled, the tool shows:
- Target (current/history/both)
- Tool that would be used
- Number of secrets to remove
- Number of regex patterns
- Preview of first 10 secret values (masked)

### Post-Clean Next Steps

**After cleaning current files only:**
1. Review changes: `git diff`
2. Commit: `git add -A && git commit -m 'Remove secrets'`
3. Push: `git push`
4. Rotate all exposed credentials

**After cleaning git history:**
1. Verify: `git log -p -S 'secret_value'`
2. Force push: `git push --force --all`
3. Force push tags: `git push --force --tags`
4. Notify collaborators to re-clone
5. Rotate all exposed credentials

---

## 4. Check Tools

Shows the installation status of cleaning tools and provides installation instructions.

### Tool Status Display

| Tool | Status | Description |
|------|--------|-------------|
| **git-filter-repo** | Installed / Not installed | Recommended — Fast and safe |
| **BFG Repo Cleaner** | Installed / Not installed | Alternative — Java based |
| **git-filter-branch** | Always available | Built-in — Slow but always works |

### Installation Methods (Go TUI)

In the Go version, selecting a non-installed tool shows installation methods:

**git-filter-repo:**
| Method | Command |
|--------|---------|
| Homebrew (macOS) | `brew install git-filter-repo` |
| pip (Python) | `pip install git-filter-repo` |
| pip3 (Python 3) | `pip3 install git-filter-repo` |
| apt (Ubuntu/Debian) | `sudo apt install -y git-filter-repo` |

**BFG:**
| Method | Command |
|--------|---------|
| Homebrew (macOS) | `brew install bfg` |

---

## Keyboard Shortcuts (Go TUI)

| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate menus |
| `Enter` | Select / Confirm |
| `Esc` | Go back / Cancel |
| `Ctrl+E` | Open configuration (in Scan form) |
| `Backspace` | Go up one directory (in file browser) |
| `Ctrl+C` | Quit |

---

## Configuration

The scanner looks for configuration in this order:

1. Custom path (via `Ctrl+E` in Go TUI or input in Python)
2. `./patterns.json`
3. `./config/patterns.json`
4. `~/.config/git-secret-scanner/patterns.json`
5. Built-in defaults

### Configuration Management (Go TUI)

The configuration menu (accessible via `Ctrl+E` from the Scan form or from the main menu) provides:

| Option | Description |
|--------|-------------|
| **View Current** | Shows loaded keyword groups, settings (min/max length, case sensitivity), and first 5 ignored values |
| **Create New** | Creates a new `patterns.json` file with all built-in defaults, at a path you specify |
| **Select Config** | Choose from discovered config files (built-in defaults, local `.json` files, home directory config) or browse the filesystem |

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
      "patterns": ["password", "passwd", "pwd", "pass", "mot_de_passe"],
      "description": "Passwords"
    },
    {
      "name": "secret",
      "patterns": ["secret", "client_secret", "app_secret", "api_secret"],
      "description": "Application secrets"
    },
    {
      "name": "api_key",
      "patterns": ["api_key", "apikey", "api-key"],
      "description": "API keys"
    },
    {
      "name": "token",
      "patterns": ["token", "access_token", "auth_token", "bearer"],
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
- `valueGroup`: Which capture group (1-indexed) contains the secret value
- `description`: Human-readable description

### Default Keyword Groups

| Group | Patterns | Description |
|-------|----------|-------------|
| `password` | password, passwd, pwd, pass, mot_de_passe | Passwords |
| `secret` | secret, client_secret, app_secret, api_secret | Application secrets |
| `api_key` | api_key, apikey, api-key | API keys |
| `token` | token, access_token, auth_token, bearer | Authentication tokens |
| `credentials` | credential, credentials, auth | Credentials |
| `private_key` | private_key, privatekey, private-key, rsa_private | Private keys |
| `connection_string` | connection_string, connectionstring, conn_str, database_url, db_url | Connection strings |
| `oauth` | oauth, client_id, client_secret, refresh_token | OAuth tokens |
| `aws` | aws_access_key, aws_secret, aws_key | AWS credentials |
| `encryption` | encryption_key, encrypt_key, aes_key, cipher | Encryption keys |

### False Positive Filtering

The scanner automatically filters out:

| Filter | Examples |
|--------|---------|
| **Too short/long** | Values < 3 or > 500 characters |
| **Code patterns** | `append(foo)`, `config.Value`, `entry.Date`, `make([]byte)` |
| **URLs** | `https://example.com`, `ssh://git@...` |
| **Exact keyword match** | Value is literally "password", "secret", "token" |
| **Placeholders** | `<empty>`, `null`, `${VAR}`, `{{template}}`, `PLACEHOLDER`, `TODO` |
| **Ignored files** | `*.md`, `*.go`, `*.js`, `*.py`, `node_modules/**`, `.git/**` |
| **Binary files** | `.jar`, `.png`, `.exe`, `.pdf`, etc. |

### Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `minSecretLength` | `3` | Minimum character length for a value to be considered a secret |
| `maxSecretLength` | `500` | Maximum character length |
| `caseSensitive` | `false` | Whether keyword searches are case-sensitive |

---

## Workflow

The recommended workflow is:

```
1. Scan → 2. Analyze → 3. Clean
```

1. **Scan** your repository to find secrets
2. **Analyze** the results to understand scope and impact
3. **Clean** to remove secrets (dry run first, then for real)
4. **Rotate** all exposed credentials

---

## Project Structure

```
.
├── cmd/gitsecret/              # Go: main entry point
├── internal/
│   ├── tui/                    # Go: Terminal UI (bubbletea, huh, lipgloss)
│   │   ├── tui.go              # Main TUI logic, navigation, state machine
│   │   ├── views.go            # View rendering and update handlers
│   │   ├── forms.go            # Huh form definitions
│   │   └── styles.go           # Lipgloss styles and colors
│   ├── scanner/                # Go: Git history scanning
│   ├── analyzer/               # Go: Results analysis & CSV export
│   ├── cleaner/                # Go: History cleaning
│   └── config/                 # Go: Configuration handling
├── python/                     # Python version (enterprise)
│   ├── gitsecret.py            # Launcher: python3 gitsecret.py
│   └── gitsecret/
│       ├── __init__.py         # Package metadata
│       ├── __main__.py         # python3 -m gitsecret
│       ├── config.py           # Configuration management
│       ├── scanner.py          # Git history scanning
│       ├── analyzer.py         # Results analysis & CSV export
│       ├── cleaner.py          # History cleaning
│       ├── tui.py              # Interactive CLI
│       └── styles.py           # ANSI colors and formatting
├── config/                     # Default pattern files
│   ├── patterns.default.json   # Default configuration schema
│   └── patterns.schema.json    # JSON Schema validation
├── ENTERPRISE.md               # Enterprise deployment guide (French)
├── CLAUDE.md                   # Developer guidelines
├── go.mod
└── README.md
```

---

## License

MIT
