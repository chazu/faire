# Self-Update Mechanism Design

## Overview

Design a goreleaser-compatible self-update mechanism for the `svf` CLI tool that enables users to upgrade to the latest version directly from the command line.

## Requirements

From faire-o8v.2:
- Check GitHub releases for latest version
- Compare with current version
- Download release asset for current platform
- Verify signature if available
- Replace binary
- Rollback on failure

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         upgrade command                          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Version Checker                              │
│  - Query GitHub Releases API                                    │
│  - Parse semver versions                                         │
│  - Compare with current version                                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Release Finder                              │
│  - Match current platform (GOOS/GOARCH)                         │
│  - Locate correct binary asset                                   │
│  - Find checksum/signature files                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Downloader                                  │
│  - Download binary to temp location                              │
│  - Download checksum/signature                                   │
│  - Verify integrity                                              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Installer                                   │
│  - Backup current binary                                         │
│  - Replace with new binary                                       │
│  - Set executable permissions                                    │
│  - Verify new binary works                                       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Rollback (on failure)                       │
│  - Restore backup                                                │
│  - Clean up temp files                                           │
│  - Report error                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Component Design

### 1. Package: `internal/upgrade`

```go
package upgrade

// Release represents a GitHub release
type Release struct {
    TagName     string    `json:"tag_name"`
    Name        string    `json:"name"`
    PublishedAt time.Time `json:"published_at"`
    Body        string    `json:"body"`
    Assets      []Asset   `json:"assets"`
    Prerelease  bool      `json:"prerelease"`
}

// Asset represents a release asset
type Asset struct {
    Name string `json:"name"`
    URL  string `json:"browser_download_url"`
    Size int64  `json:"size"`
}

// VersionInfo holds current version information
type VersionInfo struct {
    CurrentVersion string
    LatestVersion  string
    UpdateAvailable bool
    Prerelease      bool
}

// Platform identifies the current platform
type Platform struct {
    OS   string // runtime.GOOS
    Arch string // runtime.GOARCH
}
```

### 2. Version Checker (`checker.go`)

```go
package upgrade

type Checker struct {
    repoOwner    string // e.g., "chazu"
    repoName     string // e.g., "faire"
    includePre   bool   // include pre-releases
    httpClient   *http.Client
}

func (c *Checker) CheckLatest(ctx context.Context) (*Release, error)
func (c *Checker) CompareVersions(current, latest string) (int, error)
```

**Responsibilities:**
- Query GitHub Releases API: `GET /repos/{owner}/{repo}/releases`
- Filter by release criteria (latest stable, or include pre-releases)
- Parse and compare semver versions
- Return update availability status

### 3. Release Finder (`finder.go`)

```go
type AssetFinder struct {
    platform Platform
}

func (f *AssetFinder) FindBinary(release *Release) (*Asset, error)
func (f *AssetFinder) FindChecksum(release *Release) (*Asset, error)
func (f *AssetFinder) FindSignature(release *Release) (*Asset, error)
```

**Asset Naming Convention (goreleaser standard):**
```
svf_0.48.0_darwin_amd64.tar.gz
svf_0.48.0_darwin_arm64.tar.gz
svf_0.48.0_linux_amd64.tar.gz
svf_0.48.0_windows_amd64.zip
svf_0.48.0_checksums.txt
svf_0.48.0_checksums.txt.sig
```

**Supported Platforms:**
- darwin/amd64, darwin/arm64
- linux/amd64, linux/arm64
- windows/amd64

### 4. Downloader (`downloader.go`)

```go
type Downloader struct {
    httpClient   *http.Client
    maxRetries   int
    progressHook func(bytesDownloaded, totalBytes int64)
}

func (d *Downloader) Download(ctx context.Context, url, destPath string) error
func (d *Downloader) VerifyChecksum(filePath, checksumsPath, expectedBinary string) error
func (d *Downloader) VerifySignature(filePath, signaturePath, publicKeyPath string) error
```

**Download Flow:**
1. Create temp directory for downloads
2. Download binary asset
3. Download checksums.txt (if available)
4. Verify SHA256 checksum
5. Verify signature if public key available
6. Return path to verified binary

### 5. Installer (`installer.go`)

```go
type Installer struct {
    currentBinaryPath string
    backupDir         string
}

func (i *Installer) Install(newBinaryPath string) error
func (i *Installer) Backup() error
func (i *Installer) Rollback() error
```

