// Package cli provides Cobra command definitions for faire.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/chazuruo/faire/internal/config"
	"github.com/chazuruo/faire/internal/gitrepo"
	"github.com/chazuruo/faire/internal/tui"
	"github.com/chazuruo/faire/internal/workflows"
	"github.com/chazuruo/faire/internal/workflows/store"
)

// EditOptions contains the options for the edit command.
type EditOptions struct {
	ConfigPath string
	WorkflowID string
	OutputPath string
	NoCommit   bool
	TUI        bool
	NoTUI      bool   // For LLM automation
	InputFile  string // For --no-tui mode
}

// NewEditCommand creates the edit command for creating/editing workflows.
func NewEditCommand() *cobra.Command {
	opts := &EditOptions{}

	cmd := &cobra.Command{
		Use:   "edit [workflow-ref]",
		Short: "Create or edit workflows using the TUI or external editor",
		Long: `Create or edit workflows using the TUI editor or external editor (EDITOR env var).

The edit command opens a workflow for editing:
- With --tui: Uses the terminal UI editor
- Without --tui: Opens in your $EDITOR (vi, nano, code, etc.)

The workflow reference can be:
- A workflow ID (ULID) or slug
- Omitted to create a new workflow

In non-TUI mode (--no-tui), you can import workflows from YAML files:
- Use --file to specify a YAML file to import
- Use --output to save to a specific path
- Use --no-commit to skip automatic git commit

Examples:
  faire edit                    # Create a new workflow (TUI mode)
  faire edit --workflow my-id   # Edit existing workflow by ID (TUI mode)
  faire edit --output /path/save.yaml  # Save to specific path (TUI mode)
  faire edit --no-tui --file workflow.yaml  # Import from file (non-TUI)
  cat workflow.yaml | faire edit --no-tui  # Import from stdin (non-TUI)`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.WorkflowID = args[0]
			}
			return runEdit(opts)
		},
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.Flags().StringVar(&opts.OutputPath, "output", "", "output path for workflow.yaml")
	cmd.Flags().StringVar(&opts.InputFile, "file", "", "input YAML file (for --no-tui mode)")
	cmd.Flags().BoolVar(&opts.NoCommit, "no-commit", false, "skip git commit after saving")
	cmd.Flags().BoolVar(&opts.NoTUI, "no-tui", false, "disable TUI/interactive mode (use with --file)")
	cmd.Flags().BoolVar(&opts.TUI, "tui", true, "use TUI editor (default true)")

	return cmd
}

func runEdit(opts *EditOptions) error {
	// Check for --no-tui mode first
	if IsNoTUI() || opts.NoTUI {
		return runEditNonInteractive(opts)
	}
	return runEditInteractive(opts)
}

// runEditInteractive runs the TUI editor.
func runEditInteractive(opts *EditOptions) error {
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

	// Load or create workflow
	var wf *workflows.Workflow
	if opts.WorkflowID != "" {
		// Load existing workflow
		refs, err := str.List(ctx, store.Filter{})
		if err != nil {
			return fmt.Errorf("failed to list workflows: %w", err)
		}

		var ref store.WorkflowRef
		for _, r := range refs {
			if r.ID == opts.WorkflowID || r.Slug == opts.WorkflowID {
				ref = r
				break
			}
		}

		if ref.ID == "" {
			return fmt.Errorf("workflow not found: %s", opts.WorkflowID)
		}

		wf, err = str.Load(ctx, ref)
		if err != nil {
			return fmt.Errorf("failed to load workflow: %w", err)
		}
	} else {
		// Create new workflow
		wf = &workflows.Workflow{
			SchemaVersion: workflows.SchemaVersion,
			Title:         "",
			Description:   "",
			Tags:          []string{},
			Defaults:      workflows.Defaults{},
			Placeholders:  map[string]workflows.Placeholder{},
			Steps:         []workflows.Step{},
		}
	}

	var editedWf *workflows.Workflow

	// Choose editor based on flag
	if opts.TUI {
		// Launch TUI editor
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
		editedWf = finalEditor.GetWorkflow()
	} else {
		// Launch external editor
		editedWf, err = launchExternalEditor(wf)
		if err != nil {
			return fmt.Errorf("editor failed: %w", err)
		}
	}

	// Validate workflow
	if err := editedWf.Validate(); err != nil {
		return fmt.Errorf("workflow validation failed: %w", err)
	}

	// Save workflow
	saveOpts := store.SaveOptions{
		Commit: !opts.NoCommit,
	}

	if opts.OutputPath != "" {
		// Save to specific path
		if err := saveWorkflowToPath(editedWf, opts.OutputPath); err != nil {
			return fmt.Errorf("failed to save workflow: %w", err)
		}
		fmt.Printf("Workflow saved to: %s\n", opts.OutputPath)
	} else {
		// Save using store
		ref, err := str.Save(ctx, editedWf, saveOpts)
		if err != nil {
			return fmt.Errorf("failed to save workflow: %w", err)
		}
		fmt.Printf("Workflow saved: %s (id: %s)\n", ref.Slug, ref.ID)
	}

	return nil
}

