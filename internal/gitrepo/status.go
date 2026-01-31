package gitrepo

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Status returns the current status of the repository.
func (r *gitRepo) Status(ctx context.Context) (Status, error) {
	result := Status{
		Branch: "",
		Dirty:  false,
		Ahead:  0,
		Behind: 0,
		Entries: []StatusEntry{},
	}

	// Get branch name and ahead/behind counts
	if err := r.getBranchStatus(ctx, &result); err != nil {
		return Status{}, err
	}

	// Get porcelain status for file changes
	if err := r.getFileStatus(ctx, &result); err != nil {
		return Status{}, err
	}

	return result, nil
}

// getBranchStatus populates branch name and ahead/behind counts.
func (r *gitRepo) getBranchStatus(ctx context.Context, status *Status) error {
	_, output, err := r.runGit(ctx, "status", "--branch", "--porcelain")
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return nil
	}

	// First line contains branch info: ## branch-name...origin/branch-name [ahead N, behind M]
	// Or: ## No commits yet on branch-name
	firstLine := strings.TrimPrefix(lines[0], "## ")
	if firstLine == "" {
		return nil
	}

	// Handle "No commits yet on branch-name" format
	if strings.HasPrefix(firstLine, "No commits yet on ") {
		status.Branch = strings.TrimPrefix(firstLine, "No commits yet on ")
		// Remove any trailing status like [ahead N]
		if idx := strings.Index(status.Branch, " ["); idx > 0 {
			status.Branch = status.Branch[:idx]
		}
		return nil
	}

	// Handle normal format: "branch-name...origin/branch [ahead N, behind M]"
	parts := strings.Fields(firstLine)
	if len(parts) > 0 {
		// Branch name is before any "...", "]", or whitespace
		branchName := parts[0]
		if idx := strings.Index(branchName, "..."); idx > 0 {
			branchName = branchName[:idx]
		}
		status.Branch = branchName
	}

	// Parse ahead/behind counts
	statusStr := strings.Join(parts, " ")
	if strings.Contains(statusStr, "[ahead ") {
		if idx := strings.Index(statusStr, "[ahead "); idx >= 0 {
			remaining := statusStr[idx+7:]
			numStr := ""
			for _, r := range remaining {
				if r >= '0' && r <= '9' {
					numStr += string(r)
				} else {
					break
				}
			}
			if numStr != "" {
				status.Ahead, _ = strconv.Atoi(numStr)
			}
		}
	}

	if strings.Contains(statusStr, "[behind ") {
		if idx := strings.Index(statusStr, "[behind "); idx >= 0 {
			remaining := statusStr[idx+8:]
			numStr := ""
			for _, r := range remaining {
				if r >= '0' && r <= '9' {
					numStr += string(r)
				} else {
					break
				}
			}
			if numStr != "" {
				status.Behind, _ = strconv.Atoi(numStr)
			}
		}
	}

	return nil
}

// getFileStatus populates file status entries.
func (r *gitRepo) getFileStatus(ctx context.Context, status *Status) error {
	_, output, err := r.runGit(ctx, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("failed to get porcelain status: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Porcelain format: XY filename
		// X = staging area status, Y = worktree status
		if len(line) < 3 {
			continue
		}

		entry := StatusEntry{
			X: line[0],
			Y: line[1],
		}

		// Path starts after the 2-character status code
		// Handle renamed files: "R  old -> new"
		if len(line) > 3 {
			entry.Path = strings.TrimSpace(line[2:])
		}

		status.Entries = append(status.Entries, entry)

		// Mark as dirty if there's any status other than "  " (clean)
		if entry.X != ' ' || entry.Y != ' ' {
			status.Dirty = true
		}
	}

	return nil
}
