# svf User Guide

`svf` is a terminal-first workflow automation tool that stores all workflows and metadata in a Git repository. It's compatible with Savvy CLI workflows but without any proprietary SaaS dependency.

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Workflow Format](#workflow-format)
- [Commands](#commands)
  - [init](#init-initialize-configuration)
  - [edit](#edit-create-or-edit-workflows)
  - [list](#list-workflows)
  - [view](#view-workflow-details)
  - [run](#run-workflows)
  - [search](#search-workflows)
  - [record](#record-shell-sessions)
  - [history](#pick-commands-from-shell-history)
  - [ask](#generate-workflows-using-ai)
  - [sync](#sync-with-remote)
  - [export](#export-workflows)
  - [status](#show-status)
  - [whoami](#show-identity)
- [Placeholders](#placeholders)
- [TUI Keybindings](#tui-keybindings)

---

## Installation

```bash
# Clone and build
git clone https://github.com/chazu/svf.git
cd svf
make build

# Add to PATH
export PATH="$PATH:$(pwd)/bin"
```

---

## Quick Start

```bash
# 1. Initialize svf (interactive TUI wizard)
svf init

# 2. Create your first workflow
svf edit

# 3. List all workflows
svf list

# 4. Run a workflow
svf run my-workflow

# 5. Sync with remote
svf sync
```

---

## Configuration

### Initial Setup

Run `svf init` to launch the interactive setup wizard:

```bash
svf init
```

The wizard will prompt you for:
1. **Repository source**: Clone from remote URL or use existing local directory
2. **Repository path**: Where to store the git repo (default: `~/.svf/repo`)
3. **Identity path**: Your path within the repo (e.g., `chaz` or `platform/chaz`)
4. **Write mode**: `direct` (commit to main) or `pr` (create pull requests)
5. **Git author details**: Name and email for commits
6. **Sign commits**: Whether to GPG sign commits

### Non-Interactive Setup

```bash
svf init --local "$HOME/my-workflows" \
         --identity "$(whoami)" \
         --mode direct \
         --author-name "Your Name" \
         --author-email "you@example.com"
```

### Config File Location

The config is stored at `~/.config/svf/config.toml`:

```toml
[repo]
  path = "/Users/chazu/.svf/repo"    # Auto-expanded from ~
  remote = "origin"
  branch = "main"
  sync_strategy = "rebase"

[identity]
  path = "default/chazu"              # Your workflows path
  mode = "direct"                     # or "pr"

[workflows]
  root = "workflows"                  # Where user workflows go
  shared_root = "shared"              # Shared workflows
  draft_root = "drafts"               # Draft workflows
  index_path = ".svf/index.json"     # Search index
```

---

## Workflow Format

Workflows are stored as YAML files at:
```
workflows/<identity>/<slug>/workflow.yaml
```

Example workflow:

```yaml
schema_version: 1
id: wf_01h4x9y2z3a4b
title: "Deploy to Production"
description: "Build and deploy the application"
tags: [deploy, production]
placeholders:
  - name: environment
    prompt: "Which environment?"
    default: "staging"
    validate: "^(staging|production)$"
steps:
  - name: "Run tests"
    command: "go test ./..."
    shell: "bash"
    cwd: "/app"
  - name: "Build docker image"
    command: "docker build -t myapp:${environment} ."
    depends_on:
      - "Run tests"
  - name: "Deploy"
    command: "kubectl rollout restart deployment/myapp"
    dangerous: true
```

### Workflow Fields

| Field | Type | Description |
|-------|------|-------------|
| `schema_version` | int | Format version (always `1`) |
| `id` | string | Unique identifier (ULID) |
| `title` | string | Human-readable name |
| `description` | string | Detailed description |
| `tags` | []string | Tags for searching/filtering |
| `placeholders` | []Placeholder | Parameters to prompt for |
| `steps` | []Step | Workflow steps |

### Step Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Step name |
| `command` | string | Shell command to execute |
| `shell` | string | Shell: `bash`, `zsh`, `sh`, `pwsh` |
| `cwd` | string | Working directory |
| `env` | map[string]string | Environment variables |
| `continue_on_error` | bool | Continue if this step fails |
| `dangerous` | bool | Mark as dangerous command |

---

## Commands

### init: Initialize Configuration

Start the interactive TUI wizard:

```bash
svf init
```

Or use non-interactive flags:

```bash
svf init --local ~/my-workflows \
         --identity "$(whoami)" \
         --mode direct
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--local PATH` | Use existing local repository |
| `--remote URL` | Clone from remote URL |
| `--branch NAME` | Set default branch |
| `--identity PATH` | Your identity path |
| `--MODE MODE` | Write mode: `direct` or `pr` |
| `--author-name NAME` | Git author name |
| `--author-email EMAIL` | Git author email |
| `--sign` | Enable commit signing |
| `--no-commit` | Skip git commit after init |

---

### edit: Create or Edit Workflows

Launch the TUI editor:

```bash
svf edit                          # Create new workflow
svf edit --workflow my-workflow   # Edit existing workflow
```

**TUI Features:**
- Create/edit workflows with full-screen editor
- Add/remove/reorder steps
- Configure placeholders with validation
- Auto-generates YAML on save
- Automatic git commit (unless `--no-commit`)

**Non-TUI Mode** (import from YAML):

```bash
svf edit --no-tui --file workflow.yaml
svf edit --no-tui --file workflow.yaml --output /path/save.yaml
cat workflow.yaml | svf edit --no-tui
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--workflow ID` | Edit specific workflow |
| `--file PATH` | Import YAML file (non-TUI) |
| `--output PATH` | Save to path (non-TUI) |
| `--no-commit` | Skip git commit |
| `--no-tui` | Disable TUI mode |

---

### list: List Workflows

```bash
svf list                    # List all workflows (table format)
svf list --mine             # Only your workflows
svf list --shared           # Only shared workflows
svf list --tag deploy       # Filter by tag
svf list --format json      # JSON output
```

**Output:**
```
ID              Title                   Tags        Author
wf_abc123       Deploy to prod          deploy      chazu
wf_def456       Run tests               test         chazu
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--mine` | Only show your workflows |
| `--shared` | Only show shared workflows |
| `--tag TAG` | Filter by tag (repeatable) |
| `--format FORMAT` | Output: `table`, `json`, `plain` |

---

### view: View Workflow Details

```bash
svf view my-workflow         # Formatted display
svf view my-workflow --raw   # Raw YAML
svf view my-workflow --md    # Markdown format
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--raw` | Print raw YAML |
| `--md` | Print Markdown |

---

### run: Run Workflows

**Interactive mode** (default):

```bash
svf run my-workflow
```

- Shows step list with status icons
- Prompts for placeholders once per unique value
- Press Enter to execute each step
- Keybindings: `s` (skip), `r` (rerun), `q` (quit), `e` (edit step)

**Non-interactive mode** (auto-confirm):

```bash
svf run my-workflow --yes --param env=staging
```

**Other modes:**

```bash
svf run my-workflow --dry-run               # Show commands, don't execute
svf run my-workflow --local                 # Skip git fetch
svf run my-workflow --from "Build"          # Start from specific step
svf run my-workflow --until "Deploy"        # Stop before specific step
```

**Exit codes:**
| Code | Meaning |
|------|---------|
| 0 | Success |
| 13 | User canceled |
| 20 | Step failed |
| 21 | Missing parameter |

**Flags:**
| Flag | Description |
|------|-------------|
| `--yes` | Non-interactive mode |
| `--param KEY=VAL` | Set placeholder value |
| `--dry-run` | Show commands without executing |
| `--local` | Skip git fetch |
| `--from STEP` | Start from step |
| `--until STEP` | Stop before step |
| `--cwd DIR` | Working directory override |
| `--env KEY=VAL` | Environment variables |
| `--log PATH` | Write run log to file |

---

### search: Search Workflows

**Interactive mode** (default TUI):

```bash
svf search deploy        # Fuzzy search with TUI
```

- Real-time fuzzy search
- Preview workflow details
- Filter by mine/shared/tags

**Non-interactive mode:**

```bash
svf search --query deploy --format json
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--query TEXT` | Search query (non-TUI) |
| `--mine` | Only your workflows |
| `--shared` | Only shared workflows |
| `--tag TAG` | Filter by tag |
| `--json` | JSON output |

---

### record: Record Shell Sessions

Start a recording session:

```bash
svf record                    # Start with auto-detected shell
svf record --shell zsh        # Use specific shell
```

1. A subshell launches with command capture enabled
2. Run your commands normally
3. Exit with `Ctrl+D` or `exit`
4. Workflow editor opens with captured commands
5. Review, edit, and save

**Flags:**
| Flag | Description |
|------|-------------|
| `--shell SHELL` | Shell: `bash` or `zsh` |
| `--title TITLE` | Workflow title |
| `--desc DESC` | Workflow description |
| `--tags TAGS` | Comma-separated tags |
| `--identity PATH` | Identity path override |
| `--draft` | Save as draft (no commit) |
| `--no-commit` | Skip git commit |

---

### history: Pick Commands from Shell History

```bash
svf history                  # Interactive TUI picker
svf history --since 1h       # Only last hour
svf history --limit 100      # Max 100 entries
```

1. Loads shell history (bash/zsh)
2. TUI picker with fuzzy search
3. Multi-select with Space
4. `a` (all), `n` (none), Enter (confirm)
5. Converts to workflow steps

**Flags:**
| Flag | Description |
|------|-------------|
| `--shell SHELL` | Shell to use |
| `--since DURATION` | Time limit (e.g., `1h`, `1d`) |
| `--limit NUM` | Max entries |
| `--title TITLE` | Workflow title |
| `--desc DESC` | Workflow description |
| `--tags TAGS` | Comma-separated tags |
| `--identity PATH` | Identity path override |
| `--draft` | Save as draft |
| `--no-commit` | Skip git commit |

---

### ask: Generate Workflows Using AI

```bash
svf ask                      # Interactive TUI
svf ask --prompt "Deploy my app to staging"
```

1. Enter your goal in natural language
2. Redaction UI reviews sensitive data
3. AI generates workflow or step
4. Review in workflow editor
5. Save to repository

**Flags:**
| Flag | Description |
|------|-------------|
| `--prompt TEXT` | Natural language prompt |
| `--provider NAME` | AI provider (openai, ollama, etc.) |
| `--model NAME` | Model name |
| `--api-key-env VAR` | Env var for API key |
| `--as FORMAT` | `workflow` or `step` |
| `--identity PATH` | Identity path |
| `--json` | JSON output |
| `--no-commit` | Skip git commit |

---

### sync: Sync with Remote

```bash
svf sync                     # Fetch and rebase
svf sync --strategy merge    # Use merge instead
svf sync --push              # Push after sync
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--strategy STRAT` | Integration: `ff-only`, `rebase`, `merge` |
| `--remote NAME` | Remote name |
| `--branch NAME` | Branch name |
| `--push` | Push after sync |
| `--no-push` | Skip push |
| `--reindex` | Force index rebuild |
| `--conflicts MODE` | Conflict resolution: `tui`, `ours`, `theirs`, `abort` |

---

### export: Export Workflows

```bash
svf export my-workflow                # Markdown to stdout
svf export my-workflow --format json  # JSON format
svf export my-workflow --out out.md   # Write to file
svf export my-workflow --update-readme # Update README.md
```

**Template locations:**
1. `.svf/templates/export.<format>` (repo-specific)
2. `~/.config/svf/templates/export.<format>` (user-specific)
3. Built-in templates

**Flags:**
| Flag | Description |
|------|-------------|
| `--format FMT` | Format: `md`, `yaml`, `json` |
| `--out PATH` | Output file |
| `--template PATH` | Custom template |
| `--update-readme` | Update README.md |

---

### status: Show Status

```bash
svf status                  # Plain text
svf status --json           # JSON format
```

**Shows:**
- Git status (clean/dirty, branch, ahead/behind)
- Repository path
- Identity path
- Last sync time
- Index freshness

---

### whoami: Show Identity

```bash
svf whoami                  # Plain text
svf whoami --json           # JSON format
```

**Shows:**
- Config file location
- Repository path
- Identity path
- Mode (direct/pr)
- Author details

---

## Placeholders

Placeholders allow you to parameterize workflows. Use `<param>` syntax in commands:

```yaml
title: "Deploy App"
placeholders:
  - name: environment
    prompt: "Which environment?"
    default: "staging"
    validate: "^(staging|production)$"
  - name: region
    prompt: "Which region?"
    default: "us-east-1"
steps:
  - name: "Deploy"
    command: "kubectl rollout restart deployment/myapp -n <environment> --region=<region>"
```

### Placeholder Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Parameter name |
| `prompt` | string | User prompt text |
| `default` | string | Default value |
| `validate` | string | Regex validation |
| `secret` | bool | Hide input (passwords) |

### Passing Parameters

**Interactive:**
```bash
svf run my-workflow
# Prompts for each placeholder
```

**Non-interactive:**
```bash
svf run my-workflow --yes --param environment=production --param region=us-west-2
```

---

## TUI Keybindings

### Global Keybindings

| Key | Action |
|-----|--------|
| `q` | Quit |
| `Ctrl+C` | Quit |
| `Esc` | Go back/Cancel |
| `Enter` | Confirm/Select |
| `?` | Toggle help |

### Edit Workflow Editor

| Key | Action |
|-----|--------|
| `Tab` | Switch panels |
| `↑`/`↓` | Navigate |
| `e` | Edit field |
| `a` | Add step |
| `d` | Delete step |
| `s` | Save |
| `Ctrl+S` | Save |

### Run Workflow

| Key | Action |
|-----|--------|
| `Enter` | Run step |
| `s` | Skip step |
| `r` | Rerun step |
| `q` | Quit |
| `e` | Edit step |
| `p` | Show placeholder values |
| `?` | Toggle help |

### Search/History Picker

| Key | Action |
|-----|--------|
| `↑`/`↓` or `j`/`k` | Navigate |
| `Space` | Toggle selection |
| `a` | Select all |
| `n` | Select none |
| `/` | Focus filter |
| `Enter` | Confirm |
| `Esc` | Cancel |

### Redaction UI

| Key | Action |
|-----|--------|
| `a` | Redact all |
| `r` | Redact selected |
| `e` | Edit selected |
| `u` | Undo all |
| `c` | Continue to confirmation |
| `q` | Quit |

---

## Directory Structure

```
~/.svf/repo/
├── .git/
├── .svf/
│   └── index.json          # Search index
├── workflows/
│   └── <identity>/         # Your workflows
│       └── <slug>/
│           └── workflow.yaml
├── shared/                 # Shared workflows
│   └── <identity>/
│       └── <slug>/
│           └── workflow.yaml
└── drafts/                 # Draft workflows
    └── <identity>/
        └── <slug>/
            └── workflow.yaml
```

---

## Tips and Tricks

1. **Use shell history to quickly create workflows:**
   ```bash
   svf history --since 1h
   ```

2. **Record a complex session:**
   ```bash
   svf record --title "Deploy Process"
   ```

3. **Search for anything:**
   ```bash
   svf search deploy --tag production
   ```

4. **Dry-run before executing:**
   ```bash
   svf run my-workflow --dry-run
   ```

5. **Export documentation:**
   ```bash
   svf export my-workflow --update-readme
   ```

6. **Non-TUI for scripting:**
   ```bash
   svf --no-tui list --format json | jq '.'
   ```

---

## Troubleshooting

### "Repository not initialized"

Run `svf init` to set up your configuration.

### "lstat ... no such file or directory"

Run `svf init` again - it will create the folder structure automatically.

### Placeholder prompts not working

Ensure placeholder names use `<name>` syntax in commands:
```yaml
command: "echo <param>"  # Correct
command: "echo $param"   # Incorrect - use <param>
```

### Git conflicts during sync

```bash
svf sync --conflicts tui    # Interactive resolver
svf sync --conflicts ours   # Always keep yours
svf sync --conflicts theirs # Always use theirs
```
