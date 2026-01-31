// Package cli provides Cobra command definitions for faire.
package cli

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/gitrepo"
	"github.com/chazuruo/svf/internal/tui"
	"github.com/chazuruo/svf/internal/workflows"
	"github.com/chazuruo/svf/internal/workflows/store"
)

// EditOptions contains the options for the edit command.
type EditOptions struct {
	ConfigPath string
	WorkflowID string
	OutputPath string
	NoCommit   bool
}

// NewEditCommand creates the edit command for creating/editing workflows.
func NewEditCommand() *cobra.Command {
	opts := &EditOptions{}

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Create or edit workflows using the TUI editor",
		Long: `Launch the terminal UI editor for creating and editing workflows.

The edit command opens a full-screen TUI editor where you can:
- Create new workflows
- Edit existing workflows
- Add, remove, and reorder steps
- Configure placeholders for user input
- Save workflows with automatic YAML generation

Examples:
  faire edit                    # Create a new workflow
  faire edit --workflow my-id   # Edit existing workflow by ID
  faire edit --output /path/save.yaml  # Save to specific path`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEdit(opts)
		},
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.Flags().StringVar(&opts.WorkflowID, "workflow", "", "workflow ID to edit (creates new if empty)")
	cmd.Flags().StringVar(&opts.OutputPath, "output", "", "output path for workflow.yaml")
	cmd.Flags().BoolVar(&opts.NoCommit, "no-commit", false, "skip git commit after saving")

	return cmd
}

func runEdit(opts *EditOptions) error {
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
	editedWf := finalEditor.GetWorkflow()

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
