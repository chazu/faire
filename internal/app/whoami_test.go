package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWhoami(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
[repo]
path = "/Users/chaz/.local/share/gitsavvy/repo"
remote = "origin"
branch = "main"
sync_strategy = "rebase"
auto_reindex = true

[identity]
path = "platform/chaz"
mode = "pr"

[git]
author_name = "Chaz Straney"
author_email = "chaz@example.com"
sign_commits = false
push_on_save = false
pr_base_branch = "main"
feature_branch_template = "gitsavvy/{identity}/{date}/{slug}"

[workflows]
root = "workflows"
shared_root = "shared"
draft_root = "drafts"
index_path = ".gitsavvy/index.json"
schema_version = 1

[runner]
default_shell = "zsh"
confirm_each_step = true
stream_output = true
max_output_lines = 5000
dangerous_command_warnings = true

[placeholders]
prompt_style = "form"
save_defaults = "none"
keychain_service = "gitsavvy"

[tui]
enabled = true
theme = "default"
show_help = true

[editor]
command = ""

[ai]
enabled = false
provider = "openai_compat"
base_url = ""
model = ""
api_key_env = ""
redact = "basic"
confirm_send = true
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Test Whoami with valid config
	output, err := Whoami(configPath)
	if err != nil {
		t.Fatalf("Whoami() error = %v", err)
	}

	if output.RepoPath != "/Users/chaz/.local/share/gitsavvy/repo" {
		t.Errorf("RepoPath = %q, want %q", output.RepoPath, "/Users/chaz/.local/share/gitsavvy/repo")
	}

	if output.IdentityPath != "platform/chaz" {
		t.Errorf("IdentityPath = %q, want %q", output.IdentityPath, "platform/chaz")
	}

	if output.Mode != "pr" {
		t.Errorf("Mode = %q, want %q", output.Mode, "pr")
	}

	if output.Author.Name != "Chaz Straney" {
		t.Errorf("Author.Name = %q, want %q", output.Author.Name, "Chaz Straney")
	}

	if output.Author.Email != "chaz@example.com" {
		t.Errorf("Author.Email = %q, want %q", output.Author.Email, "chaz@example.com")
	}
}

func TestWhoami_MissingConfig(t *testing.T) {
	// Test Whoami with missing config file
	_, err := Whoami("/nonexistent/config.toml")
	if err == nil {
		t.Error("Whoami() expected error for missing config, got nil")
	}
}

func TestWhoami_JSONOutput(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
[repo]
path = "/test/repo"
remote = "origin"
branch = "main"
sync_strategy = "rebase"
auto_reindex = true

[identity]
path = "test/user"
mode = "direct"

[git]
author_name = "Test User"
author_email = "test@example.com"
sign_commits = false
push_on_save = false
pr_base_branch = "main"
feature_branch_template = "gitsavvy/{identity}/{date}/{slug}"

[workflows]
root = "workflows"
shared_root = "shared"
draft_root = "drafts"
index_path = ".gitsavvy/index.json"
schema_version = 1

[runner]
default_shell = "zsh"
confirm_each_step = true
stream_output = true
max_output_lines = 5000
dangerous_command_warnings = true

[placeholders]
prompt_style = "form"
save_defaults = "none"
keychain_service = "gitsavvy"

[tui]
enabled = true
theme = "default"
show_help = true

[editor]
command = ""

[ai]
enabled = false
provider = "openai_compat"
base_url = ""
model = ""
api_key_env = ""
redact = "basic"
confirm_send = true
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	output, err := Whoami(configPath)
	if err != nil {
		t.Fatalf("Whoami() error = %v", err)
	}

	// Marshal to JSON and verify it's valid
	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Unmarshal to verify structure
	var decoded WhoamiOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Author.Name != "Test User" {
		t.Errorf("Decoded Author.Name = %q, want %q", decoded.Author.Name, "Test User")
	}
}
