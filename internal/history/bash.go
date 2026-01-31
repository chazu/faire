package history

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// BashParser implements Parser for bash history
type BashParser struct{}

// Parse parses bash history from the given path
func (bp *BashParser) Parse(path string) ([]HistoryLine, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open bash history: %w", err)
	}
	defer file.Close()

	var lines []HistoryLine
	scanner := bufio.NewScanner(file)

	var currentTimestamp time.Time
	timestampPattern := regexp.MustCompile(`^#(\d+)$`)

	for scanner.Scan() {
		line := scanner.Text()

		// Check for timestamp line (#<timestamp>)
		if matches := timestampPattern.FindStringSubmatch(line); matches != nil {
			ts, err := strconv.ParseInt(matches[1], 10, 64)
			if err == nil {
				currentTimestamp = time.Unix(ts, 0)
			}
			continue
		}

		// Skip empty lines and lines starting with space (HISTCONTROL)
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// If no timestamp was set, use a zero time
		if currentTimestamp.IsZero() {
			currentTimestamp = time.Time{}
		}

		lines = append(lines, HistoryLine{
			Timestamp: currentTimestamp,
			Command:   line,
			Shell:     "bash",
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading bash history: %w", err)
	}

	return lines, nil
}

// DetectPath finds the bash history file
func (bp *BashParser) DetectPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine home directory: %w", err)
	}

	// Common bash history locations
	candidates := []string{
		filepath.Join(homeDir, ".bash_history"),
		filepath.Join(homeDir, ".local", "share", "bash", "history"),
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	// Check for bash-specific history on different platforms
	if runtime.GOOS == "darwin" {
		// macOS often stores in ~/Library
		macPath := filepath.Join(homeDir, "Library", "bash", "history")
		if info, err := os.Stat(macPath); err == nil && !info.IsDir() {
			return macPath, nil
		}
	}

	return "", fmt.Errorf("bash history file not found (tried: %s)", strings.Join(candidates, ", "))
}
