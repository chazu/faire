package index

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/workflows"
)

// setupTestIndex creates a temporary directory with test workflows for testing.
func setupTestIndex(t *testing.T) (string, *config.Config, *Builder) {
	t.Helper()

	tmpDir := t.TempDir()
	cfg := &config.Config{
		Workflows: config.WorkflowsConfig{
			Root:       "workflows",
			SharedRoot: "shared",
			IndexPath:  ".svf/index.json",
		},
		Identity: config.IdentityConfig{
			Path: "platform/test",
		},
	}

	builder := NewBuilder(tmpDir, cfg)

	// Create test workflow directory structure
	// workflows/platform/test/workflow1/
	wf1Dir := filepath.Join(tmpDir, "workflows", "platform", "test", "workflow1")
	if err := os.MkdirAll(wf1Dir, 0755); err != nil {
		t.Fatalf("failed to create workflow directory: %v", err)
	}

	wf1 := &workflows.Workflow{
		SchemaVersion: 1,
		ID:            "wf_01ABC123DEF45678",
		Title:         "Test Workflow 1",
		Description:   "First test workflow",
		Tags:          []string{"test", "example"},
		Steps: []workflows.Step{
			{Name: "Step 1", Command: "echo hello"},
		},
	}
	wf1Data, err := workflows.MarshalWorkflow(wf1)
	if err != nil {
		t.Fatalf("failed to marshal workflow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wf1Dir, "workflow.yaml"), wf1Data, 0644); err != nil {
		t.Fatalf("failed to write workflow: %v", err)
	}

	// workflows/platform/test/deploy/
	wf2Dir := filepath.Join(tmpDir, "workflows", "platform", "test", "deploy")
	if err := os.MkdirAll(wf2Dir, 0755); err != nil {
		t.Fatalf("failed to create workflow directory: %v", err)
	}

	wf2 := &workflows.Workflow{
		SchemaVersion: 1,
		Title:         "Deployment Workflow",
		Description:   "Deploy application to production",
		Tags:          []string{"deployment", "production"},
		Steps: []workflows.Step{
			{Name: "Build", Command: "docker build -t app ."},
			{Name: "Push", Command: "docker push app"},
		},
	}
	wf2Data, err := workflows.MarshalWorkflow(wf2)
	if err != nil {
		t.Fatalf("failed to marshal workflow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wf2Dir, "workflow.yaml"), wf2Data, 0644); err != nil {
		t.Fatalf("failed to write workflow: %v", err)
	}

	// shared/common/utility/
	sharedDir := filepath.Join(tmpDir, "shared", "common", "utility")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatalf("failed to create shared workflow directory: %v", err)
	}

	sharedWf := &workflows.Workflow{
		SchemaVersion: 1,
		Title:         "Shared Utility",
		Description:   "A shared utility workflow",
		Tags:          []string{"utility", "shared"},
		Steps: []workflows.Step{
			{Name: "Clean", Command: "rm -rf /tmp/cache"},
		},
	}
	sharedData, err := workflows.MarshalWorkflow(sharedWf)
	if err != nil {
		t.Fatalf("failed to marshal workflow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sharedDir, "workflow.yaml"), sharedData, 0644); err != nil {
		t.Fatalf("failed to write workflow: %v", err)
	}

	return tmpDir, cfg, builder
}

func TestNewBuilder(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		cfg := &config.Config{
			Workflows: config.WorkflowsConfig{
				Root:       "workflows",
				SharedRoot: "shared",
				IndexPath:  ".svf/index.json",
			},
		}
		builder := NewBuilder("/tmp/test", cfg)

		if builder.repoPath != "/tmp/test" {
			t.Errorf("repoPath = %s, want /tmp/test", builder.repoPath)
		}
		if builder.config != cfg {
			t.Error("config not set correctly")
		}
		if builder.GetIndexPath() != "/tmp/test/.svf/index.json" {
			t.Errorf("GetIndexPath() = %s, want /tmp/test/.svf/index.json", builder.GetIndexPath())
		}
	})

	t.Run("with nil config uses defaults", func(t *testing.T) {
		builder := NewBuilder("/tmp/test", nil)

		if builder.config == nil {
			t.Error("config should not be nil")
		}
		if builder.config.Workflows.Root == "" {
			t.Error("config should have default values")
		}
	})
}

