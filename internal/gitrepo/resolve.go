package gitrepo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ResolutionChoice represents how a conflict should be resolved.
type ResolutionChoice int

const (
	// Unresolved means the conflict has not been resolved yet.
	Unresolved ResolutionChoice = iota
	// ChooseOurs accepts our version of the file.
	ChooseOurs
	// ChooseTheirs accepts their version of the file.
	ChooseTheirs
	// ManualEdit means the user will manually edit the file.
	ManualEdit
)

// MergeState represents the current merge/rebase state.
type MergeState struct {
	// InMerge is true if a merge is in progress.
	InMerge bool
	// InRebase is true if a rebase is in progress.
	InRebase bool
	// ConflictingFiles is a list of files with conflicts.
	ConflictingFiles []string
}

// GetMergeState returns the current merge/rebase state.
func (r *gitRepo) GetMergeState(ctx context.Context) (MergeState, error) {
	state := MergeState{}

	// Check for merge in progress
	_, mergeOutput, err := r.runGit(ctx, "status", "--porcelain")
	if err == nil {
		if strings.Contains(mergeOutput, "Unmerged paths") ||
			strings.Contains(mergeOutput, "both modified") {
			state.InMerge = true
		}
	}

	// Check for rebase in progress
	rebaseDir := r.path + "/.git/rebase-apply"
	if _, err := os.Stat(rebaseDir); err == nil {
		state.InRebase = true
	}
	rebaseMergeDir := r.path + "/.git/rebase-merge"
	if _, err := os.Stat(rebaseMergeDir); err == nil {
		state.InRebase = true
	}

	// Get conflicting files
	if state.InMerge || state.InRebase {
		conflicts, err := r.GetConflictFiles(ctx)
		if err == nil {
			state.ConflictingFiles = conflicts
		}
	}

	return state, nil
}

// ResolveFile resolves a conflict in the specified file using the given choice.
func (r *gitRepo) ResolveFile(ctx context.Context, path string, choice ResolutionChoice) error {
	switch choice {
	case ChooseOurs:
		// git checkout --ours <path>
		_, _, err := r.runGit(ctx, "checkout", "--ours", path)
		if err != nil {
			return fmt.Errorf("failed to checkout our version: %w", err)
		}
	case ChooseTheirs:
		// git checkout --theirs <path>
		_, _, err := r.runGit(ctx, "checkout", "--theirs", path)
		if err != nil {
			return fmt.Errorf("failed to checkout their version: %w", err)
		}
	case ManualEdit:
		// Open editor for manual editing
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}

		// Build the full file path
		filePath := r.path
		if !strings.HasPrefix(path, "/") {
			filePath = r.path + "/" + path
		}

		// Open editor in blocking mode
		cmd := exec.Command(editor, filePath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("editor failed: %w", err)
		}

		// Verify no conflict markers remain
		hasMarkers, err := r.hasConflictMarkers(ctx, path)
		if err != nil {
			return fmt.Errorf("failed to check for conflict markers: %w", err)
		}
		if hasMarkers {
			return fmt.Errorf("file still contains conflict markers after manual edit")
		}
	case Unresolved:
		return fmt.Errorf("cannot resolve file with Unresolved choice")
	default:
		return fmt.Errorf("unknown resolution choice: %v", choice)
	}

	// Stage the resolved file
	_, _, err := r.runGit(ctx, "add", path)
	if err != nil {
		return fmt.Errorf("failed to stage resolved file: %w", err)
	}

	return nil
}

// hasConflictMarkers checks if a file still contains git conflict markers.
func (r *gitRepo) hasConflictMarkers(ctx context.Context, path string) (bool, error) {
	_, output, err := r.runGit(ctx, "diff", "--check", path)
	if err != nil {
		// git diff --check returns conflict markers as errors
		if strings.Contains(output, "<<<<<<< ") ||
			strings.Contains(output, ">>>>>>> ") {
			return true, nil
		}
	}
	return false, nil
}

// AbortMerge aborts the current merge operation.
func (r *gitRepo) AbortMerge(ctx context.Context) error {
	_, _, err := r.runGit(ctx, "merge", "--abort")
	if err != nil {
		return fmt.Errorf("failed to abort merge: %w", err)
	}
	return nil
}

// AbortRebase aborts the current rebase operation.
func (r *gitRepo) AbortRebase(ctx context.Context) error {
	_, _, err := r.runGit(ctx, "rebase", "--abort")
	if err != nil {
		return fmt.Errorf("failed to abort rebase: %w", err)
	}
	return nil
}

// ContinueMerge continues the current merge operation after resolving conflicts.
func (r *gitRepo) ContinueMerge(ctx context.Context) error {
	// Check if there are still conflicts
	hasConflicts, err := r.HasConflicts(ctx)
	if err != nil {
		return fmt.Errorf("failed to check for conflicts: %w", err)
	}
	if hasConflicts {
		return fmt.Errorf("cannot continue: unresolved conflicts remain")
	}

	// Commit the merge
	_, output, err := r.runGit(ctx, "commit", "--no-edit")
	if err != nil {
		return fmt.Errorf("failed to commit merge: %w", err)
	}
	_ = output
	return nil
}

// ContinueRebase continues the current rebase operation after resolving conflicts.
func (r *gitRepo) ContinueRebase(ctx context.Context) error {
	// Check if there are still conflicts
	hasConflicts, err := r.HasConflicts(ctx)
	if err != nil {
		return fmt.Errorf("failed to check for conflicts: %w", err)
	}
	if hasConflicts {
		return fmt.Errorf("cannot continue: unresolved conflicts remain")
	}

	// Continue the rebase
	_, output, err := r.runGit(ctx, "rebase", "--continue")
	if err != nil {
		return fmt.Errorf("failed to continue rebase: %w", err)
	}
	_ = output
	return nil
}

// IsFileResolved returns true if the file no longer has conflicts.
func (r *gitRepo) IsFileResolved(ctx context.Context, path string) (bool, error) {
	// Check if file is in the list of unmerged files
	conflicts, err := r.GetConflictFiles(ctx)
	if err != nil {
		return false, err
	}

	for _, conflict := range conflicts {
		if conflict == path {
			return false, nil
		}
	}

	return true, nil
}
