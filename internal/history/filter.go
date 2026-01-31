package history

import (
	"strings"
	"time"
)

// FilterLines filters history lines based on the given options
func FilterLines(lines []HistoryLine, opts FilterOptions) []HistoryLine {
	var result []HistoryLine
	seenCommands := make(map[string]bool)

	// Iterate in reverse (newest first) and then reverse again
	// This ensures we get the most recent commands when limiting
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]

		// Apply time filter
		if !opts.Since.IsZero() && line.Timestamp.Before(opts.Since) {
			continue
		}

		// Apply duplicate filter (all duplicates, not just consecutive)
		if opts.RemoveDup {
			if seenCommands[line.Command] {
				continue
			}
			seenCommands[line.Command] = true
		}

		// Filter out common built-ins and commands we don't want
		if isUnwantedCommand(line.Command) {
			continue
		}

		result = append(result, line)

		// Apply max lines limit
		if opts.MaxLines > 0 && len(result) >= opts.MaxLines {
			break
		}
	}

	// Reverse back to chronological order
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// isUnwantedCommand returns true if the command should be filtered out
func isUnwantedCommand(cmd string) bool {
	// Skip commands starting with space (HISTCONTROL=ignorespace)
	// Check BEFORE trimming
	if len(cmd) > 0 && cmd[0] == ' ' {
		return true
	}

	// Trim whitespace for further checks
	cmd = strings.TrimSpace(cmd)

	// Skip empty commands
	if cmd == "" {
		return true
	}

	// Skip common built-ins and simple commands we don't want in workflows
	unwantedPrefixes := []string{
		"ls", "ll", "la",
		"cd", "pwd",
		"clear", "cls",
		"exit",
		"history",
		"echo",
		"true", "false",
		":", // no-op in bash
	}

	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return true
	}

	firstWord := fields[0]
	for _, unwanted := range unwantedPrefixes {
		if firstWord == unwanted {
			return true
		}
	}

	return false
}

// FilterByLimit returns the last n commands from history
func FilterByLimit(lines []HistoryLine, limit int) []HistoryLine {
	if limit <= 0 || limit >= len(lines) {
		return lines
	}
	return lines[len(lines)-limit:]
}

// FilterBySince returns commands executed after the given time
func FilterBySince(lines []HistoryLine, since time.Time) []HistoryLine {
	var result []HistoryLine
	for _, line := range lines {
		if !line.Timestamp.IsZero() && line.Timestamp.After(since) {
			result = append(result, line)
		}
	}
	return result
}

// RemoveConsecutiveDuplicates removes consecutive duplicate commands
func RemoveConsecutiveDuplicates(lines []HistoryLine) []HistoryLine {
	if len(lines) == 0 {
		return lines
	}

	result := []HistoryLine{lines[0]}
	for i := 1; i < len(lines); i++ {
		if lines[i].Command != lines[i-1].Command {
			result = append(result, lines[i])
		}
	}
	return result
}
