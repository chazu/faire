// Package config provides configuration management for git-savvy.
//
// The configuration is stored in TOML format and supports validation
// and default values for all fields.
package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// Config is the top-level configuration struct for git-savvy.
// It contains all configuration sections as embedded structs.
type Config struct {
	Repo        RepoConfig        `toml:"repo"`
	Identity    IdentityConfig    `toml:"identity"`
	Git         GitConfig         `toml:"git"`
	Workflows   WorkflowsConfig   `toml:"workflows"`
	Runner      RunnerConfig      `toml:"runner"`
	Placeholders PlaceholdersConfig `toml:"placeholders"`
	TUI         TUIConfig         `toml:"tui"`
	Editor      EditorConfig      `toml:"editor"`
	AI          AIConfig          `toml:"ai"`
}

// RepoConfig contains repository-related settings.
type RepoConfig struct {
	// Path is the local filesystem path to the git repository.
	Path string `toml:"path"`

	// Remote is the git remote name (default: "origin").
	Remote string `toml:"remote"`

	// Branch is the default branch name (default: "main", auto-detected if possible).
	Branch string `toml:"branch"`

	// SyncStrategy determines how to integrate remote changes.
	// Valid values: "ff-only", "rebase", "merge".
	SyncStrategy string `toml:"sync_strategy"`

	// AutoReindex controls whether to rebuild the index after sync.
	AutoReindex bool `toml:"auto_reindex"`
}

// IdentityConfig contains user identity settings.
type IdentityConfig struct {
	// Path is the claimed path within the repo (e.g., "chaz" or "platform/chaz").
	// All workflow writes are under workflows/<Path>/.
	Path string `toml:"path"`

	// Mode determines the write mode.
	// Valid values: "direct", "pr".
	Mode string `toml:"mode"`

	// TeamPrefix is an optional helper for validation/UI.
	TeamPrefix string `toml:"team_prefix"`
}

// GitConfig contains git-specific settings.
type GitConfig struct {
	// AuthorName is the default git author name.
	AuthorName string `toml:"author_name"`

	// AuthorEmail is the default git author email.
	AuthorEmail string `toml:"author_email"`

	// SignCommits enables GPG signing of commits.
	SignCommits bool `toml:"sign_commits"`

	// PushOnSave controls whether to push after saving in direct mode.
	PushOnSave bool `toml:"push_on_save"`

	// PRBaseBranch is the base branch for PRs in PR mode.
	PRBaseBranch string `toml:"pr_base_branch"`

	// FeatureBranchTemplate is the template for feature branch names.
	// Supported placeholders: {identity}, {date}, {slug}.
	FeatureBranchTemplate string `toml:"feature_branch_template"`
}

// WorkflowsConfig contains workflow-related settings.
type WorkflowsConfig struct {
	// Root is the repo-relative path to user workflows.
	Root string `toml:"root"`

	// SharedRoot is the repo-relative path to shared workflows.
	SharedRoot string `toml:"shared_root"`

	// DraftRoot is the repo-relative path to draft workflows.
	DraftRoot string `toml:"draft_root"`

	// IndexPath is the repo-relative path to the index file.
	IndexPath string `toml:"index_path"`

	// SchemaVersion is the workflow schema version.
	SchemaVersion int `toml:"schema_version"`

	// Index contains search index settings.
	Index IndexConfig `toml:"index"`
}

// IndexConfig contains search index settings.
type IndexConfig struct {
	// AutoRebuild controls whether to automatically rebuild the index after sync.
	AutoRebuild bool `toml:"auto_rebuild"`
}

// RunnerConfig contains workflow runner settings.
type RunnerConfig struct {
	// DefaultShell is the default shell for running steps.
	// Valid values: "bash", "zsh", "sh", "pwsh".
	DefaultShell string `toml:"default_shell"`

	// ConfirmEachStep controls whether to prompt before each step.
	ConfirmEachStep bool `toml:"confirm_each_step"`

	// StreamOutput controls whether to stream command output.
	StreamOutput bool `toml:"stream_output"`

	// MaxOutputLines is the maximum number of output lines to keep in memory.
	MaxOutputLines int `toml:"max_output_lines"`

	// DangerousCommandWarnings enables warnings for potentially dangerous commands.
	DangerousCommandWarnings bool `toml:"dangerous_command_warnings"`
}

// PlaceholdersConfig contains placeholder/parameter settings.
type PlaceholdersConfig struct {
	// PromptStyle determines how placeholders are prompted.
	// Valid values: "form", "per-step".
	PromptStyle string `toml:"prompt_style"`

	// SaveDefaults controls how to save default placeholder values.
	// Valid values: "none", "keychain", "file".
	SaveDefaults string `toml:"save_defaults"`

	// KeychainService is the service name for keychain storage.
	KeychainService string `toml:"keychain_service"`
}

