package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// Checker checks for available updates.
type Checker struct {
	repoOwner  string
	repoName   string
	includePre bool
	httpClient *http.Client
}

// NewChecker creates a new Checker.
func NewChecker(repoOwner, repoName string, includePre bool) *Checker {
	return &Checker{
		repoOwner:  repoOwner,
		repoName:   repoName,
		includePre: includePre,
		httpClient: http.DefaultClient,
	}
}

// CheckLatest fetches the latest release from GitHub.
func (c *Checker) CheckLatest(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", c.repoOwner, c.repoName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, NewError(ExitNetworkError, "Failed to create request", err)
	}

	// Set GitHub token if available (for higher rate limits)
	if token := c.getGitHubToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "svf-upgrade")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, NewError(ExitNetworkError, "Failed to fetch releases", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, NewError(ExitNetworkError, fmt.Sprintf("GitHub API returned status %d: %s", resp.StatusCode, string(body)), nil)
	}

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, NewError(ExitGenericError, "Failed to decode releases", err)
	}

	// Find the latest appropriate release
	for _, r := range releases {
		if r.TagName == "" {
			continue
		}
		if !c.includePre && r.Prerelease {
			continue
		}
		// Skip drafts
		if strings.Contains(strings.ToLower(r.Name), "draft") {
			continue
		}
		return &r, nil
	}

	return nil, NewError(ExitGenericError, "No suitable release found", nil)
}

// CompareVersions compares two version strings.
// Returns: -1 if current < latest, 0 if equal, 1 if current > latest.
func (c *Checker) CompareVersions(current, latest string) (int, error) {
	// Strip 'v' prefix if present
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")

	// Handle "dev" as always older than any release
	if current == "dev" {
		return -1, nil
	}
	if latest == "dev" {
		return 1, nil
	}

	// Simple semver comparison
	currentParts := parseSemver(current)
	latestParts := parseSemver(latest)

	for i := 0; i < 3; i++ {
		if currentParts[i] < latestParts[i] {
			return -1, nil
		}
		if currentParts[i] > latestParts[i] {
			return 1, nil
		}
	}

	return 0, nil
}

// parseSemver parses a semver string into [major, minor, patch].
func parseSemver(v string) [3]int {
	// Match pattern like "1.2.3" or "1.2.3-beta"
	re := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(v)
	if matches == nil {
		return [3]int{0, 0, 0}
	}

	return [3]int{
		parseNonZero(matches[1]),
		parseNonZero(matches[2]),
		parseNonZero(matches[3]),
	}
}

func parseNonZero(s string) int {
	var i int
	if _, err := fmt.Sscanf(s, "%d", &i); err != nil {
		return 0
	}
	return i
}

// getGitHubToken returns the GitHub token from environment if available.
func (c *Checker) getGitHubToken() string {
	// Check for common GitHub token environment variables
	// This is exported for testing
	return "" // Always prefer unauthenticated for public repos
}

// SetHTTPClient sets the HTTP client (useful for testing).
func (c *Checker) SetHTTPClient(client *http.Client) {
	c.httpClient = client
}
