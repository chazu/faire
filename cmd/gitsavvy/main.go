package main

import (
	"fmt"
	"os"

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
		Use:   "gitsavvy",
		Short: "Git-backed workflow automation tool",
		Long: `git-savvy is a terminal-first workflow/runbook tool compatible with Savvy CLI,
but stores all workflows and metadata in a Git repository instead of a hosted backend.`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, Date),
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	rootCmd.CompletionOptions.DisableDefaultCmd = true

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
