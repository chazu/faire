// Package app provides high-level application logic for git-savvy commands.
package app

import (
	"context"
	"fmt"
	"os"

	"github.com/chazuruo/faire/internal/config"
	"github.com/chazuruo/faire/internal/gitrepo"
)

// SyncOptions contains the options for the sync operation.
type SyncOptions struct {
	// ConfigPath is the path to the config file.
	ConfigPath string
	// Strategy is the integration strategy to use.
	// If empty, uses the strategy from config.
	Strategy gitrepo.IntegrationStrategy
	// Remote is the git remote name.
	// If empty, uses the remote from config.
	Remote string
	// Branch is the remote branch name.
	// If empty, uses the branch from config.
	Branch string
	// NoFetch skips the fetch step.
	NoFetch bool
	// NoPush skips pushing after successful sync.
	NoPush bool
	// Push pushes after successful sync (overrides NoPush).
	Push bool
	// JSONOutput enables JSON output format.
	JSONOutput bool
}

// SyncOutput contains the result of a sync operation.
type SyncOutput struct {
	// Success is true if sync completed without conflicts.
	Success bool `json:"success"`
	// Branch is the current branch name.
	Branch string `json:"branch"`
	// RemoteBranch is the remote branch reference.
	RemoteBranch string `json:"remote_branch"`
	// Strategy is the integration strategy that was used.
	Strategy gitrepo.IntegrationStrategy `json:"strategy"`
	// Ahead is the number of commits ahead of remote (before sync).
	Ahead int `json:"ahead"`
	// Behind is the number of commits behind remote (before sync).
	Behind int `json:"behind"`
	// NewFiles is the list of newly added files.
	NewFiles []string `json:"new_files,omitempty"`
	// UpdatedFiles is the list of modified files.
	UpdatedFiles []string `json:"updated_files,omitempty"`
	// DeletedFiles is the list of removed files.
	DeletedFiles []string `json:"deleted_files,omitempty"`
	// Conflicts is the list of conflicting files (if any).
	Conflicts []string `json:"conflicts,omitempty"`
	// CommitHash is the HEAD commit hash after sync.
	CommitHash string `json:"commit_hash"`
	// Error is the error message if sync failed.
	Error string `json:"error,omitempty"`
}

// Sync performs a git fetch and integration operation.
// It loads the config, opens the git repository, fetches from the remote,
// and integrates changes using the specified strategy.
func Sync(ctx context.Context, opts SyncOptions) (*SyncOutput, error) {
	// Load config
	cfg, err := loadConfigForSync(opts.ConfigPath)
	if err != nil {
		return &SyncOutput{
			Success: false,
			Error:   fmt.Sprintf("config load failed: %v", err),
		}, fmt.Errorf("failed to load config: %w", err)
	}

	// Get repo path
	repoPath := cfg.Repo.Path
	if repoPath == "" {
		// Use current directory if not configured
		repoPath, err = os.Getwd()
		if err != nil {
			return &SyncOutput{
				Success: false,
				Error:   "failed to get working directory",
			}, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	// Open git repo
	repo := gitrepo.New(repoPath)
	if !repo.IsInitialized(ctx) {
		return &SyncOutput{
			Success: false,
			Error:   "not a git repository",
		}, fmt.Errorf("not a git repository: %s", repoPath)
	}
	defer repo.Close()

	// Get current status
	status, err := repo.Status(ctx)
	if err != nil {
		return &SyncOutput{
			Success: false,
			Error:   fmt.Sprintf("status check failed: %v", err),
		}, fmt.Errorf("failed to get repo status: %w", err)
	}

	if status.Dirty {
		return &SyncOutput{
			Success: false,
			Error:   "working directory has uncommitted changes",
		}, fmt.Errorf("cannot sync with uncommitted changes")
	}

	// Build integration options
	integrateOpts := gitrepo.IntegrateOptions{
		Strategy: opts.Strategy,
		Remote:   opts.Remote,
		Branch:   opts.Branch,
		NoFetch:  opts.NoFetch,
	}

	// Use config values if not specified
	if integrateOpts.Strategy == "" {
		integrateOpts.Strategy = gitrepo.IntegrationStrategy(cfg.Repo.SyncStrategy)
	}
	if integrateOpts.Remote == "" {
		integrateOpts.Remote = cfg.Repo.Remote
	}
	if integrateOpts.Branch == "" {
		integrateOpts.Branch = cfg.Repo.Branch
	}

	// Validate strategy
	if integrateOpts.Strategy == "" {
		integrateOpts.Strategy = gitrepo.Rebase // default
	}

	// Perform the integration
	result, err := repo.Integrate(ctx, integrateOpts)
	if err != nil {
		return &SyncOutput{
			Success:  false,
			Branch:   status.Branch,
			Strategy: integrateOpts.Strategy,
			Error:    fmt.Sprintf("integration failed: %v", err),
		}, err
	}

	// Build output
	output := &SyncOutput{
		Success:      result.Success,
		Branch:       status.Branch,
		RemoteBranch: fmt.Sprintf("%s/%s", integrateOpts.Remote, integrateOpts.Branch),
		Strategy:     integrateOpts.Strategy,
		Ahead:        status.Ahead,
		Behind:       status.Behind,
		NewFiles:     result.NewFiles,
		UpdatedFiles: result.UpdatedFiles,
		DeletedFiles: result.DeletedFiles,
		Conflicts:    result.Conflicts,
		CommitHash:   result.CommitHash,
	}

	// Push if requested and successful
	if result.Success && opts.Push && !opts.NoPush {
		if pushErr := pushChanges(ctx, repo, integrateOpts.Remote); pushErr != nil {
			output.Error = fmt.Sprintf("push failed: %v", pushErr)
			return output, pushErr
		}
	}

	return output, nil
}

// loadConfigForSync loads the config, falling back to default paths.
func loadConfigForSync(configPath string) (*config.Config, error) {
	if configPath != "" {
		return config.Load(configPath)
	}

	// Detect the config file path
	detectedPath := config.DetectConfigPath()
	if detectedPath == "" {
		// Return default config if none exists
		return config.DefaultConfig(), nil
	}

	return config.Load(detectedPath)
}

// pushChanges pushes local commits to the remote.
func pushChanges(ctx context.Context, repo gitrepo.Repo, remote string) error {
	// This would require adding a Push method to gitrepo.Repo
	// For now, we'll use a simpler approach with the existing interface
	// The implementation can be added later when needed
	return fmt.Errorf("push not yet implemented")
}
