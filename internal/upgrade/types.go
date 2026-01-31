// Package upgrade implements self-update functionality for svf.
package upgrade

import "time"

// Release represents a GitHub release.
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	Body        string    `json:"body"`
	Assets      []Asset   `json:"assets"`
	Prerelease  bool      `json:"prerelease"`
}

// Asset represents a release asset.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// VersionInfo holds version comparison results.
type VersionInfo struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
	Prerelease      bool
}

// Platform identifies the current platform.
type Platform struct {
	OS   string // runtime.GOOS
	Arch string // runtime.GOARCH
}

// NewPlatform returns the current platform.
func NewPlatform() Platform {
	return Platform{
		OS:   goos,
		Arch: goarch,
	}
}

// String returns the platform string in the format "os_arch".
func (p Platform) String() string {
	return p.OS + "_" + p.Arch
}

// ArchiveExtension returns the appropriate archive extension for the platform.
func (p Platform) ArchiveExtension() string {
	if p.OS == "windows" {
		return ".zip"
	}
	return ".tar.gz"
}
