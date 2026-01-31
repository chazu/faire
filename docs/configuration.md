# Configuration Reference

This is the complete reference for gitsavvy configuration options.

## Configuration File Location

The configuration file is stored at:

- **Linux/macOS**: `~/.config/gitsavvy/config.toml`
- **Windows**: `%APPDATA%\gitsavvy\config.toml`

## Configuration Structure

```toml
[repo]
# Workflow repository settings

[identity]
# Your identity and write path

[git]
# Git-specific settings

[workflows]
# Workflow storage and indexing

[runner]
# Workflow execution behavior

[placeholders]
# Placeholder/parameter behavior

[tui]
# Terminal UI settings

[editor]
# External editor configuration

[ai]
# AI provider settings (optional)
```

## Section: `[repo]`

Workflow repository configuration.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `path` | string | `~/.local/share/gitsavvy/repo` | Local filesystem path to the Git repository |
| `remote` | string | `origin` | Git remote name for push/pull operations |
| `branch` | string | `main` | Default branch name (auto-detected when possible) |
| `sync_strategy` | string | `rebase` | How to integrate remote changes: `ff-only`, `rebase`, or `merge` |
| `auto_reindex` | bool | `true` | Rebuild search index after sync operations |

### Sync Strategies

- **`ff-only`**: Only fast-forward merges. Fails if your local commits diverge.
- **`rebase`**: Rebase your local commits on top of remote (default).
- **`merge`**: Create a merge commit when integrating remote changes.

Example:

```toml
[repo]
path = "~/Documents/workflows"
remote = "upstream"
branch = "main"
sync_strategy = "rebase"
auto_reindex = true
```

## Section: `[identity]`

Your identity path determines where workflows are saved in the repository.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `path` | string | *(required)* | Your claimed path (e.g., `chaz` or `platform/chaz`) |
| `mode` | string | `pr` | Write mode: `direct` or `pr` |
| `team_prefix` | string | *(empty)* | Optional validation prefix for team paths |

### Identity Path

The path is relative to the repository root and determines where workflows are saved:

- `path = "chaz"` → Saves to `workflows/chaz/`
- `path = "platform/chaz"` → Saves to `workflows/platform/chaz/`

**Constraints:**
- Cannot contain `..` (path traversal)
- Cannot be an absolute path
- Should use forward slashes

### Write Modes

- **`direct`**: Commit directly to the configured branch. Requires push access.
- **`pr`** (default): Create feature branches and pull requests for changes.

Example:

```toml
[identity]
path = "platform/chaz"
mode = "pr"
team_prefix = "platform/"
```

## Section: `[git]`

Git-specific settings for workflow commits.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `author_name` | string | *(git config)* | Default git author name |
| `author_email` | string | *(git config)* | Default git author email |
| `sign_commits` | bool | `false` | Enable GPG signing of commits |
| `push_on_save` | bool | `false` | Automatically push after saving (direct mode only) |
| `pr_base_branch` | string | `main` | Base branch for pull requests (PR mode) |
| `feature_branch_template` | string | `gitsavvy/{identity}/{date}/{slug}` | Template for feature branch names |

### Feature Branch Template

Available placeholders:
- `{identity}` - Your identity path
- `{date}` - Current date (YYYY-MM-DD)
- `{slug}` - URL-safe workflow title

Examples:

```toml
# Default template
feature_branch_template = "gitsavvy/{identity}/{date}/{slug}"
# Result: gitsavvy/chaz/2026-01-30/deploy-application

# Custom template
feature_branch_template = "wf/{identity}/{slug}"
# Result: wf/chaz/deploy-application
```

Example:

```toml
[git]
author_name = "Jane Doe"
author_email = "jane@example.com"
sign_commits = true
push_on_save = false
pr_base_branch = "main"
feature_branch_template = "gitsavvy/{identity}/{date}/{slug}"
```

## Section: `[workflows]`

Workflow storage and indexing configuration.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `root` | string | `workflows` | Repo-relative path to user workflows |
| `shared_root` | string | `shared` | Repo-relative path to shared/team workflows |
| `draft_root` | string | `drafts` | Repo-relative path to draft workflows |
| `index_path` | string | `.gitsavvy/index.json` | Repo-relative path to search index |
| `schema_version` | int | `1` | Workflow schema version |

