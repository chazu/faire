package workflows

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestUnmarshalWorkflow_ValidMinimal(t *testing.T) {
	data := []byte(`
schema_version: 1
title: Minimal Workflow
steps:
  - name: Run command
    command: echo "Hello, World!"
`)

	wf, err := UnmarshalWorkflow(data)
	require.NoError(t, err)
	require.NotNil(t, wf)

	assert.Equal(t, 1, wf.SchemaVersion)
	assert.Equal(t, "Minimal Workflow", wf.Title)
	assert.Len(t, wf.Steps, 1)
	assert.Equal(t, "Run command", wf.Steps[0].Name)
	assert.Equal(t, `echo "Hello, World!"`, wf.Steps[0].Command)
}

func TestUnmarshalWorkflow_ValidWithPlaceholders(t *testing.T) {
	data := []byte(`
schema_version: 1
id: wf_01HZY3J9Y3G6Q9T3
title: Workflow with Placeholders
description: Demonstrates placeholder usage
tags: [example, placeholders]

defaults:
  shell: bash
  cwd: .
  confirm_each_step: true

placeholders:
  service:
    prompt: "Service name"
    default: "foo-service"
    validate: "^[a-z0-9-]+$"
  namespace:
    prompt: "Kubernetes namespace"
    default: "default"

steps:
  - name: Check pods
    command: "kubectl -n <namespace> get pods -l app=<service>"
  - command: "kubectl -n <namespace> rollout restart deploy/<service>"
`)

	wf, err := UnmarshalWorkflow(data)
	require.NoError(t, err)
	require.NotNil(t, wf)

	assert.Equal(t, 1, wf.SchemaVersion)
	assert.Equal(t, "wf_01HZY3J9Y3G6Q9T3", wf.ID)
	assert.Equal(t, "Workflow with Placeholders", wf.Title)
	assert.Equal(t, "Demonstrates placeholder usage", wf.Description)
	assert.Equal(t, []string{"example", "placeholders"}, wf.Tags)
	assert.Equal(t, "bash", wf.Defaults.Shell)
	assert.Equal(t, ".", wf.Defaults.CWD)
	assert.NotNil(t, wf.Defaults.ConfirmEachStep)
	assert.True(t, *wf.Defaults.ConfirmEachStep)

	assert.Len(t, wf.Placeholders, 2)
	assert.Equal(t, "Service name", wf.Placeholders["service"].Prompt)
	assert.Equal(t, "foo-service", wf.Placeholders["service"].Default)
	assert.Equal(t, "^[a-z0-9-]+$", wf.Placeholders["service"].Validate)

	assert.Len(t, wf.Steps, 2)
	assert.Equal(t, "Check pods", wf.Steps[0].Name)
}

func TestUnmarshalWorkflow_MissingTitle(t *testing.T) {
	data := []byte(`
schema_version: 1
steps:
  - command: echo "test"
`)

	wf, err := UnmarshalWorkflow(data)
	assert.Error(t, err)
	assert.Nil(t, wf)
	assert.Contains(t, err.Error(), "title is required")
}

func TestUnmarshalWorkflow_NoSteps(t *testing.T) {
	data := []byte(`
schema_version: 1
title: Empty Workflow
`)

	wf, err := UnmarshalWorkflow(data)
	assert.Error(t, err)
	assert.Nil(t, wf)
	assert.Contains(t, err.Error(), "at least one step")
}

func TestUnmarshalWorkflow_StepWithoutCommand(t *testing.T) {
	data := []byte(`
schema_version: 1
title: Invalid Step Workflow
steps:
  - name: Step without command
`)

	wf, err := UnmarshalWorkflow(data)
	assert.Error(t, err)
	assert.Nil(t, wf)
	assert.Contains(t, err.Error(), "command is required")
}

func TestUnmarshalWorkflow_InvalidRegex(t *testing.T) {
	data := []byte(`
schema_version: 1
title: Invalid Regex Workflow
placeholders:
  param:
    prompt: "A parameter"
    validate: "[invalid(regex"
steps:
  - command: "echo <param>"
`)

	wf, err := UnmarshalWorkflow(data)
	assert.Error(t, err)
	assert.Nil(t, wf)
	assert.Contains(t, err.Error(), "invalid regex")
}

func TestMarshalWorkflow(t *testing.T) {
	wf := &Workflow{
		SchemaVersion: 1,
		ID:            "wf_test123",
		Title:         "Test Workflow",
		Description:   "A test workflow",
		Tags:          []string{"test", "example"},
		Defaults: Defaults{
			Shell:           "bash",
			CWD:             "/tmp",
			ConfirmEachStep: boolPtr(true),
		},
		Placeholders: map[string]Placeholder{
			"param": {
				Prompt:   "Enter value",
				Default:  "default",
				Validate: "^[a-z]+$",
				Secret:   false,
			},
		},
		Steps: []Step{
			{
				Name:            "Step 1",
				Command:         "echo 'Hello'",
				Shell:           "zsh",
				ContinueOnError: true,
			},
		},
	}

	data, err := MarshalWorkflow(wf)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Test Workflow")
	assert.Contains(t, string(data), "echo 'Hello'")
}

