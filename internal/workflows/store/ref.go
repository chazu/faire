package store

import "time"

// WorkflowRef is a lightweight reference to a workflow.
type WorkflowRef struct {
	// ID is the unique identifier for the workflow (ULID).
	ID string

	// Slug is the URL-friendly identifier.
	Slug string

	// Path is the full path to the workflow.yaml file.
	Path string

	// UpdatedAt is the last modification time.
	UpdatedAt time.Time
}

// Filter defines criteria for filtering workflows.
type Filter struct {
	// IdentityPath filters workflows by identity path (e.g., "platform/chaz").
	IdentityPath string

	// Tags filters workflows by tags (all must match).
	Tags []string

	// Search performs a text search across title and description.
	Search string
}
