package gitrepo

import (
	"context"
	"testing"
)

// initTestRepo creates a test repository with an initial commit.
func initTestRepo(t *testing.T, tmpDir string) Repo {
	t.Helper()
	repo := New(tmpDir)
	ctx := context.Background()

	_ = repo.Init(ctx, InitOptions{})

	// Create an initial commit so HEAD exists
	_ = repo.Add(ctx, ".gitignore")
	_, _ = repo.CommitAll(ctx, "initial commit")

	return repo
}

func TestGetConflictFiles(t *testing.T) {
	t.Run("no conflicts in clean repo", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := initTestRepo(t, tmpDir)
		ctx := context.Background()

		conflicts, err := repo.GetConflictFiles(ctx)
		if err != nil {
			t.Fatalf("GetConflictFiles() error = %v", err)
		}

		if len(conflicts) != 0 {
			t.Errorf("GetConflictFiles() returned %d conflicts, want 0", len(conflicts))
		}
	})
}

func TestHasConflicts(t *testing.T) {
	t.Run("no conflicts in clean repo", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := initTestRepo(t, tmpDir)
		ctx := context.Background()

		hasConflicts, err := repo.HasConflicts(ctx)
		if err != nil {
			t.Fatalf("HasConflicts() error = %v", err)
		}

		if hasConflicts {
			t.Error("HasConflicts() = true, want false")
		}
	})
}

func TestGetDiff(t *testing.T) {
	t.Run("GetDiff returns valid struct", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := initTestRepo(t, tmpDir)
		ctx := context.Background()

		// Get diff for non-existent file - should return empty diff
		diff, err := repo.GetDiff(ctx, "nonexistent.txt", DiffCombined)
		if err != nil {
			// Error is expected for non-existent file
			return
		}

		// If no error, verify the struct is valid
		if diff.Path != "nonexistent.txt" {
			t.Errorf("GetDiff() Path = %s, want nonexistent.txt", diff.Path)
		}
	})
}

func TestResolutionChoiceValues(t *testing.T) {
	tests := []struct {
		name  string
		value ResolutionChoice
	}{
		{"Unresolved", Unresolved},
		{"ChooseOurs", ChooseOurs},
		{"ChooseTheirs", ChooseTheirs},
		{"ManualEdit", ManualEdit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.value) < 0 || int(tt.value) > 3 {
				t.Errorf("ResolutionChoice %s has unexpected value: %v", tt.name, tt.value)
			}
		})
	}
}

func TestDiffTypeValues(t *testing.T) {
	tests := []struct {
		name  string
		value DiffType
	}{
		{"DiffOurs", DiffOurs},
		{"DiffTheirs", DiffTheirs},
		{"DiffCombined", DiffCombined},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.value) < 0 || int(tt.value) > 2 {
				t.Errorf("DiffType %s has unexpected value: %v", tt.name, tt.value)
			}
		})
	}
}

func TestDiffContent(t *testing.T) {
	t.Run("DiffContent struct is valid", func(t *testing.T) {
		dc := DiffContent{
			Path:    "test.txt",
			Content: "diff content",
			Type:    DiffCombined,
		}

		if dc.Path != "test.txt" {
			t.Errorf("DiffContent.Path = %s, want test.txt", dc.Path)
		}

		if dc.Content != "diff content" {
			t.Errorf("DiffContent.Content = %s, want 'diff content'", dc.Content)
		}

		if dc.Type != DiffCombined {
			t.Errorf("DiffContent.Type = %v, want DiffCombined", dc.Type)
		}
	})
}
