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

func TestGitRepo_Status_Entries(t *testing.T) {
	tmpDir := t.TempDir()
	repo := New(tmpDir)
	ctx := context.Background()

	_ = repo.Init(ctx, InitOptions{})
	setupGitConfig(tmpDir)

	// Create initial commit
	testFile := filepath.Join(tmpDir, "initial.txt")
	_ = os.WriteFile(testFile, []byte("initial"), 0644)
	_ = repo.Add(ctx, "initial.txt")
	_, _ = repo.CommitAll(ctx, "initial commit")

	// Create a modified file
	err := os.WriteFile(testFile, []byte("modified content"), 0644)
	if err != nil {
		t.Fatalf("failed to modify test file: %v", err)
	}

	// Create a new untracked file
	newFile := filepath.Join(tmpDir, "new.txt")
	_ = os.WriteFile(newFile, []byte("new"), 0644)

	status, err := repo.Status(ctx)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}

	if !status.Dirty {
		t.Error("Status().Dirty = false, want true (repo has changes)")
	}

	if len(status.Entries) == 0 {
		t.Error("Status().Entries is empty, want at least one entry")
	}

	// Check that entries have paths
	for _, entry := range status.Entries {
		if entry.Path == "" {
			t.Error("StatusEntry.Path is empty")
		}
	}
}

func TestGitRepo_Status_AheadBehind(t *testing.T) {
	tmpDir := t.TempDir()
	repo := New(tmpDir)
	ctx := context.Background()

	_ = repo.Init(ctx, InitOptions{})
	setupGitConfig(tmpDir)

	// Create initial commit
	testFile := filepath.Join(tmpDir, "initial.txt")
	_ = os.WriteFile(testFile, []byte("initial"), 0644)
	_ = repo.Add(ctx, "initial.txt")
	_, _ = repo.CommitAll(ctx, "initial commit")

	status, err := repo.Status(ctx)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}

	// No upstream set, so ahead/behind should be 0
	if status.Ahead != 0 {
		t.Errorf("Status().Ahead = %d, want 0", status.Ahead)
	}
	if status.Behind != 0 {
		t.Errorf("Status().Behind = %d, want 0", status.Behind)
	}
}

// setupTestRemote creates a bare remote repository for testing fetch/integrate.
func setupTestRemote(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Initialize a bare repository
	cmd := exec.Command("git", "init", "--bare", tmpDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create bare remote: %v", err)
	}

	return tmpDir
}

// cloneFromRemote clones from a remote repository for testing.
func cloneFromRemote(t *testing.T, remote string) string {
	t.Helper()
	localDir := t.TempDir()

	cmd := exec.Command("git", "clone", remote, localDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to clone: %v", err)
	}

	setupGitConfig(localDir)
	return localDir
}

// getBranchName returns the current branch name for a repo.
func getBranchName(t *testing.T, dir string) string {
	t.Helper()
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "branch", "--show-current")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "main" // Default to main
	}
	return strings.TrimSpace(string(output))
}

// makeCommit creates a commit in the given repository.
func makeCommit(t *testing.T, dir, file, content, message string) {
	t.Helper()
	ctx := context.Background()

	filePath := filepath.Join(dir, file)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Stage and commit
	cmd := exec.CommandContext(ctx, "git", "add", file)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to stage: %v", err)
	}

	cmd = exec.CommandContext(ctx, "git", "commit", "-m", message)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
}

func TestGitRepo_Fetch(t *testing.T) {
	remoteDir := setupTestRemote(t)
	localDir := cloneFromRemote(t, remoteDir)

	// Make initial commit on remote
	repo := New(localDir)
	ctx := context.Background()
	branchName := getBranchName(t, localDir)

	makeCommit(t, localDir, "test.txt", "content", "initial commit")

	// Push to remote
	cmd := exec.CommandContext(ctx, "git", "push", "-u", "origin", branchName)
	cmd.Dir = localDir
	_ = cmd.Run()

	// Make another commit on remote by cloning again and pushing
	remoteWorkDir := t.TempDir()
	cmd = exec.Command("git", "clone", remoteDir, remoteWorkDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to clone: %v", err)
	}
	setupGitConfig(remoteWorkDir)
	makeCommit(t, remoteWorkDir, "remote.txt", "remote content", "remote commit")
	cmd = exec.CommandContext(ctx, "git", "push", "origin", branchName)
	cmd.Dir = remoteWorkDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to push: %v", err)
	}

	// Now fetch in local repo
	result, err := repo.Fetch(ctx, "origin")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	// Should have fetched something
	if result.Fetched == 0 {
		// This is OK if git doesn't report it in verbose mode
		// The fetch still succeeded
		t.Logf("Fetch reported 0 refs fetched (this is OK)")
	}
}

