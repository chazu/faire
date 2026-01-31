// Package index provides workflow indexing for search functionality.
package index

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chazuruo/svf/internal/workflows"
)

// Index represents the search index.
type Index struct {
	Version   int       `json:"version"`
	UpdatedAt string    `json:"updated_at"`
	Workflows []WorkflowEntry `json:"workflows"`
}

// WorkflowEntry represents a workflow in the index.
type WorkflowEntry struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Path       string `json:"path"`
	Tags       string `json:"tags"`           // Comma-separated for JSON
	UpdatedAt  string `json:"updated_at"`
	SearchText string `json:"search_text"`    // Concatenated searchable text
}

// Builder builds and maintains the search index.
type Builder struct {
	repoPath  string
	indexPath string
}

// NewBuilder creates a new index builder.
func NewBuilder(repoPath string) *Builder {
	return &Builder{
		repoPath:  repoPath,
		indexPath: filepath.Join(repoPath, ".svf", "index.json"),
	}
}

// Build builds the index by scanning workflow directories.
func (b *Builder) Build() (*Index, error) {
	index := &Index{
		Version:   1,
		UpdatedAt: time.Now().Format(time.RFC3339),
		Workflows: []WorkflowEntry{},
	}

	// Scan workflows directory
	workflowsDir := filepath.Join(b.repoPath, ".svf", "workflows")
	if err := b.scanDirectory(workflowsDir, index); err != nil {
		return nil, fmt.Errorf("scanning workflows directory: %w", err)
	}

	// Scan shared directory
	sharedDir := filepath.Join(b.repoPath, ".svf", "shared")
	if err := b.scanDirectory(sharedDir, index); err != nil {
		return nil, fmt.Errorf("scanning shared directory: %w", err)
	}

	// Sort by title for consistent ordering
	sort.Slice(index.Workflows, func(i, j int) bool {
		return index.Workflows[i].Title < index.Workflows[j].Title
	})

	return index, nil
}

// scanDirectory scans a directory for workflow files.
func (b *Builder) scanDirectory(dir string, index *Index) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist, skip
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .yaml files
		if !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		wfEntry, err := b.indexWorkflow(path)
		if err != nil {
			// Log but continue - don't fail entire build for one bad file
			fmt.Fprintf(os.Stderr, "Warning: failed to index %s: %v\n", path, err)
			continue
		}

		if wfEntry != nil {
			index.Workflows = append(index.Workflows, *wfEntry)
		}
	}

	return nil
}

// indexWorkflow indexes a single workflow file.
func (b *Builder) indexWorkflow(path string) (*WorkflowEntry, error) {
	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Get file mod time
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Parse workflow (lightweight parsing, just extract metadata)
	wf, err := workflows.UnmarshalWorkflow(data)
	if err != nil {
		return nil, err
	}

	// Get relative path from repo root
	relPath, err := filepath.Rel(b.repoPath, path)
	if err != nil {
		relPath = path
	}

	// Compute ID from path (remove .svf/ prefix and .yaml suffix)
	id := strings.TrimPrefix(relPath, ".svf/")
	id = strings.TrimSuffix(id, ".yaml")

	// Build searchable text from title, description, tags, and commands
	var searchText strings.Builder
	searchText.WriteString(wf.Title)
	searchText.WriteString(" ")
	if wf.Description != "" {
		searchText.WriteString(wf.Description)
		searchText.WriteString(" ")
	}
	for _, tag := range wf.Tags {
		searchText.WriteString(tag)
		searchText.WriteString(" ")
	}
	for _, step := range wf.Steps {
		searchText.WriteString(step.Command)
		searchText.WriteString(" ")
	}

	return &WorkflowEntry{
		ID:         id,
		Title:      wf.Title,
		Path:       relPath,
		Tags:       strings.Join(wf.Tags, ","),
		UpdatedAt:  info.ModTime().Format(time.RFC3339),
		SearchText: strings.TrimSpace(searchText.String()),
	}, nil
}

// Save saves the index to disk.
func (b *Builder) Save(index *Index) error {
	// Ensure directory exists
	dir := filepath.Dir(b.indexPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Marshal with pretty formatting
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(b.indexPath, data, 0644)
}

// Load loads the index from disk.
func (b *Builder) Load() (*Index, error) {
	data, err := os.ReadFile(b.indexPath)
	if err != nil {
		return nil, err
	}

	var index Index
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}

	return &index, nil
}

// IsStale checks if the index needs rebuilding.
func (b *Builder) IsStale() (bool, error) {
	index, err := b.Load()
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil // No index exists, need to build
		}
		return false, err
	}

	// Check if index version is current
	if index.Version != 1 {
		return true, nil
	}

	// Check if any workflow file is newer than the index
	indexInfo, err := os.Stat(b.indexPath)
	if err != nil {
		return false, err
	}

	// Check workflows directory
	workflowsDir := filepath.Join(b.repoPath, ".svf", "workflows")
	stale, err := b.checkDirectoryStale(workflowsDir, indexInfo.ModTime())
	if err != nil {
		return false, err
	}
	if stale {
		return true, nil
	}

	// Check shared directory
	sharedDir := filepath.Join(b.repoPath, ".svf", "shared")
	stale, err = b.checkDirectoryStale(sharedDir, indexInfo.ModTime())
	if err != nil {
		return false, err
	}

	return stale, nil
}