### Directory Layout

With default settings:

```
repo/
├── workflows/
│   └── {identity}/        # Your workflows
├── shared/                # Team workflows
├── drafts/                # Draft workflows
└── .gitsavvy/
    └── index.json         # Search index
```

Example:

```toml
[workflows]
root = "workflows"
shared_root = "shared"
draft_root = "drafts"
index_path = ".gitsavvy/index.json"
schema_version = 1
```

## Section: `[runner]`

Workflow execution behavior.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `default_shell` | string | *(detected)* | Default shell for steps: `bash`, `zsh`, `sh`, or `pwsh` |
| `confirm_each_step` | bool | `true` | Prompt before executing each step |
| `stream_output` | bool | `true` | Stream command output in real-time |
| `max_output_lines` | int | `5000` | Maximum output lines to keep in memory |
| `dangerous_command_warnings` | bool | `true` | Warn for potentially dangerous commands |

### Shells

Supported shells:
- `bash` - Bourne Again Shell
- `zsh` - Z Shell
- `sh` - POSIX Shell
- `pwsh` - PowerShell Core

The default is auto-detected from your `SHELL` environment variable.

### Dangerous Commands

When enabled, warns before executing commands that:
- Delete data (`rm`, `rmdir`, `del`)
- Modify system state (`kubectl delete`, `terraform destroy`)
- Override files (`>`, `|`)

Example:

```toml
[runner]
default_shell = "zsh"
confirm_each_step = true
stream_output = true
max_output_lines = 10000
dangerous_command_warnings = true
```

## Section: `[placeholders]`

Placeholder (parameter) behavior.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `prompt_style` | string | `form` | How to collect values: `form` or `per-step` |
| `save_defaults` | string | `none` | Save default values: `none`, `keychain`, or `file` |
| `keychain_service` | string | `gitsavvy` | Service name for keychain storage |

### Prompt Styles

- **`form`** (default): Collect all placeholder values upfront before running any steps
- **`per-step`**: Prompt for values as each step is reached

### Saving Defaults

- **`none`**: Don't save placeholder values
- **`keychain`**: Save to OS keychain (macOS Keychain, Windows Credential Manager, etc.)
- **`file`**: Save to plaintext file at `~/.config/gitsavvy/placeholders.toml`

Example:

```toml
[placeholders]
prompt_style = "form"
save_defaults = "keychain"
keychain_service = "gitsavvy"
```

## Section: `[tui]`

Terminal UI settings for Bubble Tea interfaces.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | `true` | Enable TUI (fallback to CLI if disabled) |
| `theme` | string | `default` | Color theme: `default`, `dracula`, `nord`, etc. |
| `show_help` | bool | `true` | Show help panel by default |

Example:

```toml
[tui]
enabled = true
theme = "default"
show_help = true
```

## Section: `[editor]`

External editor configuration.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `command` | string | *(empty)* | Editor command (uses `$EDITOR` if unset) |

If empty, uses the `EDITOR` environment variable. Falls back to `vi` if neither is set.

Example:

```toml
[editor]
command = "code --wait"  # VS Code
# or
command = "vim"          # Vim
# or
command = "nano"         # Nano
```

## Section: `[ai]`

AI provider settings for workflow generation (optional feature).

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | `false` | Enable AI features |
| `provider` | string | `openai_compat` | AI provider: `openai_compat`, `anthropic`, etc. |
| `base_url` | string | *(empty)* | Base URL for API requests (for compatible endpoints) |
| `model` | string | *(empty)* | AI model identifier (e.g., `gpt-4`, `claude-3-opus`) |
| `api_key_env` | string | *(empty)* | Environment variable containing API key |
| `redact` | string | `basic` | Privacy level: `none`, `basic`, `strict` |
| `confirm_send` | bool | `true` | Prompt before sending data to AI |

### Providers

- **`openai_compat`**: OpenAI-compatible API (includes OpenAI, Together, local LLMs)
- **`anthropic`**: Anthropic Claude API

### Redaction Levels

- **`none`**: Send all data without redaction
- **`basic`**: Redact obvious secrets (API keys, passwords, tokens)
- **`strict`**: Redact all potentially sensitive information (IPs, filenames, paths)

