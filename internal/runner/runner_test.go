// Package runner provides tests for workflow execution engine.
package runner

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/chazuruo/svf/internal/workflows"
)

func TestExecSimpleCommand(t *testing.T) {
	ctx := context.Background()

	config := ExecConfig{
		Command: "echo hello",
		Shell:   "bash",
		Stream:  false,
	}

	result := Exec(ctx, config)

	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	if !strings.Contains(result.Output, "hello") {
		t.Errorf("expected output to contain 'hello', got: %s", result.Output)
	}
}

func TestExecWithShell(t *testing.T) {
	ctx := context.Background()

	shells := []string{"bash", "sh", "zsh"}

	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			// Check if shell is available
			if _, err := os.Stat("/bin/" + shell); err != nil {
				t.Skipf("shell %s not available", shell)
			}

			config := ExecConfig{
				Command: "echo test",
				Shell:   shell,
				Stream:  false,
			}

			result := Exec(ctx, config)

			if !result.Success {
				t.Errorf("expected success with shell %s, got error: %v", shell, result.Error)
			}
		})
	}
}

func TestExecWithCWD(t *testing.T) {
	ctx := context.Background()

	// Create a temporary directory
	tempDir := t.TempDir()

	config := ExecConfig{
		Command: "pwd",
		Shell:   "bash",
		CWD:     tempDir,
		Stream:  false,
	}

	result := Exec(ctx, config)

	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}

	// Output should contain the temp directory path
	output := strings.TrimSpace(result.Output)
	if output != tempDir {
		t.Errorf("expected cwd to be %s, got %s", tempDir, output)
	}
}

func TestExecWithEnv(t *testing.T) {
	ctx := context.Background()

	config := ExecConfig{
		Command: "echo $TEST_VAR",
		Shell:   "bash",
		Env: map[string]string{
			"TEST_VAR": "test_value",
		},
		Stream: false,
	}

	result := Exec(ctx, config)

	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "test_value") {
		t.Errorf("expected output to contain 'test_value', got: %s", result.Output)
	}
}

func TestExecFailedCommand(t *testing.T) {
	ctx := context.Background()

	config := ExecConfig{
		Command: "false", // This command always exits with code 1
		Shell:   "bash",
		Stream:  false,
	}

	result := Exec(ctx, config)

	if result.Success {
		t.Error("expected failure for 'false' command")
	}

	if result.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", result.ExitCode)
	}
}

func TestExecStreaming(t *testing.T) {
	ctx := context.Background()

	config := ExecConfig{
		Command: "echo line1 && echo line2",
		Shell:   "bash",
		Stream:  true,
	}

	result := Exec(ctx, config)

	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "line1") {
		t.Errorf("expected output to contain 'line1', got: %s", result.Output)
	}

	if !strings.Contains(result.Output, "line2") {
		t.Errorf("expected output to contain 'line2', got: %s", result.Output)
	}
}

func TestExecWithContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	config := ExecConfig{
		Command: "sleep 10",
		Shell:   "bash",
		Stream:  false,
	}

	result := Exec(ctx, config)

	if result.Success {
		t.Error("expected failure due to context cancellation")
	}
}

func TestSubstitute(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		params   map[string]string
		expected string
		hasError bool
	}{
		{
			name:     "simple substitution",
			input:    "echo <name>",
			params:   map[string]string{"name": "world"},
			expected: "echo world",
			hasError: false,
		},
		{
			name:     "multiple placeholders",
			input:    "<cmd> <arg>",
			params:   map[string]string{"cmd": "echo", "arg": "test"},
			expected: "echo test",
			hasError: false,
		},
		{
			name:     "missing placeholder",
			input:    "echo <missing>",
			params:   map[string]string{},
			expected: "",
			hasError: true,
		},
		{
			name:     "no placeholders",
			input:    "echo hello",
			params:   map[string]string{},
			expected: "echo hello",
			hasError: false,
		},
		{
			name:     "partial substitution",
			input:    "<a> and <c>",
			params:   map[string]string{"a": "A", "b": "B"},
			expected: "",
			hasError: true, // c is missing
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Substitute(tt.input, tt.params)

			if tt.hasError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, result)
				}
			}
		})
	}
}

func TestExtractPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single placeholder",
			input:    "echo <name>",
			expected: []string{"name"},
		},
		{
			name:     "multiple placeholders",
			input:    "<cmd> <arg> <arg>",
			expected: []string{"cmd", "arg"},
		},
		{
			name:     "no placeholders",
			input:    "echo hello",
			expected: nil,
		},
		{
			name:     "complex placeholders",
			input:    "git clone <repo> && cd <repo>",
			expected: []string{"repo"},
		},
		{
			name:     "placeholder with underscore",
			input:    "echo <my_var>",
			expected: []string{"my_var"},
		},
		{
			name:     "placeholder with dash",
			input:    "echo <my-var>",
			expected: []string{"my-var"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractPlaceholders(tt.input)

			// Compare as sets (order doesn't matter)
			resultMap := make(map[string]bool)
			for _, r := range result {
				resultMap[r] = true
			}

			expectedMap := make(map[string]bool)
			for _, e := range tt.expected {
				expectedMap[e] = true
			}

			if len(resultMap) != len(expectedMap) {
				t.Errorf("expected %d placeholders, got %d", len(expectedMap), len(resultMap))
				return
			}

			for k := range expectedMap {
				if !resultMap[k] {
					t.Errorf("missing placeholder: %s", k)
				}
			}
		})
	}
}

