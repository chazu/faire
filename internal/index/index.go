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

	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/workflows"
)

const (
	// CurrentSchemaVersion is the index schema version
	CurrentSchemaVersion = 1
)

// Index represents the search index.
type Index struct {
	Version   int             `json:"version"`
	UpdatedAt string          `json:"updated_at"`
	Workflows []WorkflowEntry `json:"workflows"`
}

// WorkflowEntry represents a workflow in the index.
type WorkflowEntry struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Path       string   `json:"path"`
	Tags       []string `json:"tags"`
	UpdatedAt  string   `json:"updated_at"`
	SearchText string   `json:"search_text"` // Concatenated searchable text
}

// Builder builds and maintains the search index.
type Builder struct {
	config   *config.Config
	repoPath string
}

// NewBuilder creates a new index builder.
func NewBuilder(repoPath string, cfg *config.Config) *Builder {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	return &Builder{
		config:   cfg,
		repoPath: repoPath,
	}
}

// GetIndexPath returns the full path to the index file.
func (b *Builder) GetIndexPath() string {
	return filepath.Join(b.repoPath, b.config.Workflows.IndexPath)
}

// Build builds the index by scanning workflow directories.
func (b *Builder) Build() (*Index, error) {
	index := &Index{
		Version:   CurrentSchemaVersion,
		UpdatedAt: time.Now().Format(time.RFC3339),
		Workflows: []WorkflowEntry{},
	}

	// Scan workflows directory (user workflows)
	workflowsDir := filepath.Join(b.repoPath, b.config.Workflows.Root)
	if err := b.scanDirectory(workflowsDir, index, false); err != nil {
		return nil, fmt.Errorf("scanning workflows directory: %w", err)
	}

	// Scan shared directory
	sharedDir := filepath.Join(b.repoPath, b.config.Workflows.SharedRoot)
	if err := b.scanDirectory(sharedDir, index, true); err != nil {
		return nil, fmt.Errorf("scanning shared directory: %w", err)
	}

	// Sort by title for consistent ordering
	sort.Slice(index.Workflows, func(i, j int) bool {
		return index.Workflows[i].Title < index.Workflows[j].Title
	})

	return index, nil
}

// scanDirectory scans a directory for workflow files.
// Recursively finds all workflow.yaml files and extracts identity path from directory structure.
func (b *Builder) scanDirectory(dir string, index *Index, isShared bool) error {
	return b.scanDirectoryRecursive(dir, "", index, isShared)
}

// scanDirectoryRecursive recursively scans a directory for workflow files.
// identityPath accumulates the path components as we recurse.
func (b *Builder) scanDirectoryRecursive(dir string, identityPath string, index *Index, isShared bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist, skip
		}
		return err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			// Recurse into subdirectory
			newIdentityPath := filepath.Join(identityPath, entry.Name())
			if err := b.scanDirectoryRecursive(fullPath, newIdentityPath, index, isShared); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to scan %s: %v\n", fullPath, err)
			}
			continue
		}

		// Check if this is a workflow file
		if entry.Name() == "workflow.yaml" || entry.Name() == "workflow.yml" {
			// Extract slug from parent directory
			slug := filepath.Base(dir)
			wfEntry, err := b.indexWorkflow(fullPath, identityPath, slug, isShared)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to index %s: %v\n", fullPath, err)
				continue
			}

			if wfEntry != nil {
				index.Workflows = append(index.Workflows, *wfEntry)
			}
		}
	}

	return nil
}

// indexWorkflow indexes a single workflow file.
func (b *Builder) indexWorkflow(path string, identityPath string, slug string, isShared bool) (*WorkflowEntry, error) {
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

	// Parse workflow
	wf, err := workflows.UnmarshalWorkflow(data)
	if err != nil {
		return nil, err
	}

	// Get relative path from repo root
	relPath, err := filepath.Rel(b.repoPath, path)
	if err != nil {
		relPath = path
	}

	// Use workflow ID if available, otherwise generate from path
	id := wf.ID
	if id == "" {
		prefix := ""
		if isShared {
			prefix = "shared/"
		} else {
			prefix = identityPath + "/"
		}
		id = prefix + slug
	}

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
		Tags:       wf.Tags,
		UpdatedAt:  info.ModTime().Format(time.RFC3339),
		SearchText: strings.TrimSpace(searchText.String()),
	}, nil
}

// Save saves the index to disk.
func (b *Builder) Save(index *Index) error {
	indexPath := b.GetIndexPath()

	// Ensure directory exists
	dir := filepath.Dir(indexPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Marshal with pretty formatting
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(indexPath, data, 0644)
}

// Load loads the index from disk.
func (b *Builder) Load() (*Index, error) {
	indexPath := b.GetIndexPath()
	data, err := os.ReadFile(indexPath)
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
	if index.Version != CurrentSchemaVersion {
		return true, nil
	}

	// Check if any workflow file is newer than the index
	indexPath := b.GetIndexPath()
	indexInfo, err := os.Stat(indexPath)
	if err != nil {
		return false, err
	}

	// Check workflows directory
	workflowsDir := filepath.Join(b.repoPath, b.config.Workflows.Root)
	stale, err := b.checkDirectoryStale(workflowsDir, indexInfo.ModTime())
	if err != nil {
		return false, err
	}
	if stale {
		return true, nil
	}

	// Check shared directory
	sharedDir := filepath.Join(b.repoPath, b.config.Workflows.SharedRoot)
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
		if !entry.IsDir() {
			continue
		}

		// Check subdirectory
		subDir := filepath.Join(dir, entry.Name())
		stale, err := b.checkSubDirectoryStale(subDir, indexModTime)
		if err != nil {
			return false, err
		}
		if stale {
			return true, nil
		}
	}

	return false, nil
}

