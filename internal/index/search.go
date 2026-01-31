package index

import (
	"strings"
)

// SearchOptions contains options for searching the index.
type SearchOptions struct {
	Query        string   // Text query (substring search)
	IdentityPath string   // Filter by identity path
	Tags         []string // Filter by tags (all must match)
	Limit        int      // Maximum results (0 = unlimited)
}

// DefaultLimit is the default limit for search results.
const DefaultLimit = 50

// Search searches the index and returns matching entries.
func Search(idx *Index, opts SearchOptions) []Entry {
	if idx == nil {
		return nil
	}

	if opts.Limit <= 0 {
		opts.Limit = DefaultLimit
	}

	var results []Entry
	query := strings.ToLower(opts.Query)

	for _, entry := range idx.Workflows {
		// Filter by identity path
		if opts.IdentityPath != "" && entry.IdentityPath != opts.IdentityPath {
			continue
		}

		// Filter by tags (all must match)
		if len(opts.Tags) > 0 && !hasAllTags(entry.Tags, opts.Tags) {
			continue
		}

		// Filter by query (substring search)
		if opts.Query != "" && !matchesQuery(entry, query) {
			continue
		}

		results = append(results, entry)

		if len(results) >= opts.Limit {
			break
		}
	}

	return results
}

// hasAllTags checks if the entry has all the required tags.
func hasAllTags(entryTags, requiredTags []string) bool {
	tagMap := make(map[string]bool)
	for _, tag := range entryTags {
		tagMap[strings.ToLower(tag)] = true
	}

	for _, tag := range requiredTags {
		if !tagMap[strings.ToLower(tag)] {
			return false
		}
	}

	return true
}

// matchesQuery checks if the entry matches the query string.
func matchesQuery(entry Entry, query string) bool {
	// Search in title
	if strings.Contains(strings.ToLower(entry.Title), query) {
		return true
	}

	// Search in slug
	if strings.Contains(strings.ToLower(entry.Slug), query) {
		return true
	}

	// Search in pre-built search text
	if strings.Contains(entry.SearchText, query) {
		return true
	}

	return false
}