// checkDirectoryStale checks if any file in directory is newer than the index.
func (b *Builder) checkDirectoryStale(dir string, indexModTime time.Time) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // Directory doesn't exist
		}
		return false, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().After(indexModTime) {
			return true, nil
		}
	}

	return false, nil
}

// Search searches the index for matching workflows.
func (i *Index) Search(query string) []WorkflowEntry {
	if query == "" {
		return i.Workflows
	}

	query = strings.ToLower(query)
	var results []WorkflowEntry

	for _, entry := range i.Workflows {
		searchable := strings.ToLower(entry.SearchText)
		if strings.Contains(searchable, query) {
			results = append(results, entry)
		}
	}

	return results
}

// GetByPath retrieves a workflow entry by path.
func (i *Index) GetByPath(path string) *WorkflowEntry {
	for _, entry := range i.Workflows {
		if entry.Path == path {
			return &entry
		}
	}
	return nil
}

// SearchResult represents a search result with ranking.
type SearchResult struct {
	Entry   WorkflowEntry
	Score   float64
	Matches []string // Matched field names
}

// SearchOptions contains search options.
type SearchOptions struct {
	Query      string
	Tags       []string    // Filter by tags
	Mine       bool       // Only user's workflows (path filter)
	Shared     bool       // Only shared workflows
	MaxResults int        // Limit results (0 for no limit)
}

// FuzzySearch performs fuzzy search with ranking and filtering.
func (i *Index) FuzzySearch(opts SearchOptions) []SearchResult {
	if opts.Query == "" && len(opts.Tags) == 0 {
		// No filters, return all with basic scoring
		results := make([]SearchResult, len(i.Workflows))
		for j, entry := range i.Workflows {
			results[j] = SearchResult{Entry: entry, Score: 1.0}
		}
		return results
	}

	query := strings.ToLower(opts.Query)
	var results []SearchResult

	for _, entry := range i.Workflows {
		// Apply path filters
		if opts.Mine && !strings.HasPrefix(entry.Path, ".svf/workflows/") {
			continue
		}
		if opts.Shared && !strings.HasPrefix(entry.Path, ".svf/shared/") {
			continue
		}

		// Apply tag filters
		if len(opts.Tags) > 0 {
			entryTags := strings.Split(entry.Tags, ",")
			match := false
			for _, filterTag := range opts.Tags {
				for _, entryTag := range entryTags {
					if strings.EqualFold(strings.TrimSpace(entryTag), filterTag) {
						match = true
						break
					}
				}
				if !match {
					break
				}
			}
			if !match {
				continue
			}
		}

		// Score the entry
		score, matches := i.scoreEntry(entry, query)
		if score > 0 {
			results = append(results, SearchResult{
				Entry:   entry,
				Score:   score,
				Matches: matches,
			})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Apply max results limit
	if opts.MaxResults > 0 && len(results) > opts.MaxResults {
		results = results[:opts.MaxResults]
	}

	return results
}

// scoreEntry scores an entry against the query.
func (i *Index) scoreEntry(entry WorkflowEntry, query string) (float64, []string) {
	if query == "" {
		return 1.0, []string{}
	}

	var score float64
	var matches []string

	title := strings.ToLower(entry.Title)
	searchable := strings.ToLower(entry.SearchText)

	// Exact title match gets highest score
	if title == query {
		score += 100
		matches = append(matches, "title")
	} else if strings.Contains(title, query) {
		score += 50
		matches = append(matches, "title")
	}

	// Title starts with query
	if strings.HasPrefix(title, query) {
		score += 30
	}

	// Word boundary match in title
	if strings.Contains(title, " "+query) || strings.Contains(title, query+" ") {
		score += 20
	}

	// Match in searchable text (description, tags, commands)
	if strings.Contains(searchable, query) {
		score += 10
		matches = append(matches, "content")
	}

	// Tag match
	if strings.Contains(strings.ToLower(entry.Tags), query) {
		score += 15
		matches = append(matches, "tags")
	}

	return score, matches
}

// FilterByTag filters workflows by tag.
func (i *Index) FilterByTag(tag string) []WorkflowEntry {
	tag = strings.ToLower(tag)
	var results []WorkflowEntry

	for _, entry := range i.Workflows {
		entryTags := strings.Split(entry.Tags, ",")
		for _, entryTag := range entryTags {
			if strings.EqualFold(strings.TrimSpace(entryTag), tag) {
				results = append(results, entry)
				break
			}
		}
	}

	return results
}

// FilterByPath filters workflows by path prefix.
func (i *Index) FilterByPath(prefix string) []WorkflowEntry {
	var results []WorkflowEntry

	for _, entry := range i.Workflows {
		if strings.HasPrefix(entry.Path, prefix) {
			results = append(results, entry)
		}
	}

	return results
}
