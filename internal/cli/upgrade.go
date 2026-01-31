// Package cli provides Cobra command definitions for svf.
package cli

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/chazuruo/svf/internal/upgrade"
	"github.com/spf13/cobra"
)

// UpgradeOptions contains the options for the upgrade command.
type UpgradeOptions struct {
	CheckOnly bool
	Yes       bool
	Owner     string
	Repo      string
}

// NewUpgradeCommand creates the upgrade command.
func NewUpgradeCommand(version, owner, repo string) *cobra.Command {
	opts := &UpgradeOptions{
		Owner: owner,
		Repo:  repo,
	}

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade to the latest version",
		Long: `Upgrade svf to the latest version.

Checks for updates on GitHub and downloads the latest release if available.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(opts, version)
		},
	}

	cmd.Flags().BoolVar(&opts.CheckOnly, "check", false, "only check for updates, don't download")
	cmd.Flags().BoolVar(&opts.Yes, "yes", false, "skip confirmation prompt")

	return cmd
}

func runUpgrade(opts *UpgradeOptions, currentVersion string) error {
	upgrader := upgrade.NewUpdater(currentVersion, opts.Owner, opts.Repo)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("Checking for updates...")

	release, err := upgrader.CheckForUpdate(ctx)
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}

	if release == nil {
		fmt.Println("Already up to date.")
		return nil
	}

	fmt.Printf("\nNew version available: %s\n", release.TagName)
	if release.Name != "" {
		fmt.Printf("Release: %s\n", release.Name)
	}
	fmt.Printf("Published: %s\n", release.PublishedAt)

	if release.Body != "" {
		fmt.Printf("\nRelease notes:\n%s\n", release.Body)
	}

	if opts.CheckOnly {
		return nil
	}

	// Find asset for current platform
	platform := upgrade.CurrentPlatform()
	asset, err := release.FindAssetForPlatform(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return fmt.Errorf("finding binary for %s: %w", platform, err)
	}

	fmt.Printf("\nFound binary: %s (%.2f MB)\n", asset.Name, float64(asset.Size)/(1024*1024))

	// Confirm before downloading
	if !opts.Yes {
		fmt.Print("\nDownload and install? [y/N] ")
		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
		if response != "y" && response != "Y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	fmt.Println("\nDownloading...")

	binary, err := upgrader.Download(ctx, asset)
	if err != nil {
		return fmt.Errorf("downloading release: %w", err)
	}

	fmt.Println("Installing...")

	if err := upgrader.ApplyUpdate(binary); err != nil {
		return fmt.Errorf("applying update: %w", err)
	}

	fmt.Printf("\nSuccessfully upgraded to %s!\n", release.TagName)
	fmt.Println("Please restart to use the new version.")

	return nil
}
