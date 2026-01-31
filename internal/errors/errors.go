// Package errors provides a structured error type hierarchy for the git-savvy CLI.
//
// This package defines base error types for common error conditions, wrapped error
// types that add contextual information, and helper functions for error wrapping
// and type checking.
//
// # Error Types
//
// Base errors (sentinel errors):
//   - ErrNotFound - resource not found
//   - ErrAlreadyExists - duplicate resource
//   - ErrInvalid - validation failed
//   - ErrConflict - merge/edit conflict
//   - ErrGit - git operation failed
//   - ErrIO - file I/O error
//   - ErrCanceled - user canceled operation
//
// Wrapped error types (add context):
//   - WorkflowError{Op, Err, ID} - workflow operation errors
//   - GitError{Op, Err, Cmd} - git command errors
//   - ConfigError{Path, Err} - configuration errors
//
// # Usage
//
//	// Use sentinel errors directly
//	return errors.ErrNotFound
//
//	// Wrap with context using Wrap
//	return errors.Wrap(err, "readWorkflow")
//
//	// Use structured error types
//	return &errors.WorkflowError{Op: "create", Err: errors.ErrAlreadyExists, ID: "my-workflow"}
//
//	// Check error types
//	if errors.IsNotFound(err) {
//	    // handle not found
//	}
package errors

import (
	"errors"
	"fmt"
)

// Base error types (sentinel errors).
var (
	// ErrNotFound indicates a resource was not found.
	ErrNotFound = baseError("not found")

	// ErrAlreadyExists indicates a duplicate resource.
	ErrAlreadyExists = baseError("already exists")

	// ErrInvalid indicates validation failed.
	ErrInvalid = baseError("invalid")

	// ErrConflict indicates a merge or edit conflict.
	ErrConflict = baseError("conflict")

	// ErrGit indicates a git operation failed.
	ErrGit = baseError("git operation failed")

	// ErrIO indicates a file I/O error.
	ErrIO = baseError("I/O error")

	// ErrCanceled indicates the user canceled an operation.
	ErrCanceled = baseError("canceled")
)

// baseError is a string that implements error.
type baseError string

func (e baseError) Error() string { return string(e) }

// WorkflowError represents an error that occurred during a workflow operation.
type WorkflowError struct {
	// Op is the operation being performed (e.g., "create", "run", "delete").
	Op string
	// Err is the underlying error.
	Err error
	// ID is the workflow identifier (optional).
	ID string
}

func (e *WorkflowError) Error() string {
	if e.ID != "" {
		return fmt.Sprintf("workflow %s %q: %s", e.Op, e.ID, e.Err)
	}
	return fmt.Sprintf("workflow %s: %s", e.Op, e.Err)
}

func (e *WorkflowError) Unwrap() error { return e.Err }

// GitError represents an error that occurred during a git operation.
type GitError struct {
	// Op is the git operation being performed (e.g., "clone", "pull", "push").
	Op string
	// Err is the underlying error.
	Err error
	// Cmd is the full git command that was executed (optional).
	Cmd string
}

func (e *GitError) Error() string {
	if e.Cmd != "" {
		return fmt.Sprintf("git %s: %s\n  cmd: %s", e.Op, e.Err, e.Cmd)
	}
	return fmt.Sprintf("git %s: %s", e.Op, e.Err)
}

func (e *GitError) Unwrap() error { return e.Err }

// ConfigError represents an error related to configuration.
type ConfigError struct {
	// Path is the configuration file path (optional).
	Path string
	// Err is the underlying error.
	Err error
}

func (e *ConfigError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("config %s: %s", e.Path, e.Err)
	}
	return fmt.Sprintf("config: %s", e.Err)
}

func (e *ConfigError) Unwrap() error { return e.Err }

// Wrap adds context to an error by wrapping it with an operation name.
// The returned error implements Unwrap() allowing errors.Is and errors.As
// to work with the wrapped error.
func Wrap(err error, op string) error {
	return &wrappedError{op: op, err: err}
}

// wrappedError is an error with an operation context.
type wrappedError struct {
	op  string
	err error
}

func (e *wrappedError) Error() string { return fmt.Sprintf("%s: %s", e.op, e.err) }
func (e *wrappedError) Unwrap() error { return e.err }

// IsNotFound reports whether err is or wraps ErrNotFound.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsAlreadyExists reports whether err is or wraps ErrAlreadyExists.
func IsAlreadyExists(err error) bool {
	return errors.Is(err, ErrAlreadyExists)
}

// IsInvalid reports whether err is or wraps ErrInvalid.
func IsInvalid(err error) bool {
	return errors.Is(err, ErrInvalid)
}

// IsConflict reports whether err is or wraps ErrConflict.
func IsConflict(err error) bool {
	return errors.Is(err, ErrConflict)
}

// IsGit reports whether err is or wraps ErrGit.
func IsGit(err error) bool {
	return errors.Is(err, ErrGit)
}

// IsIO reports whether err is or wraps ErrIO.
func IsIO(err error) bool {
	return errors.Is(err, ErrIO)
}

// IsCanceled reports whether err is or wraps ErrCanceled.
func IsCanceled(err error) bool {
	return errors.Is(err, ErrCanceled)
}

// AsWorkflowError reports whether err can be typed as a *WorkflowError.
func AsWorkflowError(err error) (*WorkflowError, bool) {
	var we *WorkflowError
	if errors.As(err, &we) {
		return we, true
	}
	return nil, false
}

// AsGitError reports whether err can be typed as a *GitError.
func AsGitError(err error) (*GitError, bool) {
	var ge *GitError
	if errors.As(err, &ge) {
		return ge, true
	}
	return nil, false
}

// AsConfigError reports whether err can be typed as a *ConfigError.
func AsConfigError(err error) (*ConfigError, bool) {
	var ce *ConfigError
	if errors.As(err, &ce) {
		return ce, true
	}
	return nil, false
}
