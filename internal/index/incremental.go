package index

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chazuruo/faire/internal/workflows/store"
)

// changeSet represents changes detected since the last index build.
type changeSet struct {
	changed  []string // Paths to workflows that were modified
	new      []string // Paths to workflows that were added
	deleted  []string // Paths to workflows that were deleted
	unchanged []string // Paths to workflows that were not modified
}

// UpdateIncremental updates the index incrementally by detecting changes.
func (b *Builder) UpdateIncremental(ctx context.Context) (*Index, error) {
	// Load existing index
	existing, err := b.Load(ctx)
	if err != nil {
		return nil, err
	}

	if existing == nil {
		// No existing index, do a full build
		return b.BuildAndSave(ctx)
	}

	// Detect changes
	changes, err := b.detectChanges(ctx, existing)
	if err != nil {
		return nil, fmt.Errorf("failed to detect changes: %w", err)
	}

	// If no changes, return existing index
	if len(changes.changed) == 0 && len(changes.new) == 0 && len(changes.deleted) == 0 {
		return existing, nil
	}

	// Update entries
	updated, err := b.updateEntries(ctx, existing, changes)
	if err != nil {
		return nil, fmt.Errorf("failed to update entries: %w", err)
	}

	// Save updated index
	if err := b.Save(updated); err != nil {
		return nil, err
	}

	return updated, nil
}

// detectChanges compares the existing index with the filesystem to detect changes.
func (b *Builder) detectChanges(ctx context.Context, existing *Index) (*changeSet, error) {
	changes := &changeSet{}

	// Create a map of existing entries by path
	entryMap := make(map[string]Entry)
	for _, entry := range existing.Workflows {
		entryMap[entry.Path] = entry
	}

	// List current workflows
	refs, err := b.store.List(ctx, store.Filter{})
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows: %w", err)
	}

	// Create a set of current paths
	currentPaths := make(map[string]bool)
	for _, ref := range refs {
		currentPaths[ref.Path] = true
	}

	// Check for modifications and new workflows
	for _, ref := range refs {
		entry, exists := entryMap[ref.Path]

		if !exists {
			// New workflow
			changes.new = append(changes.new, ref.Path)
		} else {
			// Check if modified
			modTime, err := fileModTime(ref.Path)
			if err != nil {
				continue
			}

			// If file mod time is newer than index entry, mark as changed
			if modTime.After(entry.UpdatedAt) {
				changes.changed = append(changes.changed, ref.Path)
			} else {
				changes.unchanged = append(changes.unchanged, ref.Path)
			}
		}
	}

	// Check for deleted workflows
	for path := range entryMap {
		if !currentPaths[path] {
			changes.deleted = append(changes.deleted, path)
		}
	}

	return changes, nil
}

// updateEntries updates the index entries based on the detected changes.
func (b *Builder) updateEntries(ctx context.Context, existing *Index, changes *changeSet) (*Index, error) {
	updated := &Index{
		Version:   existing.Version,
		UpdatedAt: time.Now(),
		Workflows: make([]Entry, 0, len(existing.Workflows)),
	}

	// Keep unchanged entries
	for _, path := range changes.unchanged {
		if entry := existing.FindByPath(path); entry != nil {
			updated.Workflows = append(updated.Workflows, *entry)
		}
	}

	// Update changed entries
	for _, path := range changes.changed {
		entry, err := b.buildEntryFromPath(ctx, path)
		if err != nil {
			// Skip entries that fail to build
			continue
		}
		updated.Add(*entry)
	}

	// Add new entries
	for _, path := range changes.new {
		entry, err := b.buildEntryFromPath(ctx, path)
		if err != nil {
			// Skip entries that fail to build
			continue
		}
		updated.Add(*entry)
	}

	// Deleted entries are simply not added (removed by omission)

	return updated, nil
}

// buildEntryFromPath builds an index entry from a workflow file path.
func (b *Builder) buildEntryFromPath(ctx context.Context, path string) (*Entry, error) {
	// Get file info for mod time
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Create a workflow reference
	ref := store.WorkflowRef{
		Path:      path,
		Slug:      filepath.Base(filepath.Dir(path)),
		UpdatedAt: info.ModTime(),
	}

	// Load workflow
	wf, err := b.store.Load(ctx, ref)
	if err != nil {
		return nil, err
	}

	// Extract identity path
	identityPath := b.extractIdentityPath(path)

	// Generate slug from title
	slug := store.Slugify(wf.Title)
	if slug == "" {
		slug = wf.ID
	}

	return &Entry{
		ID:           wf.ID,
		Slug:         slug,
		Title:        wf.Title,
		Path:         path,
		IdentityPath: identityPath,
		Tags:         wf.Tags,
		CreatedAt:    time.Now(), // Will be updated when workflow has CreatedAt
		UpdatedAt:    info.ModTime(),
		SearchText:   b.buildSearchText(wf),
	}, nil
}
