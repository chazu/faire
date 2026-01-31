// Package config provides configuration management for git-savvy.
//
// This file contains config loading functionality including:
// - XDG config path detection
// - TOML file parsing
// - Environment variable overrides
// - Validation
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// DetectConfigPath searches for a config file using XDG standard paths.
// Returns the first config file found, or empty string if none exists.
//
// Search order:
// 1. ~/.config/svf/config.toml
//
// Returns empty string if no config file is found (caller should use defaults).
func DetectConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Check ~/.config/svf/config.toml
	configPath := filepath.Join(homeDir, ".config", "svf", "config.toml")
	if _, err := os.Stat(configPath); err == nil {
		return configPath
	}

	return ""
}

// Load loads a config from the specified path.
// If the file doesn't exist, returns an error.
// After loading, applies environment variable overrides and validates.
func Load(path string) (*Config, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", path)
	}

	// Read file contents
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	// Start with defaults
	cfg := DefaultConfig()

	// Parse TOML
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(cfg)

	// Expand tilde in paths
	expandPath(cfg)

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// LoadWithDefaults attempts to load a config from XDG standard paths.
// If no config file is found, returns a config with all default values.
// If a config file is found but fails to load/validate, returns an error.
func LoadWithDefaults() (*Config, error) {
	configPath := DetectConfigPath()
	if configPath == "" {
		// No config file found, return defaults
		cfg := DefaultConfig()
		applyEnvOverrides(cfg)
		expandPath(cfg)

		// Note: We don't validate here because defaults may have
		// intentionally empty fields (like identity.path) that users
		// must set. The caller should validate when appropriate.
		return cfg, nil
	}

	return Load(configPath)
}

// applyEnvOverrides applies environment variable overrides to the config.
// Environment variables follow the pattern: GITSAVVY_<SECTION>_<FIELD>
//
// Examples:
// - GITSAVVY_REPO_PATH overrides [repo].path
// - GITSAVVY_IDENTITY_PATH overrides [identity].path
// - GITSAVVY_REPO_BRANCH overrides [repo].branch
//
// Boolean fields: use "true"/"false" strings
// Array fields: comma-separated values (not yet implemented in schema)
func applyEnvOverrides(c *Config) {
	// Helper to lookup and apply string override
	applyString := func(key string, target *string) {
		if val, ok := os.LookupEnv(key); ok && val != "" {
			*target = val
		}
	}

	// Helper to lookup and apply bool override
	applyBool := func(key string, target *bool) {
		if val, ok := os.LookupEnv(key); ok && val != "" {
			switch strings.ToLower(val) {
			case "true", "1", "yes", "on":
				*target = true
			case "false", "0", "no", "off":
				*target = false
			}
		}
	}

	// Helper to lookup and apply int override
	applyInt := func(key string, target *int) {
		if val, ok := os.LookupEnv(key); ok && val != "" {
			var i int
			if _, err := fmt.Sscanf(val, "%d", &i); err == nil {
				*target = i
			}
		}
	}

	// Repo section
	applyString("GITSAVVY_REPO_PATH", &c.Repo.Path)
	applyString("GITSAVVY_REPO_REMOTE", &c.Repo.Remote)
	applyString("GITSAVVY_REPO_BRANCH", &c.Repo.Branch)
	applyString("GITSAVVY_REPO_SYNC_STRATEGY", &c.Repo.SyncStrategy)
	applyBool("GITSAVVY_REPO_AUTO_REINDEX", &c.Repo.AutoReindex)

	// Identity section
	applyString("GITSAVVY_IDENTITY_PATH", &c.Identity.Path)
	applyString("GITSAVVY_IDENTITY_MODE", &c.Identity.Mode)
	applyString("GITSAVVY_IDENTITY_TEAM_PREFIX", &c.Identity.TeamPrefix)

	// Git section
	applyString("GITSAVVY_GIT_AUTHOR_NAME", &c.Git.AuthorName)
	applyString("GITSAVVY_GIT_AUTHOR_EMAIL", &c.Git.AuthorEmail)
	applyBool("GITSAVVY_GIT_SIGN_COMMITS", &c.Git.SignCommits)
	applyBool("GITSAVVY_GIT_PUSH_ON_SAVE", &c.Git.PushOnSave)
	applyString("GITSAVVY_GIT_PR_BASE_BRANCH", &c.Git.PRBaseBranch)
	applyString("GITSAVVY_GIT_FEATURE_BRANCH_TEMPLATE", &c.Git.FeatureBranchTemplate)

	// Workflows section
	applyString("GITSAVVY_WORKFLOWS_ROOT", &c.Workflows.Root)
	applyString("GITSAVVY_WORKFLOWS_SHARED_ROOT", &c.Workflows.SharedRoot)
	applyString("GITSAVVY_WORKFLOWS_DRAFT_ROOT", &c.Workflows.DraftRoot)
	applyString("GITSAVVY_WORKFLOWS_INDEX_PATH", &c.Workflows.IndexPath)
	applyInt("GITSAVVY_WORKFLOWS_SCHEMA_VERSION", &c.Workflows.SchemaVersion)

	// Runner section
	applyString("GITSAVVY_RUNNER_DEFAULT_SHELL", &c.Runner.DefaultShell)
	applyBool("GITSAVVY_RUNNER_CONFIRM_EACH_STEP", &c.Runner.ConfirmEachStep)
	applyBool("GITSAVVY_RUNNER_STREAM_OUTPUT", &c.Runner.StreamOutput)
	applyInt("GITSAVVY_RUNNER_MAX_OUTPUT_LINES", &c.Runner.MaxOutputLines)
	applyBool("GITSAVVY_RUNNER_DANGEROUS_COMMAND_WARNINGS", &c.Runner.DangerousCommandWarnings)

	// Placeholders section
	applyString("GITSAVVY_PLACEHOLDERS_PROMPT_STYLE", &c.Placeholders.PromptStyle)
	applyString("GITSAVVY_PLACEHOLDERS_SAVE_DEFAULTS", &c.Placeholders.SaveDefaults)
	applyString("GITSAVVY_PLACEHOLDERS_KEYCHAIN_SERVICE", &c.Placeholders.KeychainService)

	// TUI section
	applyBool("GITSAVVY_TUI_ENABLED", &c.TUI.Enabled)
	applyString("GITSAVVY_TUI_THEME", &c.TUI.Theme)
	applyBool("GITSAVVY_TUI_SHOW_HELP", &c.TUI.ShowHelp)

	// Editor section
	applyString("GITSAVVY_EDITOR_COMMAND", &c.Editor.Command)

	// AI section
	applyBool("GITSAVVY_AI_ENABLED", &c.AI.Enabled)
	applyString("GITSAVVY_AI_PROVIDER", &c.AI.Provider)
	applyString("GITSAVVY_AI_BASE_URL", &c.AI.BaseURL)
	applyString("GITSAVVY_AI_MODEL", &c.AI.Model)
	applyString("GITSAVVY_AI_API_KEY_ENV", &c.AI.APIKeyEnv)
	applyString("GITSAVVY_AI_REDACT", &c.AI.Redact)
	applyBool("GITSAVVY_AI_CONFIRM_SEND", &c.AI.ConfirmSend)
}

// expandPath expands ~ to the home directory in the repo path.
func expandPath(c *Config) {
	if strings.HasPrefix(c.Repo.Path, "~/") || c.Repo.Path == "~" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			c.Repo.Path = filepath.Join(homeDir, strings.TrimPrefix(c.Repo.Path, "~/"))
		}
	}
}
