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
	repoPath    string
}

// Options contains export options.
type Options struct {
	Format        Format
	Out           string
	UpdateReadme  bool
	CustomTemplate string
	RepoPath      string
}

// NewExporter creates a new exporter.
func NewExporter(opts Options) (*Exporter, error) {
	e := &Exporter{
		format:      opts.Format,
		outPath:     opts.Out,
		updateReadme: opts.UpdateReadme,
		repoPath:    opts.RepoPath,
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
		// Try repo templates directory first (.svf/templates/)
		if e.repoPath != "" {
			repoTemplatePath := filepath.Join(e.repoPath, ".svf", "templates", filepath.Base(path))
			if _, err := os.Stat(repoTemplatePath); err == nil {
				path = repoTemplatePath
			}
		}

		// Try user config directory (~/.config/svf/templates/)
		if !filepath.IsAbs(path) {
			homeDir, err := os.UserHomeDir()
			if err == nil {
				configPath := filepath.Join(homeDir, ".config", "svf", "templates", filepath.Base(path))
				if _, err := os.Stat(configPath); err == nil {
					path = configPath
				}
			}
		}
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
		// Create new README with workflow markers
		workflowMarker := fmt.Sprintf("<!-- Workflow: %s -->", wf.Title)
		content := workflowMarker + "\n" + output + "\n<!-- End Workflow: " + wf.Title + " -->\n"
		return os.WriteFile(readmePath, []byte(content), 0644)
	}

	// Read existing README
	existingContent, err := os.ReadFile(readmePath)
	if err != nil {
		return fmt.Errorf("reading README: %w", err)
	}

	existingStr := string(existingContent)
	existingStr = strings.TrimSpace(existingStr)

	// Check if workflow section already exists
	workflowMarker := fmt.Sprintf("<!-- Workflow: %s -->", wf.Title)
	if strings.Contains(existingStr, workflowMarker) {
		// Replace existing workflow section
		parts := strings.Split(existingStr, workflowMarker)
		if len(parts) >= 2 {
			// Find the end of this workflow section (next <!-- Workflow: or end of file)
			afterWorkflow := parts[1]
			endIndex := strings.Index(afterWorkflow, "<!-- Workflow:")
			if endIndex == -1 {
				endIndex = len(afterWorkflow)
			}
			newContent := parts[0] + workflowMarker + "\n" + output
			if endIndex < len(afterWorkflow) {
				newContent += afterWorkflow[endIndex:]
			}
			return os.WriteFile(readmePath, []byte(newContent), 0644)
		}
	}

	// Append new workflow section with marker
	newContent := existingStr + "\n\n" + workflowMarker + "\n" + output + "\n<!-- End Workflow: " + wf.Title + " -->\n"
	return os.WriteFile(readmePath, []byte(newContent), 0644)
}

// templateData creates template data from workflow.
func (e *Exporter) templateData(wf *workflows.Workflow) map[string]interface{} {
	// Build placeholders map with full details
	placeholderMap := make(map[string]map[string]interface{})
	for name, p := range wf.Placeholders {
		placeholderMap[name] = map[string]interface{}{
			"prompt":   p.Prompt,
			"default":  p.Default,
			"validate": p.Validate,
			"secret":   p.Secret,
		}
	}

	// Build steps list with full details
	stepsData := make([]map[string]interface{}, len(wf.Steps))
	for i, step := range wf.Steps {
		stepData := map[string]interface{}{
			"index":            i + 1,
			"name":             step.Name,
			"command":          step.Command,
			"shell":            step.Shell,
			"cwd":              step.CWD,
			"env":              step.Env,
			"continueOnError":  step.ContinueOnError,
		}
		stepsData[i] = stepData
	}

	// Build defaults
	defaults := map[string]interface{}{
		"shell":            wf.Defaults.Shell,
		"cwd":              wf.Defaults.CWD,
		"confirmEachStep":  wf.Defaults.ConfirmEachStep,
	}

	return map[string]interface{}{
		"ID":          wf.ID,
		"Title":       wf.Title,
		"Description": wf.Description,
		"Tags":        wf.Tags,
		"TagsString":  strings.Join(wf.Tags, ", "),
		"Placeholders": placeholderMap,
		"Steps":       stepsData,
		"Defaults":    defaults,
		"SchemaVersion": wf.SchemaVersion,
	}
}

