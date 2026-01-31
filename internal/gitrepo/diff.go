package gitrepo

import (
	"context"
	"fmt"
	"strings"
)

// DiffType represents the type of diff to generate.
type DiffType int

const (
	// DiffOurs shows our version of the file.
	DiffOurs DiffType = iota
	// DiffTheirs shows their version of the file.
	DiffTheirs
	// DiffCombined shows a combined diff with conflict markers.
	DiffCombined
)

// DiffContent represents the diff content for a file.
type DiffContent struct {
	Path    string
	Content string
	Type    DiffType
}

// GetDiff generates a diff for the specified file and type.
func (r *gitRepo) GetDiff(ctx context.Context, path string, diffType DiffType) (DiffContent, error) {
	var args []string

	switch diffType {
	case DiffOurs:
		args = []string{"diff", "--ours", path}
	case DiffTheirs:
		args = []string{"diff", "--theirs", path}
	case DiffCombined:
		args = []string{"diff", path}
	default:
		return DiffContent{}, fmt.Errorf("unknown diff type: %v", diffType)
	}

	_, output, err := r.runGit(ctx, args...)
	if err != nil {
		return DiffContent{}, fmt.Errorf("failed to get diff: %w", err)
	}

	return DiffContent{
		Path:    path,
		Content: output,
		Type:    diffType,
	}, nil
}

// GetConflictFiles returns a list of files that have merge conflicts.
func (r *gitRepo) GetConflictFiles(ctx context.Context) ([]string, error) {
	// Use git diff --name-only --diff-filter=U to list unmerged files
	_, output, err := r.runGit(ctx, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		// If no conflicts, git returns non-zero, check if output is empty
		if strings.TrimSpace(output) == "" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get conflict files: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	var conflicts []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			conflicts = append(conflicts, line)
		}
	}

	return conflicts, nil
}

// HasConflicts returns true if there are merge conflicts.
func (r *gitRepo) HasConflicts(ctx context.Context) (bool, error) {
	conflicts, err := r.GetConflictFiles(ctx)
	if err != nil {
		return false, err
	}
	return len(conflicts) > 0, nil
}

// ConflictFile represents a file with merge conflict information.
type ConflictFile struct {
	Path   string
	Ours   string
	Theirs string
	Base   string
}

// GetConflictDetails retrieves detailed information about conflicts in a file.
func (r *gitRepo) GetConflictDetails(ctx context.Context, path string) (ConflictFile, error) {
	details := ConflictFile{Path: path}

	// Get our version
	oursContent, err := r.GetFileContent(ctx, path, "--ours")
	if err != nil {
		return details, fmt.Errorf("failed to get our version: %w", err)
	}
	details.Ours = oursContent

	// Get their version
	theirsContent, err := r.GetFileContent(ctx, path, "--theirs")
	if err != nil {
		return details, fmt.Errorf("failed to get their version: %w", err)
	}
	details.Theirs = theirsContent

	// Get base version (common ancestor)
	baseContent, err := r.GetFileContent(ctx, path, "--base")
	if err != nil {
		// Base might not exist in all cases, that's okay
		details.Base = ""
	} else {
		details.Base = baseContent
	}

	return details, nil
}

// GetFileContent returns the file content for a specific stage.
// stage should be "--ours", "--theirs", or "--base".
func (r *gitRepo) GetFileContent(ctx context.Context, path string, stage string) (string, error) {
	// Map stages to their index numbers
	// 1 = common ancestor (base)
	// 2 = ours (current branch)
	// 3 = theirs (incoming branch)
	var stageNum string
	switch stage {
	case "--ours", "-2":
		stageNum = "2"
	case "--theirs", "-3":
		stageNum = "3"
	case "--base", "-1":
		stageNum = "1"
	default:
		stageNum = "2"
	}

	_, output, err := r.runGit(ctx, "show", ":"+stageNum+":"+path)
	if err != nil {
		return "", err
	}
	return output, nil
}
