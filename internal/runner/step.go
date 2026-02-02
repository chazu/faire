package runner

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chazuruo/svf/internal/workflows"
)

// StepExecutor handles the execution of individual workflow steps.
type StepExecutor struct {
	shell          string
	cwd            string
	env            map[string]string
	confirmEach    bool
	streamOutput   bool
	dangerChecker  *DangerChecker
	autoConfirm    bool
}

// StepExecutorOption configures a StepExecutor.
type StepExecutorOption func(*StepExecutor)

// withStepShell sets the default shell for step execution.
func withStepShell(shell string) StepExecutorOption {
	return func(e *StepExecutor) {
		e.shell = shell
	}
}

// withStepCWD sets the default working directory for step execution.
func withStepCWD(cwd string) StepExecutorOption {
	return func(e *StepExecutor) {
		e.cwd = cwd
	}
}

// withStepEnv sets default environment variables for step execution.
func withStepEnv(env map[string]string) StepExecutorOption {
	return func(e *StepExecutor) {
		e.env = env
	}
}

// withStepConfirmEach enables confirmation prompts before each step.
func withStepConfirmEach(confirm bool) StepExecutorOption {
	return func(e *StepExecutor) {
		e.confirmEach = confirm
	}
}

// withStepStreamOutput enables output streaming during step execution.
func withStepStreamOutput(stream bool) StepExecutorOption {
	return func(e *StepExecutor) {
		e.streamOutput = stream
	}
}

// withStepDangerChecker sets the dangerous command checker.
func withStepDangerChecker(checker *DangerChecker) StepExecutorOption {
	return func(e *StepExecutor) {
		e.dangerChecker = checker
	}
}

// withStepAutoConfirm enables auto-confirmation for dangerous commands.
func withStepAutoConfirm(autoConfirm bool) StepExecutorOption {
	return func(e *StepExecutor) {
		e.autoConfirm = autoConfirm
	}
}

// NewStepExecutor creates a new StepExecutor with the given options.
func NewStepExecutor(opts ...StepExecutorOption) *StepExecutor {
	e := &StepExecutor{
		shell:        "bash",
		confirmEach:  false,
		streamOutput: true,
		env:          make(map[string]string),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// ExecStep executes a single workflow step with optional confirmation.
func (e *StepExecutor) ExecStep(ctx context.Context, step *workflows.Step, parameters map[string]string, repoRoot string, sink OutputSink) StepResult {
	startTime := time.Now()

	// Substitute placeholders in command
	cmd := step.Command
	if len(parameters) > 0 {
		// Use placeholders package for substitution
		// This is a simple implementation - the real one uses the placeholders package
		for key, value := range parameters {
			placeholder := "<" + key + ">"
			cmd = strings.ReplaceAll(cmd, placeholder, value)
		}
	}

	// Resolve working directory
	cwd := step.CWD
	if cwd == "" && e.cwd != "" {
		cwd = e.cwd
	}

	// Resolve shell
	shell := step.Shell
	if shell == "" {
		shell = e.shell
	}

	// Merge environment variables
	env := make(map[string]string)
	for k, v := range e.env {
		env[k] = v
	}
	for k, v := range step.Env {
		env[k] = v
	}

	// Check if we should prompt for confirmation
	shouldConfirm := e.confirmEach
	if step.Confirmation != nil {
		// Step-level confirmation takes precedence
		shouldConfirm = true
	}

	if shouldConfirm && !e.autoConfirm {
		confirmed, skipped := e.promptConfirmation(step, cmd)
		if skipped {
			return StepResult{
				Success:  false,
				Skipped:  true,
				Duration: time.Since(startTime),
			}
		}
		if !confirmed {
			// User chose to quit
			return StepResult{
				Success:  false,
				Canceled: true,
				Duration: time.Since(startTime),
			}
		}
	}

	// Execute the command
	config := ExecConfig{
		Command:       cmd,
		Shell:         shell,
		CWD:           cwd,
		Env:           env,
		Stream:        e.streamOutput,
		DangerChecker: e.dangerChecker,
		AutoConfirm:   e.autoConfirm,
	}

	result := Exec(ctx, config)

	// Write output to sink if streaming
	if e.streamOutput && sink != nil && result.Output != "" {
		for _, line := range strings.Split(result.Output, "\n") {
			if line != "" {
				_ = sink.Write(line)
			}
		}
	}

	return StepResult{
		Success:  result.Success,
		Skipped:  false,
		Canceled: false,
		ExitCode: result.ExitCode,
		Output:   result.Output,
		Duration: result.Duration,
		Error:    result.Error,
	}
}

// promptConfirmation prompts the user before executing a step.
// Returns (confirmed, skipped). If both are false, user chose to quit.
func (e *StepExecutor) promptConfirmation(step *workflows.Step, command string) (confirmed bool, skipped bool) {
	// Build prompt message
	var promptBuilder strings.Builder

	if step.Name != "" {
		promptBuilder.WriteString("Step: ")
		promptBuilder.WriteString(step.Name)
		promptBuilder.WriteString("\n")
	}

	promptBuilder.WriteString("Command: ")
	promptBuilder.WriteString(command)

	// Use custom prompt if specified
	if step.Confirmation != nil && step.Confirmation.Prompt != "" {
		promptBuilder.Reset()
		promptBuilder.WriteString(step.Confirmation.Prompt)
		promptBuilder.WriteString("\n")
		promptBuilder.WriteString("Command: ")
		promptBuilder.WriteString(command)
	}

	promptBuilder.WriteString("\n")
	promptBuilder.WriteString("Run this step? [Y/n/s/q] ")

	// Display prompt
	fmt.Print(promptBuilder.String())

	// Read user input
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error reading input: %v\n", err)
		return false, false
	}

	response = strings.TrimSpace(strings.ToLower(response))

	switch response {
	case "", "y", "yes":
		return true, false
	case "n", "no":
		return false, true // Skip
	case "s":
		return false, true // Skip
	case "q":
		return false, false // Quit
	default:
		fmt.Printf("Invalid response. Skipping step.\n")
		return false, true
	}
}
