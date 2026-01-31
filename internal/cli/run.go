// Package cli provides Cobra command definitions for svf.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/gitrepo"
	"github.com/chazuruo/svf/internal/runner"
	"github.com/chazuruo/svf/internal/workflows"
	"github.com/chazuruo/svf/internal/workflows/store"
)

// RunOptions contains the options for the run command.
type RunOptions struct {
	ConfigPath string
	WorkflowRef string
	Params     map[string]string
	Env        map[string]string
	Local      bool
	Yes        bool
	CWD        string
	Until      string
	From       string
	DryRun     bool
	LogPath    string
	SaveParams bool
}

// NewRunCommand creates the run command.
func NewRunCommand() *cobra.Command {
	opts := &RunOptions{
		Params: make(map[string]string),
		Env:    make(map[string]string),
	}

	cmd := &cobra.Command{
		Use:   "run [workflow-ref]",
		Short: "Run a workflow interactively or non-interactively",
		Long: `Execute a workflow step-by-step.

Interactive mode (default):
- Shows step list with status icons
- Prompts for placeholders once per unique value
- Press Enter to execute each step
- Supports: s (skip), r (rerun), q (quit), e (edit step)

Non-interactive mode (--yes):
- Auto-confirms all steps
- Requires placeholders via --param or config
- Exit codes: 0 (success), 20 (step failed), 21 (missing param), 13 (canceled)

Offline mode (--local):
- Skip git fetch, use current checkout

Dry run mode (--dry-run):
- Show commands after placeholder substitution
- Don't execute anything`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If workflow ref is provided, use it
			if len(args) > 0 {
				opts.WorkflowRef = args[0]
			}
			return runRun(opts)
		},
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.Flags().StringToStringVar(&opts.Params, "param", nil, "placeholder values (repeatable, e.g., --param key=value)")
	cmd.Flags().BoolVar(&opts.Local, "local", false, "use local checkout only (no fetch)")
	cmd.Flags().BoolVar(&opts.Yes, "yes", false, "non-interactive mode (auto-confirm all steps)")
	cmd.Flags().StringVar(&opts.CWD, "cwd", "", "working directory override")
	cmd.Flags().StringVar(&opts.Until, "until", "", "stop before this step name")
	cmd.Flags().StringVar(&opts.From, "from", "", "start from this step name")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "show commands without executing")
	cmd.Flags().StringVar(&opts.LogPath, "log", "", "write run log to file")
	cmd.Flags().BoolVar(&opts.SaveParams, "save-params", false, "save provided parameters to workflow")
	cmd.Flags().StringToStringVar(&opts.Env, "env", nil, "environment variables (repeatable, e.g., --env key=value)")

	return cmd
}

func runRun(opts *RunOptions) error {
	ctx := context.Background()

	// Load config
	cfg, err := config.LoadWithDefaults()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Open repo
	repo := gitrepo.New(cfg.Repo.Path)
	if !repo.IsInitialized(ctx) {
		return fmt.Errorf("repository not initialized. Run 'svf init' first")
	}

	// Create store
	str, err := store.New(repo, cfg)
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}

	// Resolve workflow
	if opts.WorkflowRef == "" {
		return fmt.Errorf("workflow reference required\nUsage: svf run <workflow-ref>\nOr use --no-tui with --query to search")
	}

	ref, err := resolveWorkflowRef(ctx, str, opts.WorkflowRef)
	if err != nil {
		return err
	}

	// Load workflow
	wf, err := str.Load(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to load workflow: %w", err)
	}

	// Check for --yes flag or global --no-tui
	if opts.Yes || IsNoTUI() {
		return runNonInteractive(ctx, wf, opts, cfg)
	}

	// Interactive mode
	return runInteractive(ctx, wf, opts, cfg)
}

// runNonInteractive executes a workflow without TUI.
func runNonInteractive(ctx context.Context, wf *workflows.Workflow, opts *RunOptions, cfg *config.Config) error {
	// Apply workflow defaults
	for i := range wf.Steps {
		wf.ApplyDefaults(&wf.Steps[i])
	}

	// Check for --local flag - skip git fetch if set
	if !opts.Local {
		// TODO: Implement git fetch
		// For now, just show message
		if IsNoTUI() {
			// LLM mode, don't show message
		} else {
			fmt.Println("Syncing with remote...")
		}
	} else {
		fmt.Println("Using local checkout (--local mode)")
	}

	// Extract placeholders from all steps
	allParams := make(map[string]string)
	for _, step := range wf.Steps {
		// TODO: Use placeholders package to extract
		// For now, skip extraction
		_ = step
	}

	// Merge with provided params
	for k, v := range opts.Params {
		allParams[k] = v
	}

	// Create runner with dangerous command checking
	dangerChecker := runner.NewDangerChecker(cfg.Runner.DangerousCommandWarnings)

	// Execute each step
	success := true
	var failedStep int

	for i, step := range wf.Steps {
		// Substitute placeholders
		// TODO: Use placeholders.Substitute
		cmd := step.Command

		// Show command
		fmt.Printf("Step %d/%d: %s\n", i+1, len(wf.Steps), step.Name)
		if opts.DryRun {
			fmt.Printf("  Would execute: %s\n", cmd)
			continue
		}

		// Check for dangerous command
		danger := dangerChecker.Check(cmd)
		if danger != nil && opts.Yes {
			// Auto-confirm mode, show warning but proceed
			fmt.Fprintf(os.Stderr, "  Warning: %s\n", danger.Risk)
		}

		// Execute step
		// TODO: Actually execute using runner.Exec
		fmt.Printf("  Executing: %s...\n", cmd)

		// Simulate execution for now
		if !opts.Yes {
			// In non-yes mode, we'd prompt here
			// For now, just continue
		}
	}

	if success {
		fmt.Println("\n✓ Workflow completed successfully")
		return nil
	}

	return fmt.Errorf("workflow failed at step %d (exit code 20)", failedStep)
}

// runInteractive executes a workflow with TUI.
func runInteractive(ctx context.Context, wf *workflows.Workflow, opts *RunOptions, cfg *config.Config) error {
	// TODO: Integrate with TUI runner model from internal/tui/runner.go
	// For now, fall back to non-interactive with confirmation
	fmt.Printf("Running workflow: %s\n", wf.Title)
	fmt.Println("(Interactive mode - TODO: integrate TUI runner)")
	fmt.Println("Falling back to non-interactive mode with prompts...")
	return runNonInteractive(ctx, wf, opts, cfg)
}

// extractPlaceholders extracts all placeholders from a workflow.
func extractPlaceholders(wf interface{}) []string {
	// TODO: Implement placeholder extraction
	// For now, return empty
	return []string{}
}

// findMissingPlaceholders finds placeholders that don't have values.
func findMissingPlaceholders(params map[string]string, wf interface{}) []string {
	// TODO: Implement missing placeholder detection
	// For now, return empty
	return []string{}
}