// TUIConfig contains terminal UI settings.
type TUIConfig struct {
	// Enabled controls whether to use the TUI (when false, falls back to CLI).
	Enabled bool `toml:"enabled"`

	// Theme is the TUI theme name.
	Theme string `toml:"theme"`

	// ShowHelp controls whether to show the help panel by default.
	ShowHelp bool `toml:"show_help"`
}

// EditorConfig contains editor settings.
type EditorConfig struct {
	// Command is the editor command to use (if unset, uses $EDITOR).
	Command string `toml:"command"`
}

// AIConfig contains AI-related settings.
type AIConfig struct {
	// Enabled enables AI features (must be explicitly enabled).
	Enabled bool `toml:"enabled"`

	// Provider is the AI provider name.
	Provider string `toml:"provider"`

	// BaseURL is the base URL for API requests (optional for compatibility).
	BaseURL string `toml:"base_url"`

	// Model is the AI model identifier.
	Model string `toml:"model"`

	// APIKeyEnv is the environment variable name containing the API key.
	APIKeyEnv string `toml:"api_key_env"`

	// Redact controls the level of redaction for privacy.
	// Valid values: "none", "basic", "strict".
	Redact string `toml:"redact"`

	// ConfirmSend prompts for confirmation before sending data to AI.
	ConfirmSend bool `toml:"confirm_send"`
}

// DefaultConfig returns a Config with all default values set.
func DefaultConfig() *Config {
	usr, _ := user.Current()
	homeDir := ""
	if usr != nil {
		homeDir = usr.HomeDir
	}

	// Detect default shell from environment
	defaultShell := os.Getenv("SHELL")
	if defaultShell == "" {
		defaultShell = "zsh"
	} else {
		// Extract shell name from path (e.g., /bin/zsh -> zsh)
		defaultShell = filepath.Base(defaultShell)
	}

	return &Config{
		Repo: RepoConfig{
			Path:         filepath.Join(homeDir, ".local", "share", "svf", "repo"),
			Remote:       "origin",
			Branch:       "main",
			SyncStrategy: "rebase",
			AutoReindex:  true,
		},
		Identity: IdentityConfig{
			Path:       "",
			Mode:       "pr",
			TeamPrefix: "",
		},
		Git: GitConfig{
			AuthorName:           gitUserName(),
			AuthorEmail:          gitUserEmail(),
			SignCommits:          false,
			PushOnSave:           false,
			PRBaseBranch:         "main",
			FeatureBranchTemplate: "svf/{identity}/{date}/{slug}",
		},
		Workflows: WorkflowsConfig{
			Root:         "workflows",
			SharedRoot:   "shared",
			DraftRoot:    "drafts",
			IndexPath:    ".svf/index.json",
			SchemaVersion: 1,
			Index: IndexConfig{
				AutoRebuild: true,
			},
		},
		Runner: RunnerConfig{
			DefaultShell:             defaultShell,
			ConfirmEachStep:          true,
			StreamOutput:             true,
			MaxOutputLines:           5000,
			DangerousCommandWarnings: true,
		},
		Placeholders: PlaceholdersConfig{
			PromptStyle:      "form",
			SaveDefaults:     "none",
			KeychainService:  "svf",
		},
		TUI: TUIConfig{
			Enabled:  true,
			Theme:    "default",
			ShowHelp: true,
		},
		Editor: EditorConfig{
			Command: "",
		},
		AI: AIConfig{
			Enabled:    false,
			Provider:   "openai_compat",
			BaseURL:    "",
			Model:      "",
			APIKeyEnv:  "",
			Redact:     "basic",
			ConfirmSend: true,
		},
	}
}

