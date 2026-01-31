package gitrepo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Init initializes a new Git repository with the given options.
func (r *gitRepo) Init(ctx context.Context, opts InitOptions) error {
	args := []string{"init"}

	if opts.Bare {
		args = append(args, "--bare")
	}

	_, _, err := r.runGit(ctx, args...)
	if err != nil {
		return fmt.Errorf("init failed: %w", err)
	}

	// Set default branch if specified
	if opts.DefaultBranch != "" {
		// Use -m option with init to set initial branch name
		// This is the modern way (Git 2.28+)
		_, _, err := r.runGit(ctx, "init", "-b", opts.DefaultBranch)
		if err != nil {
			// If -b fails, try the older method using symbolic-ref
			_, _, err = r.runGit(ctx, "symbolic-ref", "HEAD", "refs/heads/"+opts.DefaultBranch)
			if err != nil {
				return fmt.Errorf("failed to set default branch to %s: %w", opts.DefaultBranch, err)
			}
		}
	}

	return nil
}

// InitRepo creates and initializes a new Git repository at the given path.
// This is a convenience function that creates the Repo instance and initializes it.
func InitRepo(ctx context.Context, path string, opts InitOptions) (Repo, error) {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	repo := New(absPath)
	if err := repo.Init(ctx, opts); err != nil {
		return nil, err
	}

	return repo, nil
}

// Open opens an existing Git repository at the given path.
// It returns an error if the path is not a valid Git repository.
func Open(path string) (Repo, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	repo := New(absPath)
	if !repo.IsInitialized(context.Background()) {
		return nil, fmt.Errorf("not a Git repository: %s", path)
	}

	return repo, nil
}
