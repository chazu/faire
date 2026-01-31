// Package placeholders provides placeholder parsing, validation, and substitution.
package placeholders

import (
	"testing"

	"github.com/chazuruo/svf/internal/workflows"
)

func TestExtract(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "no placeholders",
			input:    "echo hello world",
			expected: []string{},
		},
		{
			name:     "single placeholder",
			input:    "echo <name>",
			expected: []string{"name"},
		},
		{
			name:     "multiple placeholders",
			input:    "echo <first> <last>",
			expected: []string{"first", "last"},
		},
		{
			name:     "duplicate placeholders",
			input:    "echo <name> and <name>",
			expected: []string{"name"},
		},
		{
			name:     "placeholder with underscore",
			input:    "echo <user_name>",
			expected: []string{"user_name"},
		},
		{
			name:     "placeholder with dash",
			input:    "echo <user-name>",
			expected: []string{"user-name"},
		},
		{
			name:     "mixed case placeholder",
			input:    "echo <UserName>",
			expected: []string{"UserName"},
		},
		{
			name:     "placeholder in command with args",
			input:    "git commit -m '<message>'",
			expected: []string{"message"},
		},
		{
			name:     "complex command",
			input:    "kubectl create deployment <name> --image=<image> -n <namespace>",
			expected: []string{"name", "image", "namespace"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Extract(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Extract() returned %d items, expected %d", len(result), len(tt.expected))
			}
			for i, exp := range tt.expected {
				if i >= len(result) || result[i] != exp {
					t.Errorf("Extract()[%d] = %q, want %q", i, result[i], exp)
				}
			}
		})
	}
}

