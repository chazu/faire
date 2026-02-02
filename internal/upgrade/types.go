// Package upgrade implements self-update functionality for svf.
package upgrade

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
