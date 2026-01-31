// Package history provides shell history parsing for various shells.
package history

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Command represents a single command from shell history.
type Command struct {
	Timestamp int64     `json:"timestamp"`
	CWD       string    `json:"cwd,omitempty"`
	Command   string    `json:"command"`
	Shell     string    `json:"shell"`
}

// Parser parses shell history files.
type Parser struct {
	shell string
	limit int
}

// NewParser creates a new history parser for the given shell.
func NewParser(shell string, limit int) *Parser {
	if limit <= 0 {
		limit = 500 // default limit
	}
	return &Parser{
		shell: shell,
		limit: limit,
	}
}

// Parse parses the history file for the configured shell.
func (p *Parser) Parse() ([]Command, error) {
	switch p.shell {
	case "bash":
		return p.parseBash()
	case "zsh":
		return p.parseZsh()
	default:
		return nil, fmt.Errorf("unsupported shell: %s (supported: bash, zsh)", p.shell)
	}
}

// parseBash parses bash history files.
// Bash history format: #timestamp followed by commands on subsequent lines.
// Example:
//   #1616420000
//   ls -la
//   #1616420100
//   git status
func (p *Parser) parseBash() ([]Command, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	histPath := filepath.Join(home, ".bash_history")
	file, err := os.Open(histPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open bash history: %w", err)
	}
	defer func() { _ = file.Close() }()

	var commands []Command
	var currentTimestamp int64
	scanner := bufio.NewScanner(file)

	timestampRegex := regexp.MustCompile(`^#(\d+)$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// Check for timestamp line
		if matches := timestampRegex.FindStringSubmatch(line); matches != nil {
			var err error
			currentTimestamp, err = parseTimestamp(matches[1])
			if err != nil {
				continue
			}
			continue
		}

		// If we have a command, add it
		if line != "" && !strings.HasPrefix(line, "#") {
			// Skip built-ins and common non-useful commands
			if p.shouldSkipCommand(line) {
				continue
			}

			commands = append(commands, Command{
				Timestamp: currentTimestamp,
				Command:   line,
				Shell:     "bash",
			})

			// Check limit
			if len(commands) >= p.limit {
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading bash history: %w", err)
	}

	return commands, nil
}

// parseZsh parses zsh history files.
// Zsh history format: :timestamp:duration;command
// Example:
//   :1616420000:0;ls -la
//   :1616420100:0;git status
func (p *Parser) parseZsh() ([]Command, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	histPath := filepath.Join(home, ".zsh_history")
	file, err := os.Open(histPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zsh history: %w", err)
	}
	defer func() { _ = file.Close() }()

	var commands []Command
	scanner := bufio.NewScanner(file)

	// Zsh history format: :timestamp:duration;command
	// But can also have multi-line commands with backslash continuation
	zshRegex := regexp.MustCompile(`^:(\d+):(\d+);(.*)`)

	var currentCmd strings.Builder
	var currentTimestamp int64

	for scanner.Scan() {
		line := scanner.Text()

		// Check if this is a new history entry
		if matches := zshRegex.FindStringSubmatch(line); matches != nil {
			// Save previous command if exists
			if currentCmd.Len() > 0 {
				cmd := strings.TrimSpace(currentCmd.String())
				if cmd != "" && !p.shouldSkipCommand(cmd) {
					commands = append(commands, Command{
						Timestamp: currentTimestamp,
						Command:   cmd,
						Shell:     "zsh",
					})
					if len(commands) >= p.limit {
						break
					}
				}
				currentCmd.Reset()
			}

			// Parse new entry
			currentTimestamp, _ = parseTimestamp(matches[1])
			currentCmd.WriteString(matches[3])
		} else if currentCmd.Len() > 0 {
			// Continuation of previous command (multi-line)
			currentCmd.WriteString("\n")
			currentCmd.WriteString(line)
		}
	}

	// Don't forget the last command
	if currentCmd.Len() > 0 && len(commands) < p.limit {
		cmd := strings.TrimSpace(currentCmd.String())
		if cmd != "" && !p.shouldSkipCommand(cmd) {
			commands = append(commands, Command{
				Timestamp: currentTimestamp,
				Command:   cmd,
				Shell:     "zsh",
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading zsh history: %w", err)
	}

	return commands, nil
}

// shouldSkipCommand returns true if a command should be skipped.
func (p *Parser) shouldSkipCommand(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return true
	}

	// Skip built-ins and navigation commands
	skipCommands := []string{
		"cd", "pushd", "popd", "dirs", "pwd",
		"ls", "la", "ll", "clear",
		"history", "exit", "logout",
		"jobs", "fg", "bg",
	}

	// Check if command starts with any skip command
	firstWord := strings.Fields(cmd)
	if len(firstWord) > 0 {
		for _, skip := range skipCommands {
			if firstWord[0] == skip {
				return true
			}
		}
	}

	return false
}

// parseTimestamp parses a unix timestamp string.
func parseTimestamp(s string) (int64, error) {
	var ts int64
	_, err := fmt.Sscanf(s, "%d", &ts)
	return ts, err
}

// ParseSince returns commands since the given duration.
func (p *Parser) ParseSince(since time.Duration) ([]Command, error) {
	commands, err := p.Parse()
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-since).Unix()

	var result []Command
	for _, cmd := range commands {
		if cmd.Timestamp >= cutoff {
			result = append(result, cmd)
		}
	}

	return result, nil
}

// DetectShell attempts to detect the user's current shell.
func DetectShell() string {
	// Check SHELL environment variable
	if shell := os.Getenv("SHELL"); shell != "" {
		return filepath.Base(shell)
	}

	// Default to bash
	return "bash"
}
