// Package cli provides tests for CLI commands.
package cli

import (
	"strings"
	"testing"

	"github.com/chazuruo/svf/internal/config"
)

// TestList_RepoNotInitialized verifies that list provides a helpful
// error message when the repository is not initialized.
// This is a regression test for fa-te77.
func TestList_RepoNotInitialized(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a config that points to a non-existent repo
	cfg := config.DefaultConfig()
	cfg.Repo.Path = tmpDir + "/nonexistent-repo"
	cfg.Identity.Path = "testuser"
	cfg.Git.AuthorName = "Test User"
	cfg.Git.AuthorEmail = "test@example.com"

	configPath := tmpDir + "/config.toml"
	if err := config.Write(configPath, cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	opts := &ListOptions{
		ConfigPath: configPath,
	}

	err := runList(opts)
	if err == nil {
		t.Fatal("runList() expected error for non-initialized repo, got nil")
	}

	// Verify the error message is helpful
	errMsg := err.Error()
	if !strings.Contains(errMsg, "repository not initialized") {
		t.Errorf("error message should mention 'repository not initialized', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, cfg.Repo.Path) {
		t.Errorf("error message should include the repo path, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "config") {
		t.Errorf("error message should mention checking the config, got: %s", errMsg)
	}
}
