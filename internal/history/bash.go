package history

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// BashParser implements Parser for bash history files.
type BashParser struct {
	// SkipCommands lists commands to skip during parsing.
	SkipCommands []string
}

// NewBashParser creates a new BashParser with default skip commands.
func NewBashParser() *BashParser {
	return &BashParser{
		SkipCommands: []string{
			"cd", "pushd", "popd", "dirs", "pwd",
			"ls", "la", "ll", "clear",
			"history", "exit", "logout",
			"jobs", "fg", "bg",
		},
	}
}

// Parse reads the bash history file at the given path and returns parsed history lines.
// Bash history format varies:
// - With HISTTIMEFORMAT: #timestamp followed by commands on subsequent lines
// - Without HISTTIMEFORMAT: just commands, one per line
//
// Example with timestamps:
//   #1616420000
//   ls -la
//   #1616420100
//   git status
//
// Example without timestamps:
//   ls -la
//   git status
func (p *BashParser) Parse(path string) ([]HistoryLine, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open bash history: %w", err)
	}
	defer file.Close()

	var lines []HistoryLine
	var currentTimestamp time.Time
	scanner := bufio.NewScanner(file)

	// Regex for timestamp line: #<unix_timestamp>
	timestampRegex := regexp.MustCompile(`^#(\d+)$`)

	for scanner.Scan() {
		rawLine := scanner.Text()
		line := strings.TrimRight(rawLine, " \t")

		// Skip empty lines
		if line == "" {
			continue
		}

		// Check for timestamp line
		if matches := timestampRegex.FindStringSubmatch(line); matches != nil {
			ts, err := strconv.ParseInt(matches[1], 10, 64)
			if err == nil {
				currentTimestamp = time.Unix(ts, 0)
			}
			continue
		}

		// If we have a command, add it
		if line != "" && !strings.HasPrefix(line, "#") {
			// Handle multi-line commands (continuation with \)
			for strings.HasSuffix(line, "\\") {
				// Remove backslash and any space before it (line continuation syntax)
				line = strings.TrimSuffix(line, "\\")
				line = strings.TrimRight(line, " \t")
				line += "\n"
				if scanner.Scan() {
					line += scanner.Text()
				}
			}

			line = strings.TrimSpace(line)

			// Skip built-ins and common non-useful commands
			if p.shouldSkipCommand(line) {
				continue
			}

			// Skip commands starting with space (HISTCONTROL=ignorespace)
			if strings.HasPrefix(line, " ") {
				continue
			}

			lines = append(lines, HistoryLine{
				Timestamp: currentTimestamp,
				Command:   line,
				Shell:     "bash",
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading bash history: %w", err)
	}

	return lines, nil
}

// DetectPath returns the default path to the bash history file.
func (p *BashParser) DetectPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Check common locations
	locations := []string{
		filepath.Join(home, ".bash_history"),
		filepath.Join(home, ".local/share/bash/history"),
	}

	for _, path := range locations {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}

	// Return default even if it doesn't exist
	return filepath.Join(home, ".bash_history"), nil
}

// shouldSkipCommand returns true if a command should be skipped.
func (p *BashParser) shouldSkipCommand(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return true
	}

	// Get first word of command
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return true
	}

	firstWord := fields[0]
	for _, skip := range p.SkipCommands {
		if firstWord == skip {
			return true
		}
	}

	return false
}

// ParseBash is a convenience function that creates a BashParser and parses the given path.
func ParseBash(path string) ([]HistoryLine, error) {
	parser := NewBashParser()
	return parser.Parse(path)
}
