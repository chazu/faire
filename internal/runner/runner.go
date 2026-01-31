// Package runner provides workflow execution engine.
package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/chazuruo/svf/internal/workflows"
)

// Runner executes workflows.
type Runner interface {
	// Run executes a workflow plan.
	Run(ctx context.Context, plan Plan, sink OutputSink) (RunResult, error)
}

// Plan represents an executable workflow plan.
type Plan struct {
	Workflow   *workflows.Workflow
	Parameters map[string]string // Resolved placeholder values
	RepoRoot   string            // Repository root path
}

// OutputSink receives command output.
type OutputSink interface {
	// Write writes a line of output.
	Write(line string) error
	// Close closes the sink.
	Close() error
}

// RunResult contains the result of a workflow run.
type RunResult struct {
	Success      bool
	FailedStep   int
	ExitCode     int
	StepResults  []StepResult
	Canceled     bool
	Duration     time.Duration
}

// StepResult contains the result of a single step.
type StepResult struct {
	Step       int
	Success    bool
	ExitCode   int
	Output     string
	Duration   time.Duration
	Error      error
}

// runner implements Runner.
type runner struct {
	shell      string
	streamOutput bool
}

// NewRunner creates a new runner.
func NewRunner(opts ...Option) Runner {
	r := &runner{
		shell:      "bash",
		streamOutput: true,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Option configures a runner.
type Option func(*runner)

// WithShell sets the default shell.
func WithShell(shell string) Option {
	return func(r *runner) {
		r.shell = shell
	}
}

// WithStreamOutput enables/disables output streaming.
func WithStreamOutput(stream bool) Option {
	return func(r *runner) {
		r.streamOutput = stream
	}
}

// Run executes a workflow plan.
func (r *runner) Run(ctx context.Context, plan Plan, sink OutputSink) (RunResult, error) {
	startTime := time.Now()
	result := RunResult{
		StepResults: make([]StepResult, len(plan.Workflow.Steps)),
	}

	// Apply workflow defaults to each step
	for i := range plan.Workflow.Steps {
		plan.Workflow.ApplyDefaults(&plan.Workflow.Steps[i])
	}

	// Execute each step
	for i, step := range plan.Workflow.Steps {
		// Substitute placeholders in command
		cmd := substitutePlaceholders(step.Command, plan.Parameters)

		// Resolve working directory
		cwd := step.CWD
		if cwd == "" && plan.Workflow.Defaults.CWD != "" {
			cwd = plan.Workflow.Defaults.CWD
		}
		if !filepath.IsAbs(cwd) && plan.RepoRoot != "" {
			cwd = filepath.Join(plan.RepoRoot, cwd)
		}

		// Get shell for this step
		shell := step.Shell
		if shell == "" {
			shell = r.shell
		}

		// Execute step
		stepResult := r.executeStep(ctx, i, cmd, shell, cwd, step.Env, sink)
		result.StepResults[i] = stepResult

		// Check if step failed
		if !stepResult.Success && !step.ContinueOnError {
			result.Success = false
			result.FailedStep = i
			result.ExitCode = stepResult.ExitCode
			result.Duration = time.Since(startTime)
			return result, nil
		}
	}

	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

// executeStep executes a single workflow step.
func (r *runner) executeStep(ctx context.Context, stepIndex int, command, shell, cwd string, env map[string]string, sink OutputSink) StepResult {
	startTime := time.Now()

	// Build command
	var cmd *exec.Cmd
	switch shell {
	case "bash", "sh":
		cmd = exec.CommandContext(ctx, shell, "-c", command)
	case "zsh":
		cmd = exec.CommandContext(ctx, "zsh", "-c", command)
	default:
		// Try to run the command directly
		parts := strings.Fields(command)
		if len(parts) > 0 {
			cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
		} else {
			return StepResult{
				Step:    stepIndex,
				Success: false,
				Error:   fmt.Errorf("empty command"),
			}
		}
	}

	// Set working directory
	if cwd != "" {
		cmd.Dir = cwd
	}

	// Set environment
	if len(env) > 0 {
		cmd.Env = append(os.Environ())
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Capture output
	var output strings.Builder
	if r.streamOutput && sink != nil {
		// Stream output in real-time
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return StepResult{
				Step:    stepIndex,
				Success: false,
				Error:   fmt.Errorf("failed to create stdout pipe: %w", err),
			}
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return StepResult{
				Step:    stepIndex,
				Success: false,
				Error:   fmt.Errorf("failed to create stderr pipe: %w", err),
			}
		}

		if err := cmd.Start(); err != nil {
			return StepResult{
				Step:    stepIndex,
				Success: false,
				Error:   fmt.Errorf("failed to start command: %w", err),
			}
		}

		// Read output
		go io.Copy(&output, stdout)
		go io.Copy(&output, stderr)

		// Wait for completion
		if err := cmd.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return StepResult{
					Step:     stepIndex,
					Success:  false,
					ExitCode: getExitCode(exitErr),
					Output:   output.String(),
					Duration: time.Since(startTime),
					Error:    err,
				}
			}
		}
	} else {
		// Capture all output at once
		out, err := cmd.CombinedOutput()
		output.Write(out)
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return StepResult{
					Step:     stepIndex,
					Success:  false,
					ExitCode: getExitCode(exitErr),
					Output:   output.String(),
					Duration: time.Since(startTime),
					Error:    err,
				}
			}
		}
	}

	return StepResult{
		Step:     stepIndex,
		Success:  true,
		ExitCode: 0,
		Output:   output.String(),
		Duration: time.Since(startTime),
	}
}

// substitutePlaceholders replaces placeholder tokens with values.
func substitutePlaceholders(command string, params map[string]string) string {
	result := command
	for key, value := range params {
		placeholder := fmt.Sprintf("<%s>", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// getExitCode extracts the exit code from an exec.ExitError.
func getExitCode(err *exec.ExitError) int {
	if status, ok := err.Sys().(syscall.WaitStatus); ok {
		return status.ExitStatus()
	}
	return 1
}

// StdioSink is an OutputSink that writes to stdout/stderr.
type StdioSink struct{}

// NewStdioSink creates a new sink that writes to stdout/stderr.
func NewStdioSink() *StdioSink {
	return &StdioSink{}
}

// Write writes a line to stdout.
func (s *StdioSink) Write(line string) error {
	fmt.Println(line)
	return nil
}

// Close closes the sink (no-op for StdioSink).
func (s *StdioSink) Close() error {
	return nil
}
