// Package cli provides Cobra command definitions for svf.
package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/export"
	"github.com/chazuruo/svf/internal/gitrepo"
	"github.com/chazuruo/svf/internal/workflows/store"
)

// ExportOptions contains the options for the export command.
type ExportOptions struct {
	ConfigPath     string
	Format         string
	Out            string
	UpdateReadme   bool
	CustomTemplate string
}

// NewExportCommand creates the export command.
func NewExportCommand() *cobra.Command {
	opts := &ExportOptions{}

	cmd := &cobra.Command{
		Use:   "export <workflow-ref>",
		Short: "Export workflow to various formats",
		Long: `Export a workflow to different formats with optional custom templates.

The workflow reference can be:
- A slug (e.g., "my-workflow")
- A path (e.g., "workflows/platform/chaz/my-workflow")
- An ID (e.g., "wf_abc123")

Supported formats:
- md (default): Markdown
- yaml: YAML format
- json: JSON format

Template locations (searched in order):
1. .svf/templates/export.<format> (repo-specific)
2. ~/.config/svf/templates/export.<format> (user-specific)
3. Built-in templates

Examples:
  svf export my-workflow                    # Export as Markdown to stdout
  svf export my-workflow --format json      # Export as JSON
  svf export my-workflow --out output.md    # Export to file
  svf export my-workflow --update-readme    # Update README.md
  svf export my-workflow --template custom.tmpl`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExport(opts, args[0])
		},
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.Flags().StringVarP(&opts.Format, "format", "f", "md", "output format (md, yaml, json)")
	cmd.Flags().StringVarP(&opts.Out, "out", "o", "-", "output path (default: stdout)")
	cmd.Flags().BoolVarP(&opts.UpdateReadme, "update-readme", "u", false, "update README.md with exported content")
	cmd.Flags().StringVarP(&opts.CustomTemplate, "template", "t", "", "custom template file")

	return cmd
}

func runExport(opts *ExportOptions, workflowRef string) error {
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

	// Parse format
	format := export.Format(opts.Format)
	if format != export.FormatMarkdown && format != export.FormatYAML && format != export.FormatJSON {
		return fmt.Errorf("invalid format: %s (must be md, yaml, or json)", opts.Format)
	}

	// Determine output path
	outPath := opts.Out
	if outPath == "-" {
		outPath = "" // Empty means stdout
	}

	// Create exporter
	exporter, err := export.NewExporter(export.Options{
		Format:         format,
		Out:            outPath,
		UpdateReadme:   opts.UpdateReadme,
		CustomTemplate: opts.CustomTemplate,
		RepoPath:       cfg.Repo.Path,
	})
	if err != nil {
		return fmt.Errorf("failed to create exporter: %w", err)
	}

	// Handle --update-readme flag
	if opts.UpdateReadme {
		if err := exporter.UpdateReadme(wf, cfg.Repo.Path); err != nil {
			return fmt.Errorf("failed to update README: %w", err)
		}
		fmt.Printf("Updated README.md with workflow: %s\n", wf.Title)
		return nil
	}

	// Export workflow
	output, err := exporter.Export(wf)
	if err != nil {
		return fmt.Errorf("failed to export workflow: %w", err)
	}

	// Write output
	if opts.Out == "-" || opts.Out == "" {
		fmt.Print(output)
	} else {
		fmt.Printf("Exported workflow to: %s\n", opts.Out)
	}

	return nil
}