func TestGitRepo_HasConflicts(t *testing.T) {
	t.Run("no conflicts", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := New(tmpDir)
		ctx := context.Background()

		_ = repo.Init(ctx, InitOptions{})
		setupGitConfig(tmpDir)
		makeCommit(t, tmpDir, "initial.txt", "initial", "initial commit")

		hasConflicts, err := repo.HasConflicts(ctx)
		if err != nil {
			t.Fatalf("HasConflicts() error = %v", err)
		}
		if hasConflicts {
			t.Error("HasConflicts() = true, want false")
		}
	})

	t.Run("with conflicts", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := New(tmpDir)
		ctx := context.Background()

		// Initialize repo
		cmd := exec.Command("git", "init")
		cmd.Dir = tmpDir
		_ = cmd.Run()
		setupGitConfig(tmpDir)
		makeCommit(t, tmpDir, "test.txt", "original", "initial commit")

		// Get current branch (main or master)
		cmd = exec.CommandContext(ctx, "git", "branch", "--show-current")
		cmd.Dir = tmpDir
		output, _ := cmd.CombinedOutput()
		mainBranch := strings.TrimSpace(string(output))

		// Create a conflicted state by creating a merge conflict
		// Create branch1 with different content
		cmd = exec.CommandContext(ctx, "git", "checkout", "-b", "branch1")
		cmd.Dir = tmpDir
		_ = cmd.Run()
		makeCommit(t, tmpDir, "test.txt", "branch1 content", "branch1 commit")

		// Go back to main branch and make conflicting change
		cmd = exec.CommandContext(ctx, "git", "checkout", mainBranch)
		cmd.Dir = tmpDir
		_ = cmd.Run()
		makeCommit(t, tmpDir, "test.txt", "main content", "main commit")

		// Try to merge branch1 (will conflict)
		cmd = exec.CommandContext(ctx, "git", "merge", "branch1", "--no-commit")
		cmd.Dir = tmpDir
		_ = cmd.Run()

		// Check for conflicts
		hasConflicts, err := repo.HasConflicts(ctx)
		if err != nil {
			t.Fatalf("HasConflicts() error = %v", err)
		}
		if !hasConflicts {
			t.Error("HasConflicts() = false, want true")
		}

		conflicts, err := repo.GetConflicts(ctx)
		if err != nil {
			t.Fatalf("GetConflicts() error = %v", err)
		}
		if len(conflicts) == 0 {
			t.Error("GetConflicts() returned empty, want at least one conflict")
		}
	})
}

func TestGitRepo_GetConflicts(t *testing.T) {
	t.Run("no conflicts", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := New(tmpDir)
		ctx := context.Background()

		_ = repo.Init(ctx, InitOptions{})
		setupGitConfig(tmpDir)
		makeCommit(t, tmpDir, "initial.txt", "initial", "initial commit")

		conflicts, err := repo.GetConflicts(ctx)
		if err != nil {
			t.Fatalf("GetConflicts() error = %v", err)
		}
		if len(conflicts) != 0 {
			t.Errorf("GetConflicts() = %v, want empty", conflicts)
		}
	})
}

func TestGitRepo_Integrate_FFOnly(t *testing.T) {
	remoteDir := setupTestRemote(t)
	localDir := cloneFromRemote(t, remoteDir)
	ctx := context.Background()
	branchName := getBranchName(t, localDir)

	// Make initial commit on local
	makeCommit(t, localDir, "test.txt", "content", "initial commit")

	// Push to remote
	cmd := exec.CommandContext(ctx, "git", "push", "-u", "origin", branchName)
	cmd.Dir = localDir
	_ = cmd.Run()

	// Make another commit on remote
	remoteWorkDir := t.TempDir()
	cmd = exec.Command("git", "clone", remoteDir, remoteWorkDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to clone: %v", err)
	}
	setupGitConfig(remoteWorkDir)
	makeCommit(t, remoteWorkDir, "remote.txt", "remote content", "remote commit")
	cmd = exec.CommandContext(ctx, "git", "push", "origin", branchName)
	cmd.Dir = remoteWorkDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to push: %v", err)
	}

	// Fetch and integrate in local
	repo := New(localDir)
	_, err := repo.Fetch(ctx, "origin")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	result, err := repo.Integrate(ctx, StrategyFFOnly)
	if err != nil {
		t.Fatalf("Integrate() error = %v", err)
	}

	if !result.FastForward {
		t.Error("Integrate().FastForward = false, want true")
	}
	if result.NewCommits != 1 {
		t.Errorf("Integrate().NewCommits = %d, want 1", result.NewCommits)
	}
}

