// Package runner provides workflow execution engine.
package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/chazuruo/svf/internal/placeholders"
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
		// Substitute placeholders in command using placeholders package
		cmd, err := placeholders.Substitute(step.Command, plan.Parameters)
		if err != nil {
			// Check if we have any placeholders at all
			phNames := placeholders.CollectFromSteps(plan.Workflow.Steps)
			if len(phNames) == 0 {
				// No placeholders in workflow, use original command
				cmd = step.Command
			} else {
				// We have placeholders but substitution failed
				result.Success = false
				result.FailedStep = i
				result.ExitCode = 21 // Missing parameter exit code
				result.Duration = time.Since(startTime)
				return result, fmt.Errorf("step %d: %w", i, err)
			}
		}

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

// executeStep executes a single workflow step using the Exec function.
func (r *runner) executeStep(ctx context.Context, stepIndex int, command, shell, cwd string, env map[string]string, sink OutputSink) StepResult {
	// Use the Exec function for execution
	execConfig := ExecConfig{
		Command: command,
		Shell:   shell,
		CWD:     cwd,
		Env:     env,
		Stream:  r.streamOutput,
	}

	result := Exec(ctx, execConfig)

	// Write output to sink if streaming
	if r.streamOutput && sink != nil && result.Output != "" {
		for _, line := range strings.Split(result.Output, "\n") {
			if line != "" {
				_ = sink.Write(line)
			}
		}
	}

	return StepResult{
		Step:     stepIndex,
		Success:  result.Success,
		ExitCode: result.ExitCode,
		Output:   result.Output,
		Duration: result.Duration,
		Error:    result.Error,
	}
}

// getExitCode extracts the exit code from an exec.ExitError.
func getExitCode(err *exec.ExitError) int {
	if status, ok := err.Sys().(syscall.WaitStatus); ok {
		return status.ExitStatus()
	}
	return 1
}

// ExecConfig contains configuration for executing a single command.
type ExecConfig struct {
	Command    string            // Command to execute
	Shell      string            // Shell to use (bash, zsh, sh, pwsh)
	CWD        string            // Working directory
	Env        map[string]string // Environment variables
	Stream     bool              // Whether to stream output
	DangerChecker *DangerChecker // For dangerous command checking
	AutoConfirm bool             // Auto-confirm dangerous commands
}

// ExecResult contains the result of executing a single command.
type ExecResult struct {
	Command    string
	ExitCode   int
	Success    bool
	Output     string
	Duration   time.Duration
	Dangerous  bool
	Danger     *DangerInfo
	Error      error
}

// Exec executes a single command with the given configuration.
func Exec(ctx context.Context, config ExecConfig) ExecResult {
	startTime := time.Now()

	result := ExecResult{
		Command: config.Command,
	}

	// Check for dangerous commands
	if config.DangerChecker != nil {
		danger := config.DangerChecker.Check(config.Command)
		result.Dangerous = danger != nil
		result.Danger = danger

		if danger != nil {
			if config.AutoConfirm {
				// Auto-confirm mode (--yes flag), show warning but proceed
				fmt.Fprintf(os.Stderr, "Warning: %s\n", danger.Warning())
			} else {
				// Interactive mode, prompt for confirmation
				confirmed, err := danger.Confirm()
				if err != nil {
					result.Error = err
					result.Success = false
					result.ExitCode = 1
					result.Duration = time.Since(startTime)
					return result
				}
				if !confirmed {
					result.Success = false
					result.ExitCode = 13 // Canceled
					result.Duration = time.Since(startTime)
					return result
				}
			}
		}
	}

	// Determine shell
	shell := config.Shell
	if shell == "" {
		shell = "bash"
	}

	// Build command
	var cmd *exec.Cmd
	switch shell {
	case "bash", "sh", "zsh", "pwsh":
		cmd = exec.CommandContext(ctx, shell, "-c", config.Command)
	default:
		// Try to run the command directly
		parts := strings.Fields(config.Command)
		if len(parts) > 0 {
			cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
		} else {
			result.Error = fmt.Errorf("empty command")
			result.Success = false
			result.Duration = time.Since(startTime)
			return result
		}
	}

	// Set working directory
	if config.CWD != "" {
		cmd.Dir = config.CWD
	}

	// Set environment
	if len(config.Env) > 0 {
		cmd.Env = append([]string{}, os.Environ()...)
		for k, v := range config.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Execute and capture output
	var output strings.Builder
	if config.Stream {
		// Stream output in real-time
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			result.Error = fmt.Errorf("failed to create stdout pipe: %w", err)
			result.Success = false
			result.Duration = time.Since(startTime)
			return result
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			result.Error = fmt.Errorf("failed to create stderr pipe: %w", err)
			result.Success = false
			result.Duration = time.Since(startTime)
			return result
		}

		if err := cmd.Start(); err != nil {
			result.Error = fmt.Errorf("failed to start command: %w", err)
			result.Success = false
			result.Duration = time.Since(startTime)
			return result
		}

		// Use WaitGroup to wait for all goroutines
		var wg sync.WaitGroup
		wg.Add(2)

		// Read stdout
		go func() {
			defer wg.Done()
			scanner := newLineScanner(stdout)
			for scanner.Scan() {
				line := scanner.Text()
				output.WriteString(line + "\n")
			}
		}()

		// Read stderr
		go func() {
			defer wg.Done()
			scanner := newLineScanner(stderr)
			for scanner.Scan() {
				line := scanner.Text()
				output.WriteString(line + "\n")
			}
		}()

		// Wait for completion
		err = cmd.Wait()
		wg.Wait() // Wait for all goroutines to finish

		result.Output = output.String()
		result.Duration = time.Since(startTime)

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = getExitCode(exitErr)
				result.Success = false
				result.Error = err
				return result
			}
			result.Error = err
			result.Success = false
			return result
		}
	} else {
		// Capture all output at once
		out, err := cmd.CombinedOutput()
		output.Write(out)
		result.Output = output.String()
		result.Duration = time.Since(startTime)

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = getExitCode(exitErr)
				result.Success = false
				result.Error = err
				return result
			}
			result.Error = err
			result.Success = false
			return result
		}
	}

	result.Success = true
	result.ExitCode = 0
	return result
}

// lineScanner provides line-by-line scanning with proper handling.
type lineScanner struct {
	reader *bufio.Reader
	line   string
	err    error
}

func newLineScanner(r io.Reader) *lineScanner {
	return &lineScanner{
		reader: bufio.NewReader(r),
	}
}

func (s *lineScanner) Scan() bool {
	s.line, s.err = s.reader.ReadString('\n')
	if s.err != nil {
		if s.err == io.EOF {
			// Return the last line if there is one
			return s.line != ""
		}
		return false
	}
	return true
}

func (s *lineScanner) Text() string {
	// Trim the newline character
	return strings.TrimSuffix(s.line, "\n")
}

func (s *lineScanner) Err() error {
	if s.err == io.EOF {
		return nil
	}
	return s.err
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
