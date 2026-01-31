// Package gitrepo provides a Git repository abstraction.
package gitrepo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// CloneOptions contains options for cloning a repository.
type CloneOptions struct {
	// Remote is the Git remote URL to clone from.
	Remote string

	// Path is the local path to clone to.
	Path string

	// Branch is the branch to checkout (optional).
	Branch string

	// Depth creates a shallow clone with the given depth (0 = full clone).
	Depth int

	// SingleBranch clones only a single branch.
	SingleBranch bool
}

// Clone clones a Git repository from remote to local path.
func Clone(remote, path, branch string) error {
	// Create parent directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	args := []string{"clone"}

	if branch != "" {
		args = append(args, "--branch", branch)
	}

	args = append(args, remote, path)

	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	return nil
}

// CloneWithResult clones a repository and returns a Repo instance.
func CloneWithResult(ctx context.Context, opts CloneOptions) (Repo, error) {
	if err := Clone(opts.Remote, opts.Path, opts.Branch); err != nil {
		return nil, err
	}

	return New(opts.Path), nil
}
