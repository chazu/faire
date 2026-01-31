package gitrepo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupGitConfig configures git user.name and user.email for testing.
func setupGitConfig(tmpDir string) {
	ctx := context.Background()
	// Set git config for the test repository
	cmd := exec.CommandContext(ctx, "git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	cmd = exec.CommandContext(ctx, "git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	_ = cmd.Run()
}

func TestNew(t *testing.T) {
	path := "/test/path"
	repo := New(path)

	if repo == nil {
		t.Fatal("New() returned nil")
	}

	if repo.Path() != path {
		t.Errorf("Path() = %s, want %s", repo.Path(), path)
	}
}

func TestGitRepo_Init(t *testing.T) {
	tmpDir := t.TempDir()
	repo := New(tmpDir)
	ctx := context.Background()

	// Init should create .git directory
	err := repo.Init(ctx, InitOptions{})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Check .git directory exists
	gitDir := filepath.Join(tmpDir, ".git")
	if info, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Error(".git directory was not created")
	} else if !info.IsDir() {
		t.Error(".git path is not a directory")
	}
}

func TestGitRepo_IsInitialized(t *testing.T) {
	t.Run("not initialized", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := New(tmpDir)
		ctx := context.Background()

		if repo.IsInitialized(ctx) {
			t.Error("IsInitialized() = true, want false")
		}
	})

	t.Run("initialized", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := New(tmpDir)
		ctx := context.Background()

		_ = repo.Init(ctx, InitOptions{})

		if !repo.IsInitialized(ctx) {
			t.Error("IsInitialized() = false, want true")
		}
	})
}

func TestGitRepo_Status(t *testing.T) {
	tmpDir := t.TempDir()
	repo := New(tmpDir)
	ctx := context.Background()

	_ = repo.Init(ctx, InitOptions{})
	setupGitConfig(tmpDir)

	// Create and commit a file so HEAD exists
	testFile := filepath.Join(tmpDir, "initial.txt")
	_ = os.WriteFile(testFile, []byte("initial"), 0644)
	_ = repo.Add(ctx, "initial.txt")
	_, _ = repo.CommitAll(ctx, "initial commit")

	status, err := repo.Status(ctx)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}

	// Should have a branch (main or master)
	if status.Branch == "" {
		t.Error("Status().Branch is empty")
	}

	// Clean repo should not be dirty
	if status.Dirty {
		t.Error("Status().Dirty = true, want false (clean repo)")
	}
}

func TestGitRepo_GetCurrentBranch(t *testing.T) {
	tmpDir := t.TempDir()
	repo := New(tmpDir)
	ctx := context.Background()

	_ = repo.Init(ctx, InitOptions{})
	setupGitConfig(tmpDir)

	// Create and commit a file so HEAD exists
	testFile := filepath.Join(tmpDir, "initial.txt")
	_ = os.WriteFile(testFile, []byte("initial"), 0644)
	_ = repo.Add(ctx, "initial.txt")
	_, _ = repo.CommitAll(ctx, "initial commit")

	branch, err := repo.GetCurrentBranch(ctx)
	if err != nil {
		t.Fatalf("GetCurrentBranch() error = %v", err)
	}

	if branch == "" {
		t.Error("GetCurrentBranch() returned empty string")
	}

	// Branch should be main or master
	if branch != "main" && branch != "master" {
		t.Errorf("GetCurrentBranch() = %s, want main or master", branch)
	}
}

func TestGitRepo_Add_Commit(t *testing.T) {
	tmpDir := t.TempDir()
	repo := New(tmpDir)
	ctx := context.Background()

	_ = repo.Init(ctx, InitOptions{})
	setupGitConfig(tmpDir)

	// Create initial commit first
	testFile := filepath.Join(tmpDir, "initial.txt")
	_ = os.WriteFile(testFile, []byte("initial"), 0644)
	_ = repo.Add(ctx, "initial.txt")
	_, _ = repo.CommitAll(ctx, "initial commit")

	// Now test adding a new file
	testFile2 := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile2, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Add the file
	err = repo.Add(ctx, "test.txt")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Commit
	hash, err := repo.CommitAll(ctx, "test commit")
	if err != nil {
		t.Fatalf("CommitAll() error = %v", err)
	}

	if hash == "" {
		t.Error("CommitAll() returned empty hash")
	}

	// Status should be clean now
	status, _ := repo.Status(ctx)
	if status.Dirty {
		t.Error("repo is dirty after commit")
	}
}

func TestGitRepo_AddAll(t *testing.T) {
	tmpDir := t.TempDir()
	repo := New(tmpDir)
	ctx := context.Background()

	_ = repo.Init(ctx, InitOptions{})
	setupGitConfig(tmpDir)

	// Create initial commit first
	testFile := filepath.Join(tmpDir, "initial.txt")
	_ = os.WriteFile(testFile, []byte("initial"), 0644)
	_ = repo.Add(ctx, "initial.txt")
	_, _ = repo.CommitAll(ctx, "initial commit")

	// Create multiple test files
	for i := 0; i < 3; i++ {
		path := filepath.Join(tmpDir, "file"+string(rune('1'+i))+".txt")
		err := os.WriteFile(path, []byte("content"), 0644)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	// Add all
	err := repo.AddAll(ctx)
	if err != nil {
		t.Fatalf("AddAll() error = %v", err)
	}

	// Commit to verify files were staged
	_, err = repo.CommitAll(ctx, "add all test")
	if err != nil {
		t.Fatalf("CommitAll() error = %v", err)
	}

	// Status should be clean
	status, _ := repo.Status(ctx)
	if status.Dirty {
		t.Error("repo is dirty after add all and commit")
	}
}

func TestGitRepo_Close(t *testing.T) {
	tmpDir := t.TempDir()
	repo := New(tmpDir)

	err := repo.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestIntegrationStrategy_String(t *testing.T) {
	tests := []struct {
		name string
		s    IntegrationStrategy
		want string
	}{
		{"fast-forward", FastForwardOnly, "ff-only"},
		{"rebase", Rebase, "rebase"},
		{"merge", Merge, "merge"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(tt.s); got != tt.want {
				t.Errorf("IntegrationStrategy = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestConflictError_Error(t *testing.T) {
	tests := []struct {
		name       string
		files      []string
		wantContains string
	}{
		{"no files", []string{}, "merge conflicts detected"},
		{"one file", []string{"file.txt"}, "merge conflict in file.txt"},
		{"multiple files", []string{"a.txt", "b.txt"}, "merge conflicts in 2 files: a.txt, b.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ConflictError{Files: tt.files}
			if got := err.Error(); got != tt.wantContains {
				t.Errorf("ConflictError.Error() = %q, want %q", got, tt.wantContains)
			}
		})
	}
}

func TestIsConflictError(t *testing.T) {
	t.Run("is conflict error", func(t *testing.T) {
		err := &ConflictError{Files: []string{"file.txt"}}
		if !IsConflictError(err) {
			t.Error("IsConflictError() = false, want true")
		}
	})

	t.Run("is not conflict error", func(t *testing.T) {
		err := &GitError{}
		if IsConflictError(err) {
			t.Error("IsConflictError() = true, want false")
		}
	})

	t.Run("nil error", func(t *testing.T) {
		if IsConflictError(nil) {
			t.Error("IsConflictError() = true, want false")
		}
	})
}
