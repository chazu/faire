// Package cli provides Cobra command definitions for svf.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

// VersionInfo contains version information for the binary.
type VersionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	BuiltBy string `json:"built_by"`
	Go      string `json:"go_version"`
}

// VersionOptions contains the options for the version command.
type VersionOptions struct {
	Short bool
	JSON  bool
}

// NewVersionCommand creates the version command.
func NewVersionCommand(version, commit, date, builtBy string) *cobra.Command {
	opts := &VersionOptions{}

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		Long: `Display the svf version information.

Shows version, commit hash, build date, who built it, and Go version.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVersion(opts, version, commit, date, builtBy)
		},
	}

	cmd.Flags().BoolVar(&opts.Short, "short", false, "print only the version number")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "output in JSON format")

	return cmd
}

func runVersion(opts *VersionOptions, version, commit, date, builtBy string) error {
	info := VersionInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
		BuiltBy: builtBy,
		Go:      runtime.Version(),
	}

	if opts.JSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(info); err != nil {
			return fmt.Errorf("failed to encode JSON: %w", err)
		}
		return nil
	}

	if opts.Short {
		fmt.Println(info.Version)
		return nil
	}

	fmt.Printf("svf version %s\n", info.Version)
	fmt.Printf("commit: %s\n", info.Commit)
	fmt.Printf("built at: %s\n", info.Date)
	if info.BuiltBy != "" && info.BuiltBy != "unknown" {
		fmt.Printf("built by: %s\n", info.BuiltBy)
	}
	fmt.Printf("go version: %s\n", info.Go)

	return nil
}
