package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/gitrepo"
	"github.com/chazuruo/svf/internal/index"
	"github.com/chazuruo/svf/internal/workflows"
)

// FileSystemStore implements the Store interface using the filesystem.
type FileSystemStore struct {
	repo        gitrepo.Repo
	config      *config.Config
	index       *index.Index
	indexMutex  sync.RWMutex
	indexLoaded bool
}

// New creates a new FileSystemStore.
func New(repo gitrepo.Repo, cfg *config.Config) (*FileSystemStore, error) {
	if repo == nil {
		return nil, fmt.Errorf("repo cannot be nil")
	}
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	return &FileSystemStore{
		repo:   repo,
		config: cfg,
	}, nil
}

// List returns workflow references matching the given filter.
func (s *FileSystemStore) List(ctx context.Context, filter Filter) ([]WorkflowRef, error) {
	var refs []WorkflowRef

	// Load index if we need to filter by tags or search
	needsIndex := len(filter.Tags) > 0 || filter.Search != ""
	if needsIndex {
		if err := s.loadIndex(ctx); err != nil {
			// If index fails to load, log warning but continue without it
			fmt.Fprintf(os.Stderr, "Warning: failed to load index: %v\n", err)
		}
	}

	// Walk workflows directory
	workflowRoot := filepath.Join(s.repo.Path(), s.config.Workflows.Root)
	err := filepath.Walk(workflowRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process workflow.yaml files
		if info.Name() != "workflow.yaml" && info.Name() != "workflow.yml" {
			return nil
		}

		// Apply filter
		ref, err := s.pathToRef(path)
		if err != nil {
			return err
		}

		if s.matchesFilter(ref, filter, path) {
			refs = append(refs, ref)
		}

		return nil
	})

	return refs, err
}

// Load reads a workflow from the store by its reference.
func (s *FileSystemStore) Load(ctx context.Context, ref WorkflowRef) (*workflows.Workflow, error) {
	data, err := os.ReadFile(ref.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow: %w", err)
	}

	wf, err := workflows.UnmarshalWorkflow(data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow: %w", err)
	}

	return wf, nil
}

// Save writes a workflow to the store.
func (s *FileSystemStore) Save(ctx context.Context, wf *workflows.Workflow, opts SaveOptions) (WorkflowRef, error) {
	// Generate slug if not set
	slug := wf.ID
	if slug == "" {
		slug = Slugify(wf.Title)
		if slug == "" {
			return WorkflowRef{}, fmt.Errorf("cannot generate slug from title")
		}
	}

	// Determine the save path
	dirPath, err := s.resolvePath(slug, opts)
	if err != nil {
		return WorkflowRef{}, err
	}

	workflowPath := filepath.Join(dirPath, "workflow.yaml")

	// Check if file exists and Force is not set
	if _, err := os.Stat(workflowPath); err == nil && !opts.Force {
		return WorkflowRef{}, fmt.Errorf("workflow already exists at %s (use Force to overwrite)", workflowPath)
	}

	// Create directory if needed
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return WorkflowRef{}, fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal workflow to YAML
	data, err := workflows.MarshalWorkflow(wf)
	if err != nil {
		return WorkflowRef{}, fmt.Errorf("failed to marshal workflow: %w", err)
	}

	// Write workflow.yaml
	if err := os.WriteFile(workflowPath, data, 0644); err != nil {
		return WorkflowRef{}, fmt.Errorf("failed to write workflow: %w", err)
	}

	// Generate README.md (optional)
	readmePath := filepath.Join(dirPath, "README.md")
	if err := s.generateReadme(readmePath, wf); err != nil {
		// Don't fail on README error
		fmt.Fprintf(os.Stderr, "Warning: failed to generate README: %v\n", err)
	}

	ref := WorkflowRef{
		ID:        wf.ID,
		Slug:      slug,
		Path:      workflowPath,
		UpdatedAt: time.Now(),
	}

	// Auto-commit if requested
	if opts.Commit {
		message := opts.Message
		if message == "" {
			message = fmt.Sprintf("Save workflow: %s", wf.Title)
		}
		if err := s.commitWorkflow(ctx, workflowPath, message); err != nil {
			return WorkflowRef{}, fmt.Errorf("failed to commit: %w", err)
		}
	}

	return ref, nil
}

// Delete removes a workflow from the store.
func (s *FileSystemStore) Delete(ctx context.Context, ref WorkflowRef) error {
	// Delete the workflow directory (containing workflow.yaml and README.md)
	workflowDir := filepath.Dir(ref.Path)

	if err := os.RemoveAll(workflowDir); err != nil {
		return fmt.Errorf("failed to delete workflow: %w", err)
	}

	return nil
}

