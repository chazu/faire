// Package runner provides workflow execution engine.
package runner

import (
	"context"
	"fmt"
	"path/filepath"
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

// RunResult contains the result of a workflow run.
type RunResult struct {
	Success     bool
	FailedStep  int
	ExitCode    int
	StepResults []StepResult
	Canceled    bool
	Duration    time.Duration
}

// StepResult contains the result of a single step.
type StepResult struct {
	Step     int // Step index
	Success  bool
	Skipped  bool
	Canceled bool
	ExitCode int
	Output   string
	Duration time.Duration
	Error    error
}

// runner implements Runner.
type runner struct {
	executor      *StepExecutor
	shell         string
	streamOutput  *bool
	confirmEach   *bool
	cwd           string
	dangerChecker *DangerChecker
	autoConfirm   *bool
}

// NewRunner creates a new runner.
func NewRunner(opts ...Option) Runner {
	r := &runner{
		shell:        "bash",
		streamOutput: boolPtr(true),
		confirmEach:  boolPtr(false),
	}

	for _, opt := range opts {
		opt(r)
	}

	// Create step executor options from runner options
	execOpts := []StepExecutorOption{
		withStepShell(r.shell),
		withStepStreamOutput(*r.streamOutput),
		withStepConfirmEach(*r.confirmEach),
	}
	if r.cwd != "" {
		execOpts = append(execOpts, withStepCWD(r.cwd))
	}
	if r.dangerChecker != nil {
		execOpts = append(execOpts, withStepDangerChecker(r.dangerChecker))
	}
	if r.autoConfirm != nil {
		execOpts = append(execOpts, withStepAutoConfirm(*r.autoConfirm))
	}

	r.executor = NewStepExecutor(execOpts...)
	return r
}

func boolPtr(b bool) *bool {
	return &b
}

// Option configures a runner.
type Option func(*runner)

// WithShell sets the default shell.
func WithShell(shell string) Option {
	return func(r *runner) {
		if shell != "" {
			r.shell = shell
		}
	}
}

// WithStreamOutput enables/disables output streaming.
func WithStreamOutput(stream bool) Option {
	return func(r *runner) {
		r.streamOutput = boolPtr(stream)
	}
}

// WithConfirmEach enables confirmation prompts before each step.
func WithConfirmEach(confirm bool) Option {
	return func(r *runner) {
		r.confirmEach = boolPtr(confirm)
	}
}

// WithCWD sets the default working directory.
func WithCWD(cwd string) Option {
	return func(r *runner) {
		r.cwd = cwd
	}
}

// WithDangerChecker sets the dangerous command checker.
func WithDangerChecker(checker *DangerChecker) Option {
	return func(r *runner) {
		r.dangerChecker = checker
	}
}

// WithAutoConfirm enables auto-confirmation for dangerous commands.
func WithAutoConfirm(autoConfirm bool) Option {
	return func(r *runner) {
		r.autoConfirm = boolPtr(autoConfirm)
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

		// Create a modified step with the substituted command
		modifiedStep := step
		modifiedStep.Command = cmd

		// Resolve working directory
		cwd := step.CWD
		if cwd == "" && plan.Workflow.Defaults.CWD != "" {
			cwd = plan.Workflow.Defaults.CWD
		}
		if !filepath.IsAbs(cwd) && plan.RepoRoot != "" {
			cwd = filepath.Join(plan.RepoRoot, cwd)
		}

		// Configure executor with step-specific settings
		executor := r.executor
		if cwd != "" || step.Env != nil {
			opts := []StepExecutorOption{}
			if cwd != "" {
				opts = append(opts, withStepCWD(cwd))
			}
			if step.Env != nil {
				// Merge with executor's env
				mergedEnv := make(map[string]string)
				// Start with executor's env (we'd need to expose this)
				for k, v := range step.Env {
					mergedEnv[k] = v
				}
				opts = append(opts, withStepEnv(mergedEnv))
			}
			executor = NewStepExecutor(opts...)
		}

		// Execute step
		stepResult := executor.ExecStep(ctx, &modifiedStep, plan.Parameters, plan.RepoRoot, sink)
		stepResult.Step = i
		result.StepResults[i] = stepResult

		// Check if step was canceled
		if stepResult.Canceled {
			result.Success = false
			result.FailedStep = i
			result.Canceled = true
			result.ExitCode = 13 // Canceled
			result.Duration = time.Since(startTime)
			return result, nil
		}

		// Check if step failed (and wasn't skipped)
		if !stepResult.Success && !stepResult.Skipped && !step.ContinueOnError {
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