func TestSubstitute(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		values   map[string]string
		expected string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "no placeholders",
			input:    "echo hello",
			values:   map[string]string{},
			expected: "echo hello",
			wantErr:  false,
		},
		{
			name:     "single substitution",
			input:    "echo <name>",
			values:   map[string]string{"name": "world"},
			expected: "echo world",
			wantErr:  false,
		},
		{
			name:     "multiple substitutions",
			input:    "echo <first> <last>",
			values:   map[string]string{"first": "John", "last": "Doe"},
			expected: "echo John Doe",
			wantErr:  false,
		},
		{
			name:     "missing placeholder",
			input:    "echo <name>",
			values:   map[string]string{},
			expected: "",
			wantErr:  true,
			errMsg:   "missing placeholders: name",
		},
		{
			name:     "partial substitution",
			input:    "echo <first> <last>",
			values:   map[string]string{"first": "John"},
			expected: "",
			wantErr:  true,
			errMsg:   "missing placeholders: last",
		},
		{
			name:     "duplicate placeholders",
			input:    "echo <name> and <name>",
			values:   map[string]string{"name": "Alice"},
			expected: "echo Alice and Alice",
			wantErr:  false,
		},
		{
			name:     "special characters in value",
			input:    "echo '<message>'",
			values:   map[string]string{"message": "Hello, World!"},
			expected: "echo 'Hello, World!'",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Substitute(tt.input, tt.values)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Substitute() expected error, got nil")
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("Substitute() error = %q, want %q", err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("Substitute() unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Substitute() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		pattern  string
		wantErr  bool
	}{
		{
			name:    "no pattern",
			value:   "anything",
			pattern: "",
			wantErr: false,
		},
		{
			name:    "valid email",
			value:   "user@example.com",
			pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
			wantErr: false,
		},
		{
			name:    "invalid email",
			value:   "not-an-email",
			pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
			wantErr: true,
		},
		{
			name:    "alphanumeric only",
			value:   "abc123",
			pattern: `^[a-zA-Z0-9]+$`,
			wantErr: false,
		},
		{
			name:    "non-alphanumeric fails",
			value:   "abc-123",
			pattern: `^[a-zA-Z0-9]+$`,
			wantErr: true,
		},
		{
			name:    "length requirement",
			value:   "abc",
			pattern: `^.{3,}$`,
			wantErr: false,
		},
		{
			name:    "length requirement fails",
			value:   "ab",
			pattern: `^.{3,}$`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.value, tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExtractWithMetadata(t *testing.T) {
	wf := &workflows.Workflow{
		Title: "Test Workflow",
		Placeholders: map[string]workflows.Placeholder{
			"service": {
				Prompt:   "Service name",
				Default:  "my-service",
				Validate: `^[a-z0-9-]+$`,
			},
			"env": {
				Prompt:  "Environment",
				Default: "dev",
			},
		},
		Steps: []workflows.Step{
			{
				Name:    "Step 1",
				Command: "echo <service>",
			},
			{
				Name:    "Step 2",
				Command: "kubectl create deployment <service> -n <namespace>",
			},
		},
	}

	result := ExtractWithMetadata(wf)

	// ExtractWithMetadata only returns placeholders that are actually used in commands
	// So we should get "service" (defined) and "namespace" (undefined), but NOT "env" (unused)
	if len(result) != 2 {
		t.Errorf("ExtractWithMetadata() returned %d placeholders, expected 2", len(result))
	}

	// Check service metadata
	serviceInfo, ok := result["service"]
	if !ok {
		t.Fatal("ExtractWithMetadata() missing 'service' placeholder")
	}
	if serviceInfo.Prompt != "Service name" {
		t.Errorf("service.Prompt = %q, want 'Service name'", serviceInfo.Prompt)
	}
	if serviceInfo.Default != "my-service" {
		t.Errorf("service.Default = %q, want 'my-service'", serviceInfo.Default)
	}
	if len(serviceInfo.UsedIn) != 2 {
		t.Errorf("service.UsedIn has %d entries, want 2", len(serviceInfo.UsedIn))
	}

	// env is defined but not used, so it should NOT be in the result
	if _, ok := result["env"]; ok {
		t.Error("ExtractWithMetadata() should not include 'env' placeholder (not used in any step)")
	}

	// Check undefined placeholder (namespace)
	namespaceInfo, ok := result["namespace"]
	if !ok {
		t.Fatal("ExtractWithMetadata() missing 'namespace' placeholder")
	}
	if namespaceInfo.Prompt != "" {
		t.Errorf("namespace.Prompt = %q, want empty (undefined)", namespaceInfo.Prompt)
	}
	if len(namespaceInfo.UsedIn) != 1 {
		t.Errorf("namespace.UsedIn has %d entries, want 1", len(namespaceInfo.UsedIn))
	}
}

func TestCollectFromSteps(t *testing.T) {
	steps := []workflows.Step{
		{Command: "echo <name>"},
		{Command: "kubectl create deployment <name> --image=<image>"},
		{Command: "echo <name> again"},
	}

	result := CollectFromSteps(steps)

	expected := []string{"name", "image"}
	if len(result) != len(expected) {
		t.Fatalf("CollectFromSteps() returned %d items, expected %d", len(result), len(expected))
	}

	for i, exp := range expected {
		if result[i] != exp {
			t.Errorf("CollectFromSteps()[%d] = %q, want %q", i, result[i], exp)
		}
	}
}

func TestValidateAtLoadTime(t *testing.T) {
	tests := []struct {
		name    string
		wf      *workflows.Workflow
		wantErr bool
	}{
		{
			name: "valid workflow with placeholders",
			wf: &workflows.Workflow{
				Title: "Test",
				Placeholders: map[string]workflows.Placeholder{
					"name": {
						Validate: `^[a-z]+$`,
					},
				},
				Steps: []workflows.Step{
					{Command: "echo <name>"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid regex pattern",
			wf: &workflows.Workflow{
				Title: "Test",
				Placeholders: map[string]workflows.Placeholder{
					"name": {
						Validate: `[invalid(regex`,
					},
				},
				Steps: []workflows.Step{
					{Command: "echo <name>"},
				},
			},
			wantErr: true,
		},
		{
			name: "workflow without placeholders",
			wf: &workflows.Workflow{
				Title: "Test",
				Steps: []workflows.Step{
					{Command: "echo hello"},
				},
			},
			wantErr: false,
		},
		{
			name: "used but undefined placeholder",
			wf: &workflows.Workflow{
				Title: "Test",
				Placeholders: map[string]workflows.Placeholder{
					"defined": {},
				},
				Steps: []workflows.Step{
					{Command: "echo <undefined>"},
				},
			},
			wantErr: false, // Should not error - just a warning
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAtLoadTime(tt.wf)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAtLoadTime() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
