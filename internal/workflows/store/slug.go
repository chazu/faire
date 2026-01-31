package store

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	// slugRegex matches characters that should be replaced with hyphens
	slugRegex = regexp.MustCompile(`[^a-z0-9]+`)
	// multiHyphenRegex matches multiple consecutive hyphens
	multiHyphenRegex = regexp.MustCompile(`-+`)
)

// Slugify converts a title into a URL-friendly slug.
// Rules:
// - Lowercase
// - Replace spaces with hyphens
// - Remove special chars (keep a-z, 0-9, hyphen)
// - Collapse multiple hyphens
// - Trim leading/trailing hyphens
// - Max length: 50 chars
//
// Examples:
//   "Restart Service Safely" -> "restart-service-safely"
//   "Fix: Bug #123!" -> "fix-bug-123"
func Slugify(title string) string {
	if title == "" {
		return ""
	}

	// Handle unicode: lowercase first
	caser := cases.Title(language.English)
	result := caser.String(strings.TrimSpace(title))

	// Convert to lowercase
	result = strings.ToLower(result)

	// Replace non-alphanumeric characters with hyphens
	result = slugRegex.ReplaceAllString(result, "-")

	// Collapse multiple hyphens
	result = multiHyphenRegex.ReplaceAllString(result, "-")

	// Trim leading/trailing hyphens
	result = strings.Trim(result, "-")

	// Truncate to max length
	if len(result) > 50 {
		// Find the last hyphen before 50 chars to avoid cutting a word
		cutoff := 50
		if idx := strings.LastIndex(result[:cutoff], "-"); idx > 0 {
			cutoff = idx
		}
		result = result[:cutoff]
	}

	return result
}

// GenerateUniqueSlug generates a slug from a title, ensuring it doesn't
// collide with existing slugs by adding a numeric suffix if needed.
func GenerateUniqueSlug(title string, existingSlugs []string) string {
	slug := Slugify(title)
	if slug == "" {
		return "workflow"
	}

	// Check for collision
	for i := 1; i <= 100; i++ {
		if !contains(existingSlugs, slug) {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", Slugify(title), i)
	}

	// Fallback: use a timestamp suffix (unlikely to hit this)
	return fmt.Sprintf("%s-%d", Slugify(title), time.Now().UnixNano())
}

// contains checks if a string exists in a slice.
func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
