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

// ZshParser implements Parser for zsh history
type ZshParser struct{}

// Parse parses zsh extended history from the given path
func (zp *ZshParser) Parse(path string) ([]HistoryLine, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open zsh history: %w", err)
	}
	defer file.Close()

	var lines []HistoryLine
	scanner := bufio.NewScanner(file)

	// Zsh extended history format: :<timestamp>:<elapsed>:<command>
	// Or simple format: just the command
	extendedHistoryPattern := regexp.MustCompile(`^:(\d+):(\d+);(.+)`)
	var currentTimestamp time.Time

	for scanner.Scan() {
		line := scanner.Text()

		// Try to match extended history format
		if matches := extendedHistoryPattern.FindStringSubmatch(line); matches != nil {
			ts, err := strconv.ParseInt(matches[1], 10, 64)
			if err == nil {
				currentTimestamp = time.Unix(ts, 0)
			}
			command := strings.TrimSpace(matches[3])

			if command != "" && !strings.HasPrefix(command, "#") {
				lines = append(lines, HistoryLine{
					Timestamp: currentTimestamp,
					Command:   command,
					Shell:     "zsh",
				})
			}
			continue
		}

		// Simple format (no timestamp)
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		lines = append(lines, HistoryLine{
			Timestamp: time.Time{}, // No timestamp available
			Command:   line,
			Shell:     "zsh",
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading zsh history: %w", err)
	}

	return lines, nil
}

// DetectPath finds the zsh history file
func (zp *ZshParser) DetectPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine home directory: %w", err)
	}

	// Common zsh history locations
	candidates := []string{
		filepath.Join(homeDir, ".zhistory"),
		filepath.Join(homeDir, ".zsh_history"),
		filepath.Join(homeDir, ".histfile"),
		filepath.Join(homeDir, ".history"),
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("zsh history file not found (tried: %s)", strings.Join(candidates, ", "))
}
