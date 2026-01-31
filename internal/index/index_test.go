package index

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIndexLoadSave(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "index.json")

	// Create an index
	idx := &Index{
		Version:   CurrentVersion,
		UpdatedAt: time.Now().UTC().Truncate(time.Second),
		Workflows: []Entry{
			{
				ID:           "wf-001",
				Slug:         "test-workflow",
				Title:        "Test Workflow",
				Path:         "/workflows/test/workflow.yaml",
				IdentityPath: "test",
				Tags:         []string{"test", "demo"},
				CreatedAt:    time.Now().UTC().Truncate(time.Second),
				UpdatedAt:    time.Now().UTC().Truncate(time.Second),
				SearchText:   "test workflow demo",
			},
		},
	}

	// Save index
	if err := idx.Save(indexPath); err != nil {
		t.Fatalf("Failed to save index: %v", err)
	}

	// Load index
	loaded, err := Load(indexPath)
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}

	// Verify
	if loaded.Version != idx.Version {
		t.Errorf("Version mismatch: got %d, want %d", loaded.Version, idx.Version)
	}

	if len(loaded.Workflows) != len(idx.Workflows) {
		t.Errorf("Workflow count mismatch: got %d, want %d", len(loaded.Workflows), len(idx.Workflows))
	}

	if len(loaded.Workflows) > 0 {
		entry := loaded.Workflows[0]
		if entry.ID != "wf-001" {
			t.Errorf("ID mismatch: got %s, want wf-001", entry.ID)
		}
		if entry.Title != "Test Workflow" {
			t.Errorf("Title mismatch: got %s, want Test Workflow", entry.Title)
		}
	}
}

func TestIndexLoadNotExist(t *testing.T) {
	// Try to load non-existent index
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "nonexistent.json")

	loaded, err := Load(indexPath)
	if err != nil {
		t.Fatalf("Failed to load non-existent index: %v", err)
	}

	if loaded != nil {
		t.Errorf("Expected nil for non-existent index, got %+v", loaded)
	}
}

func TestIndexAddRemove(t *testing.T) {
	idx := &Index{
		Version:   CurrentVersion,
		UpdatedAt: time.Now(),
		Workflows: []Entry{},
	}

	// Add an entry
	entry1 := Entry{
		ID:   "wf-001",
		Slug: "workflow-1",
		Path: "/workflows/wf1/workflow.yaml",
	}
	idx.Add(entry1)

	if idx.Len() != 1 {
		t.Errorf("Expected 1 entry after add, got %d", idx.Len())
	}

	// Add another entry
	entry2 := Entry{
		ID:   "wf-002",
		Slug: "workflow-2",
		Path: "/workflows/wf2/workflow.yaml",
	}
	idx.Add(entry2)

	if idx.Len() != 2 {
		t.Errorf("Expected 2 entries after add, got %d", idx.Len())
	}

	// Remove by ID
	if !idx.Remove("wf-001", "") {
		t.Error("Expected Remove to return true")
	}

	if idx.Len() != 1 {
		t.Errorf("Expected 1 entry after remove, got %d", idx.Len())
	}

	// Remove by path
	if !idx.Remove("", "/workflows/wf2/workflow.yaml") {
		t.Error("Expected Remove to return true")
	}

	if idx.Len() != 0 {
		t.Errorf("Expected 0 entries after remove, got %d", idx.Len())
	}
}

func TestIndexAddReplacesExisting(t *testing.T) {
	idx := &Index{
		Version:   CurrentVersion,
		UpdatedAt: time.Now(),
		Workflows: []Entry{},
	}

	// Add an entry
	entry1 := Entry{
		ID:    "wf-001",
		Slug:  "workflow-1",
		Title: "Original Title",
		Path:  "/workflows/wf1/workflow.yaml",
	}
	idx.Add(entry1)

	if idx.Len() != 1 {
		t.Errorf("Expected 1 entry, got %d", idx.Len())
	}

	// Add entry with same ID (should replace)
	entry2 := Entry{
		ID:    "wf-001",
		Slug:  "workflow-1-updated",
		Title: "Updated Title",
		Path:  "/workflows/wf1-updated/workflow.yaml",
	}
	idx.Add(entry2)

	if idx.Len() != 1 {
		t.Errorf("Expected 1 entry after replace, got %d", idx.Len())
	}

	if idx.Workflows[0].Title != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got '%s'", idx.Workflows[0].Title)
	}
}