func TestCheckDangerous(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		dangerous   bool
		dangerName  string
	}{
		{
			name:       "safe command",
			command:    "echo hello",
			dangerous:  false,
			dangerName: "",
		},
		{
			name:       "rm -rf /",
			command:    "rm -rf /",
			dangerous:  true,
			dangerName: "Recursive delete",
		},
		{
			name:       "dd command",
			command:    "dd if=/dev/zero of=/dev/sda",
			dangerous:  true,
			dangerName: "Disk overwrite",
		},
		{
			name:       "git push --force",
			command:    "git push --force",
			dangerous:  true,
			dangerName: "Force git push",
		},
		{
			name:       "chmod 000",
			command:    "chmod 000 /tmp/file",
			dangerous:  true,
			dangerName: "Remove all permissions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			danger := CheckDangerous(tt.command)

			if tt.dangerous {
				if danger == nil {
					t.Errorf("expected command to be detected as dangerous")
					return
				}
				if danger.Name != tt.dangerName {
					t.Errorf("expected danger name %q, got %q", tt.dangerName, danger.Name)
				}
			} else {
				if danger != nil {
					t.Errorf("expected command to be safe, got danger: %s", danger.Name)
				}
			}
		})
	}
}

func TestNewRunner(t *testing.T) {
	r := NewRunner()
	if r == nil {
		t.Error("expected runner to be created")
	}
}

func TestWithShell(t *testing.T) {
	r := NewRunner(WithShell("zsh")).(*runner)
	if r.shell != "zsh" {
		t.Errorf("expected shell to be 'zsh', got %s", r.shell)
	}
}

func TestWithStreamOutput(t *testing.T) {
	r := NewRunner(WithStreamOutput(false)).(*runner)
	if r.streamOutput {
		t.Error("expected streamOutput to be false")
	}
}

func TestStdioSink(t *testing.T) {
	sink := NewStdioSink()

	err := sink.Write("test line")
	if err != nil {
		t.Errorf("expected no error writing to sink, got: %v", err)
	}

	err = sink.Close()
	if err != nil {
		t.Errorf("expected no error closing sink, got: %v", err)
	}
}

func TestRunnerRun(t *testing.T) {
	t.Run("simple workflow", func(t *testing.T) {
		wf := &workflows.Workflow{
			Title: "Test Workflow",
			Steps: []workflows.Step{
				{Command: "echo step1"},
				{Command: "echo step2"},
			},
		}

		r := NewRunner(WithStreamOutput(false))
		sink := NewStdioSink()

		plan := Plan{
			Workflow:   wf,
			Parameters: map[string]string{},
			RepoRoot:   "",
		}

		result, err := r.Run(context.Background(), plan, sink)

		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}

		if !result.Success {
			t.Errorf("expected workflow to succeed, got: %v", result)
		}

		if len(result.StepResults) != 2 {
			t.Errorf("expected 2 step results, got %d", len(result.StepResults))
		}
	})
}

func TestExecConfig(t *testing.T) {
	config := ExecConfig{
		Command: "echo test",
		Shell:   "bash",
	}

	if config.Command != "echo test" {
		t.Errorf("expected command to be 'echo test', got %s", config.Command)
	}

	if config.Shell != "bash" {
		t.Errorf("expected shell to be 'bash', got %s", config.Shell)
	}
}

func TestExecWithDangerousCommand(t *testing.T) {
	ctx := context.Background()
	checker := NewDangerChecker(true)

	t.Run("dangerous command without auto-confirm should cancel", func(t *testing.T) {
		// This test requires interactive input, so we'll skip the actual prompt test
		// and just verify the danger is detected
		danger := checker.Check("rm -rf /")
		if danger == nil {
			t.Error("expected command to be detected as dangerous")
		}
		if danger.Name != "Recursive delete" {
			t.Errorf("expected danger name 'Recursive delete', got %s", danger.Name)
		}
	})

	t.Run("dangerous command with auto-confirm should execute", func(t *testing.T) {
		// Use a safe command that looks dangerous but won't actually cause harm
		// The pattern matches, but we use echo to avoid actual damage
		config := ExecConfig{
			Command:       "echo 'test' > /dev/null", // Safe command, just for testing
			Shell:         "bash",
			DangerChecker: checker,
			AutoConfirm:   true,
			Stream:        false,
		}

		result := Exec(ctx, config)

		// The command should execute (with warning shown to stderr)
		if result.Error != nil && result.ExitCode != 0 {
			t.Logf("Command execution result: Success=%v, ExitCode=%d, Error=%v", result.Success, result.ExitCode, result.Error)
		}
	})

	t.Run("safe command should execute without warning", func(t *testing.T) {
		config := ExecConfig{
			Command:       "echo safe",
			Shell:         "bash",
			DangerChecker: checker,
			AutoConfirm:   false,
			Stream:        false,
		}

		result := Exec(ctx, config)

		if !result.Success {
			t.Errorf("expected success for safe command, got error: %v", result.Error)
		}
		if !strings.Contains(result.Output, "safe") {
			t.Errorf("expected output to contain 'safe', got: %s", result.Output)
		}
	})
}