func TestBuilder_Build(t *testing.T) {
	_, _, builder := setupTestIndex(t)

	index, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// Check index metadata
	if index.Version != CurrentSchemaVersion {
		t.Errorf("Version = %d, want %d", index.Version, CurrentSchemaVersion)
	}

	// Check that we found 3 workflows
	if len(index.Workflows) != 3 {
		t.Fatalf("Workflows count = %d, want 3", len(index.Workflows))
	}

	// Check workflow entries
	found := make(map[string]bool)
	for _, entry := range index.Workflows {
		found[entry.Title] = true

		// Verify all required fields are populated
		if entry.ID == "" {
			t.Errorf("Workflow %s has empty ID", entry.Title)
		}
		if entry.Title == "" {
			t.Error("Workflow has empty title")
		}
		if entry.Path == "" {
			t.Errorf("Workflow %s has empty path", entry.Title)
		}
		if entry.SearchText == "" {
			t.Errorf("Workflow %s has empty search text", entry.Title)
		}
		if entry.UpdatedAt == "" {
			t.Errorf("Workflow %s has empty updated_at", entry.Title)
		}
	}

	// Verify all expected workflows are present
	expectedTitles := []string{"Test Workflow 1", "Deployment Workflow", "Shared Utility"}
	for _, title := range expectedTitles {
		if !found[title] {
			t.Errorf("Missing workflow: %s", title)
		}
	}
}

func TestBuilder_SaveAndLoad(t *testing.T) {
	_, _, builder := setupTestIndex(t)

	// Build the index
	index, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// Save it
	if err := builder.Save(index); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	indexPath := builder.GetIndexPath()
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Errorf("Index file not created at %s", indexPath)
	}

	// Load it back
	loaded, err := builder.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify content matches
	if loaded.Version != index.Version {
		t.Errorf("Loaded Version = %d, want %d", loaded.Version, index.Version)
	}
	if len(loaded.Workflows) != len(index.Workflows) {
		t.Errorf("Loaded Workflows count = %d, want %d", len(loaded.Workflows), len(index.Workflows))
	}
}

func TestBuilder_IsStale(t *testing.T) {
	_, _, builder := setupTestIndex(t)

	t.Run("no index exists", func(t *testing.T) {
		stale, err := builder.IsStale()
		if err != nil {
			t.Fatalf("IsStale() error = %v", err)
		}
		if !stale {
			t.Error("Expected index to be stale when it doesn't exist")
		}
	})

	t.Run("fresh index", func(t *testing.T) {
		// Build and save index
		index, err := builder.Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		if err := builder.Save(index); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Index should not be stale immediately
		stale, err := builder.IsStale()
		if err != nil {
			t.Fatalf("IsStale() error = %v", err)
		}
		if stale {
			t.Error("Expected index to be fresh immediately after build")
		}
	})

	t.Run("stale after workflow modification", func(t *testing.T) {
		// Build and save index
		index, err := builder.Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		if err := builder.Save(index); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Wait a tiny bit to ensure time difference
		time.Sleep(10 * time.Millisecond)

		// Modify a workflow file
		wfPath := filepath.Join(builder.repoPath, "workflows", "platform", "test", "workflow1", "workflow.yaml")
		wf := &workflows.Workflow{
			SchemaVersion: 1,
			Title:         "Modified Workflow",
			Steps: []workflows.Step{
				{Name: "Step", Command: "echo modified"},
			},
		}
		data, err := workflows.MarshalWorkflow(wf)
		if err != nil {
			t.Fatalf("failed to marshal workflow: %v", err)
		}
		if err := os.WriteFile(wfPath, data, 0644); err != nil {
			t.Fatalf("failed to write workflow: %v", err)
		}

		// Index should now be stale
		stale, err := builder.IsStale()
		if err != nil {
			t.Fatalf("IsStale() error = %v", err)
		}
		if !stale {
			t.Error("Expected index to be stale after workflow modification")
		}
	})
}

