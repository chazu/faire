// Package index provides workflow indexing and search functionality.
//
// The index is stored as JSON at .gitsavvy/index.json and contains
// metadata about all workflows for fast searching and filtering.
package index

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	// DefaultIndexRelPath is the default relative path to the index file.
	DefaultIndexRelPath = ".gitsavvy/index.json"

	// CurrentVersion is the current index format version.
	CurrentVersion = 1
)

// Index represents the workflow index.
type Index struct {
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
	Workflows []Entry   `json:"workflows"`
}

// Entry represents a single workflow entry in the index.
type Entry struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Path        string    `json:"path"`
	IdentityPath string   `json:"identity_path"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	SearchText  string    `json:"search_text"`
}

// Load loads an index from the given path.
// Returns nil if the index file doesn't exist.
func Load(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}

	return &idx, nil
}

// Save writes the index to the given path.
func (idx *Index) Save(path string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// FindByID finds an entry by ID.
func (idx *Index) FindByID(id string) *Entry {
	for _, entry := range idx.Workflows {
		if entry.ID == id {
			return &entry
		}
	}
	return nil
}

// FindByPath finds an entry by workflow file path.
func (idx *Index) FindByPath(path string) *Entry {
	for _, entry := range idx.Workflows {
		if entry.Path == path {
			return &entry
		}
	}
	return nil
}

// Add adds an entry to the index.
func (idx *Index) Add(entry Entry) {
	// Remove existing entry with same ID or path
	idx.Remove(entry.ID, entry.Path)
	idx.Workflows = append(idx.Workflows, entry)
}

// Remove removes an entry by ID or path.
// Returns true if an entry was removed.
func (idx *Index) Remove(id, path string) bool {
	var updated []Entry
	found := false

	for _, entry := range idx.Workflows {
		if entry.ID == id || entry.Path == path {
			found = true
			continue
		}
		updated = append(updated, entry)
	}

	if found {
		idx.Workflows = updated
	}

	return found
}

// Len returns the number of entries in the index.
func (idx *Index) Len() int {
	return len(idx.Workflows)
}