// builtinMarkdownTemplate is the default Markdown template.
const builtinMarkdownTemplate = "# {{.Title}}\n\n{{if .ID}}**ID:** {{.ID}}{{end}}\n{{if .Description}}{{.Description}}{{end}}\n{{if .Tags}}**Tags:** {{range $i, $tag := .Tags}}{{if $i}}, {{end}}{{$tag}}{{end}}{{end}}\n\n## Steps\n\n{{range .Steps}}### {{.index}}. {{if .name}}{{.name}}{{else}}Step{{end}}\n\n" + "```{{if .shell}}{{.shell}}{{else}}bash{{end}}\n{{.command}}\n```\n" + "{{if .cwd}}**Working Directory:** {{.cwd}}{{end}}\n{{if .env}}**Environment Variables:**\n{{range $key, $value := .env}}- {{$key}}={{$value}}\n{{end}}{{end}}\n{{if .continueOnError}}**Continues on error:** Yes{{end}}\n\n{{end}}\n{{if .Placeholders}}\n## Placeholders\n\n{{range $key, $ph := .Placeholders}}- **<{{$key}}>**\n  {{if $ph.prompt}}{{$ph.prompt}}{{else}}{{$key}}{{end}}\n  {{if $ph.default}}(default: {{$ph.default}}){{end}}\n  {{if $ph.secret}}*This value is secret and will be masked in output*{{end}}\n{{end}}\n{{end}}\n\n{{if .Defaults}}\n## Defaults\n\n{{if .Defaults.shell}}**Shell:** {{.Defaults.shell}}{{end}}\n{{if .Defaults.cwd}}**Working Directory:** {{.Defaults.cwd}}{{end}}\n{{if .Defaults.confirmEachStep}}**Confirm Each Step:** {{.Defaults.confirmEachStep}}{{end}}\n{{end}}\n\n---\n*Generated by svf*\n"

// builtinYAMLTemplate is the default YAML template.
const builtinYAMLTemplate = "{{if .ID}}id: {{.ID}}\n{{end}}title: {{.Title}}\n{{if .Description}}description: {{.Description}}\n{{end}}{{if .Tags}}tags:\n{{range $tag := .Tags}}  - {{$tag}}\n{{end}}{{end}}{{if .Defaults}}defaults:\n  {{if .Defaults.shell}}shell: {{.Defaults.shell}}\n  {{end}}{{if .Defaults.cwd}}cwd: {{.Defaults.cwd}}\n  {{end}}{{if .Defaults.confirmEachStep}}confirm_each_step: {{.Defaults.confirmEachStep}}\n  {{end}}{{end}}steps:\n{{range .Steps}}  - name: {{.name}}\n    command: {{.command}}\n    {{if .shell}}shell: {{.shell}}\n    {{end}}{{if .cwd}}cwd: {{.cwd}}\n    {{end}}{{if .continueOnError}}continue_on_error: {{.continueOnError}}\n    {{end}}{{if .env}}env:\n{{range $key, $value := .env}}      {{$key}}: {{$value}}\n{{end}}  {{end}}{{end}}\n{{if .Placeholders}}placeholders:\n{{range $key, $ph := .Placeholders}}  {{$key}}:\n    prompt: {{$ph.prompt}}\n    default: {{$ph.default}}\n    {{if $ph.validate}}validate: {{$ph.validate}}\n    {{end}}{{if $ph.secret}}secret: {{$ph.secret}}\n    {{end}}{{end}}\n{{end}}\n"

// builtinJSONTemplate is the default JSON template.
// Note: For JSON output, consider using encoding/json directly.
const builtinJSONTemplate = "{\n  \"title\": \"{{.Title}}\",\n{{if .Description}}  \"description\": \"{{.Description}}\",\n{{end}}{{if .Tags}}  \"tags\": [{{range $i, $tag := .Tags}}{{if $i}}, {{end}}\"{{$tag}}\"{{end}}],\n{{end}}  \"steps\": [\n{{range $i, $step := .Steps}}{{if $i}},{{end}}    {\n      \"name\": \"{{.name}}\",\n      \"command\": \"{{.command}}\"\n    }{{end}}\n  ]\n}\n"
