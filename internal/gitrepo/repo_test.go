package gitrepo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewRepo tests creating a new Repo instance.
func TestNewRepo(t *testing.T) {
	repo := New("/tmp/test")
	if repo == nil {
		t.Fatal("New() returned nil")
	}
	if repo.Path() != "/tmp/test" {
		t.Errorf("Path() = %q, want %q", repo.Path(), "/tmp/test")
	}
}

// TestInitRepo tests initializing a new repository.
func TestInitRepo(t *testing.T) {
	t.Run("basic init", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := context.Background()

		repo, err := InitRepo(ctx, tmpDir, InitOptions{})
		if err != nil {
			t.Fatalf("InitRepo() failed: %v", err)
		}
		defer repo.Close()

		if !repo.IsInitialized(ctx) {
			t.Error("IsInitialized() returned false, want true")
		}

		// Check .git directory exists
		gitDir := filepath.Join(tmpDir, ".git")
		if info, err := os.Stat(gitDir); err != nil {
			t.Errorf(".git directory does not exist: %v", err)
		} else if !info.IsDir() {
			t.Error(".git exists but is not a directory")
		}
	})

	t.Run("init with default branch", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := context.Background()

		repo, err := InitRepo(ctx, tmpDir, InitOptions{DefaultBranch: "main"})
		if err != nil {
			t.Fatalf("InitRepo() failed: %v", err)
		}
		defer repo.Close()

		// Verify branch is set by checking HEAD
		_, output, err := repo.(*gitRepo).runGit(ctx, "symbolic-ref", "--short", "HEAD")
		if err != nil {
			t.Fatalf("Failed to get HEAD: %v", err)
		}

		branch := strings.TrimSpace(output)
		if branch != "main" {
			t.Errorf("Default branch = %q, want %q", branch, "main")
		}
	})

	t.Run("bare repository", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := context.Background()

		repo, err := InitRepo(ctx, tmpDir, InitOptions{Bare: true})
		if err != nil {
			t.Fatalf("InitRepo() failed: %v", err)
		}
		defer repo.Close()

		// Bare repos have config directly in the directory, not in .git
		if _, err := os.Stat(filepath.Join(tmpDir, "config")); err != nil {
			t.Error("Bare repo should have config file in root directory")
		}
		if _, err := os.Stat(filepath.Join(tmpDir, ".git")); err == nil {
			t.Error("Bare repo should not have .git subdirectory")
		}
	})
}

// TestOpen tests opening an existing repository.
func TestOpen(t *testing.T) {
	t.Run("open existing repo", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := context.Background()

		// First init a repo
		_, err := InitRepo(ctx, tmpDir, InitOptions{})
		if err != nil {
			t.Fatalf("Failed to init repo: %v", err)
		}

		// Now open it
		repo, err := Open(tmpDir)
		if err != nil {
			t.Fatalf("Open() failed: %v", err)
		}
		defer repo.Close()

		if !repo.IsInitialized(ctx) {
			t.Error("Opened repo is not initialized")
		}
	})

	t.Run("open non-repo fails", func(t *testing.T) {
		tmpDir := t.TempDir()

		_, err := Open(tmpDir)
		if err == nil {
			t.Error("Open() should fail for non-repo directory")
		}
	})
}

