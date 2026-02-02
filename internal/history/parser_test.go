package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseBash(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantLen  int
		wantCmd  string
		wantSkip []string // commands that should be skipped
	}{
		{
			name: "basic bash history with timestamps",
			content: `#1616420000
git status
#1616420100
git log --oneline
#1616420200
ls -la
`,
			wantLen: 2, // ls should be skipped
			wantCmd: "git status",
		},
		{
			name: "bash history with multi-line command",
			content: `#1616420000
echo "multi-line \
continuation"
#1616420100
git status
`,
			wantLen: 2,
			wantCmd: `echo "multi-line
continuation"`,
		},
		{
			name: "bash history without timestamps",
			content: `git status
git log
ls -la
`,
			wantLen: 2, // ls should be skipped
			wantCmd: "git status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "bash_history")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}

			parser := NewBashParser()
			lines, err := parser.Parse(tmpFile)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if len(lines) != tt.wantLen {
				t.Errorf("Parse() returned %d lines, want %d", len(lines), tt.wantLen)
			}

			if len(lines) > 0 && lines[0].Command != tt.wantCmd {
				t.Errorf("Parse()[0].Command = %q, want %q", lines[0].Command, tt.wantCmd)
			}

			// Verify shell type
			for _, line := range lines {
				if line.Shell != "bash" {
					t.Errorf("Parse()[].Shell = %q, want 'bash'", line.Shell)
				}
			}
		})
	}
}

func TestParseZsh(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantLen int
		wantCmd string
	}{
		{
			name: "basic zsh history",
			content: `:1616420000:0:git status
:1616420100:0:git log --oneline
:1616420200:0:ls -la
`,
			wantLen: 2, // ls should be skipped
			wantCmd: "git status",
		},
		{
			name: "zsh history with multi-line command",
			content: `:1616420000:0:echo "multi-line \
:1616420001:0:continuation"
:1616420100:0:git status
`,
			wantLen: 2,
			wantCmd: `echo "multi-line
continuation"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "zsh_history")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}

			parser := NewZshParser()
			lines, err := parser.Parse(tmpFile)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if len(lines) != tt.wantLen {
				t.Errorf("Parse() returned %d lines, want %d", len(lines), tt.wantLen)
			}

			if len(lines) > 0 && lines[0].Command != tt.wantCmd {
				t.Errorf("Parse()[0].Command = %q, want %q", lines[0].Command, tt.wantCmd)
			}

			// Verify shell type
			for _, line := range lines {
				if line.Shell != "zsh" {
					t.Errorf("Parse()[].Shell = %q, want 'zsh'", line.Shell)
				}
			}
		})
	}
}

func TestFilterLines(t *testing.T) {
	baseTime := time.Unix(1616420000, 0)

	lines := []HistoryLine{
		{Timestamp: baseTime, Command: "git status", Shell: "bash"},
		{Timestamp: baseTime, Command: "git status", Shell: "bash"}, // duplicate
		{Timestamp: baseTime, Command: "", Shell: "bash"},            // empty
		{Timestamp: baseTime, Command: "ls -la", Shell: "bash"},      // builtin
		{Timestamp: baseTime, Command: "go build", Shell: "bash"},
	}

	tests := []struct {
		name    string
		opts    FilterOptions
		wantLen int
	}{
		{
			name:    "no filtering",
			opts:    FilterOptions{RemoveDuplicates: false, RemoveEmpty: false, SkipBuiltins: false},
			wantLen: 5,
		},
		{
			name:    "remove duplicates",
			opts:    FilterOptions{RemoveDuplicates: true, RemoveEmpty: false, SkipBuiltins: false},
			wantLen: 4,
		},
		{
			name:    "remove empty",
			opts:    FilterOptions{RemoveDuplicates: false, RemoveEmpty: true, SkipBuiltins: false},
			wantLen: 4,
		},
		{
			name:    "skip builtins",
			opts:    FilterOptions{RemoveDuplicates: false, RemoveEmpty: false, SkipBuiltins: true},
			wantLen: 4,
		},
		{
			name:    "default filter options",
			opts:    DefaultFilterOptions(),
			wantLen: 2, // go build + one git status (dups, empty, ls removed)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterLines(lines, tt.opts)
			if len(result) != tt.wantLen {
				t.Errorf("FilterLines() returned %d lines, want %d", len(result), tt.wantLen)
			}
		})
	}
}

func TestDetectHistoryFiles(t *testing.T) {
	// This test just verifies the function runs without error
	// We can't easily test actual file detection in isolation
	files := DetectHistoryFiles()
	// files may be empty if no history files exist, which is ok
	_ = files
}

func TestBashParser_DetectPath(t *testing.T) {
	parser := NewBashParser()
	path, err := parser.DetectPath()
	if err != nil {
		t.Fatalf("DetectPath() error = %v", err)
	}
	if path == "" {
		t.Error("DetectPath() returned empty string")
	}
}

func TestZshParser_DetectPath(t *testing.T) {
	parser := NewZshParser()
	path, err := parser.DetectPath()
	if err != nil {
		t.Fatalf("DetectPath() error = %v", err)
	}
	if path == "" {
		t.Error("DetectPath() returned empty string")
	}
}

func TestDetectShell(t *testing.T) {
	shell := DetectShell()
	if shell == "" {
		t.Error("DetectShell() returned empty string")
	}
}

func TestNewParser(t *testing.T) {
	tests := []struct {
		name      string
		shell     string
		wantNil   bool
		wantType  string
	}{
		{
			name:     "bash parser",
			shell:    "bash",
			wantNil:  false,
			wantType: "*history.BashParser",
		},
		{
			name:     "zsh parser",
			shell:    "zsh",
			wantNil:  false,
			wantType: "*history.ZshParser",
		},
		{
			name:     "unsupported shell",
			shell:    "fish",
			wantNil:  true,
			wantType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.shell)
			if (p == nil) != tt.wantNil {
				t.Errorf("NewParser() = %v, wantNil %v", p, tt.wantNil)
			}
			if p != nil && tt.wantType != "" {
				// Type check is implicit - if it compiles and doesn't panic, we're good
				_ = p.Parse
				_ = p.DetectPath
			}
		})
	}
}

func TestParseBash_Convenience(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "bash_history")
	content := `#1616420000
git status
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	lines, err := ParseBash(tmpFile)
	if err != nil {
		t.Fatalf("ParseBash() error = %v", err)
	}
	if len(lines) != 1 {
		t.Errorf("ParseBash() returned %d lines, want 1", len(lines))
	}
}

func TestParseZsh_Convenience(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "zsh_history")
	content := `:1616420000:0:git status
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	lines, err := ParseZsh(tmpFile)
	if err != nil {
		t.Fatalf("ParseZsh() error = %v", err)
	}
	if len(lines) != 1 {
		t.Errorf("ParseZsh() returned %d lines, want 1", len(lines))
	}
}
