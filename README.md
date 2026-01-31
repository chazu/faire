# svf

Git-backed workflow automation tool. Compatible with Savvy CLI workflows but stores all data in your own Git repository—no proprietary SaaS required.

## Features

- **Record workflows** from shell history or interactive subshell sessions
- **Run workflows** interactively with step-by-step confirmation
- **Parameter support** with placeholders like `<param>` for dynamic values
- **Sync via Git**—standard clone, pull, push, and PR workflows
- **AI-assisted workflow generation** (opt-in, provider-agnostic)
- **Export workflows** to Markdown, YAML, or JSON
- **TUI editor** for creating and editing workflows with Bubble Tea
- **Team collaboration** through Git branching and code review
- **Offline-first**—everything works locally without network access

## Installation

### From Source

```bash
git clone https://github.com/chazu/faire.git
cd faire
make build

# The binary will be created at ./bin/svf
sudo make install  # Optional: install to /usr/local/bin
```

### Build Options

```bash
# Standard build
make build

# Build for specific platform
GOOS=linux GOARCH=amd64 make build

# Build with version info
make build VERSION=v1.0.0
```

## Quick Start

### 1. Initialize

```bash
svf init
```

This will:
- Create a configuration file at `~/.config/svf/config.toml`
- Prompt you to select or create a Git repository for workflows
- Set up your identity path (e.g., `chaz` or `platform/chaz`)

### 2. Create Your First Workflow

From shell history:
```bash
svf record history
```

Or record a live session:
```bash
svf record
# Execute commands in the subshell
exit
# Edit and save the workflow
```

### 3. Run a Workflow

```bash
svf run <workflow-name>
```

### 4. Sync with Your Team

```bash
cd ~/.local/share/svf/repo
git pull
```

Or use the built-in sync (coming soon):
```bash
svf sync
```

## Commands

| Command | Description |
|---------|-------------|
| `init` | Initialize configuration and workflow repository |
| `whoami` | Show current identity and configuration |
| `record` | Start an interactive subshell to capture commands |
| `record history` | Create workflow from shell history |
| `run <workflow>` | Execute a workflow interactively |
| `list` | List available workflows |
| `edit <workflow>` | Open workflow in TUI editor |
| `export <workflow>` | Export workflow to Markdown/YAML/JSON |
| `sync` | Pull/push workflow repository changes |

## Configuration

The configuration file is located at `~/.config/svf/config.toml`.

Key configuration options:

```toml
[repo]
path = "~/.local/share/svf/repo"  # Workflow repository location
remote = "origin"                       # Git remote name
branch = "main"                         # Default branch

[identity]
path = "chaz"                           # Your claimed path (write target)
mode = "pr"                             # "direct" or "pr" for contributions

[workflows]
root = "workflows"                      # Path to user workflows
shared_root = "shared"                  # Path to shared workflows

[runner]
default_shell = "zsh"                   # Default shell for steps
confirm_each_step = true                # Prompt before each step
```

See [configuration.md](docs/configuration.md) for complete reference.

## Workflow Example

A workflow is defined as YAML:

```yaml
schema_version: 1
title: "Deploy to Production"
description: "Deploy application after running tests"
tags: ["deployment", "production"]

defaults:
  shell: "bash"
  confirm_each_step: true

placeholders:
  environment:
    prompt: "Which environment?"
    default: "staging"
    validate: "^(staging|production)$"

steps:
  - name: "Run tests"
    command: "go test ./..."

  - name: "Build docker image"
    command: "docker build -t myapp:<environment> ."

  - name: "Deploy"
    command: "kubectl apply -f deployment.yaml"
    confirmation: "Ready to deploy to <environment>?"
```

## Documentation

- [Workflows Guide](docs/workflows.md) - Creating and managing workflows
- [Configuration Reference](docs/configuration.md) - All config options
- [CI/CD Integration](docs/automation.md) - Using svf in automation
- [Contributing](CONTRIBUTING.md) - Development setup and contribution guidelines

## Project Structure

```
.
├── cmd/
│   └── svf/               # Main entry point
├── internal/
│   ├── app/               # Root orchestrator
│   ├── cli/               # Cobra command definitions
│   ├── config/            # Config loading/validation
│   ├── gitrepo/           # Git operations wrapper
│   ├── workflows/         # Workflow model and store
│   ├── index/             # Search index
│   ├── runner/            # Command execution engine
│   ├── history/           # Shell history parsers
│   ├── recorder/          # Subshell session capture
│   ├── placeholders/      # Parameter parsing and substitution
│   ├── export/            # Markdown/YAML/JSON export
│   ├── ai/                # AI provider interface
│   └── tui/               # Bubble Tea UI components
├── testdata/              # Test fixtures
├── docs/                  # Documentation
└── Makefile               # Build targets
```

## Development

### Run Tests

```bash
make test
```

### Format Code

```bash
make fmt
```

### Run Linter

```bash
make lint
```

### Clean Build Artifacts

```bash
make clean
```

## Version

```bash
svf --version
```

## License

MIT License - see [LICENSE](LICENSE) file.

## Acknowledgments

- Inspired by [Savvy CLI](https://github.com/getsavvyinc/savvy-cli)
- Built with [Cobra](https://github.com/spf13/cobra) and [Bubble Tea](https://github.com/charmbracelet/bubbletea)
