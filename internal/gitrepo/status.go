package gitrepo

import (
	"context"
	"strconv"
	"strings"
)

// Status returns the current status of the repository.
func (r *gitRepo) Status(ctx context.Context) (Status, error) {
	var status Status
	_, output, err := r.runGit(ctx, "status", "--porcelain=v2", "--branch")
	if err != nil {
		return status, err
	}

	// Parse porcelain v2 format:
	// # branch.head <branch>
	// # branch.upstream <upstream>
	// # branch.ab +<ahead> -<behind>
	// 1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>
	// 2 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <X><S> <path>
	// u <XY> <sub> <m1> <m2> <m3> <mW> <h1> <h2> <h3> <path>

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "# branch.head ") {
			status.Branch = strings.TrimPrefix(line, "# branch.head ")
		} else if strings.HasPrefix(line, "# branch.ab ") {
			// Format: # branch.ab +<ahead> -<behind>
			ab := strings.TrimPrefix(line, "# branch.ab ")
			parts := strings.Fields(ab)
			for _, part := range parts {
				if strings.HasPrefix(part, "+") {
					status.Ahead, _ = strconv.Atoi(strings.TrimPrefix(part, "+"))
				} else if strings.HasPrefix(part, "-") {
					status.Behind, _ = strconv.Atoi(strings.TrimPrefix(part, "-"))
				}
			}
		} else if !strings.HasPrefix(line, "#") {
			// This is a file status entry
			// Format 1: 1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>
			// Format 2: 2 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <X><S> <path>
			// Format u:  u <XY> <sub> <m1> <m2> <m3> <mW> <h1> <h2> <h3> <path>
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				entry := StatusEntry{}
				// For ordinary changed entries (1 or 2), XY is in field 1
				// For unmerged entries (u), XY is also in field 1
				xy := fields[1]
				if len(xy) >= 2 {
					entry.X = xy[0]
					entry.Y = xy[1]
				} else if len(xy) == 1 {
					entry.X = xy[0]
					entry.Y = xy[0]
				}

				// Path is the last field
				entry.Path = fields[len(fields)-1]

				status.Entries = append(status.Entries, entry)
				status.Dirty = true

				// Check for conflicts: both X and Y are 'U' (unmerged)
				if entry.X == 'U' && entry.Y == 'U' {
					status.Conflicted = true
					status.Conflicts = append(status.Conflicts, entry.Path)
				}
			}
		}
	}

	return status, nil
}
