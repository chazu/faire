package index

import (
	"context"
	"fmt"

	"github.com/chazuruo/faire/internal/config"
	"github.com/chazuruo/faire/internal/gitrepo"
	"github.com/chazuruo/faire/internal/workflows/store"
)

// Searcher provides search functionality over workflows.
type Searcher struct {
	builder *Builder
	store   store.Store
	config  *config.Config
}

// NewSearcher creates a new searcher.
func NewSearcher(repo gitrepo.Repo, str store.Store, cfg *config.Config) *Searcher {
	return &Searcher{
		builder: NewBuilder(repo, str, cfg),
		store:   str,
		config:  cfg,
	}
}

// Search searches workflows using the index if available, otherwise falls back to store search.
func (s *Searcher) Search(ctx context.Context, opts SearchOptions) ([]Entry, error) {
	// Try to use index if it exists
	if s.builder.IndexExists() {
		idx, err := s.builder.load()
		if err == nil {
			return idx.Search(opts), nil
		}
	}

	// Fallback: Build index and search
	_, err := s.builder.Build(ctx)
	if err != nil {
		// If index build fails, fall back to store search
		return s.fallbackSearch(ctx, opts)
	}

	// Load the newly built index and search
	idx, err := s.builder.load()
	if err != nil {
		return nil, fmt.Errorf("failed to load index: %w", err)
	}

	return idx.Search(opts), nil
}

// fallbackSearch performs a search using the store directly (slower).
func (s *Searcher) fallbackSearch(ctx context.Context, opts SearchOptions) ([]Entry, error) {
	refs, err := s.store.List(ctx, store.Filter{
		IdentityPath: opts.IdentityPath,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows: %w", err)
	}

	var results []Entry

	for _, ref := range refs {
		// Load workflow for full-text search
		wf, err := s.store.Load(ctx, ref)
		if err != nil {
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

		// Check if entry matches
		if s.matches(entry, opts) {
			results = append(results, entry)
			if len(results) >= opts.Limit {
				break
			}
		}
	}

	return results, nil
}

// matches checks if an entry matches the search options.
func (s *Searcher) matches(entry Entry, opts SearchOptions) bool {
	// Filter by tags (all must match)
	if len(opts.Tags) > 0 && !hasAllTags(entry.Tags, opts.Tags) {
		return false
	}

	// Filter by query
	if opts.Query != "" && !matchesQuery(entry, opts.Query) {
		return false
	}

	return true
}

// EnsureIndex ensures the index is built, building it if necessary.
func (s *Searcher) EnsureIndex(ctx context.Context) error {
	if s.builder.IndexExists() {
		// Try incremental update
		_, err := s.builder.UpdateIncremental(ctx)
		return err
	}

	// Build new index
	_, err := s.builder.Build(ctx)
	return err
}

// GetIndex returns the current index, building it if necessary.
func (s *Searcher) GetIndex(ctx context.Context) (*Index, error) {
	if s.builder.IndexExists() {
		idx, err := s.builder.load()
		if err == nil {
			return idx, nil
		}
	}

	// Build index
	return s.builder.Build(ctx)
}
