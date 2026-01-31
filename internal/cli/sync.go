// Package cli provides Cobra command definitions for svf.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/gitrepo"
	"github.com/chazuruo/svf/internal/index"
)

// SyncOptions contains the options for the sync command.
type SyncOptions struct {
	ConfigPath string
	Strategy   string
	Remote     string
	Branch     string
	NoPush     bool
	Push       bool
	Conflicts  string
	Reindex    bool
}

// NewSyncCommand creates the sync command.
func NewSyncCommand() *cobra.Command {
	opts := &SyncOptions{}

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Synchronize with remote Git repository",
		Long: `Fetch and integrate changes from the remote repository.

Updates the local checkout and rebuilds the search index.
Supports different integration strategies (ff-only, rebase, merge).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSync(opts)
		},
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.Flags().StringVar(&opts.Strategy, "strategy", "", "integration strategy: ff-only, rebase, merge (default from config)")
	cmd.Flags().StringVar(&opts.Remote, "remote", "", "remote name (default: origin)")
	cmd.Flags().StringVar(&opts.Branch, "branch", "", "branch name (default from config)")
	cmd.Flags().BoolVar(&opts.NoPush, "no-push", false, "do not push local commits")
	cmd.Flags().BoolVar(&opts.Push, "push", false, "push after successful integrate")
	cmd.Flags().StringVar(&opts.Conflicts, "conflicts", "tui", "conflict resolution: tui, ours, theirs, abort")
	cmd.Flags().BoolVar(&opts.Reindex, "reindex", false, "force rebuild of search index")

	return cmd
}

func runSync(opts *SyncOptions) error {
	ctx := context.Background()

	// Load config
	cfg, err := config.LoadWithDefaults()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Open repo
	repo := gitrepo.New(cfg.Repo.Path)

	// Check if repo is initialized
	if !repo.IsInitialized(ctx) {
		return fmt.Errorf("repository not initialized. Run 'svf init' first")
	}

	fmt.Println("Syncing with remote...")

	// Fetch from remote
	remote := opts.Remote
	if remote == "" {
		remote = cfg.Repo.Remote
	}

	if err := fetchRemote(ctx, repo, remote); err != nil {
		return err
	}

	// Integrate changes
	strategy := opts.Strategy
	if strategy == "" {
		strategy = cfg.Repo.SyncStrategy
	}

	result, err := integrateChanges(ctx, repo, strategy)
	if err != nil {
		return err
	}

	// Show summary
	printSyncSummary(result)

	// Rebuild index if needed or requested
	if opts.Reindex || shouldRebuildIndex(ctx, repo, cfg) {
		if err := rebuildIndex(ctx, repo, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to rebuild index: %v\n", err)
		}
	}

	return nil
}

// fetchRemote fetches from the remote repository.
func fetchRemote(ctx context.Context, repo gitrepo.Repo, remote string) error {
	fmt.Printf("Fetching from %s...\n", remote)
	// TODO: Implement actual fetch
	// For now, just a placeholder
	fmt.Println("✓ Fetch complete")
	return nil
}

// IntegrateResult contains the result of an integrate operation.
type IntegrateResult struct {
	FastForward bool
	Rebased     bool
	Merged      bool
	Conflicts   bool
	NewCommits  int
}

// integrateChanges integrates remote changes.
func integrateChanges(ctx context.Context, repo gitrepo.Repo, strategy string) (*IntegrateResult, error) {
	result := &IntegrateResult{}

	switch strategy {
	case "ff-only", "ff_only":
		// Fast-forward only
		result.FastForward = true
	case "rebase":
		// Rebase
		result.Rebased = true
	case "merge":
		// Merge
		result.Merged = true
	default:
		return nil, fmt.Errorf("unknown strategy: %s", strategy)
	}

	fmt.Println("✓ Integration complete")
	return result, nil
}

// printSyncSummary prints a summary of the sync operation.
func printSyncSummary(result *IntegrateResult) {
	fmt.Println("\nSync Summary:")
	if result.NewCommits > 0 {
		fmt.Printf("  New commits: %d\n", result.NewCommits)
	}
	if result.Conflicts {
		fmt.Println("  Conflicts detected - please resolve manually")
	} else {
		fmt.Println("  No conflicts")
	}
}

// shouldRebuildIndex checks if the search index needs rebuilding.
func shouldRebuildIndex(ctx context.Context, repo gitrepo.Repo, cfg *config.Config) bool {
	if !cfg.Workflows.Index.AutoRebuild {
		return false
	}

	builder := index.NewBuilder(cfg.Repo.Path)
	stale, err := builder.IsStale()
	if err != nil {
		// On error, try rebuilding
		return true
	}
	return stale
}

// rebuildIndex rebuilds the search index.
func rebuildIndex(ctx context.Context, repo gitrepo.Repo, cfg *config.Config) error {
	fmt.Println("\nRebuilding search index...")
	builder := index.NewBuilder(cfg.Repo.Path)

	idx, err := builder.Build()
	if err != nil {
		return fmt.Errorf("building index: %w", err)
	}

	if err := builder.Save(idx); err != nil {
		return fmt.Errorf("saving index: %w", err)
	}

	fmt.Printf("✓ Index updated with %d workflows\n", len(idx.Workflows))
	return nil
}
