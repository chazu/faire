// Package cli provides Cobra command definitions for svf.
package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/gitrepo"
	"github.com/chazuruo/svf/internal/placeholders"
	//nolint:staticcheck // SA1019 - Using runner for Exec, DangerChecker, Plan types (deprecated but needed)
	runnerpkg "github.com/chazuruo/svf/internal/runner"
	"github.com/chazuruo/svf/internal/tui"
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

	// Extract placeholders from workflow using placeholders package
	phInfo := placeholders.ExtractWithMetadata(wf)

	// Start with provided params
	allParams := make(map[string]string)
	for k, v := range opts.Params {
		allParams[k] = v
	}

	// In non-interactive mode, if we have placeholders without values, fail
	if len(phInfo) > 0 {
		missing := []string{}
		for name := range phInfo {
			if _, ok := allParams[name]; !ok {
				// Check if there's a default value
				if phInfo[name].Default != "" {
					allParams[name] = phInfo[name].Default
				} else {
					missing = append(missing, name)
				}
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("missing placeholder values (use --param to provide): %s\nExample: --param %s=value",
				fmt.Sprintf("<%s>", strings.Join(missing, ", <")), missing[0])
		}
	}

	// Create runner with dangerous command checking
	dangerChecker := runnerpkg.NewDangerChecker(cfg.Runner.DangerousCommandWarnings)

	// Execute each step
	success := true
	var failedStep int

	for i, step := range wf.Steps {
		// Substitute placeholders using placeholders package
		cmd, err := placeholders.Substitute(step.Command, allParams)
		if err != nil {
			// Substitution failed - check if we have any placeholders at all
			phNames := placeholders.CollectFromSteps(wf.Steps)
			if len(phNames) == 0 {
				// No placeholders in workflow, use original command
				cmd = step.Command
			} else {
				// We have placeholders but substitution failed
				return fmt.Errorf("step %d: %w", i, err)
			}
		}

		// Resolve working directory
		cwd := step.CWD
		if cwd == "" && wf.Defaults.CWD != "" {
			cwd = wf.Defaults.CWD
		}
		if !filepath.IsAbs(cwd) && cfg.Repo.Path != "" {
			cwd = filepath.Join(cfg.Repo.Path, cwd)
		}

		// Show command
		fmt.Printf("Step %d/%d: %s\n", i+1, len(wf.Steps), step.Name)
		if opts.DryRun {
			fmt.Printf("  Would execute: %s\n", cmd)
			if cwd != "" {
				fmt.Printf("  Working directory: %s\n", cwd)
			}
			if step.Shell != "" {
				fmt.Printf("  Shell: %s\n", step.Shell)
			}
			continue
		}

		// Execute step using runner.Exec
		execConfig := runnerpkg.ExecConfig{
			Command:       cmd,
			Shell:         step.Shell,
			CWD:           cwd,
			Env:           step.Env,
			Stream:        cfg.Runner.StreamOutput,
			DangerChecker: dangerChecker,
			AutoConfirm:   opts.Yes,
		}

		result := runnerpkg.Exec(context.Background(), execConfig)

		// Show output if streaming was not enabled
		if !cfg.Runner.StreamOutput && result.Output != "" {
			fmt.Print(result.Output)
		}

		// Check for cancellation
		if result.ExitCode == 13 {
			fmt.Println("\nWorkflow canceled")
			return fmt.Errorf("workflow canceled (exit code 13)")
		}

		// Check for failure
		if !result.Success {
			if !step.ContinueOnError {
				success = false
				failedStep = i
				fmt.Printf("\n✗ Step failed with exit code %d\n", result.ExitCode)
				if result.Error != nil {
					fmt.Printf("  Error: %v\n", result.Error)
				}
				break
			}
			fmt.Printf("\n⚠ Step failed (exit code %d) but continuing...\n", result.ExitCode)
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
	// Collect parameters from options
	params := make(map[string]string)
	for k, v := range opts.Params {
		params[k] = v
	}

	// Handle --from and --until options by filtering steps
	steps := wf.Steps
	startIdx := 0
	endIdx := len(steps)

	if opts.From != "" {
		for i, step := range steps {
			if step.Name == opts.From {
				startIdx = i
				break
			}
		}
	}

	if opts.Until != "" {
		for i, step := range steps {
			if step.Name == opts.Until {
				endIdx = i
				break
			}
		}
	}

	// Create a filtered workflow for execution
	filteredWf := *wf
	filteredWf.Steps = steps[startIdx:endIdx]

	// Create execution plan
	plan := runnerpkg.Plan{
		Workflow:   &filteredWf,
		Parameters: params,
		RepoRoot:   cfg.Repo.Path,
	}

	// Create TUI runner model with full config support
	model := tui.NewRunnerModelWithConfig(plan, cfg)

	// Run the TUI
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	// Check result
	result := finalModel.(tui.RunnerModel)
	if result.DidCancel() {
		return fmt.Errorf("workflow canceled (exit code 13)")
	}
	if !result.DidSucceed() {
		return fmt.Errorf("workflow failed (exit code 20)")
	}

	return nil
}
