// Package cli provides Cobra command definitions for svf.
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/chazuruo/svf/internal/history"
	"github.com/chazuruo/svf/internal/recorder"
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
	defer session.Cleanup()

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

	// TODO: Launch workflow editor with captured commands
	// For now, just show what was captured
	fmt.Printf("\nCaptured commands:\n")
	for i, cmd := range commands {
		fmt.Printf("  %d. %s\n", i+1, cmd.Command)
	}

	return nil
}

// parseCaptureFile parses the capture file and returns commands.
func parseCaptureFile(path string) ([]CapturedCommand, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open capture file: %w", err)
	}
	defer file.Close()

	// Parse the file - format is timestamp\x1Fcwd\x1Fcommand
	// For now, return empty since we haven't implemented the parser yet
	// The recorder package has the hook generation, but we need to add the parser

	return nil, nil
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
		// For now, just parse all
		commands, err = parser.Parse()
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

	// TODO: Launch history picker TUI when Bubble Tea integration is complete
	// picker := tui.NewHistoryPickerModel(commands)
	// p := tea.NewProgram(picker, tea.WithAltScreen())
	// finalModel, err := p.Run()

	// For now, just show what we got
	fmt.Printf("Found %d commands in %s history\n", len(commands), shell)
	fmt.Println("(TUI picker will be implemented in a future update)")

	return nil
}

// CapturedCommand represents a command captured during recording.
type CapturedCommand struct {
	Timestamp int64  `json:"timestamp"`
	CWD       string `json:"cwd"`
	Command   string `json:"command"`
}

// parseCaptureFileLine parses a single line from the capture file.
// Format: timestamp\x1Fcwd\x1Fcommand
func parseCaptureFileLine(line string) (CapturedCommand, error) {
	// Split by unit separator (0x1F)
	parts := splitByUnitSeparator(line)
	if len(parts) != 3 {
		return CapturedCommand{}, fmt.Errorf("invalid capture line format")
	}

	var ts int64
	fmt.Sscanf(parts[0], "%d", &ts)

	return CapturedCommand{
		Timestamp: ts,
		CWD:       parts[1],
		Command:   parts[2],
	}, nil
}

// splitByUnitSeparator splits a string by the ASCII unit separator (0x1F).
func splitByUnitSeparator(s string) []string {
	var result []string
	var current strings.Builder

	for _, r := range s {
		if r == '\x1F' {
			result = append(result, current.String())
			current.Reset()
		} else {
			current.WriteRune(r)
		}
	}

	// Add the last part
	result = append(result, current.String())

	return result
}

// getExitCode extracts the exit code from an exec.ExitError.
func getExitCode(err *exec.ExitError) int {
	if status, ok := err.Sys().(syscall.WaitStatus); ok {
		return status.ExitStatus()
	}
	return 1
}
