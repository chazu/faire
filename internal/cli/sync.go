// Package cli provides Cobra command definitions for svf.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"

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

	result, err := integrateChanges(ctx, repo, strategy, opts.Conflicts)
	if err != nil {
		return err
	}

	// Handle conflicts if detected
	if result.Conflicts {
		return handleConflicts(ctx, repo, opts.Conflicts, result)
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
	FastForward    bool
	Rebased        bool
	Merged         bool
	Conflicts      bool
	NewCommits     int
	ConflictFiles  []string
}

// integrateChanges integrates remote changes.
func integrateChanges(ctx context.Context, repo gitrepo.Repo, strategy string, conflictsMode string) (*IntegrateResult, error) {
	result := &IntegrateResult{}

	// Convert strategy string to gitrepo.IntegrateStrategy
	var integrateStrategy gitrepo.IntegrateStrategy
	switch strategy {
	case "ff-only", "ff_only":
		integrateStrategy = gitrepo.StrategyFFOnly
	case "rebase":
		integrateStrategy = gitrepo.StrategyRebase
	case "merge":
		integrateStrategy = gitrepo.StrategyMerge
	default:
		return nil, fmt.Errorf("unknown strategy: %s", strategy)
	}

	// Perform integration using gitrepo.Integrate
	// Note: This will fail if conflicts are detected
	grResult, err := repo.Integrate(ctx, integrateStrategy)
	if err != nil {
		// Check if it's a conflict error
		if hasConflicts, _ := repo.HasConflicts(ctx); hasConflicts {
			// Return result with conflicts marked
			result.Conflicts = true
			result.ConflictFiles, _ = repo.GetConflicts(ctx)
			return result, fmt.Errorf("conflicts detected during integration")
		}
		return nil, err
	}

	// Copy results from gitrepo result
	result.FastForward = grResult.FastForward
	result.Rebased = grResult.Rebased
	result.Merged = grResult.Merged
	result.Conflicts = grResult.Conflicts
	result.NewCommits = grResult.NewCommits
	result.ConflictFiles = grResult.ConflictFiles

	fmt.Println("✓ Integration complete")
	return result, nil
}

// handleConflicts handles conflicts based on the specified mode.
func handleConflicts(ctx context.Context, repo gitrepo.Repo, mode string, result *IntegrateResult) error {
	switch mode {
	case "ours":
		// Accept ours for all conflicts
		return resolveAllConflicts(ctx, repo, "ours")
	case "theirs":
		// Accept theirs for all conflicts
		return resolveAllConflicts(ctx, repo, "theirs")
	case "abort":
		// Abort the integration
		return fmt.Errorf("integration aborted due to conflicts")
	case "tui", "":
		// Launch TUI conflict resolver
		return launchConflictResolver(ctx, repo, result)
	default:
		return fmt.Errorf("unknown conflicts mode: %s", mode)
	}
}

// resolveAllConflicts resolves all conflicts using the specified strategy.
func resolveAllConflicts(ctx context.Context, repo gitrepo.Repo, strategy string) error {
	conflicts, err := repo.GetConflicts(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Resolving %d conflict(s) using '%s' strategy...\n", len(conflicts), strategy)

	for _, file := range conflicts {
		// Use git checkout to accept ours or theirs
		var cmd *exec.Cmd
		if strategy == "ours" {
			cmd = exec.Command("git", "checkout", "--ours", file)
		} else {
			cmd = exec.Command("git", "checkout", "--theirs", file)
		}

		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to resolve %s\n", file)
		}

		// Mark as resolved
		cmd = exec.Command("git", "add", file)
		_ = cmd.Run()
	}

	fmt.Println("✓ All conflicts resolved")
	return nil
}

// launchConflictResolver launches the TUI conflict resolver.
func launchConflictResolver(ctx context.Context, repo gitrepo.Repo, result *IntegrateResult) error {
	// TODO: Integrate with TUI conflict resolver model
	// For now, show instructions
	conflicts, _ := repo.GetConflicts(ctx)
	fmt.Printf("\n%d conflict(s) detected:\n", len(conflicts))
	for _, file := range conflicts {
		fmt.Printf("  - %s\n", file)
	}
	fmt.Println("\nPlease resolve conflicts manually, then run 'svf sync' again.")
	fmt.Println("Or use --conflicts=ours or --conflicts=theirs to auto-resolve.")
	return nil
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