// resolvePath determines the directory path for a workflow based on its slug.
func (s *FileSystemStore) resolvePath(slug string, opts SaveOptions) (string, error) {
	repoPath := s.repo.Path()

	// Check if this is a shared workflow or identity workflow
	// For now, we'll use the workflows root with identity path
	identityPath := s.config.Identity.Path
	if identityPath == "" {
		identityPath = "default"
	}

	// workflows/<identity.path>/<slug>/
	workflowPath := filepath.Join(repoPath, s.config.Workflows.Root, identityPath, slug)

	return workflowPath, nil
}

// pathToRef converts a file path to a WorkflowRef.
func (s *FileSystemStore) pathToRef(path string) (WorkflowRef, error) {
	info, err := os.Stat(path)
	if err != nil {
		return WorkflowRef{}, err
	}

	// Extract slug from path
	// Expected: .../workflows/<identity>/<slug>/workflow.yaml
	dir := filepath.Dir(path)
	slug := filepath.Base(dir)

	return WorkflowRef{
		ID:        "", // ID is loaded from the workflow file
		Slug:      slug,
		Path:      path,
		UpdatedAt: info.ModTime(),
	}, nil
}

// matchesFilter checks if a workflow reference matches the given filter.
func (s *FileSystemStore) matchesFilter(ref WorkflowRef, filter Filter, path string) bool {
	// Filter by identity path
	if filter.IdentityPath != "" {
		// Extract identity path from: workflows/<identity>/<slug>/workflow.yaml
		parts := strings.Split(path, string(filepath.Separator))
		found := false
		for i, part := range parts {
			if part == "workflows" && i+1 < len(parts) {
				if parts[i+1] != filter.IdentityPath {
					return false
				}
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by tags using the index
	if len(filter.Tags) > 0 {
		if !s.indexLoaded || s.index == nil {
			// Index not available, skip tag filtering
			return true
		}

		// Get relative path from repo root
		relPath, err := filepath.Rel(s.repo.Path(), path)
		if err != nil {
			relPath = path
		}

		// Find the entry in the index
		entry := s.index.GetByPath(relPath)
		if entry == nil {
			// Workflow not in index, skip it
			return false
		}

		// Check if all tags are present
		for _, filterTag := range filter.Tags {
			found := false
			for _, entryTag := range entry.Tags {
				if strings.EqualFold(strings.TrimSpace(entryTag), filterTag) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	// Filter by search using the index
	if filter.Search != "" {
		if !s.indexLoaded || s.index == nil {
			// Index not available, skip search filtering
			return true
		}

		// Get relative path from repo root
		relPath, err := filepath.Rel(s.repo.Path(), path)
		if err != nil {
			relPath = path
		}

		// Check if this workflow matches the search query
		matches := false
		searchResults := s.index.Search(filter.Search)
		for _, result := range searchResults {
			if result.Path == relPath {
				matches = true
				break
			}
		}
		if !matches {
			return false
		}
	}

	return true
}

// generateReadme creates a README.md file for the workflow.
func (s *FileSystemStore) generateReadme(path string, wf *workflows.Workflow) error {
	content := fmt.Sprintf("# %s\n\n", wf.Title)

	if wf.Description != "" {
		content += wf.Description + "\n\n"
	}

	if len(wf.Tags) > 0 {
		content += "## Tags\n\n"
		for _, tag := range wf.Tags {
			content += fmt.Sprintf("- %s\n", tag)
		}
		content += "\n"
	}

	if len(wf.Steps) > 0 {
		content += "## Steps\n\n"
		for i, step := range wf.Steps {
			name := step.Name
			if name == "" {
				name = fmt.Sprintf("Step %d", i+1)
			}
			content += fmt.Sprintf("### %s\n\n", name)
			content += fmt.Sprintf("```\n%s\n```\n\n", step.Command)
		}
	}

	return os.WriteFile(path, []byte(content), 0644)
}

// commitWorkflow adds and commits a workflow file.
func (s *FileSystemStore) commitWorkflow(ctx context.Context, path, message string) error {
	// Add all changes to ensure workflow.yaml and README.md are both staged
	if err := s.repo.AddAll(ctx); err != nil {
		return fmt.Errorf("failed to add files: %w", err)
	}

	// Commit
	if _, err := s.repo.CommitAll(ctx, message); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	return nil
}

// loadIndex loads the search index, always loading from disk to get the latest version.
func (s *FileSystemStore) loadIndex(ctx context.Context) error {
	s.indexMutex.Lock()
	defer s.indexMutex.Unlock()

	builder := index.NewBuilder(s.repo.Path(), s.config)
	idx, err := builder.Load()
	if err != nil {
		// If the index doesn't exist, try to build it
		idx, err = builder.Build()
		if err != nil {
			return fmt.Errorf("failed to build index: %w", err)
		}
		// Save the index for next time
		if saveErr := builder.Save(idx); saveErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save index: %v\n", saveErr)
		}
	}

	s.index = idx
	s.indexLoaded = true
	return nil
}
