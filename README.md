# svf

A Git-backed workflow automation tool, compatible with Savvy CLI but without any proprietary SaaS dependency. All workflows and metadata live in a Git repository of your choice.

## Features

- **Configuration management**: Initialize with `svf init` (TUI wizard or scriptable)
- **Workflow editing**: Create/edit workflows with `svf edit` (TUI editor)
- **Workflow listing**: List all workflows with `svf list`
- **Workflow viewing**: View workflow details with `svf view`
- **Shell history**: Pick commands from shell history with `svf record history`
- **Session recording**: Record shell sessions with `svf record`
- **Status**: Show repository and tool status with `svf status`
- **Sync**: Synchronize with remote Git repository with `svf sync`
- **Placeholder support**: Parameter substitution with `<param>` syntax
- **LLM-friendly**: All commands support `--no-tui` for automation

## Quick Start

```bash
# Initialize configuration
svf init

# Edit or create a workflow
svf edit

# List all workflows
svf list

# View a workflow
svf view my-workflow

# Sync with remote
svf sync
```

## Building

### Prerequisites

- Go 1.24 or later
- Make (optional, but recommended)

### Build from source

```bash
# Clone the repository
git clone https://github.com/chazu/svf.git
cd svf

# Build using Make
make build

# The binary will be created at ./bin/svf
```

### Build manually

```bash
go build -ldflags "-X main.Version=$(git describe --tags --always) -X main.Commit=$(git rev-parse --short HEAD) -X main.Date=$(date -u +'%Y-%m-%dT%H:%M:%SZ')" -o bin/svf ./cmd/svf
```

## Development

### Run tests

```bash
make test
```

### Format code

```bash
make fmt
```

### Run linter

```bash
make lint
```

### Run go vet

```bash
make vet
```

### Clean build artifacts

```bash
make clean
```

## Project Structure

```
.
├── cmd/
│   └── svf/               # Main entry point
├── internal/
│   ├── app/               # Root orchestrator
│   ├── cli/               # Cobra command definitions
│   ├── config/            # Config loading/validation
│   ├── gitrepo/           # Git operations
│   ├── workflows/         # Workflow model and store
│   ├── runner/            # Command execution
│   ├── history/           # Shell history parsers
│   ├── recorder/          # Subshell session capture
│   ├── placeholders/      # Parameter parsing
│   ├── tui/               # Bubble Tea UI components
│   └── errors/            # Error handling
├── testdata/              # Test fixtures
└── .beads/                # Issue tracking (bd)
```

## Configuration

The configuration file is stored at `~/.config/svf/config.toml`:

```toml
[repo]
path = "/path/to/workflows/repo"
remote = "origin"
branch = "main"
sync_strategy = "rebase"

[identity]
path = "username"          # Your identity path in the repo
mode = "pr"                 # "direct" or "pr"

[git]
author_name = "Your Name"
author_email = "you@example.com"

[workflows]
root = "workflows"
shared_root = "shared"
```

## LLM Automation

All commands support `--no-tui` for non-interactive usage:

```bash
# Non-TUI mode for automation
svf --no-tui list --format json
svf --no-tui view my-workflow --md

# Pipe workflow to edit command
echo "$WORKFLOW_YAML" | svf --no-tui edit
```

## Version

```bash
svf --version
```

## License

TBD
