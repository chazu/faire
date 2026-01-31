// Package cli provides Cobra command definitions for svf.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/gitrepo"
	"github.com/chazuruo/svf/internal/index"
	"github.com/chazuruo/svf/internal/tui"
)

// SearchOptions contains the options for the search command.
type SearchOptions struct {
	ConfigPath string
	Query      string
	Tags       []string
	Mine       bool
	Shared     bool
	JSON       bool
}

// NewSearchCommand creates the search command.
func NewSearchCommand() *cobra.Command {
	opts := &SearchOptions{}

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search workflows with fuzzy matching",
		Long: `Search for workflows using the search index.

Interactive mode (default):
- TUI fuzzy search with real-time results
- Preview workflow details
- Select to view or run

Non-interactive mode (--query or --no-tui):
- Plain text results with workflow IDs
- Use --json for structured output

Filters:
- --mine: only show your workflows
- --shared: only show shared workflows
- --tag: filter by tag`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Query = args[0]
			}
			return runSearch(opts)
		},
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.Flags().StringVar(&opts.Query, "query", "", "search query (non-interactive mode)")
	cmd.Flags().StringSliceVar(&opts.Tags, "tag", nil, "filter by tag (repeatable)")
	cmd.Flags().BoolVar(&opts.Mine, "mine", false, "only show my workflows")
	cmd.Flags().BoolVar(&opts.Shared, "shared", false, "only show shared workflows")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "output results as JSON")

	return cmd
}

func runSearch(opts *SearchOptions) error {
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

	// Check if index exists and is not stale
	builder := index.NewBuilder(cfg.Repo.Path, cfg)
	idx, err := builder.Load()
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("search index not found. Run 'svf sync' to build it")
		}
		return fmt.Errorf("failed to load index: %w", err)
	}

	// Check if index is stale
	stale, err := builder.IsStale()
	if err == nil && stale {
		if IsNoTUI() {
			// LLM mode, show warning but continue
			fmt.Fprintf(os.Stderr, "Warning: search index is stale. Run 'svf sync --reindex' to update.\n")
		} else {
			fmt.Fprintf(os.Stderr, "Warning: search index is stale. Run 'svf sync --reindex' to update.\n")
		}
	}

	// Determine if we should use non-interactive mode
	nonInteractive := opts.Query != "" || opts.JSON || IsNoTUI()

	if nonInteractive {
		return searchNonInteractive(ctx, idx, opts, cfg)
	}

	// Interactive mode
	return searchInteractive(ctx, idx, opts, cfg)
}

// searchNonInteractive performs non-interactive search.
func searchNonInteractive(ctx context.Context, idx *index.Index, opts *SearchOptions, cfg *config.Config) error {
	// Build search options
	searchOpts := index.SearchOptions{
		Query:      opts.Query,
		Tags:       opts.Tags,
		Mine:       opts.Mine,
		Shared:     opts.Shared,
		MaxResults: 0, // No limit
	}

	// If --mine is specified without explicit identity path, use config identity path
	if opts.Mine && searchOpts.IdentityPath == "" {
		searchOpts.IdentityPath = cfg.Identity.Path
	}

	// Perform search
	results := idx.FuzzySearch(searchOpts)

	// Output results
	if opts.JSON {
		return outputJSON(results)
	}

	return outputPlain(results)
}

// searchInteractive performs interactive TUI search.
func searchInteractive(ctx context.Context, idx *index.Index, opts *SearchOptions, cfg *config.Config) error {
	// Create TUI search model
	model := tui.NewSearchModel(idx)

	// Set initial query if provided
	if opts.Query != "" {
		model.SearchInput.SetValue(opts.Query)
		model.SearchInput.CursorEnd()
		model.PerformSearch()
	}

	// Set initial filters
	if opts.Mine {
		model.Mine = true
		model.PerformSearch()
	}
	if opts.Shared {
		model.Shared = true
		model.PerformSearch()
	}
	if len(opts.Tags) > 0 {
		model.Tags = opts.Tags
		model.PerformSearch()
	}

	// Run TUI
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run search TUI: %w", err)
	}

	// Get the final model
	finalSearch, ok := finalModel.(tui.SearchModel)
	if !ok {
		return fmt.Errorf("unexpected model type from search")
	}

	// Check if user quit without selecting
	if finalSearch.DidQuit() {
		fmt.Println("Search cancelled.")
		return nil
	}

	// Display selected workflow
	if finalSearch.DidConfirm() {
		entry := finalSearch.GetSelectedEntry()
		if entry != nil {
			fmt.Printf("\nSelected: %s\n", entry.Title)
			fmt.Printf("ID: %s\n", entry.ID)
			fmt.Printf("Path: %s\n", entry.Path)
			if len(entry.Tags) > 0 {
				fmt.Printf("Tags: [%s]\n", strings.Join(entry.Tags, ", "))
			}

			// Suggest next actions
			fmt.Printf("\nNext steps:\n")
			fmt.Printf("  svf view %s   # View workflow details\n", entry.ID)
			fmt.Printf("  svf run %s    # Run the workflow\n", entry.ID)
		}
	}

	return nil
}

// outputPlain outputs search results in plain text format.
func outputPlain(results []index.SearchResult) error {
	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	fmt.Printf("Found %d result(s):\n\n", len(results))

	for i, result := range results {
		entry := result.Entry
		fmt.Printf("%d. %s\n", i+1, entry.Title)
		fmt.Printf("   ID: %s\n", entry.ID)
		if len(entry.Tags) > 0 {
			fmt.Printf("   Tags: %s\n", strings.Join(entry.Tags, ", "))
		}
		if len(result.Matches) > 0 {
			fmt.Printf("   Matches: %v\n", result.Matches)
		}
		fmt.Println()
	}

	return nil
}

// outputJSON outputs search results in JSON format.
func outputJSON(results []index.SearchResult) error {
	output := struct {
		Count   int                    `json:"count"`
		Results []index.SearchResult   `json:"results"`
	}{
		Count:   len(results),
		Results: results,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}
