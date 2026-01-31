package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/gitrepo"
	"github.com/chazuruo/svf/internal/workflows"
)

// setupTestRepo creates a temporary Git repository for testing.
func setupTestRepo(t *testing.T) (string, gitrepo.Repo, *config.Config) {
	t.Helper()

	// Create temp directory
	tmpDir := t.TempDir()

	// Initialize git repo
	repo := gitrepo.New(tmpDir)
	ctx := context.Background()
	if err := repo.Init(ctx, gitrepo.InitOptions{}); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	// Create test config
	cfg := &config.Config{
		Workflows: config.WorkflowsConfig{
			Root:       "workflows",
			SharedRoot: "shared",
			DraftRoot:  "drafts",
			IndexPath:  "index.db",
		},
		Identity: config.IdentityConfig{
			Path: "platform/test",
		},
	}

	return tmpDir, repo, cfg
}

func makeTestWorkflow(title string, steps ...workflows.Step) *workflows.Workflow {
	return &workflows.Workflow{
		SchemaVersion: 1,
		Title:         title,
		Steps:         steps,
	}
}

func makeTestStep(command string) workflows.Step {
	return workflows.Step{
		Name:    "test step",
		Command: command,
	}
}

func TestFileSystemStore_New(t *testing.T) {
	_, repo, cfg := setupTestRepo(t)

	tests := []struct {
		name    string
		repo    gitrepo.Repo
		cfg     *config.Config
		wantErr bool
	}{
		{
			name:    "valid parameters",
			repo:    repo,
			cfg:     cfg,
			wantErr: false,
		},
		{
			name:    "nil repo",
			repo:    nil,
			cfg:     cfg,
			wantErr: true,
		},
		{
			name:    "nil config",
			repo:    repo,
			cfg:     nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := New(tt.repo, tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && store == nil {
				t.Error("New() returned nil store")
			}
		})
	}
}

func TestFileSystemStore_Save(t *testing.T) {
	_, repo, cfg := setupTestRepo(t)
	store, err := New(repo, cfg)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()

	t.Run("save basic workflow", func(t *testing.T) {
		wf := makeTestWorkflow("Test Workflow", makeTestStep("echo hello"))

		ref, err := store.Save(ctx, wf, SaveOptions{})
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Verify file exists
		if _, err := os.Stat(ref.Path); os.IsNotExist(err) {
			t.Errorf("workflow file not created at %s", ref.Path)
		}

		// Verify slug
		expectedSlug := "test-workflow"
		if ref.Slug != expectedSlug {
			t.Errorf("Slug = %s, want %s", ref.Slug, expectedSlug)
		}

		// Verify path contains identity
		if !strings.Contains(ref.Path, "platform/test") {
			t.Errorf("Path %s does not contain identity path", ref.Path)
		}
	})

	t.Run("save with auto-commit", func(t *testing.T) {
		wf := makeTestWorkflow("Commit Test", makeTestStep("true"))

		// Create an initial commit first so HEAD exists
		initialFile := filepath.Join(repo.Path(), "initial.txt")
		_ = os.WriteFile(initialFile, []byte("initial"), 0644)
		_ = repo.Add(ctx, "initial.txt")
		_, _ = repo.CommitAll(ctx, "initial commit")

		_, err := store.Save(ctx, wf, SaveOptions{
			Commit:  true,
			Message: "test commit",
		})
		if err != nil {
			t.Fatalf("Save() with commit error = %v", err)
		}

		// Verify git status shows clean (committed)
		status, err := repo.Status(ctx)
		if err != nil {
			t.Fatalf("failed to get status: %v", err)
		}
		if status.Dirty {
			t.Error("expected clean status after commit")
		}
	})

	t.Run("save generates README", func(t *testing.T) {
		wf := &workflows.Workflow{
			SchemaVersion: 1,
			Title:         "README Test",
			Description:   "This is a test workflow",
			Steps:         []workflows.Step{{Name: "Step 1", Command: "echo step1"}},
		}

		ref, err := store.Save(ctx, wf, SaveOptions{})
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Check README exists
		readmePath := filepath.Join(filepath.Dir(ref.Path), "README.md")
		if _, err := os.Stat(readmePath); os.IsNotExist(err) {
			t.Errorf("README not created at %s", readmePath)
		}
	})
}

