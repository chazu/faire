package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/chazuruo/faire/internal/app"
	"github.com/chazuruo/faire/internal/config"
	"github.com/chazuruo/faire/internal/gitrepo"
	"github.com/chazuruo/faire/internal/workflows"
	"github.com/chazuruo/faire/internal/workflows/store"
	"github.com/spf13/cobra"
)

// RecordHistoryOptions contains the options for the record history command
type RecordHistoryOptions struct {
	Shell    string
	Limit    int
	Since    string
	Title    string
	Desc     string
	Tags     []string
	Identity string
	Draft    bool
	NoCommit bool
	All      bool // Select all commands (no TUI)
}

// NewRecordHistoryCommand creates the record history command
func NewRecordHistoryCommand() *cobra.Command {
	opts := &RecordHistoryOptions{}

	cmd := &cobra.Command{
		Use:   "record history",
		Short: "Record shell history as a workflow",
		Long: `Record shell history commands as a workflow.

Loads commands from your shell history and creates a workflow from them.
You can filter by time, limit the number of commands, and provide custom
metadata.`,
		Example: `  # Record last 50 commands as a workflow
  gitsavvy record history --limit 50

  # Record commands from the last hour
  gitsavvy record history --since 1h

  # Record with custom title and description
  gitsavvy record history --title "Deploy to staging" --desc "Standard deployment process"

  # Save to drafts instead of workflows
  gitsavvy record history --draft

  # Skip git commit
  gitsavvy record history --no-commit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecordHistory(opts)
		},
	}

	cmd.Flags().StringVar(&opts.Shell, "shell", "", "Shell type (bash, zsh, pwsh). Default: auto-detect")
	cmd.Flags().IntVar(&opts.Limit, "limit", 500, "Maximum number of commands to load")
	cmd.Flags().StringVar(&opts.Since, "since", "", "Time filter (e.g., 1h, 1d, 1w)")
	cmd.Flags().StringVar(&opts.Title, "title", "", "Workflow title")
	cmd.Flags().StringVar(&opts.Desc, "desc", "", "Workflow description")
	cmd.Flags().StringSliceVar(&opts.Tags, "tags", []string{}, "Workflow tags (comma-separated)")
	cmd.Flags().StringVar(&opts.Identity, "identity", "", "Override identity path")
	cmd.Flags().BoolVar(&opts.Draft, "draft", false, "Save to drafts/ instead of workflows/")
	cmd.Flags().BoolVar(&opts.NoCommit, "no-commit", false, "Skip git commit after saving")
	cmd.Flags().BoolVar(&opts.All, "all", false, "Select all commands without TUI")

	return cmd
}

func runRecordHistory(opts *RecordHistoryOptions) error {
	ctx := context.Background()

	// Load config
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Parse since duration
	var sinceDuration time.Duration
	if opts.Since != "" {
		var err error
		sinceDuration, err = app.ParseDuration(opts.Since)
		if err != nil {
			return fmt.Errorf("invalid --since value: %w", err)
		}
	}

	// Build app options
	appOpts := app.RecordHistoryOptions{
		Shell:    opts.Shell,
		Limit:    opts.Limit,
		Since:    sinceDuration,
		Title:    opts.Title,
		Desc:     opts.Desc,
		Tags:     opts.Tags,
		Identity: opts.Identity,
		Draft:    opts.Draft,
		NoCommit: opts.NoCommit,
	}

	// Load history
	lines, err := app.LoadHistoryForRecording(appOpts)
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	if len(lines) == 0 {
		fmt.Fprintln(os.Stderr, "No commands found in history")
		os.Exit(13)
		return nil
	}

	// TODO: Launch history picker TUI when implemented (blocked on fa-1jr)
	// For now, use --all flag to select all commands, or show a simple list
	if !opts.All {
		fmt.Fprintln(os.Stderr, "WARNING: TUI history picker not yet implemented (blocked on fa-1jr)")
		fmt.Fprintln(os.Stderr, "Using --all mode. Use --all to suppress this warning.")
		fmt.Fprintln(os.Stderr)
		opts.All = true
	}

	// Extract commands from history lines
	commands := make([]string, len(lines))
	for i, line := range lines {
		commands[i] = line.Command
	}

	// Generate workflow from commands
	wf := app.GenerateWorkflowFromCommands(commands, appOpts)

	// Get workflow store path
	repoPath := cfg.Repo.Path
	if repoPath == "" {
		// Try to detect git repository from current directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		repo := gitrepo.New(cwd)
		if !repo.IsInitialized(ctx) {
			return fmt.Errorf("not in a git repository")
		}
		repoPath = repo.Path()
	}

	// Create git repo and workflow store
	repo := gitrepo.New(repoPath)
	wfStore, err := store.New(repo, cfg)
	if err != nil {
		return fmt.Errorf("failed to create workflow store: %w", err)
	}

	// Determine save options (draft vs workflow)
	saveOpts := store.SaveOptions{
		Commit: !opts.NoCommit,
	}
	if opts.Title != "" {
		saveOpts.Message = fmt.Sprintf("feat: Add workflow '%s'\n\n(fa-924)", opts.Title)
	} else {
		saveOpts.Message = "feat: Add workflow from shell history\n\n(fa-924)"
	}

	// Save workflow
	ref, err := wfStore.Save(ctx, wf, saveOpts)
	if err != nil {
		return fmt.Errorf("failed to save workflow: %w", err)
	}

	fmt.Printf("Created workflow: %s\n", ref.Path)

	return nil
}

// CommandsToSteps converts shell commands to workflow steps
// This is exported for use by the TUI when implemented
func CommandsToSteps(cmds []string) []workflows.Step {
	return app.CommandsToSteps(cmds)
}

// GenerateStepName creates a readable step name from a command
// This is exported for use by the TUI when implemented
func GenerateStepName(cmd string) string {
	return app.GenerateStepName(cmd)
}
