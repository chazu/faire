// Package cli provides Cobra command definitions for svf.
package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/gitrepo"
)

// StatusOptions contains the options for the status command.
type StatusOptions struct {
	ConfigPath string
	JSON       bool
}

// NewStatusCommand creates the status command.
func NewStatusCommand() *cobra.Command {
	opts := &StatusOptions{}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show repository and tool status",
		Long: `Display the current status of the Git repository and svf tool.

Shows:
- Git status (dirty/clean, ahead/behind counts)
- Last sync time
- Index freshness
- Identity path
- Repository path`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(opts)
		},
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "output in JSON format")

	return cmd
}

func runStatus(opts *StatusOptions) error {
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
		fmt.Println("Repository not initialized.")
		fmt.Println("Run 'svf init' to get started.")
		return nil
	}

	// Get git status
	status, err := repo.Status(ctx)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	// Print status
	if opts.JSON {
		printStatusJSON(cfg, status)
	} else {
		printStatusPlain(cfg, status)
	}

	return nil
}

// printStatusPlain prints status in plain text format.
func printStatusPlain(cfg *config.Config, status gitrepo.Status) {
	fmt.Println("Repository Status:")
	fmt.Printf("  Path:   %s\n", cfg.Repo.Path)
	fmt.Printf("  Branch: %s\n", status.Branch)

	// Git status
	if status.Dirty {
		fmt.Println("  State:  dirty (uncommitted changes)")
	} else {
		fmt.Println("  State:  clean")
	}

	// Ahead/behind
	if status.Ahead > 0 || status.Behind > 0 {
		fmt.Printf("  Sync:   %d ahead, %d behind\n", status.Ahead, status.Behind)
	} else {
		fmt.Println("  Sync:   up to date")
	}

	// Identity
	fmt.Println("\nIdentity:")
	fmt.Printf("  Path: %s\n", cfg.Identity.Path)
	fmt.Printf("  Mode: %s\n", cfg.Identity.Mode)

	// Last sync (placeholder - would be stored in state)
	fmt.Println("\nTool Status:")
	fmt.Println("  Last sync: (not yet implemented)")
	fmt.Println("  Index:     (not yet implemented)")
}

// printStatusJSON prints status in JSON format.
func printStatusJSON(cfg *config.Config, status gitrepo.Status) {
	fmt.Printf(`{
  "repo": {
    "path": "%s",
    "branch": "%s",
    "dirty": %t,
    "ahead": %d,
    "behind": %d
  },
  "identity": {
    "path": "%s",
    "mode": "%s"
  },
  "tool": {
    "last_sync": null,
    "index_fresh": null
  }
}
`, cfg.Repo.Path, status.Branch, status.Dirty, status.Ahead, status.Behind,
	cfg.Identity.Path, cfg.Identity.Mode)
}