func TestFileSystemStore_Load(t *testing.T) {
	_, repo, cfg := setupTestRepo(t)
	store, err := New(repo, cfg)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()

	t.Run("load saved workflow", func(t *testing.T) {
		// Save a workflow
		originalWf := &workflows.Workflow{
			SchemaVersion: 1,
			Title:         "Load Test",
			Description:   "Test description",
			Steps:         []workflows.Step{{Name: "Test Step", Command: "echo test"}},
		}

		ref, err := store.Save(ctx, originalWf, SaveOptions{})
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Load it back
		loadedWf, err := store.Load(ctx, ref)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		// Verify content
		if loadedWf.Title != originalWf.Title {
			t.Errorf("Title = %s, want %s", loadedWf.Title, originalWf.Title)
		}
		if loadedWf.Description != originalWf.Description {
			t.Errorf("Description = %s, want %s", loadedWf.Description, originalWf.Description)
		}
		if len(loadedWf.Steps) != len(originalWf.Steps) {
			t.Errorf("Steps count = %d, want %d", len(loadedWf.Steps), len(originalWf.Steps))
		}
	})
}

func TestFileSystemStore_List(t *testing.T) {
	_, repo, cfg := setupTestRepo(t)
	store, err := New(repo, cfg)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()

	// Save some workflows
	testWorkflows := []*workflows.Workflow{
		makeTestWorkflow("Workflow 1", makeTestStep("true")),
		makeTestWorkflow("Workflow 2", makeTestStep("false")),
		makeTestWorkflow("Workflow 3", makeTestStep("echo test")),
	}

	for _, wf := range testWorkflows {
		if _, err := store.Save(ctx, wf, SaveOptions{}); err != nil {
			t.Fatalf("failed to save workflow: %v", err)
		}
	}

	t.Run("list all workflows", func(t *testing.T) {
		refs, err := store.List(ctx, Filter{})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		if len(refs) != len(testWorkflows) {
			t.Errorf("List() returned %d refs, want %d", len(refs), len(testWorkflows))
		}
	})

	t.Run("list with empty filter", func(t *testing.T) {
		refs, err := store.List(ctx, Filter{})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		// All workflows should be returned
		if len(refs) == 0 {
			t.Error("List() returned no refs")
		}
	})
}

func TestFileSystemStore_Delete(t *testing.T) {
	_, repo, cfg := setupTestRepo(t)
	store, err := New(repo, cfg)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()

	t.Run("delete workflow", func(t *testing.T) {
		wf := makeTestWorkflow("Delete Me", makeTestStep("true"))

		ref, err := store.Save(ctx, wf, SaveOptions{})
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Verify file exists
		if _, err := os.Stat(ref.Path); os.IsNotExist(err) {
			t.Fatal("workflow file not created")
		}

		// Delete it
		if err := store.Delete(ctx, ref); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		// Verify file is gone
		if _, err := os.Stat(ref.Path); !os.IsNotExist(err) {
			t.Error("workflow file still exists after delete")
		}

		// Directory should also be gone
		if _, err := os.Stat(filepath.Dir(ref.Path)); !os.IsNotExist(err) {
			t.Error("workflow directory still exists after delete")
		}
	})
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{
			name:  "basic",
			title: "Hello World",
			want:  "hello-world",
		},
		{
			name:  "with special chars",
			title: "Fix: Bug #123!",
			want:  "fix-bug-123",
		},
		{
			name:  "multiple spaces",
			title: "Too   Many    Spaces",
			want:  "too-many-spaces",
		},
		{
			name:  "leading/trailing spaces",
			title: "  padded title  ",
			want:  "padded-title",
		},
		{
			name:  "empty string",
			title: "",
			want:  "",
		},
		{
			name:  "truncation",
			title: "This is a very long title that should be truncated because it exceeds fifty characters",
			want:  "this-is-a-very-long-title-that-should-be",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Slugify(tt.title); got != tt.want {
				t.Errorf("Slugify() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateUniqueSlug(t *testing.T) {
	t.Run("no collisions", func(t *testing.T) {
		existing := []string{"workflow-1", "workflow-2"}
		got := GenerateUniqueSlug("My Workflow", existing)
		want := "my-workflow"
		if got != want {
			t.Errorf("GenerateUniqueSlug() = %q, want %q", got, want)
		}
	})

	t.Run("with collision", func(t *testing.T) {
		existing := []string{"my-workflow", "workflow-1"}
		got := GenerateUniqueSlug("My Workflow", existing)
		want := "my-workflow-1"
		if got != want {
			t.Errorf("GenerateUniqueSlug() = %q, want %q", got, want)
		}
	})

	t.Run("multiple collisions", func(t *testing.T) {
		existing := []string{"my-workflow", "my-workflow-1", "my-workflow-2"}
		got := GenerateUniqueSlug("My Workflow", existing)
		want := "my-workflow-3"
		if got != want {
			t.Errorf("GenerateUniqueSlug() = %q, want %q", got, want)
		}
	})
}
