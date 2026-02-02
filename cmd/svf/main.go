package main

import (
	"fmt"
	"os"

	"github.com/chazuruo/svf/internal/cli"
	"github.com/spf13/cobra"
)

// Version is set at build time using ldflags
var Version = "dev"

// Commit is set at build time using ldflags
var Commit = "unknown"

// Date is set at build time using ldflags
var Date = "unknown"

func main() {
	rootCmd := &cobra.Command{
		Use:   "svf",
		Short: "Git-backed workflow automation tool",
		Long: `svf is a terminal-first workflow/runbook tool compatible with Savvy CLI,
but stores all workflows and metadata in a Git repository instead of a hosted backend.`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, Date),
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
	}

	// Add global flags
	cli.AddGlobalFlags(rootCmd)

	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Add subcommands
	rootCmd.AddCommand(cli.NewWhoamiCommand())
	rootCmd.AddCommand(cli.NewEditCommand())
	rootCmd.AddCommand(cli.NewInitCommand())
	rootCmd.AddCommand(cli.NewRecordCommand())
	rootCmd.AddCommand(cli.NewRecordHistoryCommand())
	rootCmd.AddCommand(cli.NewSyncCommand())
	rootCmd.AddCommand(cli.NewStatusCommand())
	rootCmd.AddCommand(cli.NewListCommand())
	rootCmd.AddCommand(cli.NewViewCommand())
	rootCmd.AddCommand(cli.NewRunCommand())
	rootCmd.AddCommand(cli.NewSearchCommand())
	rootCmd.AddCommand(cli.NewAskCommand())
	rootCmd.AddCommand(cli.NewExplainCommand())
	rootCmd.AddCommand(cli.NewExportCommand())
	rootCmd.AddCommand(cli.NewUpgradeCommand())
	rootCmd.AddCommand(cli.NewVersionCommand())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