func TestIndexFindByID(t *testing.T) {
	idx := &Index{
		Version:   CurrentVersion,
		UpdatedAt: time.Now(),
		Workflows: []Entry{
			{ID: "wf-001", Slug: "workflow-1", Path: "/workflows/wf1/workflow.yaml"},
			{ID: "wf-002", Slug: "workflow-2", Path: "/workflows/wf2/workflow.yaml"},
		},
	}

	// Find existing
	entry := idx.FindByID("wf-001")
	if entry == nil {
		t.Fatal("Expected to find entry wf-001")
	}
	if entry.Slug != "workflow-1" {
		t.Errorf("Expected slug 'workflow-1', got '%s'", entry.Slug)
	}

	// Find non-existing
	entry = idx.FindByID("wf-999")
	if entry != nil {
		t.Errorf("Expected nil for non-existing entry, got %+v", entry)
	}
}

func TestIndexFindByPath(t *testing.T) {
	idx := &Index{
		Version:   CurrentVersion,
		UpdatedAt: time.Now(),
		Workflows: []Entry{
			{ID: "wf-001", Slug: "workflow-1", Path: "/workflows/wf1/workflow.yaml"},
			{ID: "wf-002", Slug: "workflow-2", Path: "/workflows/wf2/workflow.yaml"},
		},
	}

	// Find existing
	entry := idx.FindByPath("/workflows/wf1/workflow.yaml")
	if entry == nil {
		t.Fatal("Expected to find entry by path")
	}
	if entry.ID != "wf-001" {
		t.Errorf("Expected ID 'wf-001', got '%s'", entry.ID)
	}

	// Find non-existing
	entry = idx.FindByPath("/workflows/nonexistent/workflow.yaml")
	if entry != nil {
		t.Errorf("Expected nil for non-existing path, got %+v", entry)
	}
}

func TestIndexJSONMarshal(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	idx := &Index{
		Version:   CurrentVersion,
		UpdatedAt: now,
		Workflows: []Entry{
			{
				ID:           "wf_01HZY3J9Y3G6Q9T3",
				Slug:         "restart-service",
				Title:        "Restart service safely",
				Path:         "workflows/platform/chaz/restart-service/workflow.yaml",
				IdentityPath: "platform/chaz",
				Tags:         []string{"kubernetes", "ops"},
				CreatedAt:    now,
				UpdatedAt:    now,
				SearchText:   "restart service safely kubernetes ops",
			},
		},
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal index: %v", err)
	}

	// Verify JSON contains expected fields
	jsonStr := string(data)
	expectedFields := []string{
		`"version": 1`,
		`"id": "wf_01HZY3J9Y3G6Q9T3"`,
		`"slug": "restart-service"`,
		`"title": "Restart service safely"`,
		`"identity_path": "platform/chaz"`,
		`"search_text": "restart service safely kubernetes ops"`,
	}

	for _, field := range expectedFields {
		if !contains(jsonStr, field) {
			t.Errorf("Expected JSON to contain '%s', got:\n%s", field, jsonStr)
		}
	}
}

func TestHasAllTags(t *testing.T) {
	tests := []struct {
		name       string
		entryTags  []string
		required   []string
		wantResult bool
	}{
		{
			name:       "all tags match",
			entryTags:  []string{"kubernetes", "ops", "deploy"},
			required:   []string{"kubernetes", "ops"},
			wantResult: true,
		},
		{
			name:       "some tags missing",
			entryTags:  []string{"kubernetes", "ops"},
			required:   []string{"kubernetes", "deploy"},
			wantResult: false,
		},
		{
			name:       "no required tags",
			entryTags:  []string{"kubernetes", "ops"},
			required:   []string{},
			wantResult: true,
		},
		{
			name:       "case insensitive match",
			entryTags:  []string{"Kubernetes", "OPS"},
			required:   []string{"kubernetes", "ops"},
			wantResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasAllTags(tt.entryTags, tt.required)
			if result != tt.wantResult {
				t.Errorf("hasAllTags() = %v, want %v", result, tt.wantResult)
			}
		})
	}
}

func TestMatchesQuery(t *testing.T) {
	now := time.Now()
	entry := Entry{
		ID:          "wf-001",
		Slug:        "restart-service",
		Title:       "Restart Service Safely",
		Path:        "/workflows/ops/restart-service/workflow.yaml",
		Tags:        []string{"kubernetes", "ops"},
		SearchText:  "restart service safely kubernetes ops deployment",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	tests := []struct {
		name        string
		query       string
		wantMatch   bool
	}{
		{
			name:      "matches title",
			query:     "restart",
			wantMatch: true,
		},
		{
			name:      "matches slug",
			query:     "service",
			wantMatch: true,
		},
		{
			name:      "matches search text",
			query:     "deployment",
			wantMatch: true,
		},
		{
			name:      "case insensitive",
			query:     "KUBERNETES",
			wantMatch: true,
		},
		{
			name:      "no match",
			query:     "database",
			wantMatch: false,
		},
		{
			name:      "empty query matches all",
			query:     "",
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesQuery(entry, strings.ToLower(tt.query))
			if result != tt.wantMatch {
				t.Errorf("matchesQuery() = %v, want %v", result, tt.wantMatch)
			}
		})
	}
}

