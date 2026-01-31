package gitrepo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestRepo_Open tests opening a repository.
func TestRepo_Open(t *testing.T) {
	repo := Open("/tmp/test")
	if repo == nil {
		t.Fatal("Open returned nil")
	}
	if repo.Path() != "/tmp/test" {
		t.Errorf("Path() = %q, want %q", repo.Path(), "/tmp/test")
	}
}

// TestRepo_Init tests initializing a new repository.
func TestRepo_Init(t *testing.T) {
	t.Run("basic init", func(t *testing.T) {
		ctx := context.Background()
		tmpDir := t.TempDir()

		repo := Open(tmpDir)
		if repo.IsInitialized(ctx) {
			t.Fatal("Repository should not be initialized yet")
		}

		err := repo.Init(ctx, InitOptions{})
		if err != nil {
			t.Fatalf("Init failed: %v", err)
		}

		if !repo.IsInitialized(ctx) {
			t.Error("Repository should be initialized after Init()")
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
		ctx := context.Background()
		tmpDir := t.TempDir()

		repo := Open(tmpDir)
		err := repo.Init(ctx, InitOptions{DefaultBranch: "main"})
		if err != nil {
			t.Fatalf("Init failed: %v", err)
		}

		// Check current branch
		status, err := repo.Status(ctx)
		if err != nil {
			t.Fatalf("Status failed: %v", err)
		}

		if status.Branch != "main" {
			t.Errorf("Branch = %q, want %q", status.Branch, "main")
		}
	})

	t.Run("bare repository", func(t *testing.T) {
		ctx := context.Background()
		tmpDir := t.TempDir()

		repo := Open(tmpDir)
		err := repo.Init(ctx, InitOptions{Bare: true})
		if err != nil {
			t.Fatalf("Init failed: %v", err)
		}

		// For bare repos, .git is not a subdirectory but config is at root
		configPath := filepath.Join(tmpDir, "config")
		if _, err := os.Stat(configPath); err != nil {
			t.Errorf("Bare repo config does not exist: %v", err)
		}

		// HEAD should be in refs/heads
		headPath := filepath.Join(tmpDir, "HEAD")
		if _, err := os.Stat(headPath); err != nil {
			t.Errorf("Bare repo HEAD does not exist: %v", err)
		}
	})
}

// TestRepo_Status tests getting repository status.
func TestRepo_Status(t *testing.T) {
	t.Run("clean repository", func(t *testing.T) {
		ctx := context.Background()
		tmpDir := t.TempDir()

		repo, _ := InitRepo(ctx, tmpDir, InitOptions{})

		status, err := repo.Status(ctx)
		if err != nil {
			t.Fatalf("Status failed: %v", err)
		}

		if status.Dirty {
			t.Error("Repository should be clean")
		}
		if len(status.Entries) != 0 {
			t.Errorf("Entries = %d, want 0", len(status.Entries))
		}
	})

	t.Run("dirty repository", func(t *testing.T) {
		ctx := context.Background()
		tmpDir := t.TempDir()

		repo, _ := InitRepo(ctx, tmpDir, InitOptions{})

		// Create an untracked file
		testFile := filepath.Join(tmpDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		status, err := repo.Status(ctx)
		if err != nil {
			t.Fatalf("Status failed: %v", err)
		}

		if !status.Dirty {
			t.Error("Repository should be dirty")
		}
		if len(status.Entries) != 1 {
			t.Errorf("Entries = %d, want 1", len(status.Entries))
		}
		if len(status.Entries) > 0 && status.Entries[0].Path != "test.txt" {
			t.Errorf("First entry path = %q, want %q", status.Entries[0].Path, "test.txt")
		}
	})

	t.Run("staged file", func(t *testing.T) {
		ctx := context.Background()
		tmpDir := t.TempDir()

		repo, _ := InitRepo(ctx, tmpDir, InitOptions{})

		// Create and stage a file
		testFile := filepath.Join(tmpDir, "staged.txt")
		if err := os.WriteFile(testFile, []byte("staged"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Stage the file using git add
		if err := repo.(*gitRepo).runGitCmd(ctx, "add", "staged.txt"); err != nil {
			t.Fatalf("Failed to stage file: %v", err)
		}

		status, err := repo.Status(ctx)
		if err != nil {
			t.Fatalf("Status failed: %v", err)
		}

		if !status.Dirty {
			t.Error("Repository should be dirty")
		}

		// Find the staged entry
		var stagedEntry *StatusEntry
		for i := range status.Entries {
			if status.Entries[i].Path == "staged.txt" {
				stagedEntry = &status.Entries[i]
				break
			}
		}

		if stagedEntry == nil {
			t.Fatal("No status entry found for staged.txt")
		}

		if !stagedEntry.Staged {
			t.Error("Entry should be marked as staged")
		}
	})
}

// TestInitRepo tests the InitRepo convenience function.
func TestInitRepo(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	repo, err := InitRepo(ctx, tmpDir, InitOptions{})
	if err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}

	if !repo.IsInitialized(ctx) {
		t.Error("Repository should be initialized")
	}

	// Calling InitRepo again on existing repo should not error
	repo2, err := InitRepo(ctx, tmpDir, InitOptions{})
	if err != nil {
		t.Fatalf("InitRepo on existing repo failed: %v", err)
	}

	if repo.Path() != repo2.Path() {
		t.Error("InitRepo should return same path for existing repo")
	}
}

// TestGitError tests the GitError type.
func TestGitError(t *testing.T) {
	innerErr := errors.New("fatal: not a git repository")
	err := &GitError{
		Command:  "status",
		Args:     []string{"--porcelain"},
		Err:      innerErr,
		ExitCode: 128,
	}

	want := "git status: fatal: not a git repository"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// TestIsNotExist tests the IsNotExist helper.
func TestIsNotExist(t *testing.T) {
	t.Run("actual not exist error", func(t *testing.T) {
		err := &GitError{
			Command: "status",
			Err:     &os.PathError{Err: os.ErrNotExist},
		}
		if !IsNotExist(err) {
			t.Error("IsNotExist should return true for PathError")
		}
	})

	t.Run("other error", func(t *testing.T) {
		err := &GitError{
			Command: "status",
			Err:     &os.PathError{Err: os.ErrPermission},
		}
		if IsNotExist(err) {
			t.Error("IsNotExist should return false for permission error")
		}
	})

	t.Run("nil error", func(t *testing.T) {
		if IsNotExist(nil) {
			t.Error("IsNotExist should return false for nil")
		}
	})
}
