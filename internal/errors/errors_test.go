package errors_test

import (
	"errors"
	"fmt"
	"os"
	"testing"

	faireerrors "github.com/chazuruo/faire/internal/errors"
)

// TestBaseErrors verifies that all base error types have correct messages.
func TestBaseErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"ErrNotFound", faireerrors.ErrNotFound, "not found"},
		{"ErrAlreadyExists", faireerrors.ErrAlreadyExists, "already exists"},
		{"ErrInvalid", faireerrors.ErrInvalid, "invalid"},
		{"ErrConflict", faireerrors.ErrConflict, "conflict"},
		{"ErrGit", faireerrors.ErrGit, "git operation failed"},
		{"ErrIO", faireerrors.ErrIO, "I/O error"},
		{"ErrCanceled", faireerrors.ErrCanceled, "canceled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestWorkflowError verifies WorkflowError formatting and unwrapping.
func TestWorkflowError(t *testing.T) {
	tests := []struct {
		name string
		err  *faireerrors.WorkflowError
		want string
	}{
		{
			name: "with ID",
			err:  &faireerrors.WorkflowError{Op: "create", Err: faireerrors.ErrNotFound, ID: "test-workflow"},
			want: `workflow create "test-workflow": not found`,
		},
		{
			name: "without ID",
			err:  &faireerrors.WorkflowError{Op: "run", Err: faireerrors.ErrInvalid},
			want: "workflow run: invalid",
		},
		{
			name: "wrapped custom error",
			err:  &faireerrors.WorkflowError{Op: "delete", Err: fmt.Errorf("custom error"), ID: "abc"},
			want: `workflow delete "abc": custom error`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("Unwrap returns original error", func(t *testing.T) {
		original := faireerrors.ErrNotFound
		wrapped := &faireerrors.WorkflowError{Op: "test", Err: original}
		if !errors.Is(wrapped, original) {
			t.Error("Unwrap() did not return the original error for errors.Is")
		}
	})
}

// TestGitError verifies GitError formatting and unwrapping.
func TestGitError(t *testing.T) {
	tests := []struct {
		name string
		err  *faireerrors.GitError
		want string
	}{
		{
			name: "with command",
			err:  &faireerrors.GitError{Op: "clone", Err: faireerrors.ErrIO, Cmd: "git clone https://example.com/repo"},
			want: "git clone: I/O error\n  cmd: git clone https://example.com/repo",
		},
		{
			name: "without command",
			err:  &faireerrors.GitError{Op: "push", Err: faireerrors.ErrGit},
			want: "git push: git operation failed",
		},
		{
			name: "wrapped os error",
			err:  &faireerrors.GitError{Op: "fetch", Err: os.ErrNotExist},
			want: "git fetch: file does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("Unwrap returns original error", func(t *testing.T) {
		original := faireerrors.ErrNotFound
		wrapped := &faireerrors.GitError{Op: "test", Err: original}
		if !errors.Is(wrapped, original) {
			t.Error("Unwrap() did not return the original error for errors.Is")
		}
	})
}

// TestConfigError verifies ConfigError formatting and unwrapping.
func TestConfigError(t *testing.T) {
	tests := []struct {
		name string
		err  *faireerrors.ConfigError
		want string
	}{
		{
			name: "with path",
			err:  &faireerrors.ConfigError{Path: "~/.config/faire/config.yaml", Err: faireerrors.ErrInvalid},
			want: "config ~/.config/faire/config.yaml: invalid",
		},
		{
			name: "without path",
			err:  &faireerrors.ConfigError{Err: faireerrors.ErrNotFound},
			want: "config: not found",
		},
		{
			name: "wrapped custom error",
			err:  &faireerrors.ConfigError{Path: "/etc/config.json", Err: fmt.Errorf("parse error")},
			want: "config /etc/config.json: parse error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("Unwrap returns original error", func(t *testing.T) {
		original := faireerrors.ErrInvalid
		wrapped := &faireerrors.ConfigError{Err: original}
		if !errors.Is(wrapped, original) {
			t.Error("Unwrap() did not return the original error for errors.Is")
		}
	})
}

// TestWrap verifies the Wrap helper function.
func TestWrap(t *testing.T) {
	original := faireerrors.ErrNotFound
	wrapped := faireerrors.Wrap(original, "readFile")

	if got := wrapped.Error(); got != "readFile: not found" {
		t.Errorf("Error() = %q, want 'readFile: not found'", got)
	}

	t.Run("Unwrap returns original error", func(t *testing.T) {
		if !errors.Is(wrapped, original) {
			t.Error("Wrap() did not preserve the original error for errors.Is")
		}
	})

	t.Run("Double wrap preserves original", func(t *testing.T) {
		doubleWrapped := faireerrors.Wrap(wrapped, "loadConfig")
		if !errors.Is(doubleWrapped, original) {
			t.Error("Double wrap did not preserve the original error")
		}
	})
}

// TestIsHelpers verifies all Is<TYPE>() helper functions.
func TestIsHelpers(t *testing.T) {
	baseTests := []struct {
		name       string
		baseErr    error
		isFunc     func(error) bool
		expectTrue bool
	}{
		{"IsNotFound", faireerrors.ErrNotFound, faireerrors.IsNotFound, true},
		{"IsAlreadyExists", faireerrors.ErrAlreadyExists, faireerrors.IsAlreadyExists, true},
		{"IsInvalid", faireerrors.ErrInvalid, faireerrors.IsInvalid, true},
		{"IsConflict", faireerrors.ErrConflict, faireerrors.IsConflict, true},
		{"IsGit", faireerrors.ErrGit, faireerrors.IsGit, true},
		{"IsIO", faireerrors.ErrIO, faireerrors.IsIO, true},
		{"IsCanceled", faireerrors.ErrCanceled, faireerrors.IsCanceled, true},
	}

	for _, tt := range baseTests {
		t.Run(tt.name+" direct", func(t *testing.T) {
			if !tt.isFunc(tt.baseErr) {
				t.Errorf("%s(%v) = false, want true", tt.name, tt.baseErr)
			}
		})
	}

	t.Run("IsNotFound with wrapped error", func(t *testing.T) {
		wrapped := &faireerrors.WorkflowError{Op: "get", Err: faireerrors.ErrNotFound}
		if !faireerrors.IsNotFound(wrapped) {
			t.Error("IsNotFound(wrapped WorkflowError) = false, want true")
		}
	})

	t.Run("IsGit with GitError", func(t *testing.T) {
		wrapped := &faireerrors.GitError{Op: "push", Err: faireerrors.ErrGit}
		if !faireerrors.IsGit(wrapped) {
			t.Error("IsGit(GitError) = false, want true")
		}
	})

	t.Run("IsNotFound with different error", func(t *testing.T) {
		if faireerrors.IsNotFound(faireerrors.ErrInvalid) {
			t.Error("IsNotFound(ErrInvalid) = true, want false")
		}
	})
}

// TestAsHelpers verifies all As<TYPE>Error>() helper functions.
func TestAsHelpers(t *testing.T) {
	t.Run("AsWorkflowError", func(t *testing.T) {
		we := &faireerrors.WorkflowError{Op: "create", Err: faireerrors.ErrAlreadyExists, ID: "test"}
		result, ok := faireerrors.AsWorkflowError(we)
		if !ok {
			t.Fatal("AsWorkflowError(valid) = false, want true")
		}
		if result.Op != "create" || result.ID != "test" {
			t.Errorf("AsWorkflowError returned wrong struct: got Op=%q, ID=%q", result.Op, result.ID)
		}
	})

	t.Run("AsWorkflowError with wrapped", func(t *testing.T) {
		wrapped := faireerrors.Wrap(&faireerrors.WorkflowError{Op: "run", Err: faireerrors.ErrInvalid}, "outer")
		result, ok := faireerrors.AsWorkflowError(wrapped)
		if !ok {
			t.Fatal("AsWorkflowError(wrapped) = false, want true")
		}
		if result.Op != "run" {
			t.Errorf("AsWorkflowError returned wrong Op: got %q, want 'run'", result.Op)
		}
	})

	t.Run("AsWorkflowError with wrong type", func(t *testing.T) {
		_, ok := faireerrors.AsWorkflowError(faireerrors.ErrNotFound)
		if ok {
			t.Error("AsWorkflowError(ErrNotFound) = true, want false")
		}
	})

	t.Run("AsGitError", func(t *testing.T) {
		ge := &faireerrors.GitError{Op: "clone", Err: faireerrors.ErrIO, Cmd: "git clone"}
		result, ok := faireerrors.AsGitError(ge)
		if !ok {
			t.Fatal("AsGitError(valid) = false, want true")
		}
		if result.Op != "clone" || result.Cmd != "git clone" {
			t.Errorf("AsGitError returned wrong struct: got Op=%q, Cmd=%q", result.Op, result.Cmd)
		}
	})

	t.Run("AsGitError with wrong type", func(t *testing.T) {
		_, ok := faireerrors.AsGitError(faireerrors.ErrGit)
		if ok {
			t.Error("AsGitError(ErrGit) = true, want false")
		}
	})

	t.Run("AsConfigError", func(t *testing.T) {
		ce := &faireerrors.ConfigError{Path: "/path/to/config", Err: faireerrors.ErrInvalid}
		result, ok := faireerrors.AsConfigError(ce)
		if !ok {
			t.Fatal("AsConfigError(valid) = false, want true")
		}
		if result.Path != "/path/to/config" {
			t.Errorf("AsConfigError returned wrong Path: got %q, want '/path/to/config'", result.Path)
		}
	})

	t.Run("AsConfigError with wrong type", func(t *testing.T) {
		_, ok := faireerrors.AsConfigError(faireerrors.ErrInvalid)
		if ok {
			t.Error("AsConfigError(ErrInvalid) = true, want false")
		}
	})
}

// TestStandardLibraryErrors verifies compatibility with standard library errors package.
func TestStandardLibraryErrors(t *testing.T) {
	t.Run("errors.Is with base errors", func(t *testing.T) {
		if !errors.Is(faireerrors.ErrNotFound, faireerrors.ErrNotFound) {
			t.Error("errors.Is(ErrNotFound, ErrNotFound) = false, want true")
		}
		if errors.Is(faireerrors.ErrNotFound, faireerrors.ErrInvalid) {
			t.Error("errors.Is(ErrNotFound, ErrInvalid) = true, want false")
		}
	})

	t.Run("errors.Is with wrapped errors", func(t *testing.T) {
		wrapped := &faireerrors.WorkflowError{Op: "test", Err: faireerrors.ErrNotFound}
		if !errors.Is(wrapped, faireerrors.ErrNotFound) {
			t.Error("errors.Is(wrapped, ErrNotFound) = false, want true")
		}
	})

	t.Run("errors.As with WorkflowError", func(t *testing.T) {
		wrapped := &faireerrors.WorkflowError{Op: "create", Err: faireerrors.ErrAlreadyExists}
		var we *faireerrors.WorkflowError
		if !errors.As(wrapped, &we) {
			t.Error("errors.As(wrapped, &WorkflowError) = false, want true")
		}
		if we.Op != "create" {
			t.Errorf("errors.As extracted wrong Op: got %q, want 'create'", we.Op)
		}
	})

	t.Run("errors.As with wrong type", func(t *testing.T) {
		var we *faireerrors.WorkflowError
		if errors.As(faireerrors.ErrNotFound, &we) {
			t.Error("errors.As(ErrNotFound, &WorkflowError) = true, want false")
		}
	})
}

// TestErrorChaining verifies that error chaining works correctly.
func TestErrorChaining(t *testing.T) {
	t.Run("Chain of wrapped errors", func(t *testing.T) {
		base := faireerrors.ErrNotFound
		layer1 := faireerrors.Wrap(base, "layer1")
		layer2 := faireerrors.Wrap(layer1, "layer2")
		layer3 := faireerrors.Wrap(layer2, "layer3")

		if !errors.Is(layer3, base) {
			t.Error("Triple-wrapped error does not match base via errors.Is")
		}

		expected := "layer3: layer2: layer1: not found"
		if got := layer3.Error(); got != expected {
			t.Errorf("Chained error message = %q, want %q", got, expected)
		}
	})

	t.Run("WorkflowError in chain", func(t *testing.T) {
		base := faireerrors.ErrInvalid
		workflowErr := &faireerrors.WorkflowError{Op: "run", Err: base, ID: "test"}
		wrapped := faireerrors.Wrap(workflowErr, "execute")

		if !errors.Is(wrapped, base) {
			t.Error("Chained error does not match base via errors.Is")
		}

		var we *faireerrors.WorkflowError
		if !errors.As(wrapped, &we) {
			t.Error("Cannot extract WorkflowError from chain via errors.As")
		}
		if we.ID != "test" {
			t.Errorf("Extracted WorkflowError has wrong ID: got %q, want 'test'", we.ID)
		}
	})
}
