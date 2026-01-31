# git-savvy

A Git-backed workflow automation tool, compatible with Savvy CLI but without any proprietary SaaS dependency. All workflows and metadata live in a Git repository of your choice.

## Features

- Create workflows from terminal sessions (`record`) and shell history (`record history`)
- Run workflows interactively with auto-advancing steps
- Parameter support with placeholders (`<param>`)
- Sync via standard Git operations
- AI-assisted workflow generation (opt-in)
- Export workflows to Markdown

## Building

### Prerequisites

- Go 1.21 or later
- Make (optional, but recommended)

### Build from source

```bash
# Clone the repository
git clone https://github.com/chazu/faire.git
cd faire

# Build using Make
make build

# The binary will be created at ./bin/gitsavvy
```

### Build manually

```bash
go build -ldflags "-X main.Version=$(git describe --tags --always) -X main.Commit=$(git rev-parse --short HEAD) -X main.Date=$(date -u +'%Y-%m-%dT%H:%M:%SZ')" -o bin/gitsavvy ./cmd/gitsavvy
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
│   └── gitsavvy/          # Main entry point
├── internal/
│   ├── app/               # Root orchestrator
│   ├── cli/               # Cobra command definitions
│   ├── config/            # Config loading/validation
│   ├── gitrepo/           # Git operations
│   ├── workflows/         # Workflow model
│   ├── index/             # Search index
│   ├── runner/            # Command execution
│   ├── history/           # Shell history parsers
│   ├── recorder/          # Subshell session capture
│   ├── placeholders/      # Parameter parsing
│   ├── export/            # Markdown/YAML/JSON export
│   ├── ai/                # AI provider interface
│   └── tui/               # Bubble Tea UI components
├── pkg/
│   └── types/             # Public types (optional)
├── testdata/              # Test fixtures
├── configs/               # Example configs
└── docs/                  # Documentation
```

## Version

```
gitsavvy --version
```

## License

TBD
