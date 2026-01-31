// Package export provides workflow export functionality with template support.
package export

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/chazuruo/svf/internal/workflows"
)

// Format represents the export format.
type Format string

const (
	// FormatMarkdown exports as Markdown.
	FormatMarkdown Format = "md"
	// FormatYAML exports as YAML.
	FormatYAML Format = "yaml"
	// FormatJSON exports as JSON.
	FormatJSON Format = "json"
)

// Exporter exports workflows in various formats.
type Exporter struct {
	format      Format
	outPath     string
	updateReadme bool
	template    *template.Template
}

// Options contains export options.
type Options struct {
	Format      Format
	Out         string
	UpdateReadme bool
	CustomTemplate string
}

// NewExporter creates a new exporter.
func NewExporter(opts Options) (*Exporter, error) {
	e := &Exporter{
		format:      opts.Format,
		outPath:     opts.Out,
		updateReadme: opts.UpdateReadme,
	}

	// Load template
	tmpl, err := e.loadTemplate(opts.CustomTemplate)
	if err != nil {
		return nil, err
	}
	e.template = tmpl

	return e, nil
}

// loadTemplate loads the export template.
func (e *Exporter) loadTemplate(customPath string) (*template.Template, error) {
	// If custom template is provided, use it
	if customPath != "" {
		return e.parseTemplateFile(customPath)
	}

	// Check built-in templates based on format
	var tmplContent string
	switch e.format {
	case FormatMarkdown:
		tmplContent = builtinMarkdownTemplate
	case FormatYAML:
		tmplContent = builtinYAMLTemplate
	case FormatJSON:
		tmplContent = builtinJSONTemplate
	default:
		return nil, fmt.Errorf("unsupported format: %s", e.format)
	}

	return template.New("export").Parse(tmplContent)
}

// parseTemplateFile parses a template file.
func (e *Exporter) parseTemplateFile(path string) (*template.Template, error) {
	// Check if path is absolute or relative
	if !filepath.IsAbs(path) {
		// Try user config directory
		homeDir, err := os.UserHomeDir()
		if err == nil {
			configPath := filepath.Join(homeDir, ".config", "svf", "templates", filepath.Base(path))
			if _, err := os.Stat(configPath); err == nil {
				path = configPath
			}
		}

		// Try repo templates directory
		// TODO: Get repo path from config
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading template file: %w", err)
	}

	return template.New("export").Parse(string(data))
}

// Export exports a workflow.
func (e *Exporter) Export(wf *workflows.Workflow) (string, error) {
	var buf bytes.Buffer

	data := e.templateData(wf)
	if err := e.template.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	output := buf.String()

	// Write to file if outPath is specified
	if e.outPath != "" && e.outPath != "-" {
		if err := os.WriteFile(e.outPath, []byte(output), 0644); err != nil {
			return "", fmt.Errorf("writing output file: %w", err)
		}
	}

	return output, nil
}

// ExportToFile exports a workflow to a file.
func (e *Exporter) ExportToFile(wf *workflows.Workflow, path string) error {
	output, err := e.Export(wf)
	if err != nil {
		return err
	}

	// If outPath was used, content is already written
	if e.outPath == path {
		return nil
	}

	return os.WriteFile(path, []byte(output), 0644)
}

// UpdateReadme updates README.md with workflow content.
func (e *Exporter) UpdateReadme(wf *workflows.Workflow, repoPath string) error {
	if !e.updateReadme {
		return nil
	}

	readmePath := filepath.Join(repoPath, "README.md")
	output, err := e.Export(wf)
	if err != nil {
		return err
	}

	// Check if README exists
	_, err = os.Stat(readmePath)
	if os.IsNotExist(err) {
		// Create new README
		return os.WriteFile(readmePath, []byte(output), 0644)
	}

	// Append to existing README
	// TODO: Implement smarter merging
	f, err := os.OpenFile(readmePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString("\n\n" + output)
	return err
}

// templateData creates template data from workflow.
func (e *Exporter) templateData(wf *workflows.Workflow) map[string]interface{} {
	// Build placeholders map
	placeholderMap := make(map[string]string)
	for name, p := range wf.Placeholders {
		placeholderMap[name] = p.Default
	}

	// Build steps list
	stepsData := make([]map[string]interface{}, len(wf.Steps))
	for i, step := range wf.Steps {
		stepsData[i] = map[string]interface{}{
			"index":   i + 1,
			"name":    step.Name,
			"command": step.Command,
		}
	}

	return map[string]interface{}{
		"Title":       wf.Title,
		"Description": wf.Description,
		"Tags":        strings.Join(wf.Tags, ", "),
		"Placeholders": placeholderMap,
		"Steps":       stepsData,
	}
}

// builtinMarkdownTemplate is the default Markdown template.
const builtinMarkdownTemplate = "# {{.Title}}\n\n{{if .Description}}{{.Description}}{{end}}\n\n{{if .Tags}}**Tags:** {{.Tags}}{{end}}\n\n## Steps\n\n{{range .Steps}}{{if .name}}\n### {{.index}}. {{.name}}\n{{else}}\n### {{.index}}.\n{{end}}\n```\n{{.command}}\n```\n{{end}}\n\n{{if .Placeholders}}\n## Placeholders\n\n{{range $key, $value := .Placeholders}}\n- `<{{$key}}>`{{if $value}} (default: {{$value}}){{end}}\n{{end}}\n{{end}}\n---\n*Generated by svf*\n"

// builtinYAMLTemplate is the default YAML template.
const builtinYAMLTemplate = `title: {{.Title}}
{{if .Description}}description: {{.Description}}{{end}}
{{if .Tags}}tags:
{{range $tag := .Tags}}  - {{$tag}}
{{end}}{{end}}
steps:
{{range .Steps}}  - name: {{.name}}
    command: {{.command}}
{{end}}
{{if .Placeholders}}placeholders:
{{range $key, $value := .Placeholders}}  {{ $key }}: {{ $value }}
{{end}}{{end}}
`

// builtinJSONTemplate is the default JSON template.
// Note: For JSON output, consider using encoding/json directly.
const builtinJSONTemplate = "{\n  \"title\": \"{{.Title}}\",\n{{if .Description}}  \"description\": \"{{.Description}}\",\n{{end}}{{if .Tags}}  \"tags\": [{{range $i, $tag := .Tags}}{{if $i}}, {{end}}\"{{$tag}}\"{{end}}],\n{{end}}  \"steps\": [\n{{range $i, $step := .Steps}}{{if $i}},{{end}}    {\n      \"name\": \"{{.name}}\",\n      \"command\": \"{{.command}}\"\n    }{{end}}\n  ]\n}\n"
