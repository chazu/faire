package history

import "time"

// HistoryLine represents a single command from shell history.
type HistoryLine struct {
	Timestamp time.Time
	Command   string
	Shell     string // bash, zsh
}

// Parser is the interface for parsing shell history files.
type Parser interface {
	// Parse reads the history file at the given path and returns parsed history lines.
	Parse(path string) ([]HistoryLine, error)

	// DetectPath returns the default path to the history file for this shell.
	DetectPath() (string, error)
}
