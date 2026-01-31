// Package cli provides global state and utilities for CLI commands.
package cli

import (
	"sync"

	"github.com/spf13/cobra"
)

var (
	// NoTUI indicates that TUI/interactive mode should be disabled.
	// This is set by the global --no-tui flag.
	NoTUI bool

	// noTUIMutex protects NoTUI for concurrent access.
	noTUIMutex sync.RWMutex
)

// AddGlobalFlags adds global flags to a command.
func AddGlobalFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolVar(&NoTUI, "no-tui", false,
		"disable TUI/interactive mode; use plain text or JSON output")
}

// IsNoTUI returns true if TUI mode is disabled.
func IsNoTUI() bool {
	noTUIMutex.RLock()
	defer noTUIMutex.RUnlock()
	return NoTUI
}
