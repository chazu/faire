// Package cli provides Cobra command definitions for faire.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/chazuruo/faire/internal/app"
	"github.com/chazuruo/faire/internal/gitrepo"
)

// NewSyncCommand creates the sync command for fetching and integrating remote changes.
func NewSyncCommand() *cobra.Command {
	opts := &app.SyncOptions{}
	strategyStr := ""

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Fetch and integrate remote changes",
		Long: `Fetch changes from the remote repository and integrate them using the configured strategy.

The sync command performs a git fetch followed by integration using one of three strategies:
- ff-only: Only fast-forward, fail if not possible
- rebase: Rebase local commits on top of remote
- merge: Merge remote changes into local branch

The default strategy is configured in config.toml (repo.sync_strategy), defaulting to "rebase".

Examples:
  gitsavvy sync                    # Sync with default settings
  gitsavvy sync --strategy merge   # Use merge strategy
  gitsavvy sync --no-push          # Sync but don't push
  gitsavvy sync --json             # Output in JSON format`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Convert strategy string to IntegrationStrategy
			if strategyStr != "" {
				opts.Strategy = gitrepo.IntegrationStrategy(strategyStr)
			}
			return runSync(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.Flags().StringVar(&strategyStr, "strategy", "", "integration strategy: ff-only, rebase, merge")
	cmd.Flags().StringVar(&opts.Remote, "remote", "", "git remote name (default from config)")
	cmd.Flags().StringVar(&opts.Branch, "branch", "", "remote branch name (default from config)")
	cmd.Flags().BoolVar(&opts.NoFetch, "no-fetch", false, "skip fetch step")
	cmd.Flags().BoolVar(&opts.NoPush, "no-push", false, "skip pushing after sync")
	cmd.Flags().BoolVar(&opts.Push, "push", false, "push after successful sync")
	cmd.Flags().BoolVar(&opts.JSONOutput, "json", false, "output in JSON format")

	return cmd
}

func runSync(ctx context.Context, opts *app.SyncOptions) error {
	result, err := app.Sync(ctx, *opts)
	if err != nil && !gitrepo.IsConflictError(err) {
		// For non-conflict errors, return the error
		return fmt.Errorf("sync failed: %w", err)
	}

	if opts.JSONOutput {
		return printSyncJSON(result)
	}

	printSyncPlain(result)

	// Exit codes: 0 ok, 10 git failure, 12 conflicts unresolved
	if !result.Success {
		if len(result.Conflicts) > 0 {
			os.Exit(12)
		}
		os.Exit(10)
	}

	return nil
}

// printSyncPlain prints sync result in plain text format.
func printSyncPlain(result *app.SyncOutput) {
	if result.Success {
		fmt.Println("Sync complete")
		fmt.Println()
		fmt.Printf("Branch: %s (%s)\n", result.Branch, result.RemoteBranch)
		fmt.Printf("Strategy: %s\n", result.Strategy)
		fmt.Printf("Ahead: %d, Behind: %d\n", result.Ahead, result.Behind)
		fmt.Println()

		if len(result.NewFiles) > 0 || len(result.UpdatedFiles) > 0 || len(result.DeletedFiles) > 0 {
			fmt.Println("Changes:")
			for _, f := range result.NewFiles {
				fmt.Printf("  + %s\n", f)
			}
			for _, f := range result.UpdatedFiles {
				fmt.Printf("  ~ %s\n", f)
			}
			for _, f := range result.DeletedFiles {
				fmt.Printf("  - %s\n", f)
			}
		}
	} else {
		fmt.Println("Sync failed")
		fmt.Println()
		fmt.Printf("Branch: %s (%s)\n", result.Branch, result.RemoteBranch)
		fmt.Printf("Strategy: %s\n", result.Strategy)
		fmt.Println()

		if len(result.Conflicts) > 0 {
			fmt.Println("Conflicts:")
			for _, f := range result.Conflicts {
				fmt.Printf("  ✗ %s\n", f)
			}
			fmt.Println()
			fmt.Println("Run 'gitsavvy sync --resolve' to resolve conflicts")
		} else if result.Error != "" {
			fmt.Printf("Error: %s\n", result.Error)
		}
	}
}

// printSyncJSON prints sync result in JSON format.
func printSyncJSON(result *app.SyncOutput) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
