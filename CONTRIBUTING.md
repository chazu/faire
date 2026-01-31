# Contributing to gitsavvy

Thank you for your interest in contributing to gitsavvy! This document provides guidelines for contributing to the project.

## Code of Conduct

Be respectful, inclusive, and constructive. We aim to maintain a welcoming environment for all contributors.

## Getting Started

### Prerequisites

- **Go 1.21 or later**
- **Git**
- **Make** (optional, but recommended)

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork:

```bash
git clone https://github.com/your-username/faire.git
cd faire
```

3. Add the upstream remote:

```bash
git remote add upstream https://github.com/chazu/faire.git
```

### Development Setup

```bash
# Install dependencies
go mod download

# Build the binary
make build

# Run tests
make test

# Install locally (optional)
sudo make install
```

## Project Structure

```
.
├── cmd/
│   └── gitsavvy/          # CLI entry point
├── internal/
│   ├── app/               # Application orchestration
│   ├── cli/               # Cobra command definitions
│   ├── config/            # Configuration management
│   ├── gitrepo/           # Git repository wrapper
│   ├── workflows/         # Workflow types and storage
│   ├── index/             # Search index
│   ├── runner/            # Command execution
│   ├── history/           # Shell history parsers
│   ├── recorder/          # Subshell session capture
│   ├── placeholders/      # Parameter parsing
│   ├── export/            # Format exporters
│   ├── ai/                # AI provider interface
│   ├── tui/               # Bubble Tea UI components
│   └── errors/            # Error types
├── testdata/              # Test fixtures
├── docs/                  # Documentation
└── Makefile               # Build targets
```

## Development Workflow

### 1. Create a Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/your-bug-fix
```

Branch naming conventions:
- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation changes
- `refactor/` - Code refactoring
- `test/` - Test improvements

### 2. Make Your Changes

#### Code Style

- Follow standard Go conventions (`gofmt`)
- Use meaningful variable and function names
- Add comments for exported functions and types
- Keep functions focused and concise

#### Formatting

```bash
# Format code
make fmt

# Or use gofmt directly
gofmt -w -s .
```

#### Linting

```bash
# Run linter
make lint

# Or use golangci-lint directly
golangci-lint run
```

### 3. Write Tests

We aim for high test coverage. Write tests for new functionality:

```go
func TestMyFunction(t *testing.T) {
    // Arrange
    input := "test"

    // Act
    result := MyFunction(input)

    // Assert
    if result != "expected" {
        t.Errorf("expected 'expected', got '%s'", result)
    }
}
```

#### Run Tests

```bash
# Run all tests
make test

# Run tests for specific package
go test ./internal/config/...

# Run with coverage
go test -cover ./...

# Run with race detection
go test -race ./...
```

### 4. Commit Your Changes

Use clear, descriptive commit messages:

```
feat: Add workflow export to JSON format

Add JSON export functionality to the export command.
Includes tests for the new format.

Closes #123
```

Commit message prefixes:
- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation change
- `refactor:` - Code refactoring
- `test:` - Adding or updating tests
- `chore:` - Maintenance tasks

```bash
# Stage changes
git add .

# Commit
git commit -m "feat: description of changes"
```

### 5. Sync with Upstream

Before creating a pull request, sync with the upstream repository:

```bash
# Fetch upstream changes
git fetch upstream

# Rebase your branch on upstream/main
git rebase upstream/main

# Push to your fork
git push origin feature/your-feature-name
```

### 6. Create a Pull Request

1. Go to your fork on GitHub
2. Click "New Pull Request"
3. Provide a clear description of your changes
4. Link any related issues
5. Wait for review

## Pull Request Guidelines

### Title

Use a clear title with the appropriate prefix:

```
feat: Add support for PowerShell in workflow steps
fix: Correct placeholder validation regex
docs: Update configuration reference
```

### Description

Include:
- **What** you changed and why
- **How** you tested your changes
- **Screenshots** for UI changes (if applicable)
- **Breaking changes** (if any)
- **Related issues** using `Closes #123` syntax

### Checklist

- [ ] Tests pass (`make test`)
- [ ] Code is formatted (`make fmt`)
- [ ] Linter passes (`make lint`)
- [ ] Documentation updated (if needed)
- [ ] Commit messages follow conventions

## Architecture Guidelines

### Package Organization

- **`internal/`** - Private implementation, not exported
- **`pkg/`** - Public API (if we ever have one)
- Each package should have a single responsibility

### Error Handling

Use the `errors` package for consistent error types:

```go
import "github.com/chazuruo/faire/internal/errors"

return errors.New("something went wrong")
return errors.Wrap(err, "context")
```

### Configuration

Configuration belongs in `internal/config/`. Add new options to the `Config` struct and update validation.

### Adding Commands

1. Create command file in `internal/cli/`
2. Register in `cmd/gitsavvy/main.go`
3. Add tests in `internal/cli/<name>_test.go`
4. Update documentation

### Adding TUI Components

TUI components use Bubble Tea and live in `internal/tui/`:

1. Create a new model implementing `tea.Model`
2. Define `Init`, `Update`, and `View` methods
3. Add tests for the model
4. Wire up in the appropriate command

### AI Provider Integration

AI providers implement a common interface in `internal/ai/`:

1. Implement the provider interface
2. Add configuration options
3. Register the provider
4. Add tests with mocked responses

## Testing Guidelines

### Unit Tests

- Test functions in isolation
- Use table-driven tests for multiple cases
- Mock external dependencies (Git, HTTP, etc.)

Example:

```go
func TestValidateConfig(t *testing.T) {
    tests := []struct {
        name    string
        config  *Config
        wantErr bool
    }{
        {"valid config", &Config{...}, false},
        {"empty path", &Config{Path: ""}, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Integration Tests

Place integration tests in `testdata/` or use build tags:

```go
//go:build integration
// +build integration

func TestGitIntegration(t *testing.T) {
    // Tests that require actual Git operations
}
```

### Test Utilities

Use utilities from `internal/testutil/` for common test setup.

## Documentation

### Code Documentation

Add godoc comments for exported types and functions:

```go
// Workflow represents a runnable workflow definition.
// It contains metadata, placeholders, and execution steps.
type Workflow struct {
    // Title is the human-readable name of the workflow.
    Title string
    // ...
}
```

### User Documentation

Update relevant files in `docs/` when changing features:
- `docs/workflows.md` - Workflow-related changes
- `docs/configuration.md` - Config changes
- `docs/automation.md` - CI/CD changes
- `README.md` - High-level changes

### Examples

Add example workflows to `testdata/` or `examples/` for new features.

## Release Process

Maintainers handle releases. The general process:

1. Update version in `cmd/gitsavvy/main.go`
2. Update CHANGELOG.md
3. Create Git tag
4. Build release binaries
5. Create GitHub release
6. Publish to package registry (if applicable)

## Getting Help

- **GitHub Issues** - Bug reports and feature requests
- **Discussions** - Questions and ideas
- **Existing Code** - Look at similar functionality

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

## Recognition

Contributors are recognized in the CONTRIBUTORS.md file. Thank you for your contributions!