// TestStatus tests repository status checking.
func TestStatus(t *testing.T) {
	t.Run("clean repo status", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := context.Background()

		repo, err := InitRepo(ctx, tmpDir, InitOptions{DefaultBranch: "main"})
		if err != nil {
			t.Fatalf("InitRepo() failed: %v", err)
		}
		defer repo.Close()

		status, err := repo.Status(ctx)
		if err != nil {
			t.Fatalf("Status() failed: %v", err)
		}

		if status.Branch != "main" {
			t.Errorf("Branch = %q, want %q", status.Branch, "main")
		}
		if status.Dirty {
			t.Error("Dirty = true, want false for clean repo")
		}
		if len(status.Entries) != 0 {
			t.Errorf("Entries = %d, want 0", len(status.Entries))
		}
	})

	t.Run("dirty repo detection", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := context.Background()

		repo, err := InitRepo(ctx, tmpDir, InitOptions{})
		if err != nil {
			t.Fatalf("InitRepo() failed: %v", err)
		}
		defer repo.Close()

		// Create an untracked file
		testFile := filepath.Join(tmpDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		status, err := repo.Status(ctx)
		if err != nil {
			t.Fatalf("Status() failed: %v", err)
		}

		if !status.Dirty {
			t.Error("Dirty = false, want true for repo with untracked file")
		}
	})

	t.Run("staged file detection", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := context.Background()

		repo, err := InitRepo(ctx, tmpDir, InitOptions{})
		if err != nil {
			t.Fatalf("InitRepo() failed: %v", err)
		}
		defer repo.Close()

		// Create and stage a file
		testFile := filepath.Join(tmpDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		_, _, err = repo.(*gitRepo).runGit(ctx, "add", "test.txt")
		if err != nil {
			t.Fatalf("Failed to stage file: %v", err)
		}

		status, err := repo.Status(ctx)
		if err != nil {
			t.Fatalf("Status() failed: %v", err)
		}

		if !status.Dirty {
			t.Error("Dirty = false, want true for repo with staged file")
		}

		// Find the staged entry
		found := false
		for _, entry := range status.Entries {
			if strings.Contains(entry.Path, "test.txt") {
				found = true
				// Staged files have X='A' (added) and H=' ' (no worktree changes)
				if entry.X != 'A' && entry.X != 'M' {
					t.Errorf("Staged file X = %c, want 'A' or 'M'", entry.X)
				}
				break
			}
		}
		if !found {
			t.Error("Staged file not found in status entries")
		}
	})

	t.Run("modified file detection", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := context.Background()

		repo, err := InitRepo(ctx, tmpDir, InitOptions{})
		if err != nil {
			t.Fatalf("InitRepo() failed: %v", err)
		}
		defer repo.Close()

		// Create, commit, then modify a file
		testFile := filepath.Join(tmpDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Configure git for commits
		repo.(*gitRepo).runGit(ctx, "config", "user.name", "Test User")
		repo.(*gitRepo).runGit(ctx, "config", "user.email", "test@example.com")

		_, _, err = repo.(*gitRepo).runGit(ctx, "add", "test.txt")
		if err != nil {
			t.Fatalf("Failed to stage file: %v", err)
		}

		_, _, err = repo.(*gitRepo).runGit(ctx, "commit", "-m", "initial")
		if err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}

		// Modify the file
		if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
			t.Fatalf("Failed to modify file: %v", err)
		}

		status, err := repo.Status(ctx)
		if err != nil {
			t.Fatalf("Status() failed: %v", err)
		}

		if !status.Dirty {
			t.Error("Dirty = false, want true for repo with modified file")
		}

		// Find the modified entry
		found := false
		for _, entry := range status.Entries {
			t.Logf("Entry: Path='%s' X='%c' Y='%c'", entry.Path, entry.X, entry.Y)
			if strings.Contains(entry.Path, "test.txt") {
				found = true
				// Modified files have Y='M'
				if entry.Y != 'M' {
					t.Errorf("Modified file Y = %c, want 'M'", entry.Y)
				}
				break
			}
		}
		if !found {
			t.Error("Modified file not found in status entries")
		}
	})
}

// TestGitError tests the GitError wrapper.
func TestGitError(t *testing.T) {
	t.Run("git error message", func(t *testing.T) {
		err := &GitError{
			Args:     []string{"status"},
			Err:      &testError{"not a git repository"},
			ExitCode: 128,
		}

		msg := err.Error()
		if !strings.Contains(msg, "git status") {
			t.Errorf("Error message should contain command, got: %s", msg)
		}
		if !strings.Contains(msg, "not a git repository") {
			t.Errorf("Error message should contain underlying error, got: %s", msg)
		}
	})

	t.Run("unwrap", func(t *testing.T) {
		underlying := &testError{"underlying error"}
		err := &GitError{
			Args: []string{"status"},
			Err:  underlying,
		}

		if err.Unwrap() != underlying {
			t.Error("Unwrap() should return the underlying error")
		}
	})
}

// testError is a simple error type for testing.
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
