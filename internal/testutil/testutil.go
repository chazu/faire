// Package testutil provides helper functions for testing.
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// TempDir creates a temporary directory and registers a cleanup function.
// The directory is automatically deleted when the test completes.
func TempDir(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("failed to cleanup temp dir %s: %v", dir, err)
		}
	})

	return dir
}

// WriteWorkflow writes content to a temporary file and returns the path.
// The file is automatically deleted when the test completes.
func WriteWorkflow(t *testing.T, content string) string {
	t.Helper()

	dir := TempDir(t)
	path := filepath.Join(dir, "workflow.yaml")

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write workflow file: %v", err)
	}

	return path
}
