// Package cli provides Cobra command definitions for faire.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chazuruo/faire/internal/config"
	"github.com/chazuruo/faire/internal/gitrepo"
	"github.com/chazuruo/faire/internal/workflows/store"
	"github.com/spf13/cobra"
)

// OutputFormat defines the output format for the list command.
type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
	FormatPlain OutputFormat = "plain"
)

// ListOptions contains the options for the list command.
type ListOptions struct {
	ConfigPath string
	Mine       bool
	Shared     bool
	Tags       []string
	Format     string
}

// NewListCommand creates the list command for listing workflows.
func NewListCommand() *cobra.Command {
	opts := &ListOptions{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workflows with optional filtering",
		Long: `List all available workflows with filtering options.

Workflows can be filtered by:
- --mine: Only show workflows under your identity path
- --shared: Only show shared workflows
- --tag: Filter by tag (can be specified multiple times)
- --format: Output format (table, json, plain)

Examples:
  faire list                  # List all workflows in table format
  faire list --mine           # List only my workflows
  faire list --tag ops        # List workflows tagged with 'ops'
  faire list --format json    # List workflows in JSON format`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(opts)
		},
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.Flags().BoolVar(&opts.Mine, "mine", false, "only show workflows under your identity path")
	cmd.Flags().BoolVar(&opts.Shared, "shared", false, "only show shared workflows")
	cmd.Flags().StringSliceVar(&opts.Tags, "tag", nil, "filter by tag (repeatable)")
	cmd.Flags().StringVar(&opts.Format, "format", "table", "output format (table, json, plain)")

	return cmd
}

func runList(opts *ListOptions) error {
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

	// Build filter
	filter := store.Filter{
		Tags: opts.Tags,
	}

	if opts.Mine {
		filter.IdentityPath = cfg.Identity.Path
	}
	// Note: --shared is mutually exclusive with --mine
	// If --shared is specified, we filter to show workflows NOT under identity path
	// This will be handled in post-processing

	// List workflows
	refs, err := str.List(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list workflows: %w", err)
	}

	// Post-process for --shared flag
	if opts.Shared {
		refs = filterSharedWorkflows(refs, cfg.Identity.Path)
	}

	// Format output
	switch OutputFormat(opts.Format) {
	case FormatTable:
		printTable(refs, str, ctx)
	case FormatJSON:
		if err := printJSON(refs, str, ctx); err != nil {
			return err
		}
	case FormatPlain:
		printPlain(refs, str, ctx)
	default:
		return fmt.Errorf("invalid format: %s (must be table, json, or plain)", opts.Format)
	}

	return nil
}

// filterSharedWorkflows filters to show only workflows NOT under the identity path.
func filterSharedWorkflows(refs []store.WorkflowRef, identityPath string) []store.WorkflowRef {
	var filtered []store.WorkflowRef
	for _, ref := range refs {
		refIdentityPath := extractIdentityPathFromRef(ref)
		if refIdentityPath != identityPath {
			filtered = append(filtered, ref)
		}
	}
	return filtered
}

