// Package cli provides Cobra command definitions for svf.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/gitrepo"
	"github.com/chazuruo/svf/internal/index"
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
	builder := index.NewBuilder(cfg.Repo.Path)
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
	// TODO: Integrate with TUI search model
	// For now, fall back to non-interactive
	fmt.Println("(Interactive mode - TODO: integrate TUI search)")
	fmt.Println("Falling back to non-interactive mode...")

	// Set query to empty to show all results if no query provided
	if opts.Query == "" {
		opts.Query = ""
	}

	return searchNonInteractive(ctx, idx, opts, cfg)
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
		if entry.Tags != "" {
			fmt.Printf("   Tags: %s\n", entry.Tags)
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
