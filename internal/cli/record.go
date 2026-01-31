// Package cli provides Cobra command definitions for gitsavvy.
package cli

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/chazuruo/faire/internal/config"
	"github.com/chazuruo/faire/internal/gitrepo"
	"github.com/chazuruo/faire/internal/recorder"
	"github.com/chazuruo/faire/internal/tui"
	"github.com/chazuruo/faire/internal/workflows"
	"github.com/chazuruo/faire/internal/workflows/store"
)

// RecordOptions contains the options for the record command.
type RecordOptions struct {
	ConfigPath string
	Title      string
	Desc       string
	Tags       string
	Shell      string
	CWD        string
	Draft      bool
	NoCommit   bool
	FromLog    string
}

// NewRecordCommand creates the record command for capturing shell sessions.
func NewRecordCommand() *cobra.Command {
	opts := &RecordOptions{}

	cmd := &cobra.Command{
		Use:   "record",
		Short: "Record a shell session and create a workflow",
		Long: `Record shell commands from an interactive session and create a workflow.

The record command spawns a subshell with hooks that capture all commands you run.
When you exit the shell, the captured commands are opened in the workflow editor
where you can review, edit, and save them as a workflow.

The recording indicator [REC] is shown in your prompt while recording.

Supported shells: bash, zsh

Exit codes:
  0  - Success
  13 - No commands captured
  10 - Git operation failed

Examples:
  gitsavvy record                    # Record with auto-detected shell
  gitsavvy record --shell zsh        # Record with zsh
  gitsavvy record --title "Fix bug"  # Set workflow title
  gitsavvy record --tags fix,urgent  # Add tags to workflow`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecord(opts)
		},
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.Flags().StringVar(&opts.Title, "title", "", "workflow title")
	cmd.Flags().StringVar(&opts.Desc, "desc", "", "workflow description")
	cmd.Flags().StringVar(&opts.Tags, "tags", "", "comma-separated tags")
	cmd.Flags().StringVar(&opts.Shell, "shell", "", "shell to use (bash, zsh, sh)")
	cmd.Flags().StringVar(&opts.CWD, "cwd", "", "working directory for session")
	cmd.Flags().BoolVar(&opts.Draft, "draft", false, "save as draft (skip validation)")
	cmd.Flags().BoolVar(&opts.NoCommit, "no-commit", false, "skip git commit after saving")
	cmd.Flags().StringVar(&opts.FromLog, "from-log", "", "import commands from existing log file")

	return cmd
}

func runRecord(opts *RecordOptions) error {
	ctx := context.Background()

	// Load config
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize repo
	repoPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	repo := gitrepo.New(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open git repo: %w", err)
	}

	// Create store
	str, err := store.New(repo, cfg)
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}

	// Determine shell to use
	shell := opts.Shell
	if shell == "" {
		shell = recorder.DetectShell()
	}

	// Create recorder
	rec, err := recorder.NewRecorder(shell)
	if err != nil {
		if err == recorder.ErrUnsupportedShell {
			return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, sh)", shell)
		}
		return fmt.Errorf("failed to create recorder: %w", err)
	}

	// Set CWD if specified
	if opts.CWD != "" {
		rec.SetCWD(opts.CWD)
	}

	// Record session
	commands, err := rec.StartRecordingSession(ctx)
	if err != nil {
		if err == recorder.ErrNoCommandsCaptured {
			fmt.Println("⚠️  No commands were captured during the session.")
			os.Exit(recorder.ExitCodeNothingCaptured)
			return nil
		}
		return fmt.Errorf("recording session failed: %w", err)
	}

	fmt.Printf("\n✅ Captured %d command(s)\n", len(commands))

	// Convert commands to workflow steps
	shellForWorkflow := shell
	if shellForWorkflow == "" {
		shellForWorkflow = "bash"
	}

	// Create workflow from captured commands
	wf := &workflows.Workflow{
		SchemaVersion: workflows.SchemaVersion,
		Title:         opts.Title,
		Description:   opts.Desc,
		Tags:          parseTags(opts.Tags),
		Defaults: workflows.Defaults{
			Shell: shellForWorkflow,
		},
		Placeholders: map[string]workflows.Placeholder{},
		Steps:        make([]workflows.Step, 0, len(commands)),
	}

	for _, cmd := range commands {
		step := workflows.Step{
			Command: cmd.Command,
		}
		if cmd.CWD != "" && cmd.CWD != repoPath {
			step.CWD = cmd.CWD
		}
		wf.Steps = append(wf.Steps, step)
	}

	// Apply defaults to steps
	for i := range wf.Steps {
		wf.ApplyDefaults(&wf.Steps[i])
	}

	// Launch TUI editor for review
	editor := tui.NewWorkflowEditor(ctx, wf)
	p := tea.NewProgram(editor, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	finalEditor := finalModel.(tui.WorkflowEditorModel)

	// Handle quit without save
	if finalEditor.DidQuit() {
		fmt.Println("Quit without saving.")
		return nil
	}

	// Get the edited workflow
	editedWf := finalEditor.GetWorkflow()

	// Validate workflow (unless --draft)
	if !opts.Draft {
		if err := editedWf.Validate(); err != nil {
			return fmt.Errorf("workflow validation failed: %w", err)
		}
	}

	// Save workflow
	saveOpts := store.SaveOptions{
		Commit: !opts.NoCommit,
	}

	ref, err := str.Save(ctx, editedWf, saveOpts)
	if err != nil {
		return fmt.Errorf("failed to save workflow: %w", err)
	}

	fmt.Printf("✅ Workflow saved: %s (id: %s)\n", ref.Slug, ref.ID)
	return nil
}

// parseTags parses a comma-separated string of tags into a slice.
func parseTags(tagsStr string) []string {
	if tagsStr == "" {
		return nil
	}

	tags := make([]string, 0)
	for _, tag := range splitTags(tagsStr) {
		tag = trimSpace(tag)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

// splitTags splits a comma-separated string.
func splitTags(s string) []string {
	var result []string
	current := ""

	for _, r := range s {
		if r == ',' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}

// trimSpace removes leading and trailing whitespace from a string.
func trimSpace(s string) string {
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}

	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	return s[start:end]
}