// extractIdentityPathFromRef extracts the identity path from a workflow reference.
func extractIdentityPathFromRef(ref store.WorkflowRef) string {
	// Expected format: .../workflows/<identity>/<slug>/workflow.yaml
	parts := strings.Split(ref.Path, string("/"))
	for i, part := range parts {
		if part == "workflows" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// printTable prints workflows in table format.
func printTable(refs []store.WorkflowRef, str store.Store, ctx context.Context) {
	if len(refs) == 0 {
		fmt.Println("No workflows found.")
		return
	}

	// Calculate column widths
	maxSlugLen := 10 // default "WORKFLOW"
	maxTagsLen := 8  // default "TAGS"
	maxTimeLen := 10 // default "UPDATED"

	// First pass: collect data
	type row struct {
		slug    string
		tags    string
		updated string
	}
	var rows []row

	for _, ref := range refs {
		// Load workflow for tags
		wf, err := str.Load(ctx, ref)
		tagsStr := ""
		if err == nil && len(wf.Tags) > 0 {
			tagsStr = strings.Join(wf.Tags, ", ")
		}

		slug := ref.Slug
		if slug == "" {
			slug = ref.ID
		}

		updated := formatTimeAgo(ref.UpdatedAt)

		if len(slug) > maxSlugLen {
			maxSlugLen = len(slug)
		}
		if len(tagsStr) > maxTagsLen {
			maxTagsLen = len(tagsStr)
		}

		rows = append(rows, row{
			slug:    slug,
			tags:    tagsStr,
			updated: updated,
		})
	}

	// Print header
	fmt.Printf("%-*s  %-*s  %s\n", maxSlugLen, "WORKFLOW", maxTagsLen, "TAGS", "UPDATED")
	fmt.Printf("%s  %s  %s\n", strings.Repeat("-", maxSlugLen), strings.Repeat("-", maxTagsLen), strings.Repeat("-", maxTimeLen))

	// Print rows
	for _, r := range rows {
		tags := r.tags
		if tags == "" {
			tags = "-"
		}
		fmt.Printf("%-*s  %-*s  %s\n", maxSlugLen, r.slug, maxTagsLen, tags, r.updated)
	}

	fmt.Printf("\nTotal: %d workflow(s)\n", len(refs))
}

// printJSON prints workflows in JSON format.
func printJSON(refs []store.WorkflowRef, str store.Store, ctx context.Context) error {
	type JSONWorkflow struct {
		ID        string    `json:"id"`
		Slug      string    `json:"slug"`
		Path      string    `json:"path"`
		UpdatedAt time.Time `json:"updated_at"`
		Tags      []string  `json:"tags,omitempty"`
		Title     string    `json:"title,omitempty"`
	}

	var workflows []JSONWorkflow
	for _, ref := range refs {
		// Load workflow for title and tags
		wf, err := str.Load(ctx, ref)
		tags := []string{}
		title := ""
		if err == nil {
			tags = wf.Tags
			title = wf.Title
		}

		workflows = append(workflows, JSONWorkflow{
			ID:        ref.ID,
			Slug:      ref.Slug,
			Path:      ref.Path,
			UpdatedAt: ref.UpdatedAt,
			Tags:      tags,
			Title:     title,
		})
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(workflows)
}

// printPlain prints workflows in plain text format.
func printPlain(refs []store.WorkflowRef, str store.Store, ctx context.Context) {
	if len(refs) == 0 {
		fmt.Println("No workflows found.")
		return
	}

	for i, ref := range refs {
		// Load workflow for title
		wf, err := str.Load(ctx, ref)
		title := ref.Slug
		if err == nil && wf.Title != "" {
			title = fmt.Sprintf("%s (%s)", wf.Title, ref.Slug)
		}

		fmt.Printf("%d. %s\n", i+1, title)
		fmt.Printf("   Path: %s\n", ref.Path)
		fmt.Printf("   Updated: %s\n", ref.UpdatedAt.Format(time.RFC3339))

		// Print tags if available
		if err == nil && len(wf.Tags) > 0 {
			fmt.Printf("   Tags: %s\n", strings.Join(wf.Tags, ", "))
		}

		fmt.Println()
	}

	fmt.Printf("Total: %d workflow(s)\n", len(refs))
}

// formatTimeAgo formats a time as a relative "time ago" string.
func formatTimeAgo(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", mins)
	}
	if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", hours)
	}
	if diff < 30*24*time.Hour {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
	if diff < 365*24*time.Hour {
		months := int(diff.Hours() / 24 / 30)
		if months == 1 {
			return "1mo ago"
		}
		return fmt.Sprintf("%dmo ago", months)
	}
	years := int(diff.Hours() / 24 / 365)
	if years == 1 {
		return "1y ago"
	}
	return fmt.Sprintf("%dy ago", years)
}
