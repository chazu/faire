package workflows

import (
	"errors"
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

// SchemaVersion is the current workflow schema version
const SchemaVersion = 1

// Workflow represents a runnable workflow definition
type Workflow struct {
	SchemaVersion int                      `yaml:"schema_version"`
	ID            string                   `yaml:"id,omitempty"`            // Optional ULID
	Title         string                   `yaml:"title"`                   // Required
	Description   string                   `yaml:"description,omitempty"`
	Tags          []string                 `yaml:"tags,omitempty"`
	Defaults      Defaults                 `yaml:"defaults,omitempty"`
	Placeholders  map[string]Placeholder   `yaml:"placeholders,omitempty"`
	Steps         []Step                   `yaml:"steps"`
}

// Defaults specifies default values for workflow steps
type Defaults struct {
	Shell            string `yaml:"shell,omitempty"`             // Default shell (bash, zsh, sh, pwsh)
	CWD              string `yaml:"cwd,omitempty"`               // Default working directory
	ConfirmEachStep  *bool  `yaml:"confirm_each_step,omitempty"` // Default confirmation behavior
}

// Step represents a single step in a workflow
type Step struct {
	Name            string            `yaml:"name,omitempty"`            // Step name/identifier
	Command         string            `yaml:"command"`                   // Required command to execute
	Shell           string            `yaml:"shell,omitempty"`           // Override default shell
	CWD             string            `yaml:"cwd,omitempty"`             // Override default working directory
	Env             map[string]string `yaml:"env,omitempty"`             // Environment variables
	ContinueOnError bool              `yaml:"continue_on_error,omitempty"` // Continue if this step fails
	Confirmation    *StepConfirmation `yaml:"confirmation,omitempty"`    // Confirmation prompt
}

// StepConfirmation defines the confirmation behavior for a step
type StepConfirmation struct {
	Prompt string // Custom prompt text
	// Alternatively, can be a bool for simple enable/disable
}

// MarshalYAML implements custom YAML marshaling for StepConfirmation
func (sc *StepConfirmation) MarshalYAML() (interface{}, error) {
	if sc.Prompt != "" {
		return sc.Prompt, nil
	}
	return true, nil
}

// UnmarshalYAML implements custom YAML unmarshaling for StepConfirmation
func (sc *StepConfirmation) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		var scalar interface{}
		if err := value.Decode(&scalar); err != nil {
			return err
		}

		switch v := scalar.(type) {
		case bool:
			if v {
				*sc = StepConfirmation{Prompt: ""}
			}
		case string:
			*sc = StepConfirmation{Prompt: v}
		}
	}
	return nil
}

// Placeholder defines a parameter that can be substituted in workflow commands
type Placeholder struct {
	Prompt   string `yaml:"prompt,omitempty"`   // Prompt text for the user
	Default  string `yaml:"default,omitempty"`  // Default value
	Validate string `yaml:"validate,omitempty"` // Regex validation pattern
	Secret   bool   `yaml:"secret,omitempty"`   // Mask value in output
}

// Validate validates the workflow structure and content
func (w *Workflow) Validate() error {
	// Title is required
	if w.Title == "" {
		return errors.New("workflow title is required")
	}

	// At least one step is required
	if len(w.Steps) == 0 {
		return errors.New("workflow must have at least one step")
	}

	// Validate each step
	for i, step := range w.Steps {
		if err := step.Validate(); err != nil {
			return fmt.Errorf("step %d: %w", i, err)
		}
	}

	// Validate placeholders
	for name, ph := range w.Placeholders {
		if err := ph.ValidatePlaceholder(); err != nil {
			return fmt.Errorf("placeholder %s: %w", name, err)
		}
	}

	return nil
}

// Validate validates a step
func (s *Step) Validate() error {
	if s.Command == "" {
		return errors.New("step command is required")
	}
	return nil
}

// ValidatePlaceholder validates a placeholder's regex pattern
func (p *Placeholder) ValidatePlaceholder() error {
	if p.Validate != "" {
		if _, err := regexp.Compile(p.Validate); err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
	}
	return nil
}

// ApplyDefaults applies workflow-level defaults to a step
func (w *Workflow) ApplyDefaults(step *Step) {
	if step.Shell == "" && w.Defaults.Shell != "" {
		step.Shell = w.Defaults.Shell
	}
	if step.CWD == "" && w.Defaults.CWD != "" {
		step.CWD = w.Defaults.CWD
	}
	if step.Confirmation == nil && w.Defaults.ConfirmEachStep != nil {
		if *w.Defaults.ConfirmEachStep {
			step.Confirmation = &StepConfirmation{}
		}
	}
}

// UnmarshalWorkflow unmarshals a workflow from YAML bytes
func UnmarshalWorkflow(data []byte) (*Workflow, error) {
	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow: %w", err)
	}

	// Validate the workflow
	if err := wf.Validate(); err != nil {
		return nil, fmt.Errorf("workflow validation failed: %w", err)
	}

	return &wf, nil
}

// MarshalWorkflow marshals a workflow to YAML bytes
func MarshalWorkflow(wf *Workflow) ([]byte, error) {
	data, err := yaml.Marshal(wf)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workflow: %w", err)
	}
	return data, nil
}
