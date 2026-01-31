// Package cli provides Cobra command definitions for svf.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/gitrepo"
	"github.com/chazuruo/svf/internal/history"
	"github.com/chazuruo/svf/internal/recorder"
	"github.com/chazuruo/svf/internal/tui"
	"github.com/chazuruo/svf/internal/workflows"
	"github.com/chazuruo/svf/internal/workflows/store"
)

// RecordOptions contains the options for the record command.
type RecordOptions struct {
	ConfigPath string
	Shell      string
	Title      string
	Desc       string
	Tags       string
	Identity   string
	Draft      bool
	NoCommit   bool
	NoTUI      bool
}

// NewRecordCommand creates the record command.
func NewRecordCommand() *cobra.Command {
	opts := &RecordOptions{}

	cmd := &cobra.Command{
		Use:   "record",
		Short: "Record a shell session to create a workflow",
		Long: `Record a shell session and capture commands to create a workflow.

The record command launches a subshell with command capture hooks enabled.
All commands executed in the shell are captured and presented in a workflow
editor for review, selection, and saving.

Example:
  svf record          # Start recording session
  svf record --shell zsh   # Use specific shell`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecord(opts)
		},
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.Flags().StringVar(&opts.Shell, "shell", "", "shell to use (bash/zsh, default: auto-detect)")
	cmd.Flags().StringVar(&opts.Title, "title", "", "workflow title")
	cmd.Flags().StringVar(&opts.Desc, "desc", "", "workflow description")
	cmd.Flags().StringVar(&opts.Tags, "tags", "", "workflow tags (comma-separated)")
	cmd.Flags().StringVar(&opts.Identity, "identity", "", "identity path override")
	cmd.Flags().BoolVar(&opts.Draft, "draft", false, "save as draft (don't commit)")
	cmd.Flags().BoolVar(&opts.NoCommit, "no-commit", false, "skip git commit after saving")

	return cmd
}

func runRecord(opts *RecordOptions) error {
	// Create recording session
	session, err := recorder.NewSession(opts.Shell)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer func() { _ = session.Cleanup() }()

	// Get shell arguments
	shellArgs, err := session.GetShellArgs()
	if err != nil {
		return fmt.Errorf("failed to get shell args: %w", err)
	}

	// Build command
	cmd := exec.Command(session.Shell, shellArgs...)

	// Set environment
	cmd.Env = append(os.Environ(), session.GetEnv()...)

	// Attach stdio for interactive use
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Print instructions
	fmt.Printf("🎬 Recording session started\n")
	fmt.Printf("   Shell: %s\n", session.Shell)
	fmt.Printf("   Capture file: %s\n", session.CaptureFile)
	fmt.Printf("   Session ID: %s\n\n", session.SessionID)
	fmt.Printf("Type 'exit' or Ctrl+D to finish recording\n\n")

	// Note about prompt indicator
	fmt.Printf("Your prompt will show [REC] while recording.\n\n")

	// Run shell (blocks until exit)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Shell exited with some code - this is normal
			_ = exitErr
		} else {
			return fmt.Errorf("shell error: %w", err)
		}
	}

	fmt.Printf("\n\n🎬 Recording session ended\n\n")

	// Parse capture file
	commands, err := parseCaptureFile(session.CaptureFile)
	if err != nil {
		return err
	}

	if len(commands) == 0 {
		fmt.Printf("No commands captured.\n")
		return nil
	}

	fmt.Printf("Captured %d command(s)\n", len(commands))

	// Check for --no-tui mode
	if opts.NoTUI || IsNoTUI() {
		// Non-TUI mode: show all commands and exit
		fmt.Printf("\nCaptured commands:\n")
		for i, cmd := range commands {
			fmt.Printf("  %d. %s\n", i+1, cmd.Command)
		}
		return nil
	}

	// Convert captured commands to a workflow
	workflow := commandsToWorkflow(commands, opts.Title, opts.Desc, opts.Tags)

	// Launch workflow editor
	ctx := context.Background()
	editor := tui.NewWorkflowEditor(ctx, workflow)

	p := tea.NewProgram(&editor, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run editor: %w", err)
	}

	// Get the final model
	finalEditor, ok := finalModel.(*tui.WorkflowEditorModel)
	if !ok {
		return fmt.Errorf("unexpected model type from editor")
	}

	// Check if user saved or quit
	if finalEditor.DidQuit() && !finalEditor.DidSave() {
		if finalEditor.IsDirty() {
			fmt.Printf("Workflow discarded without saving.\n")
		}
		return nil
	}

	// Get the edited workflow
	finalWorkflow := finalEditor.GetWorkflow()

	// TODO: Save workflow to file
	// For now, just show what was saved
	fmt.Printf("\nWorkflow saved with %d steps:\n", len(finalWorkflow.Steps))
	fmt.Printf("Title: %s\n", finalWorkflow.Title)
	if finalWorkflow.Description != "" {
		fmt.Printf("Description: %s\n", finalWorkflow.Description)
	}
	if len(finalWorkflow.Tags) > 0 {
		fmt.Printf("Tags: %s\n", strings.Join(finalWorkflow.Tags, ", "))
	}

	return nil
}

