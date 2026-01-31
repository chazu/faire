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
	"strconv"
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

	// HasConflicts returns true if there are unresolved merge/rebase conflicts.
	HasConflicts(ctx context.Context) (bool, error)

	// GetConflicts returns the list of files with unresolved conflicts.
	GetConflicts(ctx context.Context) ([]string, error)

	// Fetch fetches updates from a remote repository.
	Fetch(ctx context.Context, remote string) error

	// Integrate integrates remote changes using the specified strategy.
	Integrate(ctx context.Context, opts IntegrateOptions) (IntegrationResult, error)
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
	// Conflicted is true if there are merge/rebase conflicts.
	Conflicted bool
	// Conflicts contains the list of conflicted files.
	Conflicts []string
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

// IntegrationStrategy defines how to integrate remote changes.
type IntegrationStrategy string

const (
	// FastForwardOnly only fast-forwards, fails if not possible.
	FastForwardOnly IntegrationStrategy = "ff-only"
	// Rebase rebases local commits on top of remote.
	Rebase IntegrationStrategy = "rebase"
	// Merge merges remote changes into local branch.
	Merge IntegrationStrategy = "merge"
)

// IntegrateOptions contains options for integrating remote changes.
type IntegrateOptions struct {
	// Strategy is the integration strategy to use.
	Strategy IntegrationStrategy
	// Remote is the remote name (e.g., "origin").
	Remote string
	// Branch is the remote branch name (e.g., "main").
	Branch string
	// NoFetch skips the fetch step (assumes fetch was already done).
	NoFetch bool
}

// IntegrationResult contains the result of an integration operation.
type IntegrationResult struct {
	// Strategy is the strategy that was used.
	Strategy IntegrationStrategy
	// Success is true if the integration completed without conflicts.
	Success bool
	// Conflicts contains the list of conflicting files (if any).
	Conflicts []string
	// NewFiles contains the list of newly added files.
	NewFiles []string
	// UpdatedFiles contains the list of modified files.
	UpdatedFiles []string
	// DeletedFiles contains the list of removed files.
	DeletedFiles []string
	// CommitHash is the HEAD commit hash after integration.
	CommitHash string
	// Error contains any error that occurred.
	Error error
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

// ConflictError represents a merge/rebase conflict error.
type ConflictError struct {
	// Files is the list of files with conflicts.
	Files []string
}

// Error returns the error message.
func (e *ConflictError) Error() string {
	if len(e.Files) == 0 {
		return "merge conflicts detected"
	}
	if len(e.Files) == 1 {
		return fmt.Sprintf("merge conflict in %s", e.Files[0])
	}
	return fmt.Sprintf("merge conflicts in %d files: %s", len(e.Files), strings.Join(e.Files, ", "))
}

// IsConflictError returns true if the error is a ConflictError.
func IsConflictError(err error) bool {
	_, ok := err.(*ConflictError)
	return ok
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

// HasConflicts returns true if there are unresolved merge/rebase conflicts.
func (r *gitRepo) HasConflicts(ctx context.Context) (bool, error) {
	conflicts, err := r.GetConflicts(ctx)
	if err != nil {
		return false, err
	}
	return len(conflicts) > 0, nil
}

// GetConflicts returns the list of files with unresolved conflicts.
func (r *gitRepo) GetConflicts(ctx context.Context) ([]string, error) {
	_, output, err := r.runGit(ctx, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}

	output = strings.TrimSpace(output)
	if output == "" {
		return []string{}, nil
	}

	return strings.Split(output, "\n"), nil
}

// Fetch fetches updates from a remote repository.
func (r *gitRepo) Fetch(ctx context.Context, remote string) error {
	_, _, err := r.runGit(ctx, "fetch", remote)
	return err
}

// Integrate integrates remote changes using the specified strategy.
func (r *gitRepo) Integrate(ctx context.Context, opts IntegrateOptions) (IntegrationResult, error) {
	result := IntegrationResult{
		Strategy: opts.Strategy,
	}

	// Fetch if not skipped
	if !opts.NoFetch {
		if err := r.Fetch(ctx, opts.Remote); err != nil {
			result.Error = fmt.Errorf("fetch failed: %w", err)
			return result, result.Error
		}
	}

	// Get the current branch
	currentBranch, err := r.GetCurrentBranch(ctx)
	if err != nil {
		result.Error = fmt.Errorf("get current branch failed: %w", err)
		return result, result.Error
	}

	// Get remote tracking branch
	remoteBranch := opts.Remote + "/" + currentBranch
	if opts.Branch != "" {
		remoteBranch = opts.Remote + "/" + opts.Branch
	}

	// Check for conflicts first
	hasConflicts, conflictFiles := r.checkForConflicts(ctx, remoteBranch)
	if hasConflicts {
		result.Success = false
		result.Conflicts = conflictFiles
		result.Error = &ConflictError{Files: conflictFiles}
		return result, result.Error
	}

	// Execute the integration strategy
	switch opts.Strategy {
	case FastForwardOnly:
		return r.fastForward(ctx, remoteBranch, result)
	case Rebase:
		return r.rebase(ctx, remoteBranch, result)
	case Merge:
		return r.merge(ctx, remoteBranch, result)
	default:
		result.Error = fmt.Errorf("unknown integration strategy: %s", opts.Strategy)
		return result, result.Error
	}
}

// checkForConflicts checks if integrating the remote branch would cause conflicts.
func (r *gitRepo) checkForConflicts(ctx context.Context, remoteBranch string) (bool, []string) {
	// Try to merge in dry-run mode to detect conflicts
	_, output, err := r.runGit(ctx, "merge", "--no-commit", "--no-ff", remoteBranch)
	if err != nil {
		// Abort the merge
		_, _, _ = r.runGit(ctx, "merge", "--abort")

		// Check if it's a conflict error
		if strings.Contains(output, "CONFLICT") || strings.Contains(output, "Automatic merge failed") {
			return r.parseConflictFiles(output), nil
		}
		// Other errors (e.g., divergent branches) are not conflicts
		return false, nil
	}

	// No conflicts, abort the merge
	_, _, _ = r.runGit(ctx, "merge", "--abort")
	return false, nil
}

// parseConflictFiles extracts conflict file paths from git merge output.
func (r *gitRepo) parseConflictFiles(output string) []string {
	var conflicts []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "CONFLICT (content): Merge conflict in ") {
			file := strings.TrimPrefix(line, "CONFLICT (content): Merge conflict in ")
			conflicts = append(conflicts, file)
		} else if strings.HasPrefix(line, "CONFLICT (rename/delete)") {
			// Extract the file name from rename/delete conflicts
			parts := strings.Fields(line)
			if len(parts) >= 5 {
				conflicts = append(conflicts, parts[len(parts)-1])
			}
		}
	}
	return conflicts
}

