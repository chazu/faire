package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultConfig verifies that default values are correctly set.
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		name     string
		got      any
		want     any
		optional bool // if true, we only check it's not empty
	}{
		// Repo section defaults
		{"repo.path", cfg.Repo.Path, filepath.Join(os.Getenv("HOME"), ".local", "share", "gitsavvy", "repo"), false},
		{"repo.remote", cfg.Repo.Remote, "origin", false},
		{"repo.branch", cfg.Repo.Branch, "main", false},
		{"repo.sync_strategy", cfg.Repo.SyncStrategy, "rebase", false},
		{"repo.auto_reindex", cfg.Repo.AutoReindex, true, false},

		// Identity section defaults
		{"identity.path", cfg.Identity.Path, "", false}, // Empty - must be set by user
		{"identity.mode", cfg.Identity.Mode, "pr", false},
		{"identity.team_prefix", cfg.Identity.TeamPrefix, "", false},

		// Git section defaults
		{"git.author_name", cfg.Git.AuthorName, "", true}, // Non-empty
		{"git.author_email", cfg.Git.AuthorEmail, "", true}, // Non-empty
		{"git.sign_commits", cfg.Git.SignCommits, false, false},
		{"git.push_on_save", cfg.Git.PushOnSave, false, false},
		{"git.pr_base_branch", cfg.Git.PRBaseBranch, "main", false},
		{"git.feature_branch_template", cfg.Git.FeatureBranchTemplate, "gitsavvy/{identity}/{date}/{slug}", false},

		// Workflows section defaults
		{"workflows.root", cfg.Workflows.Root, "workflows", false},
		{"workflows.shared_root", cfg.Workflows.SharedRoot, "shared", false},
		{"workflows.draft_root", cfg.Workflows.DraftRoot, "drafts", false},
		{"workflows.index_path", cfg.Workflows.IndexPath, ".gitsavvy/index.json", false},
		{"workflows.schema_version", cfg.Workflows.SchemaVersion, 1, false},

		// Runner section defaults
		{"runner.default_shell", cfg.Runner.DefaultShell, "", true}, // Non-empty, depends on $SHELL
		{"runner.confirm_each_step", cfg.Runner.ConfirmEachStep, true, false},
		{"runner.stream_output", cfg.Runner.StreamOutput, true, false},
		{"runner.max_output_lines", cfg.Runner.MaxOutputLines, 5000, false},
		{"runner.dangerous_command_warnings", cfg.Runner.DangerousCommandWarnings, true, false},

		// Placeholders section defaults
		{"placeholders.prompt_style", cfg.Placeholders.PromptStyle, "form", false},
		{"placeholders.save_defaults", cfg.Placeholders.SaveDefaults, "none", false},
		{"placeholders.keychain_service", cfg.Placeholders.KeychainService, "gitsavvy", false},

		// TUI section defaults
		{"tui.enabled", cfg.TUI.Enabled, true, false},
		{"tui.theme", cfg.TUI.Theme, "default", false},
		{"tui.show_help", cfg.TUI.ShowHelp, true, false},

		// Editor section defaults
		{"editor.command", cfg.Editor.Command, "", false}, // Empty - uses $EDITOR

		// AI section defaults
		{"ai.enabled", cfg.AI.Enabled, false, false},
		{"ai.provider", cfg.AI.Provider, "openai_compat", false},
		{"ai.base_url", cfg.AI.BaseURL, "", false},
		{"ai.model", cfg.AI.Model, "", false},
		{"ai.api_key_env", cfg.AI.APIKeyEnv, "", false},
		{"ai.redact", cfg.AI.Redact, "basic", false},
		{"ai.confirm_send", cfg.AI.ConfirmSend, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.optional {
				// For optional fields, just check they're non-empty
				switch v := tt.got.(type) {
				case string:
					if v == "" {
						t.Errorf("expected non-empty value")
					}
				default:
					// Skip non-string optional checks
				}
			} else {
				if tt.got != tt.want {
					t.Errorf("got %v, want %v", tt.got, tt.want)
				}
			}
		})
	}
}

// TestValidate_ValidConfig tests that a valid config passes validation.
func TestValidate_ValidConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Identity.Path = "testuser"

	if err := cfg.Validate(); err != nil {
		t.Errorf("valid config failed validation: %v", err)
	}
}

