package gitrepo

import (
	"context"
	"errors"
	"io/fs"
	"os"
)

// Repo represents a Git repository.
type Repo interface {
	// Path returns the filesystem path to the repository.
	Path() string

	// Init initializes a new Git repository.
	Init(ctx context.Context, opts InitOptions) error

	// Status returns the current status of the repository.
	Status(ctx context.Context) (Status, error)

	// IsInitialized returns true if the repository is already initialized.
	IsInitialized(ctx context.Context) bool

	// Close releases any resources held by the repository.
	Close() error
}

// InitOptions contains options for initializing a repository.
type InitOptions struct {
	// DefaultBranch is the name of the default branch (e.g., "main", "master").
	// If empty, Git's default will be used.
	DefaultBranch string

	// Bare creates a bare repository if true.
	Bare bool
}

// Status represents the status of a Git repository.
type Status struct {
	// Branch is the current branch name.
	Branch string

	// Dirty is true if there are uncommitted changes.
	Dirty bool

	// Ahead is the number of commits ahead of the remote branch.
	Ahead int

	// Behind is the number of commits behind the remote branch.
	Behind int

	// Entries contains individual file status entries.
	Entries []StatusEntry
}

// StatusEntry represents the status of a single file.
type StatusEntry struct {
	// Path is the file path relative to the repo root.
	Path string

	// Status is the Git status code (e.g., "M", "A", "D", "??").
	// See git-status(1) for the full list.
	Status string

	// Staged is true if the change is staged.
	Staged bool
}

// GitError represents an error from a Git command.
type GitError struct {
	// Command is the Git command that failed (e.g., "status", "init").
	Command string

	// Args are the arguments passed to the command.
	Args []string

	// Err is the underlying error.
	Err error

	// ExitCode is the exit code from Git, if available.
	ExitCode int
}

// Error returns the error message.
func (e *GitError) Error() string {
	return "git " + e.Command + ": " + e.Err.Error()
}

// Unwrap returns the underlying error.
func (e *GitError) Unwrap() error {
	return e.Err
}

// Is returns true if the target error is a GitError.
func (e *GitError) Is(target error) bool {
	_, ok := target.(*GitError)
	return ok
}

// IsNotExist returns true if the error indicates a path does not exist.
func IsNotExist(err error) bool {
	if err == nil {
		return false
	}
	// Check for os.ErrNotExist directly
	if err == fs.ErrNotExist || os.IsNotExist(err) {
		return true
	}
	// Check for GitError wrapping a not-exist error
	var gitErr *GitError
	if errors.As(err, &gitErr) {
		return os.IsNotExist(gitErr.Err)
	}
	return false
}
