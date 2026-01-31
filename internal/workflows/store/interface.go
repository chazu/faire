package store

import (
	"context"

	"github.com/chazuruo/svf/internal/workflows"
)

// Store defines the interface for workflow persistence operations.
type Store interface {
	// List returns workflow references matching the given filter.
	// If filter is empty, returns all workflows.
	List(ctx context.Context, filter Filter) ([]WorkflowRef, error)

	// Load reads a workflow from the store by its reference.
	Load(ctx context.Context, ref WorkflowRef) (*workflows.Workflow, error)

	// Save writes a workflow to the store.
	// Returns the reference to the saved workflow.
	Save(ctx context.Context, wf *workflows.Workflow, opts SaveOptions) (WorkflowRef, error)

	// Delete removes a workflow from the store.
	Delete(ctx context.Context, ref WorkflowRef) error
}

// SaveOptions contains options for saving a workflow.
type SaveOptions struct {
	// Commit creates a git commit after saving if true.
	Commit bool

	// Message is the commit message to use (defaults to auto-generated).
	Message string

	// Force allows overwriting an existing workflow if true.
	Force bool
}
