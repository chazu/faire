// Package runner provides workflow execution with dangerous command detection.
package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
)

// dangerousPatterns contains patterns for potentially dangerous commands.
var dangerousPatterns = []struct {
	pattern *regexp.Regexp
	name    string
	risk    string
}{
	{
		pattern: regexp.MustCompile(`(?i)\brm\s+(-rf|-r|-fr|\-recursive)\s+/`),
		name:    "Recursive delete",
		risk:    "Will delete all files in the target path",
	},
	{
		pattern: regexp.MustCompile(`(?i)\bdd\s+(if=|of=)/dev/`),
		name:    "Disk overwrite",
		risk:    "Will destroy all data on the target device",
	},
	{
		pattern: regexp.MustCompile(`(?i)\bmkfs\.`),
		name:    "Filesystem creation",
		risk:    "Will create a new filesystem (destroys existing data)",
	},
	{
		pattern: regexp.MustCompile(`(?i)\b:>\s*\S+`),
		name:    "File truncation",
		risk:    "Will truncate file to zero bytes",
	},
	{
		pattern: regexp.MustCompile(`(?i)\bshutdown\s+( -h\s+now|-P\s+0|now)`),
		name:    "Immediate shutdown",
		risk:    "Will shut down the system immediately",
	},
	{
		pattern: regexp.MustCompile(`(?i)\breboot\s+( -f|now)`),
		name:    "Immediate reboot",
		risk:    "Will reboot the system immediately",
	},
	{
		pattern: regexp.MustCompile(`(?i)\bgit\s+branch\s+-D`),
		name:    "Git branch deletion",
		risk:    "Will delete the specified git branch",
	},
	{
		pattern: regexp.MustCompile(`(?i)\bgit\s+push\s+--force`),
		name:    "Force git push",
		risk:    "May overwrite remote history",
	},
	{
		pattern: regexp.MustCompile(`(?i)chmod\s+-R\s+777`),
		name:    "World-writable permissions",
		risk:    "Sets all files to world-writable (security risk)",
	},
	{
		pattern: regexp.MustCompile(`(?i)chmod\s+000`),
		name:    "Remove all permissions",
		risk:    "Removes all permissions from files",
	},
	{
		pattern: regexp.MustCompile(`(?i)\bmv\s+.*~/`),
		name:    "Move to home",
		risk:    "Moving to home directory (might be unintended)",
	},
}

// CheckDangerous checks if a command is potentially dangerous.
// Returns the danger info if dangerous, nil otherwise.
func CheckDangerous(command string) *DangerInfo {
	// Trim leading/trailing whitespace
	cmd := strings.TrimSpace(command)

	// Check against dangerous patterns
	for _, p := range dangerousPatterns {
		if p.pattern.MatchString(cmd) {
			return &DangerInfo{
				Name:     p.name,
				Risk:     p.risk,
				Pattern:  p.pattern.String(),
				Command:  cmd,
			}
		}
	}

	return nil
}

// DangerInfo contains information about a dangerous command.
type DangerInfo struct {
	Name    string
	Risk    string
	Pattern string
	Command string
}

// Warning returns a formatted warning message.
func (d *DangerInfo) Warning() string {
	return fmt.Sprintf("⚠️  %s detected\n   Risk: %s\n   Command: %s", d.Name, d.Risk, d.Command)
}

// Confirm prompts the user to confirm execution.
func (d *DangerInfo) Confirm() (bool, error) {
	fmt.Println(d.Warning())
	fmt.Print("\nContinue? [y/N]: ")

	var response string
	_, _ = fmt.Scanln(&response)

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes", nil
}

// IsDangerous returns true if the command is potentially dangerous.
func (d *DangerInfo) IsDangerous() bool {
	return true
}

// DangerChecker provides dangerous command checking.
type DangerChecker struct {
	enabled bool
}

// NewDangerChecker creates a new danger checker.
func NewDangerChecker(enabled bool) *DangerChecker {
	return &DangerChecker{enabled: enabled}
}

// Check checks if a command is dangerous and returns warning info.
func (dc *DangerChecker) Check(command string) *DangerInfo {
	if !dc.enabled {
		return nil
	}
	return CheckDangerous(command)
}

// ShouldWarn returns true if warnings are enabled.
func (dc *DangerChecker) ShouldWarn() bool {
	return dc.enabled
}

// PromptForConfirmation prompts the user if a command is dangerous.
// Returns true if the user confirms, false otherwise.
func (dc *DangerChecker) PromptForConfirmation(command string, autoConfirm bool) (bool, error) {
	danger := dc.Check(command)
	if danger == nil {
		return true, nil // Not dangerous, auto-proceed
	}

	if autoConfirm {
		// Auto-confirm mode (--yes flag), skip prompt but show warning
		fmt.Fprintf(os.Stderr, "Warning: %s\n", danger.Warning())
		return true, nil
	}

	// Interactive mode, prompt for confirmation
	return danger.Confirm()
}

// ExecuteWithDangerCheck executes a command with danger checking.
// Deprecated: Use runner.Exec with ExecConfig instead.
func ExecuteWithDangerCheck(command string, checker *DangerChecker, autoConfirm bool) ExecResult {
	// Check for danger first
	danger := checker.Check(command)

	result := ExecResult{
		Command:   command,
		Dangerous: danger != nil,
		Danger:    danger,
	}

	// Prompt for confirmation if dangerous
	if danger != nil && !autoConfirm {
		confirmed, err := danger.Confirm()
		if err != nil {
			result.ExitCode = 1
			result.Success = false
			return result
		}
		if !confirmed {
			result.ExitCode = 13 // Canceled
			result.Success = false
			return result
		}
	}

	// Execute the command using the new Exec function
	execConfig := ExecConfig{
		Command:       command,
		DangerChecker: checker,
		AutoConfirm:   autoConfirm,
	}
	execResult := Exec(context.Background(), execConfig)

	return ExecResult{
		Command:   execResult.Command,
		ExitCode:  execResult.ExitCode,
		Success:   execResult.Success,
		Output:    execResult.Output,
		Dangerous: execResult.Dangerous,
		Danger:    execResult.Danger,
	}
}

// IsExitCode checks if an error is an exec.ExitError.
func IsExitCode(err error) bool {
	_, ok := err.(*exec.ExitError)
	return ok
}

// GetExitCode extracts the exit code from an error.
func GetExitCode(err error) int {
	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
		return 1
	}
	return 1
}
