package gitrepo

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// gitRepo is the concrete implementation of Repo.
type gitRepo struct {
	path string
}

// Open opens a Git repository at the given path.
// The repository may or may not be initialized yet.
func Open(path string) Repo {
	return &gitRepo{path: path}
}

// Path returns the filesystem path to the repository.
func (r *gitRepo) Path() string {
	return r.path
}

// Init initializes a new Git repository.
func (r *gitRepo) Init(ctx context.Context, opts InitOptions) error {
	args := []string{"init"}
	if opts.Bare {
		args = append(args, "--bare")
	}

	// Run git init
	if err := r.runGitCmd(ctx, args...); err != nil {
		return fmt.Errorf("init failed: %w", err)
	}

	// Set default branch if specified
	if opts.DefaultBranch != "" {
		branchArgs := []string{"branch", "-M", opts.DefaultBranch}
		if err := r.runGitCmd(ctx, branchArgs...); err != nil {
			return fmt.Errorf("set default branch failed: %w", err)
		}
	}

	return nil
}

// IsInitialized returns true if the repository is already initialized.
func (r *gitRepo) IsInitialized(ctx context.Context) bool {
	gitDir := r.path + "/.git"
	// Check if .git directory exists
	cmd := exec.CommandContext(ctx, "test", "-d", gitDir)
	return cmd.Run() == nil
}

// Close releases any resources held by the repository.
func (r *gitRepo) Close() error {
	// No resources to release in this implementation
	return nil
}

// runGitCmd executes a git command in the repository.
func (r *gitRepo) runGitCmd(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.path

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &GitError{
			Command:  args[0],
			Args:     args[1:],
			Err:      fmt.Errorf("%s: %w", strings.TrimSpace(string(output)), err),
			ExitCode: cmd.ProcessState.ExitCode(),
		}
	}

	return nil
}

// InitRepo creates and initializes a new Git repository at the given path.
func InitRepo(ctx context.Context, path string, opts InitOptions) (Repo, error) {
	repo := Open(path)

	if repo.IsInitialized(ctx) {
		return repo, nil
	}

	if err := repo.Init(ctx, opts); err != nil {
		return nil, err
	}

	return repo, nil
}