func TestApplyDefaults(t *testing.T) {
	tests := []struct {
		name     string
		workflow *Workflow
		step     *Step
		want     *Step
	}{
		{
			name: "Apply shell default",
			workflow: &Workflow{
				Defaults: Defaults{Shell: "bash"},
			},
			step: &Step{Command: "ls"},
			want: &Step{Command: "ls", Shell: "bash"},
		},
		{
			name: "Apply cwd default",
			workflow: &Workflow{
				Defaults: Defaults{CWD: "/tmp"},
			},
			step: &Step{Command: "ls"},
			want: &Step{Command: "ls", CWD: "/tmp"},
		},
		{
			name: "Apply confirmation default",
			workflow: &Workflow{
				Defaults: Defaults{ConfirmEachStep: boolPtr(true)},
			},
			step: &Step{Command: "ls"},
			want: &Step{Command: "ls", Confirmation: &StepConfirmation{}},
		},
		{
			name: "Step overrides default",
			workflow: &Workflow{
				Defaults: Defaults{Shell: "bash", CWD: "/tmp"},
			},
			step: &Step{Command: "ls", Shell: "zsh", CWD: "/home"},
			want: &Step{Command: "ls", Shell: "zsh", CWD: "/home"},
		},
		{
			name:     "No defaults to apply",
			workflow: &Workflow{},
			step:     &Step{Command: "ls"},
			want:     &Step{Command: "ls"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.workflow.ApplyDefaults(tt.step)
			assert.Equal(t, tt.want.Shell, tt.step.Shell)
			assert.Equal(t, tt.want.CWD, tt.step.CWD)
			if tt.want.Confirmation != nil {
				assert.NotNil(t, tt.step.Confirmation)
			} else {
				assert.Nil(t, tt.step.Confirmation)
			}
		})
	}
}

func TestStepConfirmation_Unmarshal(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		want     *StepConfirmation
	}{
		{
			name: "Boolean true",
			yaml: "confirmation: true",
			want: &StepConfirmation{},
		},
		{
			name: "Boolean false",
			yaml: "confirmation: false",
			want: nil,
		},
		{
			name: "Custom prompt",
			yaml: `confirmation: "Are you sure?"`,
			want: &StepConfirmation{Prompt: "Are you sure?"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var step struct {
				Confirmation *StepConfirmation `yaml:"confirmation"`
			}
			err := yamlUnmarshal([]byte(tt.yaml), &step)
			require.NoError(t, err)

			if tt.want == nil {
				// When confirmation is false, it should be nil or have no prompt
				assert.True(t, step.Confirmation == nil || step.Confirmation.Prompt == "", "confirmation should be nil or empty")
			} else {
				require.NotNil(t, step.Confirmation)
				assert.Equal(t, tt.want.Prompt, step.Confirmation.Prompt)
			}
		})
	}
}

// Test fixtures integration tests
func TestUnmarshalWorkflow_FromFixtures(t *testing.T) {
	fixtureDir := filepath.Join("..", "..", "testdata", "workflows")

	validFixtures := []string{
		"minimal.yaml",
		"with_placeholders.yaml",
		"multi_step.yaml",
	}

	for _, fixture := range validFixtures {
		t.Run(fixture, func(t *testing.T) {
			path := filepath.Join(fixtureDir, fixture)
			data, err := os.ReadFile(path)
			require.NoError(t, err)

			wf, err := UnmarshalWorkflow(data)
			require.NoError(t, err)
			require.NotNil(t, wf)
		})
	}
}

func TestUnmarshalWorkflow_FromInvalidFixtures(t *testing.T) {
	invalidDir := filepath.Join("..", "..", "testdata", "workflows", "invalid_v1")

	entries, err := os.ReadDir(invalidDir)
	require.NoError(t, err)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			path := filepath.Join(invalidDir, entry.Name())
			data, err := os.ReadFile(path)
			require.NoError(t, err)

			wf, err := UnmarshalWorkflow(data)
			assert.Error(t, err, "Expected validation error for %s", entry.Name())
			assert.Nil(t, wf)
		})
	}
}

// Helper functions
func boolPtr(b bool) *bool {
	return &b
}

// yamlUnmarshal is a helper for testing YAML unmarshaling
func yamlUnmarshal(data []byte, v interface{}) error {
	return yamlNewDecoder(data).Decode(v)
}

// Minimal yaml decoder interface for testing
type yamlDecoder interface {
	Decode(v interface{}) error
}

func yamlNewDecoder(data []byte) yamlDecoder {
	// Using gopkg.in/yaml.v3
	return &yamlDecoderImpl{data: data}
}

type yamlDecoderImpl struct {
	data []byte
}

func (d *yamlDecoderImpl) Decode(v interface{}) error {
	return yaml.Unmarshal(d.data, v)
}
