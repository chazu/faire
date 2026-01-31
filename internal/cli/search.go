// Package cli provides Cobra command definitions for faire.
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/chazuruo/faire/internal/config"
	"github.com/chazuruo/faire/internal/gitrepo"
	"github.com/chazuruo/faire/internal/index"
	"github.com/chazuruo/faire/internal/tui"
	"github.com/chazuruo/faire/internal/workflows/store"
)

// SearchOptions contains the options for the search command.
type SearchOptions struct {
	ConfigPath  string
	Query       string
	Mine        bool
	Shared      bool
	Tags        []string
	Interactive bool
}

// NewSearchCommand creates the search command for searching workflows.
func NewSearchCommand() *cobra.Command {
	opts := &SearchOptions{}

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search workflows using fuzzy search",
		Long: `Search workflows using an interactive fuzzy search TUI.

The search command opens a full-screen terminal UI for searching workflows:
- Type to filter workflows
- Use arrow keys to navigate
- Press enter to select a workflow
- Press q or esc to quit

Examples:
  faire search                  # Open interactive search
  faire search --query deploy   # Pre-fill search query
  faire search --mine           # Search only your workflows
  faire search --tag ops        # Search workflows tagged with 'ops'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(opts)
		},
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.Flags().StringVar(&opts.Query, "query", "", "pre-fill search query")
	cmd.Flags().BoolVar(&opts.Mine, "mine", false, "only search workflows under your identity path")
	cmd.Flags().BoolVar(&opts.Shared, "shared", false, "only search shared workflows")
	cmd.Flags().StringSliceVar(&opts.Tags, "tag", nil, "filter by tag (repeatable)")
	cmd.Flags().BoolVar(&opts.Interactive, "interactive", true, "run in interactive mode (default true)")

	return cmd
}

func runSearch(opts *SearchOptions) error {
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

	// Create searcher
	searcher := index.NewSearcher(repo, str, cfg)

	// Ensure index is up to date
	if err := searcher.EnsureIndex(ctx); err != nil {
		return fmt.Errorf("failed to build index: %w", err)
	}

	// Get index
	idx, err := searcher.GetIndex(ctx)
	if err != nil {
		return fmt.Errorf("failed to get index: %w", err)
	}

	// Build search options
	searchOpts := index.SearchOptions{
		Query: opts.Query,
		Limit: 50,
	}

	if opts.Mine {
		searchOpts.IdentityPath = cfg.Identity.Path
	}

	if len(opts.Tags) > 0 {
		searchOpts.Tags = opts.Tags
	}

	// If not interactive, just print results and exit
	if !opts.Interactive {
		return printSearchResults(idx, searchOpts)
	}

	// Launch TUI
	searchModel := tui.NewWorkflowSearch(idx, opts.Query)
	p := tea.NewProgram(searchModel, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	finalSearch := finalModel.(tui.WorkflowSearchModel)

	// Handle quit without selection
	if finalSearch.DidQuit() {
		fmt.Println("Search cancelled.")
		return nil
	}

	// Get selected workflow
	selected := finalSearch.GetSelected()
	if selected == nil {
		fmt.Println("No workflow selected.")
		return nil
	}

	// Print selected workflow
	fmt.Printf("Selected: %s (id: %s)\n", selected.Title, selected.ID)
	fmt.Printf("Path: %s\n", selected.Path)

	if len(selected.Tags) > 0 {
		fmt.Printf("Tags: %s\n", formatTags(selected.Tags))
	}

	return nil
}

// printSearchResults prints search results without TUI.
func printSearchResults(idx *index.Index, opts index.SearchOptions) error {
	results := idx.Search(opts)

	if len(results) == 0 {
		fmt.Println("No matching workflows found.")
		return nil
	}

	for i, entry := range results {
		fmt.Printf("%d. %s\n", i+1, entry.Title)
		fmt.Printf("   ID: %s\n", entry.ID)
		fmt.Printf("   Slug: %s\n", entry.Slug)

		if len(entry.Tags) > 0 {
			fmt.Printf("   Tags: %s\n", formatTags(entry.Tags))
		}

		fmt.Println()
	}

	fmt.Printf("Total: %d result(s)\n", len(results))
	return nil
}

// formatTags formats tags for display.
func formatTags(tags []string) string {
	if len(tags) == 0 {
		return "-"
	}
	result := ""
	for i, tag := range tags {
		if i > 0 {
			result += ", "
		}
		result += tag
	}
	return result
}