// TestValidate_EmptyRequiredFields tests that empty required fields fail validation.
func TestValidate_EmptyRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "empty repo.path",
			mutate: func(c *Config) { c.Repo.Path = "" },
			wantErr: "repo.path cannot be empty",
		},
		{
			name: "empty repo.remote",
			mutate: func(c *Config) { c.Repo.Remote = "" },
			wantErr: "repo.remote cannot be empty",
		},
		{
			name: "empty repo.branch",
			mutate: func(c *Config) { c.Repo.Branch = "" },
			wantErr: "repo.branch cannot be empty",
		},
		{
			name: "invalid sync_strategy",
			mutate: func(c *Config) { c.Repo.SyncStrategy = "invalid" },
			wantErr: "repo.sync_strategy must be one of",
		},
		{
			name: "empty identity.path",
			mutate: func(c *Config) { c.Identity.Path = "" },
			wantErr: "identity.path cannot be empty",
		},
		{
			name: "identity.path contains ..",
			mutate: func(c *Config) { c.Identity.Path = "../etc" },
			wantErr: "cannot contain '..'",
		},
		{
			name: "identity.path is absolute",
			mutate: func(c *Config) { c.Identity.Path = "/etc/passwd" },
			wantErr: "cannot be an absolute path",
		},
		{
			name: "invalid identity.mode",
			mutate: func(c *Config) { c.Identity.Mode = "invalid" },
			wantErr: "identity.mode must be one of",
		},
		{
			name: "empty git.author_name",
			mutate: func(c *Config) { c.Git.AuthorName = "" },
			wantErr: "git.author_name cannot be empty",
		},
		{
			name: "empty git.author_email",
			mutate: func(c *Config) { c.Git.AuthorEmail = "" },
			wantErr: "git.author_email cannot be empty",
		},
		{
			name: "empty git.pr_base_branch",
			mutate: func(c *Config) { c.Git.PRBaseBranch = "" },
			wantErr: "git.pr_base_branch cannot be empty",
		},
		{
			name: "empty workflows.root",
			mutate: func(c *Config) { c.Workflows.Root = "" },
			wantErr: "workflows.root cannot be empty",
		},
		{
			name: "empty workflows.shared_root",
			mutate: func(c *Config) { c.Workflows.SharedRoot = "" },
			wantErr: "workflows.shared_root cannot be empty",
		},
		{
			name: "empty workflows.draft_root",
			mutate: func(c *Config) { c.Workflows.DraftRoot = "" },
			wantErr: "workflows.draft_root cannot be empty",
		},
		{
			name: "empty workflows.index_path",
			mutate: func(c *Config) { c.Workflows.IndexPath = "" },
			wantErr: "workflows.index_path cannot be empty",
		},
		{
			name: "invalid schema_version",
			mutate: func(c *Config) { c.Workflows.SchemaVersion = 0 },
			wantErr: "workflows.schema_version must be >= 1",
		},
		{
			name: "invalid default_shell",
			mutate: func(c *Config) { c.Runner.DefaultShell = "invalid" },
			wantErr: "runner.default_shell must be one of",
		},
		{
			name: "negative max_output_lines",
			mutate: func(c *Config) { c.Runner.MaxOutputLines = -1 },
			wantErr: "runner.max_output_lines must be >= 0",
		},
		{
			name: "invalid prompt_style",
			mutate: func(c *Config) { c.Placeholders.PromptStyle = "invalid" },
			wantErr: "placeholders.prompt_style must be one of",
		},
		{
			name: "invalid save_defaults",
			mutate: func(c *Config) { c.Placeholders.SaveDefaults = "invalid" },
			wantErr: "placeholders.save_defaults must be one of",
		},
		{
			name: "empty keychain_service",
			mutate: func(c *Config) { c.Placeholders.KeychainService = "" },
			wantErr: "placeholders.keychain_service cannot be empty",
		},
		{
			name: "empty tui.theme",
			mutate: func(c *Config) { c.TUI.Theme = "" },
			wantErr: "tui.theme cannot be empty",
		},
		{
			name: "ai.enabled but empty provider",
			mutate: func(c *Config) {
				c.AI.Enabled = true
				c.AI.Provider = ""
			},
			wantErr: "ai.provider cannot be empty when ai.enabled is true",
		},
		{
			name: "ai.enabled but empty model",
			mutate: func(c *Config) {
				c.AI.Enabled = true
				c.AI.Provider = "openai"
				c.AI.Model = ""
			},
			wantErr: "ai.model cannot be empty when ai.enabled is true",
		},
		{
			name: "ai.enabled but empty api_key_env",
			mutate: func(c *Config) {
				c.AI.Enabled = true
				c.AI.Provider = "openai"
				c.AI.Model = "gpt-4"
				c.AI.APIKeyEnv = ""
			},
			wantErr: "ai.api_key_env cannot be empty when ai.enabled is true",
		},
		{
			name: "invalid redact level",
			mutate: func(c *Config) { c.AI.Redact = "invalid" },
			wantErr: "ai.redact must be one of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Identity.Path = "testuser" // Set required field
			tt.mutate(cfg)

			err := cfg.Validate()
			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.wantErr)
				return
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want error containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// TestValidate_ValidValues tests that valid enum values pass validation.
func TestValidate_ValidValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{
			name: "sync_strategy ff-only",
			mutate: func(c *Config) { c.Repo.SyncStrategy = "ff-only" },
		},
		{
			name: "sync_strategy rebase",
			mutate: func(c *Config) { c.Repo.SyncStrategy = "rebase" },
		},
		{
			name: "sync_strategy merge",
			mutate: func(c *Config) { c.Repo.SyncStrategy = "merge" },
		},
		{
			name: "identity.mode direct",
			mutate: func(c *Config) { c.Identity.Mode = "direct" },
		},
		{
			name: "identity.mode pr",
			mutate: func(c *Config) { c.Identity.Mode = "pr" },
		},
		{
			name: "shell bash",
			mutate: func(c *Config) { c.Runner.DefaultShell = "bash" },
		},
		{
			name: "shell zsh",
			mutate: func(c *Config) { c.Runner.DefaultShell = "zsh" },
		},
		{
			name: "shell sh",
			mutate: func(c *Config) { c.Runner.DefaultShell = "sh" },
		},
		{
			name: "shell pwsh",
			mutate: func(c *Config) { c.Runner.DefaultShell = "pwsh" },
		},
		{
			name: "prompt_style form",
			mutate: func(c *Config) { c.Placeholders.PromptStyle = "form" },
		},
		{
			name: "prompt_style per-step",
			mutate: func(c *Config) { c.Placeholders.PromptStyle = "per-step" },
		},
		{
			name: "save_defaults none",
			mutate: func(c *Config) { c.Placeholders.SaveDefaults = "none" },
		},
		{
			name: "save_defaults keychain",
			mutate: func(c *Config) { c.Placeholders.SaveDefaults = "keychain" },
		},
		{
			name: "save_defaults file",
			mutate: func(c *Config) { c.Placeholders.SaveDefaults = "file" },
		},
		{
			name: "redact none",
			mutate: func(c *Config) { c.AI.Redact = "none" },
		},
		{
			name: "redact basic",
			mutate: func(c *Config) { c.AI.Redact = "basic" },
		},
		{
			name: "redact strict",
			mutate: func(c *Config) { c.AI.Redact = "strict" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Identity.Path = "testuser" // Set required field
			tt.mutate(cfg)

			if err := cfg.Validate(); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidate_IdentityPaths tests various identity path validations.
func TestValidate_IdentityPaths(t *testing.T) {
	tests := []struct {
		name      string
		identity  string
		wantValid bool
	}{
		{"simple username", "chaz", true},
		{"nested path", "platform/chaz", true},
		{"deeply nested", "org/team/subteam/user", true},
		{"path with dot", "chaz.test", true},
		{"path with dash", "chaz-test", true},
		{"path with underscore", "chaz_test", true},
		{"path traversal", "../etc", false},
		{"path traversal middle", "team/../user", false},
		{"absolute path", "/etc/passwd", false},
		{"absolute windows path", "C:\\Windows", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Identity.Path = tt.identity

			err := cfg.Validate()
			gotValid := err == nil

			if gotValid != tt.wantValid {
				t.Errorf("Validate() for path %q: valid=%v, want valid=%v (err: %v)",
					tt.identity, gotValid, tt.wantValid, err)
			}
		})
	}
}

// TestValidate_AIEnabled tests AI-specific validation when enabled.
func TestValidate_AIEnabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Identity.Path = "testuser"
	cfg.AI.Enabled = true
	cfg.AI.Provider = "openai"
	cfg.AI.Model = "gpt-4"
	cfg.AI.APIKeyEnv = "OPENAI_API_KEY"

	if err := cfg.Validate(); err != nil {
		t.Errorf("valid AI config failed validation: %v", err)
	}

	// Test with missing fields
	cfg.AI.Provider = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected error when ai.enabled but provider is empty")
	}
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findInString(s, substr)))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