// fastForward attempts to fast-forward the branch to the remote.
func (r *gitRepo) fastForward(ctx context.Context, remoteBranch string, result IntegrationResult) (IntegrationResult, error) {
	// Try to merge with ff-only
	_, output, err := r.runGit(ctx, "merge", "--ff-only", remoteBranch)
	if err != nil {
		result.Error = fmt.Errorf("fast-forward failed: %w", err)
		return result, result.Error
	}

	// Get changes since before the merge
	result.Success = true
	result.NewFiles, result.UpdatedFiles, result.DeletedFiles = r.parseMergeChanges(output)

	// Get current HEAD
	hash, _ := r.runGit(ctx, "rev-parse", "HEAD")
	result.CommitHash = strings.TrimSpace(hash)

	return result, nil
}

// rebase rebases local commits on top of the remote branch.
func (r *gitRepo) rebase(ctx context.Context, remoteBranch string, result IntegrationResult) (IntegrationResult, error) {
	_, output, err := r.runGit(ctx, "rebase", remoteBranch)
	if err != nil {
		// Check if it's a conflict error
		if strings.Contains(output, "CONFLICT") || strings.Contains(output, "could not apply") {
			// Abort rebase on conflict
			_, _, _ = r.runGit(ctx, "rebase", "--abort")
			result.Success = false
			result.Conflicts = r.parseConflictFiles(output)
			result.Error = &ConflictError{Files: result.Conflicts}
			return result, result.Error
		}
		result.Error = fmt.Errorf("rebase failed: %w", err)
		return result, result.Error
	}

	// Get changes
	result.Success = true
	result.NewFiles, result.UpdatedFiles, result.DeletedFiles = r.parseRebaseChanges(output)

	// Get current HEAD
	hash, _ := r.runGit(ctx, "rev-parse", "HEAD")
	result.CommitHash = strings.TrimSpace(hash)

	return result, nil
}

// merge merges the remote branch into the current branch.
func (r *gitRepo) merge(ctx context.Context, remoteBranch string, result IntegrationResult) (IntegrationResult, error) {
	_, output, err := r.runGit(ctx, "merge", remoteBranch)
	if err != nil {
		// Check if it's a conflict error
		if strings.Contains(output, "CONFLICT") || strings.Contains(output, "Automatic merge failed") {
			// Abort merge on conflict
			_, _, _ = r.runGit(ctx, "merge", "--abort")
			result.Success = false
			result.Conflicts = r.parseConflictFiles(output)
			result.Error = &ConflictError{Files: result.Conflicts}
			return result, result.Error
		}
		result.Error = fmt.Errorf("merge failed: %w", err)
		return result, result.Error
	}

	// Get changes
	result.Success = true
	result.NewFiles, result.UpdatedFiles, result.DeletedFiles = r.parseMergeChanges(output)

	// Get current HEAD
	hash, _ := r.runGit(ctx, "rev-parse", "HEAD")
	result.CommitHash = strings.TrimSpace(hash)

	return result, nil
}

// parseMergeChanges parses git merge output to extract file changes.
func (r *gitRepo) parseMergeChanges(output string) (newFiles, updated, deleted []string) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, " create mode ") {
			// Extract file name
			file := strings.TrimPrefix(line, " create mode ")
			if idx := strings.Index(file, " "); idx > 0 {
				file = file[idx+1:]
			}
			newFiles = append(newFiles, file)
		} else if strings.HasPrefix(line, " update mode ") {
			file := strings.TrimPrefix(line, " update mode ")
			if idx := strings.Index(file, " "); idx > 0 {
				file = file[idx+1:]
			}
			updated = append(updated, file)
		} else if strings.HasPrefix(line, " delete mode ") {
			file := strings.TrimPrefix(line, " delete mode ")
			if idx := strings.Index(file, " "); idx > 0 {
				file = file[idx+1:]
			}
			deleted = append(deleted, file)
		}
	}
	return
}

// parseRebaseChanges parses git rebase output to extract file changes.
func (r *gitRepo) parseRebaseChanges(output string) (newFiles, updated, deleted []string) {
	// Rebase doesn't show file changes in the same way as merge
	// We'd need to compare with the remote branch, which is complex
	// For now, return empty slices - the summary can still be generated
	// by comparing the index before and after
	return nil, nil, nil
}