### Example: OpenAI

```toml
[ai]
enabled = true
provider = "openai_compat"
base_url = "https://api.openai.com/v1"
model = "gpt-4"
api_key_env = "OPENAI_API_KEY"
redact = "basic"
confirm_send = true
```

### Example: Anthropic Claude

```toml
[ai]
enabled = true
provider = "anthropic"
model = "claude-3-opus-20240229"
api_key_env = "ANTHROPIC_API_KEY"
redact = "basic"
confirm_send = true
```

### Example: Local LLM (Ollama)

```toml
[ai]
enabled = true
provider = "openai_compat"
base_url = "http://localhost:11434/v1"
model = "llama2"
api_key_env = "OLLAMA_API_KEY"  # May not be needed for local
redact = "none"
confirm_send = false
```

## Environment Variable Overrides

Configuration options can be overridden via environment variables. Variables use the `GITSAVVY_` prefix and double underscores (`__`) for nested sections.

| Environment Variable | Maps To |
|---------------------|---------|
| `GITSAVVY_REPO__PATH` | `repo.path` |
| `GITSAVVY_IDENTITY__PATH` | `identity.path` |
| `GITSAVVY_IDENTITY__MODE` | `identity.mode` |
| `GITSAVVY_GIT__AUTHOR_NAME` | `git.author_name` |
| `GITSAVVY_GIT__AUTHOR_EMAIL` | `git.author_email` |
| `GITSAVVY_WORKFLOWS__ROOT` | `workflows.root` |
| `GITSAVVY_RUNNER__DEFAULT_SHELL` | `runner.default_shell` |
| `GITSAVVY_AI__ENABLED` | `ai.enabled` |
| `GITSAVVY_AI__API_KEY_ENV` | `ai.api_key_env` |

Example:

```bash
# Override repo path
export GITSAVVY_REPO__PATH="~/my-workflows"

# Enable AI for current session
export GITSAVVY_AI__ENABLED="true"

# Use specific shell
export GITSAVVY_RUNNER__DEFAULT_SHELL="bash"
```

## Complete Example Configuration

```toml
# gitsavvy configuration file

[repo]
path = "~/.local/share/gitsavvy/repo"
remote = "origin"
branch = "main"
sync_strategy = "rebase"
auto_reindex = true

[identity]
path = "chaz"
mode = "pr"
team_prefix = ""

[git]
author_name = "Jane Doe"
author_email = "jane@example.com"
sign_commits = false
push_on_save = false
pr_base_branch = "main"
feature_branch_template = "gitsavvy/{identity}/{date}/{slug}"

[workflows]
root = "workflows"
shared_root = "shared"
draft_root = "drafts"
index_path = ".gitsavvy/index.json"
schema_version = 1

[runner]
default_shell = "zsh"
confirm_each_step = true
stream_output = true
max_output_lines = 5000
dangerous_command_warnings = true

[placeholders]
prompt_style = "form"
save_defaults = "none"
keychain_service = "gitsavvy"

[tui]
enabled = true
theme = "default"
show_help = true

[editor]
command = ""

[ai]
enabled = false
provider = "openai_compat"
base_url = ""
model = ""
api_key_env = ""
redact = "basic"
confirm_send = true
```

## Validation

Configuration is validated on load. Common validation errors:

| Error | Cause | Fix |
|-------|-------|-----|
| `repo.path cannot be empty` | Missing or empty `repo.path` | Set a valid path |
| `identity.path cannot be empty` | Missing or empty `identity.path` | Set your identity path |
| `identity.path cannot contain '..'` | Path traversal attempt | Use a simple path like `chaz` |
| `identity.path cannot be an absolute path` | Absolute path not allowed | Use relative path |
| `git.author_name cannot be empty` | Missing git author | Set in config or git config |
| `ai.provider cannot be empty when ai.enabled is true` | AI enabled but no provider | Set provider or disable AI |

## Reset to Defaults

To regenerate the configuration with default values:

```bash
# Backup current config
mv ~/.config/gitsavvy/config.toml ~/.config/gitsavvy/config.toml.bak

# Reinitialize (will create new config with defaults)
gitsavvy init
```
