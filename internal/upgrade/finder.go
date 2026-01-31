package upgrade

import (
	"fmt"
	"path/filepath"
	"strings"
)

// AssetFinder finds the appropriate assets for a release.
type AssetFinder struct {
	platform Platform
	binaryName string
}

// NewAssetFinder creates a new AssetFinder.
func NewAssetFinder(platform Platform, binaryName string) *AssetFinder {
	return &AssetFinder{
		platform:  platform,
		binaryName: binaryName,
	}
}

// FindBinary finds the binary asset for the current platform.
func (f *AssetFinder) FindBinary(release *Release) (*Asset, error) {
	// goreleaser naming: {binary}_{version}_{os}_{arch}.{ext}
	pattern := fmt.Sprintf("%s_%s_%s.%s",
		f.binaryName,
		release.TagName,
		f.platform.OS,
		f.platform.Arch,
	)
	ext := f.platform.ArchiveExtension()
	fullPattern := pattern + strings.TrimPrefix(ext, ".")

	for _, asset := range release.Assets {
		if asset.Name == fullPattern {
			return &asset, nil
		}
	}

	return nil, NewError(ExitGenericError, fmt.Sprintf("No binary found for platform %s", f.platform.String()), nil)
}

// FindChecksum finds the checksums file.
func (f *AssetFinder) FindChecksum(release *Release) (*Asset, error) {
	// goreleaser naming: {binary}_{version}_checksums.txt
	pattern := fmt.Sprintf("%s_%s_checksums.txt", f.binaryName, release.TagName)

	for _, asset := range release.Assets {
		if asset.Name == pattern {
			return &asset, nil
		}
	}

	// Checksums are optional
	return nil, nil
}

// FindSignature finds the signature file.
func (f *AssetFinder) FindSignature(release *Release) (*Asset, error) {
	// goreleaser naming: {binary}_{version}_checksums.txt.sig
	pattern := fmt.Sprintf("%s_%s_checksums.txt.sig", f.binaryName, release.TagName)

	for _, asset := range release.Assets {
		if asset.Name == pattern {
			return &asset, nil
		}
	}

	// Signatures are optional
	return nil, nil
}

// ExtractBinaryName extracts the binary name from the archive filename.
// E.g., "svf_1.0.0_darwin_arm64.tar.gz" -> "svf"
func ExtractBinaryName(archivePath string) string {
	base := filepath.Base(archivePath)
	// Remove version and platform suffixes
	// svf_1.0.0_darwin_arm64.tar.gz -> svf
	parts := strings.Split(base, "_")
	if len(parts) >= 1 {
		return parts[0]
	}
	return "svf"
}
