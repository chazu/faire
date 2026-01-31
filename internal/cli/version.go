// Package cli provides Cobra command definitions for svf.
package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// versionInfo contains version information.
type versionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
}

// Version is set at build time using ldflags.
var Version = "dev"

// Commit is set at build time using ldflags.
var Commit = "unknown"

// BuildDate is set at build time using ldflags.
var BuildDate = "unknown"

// NewVersionCommand creates the version command.
func NewVersionCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  `Display version information including semantic version, git commit hash, and build timestamp.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVersion(jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output version information as JSON")

	return cmd
}

func runVersion(jsonOutput bool) error {
	info := versionInfo{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: "",
	}

	if jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(info); err != nil {
			return err
		}
		return nil
	}

	// Text output
	fmt.Printf("svf version %s\n", info.Version)
	if info.Commit != "unknown" && info.Commit != "" {
		fmt.Printf("commit: %s\n", info.Commit)
	}
	if info.BuildDate != "unknown" && info.BuildDate != "" {
		fmt.Printf("built: %s\n", info.BuildDate)
	}

	return nil
}
