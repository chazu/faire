// Package cli provides Cobra command definitions for svf.
package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/gitrepo"
	"github.com/chazuruo/svf/internal/workflows"
	"github.com/chazuruo/svf/internal/workflows/store"
)

// ViewOptions contains the options for the view command.
type ViewOptions struct {
	ConfigPath string
	Raw        bool
	Markdown   bool
}

// NewViewCommand creates the view command.
func NewViewCommand() *cobra.Command {
	opts := &ViewOptions{}

	cmd := &cobra.Command{
		Use:   "view <workflow-ref>",
		Short: "View workflow details",
		Long: `Display detailed information about a workflow.

The workflow reference can be:
- A slug (e.g., "my-workflow")
- A path (e.g., "workflows/platform/chaz/my-workflow")
- An ID (e.g., "wf_abc123")

Output formats:
- Default: Formatted display
- --raw: Print raw YAML
- --md: Print generated Markdown`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runView(opts, args[0])
		},
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.Flags().BoolVar(&opts.Raw, "raw", false, "print raw YAML")
	cmd.Flags().BoolVar(&opts.Markdown, "md", false, "print Markdown")

	return cmd
}

func runView(opts *ViewOptions, workflowRef string) error {
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

	// Resolve workflow reference
	ref, err := resolveWorkflowRef(ctx, str, workflowRef)
	if err != nil {
		return err
	}

	// Load workflow
	wf, err := str.Load(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to load workflow: %w", err)
	}

	// Output
	if opts.Raw {
		return printWorkflowRaw(wf)
	}
	if opts.Markdown {
		return printWorkflowMarkdown(wf)
	}

	return printWorkflowFormatted(wf)
}

// resolveWorkflowRef resolves a workflow reference string to a WorkflowRef.
func resolveWorkflowRef(ctx context.Context, str store.Store, refStr string) (store.WorkflowRef, error) {
	// Try as ID first
	refs, err := str.List(ctx, store.Filter{})
	if err != nil {
		return store.WorkflowRef{}, err
	}

	// Look for exact match on ID or slug
	for _, ref := range refs {
		if ref.ID == refStr || ref.Slug == refStr {
			return ref, nil
		}
	}

	// Not found
	return store.WorkflowRef{}, fmt.Errorf("workflow not found: %s", refStr)
}

// printWorkflowRaw prints the raw YAML of a workflow.
func printWorkflowRaw(wf *workflows.Workflow) error {
	data, err := workflows.MarshalWorkflow(wf)
	if err != nil {
		return err
	}
	fmt.Print(string(data))
	return nil
}

// printWorkflowMarkdown prints a workflow as Markdown.
func printWorkflowMarkdown(wf *workflows.Workflow) error {
	var sb strings.Builder

	sb.WriteString("# ")
	sb.WriteString(wf.Title)
	sb.WriteString("\n\n")

	if wf.Description != "" {
		sb.WriteString(wf.Description)
		sb.WriteString("\n\n")
	}

	if len(wf.Tags) > 0 {
		sb.WriteString("**Tags:** ")
		sb.WriteString(strings.Join(wf.Tags, ", "))
		sb.WriteString("\n\n")
	}

	// Placeholders
	if len(wf.Placeholders) > 0 {
		sb.WriteString("## Parameters\n\n")
		for name, ph := range wf.Placeholders {
			sb.WriteString("- **<")
			sb.WriteString(name)
			sb.WriteString(">**")
			if ph.Prompt != "" {
				sb.WriteString(": ")
				sb.WriteString(ph.Prompt)
			}
			if ph.Default != "" {
				sb.WriteString(" (default: ")
				sb.WriteString(ph.Default)
				sb.WriteString(")")
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Steps
	sb.WriteString("## Steps\n\n")
	for i, step := range wf.Steps {
		sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, step.Name))
		sb.WriteString(fmt.Sprintf("   ```\n   %s\n   ```\n\n", step.Command))
	}

	fmt.Print(sb.String())
	return nil
}

// printWorkflowFormatted prints a workflow in formatted text.
func printWorkflowFormatted(wf *workflows.Workflow) error {
	fmt.Printf("Title: %s\n", wf.Title)
	if wf.Description != "" {
		fmt.Printf("Description: %s\n", wf.Description)
	}
	if len(wf.Tags) > 0 {
		fmt.Printf("Tags: %s\n", strings.Join(wf.Tags, ", "))
	}
	fmt.Printf("\nSteps:\n")
	for i, step := range wf.Steps {
		fmt.Printf("  %d. %s\n", i+1, step.Name)
		fmt.Printf("     %s\n", step.Command)
	}
	return nil
}
