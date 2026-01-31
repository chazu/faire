// Package cli provides Cobra command definitions for faire.
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chazuruo/faire/internal/config"
	"github.com/chazuruo/faire/internal/gitrepo"
	"github.com/chazuruo/faire/internal/workflows"
	"github.com/chazuruo/faire/internal/workflows/store"
)

// ViewOptions contains the options for the view command.
type ViewOptions struct {
	ConfigPath string
	Raw        bool
	MD         bool
}

// NewViewCommand creates the view command for viewing workflows.
func NewViewCommand() *cobra.Command {
	opts := &ViewOptions{}

	cmd := &cobra.Command{
		Use:   "view <workflow-ref>",
		Short: "View a workflow's details",
		Long: `View a workflow's details with optional formatting.

The workflow reference can be:
- A workflow ID (ULID)
- A workflow slug
- A path to a workflow.yaml file

Examples:
  faire view restart-service     # View by slug
  faire view --raw restart-service    # View raw YAML
  faire view --md restart-service     # View as markdown`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runView(opts, args[0])
		},
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.Flags().BoolVar(&opts.Raw, "raw", false, "print raw YAML")
	cmd.Flags().BoolVar(&opts.MD, "md", false, "print markdown")

	return cmd
}

func runView(opts *ViewOptions, refStr string) error {
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

	// Resolve reference
	ref, err := resolveRef(str, ctx, refStr)
	if err != nil {
		return fmt.Errorf("failed to resolve workflow reference: %w", err)
	}

	// Load workflow
	wf, err := str.Load(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to load workflow: %w", err)
	}

	// Output based on format
	switch {
	case opts.Raw:
		return printRaw(wf)
	case opts.MD:
		return printMarkdown(wf)
	default:
		return printRendered(wf)
	}
}

// resolveRef resolves a workflow reference string to a WorkflowRef.
func resolveRef(str store.Store, ctx context.Context, refStr string) (store.WorkflowRef, error) {
	// Check if it's a file path
	if strings.Contains(refStr, string("/")) && strings.HasSuffix(refStr, ".yaml") {
		return store.WorkflowRef{Path: refStr}, nil
	}

	// Try to find by listing and matching
	refs, err := str.List(ctx, store.Filter{})
	if err != nil {
		return store.WorkflowRef{}, err
	}

	// First try exact ID match
	for _, ref := range refs {
		if ref.ID == refStr {
			return ref, nil
		}
	}

	// Then try slug match
	for _, ref := range refs {
		if ref.Slug == refStr {
			return ref, nil
		}
	}

	return store.WorkflowRef{}, fmt.Errorf("workflow not found: %s", refStr)
}

