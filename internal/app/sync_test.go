package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/chazuruo/faire/internal/config"
	"github.com/chazuruo/faire/internal/gitrepo"
)

func TestSync_NotARepo(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create a config pointing to the temp directory
	cfgPath := filepath.Join(tmpDir, "config.toml")
	cfg := config.DefaultConfig()
	cfg.Repo.Path = tmpDir
	cfg.Identity.Path = "test"
	cfgData := `repo.path = "` + tmpDir + `"
identity.path = "test"
repo.remote = "origin"
repo.branch = "main"`
	_ = os.WriteFile(cfgPath, []byte(cfgData), 0644)

	opts := SyncOptions{
		ConfigPath: cfgPath,
	}

	result, err := Sync(ctx, opts)
	if err == nil {
		t.Error("Sync() should return error for non-repo")
	}

	if result.Success {
		t.Error("Sync() result.Success = true, want false")
	}

	if result.Error == "" {
		t.Error("Sync() result.Error is empty")
	}
}

func TestSync_DirtyRepo(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Initialize a git repo
	repo := gitrepo.New(tmpDir)
	_ = repo.Init(ctx, gitrepo.InitOptions{})

	// Create initial commit
	testFile := filepath.Join(tmpDir, "initial.txt")
	_ = os.WriteFile(testFile, []byte("initial"), 0644)
	_ = repo.Add(ctx, "initial.txt")
	_, _ = repo.CommitAll(ctx, "initial commit")

	// Create an uncommitted change
	dirtyFile := filepath.Join(tmpDir, "dirty.txt")
	_ = os.WriteFile(dirtyFile, []byte("dirty"), 0644)

	// Create valid config (identity.path is required for validation)
	cfgPath := filepath.Join(tmpDir, "config.toml")
	cfg := config.DefaultConfig()
	cfg.Repo.Path = tmpDir
	cfg.Identity.Path = "test"
	cfgData := `repo.path = "` + tmpDir + `"
identity.path = "test"
repo.remote = "origin"
repo.branch = "main"`
	_ = os.WriteFile(cfgPath, []byte(cfgData), 0644)

	opts := SyncOptions{
		ConfigPath: cfgPath,
	}

	result, err := Sync(ctx, opts)
	if err == nil {
		t.Error("Sync() should return error for dirty repo")
	}

	if result.Success {
		t.Error("Sync() result.Success = true, want false")
	}

	if result.Error != "working directory has uncommitted changes" {
		t.Errorf("Sync() result.Error = %q, want %q", result.Error, "working directory has uncommitted changes")
	}
}

func TestSyncOptions_Defaults(t *testing.T) {
	opts := SyncOptions{}

	if opts.Strategy != "" {
		t.Errorf("SyncOptions.Strategy = %s, want empty", opts.Strategy)
	}

	if opts.Remote != "" {
		t.Errorf("SyncOptions.Remote = %s, want empty", opts.Remote)
	}

	if opts.Branch != "" {
		t.Errorf("SyncOptions.Branch = %s, want empty", opts.Branch)
	}

	if opts.NoFetch {
		t.Error("SyncOptions.NoFetch = true, want false")
	}

	if opts.NoPush {
		t.Error("SyncOptions.NoPush = true, want false")
	}

	if opts.Push {
		t.Error("SyncOptions.Push = true, want false")
	}

	if opts.JSONOutput {
		t.Error("SyncOptions.JSONOutput = true, want false")
	}
}

func TestSyncOutput_Empty(t *testing.T) {
	output := &SyncOutput{}

	if output.Success {
		t.Error("SyncOutput.Success = true, want false")
	}

	if output.Branch != "" {
		t.Errorf("SyncOutput.Branch = %s, want empty", output.Branch)
	}

	if output.Strategy != "" {
		t.Errorf("SyncOutput.Strategy = %s, want empty", output.Strategy)
	}

	if output.Ahead != 0 {
		t.Errorf("SyncOutput.Ahead = %d, want 0", output.Ahead)
	}

	if output.Behind != 0 {
		t.Errorf("SyncOutput.Behind = %d, want 0", output.Behind)
	}

	if output.NewFiles != nil {
		t.Errorf("SyncOutput.NewFiles = %v, want nil", output.NewFiles)
	}

	if output.UpdatedFiles != nil {
		t.Errorf("SyncOutput.UpdatedFiles = %v, want nil", output.UpdatedFiles)
	}

	if output.DeletedFiles != nil {
		t.Errorf("SyncOutput.DeletedFiles = %v, want nil", output.DeletedFiles)
	}

	if output.Conflicts != nil {
		t.Errorf("SyncOutput.Conflicts = %v, want nil", output.Conflicts)
	}
}