// Validate checks the configuration for valid values.
// Returns a nil error if the config is valid, or an error describing the problem.
func (c *Config) Validate() error {
	// Validate Repo section
	if c.Repo.Path == "" {
		return fmt.Errorf("repo.path cannot be empty")
	}
	if c.Repo.Remote == "" {
		return fmt.Errorf("repo.remote cannot be empty")
	}
	if c.Repo.Branch == "" {
		return fmt.Errorf("repo.branch cannot be empty")
	}
	validSyncStrategies := map[string]bool{
		"ff-only": true,
		"rebase":  true,
		"merge":   true,
	}
	if !validSyncStrategies[c.Repo.SyncStrategy] {
		return fmt.Errorf("repo.sync_strategy must be one of: ff-only, rebase, merge; got %q", c.Repo.SyncStrategy)
	}

	// Validate Identity section
	if c.Identity.Path == "" {
		return fmt.Errorf("identity.path cannot be empty")
	}
	// Check for path traversal
	if strings.Contains(c.Identity.Path, "..") {
		return fmt.Errorf("identity.path cannot contain '..': %q", c.Identity.Path)
	}
	if filepath.IsAbs(c.Identity.Path) || isWindowsAbsPath(c.Identity.Path) {
		return fmt.Errorf("identity.path cannot be an absolute path: %q", c.Identity.Path)
	}
	validIdentityModes := map[string]bool{
		"direct": true,
		"pr":     true,
	}
	if !validIdentityModes[c.Identity.Mode] {
		return fmt.Errorf("identity.mode must be one of: direct, pr; got %q", c.Identity.Mode)
	}

	// Validate Git section
	if c.Git.AuthorName == "" {
		return fmt.Errorf("git.author_name cannot be empty")
	}
	if c.Git.AuthorEmail == "" {
		return fmt.Errorf("git.author_email cannot be empty")
	}
	if c.Git.PRBaseBranch == "" {
		return fmt.Errorf("git.pr_base_branch cannot be empty")
	}
	if c.Git.FeatureBranchTemplate == "" {
		return fmt.Errorf("git.feature_branch_template cannot be empty")
	}

	// Validate Workflows section
	if c.Workflows.Root == "" {
		return fmt.Errorf("workflows.root cannot be empty")
	}
	if c.Workflows.SharedRoot == "" {
		return fmt.Errorf("workflows.shared_root cannot be empty")
	}
	if c.Workflows.DraftRoot == "" {
		return fmt.Errorf("workflows.draft_root cannot be empty")
	}
	if c.Workflows.IndexPath == "" {
		return fmt.Errorf("workflows.index_path cannot be empty")
	}
	if c.Workflows.SchemaVersion < 1 {
		return fmt.Errorf("workflows.schema_version must be >= 1; got %d", c.Workflows.SchemaVersion)
	}

	// Validate Runner section
	validShells := map[string]bool{
		"bash": true,
		"zsh":  true,
		"sh":   true,
		"pwsh": true,
	}
	if !validShells[c.Runner.DefaultShell] {
		return fmt.Errorf("runner.default_shell must be one of: bash, zsh, sh, pwsh; got %q", c.Runner.DefaultShell)
	}
	if c.Runner.MaxOutputLines < 0 {
		return fmt.Errorf("runner.max_output_lines must be >= 0; got %d", c.Runner.MaxOutputLines)
	}

	// Validate Placeholders section
	validPromptStyles := map[string]bool{
		"form":     true,
		"per-step": true,
	}
	if !validPromptStyles[c.Placeholders.PromptStyle] {
		return fmt.Errorf("placeholders.prompt_style must be one of: form, per-step; got %q", c.Placeholders.PromptStyle)
	}
	validSaveDefaults := map[string]bool{
		"none":     true,
		"keychain": true,
		"file":     true,
	}
	if !validSaveDefaults[c.Placeholders.SaveDefaults] {
		return fmt.Errorf("placeholders.save_defaults must be one of: none, keychain, file; got %q", c.Placeholders.SaveDefaults)
	}
	if c.Placeholders.KeychainService == "" {
		return fmt.Errorf("placeholders.keychain_service cannot be empty")
	}

	// Validate TUI section
	if c.TUI.Theme == "" {
		return fmt.Errorf("tui.theme cannot be empty")
	}

	// Validate AI section (only if enabled)
	if c.AI.Enabled {
		if c.AI.Provider == "" {
			return fmt.Errorf("ai.provider cannot be empty when ai.enabled is true")
		}
		if c.AI.Model == "" {
			return fmt.Errorf("ai.model cannot be empty when ai.enabled is true")
		}
		if c.AI.APIKeyEnv == "" {
			return fmt.Errorf("ai.api_key_env cannot be empty when ai.enabled is true")
		}
	}
	validRedactLevels := map[string]bool{
		"none":   true,
		"basic":  true,
		"strict": true,
	}
	if !validRedactLevels[c.AI.Redact] {
		return fmt.Errorf("ai.redact must be one of: none, basic, strict; got %q", c.AI.Redact)
	}

	return nil
}

// isWindowsAbsPath detects Windows-style absolute paths (e.g., "C:\Windows", "D:\path").
func isWindowsAbsPath(path string) bool {
	if len(path) < 3 {
		return false
	}
	// Check for drive letter pattern: X:\ or X:/
	return (path[0] >= 'A' && path[0] <= 'Z' || path[0] >= 'a' && path[0] <= 'z') &&
		path[1] == ':' &&
		(path[2] == '\\' || path[2] == '/')
}

// gitUserName returns the git user.name or the current username.
func gitUserName() string {
	// Try to get from git config
	if runtime.GOOS != "windows" {
		return fallbackUserName()
	}
	return fallbackUserName()
}

func fallbackUserName() string {
	if usr, err := user.Current(); err == nil && usr.Name != "" {
		return usr.Name
	}
	if usr, err := user.Current(); err == nil && usr.Username != "" {
		return usr.Username
	}
	return "Git Savvy User"
}

// gitUserEmail returns the git user.email or a default.
func gitUserEmail() string {
	if usr, err := user.Current(); err == nil && usr.Username != "" {
		hostname, _ := os.Hostname()
		if hostname != "" {
			return fmt.Sprintf("%s@%s", usr.Username, hostname)
		}
		return fmt.Sprintf("%s@localhost", usr.Username)
	}
	return "svf@localhost"
}
