// Package recorder provides shell session recording functionality.
package recorder

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// CapturedCommand represents a single command captured during a shell session.
type CapturedCommand struct {
	Timestamp int64
	CWD       string
	Command   string
}

// parseCaptureFile reads and parses the capture file, extracting unique commands.
func (r *Recorder) parseCaptureFile(path string) ([]CapturedCommand, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open capture file: %w", err)
	}
	defer file.Close()
	defer os.Remove(path) // Cleanup temp file

	var commands []CapturedCommand
	seen := make(map[string]bool)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}

		ts, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			continue
		}

		cwd := parts[1]
		cmd := parts[2]

		// Skip empty commands
		if cmd == "" {
			continue
		}

		// Deduplicate by cwd:command
		key := fmt.Sprintf("%s:%s", cwd, cmd)
		if seen[key] {
			continue
		}
		seen[key] = true

		commands = append(commands, CapturedCommand{
			Timestamp: ts,
			CWD:       cwd,
			Command:   cmd,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading capture file: %w", err)
	}

	if len(commands) == 0 {
		return nil, ErrNoCommandsCaptured
	}

	return commands, nil
}

// CommandsToWorkflow converts captured commands to workflow steps.
func CommandsToWorkflow(commands []CapturedCommand, title, desc string, shell string) []WorkflowStep {
	steps := make([]WorkflowStep, 0, len(commands))
	for _, cmd := range commands {
		steps = append(steps, WorkflowStep{
			Command: cmd.Command,
			CWD:     cmd.CWD,
			Shell:   shell,
		})
	}
	return steps
}

// WorkflowStep represents a step that can be added to a workflow.
type WorkflowStep struct {
	Command string
	CWD     string
	Shell   string
}
