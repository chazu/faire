package gitrepo

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Status returns the current status of the repository.
func (r *gitRepo) Status(ctx context.Context) (Status, error) {
	status := Status{}

	// Get current branch
	branch, err := r.getCurrentBranch(ctx)
	if err != nil {
		return status, fmt.Errorf("get branch: %w", err)
	}
	status.Branch = branch

	// Get ahead/behind counts
	ahead, behind, err := r.getAheadBehind(ctx)
	if err != nil {
		// Don't fail on ahead/behind, just use zeros
		ahead, behind = 0, 0
	}
	status.Ahead = ahead
	status.Behind = behind

	// Get file status entries
	entries, dirty, err := r.getStatusEntries(ctx)
	if err != nil {
		return status, fmt.Errorf("get status entries: %w", err)
	}
	status.Entries = entries
	status.Dirty = dirty

	return status, nil
}

// getCurrentBranch returns the name of the current branch.
func (r *gitRepo) getCurrentBranch(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = r.path

	output, err := cmd.Output()
	if err != nil {
		// If git fails, try to get branch from symbolic-ref
		// This handles the case of no commits yet
		cmd2 := exec.CommandContext(ctx, "git", "symbolic-ref", "--short", "HEAD")
		cmd2.Dir = r.path
		output2, err2 := cmd2.Output()
		if err2 != nil {
			// No branch set yet (e.g., no commits)
			return "", nil
		}
		return strings.TrimSpace(string(output2)), nil
	}

	branch := strings.TrimSpace(string(output))
	if branch == "HEAD" {
		// Detached HEAD state or no commits
		return "", nil
	}
	return branch, nil
}

// getAheadBehind returns the number of commits ahead/behind the remote branch.
func (r *gitRepo) getAheadBehind(ctx context.Context) (ahead, behind int, err error) {
	branch, err := r.getCurrentBranch(ctx)
	if err != nil || branch == "" {
		return 0, 0, nil
	}

	// Get the upstream branch for the current branch
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", branch+"@{u}")
	cmd.Dir = r.path

	output, err := cmd.Output()
	if err != nil {
		// No upstream branch configured
		return 0, 0, nil
	}

	// Get ahead/behind counts using git rev-list
	cmd = exec.CommandContext(ctx, "git", "rev-list", "--left-right", "--count", branch+"..."+strings.TrimSpace(string(output)))
	cmd.Dir = r.path

	output, err = cmd.Output()
	if err != nil {
		return 0, 0, nil
	}

	parts := strings.Fields(strings.TrimSpace(string(output)))
	if len(parts) != 2 {
		return 0, 0, nil
	}

	ahead, _ = strconv.Atoi(parts[1])
	behind, _ = strconv.Atoi(parts[0])

	return ahead, behind, nil
}

// getStatusEntries parses git status --porcelain to get file status.
func (r *gitRepo) getStatusEntries(ctx context.Context) ([]StatusEntry, bool, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = r.path

	output, err := cmd.Output()
	if err != nil {
		return nil, false, &GitError{
			Command:  "status",
			Args:     []string{"--porcelain"},
			Err:      err,
			ExitCode: getExitCode(err),
		}
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, false, nil
	}

	entries := make([]StatusEntry, 0, len(lines))
	dirty := false

	for _, line := range lines {
		if line == "" {
			continue
		}

		if len(line) < 3 {
			continue
		}

		entry := StatusEntry{
			Staged: line[0] != ' ' && line[0] != '?',
			Status: strings.TrimRight(line[0:2], " "),
			Path:   line[3:],
		}

		entries = append(entries, entry)
		dirty = true
	}

	return entries, dirty, nil
}

// getExitCode extracts the exit code from an error if possible.
func getExitCode(err error) int {
	if err == nil {
		return 0
	}

	// Try to get exit code from *exec.ExitError
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() >= 0 {
			return exitErr.ExitCode()
		}
	}

	return -1
}
