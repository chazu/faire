package export

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chazuruo/svf/internal/workflows"
)

func TestNewExporter(t *testing.T) {
	tests := []struct {
		name    string
		opts    Options
		wantErr bool
	}{
		{
			name: "markdown format",
			opts: Options{
				Format:   FormatMarkdown,
				Out:      "",
				RepoPath: "/tmp/test",
			},
			wantErr: false,
		},
		{
			name: "yaml format",
			opts: Options{
				Format:   FormatYAML,
				Out:      "",
				RepoPath: "/tmp/test",
			},
			wantErr: false,
		},
		{
			name: "json format",
			opts: Options{
				Format:   FormatJSON,
				Out:      "",
				RepoPath: "/tmp/test",
			},
			wantErr: false,
		},
		{
			name: "invalid format",
			opts: Options{
				Format:   Format("invalid"),
				Out:      "",
				RepoPath: "/tmp/test",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewExporter(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewExporter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExporter_Export(t *testing.T) {
	wf := &workflows.Workflow{
		ID:          "test-123",
		SchemaVersion: 1,
		Title:       "Test Workflow",
		Description: "A test workflow for export",
		Tags:        []string{"test", "export"},
		Steps: []workflows.Step{
			{
				Name:    "Step 1",
				Command: "echo 'hello'",
			},
			{
				Name:    "Step 2",
				Command: "ls -la",
			},
		},
		Placeholders: map[string]workflows.Placeholder{
			"name": {
				Prompt:  "Enter your name",
				Default: "World",
			},
		},
		Defaults: workflows.Defaults{
			Shell: "bash",
		},
	}

	tests := []struct {
		name    string
		format  Format
		wantErr bool
	}{
		{
			name:    "markdown export",
			format:  FormatMarkdown,
			wantErr: false,
		},
		{
			name:    "yaml export",
			format:  FormatYAML,
			wantErr: false,
		},
		{
			name:    "json export",
			format:  FormatJSON,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, err := NewExporter(Options{
				Format:   tt.format,
				RepoPath: "/tmp/test",
			})
			if err != nil {
				t.Fatalf("NewExporter() error = %v", err)
			}

			output, err := e.Export(wf)
			if (err != nil) != tt.wantErr {
				t.Errorf("Exporter.Export() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && output == "" {
				t.Error("Exporter.Export() returned empty output")
			}

			// Check that output contains expected content
			if !tt.wantErr {
				if !contains(output, wf.Title) {
					t.Errorf("Export output does not contain workflow title")
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestExporter_ExportToFile(t *testing.T) {
	wf := &workflows.Workflow{
		ID:          "test-file-123",
		SchemaVersion: 1,
		Title:       "Test File Export",
		Description: "Testing file export functionality",
		Tags:        []string{"file", "test"},
		Steps: []workflows.Step{
			{
				Name:    "Test Step",
				Command: "echo 'test'",
			},
		},
	}

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "output.md")

	e, err := NewExporter(Options{
		Format:   FormatMarkdown,
		Out:      outPath,
		RepoPath: tmpDir,
	})
	if err != nil {
		t.Fatalf("NewExporter() error = %v", err)
	}

	output, err := e.Export(wf)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	// Check file was created
	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Errorf("Export() did not create output file")
	}

	// Check file contents
	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if !contains(string(content), wf.Title) {
		t.Errorf("Output file does not contain workflow title")
	}

	// Check that returned output matches file content
	if output != string(content) {
		t.Errorf("Export() output does not match file content")
	}
}

func TestExporter_UpdateReadme(t *testing.T) {
	wf := &workflows.Workflow{
		ID:          "test-readme-123",
		SchemaVersion: 1,
		Title:       "Test README Update",
		Description: "Testing README update functionality",
		Tags:        []string{"readme", "test"},
		Steps: []workflows.Step{
			{
				Name:    "Test Step",
				Command: "echo 'test'",
			},
		},
	}

	tmpDir := t.TempDir()

	e, err := NewExporter(Options{
		Format:       FormatMarkdown,
		UpdateReadme: true,
		RepoPath:     tmpDir,
	})
	if err != nil {
		t.Fatalf("NewExporter() error = %v", err)
	}

	// Test creating new README
	err = e.UpdateReadme(wf, tmpDir)
	if err != nil {
		t.Fatalf("UpdateReadme() error = %v", err)
	}

	readmePath := filepath.Join(tmpDir, "README.md")
	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("Failed to read README: %v", err)
	}

	if !contains(string(content), wf.Title) {
		t.Errorf("README does not contain workflow title")
	}

	// Test updating existing README
	wf2 := &workflows.Workflow{
		ID:          "test-readme-456",
		SchemaVersion: 1,
		Title:       "Second Workflow",
		Description: "Second workflow for README",
		Tags:        []string{"readme"},
		Steps: []workflows.Step{
			{
				Name:    "Another Step",
				Command: "echo 'another'",
			},
		},
	}

	err = e.UpdateReadme(wf2, tmpDir)
	if err != nil {
		t.Fatalf("UpdateReadme() (second) error = %v", err)
	}

	content2, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("Failed to read README (second): %v", err)
	}

	contentStr := string(content2)
	if !contains(contentStr, wf.Title) {
		t.Errorf("README no longer contains first workflow title")
	}
	if !contains(contentStr, wf2.Title) {
		t.Errorf("README does not contain second workflow title")
	}

	// Check for workflow markers
	if !contains(contentStr, "<!-- Workflow: "+wf.Title+" -->") {
		t.Errorf("README does not contain workflow marker for first workflow")
	}
	if !contains(contentStr, "<!-- Workflow: "+wf2.Title+" -->") {
		t.Errorf("README does not contain workflow marker for second workflow")
	}
}

func TestExporter_TemplateData(t *testing.T) {
	wf := &workflows.Workflow{
		ID:          "test-data-123",
		SchemaVersion: 1,
		Title:       "Test Template Data",
		Description: "Testing template data generation",
		Tags:        []string{"data", "test"},
		Defaults: workflows.Defaults{
			Shell:           "zsh",
			CWD:             "/home/user",
			ConfirmEachStep: boolPtr(true),
		},
		Steps: []workflows.Step{
			{
				Name:            "Step 1",
				Command:         "echo 'step 1'",
				Shell:           "bash",
				CWD:             "/tmp",
				ContinueOnError: true,
				Env: map[string]string{
					"VAR1": "value1",
					"VAR2": "value2",
				},
			},
		},
		Placeholders: map[string]workflows.Placeholder{
			"username": {
				Prompt:   "Enter username",
				Default:  "admin",
				Validate: "^[a-zA-Z0-9_]+$",
				Secret:   false,
			},
			"password": {
				Prompt:   "Enter password",
				Default:  "",
				Validate: "",
				Secret:   true,
			},
		},
	}

	e, err := NewExporter(Options{
		Format:   FormatMarkdown,
		RepoPath: "/tmp/test",
	})
	if err != nil {
		t.Fatalf("NewExporter() error = %v", err)
	}

	data := e.templateData(wf)

	// Check basic fields
	if data["ID"] != wf.ID {
		t.Errorf("templateData ID = %v, want %v", data["ID"], wf.ID)
	}
	if data["Title"] != wf.Title {
		t.Errorf("templateData Title = %v, want %v", data["Title"], wf.Title)
	}

	// Check tags
	tags, ok := data["Tags"].([]string)
	if !ok {
		t.Errorf("templateData Tags is not a []string")
	} else if len(tags) != len(wf.Tags) {
		t.Errorf("templateData Tags length = %d, want %d", len(tags), len(wf.Tags))
	}

	// Check placeholders
	placeholders, ok := data["Placeholders"].(map[string]map[string]interface{})
	if !ok {
		t.Errorf("templateData Placeholders is not a map[string]map[string]interface{}")
	} else {
		if _, exists := placeholders["username"]; !exists {
			t.Errorf("templateData Placeholders missing 'username'")
		}
		if _, exists := placeholders["password"]; !exists {
			t.Errorf("templateData Placeholders missing 'password'")
		}
	}

	// Check steps
	steps, ok := data["Steps"].([]map[string]interface{})
	if !ok {
		t.Errorf("templateData Steps is not a []map[string]interface{}")
	} else if len(steps) != len(wf.Steps) {
		t.Errorf("templateData Steps length = %d, want %d", len(steps), len(wf.Steps))
	}

	// Check defaults
	defaults, ok := data["Defaults"].(map[string]interface{})
	if !ok {
		t.Errorf("templateData Defaults is not a map[string]interface{}")
	} else {
		if defaults["shell"] != wf.Defaults.Shell {
			t.Errorf("templateData Defaults.shell = %v, want %v", defaults["shell"], wf.Defaults.Shell)
		}
	}
}

func TestParseTemplateFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a custom template file
	templatePath := filepath.Join(tmpDir, "custom.tmpl")
	templateContent := "Custom: {{.Title}}\nDescription: {{.Description}}\n"
	err := os.WriteFile(templatePath, []byte(templateContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create template file: %v", err)
	}

	e, err := NewExporter(Options{
		Format:         FormatMarkdown,
		CustomTemplate: templatePath,
		RepoPath:       tmpDir,
	})
	if err != nil {
		t.Fatalf("NewExporter() error = %v", err)
	}

	if e.template == nil {
		t.Errorf("Exporter template is nil after loading custom template")
	}

	wf := &workflows.Workflow{
		ID:          "test-custom-123",
		SchemaVersion: 1,
		Title:       "Custom Template Test",
		Description: "Testing custom template",
		Steps: []workflows.Step{
			{
				Name:    "Test",
				Command: "echo 'test'",
			},
		},
	}

	output, err := e.Export(wf)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if !contains(output, "Custom: Custom Template Test") {
		t.Errorf("Custom template not applied correctly")
	}
	if !contains(output, "Description: Testing custom template") {
		t.Errorf("Custom template not applied correctly")
	}
}

func TestExporter_TemplateSearchPaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .svf/templates directory
	svfTemplatesDir := filepath.Join(tmpDir, ".svf", "templates")
	err := os.MkdirAll(svfTemplatesDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .svf/templates: %v", err)
	}

	// Create custom template in .svf/templates
	repoTemplatePath := filepath.Join(svfTemplatesDir, "export.md")
	repoTemplateContent := "# Repo Template: {{.Title}}\n"
	err = os.WriteFile(repoTemplatePath, []byte(repoTemplateContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create repo template: %v", err)
	}

	// Create exporter and load template by basename
	e, err := NewExporter(Options{
		Format:         FormatMarkdown,
		CustomTemplate: "export.md", // Just basename, should find in .svf/templates
		RepoPath:       tmpDir,
	})
	if err != nil {
		t.Fatalf("NewExporter() error = %v", err)
	}

	wf := &workflows.Workflow{
		ID:          "test-search-123",
		SchemaVersion: 1,
		Title:       "Template Search Test",
		Steps: []workflows.Step{
			{
				Name:    "Test",
				Command: "echo 'test'",
			},
		},
	}

	output, err := e.Export(wf)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if !contains(output, "Repo Template: Template Search Test") {
		t.Errorf("Repo template not found and applied")
	}
}

func boolPtr(b bool) *bool {
	return &b
}

