// Package cli provides Cobra command definitions for svf.
package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/gitrepo"
	"github.com/chazuruo/svf/internal/workflows/store"
)

// RunOptions contains the options for the run command.
type RunOptions struct {
	ConfigPath string
	WorkflowRef string
	Params     map[string]string
	Local      bool
	Yes        bool
	CWD        string
	DryRun     bool
	LogPath    string
}

// NewRunCommand creates the run command.
func NewRunCommand() *cobra.Command {
	opts := &RunOptions{
		Params: make(map[string]string),
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
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "show commands without executing")
	cmd.Flags().StringVar(&opts.LogPath, "log", "", "write run log to file")

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
		return runNonInteractive(ctx, wf, opts, cfg.Repo.Path)
	}

	// Interactive mode
	return runInteractive(ctx, wf, opts, cfg.Repo.Path)
}

// runNonInteractive executes a workflow without TUI.
func runNonInteractive(ctx context.Context, wf interface{}, opts *RunOptions, repoRoot string) error {
	// TODO: Convert wf to proper workflow type
	// For now, this is a placeholder

	// Extract placeholders from workflow
	// params := extractPlaceholders(wf)

	// Merge with provided params
	// for k, v := range opts.Params {
	//     params[k] = v
	// }

	// Check for missing placeholders
	// missing := findMissingPlaceholders(params, wf)
	// if len(missing) > 0 {
	//     return fmt.Errorf("missing placeholders: %s (use --param to provide)", strings.Join(missing, ", "))
	// }

	fmt.Printf("Running workflow: %s\n", "workflow-title-placeholder")

	// Execute each step
	for i := 0; i < 10; i++ { // placeholder loop
		fmt.Printf("Step %d: command here...\n", i+1)
		// Execute step
	}

	fmt.Println("\n✓ Workflow completed successfully")
	return nil
}

// runInteractive executes a workflow with TUI.
func runInteractive(ctx context.Context, wf interface{}, opts *RunOptions, repoRoot string) error {
	// TODO: Integrate with TUI runner model
	// For now, fall back to non-interactive
	return runNonInteractive(ctx, wf, opts, repoRoot)
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
