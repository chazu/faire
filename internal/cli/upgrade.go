// Package cli provides Cobra command definitions for svf.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/chazuruo/svf/internal/upgrade"
)

// NewUpgradeCommand creates the upgrade command.
func NewUpgradeCommand() *cobra.Command {
	var (
		checkOnly bool
		yes       bool
		pre       bool
		noVerify  bool
	)

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Update to the latest version",
		Long: `Check for updates and upgrade to the latest version of svf.

This command will:
1. Check GitHub releases for the latest version
2. Compare with your current version
3. Download the new binary if available
4. Verify checksums and GPG signatures (if available)
5. Install the update (with automatic rollback on failure)

GPG Signature Verification:
If SVF_PUBLIC_KEY environment variable is set to a public key file path,
the upgrade command will verify GPG signatures of release checksums.
Set SVF_PUBLIC_KEY to enable this feature.

Exit codes:
  0 - Success or already up-to-date
  1 - Generic error
  2 - Network error
  3 - Verification failed
  4 - Installation failed
  5 - Already on latest version (with --check-only)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(cmd.Context(), checkOnly, yes, pre, noVerify)
		},
	}

	cmd.Flags().BoolVar(&checkOnly, "check-only", false,
		"check for updates without installing")
	cmd.Flags().BoolVar(&yes, "yes", false,
		"skip confirmation prompt")
	cmd.Flags().BoolVar(&pre, "pre", false,
		"include pre-releases")
	cmd.Flags().BoolVar(&noVerify, "no-verify", false,
		"skip signature verification")

	return cmd
}

func runUpgrade(ctx context.Context, checkOnly, yes, pre, noVerify bool) error {
	// Get current binary path
	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Get current version
	currentVersion := Version // From version.go

	// Create checker
	checker := upgrade.NewChecker("chazu", "faire", pre)

	// Check for updates
	fmt.Println("Checking for updates...")
	release, err := checker.CheckLatest(ctx)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	// Compare versions
	cmp, err := checker.CompareVersions(currentVersion, release.TagName)
	if err != nil {
		return fmt.Errorf("failed to compare versions: %w", err)
	}

	if cmp == 0 || cmp > 0 {
		fmt.Printf("Already on latest version: %s\n", currentVersion)
		if checkOnly {
			return upgrade.NewError(upgrade.ExitAlreadyLatest, "Already on latest version", nil)
		}
		return nil
	}

	// Update available
	fmt.Printf("Update available: %s -> %s\n", currentVersion, release.TagName)
	if release.Body != "" {
		// Print first line of release notes
		notes := release.Body
		if len(notes) > 200 {
			notes = notes[:200] + "..."
		}
		fmt.Printf("\nRelease notes:\n%s\n", notes)
	}

	if checkOnly {
		fmt.Println("\nUse --yes to install the update")
		return nil
	}

	// Confirm before downloading
	if !yes {
		fmt.Print("\nInstall update? [y/N] ")
		var response string
		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Update cancelled")
			return nil
		}
	}

	// Download and install
	return installUpdate(ctx, binaryPath, release, noVerify)
}

func installUpdate(ctx context.Context, binaryPath string, release *upgrade.Release, noVerify bool) error {
	platform := upgrade.NewPlatform()
	finder := upgrade.NewAssetFinder(platform, "svf")

	// Find binary asset
	binaryAsset, err := finder.FindBinary(release)
	if err != nil {
		return fmt.Errorf("failed to find binary: %w", err)
	}

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "svf-upgrade-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Download
	downloader := upgrade.NewDownloader()
	archivePath := fmt.Sprintf("%s/%s", tempDir, binaryAsset.Name)

	fmt.Printf("\nDownloading %s...\n", binaryAsset.Name)
	if err := downloader.Download(ctx, binaryAsset.URL, archivePath); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	fmt.Printf("Downloaded to %s\n", archivePath)

	// Verify checksums if available
	if !noVerify {
		checksumAsset, err := finder.FindChecksum(release)
		if err == nil && checksumAsset != nil {
			checksumPath := fmt.Sprintf("%s/%s", tempDir, checksumAsset.Name)
			fmt.Printf("Downloading checksums...\n")
			if dlErr := downloader.Download(ctx, checksumAsset.URL, checksumPath); dlErr == nil {
				if err := downloader.VerifyChecksum(archivePath, checksumPath, binaryAsset.Name); err != nil {
					return fmt.Errorf("checksum verification failed: %w", err)
				}
				fmt.Println("Checksum verified")

				// Verify GPG signature if available
				signatureAsset, sigErr := finder.FindSignature(release)
				if sigErr == nil && signatureAsset != nil {
					signaturePath := fmt.Sprintf("%s/%s", tempDir, signatureAsset.Name)
					fmt.Printf("Downloading signature...\n")
					if sigDlErr := downloader.Download(ctx, signatureAsset.URL, signaturePath); sigDlErr == nil {
						// Try to get public key from environment or config
						publicKeyPath := os.Getenv("SVF_PUBLIC_KEY")
						if publicKeyPath != "" {
							if sigErr := downloader.VerifySignature(checksumPath, signaturePath, publicKeyPath); sigErr != nil {
								return fmt.Errorf("signature verification failed: %w", sigErr)
							}
							fmt.Println("Signature verified")
						} else {
							fmt.Println("Warning: Signature file found but SVF_PUBLIC_KEY not set, skipping signature verification")
						}
					}
				}
			}
		}
	}

	// Install
	fmt.Println("Installing update...")
	installer := upgrade.NewInstaller(binaryPath)
	defer installer.Cleanup()

	if err := installer.Install(archivePath); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	fmt.Printf("\nSuccessfully updated to %s!\n", release.TagName)
	fmt.Printf("Run 'svf version' to verify.\n")

	return nil
}
