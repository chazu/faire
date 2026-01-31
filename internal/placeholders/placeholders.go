// Package placeholders provides placeholder parsing, validation, and substitution.
package placeholders

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/chazuruo/svf/internal/workflows"
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

// ExtractWithMetadata extracts all unique placeholders from a workflow and returns
// their metadata including which steps they're used in.
func ExtractWithMetadata(wf *workflows.Workflow) map[string]PlaceholderInfo {
	result := make(map[string]PlaceholderInfo)

	for i, step := range wf.Steps {
		for _, name := range Extract(step.Command) {
			stepName := step.Name
			if stepName == "" {
				stepName = fmt.Sprintf("Step %d", i+1)
			}

			info, exists := result[name]
			if !exists {
				// Get metadata from workflow definition if available
				var ph workflows.Placeholder
				if wf.Placeholders != nil {
					ph = wf.Placeholders[name]
				}

				info = PlaceholderInfo{
					Name:     name,
					Prompt:   ph.Prompt,
					Default:  ph.Default,
					Validate: ph.Validate,
					Secret:   ph.Secret,
					UsedIn:   []string{stepName},
				}
			} else {
				// Add step to UsedIn if not already present
				found := false
				for _, s := range info.UsedIn {
					if s == stepName {
						found = true
						break
					}
				}
				if !found {
					info.UsedIn = append(info.UsedIn, stepName)
				}
			}

			result[name] = info
		}
	}

	return result
}

// PlaceholderInfo contains metadata about a placeholder.
type PlaceholderInfo struct {
	Name     string
	Prompt   string
	Default  string
	Validate string
	Secret   bool
	UsedIn   []string // Step names where this placeholder is used
}

// ExtractFromWorkflow extracts all unique placeholders from all workflow steps.
// Deprecated: Use ExtractWithMetadata instead for full metadata support.
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
// Deprecated: Use workflows.Step instead.
type Step struct {
	Name    string
	Command string
}

// PromptForValues prompts the user for placeholder values interactively.
// It uses the provided metadata to build prompts and validate input.
func PromptForValues(placeholders map[string]PlaceholderInfo, existingValues map[string]string) (map[string]string, error) {
	result := make(map[string]string)

	// Copy existing values
	for k, v := range existingValues {
		result[k] = v
	}

	// Sort placeholder names for consistent ordering
	names := make([]string, 0, len(placeholders))
	for name := range placeholders {
		names = append(names, name)
	}
	sort.Strings(names)

	reader := bufio.NewReader(os.Stdin)

	for _, name := range names {
		info := placeholders[name]

		// Skip if we already have a value
		if _, ok := result[name]; ok {
			continue
		}

		value, err := promptForValue(reader, info)
		if err != nil {
			return nil, fmt.Errorf("failed to prompt for %s: %w", name, err)
		}

		result[name] = value
	}

	return result, nil
}

// promptForValue prompts for a single placeholder value.
func promptForValue(reader *bufio.Reader, info PlaceholderInfo) (string, error) {
	// Build prompt text
	promptText := info.Prompt
	if promptText == "" {
		promptText = fmt.Sprintf("Enter value for <%s>", info.Name)
	}

	// Show usage info
	if len(info.UsedIn) > 0 {
		fmt.Printf("Used in: %s\n", strings.Join(info.UsedIn, ", "))
	}

	// Show default if available
	if info.Default != "" {
		fmt.Printf("%s [%s]: ", promptText, info.Default)
	} else {
		fmt.Printf("%s: ", promptText)
	}

	// Read input
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	value := strings.TrimSpace(line)

	// Use default if empty
	if value == "" && info.Default != "" {
		value = info.Default
	}

	// Validate if pattern is provided
	if info.Validate != "" {
		if err := Validate(value, info.Validate); err != nil {
			fmt.Printf("Validation failed: %v\n", err)
			// Retry once
			fmt.Printf("%s: ", promptText)
			line, err := reader.ReadString('\n')
			if err != nil {
				return "", err
			}
			value = strings.TrimSpace(line)
			if value == "" && info.Default != "" {
				value = info.Default
			}
			if err := Validate(value, info.Validate); err != nil {
				return "", fmt.Errorf("validation failed: %w", err)
			}
		}
	}

	return value, nil
}

// ValidateAtLoadTime validates all placeholders defined in a workflow.
// This checks that regex patterns are valid and other constraints are met.
func ValidateAtLoadTime(wf *workflows.Workflow) error {
	// Extract placeholders with metadata
	info := ExtractWithMetadata(wf)

	// Check for placeholders used in commands but not defined
	for name := range info {
		if wf.Placeholders == nil {
			continue
		}
		if _, exists := wf.Placeholders[name]; !exists {
			// Placeholder is used but not defined - this is OK, just a warning
			// We'll prompt for it at runtime
			_ = exists // Explicitly ignore as we only check for existence
			continue
		}
	}

	// Validate all defined placeholders
	for name, ph := range wf.Placeholders {
		if ph.Validate != "" {
			if _, err := regexp.Compile(ph.Validate); err != nil {
				return fmt.Errorf("placeholder %s: invalid regex pattern %q: %w", name, ph.Validate, err)
			}
		}
	}

	return nil
}

// CollectFromSteps extracts placeholders from a slice of workflow steps.
// This is useful for the runner package which may not have access to the full workflow.
func CollectFromSteps(steps []workflows.Step) []string {
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
