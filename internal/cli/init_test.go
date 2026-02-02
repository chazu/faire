// Package cli provides tests for CLI commands.
package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/gitrepo"
)

// TestInitNonInteractive_RepoInitialized verifies that after init,
// the repository is actually initialized at the configured path.
// This is a regression test for fa-te77.
func TestInitNonInteractive_RepoInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")
	configPath := filepath.Join(tmpDir, "config.toml")

	opts := &InitOptions{
		ConfigPath:  configPath,
		Local:       repoPath,
		Identity:    "testuser",
		Mode:        "direct",
		AuthorName:  "Test User",
		AuthorEmail: "test@example.com",
		NoCommit:    true, // Skip commit to avoid git config issues
	}

	// Run init
	err := runInitNonInteractive(opts)
	if err != nil {
		t.Fatalf("runInitNonInteractive() error = %v", err)
	}

	// Verify config was written
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	// Verify the repo path in config matches what we specified
	if cfg.Repo.Path != repoPath {
		t.Errorf("config.Repo.Path = %s, want %s", cfg.Repo.Path, repoPath)
	}

	// Verify the repository is actually initialized at the configured path
	repo := gitrepo.New(cfg.Repo.Path)
	ctx := context.Background()
	if !repo.IsInitialized(ctx) {
		t.Errorf("repo.IsInitialized() = false, want true (repo path: %s)", cfg.Repo.Path)
	}

	// Verify the workflows directory was created
	workflowsDir := filepath.Join(cfg.Repo.Path, cfg.Workflows.Root)
	if _, err := os.Stat(workflowsDir); os.IsNotExist(err) {
		t.Errorf("workflows directory not created at %s", workflowsDir)
	}
}

// TestInitNonInteractive_ExistingRepo verifies that init works
// with an existing git repository.
func TestInitNonInteractive_ExistingRepo(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "existing-repo")
	configPath := filepath.Join(tmpDir, "config.toml")

	// Create the directory first
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Create an existing git repo
	repo := gitrepo.New(repoPath)
	ctx := context.Background()
	if err := repo.Init(ctx, gitrepo.InitOptions{}); err != nil {
		t.Fatalf("failed to create test repo: %v", err)
	}

	opts := &InitOptions{
		ConfigPath:  configPath,
		Local:       repoPath,
		Identity:    "testuser",
		Mode:        "direct",
		AuthorName:  "Test User",
		AuthorEmail: "test@example.com",
		NoCommit:    true,
	}

	// Run init with existing repo
	err := runInitNonInteractive(opts)
	if err != nil {
		t.Fatalf("runInitNonInteractive() error = %v", err)
	}

	// Verify the repository is still initialized
	if !repo.IsInitialized(ctx) {
		t.Errorf("repo.IsInitialized() = false after init, want true")
	}
}
