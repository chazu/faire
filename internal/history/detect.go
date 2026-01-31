package history

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ShellType represents the detected shell type
type ShellType string

const (
	ShellBash ShellType = "bash"
	ShellZsh  ShellType = "zsh"
	ShellPwsh ShellType = "pwsh"
	ShellUnknown ShellType = ""
)

// DetectShell detects the current shell from environment
func DetectShell() ShellType {
	// Check SHELL environment variable
	shellPath := os.Getenv("SHELL")
	if shellPath != "" {
		if strings.Contains(shellPath, "bash") {
			return ShellBash
		}
		if strings.Contains(shellPath, "zsh") {
			return ShellZsh
		}
		if strings.Contains(shellPath, "pwsh") || strings.Contains(shellPath, "powershell") {
			return ShellPwsh
		}
	}

	return ShellUnknown
}

// DetectHistoryFiles returns all found history files
func DetectHistoryFiles() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	var found []string

	// Bash history locations
	bashCandidates := []string{
		filepath.Join(homeDir, ".bash_history"),
		filepath.Join(homeDir, ".local", "share", "bash", "history"),
	}

	// Zsh history locations
	zshCandidates := []string{
		filepath.Join(homeDir, ".zhistory"),
		filepath.Join(homeDir, ".zsh_history"),
		filepath.Join(homeDir, ".histfile"),
		filepath.Join(homeDir, ".history"),
	}

	// PowerShell history locations
	pwshPath := filepath.Join(homeDir, ".config", "powershell", "PSReadLine", "ConsoleHost_history.txt")
	bashCandidates = append(bashCandidates, pwshPath)

	allCandidates := append(bashCandidates, zshCandidates...)

	for _, candidate := range allCandidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			found = append(found, candidate)
		}
	}

	return found
}

// NewParser creates a parser for the given shell type
func NewParser(shellType ShellType) (Parser, error) {
	switch shellType {
	case ShellBash:
		return &BashParser{}, nil
	case ShellZsh:
		return &ZshParser{}, nil
	case ShellPwsh:
		return nil, fmt.Errorf("PowerShell history parser not yet implemented")
	default:
		return nil, fmt.Errorf("unknown shell type: %s", shellType)
	}
}

// DetectAndParse automatically detects the shell and parses its history
func DetectAndParse() ([]HistoryLine, ShellType, error) {
	shellType := DetectShell()
	if shellType == ShellUnknown {
		// Try to find any history file
		files := DetectHistoryFiles()
		if len(files) == 0 {
			return nil, ShellUnknown, fmt.Errorf("no shell history files found")
		}

		// Heuristic: check filename to determine shell type
		for _, file := range files {
			if strings.Contains(file, "bash") {
				shellType = ShellBash
				break
			}
			if strings.Contains(file, "zsh") || strings.Contains(file, "zhistory") {
				shellType = ShellZsh
				break
			}
		}

		if shellType == ShellUnknown {
			// Default to zsh on macOS, bash elsewhere
			if os.Getenv("GOOS") == "darwin" {
				shellType = ShellZsh
			} else {
				shellType = ShellBash
			}
		}
	}

	parser, err := NewParser(shellType)
	if err != nil {
		return nil, shellType, err
	}

	path, err := parser.DetectPath()
	if err != nil {
		return nil, shellType, err
	}

	lines, err := parser.Parse(path)
	if err != nil {
		return nil, shellType, fmt.Errorf("failed to parse %s history from %s: %w", shellType, path, err)
	}

	return lines, shellType, nil
}