func TestIndex_Search(t *testing.T) {
	_, _, builder := setupTestIndex(t)

	index, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	tests := []struct {
		name          string
		query         string
		expectedCount int
		mustContain   []string
	}{
		{
			name:          "empty query returns all",
			query:         "",
			expectedCount: 3,
		},
		{
			name:          "title match",
			query:         "test",
			expectedCount: 1,
			mustContain:   []string{"Test Workflow 1"},
		},
		{
			name:          "description match",
			query:         "production",
			expectedCount: 1,
			mustContain:   []string{"Deployment Workflow"},
		},
		{
			name:          "tag match",
			query:         "deployment",
			expectedCount: 1,
			mustContain:   []string{"Deployment Workflow"},
		},
		{
			name:          "command match",
			query:         "docker",
			expectedCount: 1,
			mustContain:   []string{"Deployment Workflow"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := index.Search(tt.query)

			if len(results) != tt.expectedCount {
				t.Errorf("Search() returned %d results, want %d", len(results), tt.expectedCount)
			}

			for _, title := range tt.mustContain {
				found := false
				for _, r := range results {
					if r.Title == title {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected result %s not found", title)
				}
			}
		})
	}
}

func TestIndex_FilterByTags(t *testing.T) {
	_, _, builder := setupTestIndex(t)

	index, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	tests := []struct {
		name          string
		tags          []string
		expectedCount int
		mustContain   []string
	}{
		{
			name:          "no tags returns all",
			tags:          []string{},
			expectedCount: 3,
		},
		{
			name:          "single tag",
			tags:          []string{"test"},
			expectedCount: 1,
			mustContain:   []string{"Test Workflow 1"},
		},
		{
			name:          "deployment tag",
			tags:          []string{"deployment"},
			expectedCount: 1,
			mustContain:   []string{"Deployment Workflow"},
		},
		{
			name:          "multiple tags (AND)",
			tags:          []string{"deployment", "production"},
			expectedCount: 1,
			mustContain:   []string{"Deployment Workflow"},
		},
		{
			name:          "non-existent tag",
			tags:          []string{"nonexistent"},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := index.FilterByTags(tt.tags)

			if len(results) != tt.expectedCount {
				t.Errorf("FilterByTags() returned %d results, want %d", len(results), tt.expectedCount)
			}

			for _, title := range tt.mustContain {
				found := false
				for _, r := range results {
					if r.Title == title {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected result %s not found", title)
				}
			}
		})
	}
}

func TestIndex_FilterByPath(t *testing.T) {
	_, _, builder := setupTestIndex(t)

	index, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	t.Run("filter by workflows prefix", func(t *testing.T) {
		results := index.FilterByPath("workflows/")

		// Should return 2 workflows from workflows/
		if len(results) != 2 {
			t.Errorf("FilterByPath(workflows/) returned %d results, want 2", len(results))
		}

		// None should be from shared
		for _, r := range results {
			if r.Path == "shared/common/utility/workflow.yaml" {
				t.Error("Shared workflow should not be in workflows/ filter")
			}
		}
	})

	t.Run("filter by shared prefix", func(t *testing.T) {
		results := index.FilterByPath("shared/")

		if len(results) != 1 {
			t.Errorf("FilterByPath(shared/) returned %d results, want 1", len(results))
		}

		if results[0].Title != "Shared Utility" {
			t.Errorf("Expected Shared Utility, got %s", results[0].Title)
		}
	})
}

func TestIndex_FuzzySearch(t *testing.T) {
	_, _, builder := setupTestIndex(t)

	index, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	tests := []struct {
		name          string
		opts          SearchOptions
		expectedCount int
		topResult     string
	}{
		{
			name:          "no filters",
			opts:          SearchOptions{},
			expectedCount: 3,
		},
		{
			name: "query only",
			opts: SearchOptions{
				Query: "test",
			},
			expectedCount: 1,
			topResult:     "Test Workflow 1",
		},
		{
			name: "tag filter",
			opts: SearchOptions{
				Tags: []string{"deployment"},
			},
			expectedCount: 1,
			topResult:     "Deployment Workflow",
		},
		{
			name: "identity path filter",
			opts: SearchOptions{
				IdentityPath: "platform/test",
			},
			expectedCount: 2, // Two workflows in platform/test
		},
		{
			name: "combined filters",
			opts: SearchOptions{
				Query:        "workflow",
				Tags:         []string{"test"},
				IdentityPath: "platform/test",
			},
			expectedCount: 1,
			topResult:     "Test Workflow 1",
		},
		{
			name: "mine filter (non-shared)",
			opts: SearchOptions{
				Mine: true,
			},
			expectedCount: 2, // All workflows from workflows/ dir (not shared/)
		},
		{
			name: "shared filter",
			opts: SearchOptions{
				Shared: true,
			},
			expectedCount: 1, // Only shared workflows
			topResult:     "Shared Utility",
		},
		{
			name: "fuzzy subsequence match",
			opts: SearchOptions{
				Query: "tst", // Should match "Test Workflow" via "Test"
			},
			expectedCount: 1,
			topResult:     "Test Workflow 1",
		},
		{
			name: "fuzzy tag match",
			opts: SearchOptions{
				Query: "dep", // Should match "deployment" tag
			},
			expectedCount: 1,
			topResult:     "Deployment Workflow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := index.FuzzySearch(tt.opts)

			if len(results) != tt.expectedCount {
				t.Errorf("FuzzySearch() returned %d results, want %d", len(results), tt.expectedCount)
			}

			if tt.topResult != "" && len(results) > 0 {
				if results[0].Entry.Title != tt.topResult {
					t.Errorf("Top result = %s, want %s", results[0].Entry.Title, tt.topResult)
				}
			}

			// Results should be sorted by score (descending)
			for i := 1; i < len(results); i++ {
				if results[i].Score > results[i-1].Score {
					t.Error("Results not sorted by score")
				}
			}
		})
	}
}

