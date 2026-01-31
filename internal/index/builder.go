package index

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chazuruo/faire/internal/gitrepo"
	"github.com/chazuruo/faire/internal/workflows"
	"github.com/chazuruo/faire/internal/workflows/store"
)

// Builder builds a workflow index.
type Builder struct {
	repo      gitrepo.Repo
	store     store.Store
	indexPath string // Path to index file (e.g., .gitsavvy/index.json)
}

// NewBuilder creates a new Builder.
func NewBuilder(repo gitrepo.Repo, st store.Store, indexPath string) *Builder {
	if indexPath == "" {
		indexPath = filepath.Join(repo.Path(), DefaultIndexRelPath)
	}
	return &Builder{
		repo:      repo,
		store:     st,
		indexPath: indexPath,
	}
}

// Build builds a new index from all workflows.
func (b *Builder) Build(ctx context.Context) (*Index, error) {
	// List all workflows
	refs, err := b.store.List(ctx, store.Filter{})
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows: %w", err)
	}

	// Build entries
	entries := make([]Entry, 0, len(refs))
	for _, ref := range refs {
		// Load workflow for metadata
		wf, err := b.store.Load(ctx, ref)
		if err != nil {
			// Skip workflows that fail to load
			continue
		}

		// Extract identity path from workflow path
		// Expected: .../workflows/<identity>/<slug>/workflow.yaml
		identityPath := b.extractIdentityPath(ref.Path)

		// Generate slug from title if not provided
		slug := store.Slugify(wf.Title)
		if slug == "" {
			slug = wf.ID
		}

		entry := Entry{
			ID:           wf.ID,
			Slug:         slug,
			Title:        wf.Title,
			Path:         ref.Path,
			IdentityPath: identityPath,
			Tags:         wf.Tags,
			CreatedAt:    time.Now(), // Will be updated when workflow has CreatedAt
			UpdatedAt:    ref.UpdatedAt,
			SearchText:   b.buildSearchText(wf),
		}

		entries = append(entries, entry)
	}

	// Create index
	idx := &Index{
		Version:   CurrentVersion,
		UpdatedAt: time.Now(),
		Workflows: entries,
	}

	return idx, nil
}

// buildSearchText builds searchable text from a workflow.
func (b *Builder) buildSearchText(wf *workflows.Workflow) string {
	slug := store.Slugify(wf.Title)

	parts := []string{
		wf.Title,
		slug,
	}
	parts = append(parts, wf.Tags...)

	// Add description words
	if wf.Description != "" {
		parts = append(parts, strings.Fields(wf.Description)...)
	}

	// Add placeholder names
	for name := range wf.Placeholders {
		parts = append(parts, name)
	}

	return strings.ToLower(strings.Join(parts, " "))
}

// extractIdentityPath extracts the identity path from a workflow file path.
// Expected: .../workflows/<identity>/<slug>/workflow.yaml
func (b *Builder) extractIdentityPath(path string) string {
	// Normalize path separators
	path = filepath.ToSlash(path)

	// Find "workflows/" in path
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "workflows" && i+1 < len(parts) {
			// Return the identity part (after "workflows/")
			return parts[i+1]
		}
	}

	return ""
}

// Load loads the existing index from disk.
func (b *Builder) Load(ctx context.Context) (*Index, error) {
	idx, err := Load(b.indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load index: %w", err)
	}
	return idx, nil
}

// Save saves the index to disk.
func (b *Builder) Save(idx *Index) error {
	idx.UpdatedAt = time.Now()
	if err := idx.Save(b.indexPath); err != nil {
		return fmt.Errorf("failed to save index: %w", err)
	}
	return nil
}

// BuildAndSave builds a new index and saves it to disk.
func (b *Builder) BuildAndSave(ctx context.Context) (*Index, error) {
	idx, err := b.Build(ctx)
	if err != nil {
		return nil, err
	}

	if err := b.Save(idx); err != nil {
		return nil, err
	}

	return idx, nil
}

// fileModTime returns the modification time of a file.
func fileModTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