func TestSearch(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	idx := &Index{
		Version:   CurrentVersion,
		UpdatedAt: now,
		Workflows: []Entry{
			{
				ID:           "wf-001",
				Slug:         "restart-service",
				Title:        "Restart Service Safely",
				Path:         "/workflows/ops/restart-service/workflow.yaml",
				IdentityPath: "ops",
				Tags:         []string{"kubernetes", "ops"},
				SearchText:   "restart service safely kubernetes ops",
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			{
				ID:           "wf-002",
				Slug:         "deploy-app",
				Title:        "Deploy Application",
				Path:         "/workflows/platform/deploy-app/workflow.yaml",
				IdentityPath: "platform",
				Tags:         []string{"deploy", "production"},
				SearchText:   "deploy application production",
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			{
				ID:           "wf-003",
				Slug:         "backup-db",
				Title:        "Backup Database",
				Path:         "/workflows/db/backup-db/workflow.yaml",
				IdentityPath: "db",
				Tags:         []string{"database", "backup"},
				SearchText:   "backup database postgresql",
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		},
	}

	tests := []struct {
		name    string
		opts    SearchOptions
		wantLen int
		wantIDs []string
	}{
		{
			name: "search by title",
			opts: SearchOptions{
				Query: "restart",
			},
			wantLen: 1,
			wantIDs: []string{"wf-001"},
		},
		{
			name: "filter by identity path",
			opts: SearchOptions{
				IdentityPath: "platform",
			},
			wantLen: 1,
			wantIDs: []string{"wf-002"},
		},
		{
			name: "filter by tag",
			opts: SearchOptions{
				Tags: []string{"kubernetes"},
			},
			wantLen: 1,
			wantIDs: []string{"wf-001"},
		},
		{
			name: "filter by multiple tags",
			opts: SearchOptions{
				Tags: []string{"kubernetes", "ops"},
			},
			wantLen: 1,
			wantIDs: []string{"wf-001"},
		},
		{
			name: "no filters returns all",
			opts: SearchOptions{},
			wantLen: 3,
			wantIDs: []string{"wf-001", "wf-002", "wf-003"},
		},
		{
			name: "limit results",
			opts: SearchOptions{
				Limit: 2,
			},
			wantLen: 2,
			wantIDs: []string{"wf-001", "wf-002"},
		},
		{
			name: "no matches",
			opts: SearchOptions{
				Query: "nonexistent",
			},
			wantLen: 0,
			wantIDs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := Search(idx, tt.opts)

			if len(results) != tt.wantLen {
				t.Errorf("Search() returned %d results, want %d", len(results), tt.wantLen)
			}

			if len(tt.wantIDs) > 0 {
				gotIDs := make([]string, len(results))
				for i, r := range results {
					gotIDs[i] = r.ID
				}
				for _, wantID := range tt.wantIDs {
					if !containsID(gotIDs, wantID) {
						t.Errorf("Expected results to contain ID %s, got %v", wantID, gotIDs)
					}
				}
			}
		})
	}
}

func TestExtractIdentityPath(t *testing.T) {
	b := &Builder{}

	tests := []struct {
		name     string
		path     string
		wantPath string
	}{
		{
			name:     "unix path with workflows",
			path:     "/home/user/repo/workflows/platform/chaz/restart-service/workflow.yaml",
			wantPath: "platform",
		},
		{
			name:     "no workflows in path",
			path:     "/home/user/repo/platform/chaz/workflow.yaml",
			wantPath: "",
		},
		{
			name:     "shallow workflow path",
			path:     "/home/user/repo/workflows/test/workflow.yaml",
			wantPath: "test",
		},
		{
			name:     "relative path with workflows",
			path:     "workflows/ops/deploy/workflow.yaml",
			wantPath: "ops",
		},
		{
			name:     "relative path without workflows",
			path:     "ops/deploy/workflow.yaml",
			wantPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := b.extractIdentityPath(tt.path)
			if result != tt.wantPath {
				t.Errorf("extractIdentityPath() = %q, want %q", result, tt.wantPath)
			}
		})
	}
}

func TestFileModTime(t *testing.T) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	// Write content
	content := []byte("test content")
	if _, err := tmpFile.Write(content); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// Get mod time
	modTime, err := fileModTime(tmpFile.Name())
	if err != nil {
		t.Fatalf("fileModTime() failed: %v", err)
	}

	if modTime.IsZero() {
		t.Error("Expected non-zero mod time")
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr))))
}

func containsID(ids []string, id string) bool {
	for _, i := range ids {
		if i == id {
			return true
		}
	}
	return false
}
