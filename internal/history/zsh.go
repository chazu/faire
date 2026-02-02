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

// ZshParser implements Parser for zsh history files.
type ZshParser struct {
	// SkipCommands lists commands to skip during parsing.
	SkipCommands []string
}

// NewZshParser creates a new ZshParser with default skip commands.
func NewZshParser() *ZshParser {
	return &ZshParser{
		SkipCommands: []string{
			"cd", "pushd", "popd", "dirs", "pwd",
			"ls", "la", "ll", "clear",
			"history", "exit", "logout",
			"jobs", "fg", "bg",
		},
	}
}

// Parse reads the zsh history file at the given path and returns parsed history lines.
// Zsh extended history format: :timestamp:elapsed:command
//
// Example:
//   :1616420000:0:ls -la
//   :1616420100:1:git status
//
// Multi-line commands are handled with continuation:
//   :1616420200:0:echo "multi\
//   :line\
//   :command"
func (p *ZshParser) Parse(path string) ([]HistoryLine, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open zsh history: %w", err)
	}
	defer file.Close()

	var lines []HistoryLine
	scanner := bufio.NewScanner(file)

	// Zsh history format: :timestamp:elapsed:command
	// The regex captures timestamp and elapsed time (which we ignore)
	zshRegex := regexp.MustCompile(`^:(\d+):(\d+):(.*)`)

	var currentCmd strings.Builder
	var currentTimestamp time.Time
	var pendingBackslash bool

	for scanner.Scan() {
		line := scanner.Text()

		// Check for empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Check if this is a new history entry
		if matches := zshRegex.FindStringSubmatch(line); matches != nil {
			// Check if previous command ended with backslash (continuation)
			if pendingBackslash && currentCmd.Len() > 0 {
				// Remove backslash and any space before it (line continuation syntax)
				cmdSoFar := strings.TrimSuffix(currentCmd.String(), "\\")
				cmdSoFar = strings.TrimRight(cmdSoFar, " \t")
				currentCmd.Reset()
				currentCmd.WriteString(cmdSoFar)
				currentCmd.WriteString("\n")
				currentCmd.WriteString(matches[3])
				pendingBackslash = strings.HasSuffix(matches[3], "\\")
				continue
			}

			// Save previous command if exists
			if currentCmd.Len() > 0 {
				cmd := strings.TrimSpace(currentCmd.String())
				if cmd != "" && !p.shouldSkipCommand(cmd) {
					lines = append(lines, HistoryLine{
						Timestamp: currentTimestamp,
						Command:   cmd,
						Shell:     "zsh",
					})
				}
				currentCmd.Reset()
			}

			// Parse new entry
			ts, err := strconv.ParseInt(matches[1], 10, 64)
			if err == nil {
				currentTimestamp = time.Unix(ts, 0)
			} else {
				currentTimestamp = time.Time{}
			}
			currentCmd.WriteString(matches[3])
			pendingBackslash = strings.HasSuffix(matches[3], "\\")
		} else if currentCmd.Len() > 0 {
			// Continuation of previous command (multi-line)
			// This handles the case where lines don't have the timestamp prefix
			currentCmd.WriteString("\n")
			currentCmd.WriteString(line)
			pendingBackslash = strings.HasSuffix(line, "\\")
		}
	}

	// Don't forget the last command
	if currentCmd.Len() > 0 {
		cmd := strings.TrimSpace(currentCmd.String())
		if cmd != "" && !p.shouldSkipCommand(cmd) {
			lines = append(lines, HistoryLine{
				Timestamp: currentTimestamp,
				Command:   cmd,
				Shell:     "zsh",
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading zsh history: %w", err)
	}

	return lines, nil
}

// DetectPath returns the default path to the zsh history file.
func (p *ZshParser) DetectPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Check common locations
	locations := []string{
		filepath.Join(home, ".zsh_history"),
		filepath.Join(home, ".zhistory"),
		filepath.Join(home, ".histfile"),
	}

	for _, path := range locations {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}

	// Return default even if it doesn't exist
	return filepath.Join(home, ".zsh_history"), nil
}

// shouldSkipCommand returns true if a command should be skipped.
func (p *ZshParser) shouldSkipCommand(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return true
	}

	// Skip commands starting with space (HISTCONTROL=ignorespace)
	if strings.HasPrefix(cmd, " ") {
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

// ParseZsh is a convenience function that creates a ZshParser and parses the given path.
func ParseZsh(path string) ([]HistoryLine, error) {
	parser := NewZshParser()
	return parser.Parse(path)
}
