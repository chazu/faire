// Package cli provides Cobra command definitions for svf.
package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/rodaine/table"
	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/gitrepo"
	"github.com/chazuruo/svf/internal/workflows"
	"github.com/chazuruo/svf/internal/workflows/store"
)

// ListOptions contains the options for the list command.
type ListOptions struct {
	ConfigPath string
	Mine       bool
	Shared     bool
	Tags       []string
	Format     string
}

// NewListCommand creates the list command.
func NewListCommand() *cobra.Command {
	opts := &ListOptions{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workflows",
		Long: `List all available workflows.

Supports filtering by owner (mine/shared) and tags.
Multiple output formats: table (default), json, plain.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(opts)
		},
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.Flags().BoolVar(&opts.Mine, "mine", false, "only show workflows under identity path")
	cmd.Flags().BoolVar(&opts.Shared, "shared", false, "only show shared workflows")
	cmd.Flags().StringSliceVar(&opts.Tags, "tag", nil, "filter by tag (repeatable)")
	cmd.Flags().StringVar(&opts.Format, "format", "table", "output format: table, json, plain")

	return cmd
}

func runList(opts *ListOptions) error {
	ctx := context.Background()

	// Load config
	var cfg *config.Config
	var err error
	if opts.ConfigPath != "" {
		cfg, err = config.Load(opts.ConfigPath)
	} else {
		cfg, err = config.LoadWithDefaults()
	}
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Open repo
	repo := gitrepo.New(cfg.Repo.Path)
	if !repo.IsInitialized(ctx) {
		return fmt.Errorf("repository not initialized at %s. Run 'svf init' first, or check your config at %s", cfg.Repo.Path, config.DetectConfigPath())
	}

	// Create store
	str, err := store.New(repo, cfg)
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}

	// Build filter
	filter := store.Filter{}
	if opts.Mine {
		filter.IdentityPath = cfg.Identity.Path
	}
	if len(opts.Tags) > 0 {
		filter.Tags = opts.Tags
	}

	// List workflows
	refs, err := str.List(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list workflows: %w", err)
	}

	// Load workflows to get title and tags (for display)
	var workflowInfos []workflowInfo
	for _, ref := range refs {
		wf, loadErr := str.Load(ctx, ref)
		if loadErr != nil {
			continue // Skip workflows we can't load
		}
		workflowInfos = append(workflowInfos, workflowInfo{
			Ref:       ref,
			Workflow:  wf,
		})
	}

	// Output
	switch opts.Format {
	case "json":
		printListJSON(workflowInfos)
	case "plain":
		printListPlain(workflowInfos)
	case "table":
		printListTable(workflowInfos)
	default:
		return fmt.Errorf("unknown format: %s", opts.Format)
	}

	return nil
}

// workflowInfo combines a WorkflowRef with its Workflow for display.
type workflowInfo struct {
	Ref      store.WorkflowRef
	Workflow *workflows.Workflow
}

// printListTable prints workflows in table format.
func printListTable(workflows []workflowInfo) {
	if len(workflows) == 0 {
		fmt.Println("No workflows found.")
		return
	}

	tbl := table.New("ID", "Title", "Tags", "Updated")
	for _, info := range workflows {
		tags := strings.Join(info.Workflow.Tags, ", ")
		updated := info.Ref.UpdatedAt.Format("2006-01-02")
		tbl.AddRow(info.Ref.ID, info.Workflow.Title, tags, updated)
	}
	tbl.Print()
}

// printListPlain prints workflows in plain format.
func printListPlain(workflows []workflowInfo) {
	if len(workflows) == 0 {
		fmt.Println("No workflows found.")
		return
	}

	for _, info := range workflows {
		tags := ""
		if len(info.Workflow.Tags) > 0 {
			tags = fmt.Sprintf(" [%s]", strings.Join(info.Workflow.Tags, ", "))
		}
		fmt.Printf("%s: %s%s\n", info.Ref.ID, info.Workflow.Title, tags)
	}
}

// printListJSON prints workflows in JSON format.
func printListJSON(workflows []workflowInfo) {
	fmt.Print("[")
	for i, info := range workflows {
		if i > 0 {
			fmt.Print(",")
		}
		tags := "[]"
		if len(info.Workflow.Tags) > 0 {
			tags = fmt.Sprintf(`["%s"]`, strings.Join(info.Workflow.Tags, `", "`))
		}
		fmt.Printf(`{"id":"%s","title":"%s","tags":%s,"updated_at":"%s"}`,
			info.Ref.ID, info.Workflow.Title, tags, info.Ref.UpdatedAt.Format(time.RFC3339))
	}
	fmt.Println("]")
}