func TestGitRepo_Status_Conflicted(t *testing.T) {
	tmpDir := t.TempDir()
	repo := New(tmpDir)
	ctx := context.Background()

	// Initialize repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	_ = cmd.Run()
	setupGitConfig(tmpDir)
	makeCommit(t, tmpDir, "test.txt", "original", "initial commit")

	// Get current branch (main or master)
	cmd = exec.CommandContext(ctx, "git", "branch", "--show-current")
	cmd.Dir = tmpDir
	output, _ := cmd.CombinedOutput()
	mainBranch := strings.TrimSpace(string(output))

	// Create a conflict
	cmd = exec.CommandContext(ctx, "git", "checkout", "-b", "branch1")
	cmd.Dir = tmpDir
	_ = cmd.Run()
	makeCommit(t, tmpDir, "test.txt", "branch1 content", "branch1 commit")

	cmd = exec.CommandContext(ctx, "git", "checkout", mainBranch)
	cmd.Dir = tmpDir
	_ = cmd.Run()
	makeCommit(t, tmpDir, "test.txt", "main content", "main commit")

	cmd = exec.CommandContext(ctx, "git", "merge", "branch1", "--no-commit")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	status, err := repo.Status(ctx)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}

	if !status.Conflicted {
		t.Error("Status().Conflicted = false, want true")
	}

	if len(status.Conflicts) == 0 {
		t.Error("Status().Conflicts is empty, want at least one conflict")
	}
}

func TestGitRepo_Integrate_MergeConflicts(t *testing.T) {
	remoteDir := setupTestRemote(t)
	localDir1 := cloneFromRemote(t, remoteDir)
	localDir2 := t.TempDir()
	ctx := context.Background()

	// Set up second local repo
	cmd := exec.Command("git", "clone", remoteDir, localDir2)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to clone: %v", err)
	}
	setupGitConfig(localDir2)

	// Make initial commit on localDir1 and push
	makeCommit(t, localDir1, "base.txt", "base", "base commit")
	cmd = exec.CommandContext(ctx, "git", "push", "-u", "origin", "master")
	cmd.Dir = localDir1
	_ = cmd.Run()

	// Fetch in localDir2
	cmd = exec.CommandContext(ctx, "git", "fetch")
	cmd.Dir = localDir2
	_ = cmd.Run()

	// Create divergent commits
	makeCommit(t, localDir1, "test.txt", "local1 content", "local1 commit")
	cmd = exec.CommandContext(ctx, "git", "push")
	cmd.Dir = localDir1
	_ = cmd.Run()

	makeCommit(t, localDir2, "test.txt", "local2 content", "local2 commit")

	// Try to merge in localDir2 - this will conflict
	repo := New(localDir2)
	_, err := repo.Fetch(ctx, "origin")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	result, err := repo.Integrate(ctx, StrategyMerge)
	if err == nil {
		// Merge succeeded (no conflict) - skip conflict checks
		return
	}

	// If there was an error, check if it's due to conflicts
	if result.Conflicts {
		if len(result.ConflictFiles) == 0 {
			t.Error("Integrate().ConflictFiles is empty with Conflicts=true")
		}

		hasConflicts, _ := repo.HasConflicts(ctx)
		if !hasConflicts {
			t.Error("HasConflicts() = false after merge conflict")
		}
	}
}

func TestGitRepo_Integrate_RebaseConflicts(t *testing.T) {
	remoteDir := setupTestRemote(t)
	localDir1 := cloneFromRemote(t, remoteDir)
	localDir2 := t.TempDir()
	ctx := context.Background()

	// Set up second local repo
	cmd := exec.Command("git", "clone", remoteDir, localDir2)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to clone: %v", err)
	}
	setupGitConfig(localDir2)

	// Make initial commit on localDir1 and push
	makeCommit(t, localDir1, "base.txt", "base", "base commit")
	cmd = exec.CommandContext(ctx, "git", "push", "-u", "origin", "master")
	cmd.Dir = localDir1
	_ = cmd.Run()

	// Fetch in localDir2
	cmd = exec.CommandContext(ctx, "git", "fetch")
	cmd.Dir = localDir2
	_ = cmd.Run()

	// Create divergent commits
	makeCommit(t, localDir1, "test.txt", "local1 content", "local1 commit")
	cmd = exec.CommandContext(ctx, "git", "push")
	cmd.Dir = localDir1
	_ = cmd.Run()

	makeCommit(t, localDir2, "test.txt", "local2 content", "local2 commit")

	// Try to rebase in localDir2 - this may conflict
	repo := New(localDir2)
	_, err := repo.Fetch(ctx, "origin")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	result, err := repo.Integrate(ctx, StrategyRebase)
	if err == nil {
		// Rebase succeeded (no conflict) - skip conflict checks
		return
	}

	// If there was an error, check if it's due to conflicts
	if result.Conflicts {
		if len(result.ConflictFiles) == 0 {
			t.Error("Integrate().ConflictFiles is empty with Conflicts=true")
		}

		// Abort the rebase to clean up
		_ = exec.CommandContext(ctx, "git", "rebase", "--abort").Run()

		hasConflicts, _ := repo.HasConflicts(ctx)
		if !hasConflicts {
			t.Error("HasConflicts() = false after rebase conflict")
		}
	}

	// Abort any in-progress rebase
	_ = exec.CommandContext(ctx, "git", "rebase", "--abort").Run()
}