// printRaw prints the workflow as raw YAML.
func printRaw(wf *workflows.Workflow) error {
	data, err := workflows.MarshalWorkflow(wf)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

// printMarkdown prints the workflow as markdown.
func printMarkdown(wf *workflows.Workflow) error {
	var b strings.Builder

	b.WriteString("# ")
	b.WriteString(wf.Title)
	b.WriteString("\n\n")

	if wf.Description != "" {
		b.WriteString(wf.Description)
		b.WriteString("\n\n")
	}

	if len(wf.Tags) > 0 {
		b.WriteString("## Tags\n\n")
		for _, tag := range wf.Tags {
			b.WriteString("- ")
			b.WriteString(tag)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(wf.Placeholders) > 0 {
		b.WriteString("## Placeholders\n\n")
		for name, ph := range wf.Placeholders {
			b.WriteString("### ")
			b.WriteString(name)
			b.WriteString("\n\n")
			if ph.Prompt != "" {
				b.WriteString("Prompt: ")
				b.WriteString(ph.Prompt)
				b.WriteString("\n\n")
			}
			if ph.Default != "" {
				b.WriteString("Default: `")
				b.WriteString(ph.Default)
				b.WriteString("`\n\n")
			}
			if ph.Validate != "" {
				b.WriteString("Validation: `")
				b.WriteString(ph.Validate)
				b.WriteString("`\n\n")
			}
			if ph.Secret {
				b.WriteString("**Secret**\n\n")
			}
		}
	}

	if len(wf.Steps) > 0 {
		b.WriteString("## Steps\n\n")
		for i, step := range wf.Steps {
			b.WriteString("### ")
			if step.Name != "" {
				b.WriteString(step.Name)
			} else {
				b.WriteString(fmt.Sprintf("Step %d", i+1))
			}
			b.WriteString("\n\n")

			b.WriteString("```bash\n")
			b.WriteString(step.Command)
			b.WriteString("\n```\n\n")

			if step.Shell != "" {
				b.WriteString(fmt.Sprintf("**Shell:** `%s`\n\n", step.Shell))
			}
			if step.CWD != "" {
				b.WriteString(fmt.Sprintf("**Working Directory:** `%s`\n\n", step.CWD))
			}
			if step.ContinueOnError {
				b.WriteString("**Continues on error:** Yes\n\n")
			}
		}
	}

	fmt.Println(b.String())
	return nil
}

// printRendered prints the workflow with formatted output.
func printRendered(wf *workflows.Workflow) error {
	// Title
	fmt.Printf("%s\n", strings.Repeat("=", len(wf.Title)+4))
	fmt.Printf("  %s  \n", wf.Title)
	fmt.Printf("%s\n\n", strings.Repeat("=", len(wf.Title)+4))

	// Description
	if wf.Description != "" {
		fmt.Printf("%s\n\n", wf.Description)
	}

	// Metadata
	fmt.Println("Metadata:")
	fmt.Printf("  ID: %s\n", wf.ID)
	fmt.Printf("  Schema Version: %d\n", wf.SchemaVersion)

	// Tags
	if len(wf.Tags) > 0 {
		fmt.Printf("  Tags: %s\n", strings.Join(wf.Tags, ", "))
	}
	fmt.Println()

	// Defaults
	if wf.Defaults.Shell != "" {
		fmt.Printf("Default Shell: %s\n", wf.Defaults.Shell)
	}
	if wf.Defaults.CWD != "" {
		fmt.Printf("Default Working Directory: %s\n", wf.Defaults.CWD)
	}
	if wf.Defaults.ConfirmEachStep != nil {
		fmt.Printf("Confirm Each Step: %v\n", *wf.Defaults.ConfirmEachStep)
	}
	if wf.Defaults.Shell != "" || wf.Defaults.CWD != "" || wf.Defaults.ConfirmEachStep != nil {
		fmt.Println()
	}

	// Placeholders
	if len(wf.Placeholders) > 0 {
		fmt.Println("Placeholders:")
		for name, ph := range wf.Placeholders {
			fmt.Printf("  %s:", name)
			if ph.Prompt != "" {
				fmt.Printf(" prompt=%q", ph.Prompt)
			}
			if ph.Default != "" {
				fmt.Printf(" default=%q", ph.Default)
			}
			if ph.Secret {
				fmt.Printf(" [secret]")
			}
			fmt.Println()
		}
		fmt.Println()
	}

	// Steps
	fmt.Println("Steps:")
	for i, step := range wf.Steps {
		name := step.Name
		if name == "" {
			name = fmt.Sprintf("Step %d", i+1)
		}

		fmt.Printf("\n  %s:\n", name)
		fmt.Printf("    Command: %s\n", step.Command)

		if step.Shell != "" {
			fmt.Printf("    Shell: %s\n", step.Shell)
		}
		if step.CWD != "" {
			fmt.Printf("    CWD: %s\n", step.CWD)
		}
		if step.ContinueOnError {
			fmt.Printf("    Continue on Error: true\n")
		}
		if len(step.Env) > 0 {
			fmt.Printf("    Environment Variables:\n")
			for k, v := range step.Env {
				fmt.Printf("      %s=%s\n", k, v)
			}
		}
	}
	fmt.Println()

	return nil
}
