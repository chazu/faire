package gitrepo

import (
	"context"
)

// Init initializes a new Git repository.
func (r *gitRepo) Init(ctx context.Context, opts InitOptions) error {
	args := []string{"init"}
	if opts.Bare {
		args = append(args, "--bare")
	}
	if _, _, err := r.runGit(ctx, args...); err != nil {
		return err
	}

	// Set default branch if specified
	if opts.DefaultBranch != "" {
		_, _, _ = r.runGit(ctx, "branch", "-M", opts.DefaultBranch)
	}

	return nil
}

// IsInitialized returns true if the repository is already initialized.
func (r *gitRepo) IsInitialized(ctx context.Context) bool {
	_, _, err := r.runGit(ctx, "rev-parse", "--git-dir")
	return err == nil
}
