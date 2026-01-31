// Package runner provides placeholder parsing and substitution.
package runner

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// placeholderRegex matches <parameter> tokens.
	placeholderRegex = regexp.MustCompile(`<([a-zA-Z_][a-zA-Z0-9_-]*)>`)
)

// ExtractPlaceholders extracts all unique placeholders from a string.
func ExtractPlaceholders(s string) []string {
	matches := placeholderRegex.FindAllStringSubmatch(s, -1)
	seen := make(map[string]bool)
	var result []string

	for _, m := range matches {
		if len(m) > 1 {
			name := m[1]
			if !seen[name] {
				seen[name] = true
				result = append(result, name)
			}
		}
	}

	return result
}

// Substitute replaces placeholders with values.
func Substitute(s string, values map[string]string) (string, error) {
	result := s
	var missing []string

	// Find all placeholders
	matches := placeholderRegex.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		if len(m) > 1 {
			name := m[1]
			placeholder := m[0]
			value, ok := values[name]
			if !ok {
				missing = append(missing, name)
				continue
			}
			result = strings.ReplaceAll(result, placeholder, value)
		}
	}

	if len(missing) > 0 {
		return "", fmt.Errorf("missing placeholders: %s", strings.Join(missing, ", "))
	}

	return result, nil
}

// ExtractFromWorkflow extracts all placeholders from a workflow.
// This is a placeholder that will be implemented when needed.
func ExtractFromWorkflow(workflow interface{}) map[string]string {
	// TODO: Implement extraction from workflow
	// For now, return empty map
	return make(map[string]string)
}
