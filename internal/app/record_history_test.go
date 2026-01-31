package app

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateStepName(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected string
	}{
		{"kubectl get", "kubectl get pods", "Kubernetes get"},
		{"kubectl apply", "kubectl apply -f deployment.yaml", "Kubernetes apply"},
		{"git status", "git status", "Git status"},
		{"git commit", "git commit -m 'message'", "Git commit"},
		{"docker ps", "docker ps", "Docker ps"},
		{"docker build", "docker build -t app .", "Docker build"},
		{"ls command", "ls -la", "Ls"},
		{"make build", "make build", "Make build"},
		{"npm install", "npm install", "NPM install"},
		{"terraform apply", "terraform apply", "Terraform"},
		{"empty command", "", "Unnamed command"},
		{"aws s3", "aws s3 ls", "AWS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateStepName(tt.command)
			if result != tt.expected {
				t.Errorf("GenerateStepName(%q) = %q, want %q", tt.command, result, tt.expected)
			}
		})
	}
}

func TestCommandsToSteps(t *testing.T) {
	commands := []string{
		"kubectl get pods",
		"kubectl apply -f deployment.yaml",
		"git status",
	}

	steps := CommandsToSteps(commands)

	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(steps))
	}

	if steps[0].Command != commands[0] {
		t.Errorf("step 0 command = %q, want %q", steps[0].Command, commands[0])
	}

	if steps[0].Name != "Kubernetes get" {
		t.Errorf("step 0 name = %q, want 'Kubernetes get'", steps[0].Name)
	}
}

func TestDetectTagsFromCommands(t *testing.T) {
	tests := []struct {
		name     string
		commands []string
		expected []string
	}{
		{
			"kubernetes commands",
			[]string{"kubectl get pods", "kubectl apply -f x.yaml"},
			[]string{"kubernetes"},
		},
		{
			"mixed commands",
			[]string{"kubectl get pods", "docker ps", "terraform apply"},
			[]string{"docker", "kubernetes", "terraform"},
		},
		{
			"deployment",
			[]string{"kubectl deploy app", "docker build"},
			[]string{"deployment", "docker", "kubernetes"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectTagsFromCommands(tt.commands)
			// Check that all expected tags are present
			expectedMap := make(map[string]bool)
			for _, tag := range tt.expected {
				expectedMap[tag] = true
			}
			for _, tag := range result {
				if !expectedMap[tag] {
					t.Logf("unexpected tag: %s", tag)
				}
				delete(expectedMap, tag)
			}
			for tag := range expectedMap {
				t.Errorf("missing expected tag: %s", tag)
			}
		})
	}
}

func TestGenerateWorkflowFromCommands(t *testing.T) {
	commands := []string{
		"kubectl get pods",
		"kubectl apply -f deployment.yaml",
	}

	opts := RecordHistoryOptions{
		Title: "Test Workflow",
		Desc:  "Test description",
	}

	wf := GenerateWorkflowFromCommands(commands, opts)

	if wf.Title != "Test Workflow" {
		t.Errorf("title = %q, want 'Test Workflow'", wf.Title)
	}

	if wf.Description != "Test description" {
		t.Errorf("description = %q, want 'Test description'", wf.Description)
	}

	if len(wf.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(wf.Steps))
	}

	if wf.Steps[0].Command != commands[0] {
		t.Errorf("step 0 command = %q, want %q", wf.Steps[0].Command, commands[0])
	}
}

func TestGenerateWorkflowFromCommands_AutoTitle(t *testing.T) {
	commands := []string{"kubectl get pods"}

	opts := RecordHistoryOptions{}
	wf := GenerateWorkflowFromCommands(commands, opts)

	// Title should be auto-generated
	if wf.Title != "Kubernetes get" {
		t.Errorf("auto-generated title = %q, want 'Kubernetes get'", wf.Title)
	}

	// Tags should be detected
	if len(wf.Tags) == 0 {
		t.Error("expected tags to be detected, got none")
	}

	hasKubernetesTag := false
	for _, tag := range wf.Tags {
		if tag == "kubernetes" {
			hasKubernetesTag = true
			break
		}
	}
	if !hasKubernetesTag {
		t.Error("expected 'kubernetes' tag to be detected")
	}
}

func TestGenerateWorkflowFromCommands_MultipleCommandsAutoTitle(t *testing.T) {
	commands := []string{
		"kubectl get pods",
		"kubectl apply -f x.yaml",
	}

	opts := RecordHistoryOptions{}
	wf := GenerateWorkflowFromCommands(commands, opts)

	// Title should have "and more" suffix
	if !strings.Contains(wf.Title, "and more") {
		t.Errorf("auto-generated title with multiple commands should contain 'and more', got %q", wf.Title)
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"1h", time.Hour},
		{"2h", 2 * time.Hour},
		{"1d", 24 * time.Hour},
		{"2d", 48 * time.Hour},
		{"1w", 7 * 24 * time.Hour},
		{"30m", 30 * time.Minute},
		{"1h30m", 90 * time.Minute}, // Standard format
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseDuration(tt.input)
			if err != nil {
				t.Fatalf("ParseDuration(%q) error: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseDuration_Invalid(t *testing.T) {
	_, err := ParseDuration("invalid")
	if err == nil {
		t.Error("ParseDuration('invalid') should return error")
	}

	_, err = ParseDuration("1x")
	if err == nil {
		t.Error("ParseDuration('1x') should return error")
	}
}