**Installation Flow:**
1. Create backup of current binary
2. Extract binary from archive (tar.gz/zip)
3. Replace current binary
4. Set executable permissions (0755)
5. Verify new binary runs (exec `svf version`)
6. On failure: restore from backup

### 6. CLI Command (`internal/cli/upgrade.go`)

```go
package cli

var upgradeCmd = &cobra.Command{
    Use:   "upgrade",
    Short: "Update to the latest version",
    RunE: func(cmd *cobra.Command, args []string) error {
        return runUpgrade(cmd.Context())
    },
}

func runUpgrade(ctx context.Context) error {
    // 1. Check current version
    // 2. Fetch latest release
    // 3. Compare versions
    // 4. If update available:
    //    - Prompt user (unless --yes)
    //    - Download
    //    - Verify
    //    - Install
    // 5. Report result
    return nil
}
```

**Flags:**
- `--check-only` - Check for updates without installing
- `--yes` - Skip confirmation prompt
- `--pre` - Include pre-releases
- `--no-verify` - Skip signature verification

## Exit Codes

Following the task requirements:
- `0` - Success (upgraded or already up-to-date)
- `1` - Generic error
- `2` - Network error (couldn't reach GitHub)
- `3` - Verification failed (checksum/signature mismatch)
- `4` - Installation failed (rollback attempted)
- `5` - Already on latest version (with --check-only)

## Security Considerations

1. **HTTPS Only**: All downloads over HTTPS
2. **Checksum Verification**: Always verify SHA256 if available
3. **Signature Verification**: Optional GPG signature verification
4. **Permissions**: Require write access to binary location
5. **Backup**: Always keep backup for rollback
6. **Checksum Injection**: Download checksums from same release, not separate source

## Error Handling

```go
type UpgradeError struct {
    Code    int
    Message string
    Cause   error
}

func (e *UpgradeError) Error() string {
    return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Cause)
}

// Specific error types
var (
    ErrNetworkFailure     = &UpgradeError{Code: 2, Message: "Network error"}
    ErrVerificationFailed = &UpgradeError{Code: 3, Message: "Verification failed"}
    ErrInstallFailed      = &UpgradeError{Code: 4, Message: "Installation failed"}
)
```

## goreleaser Configuration

Create `.goreleaser.yml`:

```yaml
project_name: svf
before:
  hooks:
    - go mod tidy
builds:
  - env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ldflags:
      - -X main.Version={{.Version}}
      - -X main.Commit={{.Commit}}
      - -X main.Date={{.Date}}
    main: ./cmd/svf
archives:
  - format: tar.gz
    format_overrides:
      - goos: windows
        format: zip
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    wrap_in_directory: false
    files:
      - LICENSE*
      - README*
checksum:
  name_template: "{{ .ProjectName }}_{{ .Version }}_checksums.txt"
  algorithm: sha256
release:
  github:
    owner: chazu
    name: faire
  draft: false
  prerelease: auto
```

## Implementation Checklist

For faire-o8v.3 (Implement upgrade command):

1. [ ] Create `internal/upgrade` package
2. [ ] Implement version checker with GitHub API
3. [ ] Implement asset finder for platform matching
4. [ ] Implement downloader with progress tracking
5. [ ] Implement installer with backup/rollback
6. [ ] Create `upgrade` CLI command
7. [ ] Add flags: --check-only, --yes, --pre, --no-verify
8. [ ] Add tests for each component
9. [ ] Create `.goreleaser.yml` configuration
10. [ ] Add upgrade check to version command output
11. [ ] Document upgrade flow in README

## Open Questions

1. **GitHub Token**: Should we support `GITHUB_TOKEN` for higher rate limits?
   - Recommendation: Yes, for private repos or high-usage scenarios
2. **Proxy Support**: Should we support HTTP proxies?
   - Recommendation: Use standard `HTTP_PROXY` env var
3. **Offline Detection**: How to detect if running in offline mode?
   - Recommendation: Check for existing `--local` flag pattern in codebase
4. **Auto-update**: Should we periodically check for updates?
   - Recommendation: No, explicit opt-in only for security

## Testing Strategy

```go
// Mock GitHub API for testing
type MockGitHubAPI struct {
    releases []Release
    error    error
}

// Test with fake release files
func TestDownloadAndInstall(t *testing.T) {
    // Create temp directory with fake release
    // Test download, verification, install
    // Verify rollback on failure
}

// Integration tests with real releases
func TestUpgradeIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    // Test against real GitHub releases
}
```
