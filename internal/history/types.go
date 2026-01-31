package history

import "time"

// HistoryLine represents a single command from shell history
type HistoryLine struct {
	Timestamp time.Time
	Command   string
	Shell     string // "bash", "zsh"
}

// Parser defines the interface for shell history parsers
type Parser interface {
	Parse(path string) ([]HistoryLine, error)
	DetectPath() (string, error)
}

// FilterOptions specifies filtering criteria for history lines
type FilterOptions struct {
	Since     time.Time // Only include commands after this time
	MaxLines  int       // Maximum number of lines to return (0 = no limit)
	RemoveDup bool      // Remove consecutive duplicate commands
}