// checkSubDirectoryStale recursively checks subdirectories.
func (b *Builder) checkSubDirectoryStale(dir string, indexModTime time.Time) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			// Recursively check subdirectory
			stale, err := b.checkSubDirectoryStale(fullPath, indexModTime)
			if err != nil {
				return false, err
			}
			if stale {
				return true, nil
			}
		} else if entry.Name() == "workflow.yaml" || entry.Name() == "workflow.yml" {
			// Check workflow file mod time
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(indexModTime) {
				return true, nil
			}
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

// FilterByTags filters workflows by tags (all must match).
func (i *Index) FilterByTags(tags []string) []WorkflowEntry {
	if len(tags) == 0 {
		return i.Workflows
	}

	var results []WorkflowEntry

	for _, entry := range i.Workflows {
		// Check if all tags are present
		allMatch := true
		for _, filterTag := range tags {
			found := false
			for _, entryTag := range entry.Tags {
				if strings.EqualFold(strings.TrimSpace(entryTag), filterTag) {
					found = true
					break
				}
			}
			if !found {
				allMatch = false
				break
			}
		}
		if allMatch {
			results = append(results, entry)
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

// SearchResult represents a search result with ranking.
type SearchResult struct {
	Entry   WorkflowEntry
	Score   float64
	Matches []string // Matched field names
}

// SearchOptions contains search options.
type SearchOptions struct {
	Query        string
	Tags         []string // Filter by tags
	IdentityPath string   // Filter by identity path (e.g., "platform/chaz")
	Mine         bool     // Filter by identity path only (user's workflows)
	Shared       bool     // Filter by shared workflows only
	MaxResults   int      // Limit results (0 for no limit)
}

// FuzzySearch performs fuzzy search with ranking and filtering.
func (i *Index) FuzzySearch(opts SearchOptions) []SearchResult {
	if opts.Query == "" && len(opts.Tags) == 0 && opts.IdentityPath == "" && !opts.Mine && !opts.Shared {
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
		// Apply identity path filter
		if opts.IdentityPath != "" {
			// Entry path format: workflows/<identity-path>/<slug>/workflow.yaml
			// Identity path can contain slashes, so we need to reconstruct it
			// by removing the workflows/ prefix and the /<slug>/workflow.yaml suffix
			relPath := strings.TrimPrefix(entry.Path, "workflows/")
			// Remove the trailing "/<slug>/workflow.yaml" parts
			pathParts := strings.Split(relPath, string(filepath.Separator))
			if len(pathParts) < 2 {
				continue
			}
			// Identity path is everything except the last two parts (slug, filename)
			entryIdentityPath := filepath.Join(pathParts[:len(pathParts)-2]...)
			if entryIdentityPath != opts.IdentityPath {
				continue
			}
		}

		// Apply --mine filter (only user's workflows from workflows/ directory)
		if opts.Mine && opts.IdentityPath == "" {
			// Filter out shared workflows
			if strings.HasPrefix(entry.Path, "shared/") {
				continue
			}
		}

		// Apply --shared filter (only shared workflows)
		if opts.Shared {
			if !strings.HasPrefix(entry.Path, "shared/") {
				continue
			}
		}

		// Apply tag filters
		if len(opts.Tags) > 0 {
			allMatch := true
			for _, filterTag := range opts.Tags {
				found := false
				for _, entryTag := range entry.Tags {
					if strings.EqualFold(strings.TrimSpace(entryTag), filterTag) {
						found = true
						break
					}
				}
				if !found {
					allMatch = false
					break
				}
			}
			if !allMatch {
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

	// Fuzzy subsequence match in title (e.g., "tw" matches "test workflow")
	if fuzzyMatch(title, query) {
		if !strings.Contains(title, query) {
			score += 15
			matches = append(matches, "title")
		}
	}

	// Match in searchable text (description, tags, commands)
	if strings.Contains(searchable, query) {
		score += 10
		if !contains(matches, "content") {
			matches = append(matches, "content")
		}
	}

	// Tag exact or partial match
	for _, tag := range entry.Tags {
		tagLower := strings.ToLower(tag)
		if tagLower == query {
			score += 25
			if !contains(matches, "tags") {
				matches = append(matches, "tags")
			}
			break
		} else if strings.Contains(tagLower, query) {
			score += 15
			if !contains(matches, "tags") {
				matches = append(matches, "tags")
			}
		} else if fuzzyMatch(tagLower, query) {
			score += 8
			if !contains(matches, "tags") {
				matches = append(matches, "tags")
			}
		}
	}

	return score, matches
}

// fuzzyMatch checks if query is a fuzzy subsequence of text.
// For example, "tw" matches "test workflow" or "workflow".
func fuzzyMatch(text, query string) bool {
	if len(query) == 0 {
		return true
	}
	if len(query) > len(text) {
		return false
	}

	// For single character queries, just check if it exists
	if len(query) == 1 {
		return strings.Contains(text, query)
	}

	// For multi-character queries, check if all characters appear in order
	textIndex := 0
	queryIndex := 0

	for textIndex < len(text) && queryIndex < len(query) {
		if text[textIndex] == query[queryIndex] {
			queryIndex++
		}
		textIndex++
	}

	return queryIndex == len(query)
}

// contains checks if a string slice contains a value.
func contains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
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

// GetByID retrieves a workflow entry by ID.
func (i *Index) GetByID(id string) *WorkflowEntry {
	for _, entry := range i.Workflows {
		if entry.ID == id {
			return &entry
		}
	}
	return nil
}
