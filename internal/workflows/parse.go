package workflows

import (
	"io"
	"os"
)

// LoadYAML reads and unmarshals a workflow from a YAML file.
//
// LoadYAML combines file reading with validation - it returns an error
// if the file cannot be read or if the workflow content is invalid.
//
// Parameters:
//   - path: Path to the workflow.yaml file
//
// Returns:
//   - *Workflow: The parsed workflow
//   - error: Any error from reading or parsing the file
func LoadYAML(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return UnmarshalWorkflow(data)
}

// LoadYAMLReader unmarshals a workflow from an io.Reader.
//
// LoadYAMLReader is useful for reading workflows from stdin,
// HTTP responses, or other streaming sources.
//
// Parameters:
//   - r: io.Reader containing YAML workflow data
//
// Returns:
//   - *Workflow: The parsed workflow
//   - error: Any error from reading or parsing the data
func LoadYAMLReader(r io.Reader) (*Workflow, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return UnmarshalWorkflow(data)
}
