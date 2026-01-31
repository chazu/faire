// Package upgrade provides self-update functionality for git-savvy.
package upgrade

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
)

// Updater handles self-update operations.
type Updater struct {
	CurrentVersion string
	Client         *ReleaseClient
}

// NewUpdater creates a new Updater instance.
func NewUpdater(currentVersion, owner, repo string) *Updater {
	return &Updater{
		CurrentVersion: currentVersion,
		Client:         NewReleaseClient(owner, repo),
	}
}

// CheckForUpdate checks if a newer version is available.
func (u *Updater) CheckForUpdate(ctx context.Context) (*Release, error) {
	release, err := u.Client.FetchLatestRelease(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching latest release: %w", err)
	}

	if !release.IsNewer(u.CurrentVersion) {
		return nil, nil // No update available
	}

	return release, nil
}

// Download downloads a release asset.
func (u *Updater) Download(ctx context.Context, asset *Asset) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", asset.BrowserDownloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return data, nil
}

// ApplyUpdate replaces the current binary with the new one.
func (u *Updater) ApplyUpdate(binary []byte) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("getting executable path: %w", err)
	}

	backupPath := execPath + ".bak"

	// Create backup of current binary
	if err := os.Rename(execPath, backupPath); err != nil {
		return fmt.Errorf("creating backup: %w", err)
	}

	// Write new binary
	if err := os.WriteFile(execPath, binary, 0755); err != nil {
		// Rollback on failure
		_ = os.Rename(backupPath, execPath)
		return fmt.Errorf("writing new binary (rolled back): %w", err)
	}

	// Remove backup on success
	_ = os.Remove(backupPath)

	return nil
}

// CurrentPlatform returns the current platform identifier.
func CurrentPlatform() string {
	return fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
}