func TestIndex_GetByPath(t *testing.T) {
	_, _, builder := setupTestIndex(t)

	index, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	t.Run("existing path", func(t *testing.T) {
		entry := index.GetByPath("workflows/platform/test/workflow1/workflow.yaml")
		if entry == nil {
			t.Error("GetByPath() returned nil for existing path")
		} else if entry.Title != "Test Workflow 1" {
			t.Errorf("Got %s, want Test Workflow 1", entry.Title)
		}
	})

	t.Run("non-existent path", func(t *testing.T) {
		entry := index.GetByPath("nonexistent/path")
		if entry != nil {
			t.Error("GetByPath() should return nil for non-existent path")
		}
	})
}

func TestIndex_GetByID(t *testing.T) {
	_, _, builder := setupTestIndex(t)

	index, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	t.Run("existing ID", func(t *testing.T) {
		entry := index.GetByID("wf_01ABC123DEF45678")
		if entry == nil {
			t.Error("GetByID() returned nil for existing ID")
		} else if entry.Title != "Test Workflow 1" {
			t.Errorf("Got %s, want Test Workflow 1", entry.Title)
		}
	})

	t.Run("non-existent ID", func(t *testing.T) {
		entry := index.GetByID("nonexistent")
		if entry != nil {
			t.Error("GetByID() should return nil for non-existent ID")
		}
	})
}
