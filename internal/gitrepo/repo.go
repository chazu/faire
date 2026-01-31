// Package gitrepo provides a Git repository abstraction.
// It shells out to the git binary for operations, making it
// a lightweight wrapper around standard Git functionality.
//
// NOTE: This is a minimal stub to unblock the Store implementation.
// The full implementation will be completed by fa-t1n and fa-189.
package gitrepo

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// gitRepo represents a Git repository.
type gitRepo struct {
	path     string
	gitDir   string
	workTree string
}

// Repo is the interface for Git repository operations.
type Repo interface {
	// Path returns the repository path.
	Path() string

	// Init initializes a new Git repository with the given options.
	Init(ctx context.Context, opts InitOptions) error

	// Status returns the current status of the repository.
	Status(ctx context.Context) (Status, error)

	// IsInitialized returns true if the repository is already initialized.
	IsInitialized(ctx context.Context) bool

	// Close releases any resources held by the repository.
	Close() error

	// Add stages a specific file for commit.
	Add(ctx context.Context, path string) error

	// AddAll stages all changes for commit.
	AddAll(ctx context.Context) error

	// CommitAll commits all staged changes with the given message.
	CommitAll(ctx context.Context, message string) (hash string, err error)

	// GetCurrentBranch returns the current branch name.
	GetCurrentBranch(ctx context.Context) (string, error)

	// GetConfig reads a git config value.
	GetConfig(ctx context.Context, key string) (string, error)
}

// InitOptions contains options for initializing a repository.
type InitOptions struct {
	// DefaultBranch is the branch name to use (default: "main" or "master").
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
	// Ahead is the number of commits ahead of upstream.
	Ahead int
	// Behind is the number of commits behind upstream.
	Behind int
	// Entries contains detailed status entries for each changed file.
	Entries []StatusEntry
}

// StatusEntry represents a single file's status.
type StatusEntry struct {
	// Path is the file path.
	Path string
	// X is the first status character (see git status --porcelain documentation).
	X byte
	// Y is the second status character.
	Y byte
}

// GitError wraps an error from a Git command.
type GitError struct {
	// Args is the arguments passed to the Git command.
	Args []string
	// Err is the underlying error.
	Err error
	// ExitCode is the exit code from the Git command.
	ExitCode int
}

// Error returns the error message.
func (e *GitError) Error() string {
	return fmt.Sprintf("git %s: %s", strings.Join(e.Args, " "), e.Err.Error())
}

// Unwrap returns the underlying error.
func (e *GitError) Unwrap() error {
	return e.Err
}

// New creates a new Repo instance for the given path.
func New(path string) Repo {
	return &gitRepo{
		path:     path,
		gitDir:   "",
		workTree: "",
	}
}

// Path returns the repository path.
func (r *gitRepo) Path() string {
	return r.path
}

// Close releases any resources held by the repository.
func (r *gitRepo) Close() error {
	return nil
}

// runGit executes a git command with the given arguments.
func (r *gitRepo) runGit(ctx context.Context, args ...string) (*exec.Cmd, string, error) {
	cmdArgs := []string{}
	if r.gitDir != "" {
		cmdArgs = append(cmdArgs, "--git-dir="+r.gitDir)
	}
	if r.workTree != "" {
		cmdArgs = append(cmdArgs, "--work-tree="+r.workTree)
	}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Dir = r.path

	output, err := cmd.CombinedOutput()
	if err != nil {
		var exitCode int
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		}
		return cmd, "", &GitError{
			Args:     cmdArgs,
			Err:      fmt.Errorf("%w: %s", err, string(output)),
			ExitCode: exitCode,
		}
	}

	return cmd, string(output), nil
}

// Init, IsInitialized, and Status methods are in init.go and status.go.

// Add stages a specific file for commit.
func (r *gitRepo) Add(ctx context.Context, path string) error {
	_, _, err := r.runGit(ctx, "add", path)
	return err
}

// AddAll stages all changes for commit.
func (r *gitRepo) AddAll(ctx context.Context) error {
	_, _, err := r.runGit(ctx, "add", "-A")
	return err
}

// CommitAll commits all staged changes.
func (r *gitRepo) CommitAll(ctx context.Context, message string) (string, error) {
	_, output, err := r.runGit(ctx, "commit", "-m", message)
	if err != nil {
		return "", err
	}

	// Parse commit hash from output
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "[") {
			// Format: [branch-name hash] message
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				// Extract hash from [branch hash]
				trimmed := strings.Trim(parts[0], "[]")
				hashParts := strings.Fields(trimmed)
				if len(hashParts) >= 2 {
					return hashParts[1], nil
				}
			}
		}
	}

	// Fallback: get the HEAD hash
	_, hashOutput, err := r.runGit(ctx, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(hashOutput), nil
}

// GetCurrentBranch returns the current branch name.
func (r *gitRepo) GetCurrentBranch(ctx context.Context) (string, error) {
	_, output, err := r.runGit(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// GetConfig reads a git config value.
func (r *gitRepo) GetConfig(ctx context.Context, key string) (string, error) {
	_, output, err := r.runGit(ctx, "config", "--get", key)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}
