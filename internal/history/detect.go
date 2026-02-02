package history

import (
	"os"
	"path/filepath"
)

// DetectHistoryFiles returns all found shell history files.
// It checks common locations for bash and zsh history files.
func DetectHistoryFiles() []string {
	var found []string
	home, err := os.UserHomeDir()
	if err != nil {
		return found
	}

	// Bash history locations
	bashLocations := []string{
		filepath.Join(home, ".bash_history"),
		filepath.Join(home, ".local/share/bash/history"),
	}

	// Zsh history locations
	zshLocations := []string{
		filepath.Join(home, ".zhistory"),
		filepath.Join(home, ".zsh_history"),
		filepath.Join(home, ".histfile"),
	}

	// Check each location
	for _, path := range bashLocations {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			found = append(found, path)
		}
	}

	for _, path := range zshLocations {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			found = append(found, path)
		}
	}

	return found
}
