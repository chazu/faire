// Package placeholders provides placeholder parsing, validation, and substitution.
package placeholders

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// placeholderRegex matches <parameter> tokens in commands.
	placeholderRegex = regexp.MustCompile(`<([a-zA-Z_][a-zA-Z0-9_-]*)>`)
)

// Extract extracts all unique placeholders from a string.
func Extract(s string) []string {
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

// ExtractFromWorkflow extracts all unique placeholders from all workflow steps.
func ExtractFromWorkflow(steps []Step) []string {
	seen := make(map[string]bool)
	var result []string

	for _, step := range steps {
		for _, name := range Extract(step.Command) {
			if !seen[name] {
				seen[name] = true
				result = append(result, name)
			}
		}
	}

	return result
}

// Substitute replaces placeholders with values.
// Returns an error if any placeholders are missing.
func Substitute(s string, values map[string]string) (string, error) {
	result := s
	var missing []string

	// Find all unique placeholders
	matches := placeholderRegex.FindAllStringSubmatch(s, -1)
	seen := make(map[string]bool)

	for _, m := range matches {
		if len(m) > 1 {
			name := m[1]
			placeholder := m[0]

			// Skip if we've already processed this placeholder
			if seen[name] {
				continue
			}
			seen[name] = true

			value, ok := values[name]
			if !ok {
				missing = append(missing, name)
				continue
			}
			result = strings.ReplaceAll(result, placeholder, value)
		}
	}

	if len(missing) > 0 {
		return "", &MissingError{MissingNames: missing}
	}

	return result, nil
}

// SubstituteAll substitutes placeholders across all workflow steps.
func SubstituteAll(steps []Step, values map[string]string) ([]Step, error) {
	result := make([]Step, len(steps))

	for i, step := range steps {
		newCommand, err := Substitute(step.Command, values)
		if err != nil {
			return nil, fmt.Errorf("step %d (%s): %w", i, step.Name, err)
		}
		result[i] = step
		result[i].Command = newCommand
	}

	return result, nil
}

// MissingError is returned when placeholders are missing values.
type MissingError struct {
	MissingNames []string
}

func (e *MissingError) Error() string {
	return fmt.Sprintf("missing placeholders: %s", strings.Join(e.MissingNames, ", "))
}

// Missing returns the list of missing placeholder names.
func (e *MissingError) Missing() []string {
	return e.MissingNames
}

// Validate validates a placeholder value against a regex pattern.
func Validate(value, pattern string) error {
	if pattern == "" {
		return nil // No validation
	}

	matched, err := regexp.MatchString(pattern, value)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}

	if !matched {
		return fmt.Errorf("value does not match pattern %s", pattern)
	}

	return nil
}

// Step represents a step with a command that may contain placeholders.
type Step struct {
	Name    string
	Command string
}
