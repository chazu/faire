// Package upgrade provides self-update functionality for git-savvy.
package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Release represents a GitHub release.
type Release struct {
	TagName     string  `json:"tag_name"`
	Name        string  `json:"name"`
	Draft       bool    `json:"draft"`
	Prerelease  bool    `json:"prerelease"`
	PublishedAt string  `json:"published_at"`
	Body        string  `json:"body"`
	Assets      []Asset `json:"assets"`
}

// Asset represents a release asset (binary).
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// ReleaseClient fetches release information from GitHub.
type ReleaseClient struct {
	BaseURL string
	Owner   string
	Repo    string
}

// NewReleaseClient creates a new GitHub release client.
func NewReleaseClient(owner, repo string) *ReleaseClient {
	return &ReleaseClient{
		BaseURL: "https://api.github.com",
		Owner:   owner,
		Repo:    repo,
	}
}

// FetchLatestRelease fetches the latest release from GitHub.
func (c *ReleaseClient) FetchLatestRelease(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.BaseURL, c.Owner, c.Repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// GitHub API requires a user agent
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "gitsavvy-upgrade")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &release, nil
}

// FindAssetForPlatform finds the appropriate binary asset for the current platform.
func (r *Release) FindAssetForPlatform(goos, goarch string) (*Asset, error) {
	// Common platform patterns to try
	patterns := []string{
		fmt.Sprintf("%s-%s", goos, goarch),
		fmt.Sprintf("%s_%s", goos, goarch),
		fmt.Sprintf("%s-%s", strings.ToLower(goos), strings.ToLower(goarch)),
	}

	for _, pattern := range patterns {
		for _, asset := range r.Assets {
			if strings.Contains(strings.ToLower(asset.Name), pattern) {
				return &asset, nil
			}
		}
	}

	// Try finding by OS only (fallback)
	for _, asset := range r.Assets {
		if strings.HasPrefix(strings.ToLower(asset.Name), strings.ToLower(goos)) {
			return &asset, nil
		}
	}

	return nil, fmt.Errorf("no binary found for platform %s/%s", goos, goarch)
}

// IsNewer returns true if the release version is newer than the current version.
// This does a simple comparison - for production you'd want proper semver comparison.
func (r *Release) IsNewer(currentVersion string) bool {
	if currentVersion == "dev" || currentVersion == "unknown" {
		return true
	}
	// Strip 'v' prefix if present
	current := strings.TrimPrefix(currentVersion, "v")
	latest := strings.TrimPrefix(r.TagName, "v")
	return current != latest
}