// parseCaptureFile parses the capture file and returns commands.
func parseCaptureFile(path string) ([]CapturedCommand, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open capture file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Read the entire file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read capture file: %w", err)
	}

	// Parse the file - format is timestamp\x1Fcwd\x1Fcommand
	var commands []CapturedCommand
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Split by unit separator (0x1F)
		parts := strings.Split(line, "\x1F")
		if len(parts) != 3 {
			continue
		}

		// Parse timestamp
		var timestamp int64
		_, err := fmt.Sscanf(parts[0], "%d", &timestamp)
		if err != nil {
			continue
		}

		cmd := CapturedCommand{
			Timestamp: timestamp,
			CWD:       parts[1],
			Command:   parts[2],
		}
		commands = append(commands, cmd)
	}

	return commands, nil
}

// RecordHistoryOptions contains the options for the record history command.
type RecordHistoryOptions struct {
	ConfigPath string
	Shell      string
	Limit      int
	Since      string
	Title      string
	Desc       string
	Tags       string
	Identity   string
	Draft      bool
	NoCommit   bool
}

// NewRecordHistoryCommand creates the record history command.
func NewRecordHistoryCommand() *cobra.Command {
	opts := &RecordHistoryOptions{}

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Pick commands from shell history to create a workflow",
		Long: `Pick commands from your shell history to create a workflow.

Loads your shell history and presents a TUI for selecting commands.
Selected commands are converted into workflow steps.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecordHistory(opts)
		},
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.Flags().StringVar(&opts.Shell, "shell", "", "shell to use (bash/zsh, default: auto-detect)")
	cmd.Flags().IntVar(&opts.Limit, "limit", 500, "maximum number of history entries to load")
	cmd.Flags().StringVar(&opts.Since, "since", "", "only show commands since duration (e.g., 1h, 1d)")
	cmd.Flags().StringVar(&opts.Title, "title", "", "workflow title")
	cmd.Flags().StringVar(&opts.Desc, "desc", "", "workflow description")
	cmd.Flags().StringVar(&opts.Tags, "tags", "", "workflow tags (comma-separated)")
	cmd.Flags().StringVar(&opts.Identity, "identity", "", "identity path override")
	cmd.Flags().BoolVar(&opts.Draft, "draft", false, "save as draft (don't commit)")
	cmd.Flags().BoolVar(&opts.NoCommit, "no-commit", false, "skip git commit after saving")

	return cmd
}

func runRecordHistory(opts *RecordHistoryOptions) error {
	ctx := context.Background()

	// Detect shell if not specified
	shell := opts.Shell
	if shell == "" {
		shell = history.DetectShell()
	}

	// Create parser
	parser := history.NewParser(shell, opts.Limit)

	// Parse history
	var commands []history.Command
	var err error

	if opts.Since != "" {
		// Parse duration (e.g., "1h", "1d")
		duration, err := time.ParseDuration(opts.Since)
		if err != nil {
			return fmt.Errorf("failed to parse duration %q: %w", opts.Since, err)
		}
		commands, err = parser.ParseSince(duration)
	} else {
		commands, err = parser.Parse()
	}

	if err != nil {
		return fmt.Errorf("failed to parse history: %w", err)
	}

	if len(commands) == 0 {
		fmt.Printf("No commands found in shell history.\n")
		return nil
	}

	// Check for --no-tui mode
	if IsNoTUI() {
		// Non-TUI mode: show all commands and exit
		for i, cmd := range commands {
			fmt.Printf("%d. %s\n", i+1, cmd.Command)
		}
		return nil
	}

	// Launch history picker TUI
	picker := tui.NewHistoryPickerModel(commands)
	p := tea.NewProgram(picker, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	finalPicker := finalModel.(tui.HistoryPickerModel)

	// Handle quit without selection
	if finalPicker.DidQuit() {
		fmt.Println("Quit without selecting commands.")
		return nil
	}

	// Get selected commands
	selectedCommands := finalPicker.GetSelectedCommands()
	if len(selectedCommands) == 0 {
		fmt.Println("No commands selected.")
		return nil
	}

	fmt.Printf("Selected %d command(s)\n", len(selectedCommands))

	// Convert commands to workflow steps
	steps := convertHistoryCommandsToSteps(selectedCommands)

	// Create workflow
	wf := &workflows.Workflow{
		SchemaVersion: workflows.SchemaVersion,
		Title:         opts.Title,
		Description:   opts.Desc,
		Tags:          parseTags(opts.Tags),
		Steps:         steps,
	}

	// If no title provided, use default
	if wf.Title == "" {
		wf.Title = fmt.Sprintf("Workflow from %s history", shell)
	}

	// Load config for saving
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

	// Save workflow
	saveOpts := store.SaveOptions{
		Commit: !opts.NoCommit && !opts.Draft,
	}

	ref, err := str.Save(ctx, wf, saveOpts)
	if err != nil {
		return fmt.Errorf("failed to save workflow: %w", err)
	}

	fmt.Printf("Workflow saved: %s (id: %s)\n", ref.Slug, ref.ID)
	return nil
}

// convertHistoryCommandsToSteps converts history commands to workflow steps.
func convertHistoryCommandsToSteps(commands []history.Command) []workflows.Step {
	steps := make([]workflows.Step, len(commands))

	for i, cmd := range commands {
		stepName := fmt.Sprintf("Step %d", i+1)

		// Try to extract a meaningful name from the command
		parts := strings.Fields(cmd.Command)
		if len(parts) > 0 {
			// Use the first word (command name) as part of the step name
			stepName = fmt.Sprintf("%s %d", parts[0], i+1)
		}

		steps[i] = workflows.Step{
			Name:    stepName,
			Command: cmd.Command,
			CWD:     cmd.CWD,
			Shell:   cmd.Shell,
		}
	}

	return steps
}

// CapturedCommand represents a command captured during recording.
type CapturedCommand struct {
	Timestamp int64  `json:"timestamp"`
	CWD       string `json:"cwd"`
	Command   string `json:"command"`
}

// commandsToWorkflow converts captured commands to a workflow.
func commandsToWorkflow(commands []CapturedCommand, title, desc, tagsStr string) *workflows.Workflow {
	// Create workflow
	wf := &workflows.Workflow{
		SchemaVersion: workflows.SchemaVersion,
		Title:         title,
		Description:   desc,
		Tags:          parseTags(tagsStr),
		Steps:         make([]workflows.Step, 0, len(commands)),
	}

	// Set default title if not provided
	if wf.Title == "" {
		wf.Title = "Recorded Workflow"
	}

	// Convert commands to steps
	for i, cmd := range commands {
		// Skip empty commands
		if strings.TrimSpace(cmd.Command) == "" {
			continue
		}

		step := workflows.Step{
			Name:    fmt.Sprintf("Step %d", i+1),
			Command: cmd.Command,
			CWD:     cmd.CWD,
		}

		// Use command as name if it's short enough
		if len(cmd.Command) <= 50 {
			step.Name = cmd.Command
		} else {
			step.Name = fmt.Sprintf("%.50s...", cmd.Command)
		}

		wf.Steps = append(wf.Steps, step)
	}

	return wf
}

// parseTags parses a comma-separated string into a slice of tags.
func parseTags(s string) []string {
	if s == "" {
		return nil
	}

	tags := strings.Split(s, ",")
	var result []string
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			result = append(result, tag)
		}
	}
	return result
}
