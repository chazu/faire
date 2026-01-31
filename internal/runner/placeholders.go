// Package runner provides placeholder parsing and substitution.
//
// Deprecated: This package is deprecated. Use github.com/chazuruo/svf/internal/placeholders
// instead. This file is kept for backwards compatibility only.
package runner

import (
	"github.com/chazuruo/svf/internal/placeholders"
)

// ExtractPlaceholders extracts all unique placeholders from a string.
//
// Deprecated: Use placeholders.Extract instead.
func ExtractPlaceholders(s string) []string {
	return placeholders.Extract(s)
}

// Substitute replaces placeholders with values.
//
// Deprecated: Use placeholders.Substitute instead.
func Substitute(s string, values map[string]string) (string, error) {
	return placeholders.Substitute(s, values)
}

// ExtractFromWorkflow extracts all placeholders from a workflow.
//
// Deprecated: Use placeholders.ExtractWithMetadata or placeholders.CollectFromSteps instead.
func ExtractFromWorkflow(workflow interface{}) map[string]string {
	// This function is deprecated and should not be used.
	// Use placeholders.ExtractWithMetadata or placeholders.CollectFromSteps instead.
	return make(map[string]string)
}