// saveWorkflowToPath saves a workflow to a specific file path.
func saveWorkflowToPath(wf *workflows.Workflow, path string) error {
	data, err := workflows.MarshalWorkflow(wf)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write workflow: %w", err)
	}

	return nil
}

// runEditNonInteractive runs edit in non-TUI mode for LLM automation.
func runEditNonInteractive(opts *EditOptions) error {
	ctx := context.Background()

	// Load config
	cfg, err := config.LoadWithDefaults()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Open repo
	repo := gitrepo.New(cfg.Repo.Path)
	if !repo.IsInitialized(ctx) {
		return fmt.Errorf("repository not initialized. Run 'faire init' first")
	}

	// Create store
	str, err := store.New(repo, cfg)
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}

	// Load workflow from file or stdin
	var wf *workflows.Workflow
	if opts.InputFile != "" {
		// Load from file
		data, err := os.ReadFile(opts.InputFile)
		if err != nil {
			return fmt.Errorf("failed to read input file: %w", err)
		}
		wf, err = workflows.UnmarshalWorkflow(data)
		if err != nil {
			return fmt.Errorf("failed to parse workflow: %w", err)
		}
	} else {
		// Read from stdin
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read stdin: %w", err)
		}
		if len(data) == 0 {
			return fmt.Errorf("no input provided (use --file or pipe via stdin)")
		}
		wf, err = workflows.UnmarshalWorkflow(data)
		if err != nil {
			return fmt.Errorf("failed to parse workflow: %w", err)
		}
	}

	// Validate workflow
	if err := wf.Validate(); err != nil {
		return fmt.Errorf("workflow validation failed: %w", err)
	}

	// Save workflow
	saveOpts := store.SaveOptions{
		Commit: !opts.NoCommit,
	}

	if opts.OutputPath != "" {
		// Save to specific path
		if err := saveWorkflowToPath(wf, opts.OutputPath); err != nil {
			return fmt.Errorf("failed to save workflow: %w", err)
		}
		fmt.Printf("Workflow saved to: %s\n", opts.OutputPath)
	} else {
		// Save using store
		ref, err := str.Save(ctx, wf, saveOpts)
		if err != nil {
			return fmt.Errorf("failed to save workflow: %w", err)
		}
		fmt.Printf("Workflow saved: %s (id: %s)\n", ref.Slug, ref.ID)
	}

	return nil
}

// launchExternalEditor launches an external editor to edit the workflow.
func launchExternalEditor(wf *workflows.Workflow) (*workflows.Workflow, error) {
	// Create temp file
	tmp, err := os.CreateTemp("", "faire-edit-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	// Write workflow to temp file
	data, err := workflows.MarshalWorkflow(wf)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workflow: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}

	tmp.Close()

	// Get editor from environment
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	// Get modified time before editing
	infoBefore, err := os.Stat(tmp.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to stat temp file: %w", err)
	}

	// Launch editor
	fmt.Printf("Launching %s...\n", editor)
	if err := launchEditor(editor, tmp.Name()); err != nil {
		return nil, fmt.Errorf("editor failed: %w", err)
	}

	// Check if file was modified
	infoAfter, err := os.Stat(tmp.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to stat temp file: %w", err)
	}

	// If not modified, return original
	if infoAfter.ModTime().Equal(infoBefore.ModTime()) {
		fmt.Println("No changes made.")
		return wf, nil
	}

	// Read back and validate
	updatedData, err := os.ReadFile(tmp.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read temp file: %w", err)
	}

	var updated workflows.Workflow
	if err := yaml.Unmarshal(updatedData, &updated); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	if err := updated.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &updated, nil
}

// launchEditor launches the editor with the given file.
func launchEditor(editor, path string) error {
	// For VS Code and similar GUI editors, we need to wait
	// This is a simplified implementation
	// In production, you'd handle this more carefully

	cmd := exec.Command("sh", "-c", editor+" "+path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
