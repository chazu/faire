// Package cli provides Cobra command definitions for git-savvy.
package cli

import (
	"fmt"
	"os"

	"github.com/chazuruo/faire/internal/app"
	"github.com/spf13/cobra"
)

// WhoamiOptions contains the options for the whoami command.
type WhoamiOptions struct {
	JSON bool
}

// NewWhoamiCommand creates the whoami command.
func NewWhoamiCommand() *cobra.Command {
	opts := &WhoamiOptions{}

	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Display identity configuration information",
		Long: `Display the current git-savvy identity configuration.

Shows information about the config file location, repository path,
identity path, mode, and author details.

By default, output is in plain text format. Use --json for JSON output.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWhoami(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.JSON, "json", false, "output in JSON format")

	return cmd
}

func runWhoami(opts *WhoamiOptions) error {
	output, err := app.Whoami("")
	if err != nil {
		// Check if this is a "config not found" error
		if os.Getenv("GITSAVVY_CONFIG") != "" {
			// User specified a config path via env var
			return fmt.Errorf("config error: %w", err)
		}
		// No config file found - provide helpful error message
		fmt.Fprintf(os.Stderr, "Error: Config file not found\n")
		fmt.Fprintf(os.Stderr, "Expected location: ~/.config/gitsavvy/config.toml\n")
		fmt.Fprintf(os.Stderr, "Run 'gitsavvy init' to create a default config, or create the file manually.\n")
		os.Exit(2)
		return err // unreachable but needed for type safety
	}

	if opts.JSON {
		if err := app.PrintWhoamiJSON(output); err != nil {
			return fmt.Errorf("failed to output JSON: %w", err)
		}
	} else {
		app.PrintWhoami(output)
	}

	return nil
}
