package history

import (
	"os"
	"path/filepath"
	"strings"
)

// FilterOptions contains options for filtering history lines.
type FilterOptions struct {
	// RemoveDuplicates removes consecutive duplicate commands.
	RemoveDuplicates bool

	// RemoveEmpty removes empty commands.
	RemoveEmpty bool

	// SkipCommands is a list of commands to skip.
	SkipCommands []string

	// SkipBuiltins skips common built-in commands.
	SkipBuiltins bool

	// MinLength is the minimum command length to keep.
	MinLength int
}

// DefaultFilterOptions returns default filter options.
func DefaultFilterOptions() FilterOptions {
	return FilterOptions{
		RemoveDuplicates: true,
		RemoveEmpty:      true,
		SkipBuiltins:     true,
		SkipCommands:     []string{},
		MinLength:        0,
	}
}

// FilterLines filters history lines based on the given options.
func FilterLines(lines []HistoryLine, opts FilterOptions) []HistoryLine {
	if !opts.RemoveEmpty && len(opts.SkipCommands) == 0 && !opts.RemoveDuplicates && opts.MinLength == 0 && !opts.SkipBuiltins {
		return lines
	}

	var result []HistoryLine
	var lastCommand string

	for _, line := range lines {
		cmd := strings.TrimSpace(line.Command)

		// Remove empty lines
		if opts.RemoveEmpty && cmd == "" {
			continue
		}

		// Remove duplicates (consecutive)
		if opts.RemoveDuplicates && cmd == lastCommand {
			continue
		}

		// Check min length
		if opts.MinLength > 0 && len(cmd) < opts.MinLength {
			continue
		}

		// Skip specific commands
		skip := false
		for _, skipCmd := range opts.SkipCommands {
			if strings.HasPrefix(cmd, skipCmd) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// Skip built-ins
		if opts.SkipBuiltins && isBuiltin(cmd) {
			continue
		}

		result = append(result, line)
		lastCommand = cmd
	}

	return result
}

// isBuiltin returns true if the command is a shell builtin.
func isBuiltin(cmd string) bool {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return false
	}

	builtins := []string{
		"cd", "pushd", "popd", "dirs", "pwd",
		"ls", "la", "ll", "clear",
		"history", "exit", "logout",
		"jobs", "fg", "bg", "kill",
		"echo", "printf", "test", "[[", "[",
		"true", "false", "type", "which",
		"alias", "unalias", "export", "unset",
		"read", "readonly", "shift", "set",
	}

	for _, b := range builtins {
		if fields[0] == b {
			return true
		}
	}

	return false
}

// DetectShell attempts to detect the user's current shell from environment.
func DetectShell() string {
	// Check SHELL environment variable
	if shell := os.Getenv("SHELL"); shell != "" {
		return filepath.Base(shell)
	}

	// Default to bash
	return "bash"
}

// NewParser creates a Parser for the given shell type.
// Returns nil if the shell is not supported.
func NewParser(shell string) Parser {
	switch shell {
	case "bash":
		return NewBashParser()
	case "zsh":
		return NewZshParser()
	default:
		return nil
	}
}
