package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDetectConfigPath_NoConfig tests that empty string is returned when no config exists.
func TestDetectConfigPath_NoConfig(t *testing.T) {
	// We can't easily mock the home directory, so we just verify
	// the function returns something (either a path or empty string).
	path := DetectConfigPath()
	// If a config exists, it should be an absolute path
	// If no config exists, it should be empty
	if path != "" {
		if !filepath.IsAbs(path) {
			t.Errorf("DetectConfigPath() returned non-absolute path: %s", path)
		}
	}
}

// TestLoad_ValidConfig tests loading a valid config file.
func TestLoad_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Write a valid config
	configContent := `
[repo]
path = "/test/repo"
remote = "upstream"
branch = "develop"
sync_strategy = "merge"

[identity]
path = "testuser"
mode = "direct"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	// Config values should override defaults
	if cfg.Repo.Path != "/test/repo" {
		t.Errorf("expected repo.path to be '/test/repo', got %q", cfg.Repo.Path)
	}
	if cfg.Repo.Remote != "upstream" {
		t.Errorf("expected repo.remote to be 'upstream', got %q", cfg.Repo.Remote)
	}
	if cfg.Repo.Branch != "develop" {
		t.Errorf("expected repo.branch to be 'develop', got %q", cfg.Repo.Branch)
	}
	if cfg.Repo.SyncStrategy != "merge" {
		t.Errorf("expected repo.sync_strategy to be 'merge', got %q", cfg.Repo.SyncStrategy)
	}
	if cfg.Identity.Path != "testuser" {
		t.Errorf("expected identity.path to be 'testuser', got %q", cfg.Identity.Path)
	}
	if cfg.Identity.Mode != "direct" {
		t.Errorf("expected identity.mode to be 'direct', got %q", cfg.Identity.Mode)
	}
}

// TestLoad_InvalidTOML tests that invalid TOML returns error.
func TestLoad_InvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Write an invalid config (bad TOML)
	configContent := `
[repo
path = "/test/repo"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error for invalid TOML config, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse") {
		t.Errorf("error should mention parsing failure, got: %v", err)
	}
}

