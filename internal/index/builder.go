package index

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chazuruo/faire/internal/config"
	"github.com/chazuruo/faire/internal/gitrepo"
	"github.com/chazuruo/faire/internal/workflows/store"
)

// Builder builds and updates the workflow index.
type Builder struct {
	repo      gitrepo.Repo
	store     store.Store
	config    *config.Config
	indexPath string
}

// NewBuilder creates a new index builder.
func NewBuilder(repo gitrepo.Repo, str store.Store, cfg *config.Config) *Builder {
	indexPath := filepath.Join(repo.Path(), DefaultIndexDir, DefaultIndexFileName)

	return &Builder{
		repo:      repo,
		store:     str,
		config:    cfg,
		indexPath: indexPath,
	}
}

// Build builds a new index from scratch.
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

		entry := Entry{
			ID:           wf.ID,
			Slug:         ref.Slug,
			Title:        wf.Title,
			Path:         ref.Path,
			IdentityPath: extractIdentityPath(ref.Path),
			Tags:         wf.Tags,
			UpdatedAt:    ref.UpdatedAt,
			SearchText:   buildSearchText(wf),
		}
		entries = append(entries, entry)
	}

	// Create index
	idx := &Index{
		Version:   IndexVersion,
		UpdatedAt: time.Now(),
		Workflows: entries,
	}

	// Save index
	if err := idx.Save(b.indexPath); err != nil {
		return nil, fmt.Errorf("failed to save index: %w", err)
	}

	return idx, nil
}

// UpdateIncremental updates the index incrementally by checking for changes.
func (b *Builder) UpdateIncremental(ctx context.Context) (*Index, error) {
	// Load existing index
	existing, err := b.load()
	if err != nil {
		// If no existing index, do a full build
		return b.Build(ctx)
	}

	// Detect changes
	changed, new, deleted := b.detectChanges(existing, ctx)

	// If no changes, return existing
	if len(changed) == 0 && len(new) == 0 && len(deleted) == 0 {
		return existing, nil
	}

	// Update entries
	updated := b.updateEntries(existing, changed, new, deleted, ctx)

	// Save updated index
	if err := updated.Save(b.indexPath); err != nil {
		return nil, fmt.Errorf("failed to save index: %w", err)
	}

	return updated, nil
}

// load loads the existing index from disk.
func (b *Builder) load() (*Index, error) {
	return Load(b.indexPath)
}

// detectChanges detects new, changed, and deleted workflows.
func (b *Builder) detectChanges(existing *Index, ctx context.Context) (changed, new, deleted []string) {
	// Get current workflow list
	refs, err := b.store.List(ctx, store.Filter{})
	if err != nil {
		return nil, nil, nil
	}

	// Build map of existing entries by path
	existingMap := make(map[string]Entry)
	for _, entry := range existing.Workflows {
		existingMap[entry.Path] = entry
	}

	// Build map of current refs by path
	currentMap := make(map[string]store.WorkflowRef)
	for _, ref := range refs {
		currentMap[ref.Path] = ref
	}

	// Find new and changed workflows
	for path, ref := range currentMap {
		existingEntry, exists := existingMap[path]

		if !exists {
			// New workflow
			new = append(new, path)
		} else if ref.UpdatedAt.After(existingEntry.UpdatedAt) {
			// Changed workflow
			changed = append(changed, path)
		}
	}

	// Find deleted workflows
	for path := range existingMap {
		if _, exists := currentMap[path]; !exists {
			deleted = append(deleted, path)
		}
	}

	return changed, new, deleted
}

// updateEntries updates the index with new, changed, and deleted workflows.
func (b *Builder) updateEntries(existing *Index, changed, new, deleted []string, ctx context.Context) *Index {
	// Create new index
	updated := &Index{
		Version:   IndexVersion,
		UpdatedAt: time.Now(),
		Workflows: make([]Entry, 0, len(existing.Workflows)),
	}

	// Build map of workflows to process
	processMap := make(map[string]bool)
	for _, path := range changed {
		processMap[path] = true
	}
	for _, path := range new {
		processMap[path] = true
	}

	// Copy existing entries that aren't changed or deleted
	deletedMap := make(map[string]bool)
	for _, path := range deleted {
		deletedMap[path] = true
	}

	for _, entry := range existing.Workflows {
		if !processMap[entry.Path] && !deletedMap[entry.Path] {
			updated.Workflows = append(updated.Workflows, entry)
		}
	}

	// Process changed and new workflows
	for path := range processMap {
		ref := store.WorkflowRef{Path: path}
		wf, err := b.store.Load(ctx, ref)
		if err != nil {
			continue
		}

		entry := Entry{
			ID:           wf.ID,
			Slug:         slugify(wf.Title),
			Title:        wf.Title,
			Path:         path,
			IdentityPath: extractIdentityPath(path),
			Tags:         wf.Tags,
			UpdatedAt:    time.Now(),
			SearchText:   buildSearchText(wf),
		}
		updated.Workflows = append(updated.Workflows, entry)
	}

	return updated
}

// GetIndexPath returns the path to the index file.
func (b *Builder) GetIndexPath() string {
	return b.indexPath
}

// IndexExists checks if the index file exists.
func (b *Builder) IndexExists() bool {
	_, err := os.Stat(b.indexPath)
	return err == nil
}

// ForceRebuild forces a full rebuild of the index.
func (b *Builder) ForceRebuild(ctx context.Context) (*Index, error) {
	return b.Build(ctx)
}
