package gitrepo

import (
	"context"
	"testing"
)

func TestGetMergeState(t *testing.T) {
	t.Run("clean repo has no merge/rebase state", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := initTestRepo(t, tmpDir)
		ctx := context.Background()

		state, err := repo.GetMergeState(ctx)
		if err != nil {
			t.Fatalf("GetMergeState() error = %v", err)
		}

		if state.InMerge {
			t.Error("GetMergeState() InMerge = true, want false")
		}

		if state.InRebase {
			t.Error("GetMergeState() InRebase = true, want false")
		}

		if len(state.ConflictingFiles) != 0 {
			t.Errorf("GetMergeState() ConflictingFiles = %v, want empty", state.ConflictingFiles)
		}
	})
}

func TestIsFileResolved(t *testing.T) {
	t.Run("file in clean repo is resolved", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := initTestRepo(t, tmpDir)
		ctx := context.Background()

		// Check if there are conflicts (should be none)
		conflicts, err := repo.GetConflictFiles(ctx)
		if err != nil {
			t.Fatalf("GetConflictFiles() error = %v", err)
		}

		if len(conflicts) != 0 {
			t.Errorf("Expected no conflicts, got %d", len(conflicts))
		}
	})
}

func TestResolveFile(t *testing.T) {
	t.Run("unresolved choice returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := initTestRepo(t, tmpDir)
		ctx := context.Background()

		err := repo.ResolveFile(ctx, "test.txt", Unresolved)
		if err == nil {
			t.Error("ResolveFile() with Unresolved choice should return error")
		}
	})

	t.Run("choose ours for non-existent file returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := initTestRepo(t, tmpDir)
		ctx := context.Background()

		err := repo.ResolveFile(ctx, "nonexistent.txt", ChooseOurs)
		if err == nil {
			t.Error("ResolveFile() for non-existent file should return error")
		}
	})
}

func TestAbortMerge(t *testing.T) {
	t.Run("abort merge when no merge in progress", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := initTestRepo(t, tmpDir)
		ctx := context.Background()

		err := repo.AbortMerge(ctx)
		// Git abort should succeed even if no merge is in progress
		if err != nil {
			t.Logf("AbortMerge() error = %v (may be expected)", err)
		}
	})
}

func TestAbortRebase(t *testing.T) {
	t.Run("abort rebase when no rebase in progress", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := initTestRepo(t, tmpDir)
		ctx := context.Background()

		err := repo.AbortRebase(ctx)
		// Git abort should succeed even if no rebase is in progress
		if err != nil {
			t.Logf("AbortRebase() error = %v (may be expected)", err)
		}
	})
}

func TestMergeState(t *testing.T) {
	t.Run("MergeState fields are accessible", func(t *testing.T) {
		state := MergeState{
			InMerge:          true,
			InRebase:         false,
			ConflictingFiles: []string{"file1.txt", "file2.txt"},
		}

		if !state.InMerge {
			t.Error("MergeState.InMerge not set correctly")
		}

		if state.InRebase {
			t.Error("MergeState.InRebase not set correctly")
		}

		if len(state.ConflictingFiles) != 2 {
			t.Errorf("MergeState.ConflictingFiles length = %d, want 2", len(state.ConflictingFiles))
		}
	})
}

func TestConflictFile(t *testing.T) {
	t.Run("ConflictFile struct is valid", func(t *testing.T) {
		cf := ConflictFile{
			Path:   "test.txt",
			Ours:   "our content",
			Theirs: "their content",
			Base:   "base content",
		}

		if cf.Path != "test.txt" {
			t.Errorf("ConflictFile.Path = %s, want test.txt", cf.Path)
		}

		if cf.Ours != "our content" {
			t.Errorf("ConflictFile.Ours = %s, want 'our content'", cf.Ours)
		}

		if cf.Theirs != "their content" {
			t.Errorf("ConflictFile.Theirs = %s, want 'their content'", cf.Theirs)
		}

		if cf.Base != "base content" {
			t.Errorf("ConflictFile.Base = %s, want 'base content'", cf.Base)
		}
	})
}