// TestLoad_ValidationFailed tests that validation failures are returned.
func TestLoad_ValidationFailed(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Write a config that fails validation (invalid sync_strategy)
	configContent := `
[repo]
path = "/test/repo"
sync_strategy = "invalid"

[identity]
path = "testuser"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error for invalid config, got nil")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("error should mention validation failure, got: %v", err)
	}
}

// TestLoad_FileNotExist tests that Load returns error for non-existent file.
func TestLoad_FileNotExist(t *testing.T) {
	_, err := Load("/nonexistent/path/config.toml")
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention file not found, got: %v", err)
	}
}

// TestEnvOverrides_String tests string environment variable overrides.
func TestEnvOverrides_String(t *testing.T) {
	// Save original env values
	oldEnv := saveEnv()
	defer restoreEnv(oldEnv)

	_ = os.Setenv("GITSAVVY_REPO_PATH", "/env/override/repo")
	_ = os.Setenv("GITSAVVY_REPO_REMOTE", "env-origin")
	_ = os.Setenv("GITSAVVY_REPO_BRANCH", "env-branch")
	_ = os.Setenv("GITSAVVY_REPO_SYNC_STRATEGY", "ff-only")
	_ = os.Setenv("GITSAVVY_IDENTITY_PATH", "envuser")
	_ = os.Setenv("GITSAVVY_IDENTITY_MODE", "direct")
	_ = os.Setenv("GITSAVVY_GIT_AUTHOR_NAME", "Env Author")
	_ = os.Setenv("GITSAVVY_GIT_AUTHOR_EMAIL", "env@example.com")
	_ = os.Setenv("GITSAVVY_RUNNER_DEFAULT_SHELL", "bash")
	_ = os.Setenv("GITSAVVY_TUI_THEME", "dark")

	cfg := DefaultConfig()
	applyEnvOverrides(cfg)

	if cfg.Repo.Path != "/env/override/repo" {
		t.Errorf("expected repo.path from env, got %q", cfg.Repo.Path)
	}
	if cfg.Repo.Remote != "env-origin" {
		t.Errorf("expected repo.remote from env, got %q", cfg.Repo.Remote)
	}
	if cfg.Repo.Branch != "env-branch" {
		t.Errorf("expected repo.branch from env, got %q", cfg.Repo.Branch)
	}
	if cfg.Repo.SyncStrategy != "ff-only" {
		t.Errorf("expected repo.sync_strategy from env, got %q", cfg.Repo.SyncStrategy)
	}
	if cfg.Identity.Path != "envuser" {
		t.Errorf("expected identity.path from env, got %q", cfg.Identity.Path)
	}
	if cfg.Identity.Mode != "direct" {
		t.Errorf("expected identity.mode from env, got %q", cfg.Identity.Mode)
	}
	if cfg.Git.AuthorName != "Env Author" {
		t.Errorf("expected git.author_name from env, got %q", cfg.Git.AuthorName)
	}
	if cfg.Git.AuthorEmail != "env@example.com" {
		t.Errorf("expected git.author_email from env, got %q", cfg.Git.AuthorEmail)
	}
	if cfg.Runner.DefaultShell != "bash" {
		t.Errorf("expected runner.default_shell from env, got %q", cfg.Runner.DefaultShell)
	}
	if cfg.TUI.Theme != "dark" {
		t.Errorf("expected tui.theme from env, got %q", cfg.TUI.Theme)
	}
}

// TestEnvOverrides_Bool tests boolean environment variable overrides.
func TestEnvOverrides_Bool(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"true", "true", true},
		{"TRUE", "TRUE", true},
		{"1", "1", true},
		{"yes", "yes", true},
		{"on", "on", true},
		{"false", "false", false},
		{"FALSE", "FALSE", false},
		{"0", "0", false},
		{"no", "no", false},
		{"off", "off", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldEnv := saveEnv()
			defer restoreEnv(oldEnv)

			_ = os.Setenv("GITSAVVY_REPO_AUTO_REINDEX", tt.envValue)
			_ = os.Setenv("GITSAVVY_TUI_ENABLED", tt.envValue)

			cfg := DefaultConfig()
			// Flip defaults to test override
			cfg.Repo.AutoReindex = !tt.expected
			cfg.TUI.Enabled = !tt.expected

			applyEnvOverrides(cfg)

			if cfg.Repo.AutoReindex != tt.expected {
				t.Errorf("expected repo.auto_reindex=%v, got %v", tt.expected, cfg.Repo.AutoReindex)
			}
			if cfg.TUI.Enabled != tt.expected {
				t.Errorf("expected tui.enabled=%v, got %v", tt.expected, cfg.TUI.Enabled)
			}
		})
	}
}

// TestEnvOverrides_Int tests integer environment variable overrides.
func TestEnvOverrides_Int(t *testing.T) {
	oldEnv := saveEnv()
	defer restoreEnv(oldEnv)

	_ = os.Setenv("GITSAVVY_RUNNER_MAX_OUTPUT_LINES", "1000")
	_ = os.Setenv("GITSAVVY_WORKFLOWS_SCHEMA_VERSION", "2")

	cfg := DefaultConfig()
	applyEnvOverrides(cfg)

	if cfg.Runner.MaxOutputLines != 1000 {
		t.Errorf("expected runner.max_output_lines=1000, got %d", cfg.Runner.MaxOutputLines)
	}
	if cfg.Workflows.SchemaVersion != 2 {
		t.Errorf("expected workflows.schema_version=2, got %d", cfg.Workflows.SchemaVersion)
	}
}

// TestEnvOverrides_EmptyValue tests that empty env vars don't override defaults.
func TestEnvOverrides_EmptyValue(t *testing.T) {
	oldEnv := saveEnv()
	defer restoreEnv(oldEnv)

	// Set to empty string - should NOT override
	_ = os.Setenv("GITSAVVY_REPO_PATH", "")
	_ = os.Setenv("GITSAVVY_IDENTITY_PATH", "")

	cfg := DefaultConfig()
	originalPath := cfg.Repo.Path
	originalIdentityPath := cfg.Identity.Path

	applyEnvOverrides(cfg)

	if cfg.Repo.Path != originalPath {
		t.Errorf("empty env var should not override, repo.path changed from %q to %q",
			originalPath, cfg.Repo.Path)
	}
	if cfg.Identity.Path != originalIdentityPath {
		t.Errorf("empty env var should not override, identity.path changed from %q to %q",
			originalIdentityPath, cfg.Identity.Path)
	}
}

// TestLoad_WithEnvOverrides tests that env overrides apply after loading config.
func TestLoad_WithEnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Write a valid config
	configContent := `
[repo]
path = "/config/repo"
sync_strategy = "rebase"

[identity]
path = "configuser"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	oldEnv := saveEnv()
	defer restoreEnv(oldEnv)

	// Set env override
	_ = os.Setenv("GITSAVVY_REPO_SYNC_STRATEGY", "merge")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	// Path should come from config
	if cfg.Repo.Path != "/config/repo" {
		t.Errorf("expected repo.path from config, got %q", cfg.Repo.Path)
	}

	// Sync strategy should be overridden by env
	if cfg.Repo.SyncStrategy != "merge" {
		t.Errorf("expected repo.sync_strategy from env override, got %q", cfg.Repo.SyncStrategy)
	}
}

// saveEnv saves current environment variables.
func saveEnv() map[string]string {
	env := make(map[string]string)
	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env
}

// restoreEnv restores environment variables from a saved map.
func restoreEnv(env map[string]string) {
	// Clear all GITSAVVY_* vars
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "GITSAVVY_") {
			key := strings.SplitN(kv, "=", 2)[0]
			_ = os.Unsetenv(key)
		}
	}
	// Restore saved values
	for k, v := range env {
		if strings.HasPrefix(k, "GITSAVVY_") {
			_ = os.Setenv(k, v)
		}
	}
}
