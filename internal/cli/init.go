// Package cli provides Cobra command definitions for svf.
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/gitrepo"
)

// InitOptions contains the options for the init command.
type InitOptions struct {
	ConfigPath string

	// Scriptable/flag options for --no-tui mode
	Remote      string
	Local       string
	Branch      string
	Identity    string
	Mode        string
	AuthorName  string
	AuthorEmail string
	SignCommits bool
	NoCommit    bool
}

// NewInitCommand creates the init command.
func NewInitCommand() *cobra.Command {
	opts := &InitOptions{}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize svf configuration",
		Long: `Initialize svf configuration and optionally clone a Git repository.

The init command guides you through setting up your svf configuration:
- Choose a Git repository (remote URL or local path)
- Set your identity path (e.g., "username" or "team/username")
- Configure git author details
- Choose write mode (direct or PR-based)

Use --no-tui with flags for scripted setup.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(opts)
		},
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.Flags().StringVar(&opts.Remote, "remote", "", "remote Git URL to clone")
	cmd.Flags().StringVar(&opts.Local, "local", "", "use existing local repository path")
	cmd.Flags().StringVar(&opts.Branch, "branch", "", "default branch name (auto-detect if omitted)")
	cmd.Flags().StringVar(&opts.Identity, "identity", "", "identity path in repo (e.g., chaz or platform/chaz)")
	cmd.Flags().StringVar(&opts.Mode, "mode", "", "write mode: direct or pr")
	cmd.Flags().StringVar(&opts.AuthorName, "author-name", "", "git author name")
	cmd.Flags().StringVar(&opts.AuthorEmail, "author-email", "", "git author email")
	cmd.Flags().BoolVar(&opts.SignCommits, "sign", false, "sign commits")
	cmd.Flags().BoolVar(&opts.NoCommit, "no-commit", false, "skip git commit after saving")

	return cmd
}

func runInit(opts *InitOptions) error {
	// Check if --no-tui mode
	if IsNoTUI() {
		return runInitNonInteractive(opts)
	}

	// Interactive TUI mode
	return runInitInteractive(opts)
}

// runInitInteractive runs the init wizard with TUI.
func runInitInteractive(opts *InitOptions) error {
	var (
		repoSource   string // "remote" or "local"
		remoteURL    string
		localPath    string
		branch       string
		identityPath string
		mode         string // "direct" or "pr"
		authorName   string
		authorEmail  string
		signCommits  bool
	)

	cfg := config.DefaultConfig()

	// Step 1: Repo source
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Repository source").
				Options(
					huh.NewOption("Clone from remote URL", "remote"),
					huh.NewOption("Use existing local repository", "local"),
				).
				Value(&repoSource),
		),
	).Run(); err != nil {
		return fmt.Errorf("form error: %w", err)
	}

	// Step 2: Repository details
	repoGroup := []huh.Field{
		huh.NewInput().
			Title("Repository path").
			Description("Local path where the repo will be/is located").
			Value(&localPath).Placeholder(cfg.Repo.Path),
	}

	if repoSource == "remote" {
		repoGroup = append([]huh.Field{
			huh.NewInput().
				Title("Remote URL").
				Description("Git remote URL (HTTPS or SSH)").
				Value(&remoteURL).Placeholder("https://github.com/user/workflows.git"),
		}, repoGroup...)
	}

	if err := huh.NewForm(
		huh.NewGroup(repoGroup...),
	).Run(); err != nil {
		return fmt.Errorf("form error: %w", err)
	}

	// Set defaults from input
	if localPath == "" {
		localPath = cfg.Repo.Path
	}

	// Step 3: Identity path
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Identity path").
				Description("Your path in the repository (e.g., 'chaz' or 'platform/chaz')").
				Value(&identityPath).Placeholder("username"),
		),
	).Run(); err != nil {
		return fmt.Errorf("form error: %w", err)
	}

	// Step 4: Mode
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Write mode").
				Description("How should changes be committed?").
				Options(
					huh.NewOption("Direct - commit directly to main branch", "direct"),
					huh.NewOption("PR - create feature branches and PRs", "pr"),
				).
				Value(&mode),
		),
	).Run(); err != nil {
		return fmt.Errorf("form error: %w", err)
	}

	// Step 5: Git author details
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Author name").
				Value(&authorName).Placeholder(cfg.Git.AuthorName),
			huh.NewInput().
				Title("Author email").
				Value(&authorEmail).Placeholder(cfg.Git.AuthorEmail),
			huh.NewConfirm().
				Title("Sign commits?").
				Value(&signCommits),
		),
	).Run(); err != nil {
		return fmt.Errorf("form error: %w", err)
	}

	// Step 6: Clone or setup repo
	if repoSource == "remote" {
		if err := cloneRepoSpinner(remoteURL, localPath, branch); err != nil {
			return err
		}
	} else {
		// Local repo - initialize if needed
		if err := initLocalRepoSpinner(localPath, branch); err != nil {
			return err
		}
	}

	// Build config
	finalCfg := buildConfig(cfg, localPath, identityPath, mode, branch, authorName, authorEmail, signCommits)

	// Validate config
	if err := finalCfg.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Create folder structure
	if err := createFolderStructure(localPath, finalCfg); err != nil {
		return err
	}

	// Write config
	if err := writeConfigSpinner(finalCfg); err != nil {
		return err
	}

	// Verify that the repository is actually initialized at the configured path
	repo := gitrepo.New(finalCfg.Repo.Path)
	ctx := context.Background()
	if !repo.IsInitialized(ctx) {
		return fmt.Errorf("repository was not initialized correctly at %s. Please check the path and try 'svf init' again", finalCfg.Repo.Path)
	}

	// Summary
	fmt.Println("\n✓ Configuration written successfully!")
	fmt.Printf("  Config: %s\n", getConfigPath(opts.ConfigPath))
	fmt.Printf("  Repo:   %s\n", finalCfg.Repo.Path)
	fmt.Printf("  Identity: %s\n", finalCfg.Identity.Path)
	fmt.Printf("  Mode:   %s\n", finalCfg.Identity.Mode)
	fmt.Println("\nYou're ready to go! Try 'svf whoami' to verify.")

	return nil
}

// runInitNonInteractive runs init in non-TUI mode using flags.
func runInitNonInteractive(opts *InitOptions) error {
	cfg := config.DefaultConfig()

	// Set values from flags
	if opts.Local != "" {
		cfg.Repo.Path = opts.Local
	}
	if opts.Remote != "" {
		cfg.Repo.Path = opts.Local
		if cfg.Repo.Path == "" {
			cfg.Repo.Path = filepath.Join(os.Getenv("HOME"), ".local", "share", "svf", "repo")
		}
	}
	if opts.Branch != "" {
		cfg.Repo.Branch = opts.Branch
	}
	if opts.Identity != "" {
		cfg.Identity.Path = opts.Identity
	} else {
		return fmt.Errorf("--identity is required in non-interactive mode")
	}
	if opts.Mode != "" {
		cfg.Identity.Mode = opts.Mode
	}
	if opts.AuthorName != "" {
		cfg.Git.AuthorName = opts.AuthorName
	}
	if opts.AuthorEmail != "" {
		cfg.Git.AuthorEmail = opts.AuthorEmail
	}
	if opts.SignCommits {
		cfg.Git.SignCommits = true
	}

	// Clone from remote if specified
	if opts.Remote != "" {
		if err := gitrepo.Clone(opts.Remote, cfg.Repo.Path, cfg.Repo.Branch); err != nil {
			return fmt.Errorf("failed to clone repo: %w", err)
		}
		fmt.Printf("Cloned %s to %s\n", opts.Remote, cfg.Repo.Path)
	} else {
		// Local repo - initialize if needed
		if err := initLocalRepoSpinner(cfg.Repo.Path, cfg.Repo.Branch); err != nil {
			return fmt.Errorf("failed to initialize local repo: %w", err)
		}
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Create folder structure
	if err := createFolderStructure(cfg.Repo.Path, cfg); err != nil {
		return fmt.Errorf("failed to create folder structure: %w", err)
	}

	// Write config
	configPath := getConfigPath(opts.ConfigPath)
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	if err := config.Write(configPath, cfg); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Verify that the repository is actually initialized at the configured path
	repo := gitrepo.New(cfg.Repo.Path)
	ctx := context.Background()
	if !repo.IsInitialized(ctx) {
		return fmt.Errorf("repository was not initialized correctly at %s. Please check the path and try 'svf init' again", cfg.Repo.Path)
	}

	fmt.Printf("Configuration written to: %s\n", configPath)
	return nil
}

// cloneRepoSpinner clones a repository with a spinner.
func cloneRepoSpinner(remoteURL, localPath, branch string) error {
	fmt.Printf("Cloning repository from %s...\n", remoteURL)
	if branch == "" {
		branch = "main" // default
	}

	if err := gitrepo.Clone(remoteURL, localPath, branch); err != nil {
		return fmt.Errorf("failed to clone repo: %w", err)
	}

	fmt.Println("✓ Repository cloned")
	return nil
}

// initLocalRepoSpinner initializes a local repository if needed.
func initLocalRepoSpinner(localPath, branch string) error {
	fmt.Printf("Setting up local repository at %s...\n", localPath)

	// Check if path exists
	info, err := os.Stat(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create directory and initialize
			if err := os.MkdirAll(localPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			return initGitRepo(localPath, branch)
		}
		return fmt.Errorf("failed to stat path: %w", err)
	}

	// Path exists - check if it's already a git repo
	if info.IsDir() {
		repo := gitrepo.New(localPath)
		ctx := context.Background()
		if repo.IsInitialized(ctx) {
			fmt.Println("✓ Using existing git repository")
			return nil
		}
		// Directory exists but not a git repo - initialize it
		return initGitRepo(localPath, branch)
	}

	return fmt.Errorf("path exists but is not a directory: %s", localPath)
}

// initGitRepo initializes a git repository in the given directory.
func initGitRepo(path, branch string) error {
	repo := gitrepo.New(path)
	ctx := context.Background()

	opts := gitrepo.InitOptions{}
	if branch != "" {
		opts.DefaultBranch = branch
	}

	if err := repo.Init(ctx, opts); err != nil {
		return fmt.Errorf("failed to initialize git repo: %w", err)
	}

	fmt.Println("✓ Git repository initialized")
	return nil
}

// createFolderStructure creates the necessary folder structure for svf.
func createFolderStructure(repoPath string, cfg *config.Config) error {
	fmt.Println("Creating folder structure...")

	// Create workflows directory
	workflowsDir := filepath.Join(repoPath, cfg.Workflows.Root)
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		return fmt.Errorf("failed to create workflows directory: %w", err)
	}

	// Create identity path directory
	identityDir := filepath.Join(workflowsDir, cfg.Identity.Path)
	if err := os.MkdirAll(identityDir, 0755); err != nil {
		return fmt.Errorf("failed to create identity directory: %w", err)
	}

	// Create shared directory
	sharedDir := filepath.Join(repoPath, cfg.Workflows.SharedRoot)
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		return fmt.Errorf("failed to create shared directory: %w", err)
	}

	// Create drafts directory
	draftsDir := filepath.Join(repoPath, cfg.Workflows.DraftRoot)
	if err := os.MkdirAll(draftsDir, 0755); err != nil {
		return fmt.Errorf("failed to create drafts directory: %w", err)
	}

	// Create .svf directory for index
	svfDir := filepath.Join(repoPath, ".svf")
	if err := os.MkdirAll(svfDir, 0755); err != nil {
		return fmt.Errorf("failed to create .svf directory: %w", err)
	}

	fmt.Println("✓ Folder structure created")
	return nil
}

// writeConfigSpinner writes the config with a spinner.
func writeConfigSpinner(cfg *config.Config) error {
	fmt.Println("Writing configuration...")
	configPath := getConfigPath("")

	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := config.Write(configPath, cfg); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Println("✓ Configuration written")
	return nil
}

// buildConfig builds the final config from wizard inputs.
func buildConfig(base *config.Config, repoPath, identityPath, mode, branch, authorName, authorEmail string, signCommits bool) *config.Config {
	cfg := *base // copy defaults

	cfg.Repo.Path = repoPath
	if branch != "" {
		cfg.Repo.Branch = branch
	}

	cfg.Identity.Path = identityPath
	if mode != "" {
		cfg.Identity.Mode = mode
	}

	if authorName != "" {
		cfg.Git.AuthorName = authorName
	}
	if authorEmail != "" {
		cfg.Git.AuthorEmail = authorEmail
	}
	cfg.Git.SignCommits = signCommits

	return &cfg
}

// getConfigPath returns the config file path.
func getConfigPath(override string) string {
	if override != "" {
		return override
	}
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".config", "svf", "config.toml")
}
