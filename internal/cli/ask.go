// Package cli provides Cobra command definitions for svf.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/chazuruo/svf/internal/ai"
	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/gitrepo"
	"github.com/chazuruo/svf/internal/workflows"
)

// AskOptions contains the options for the ask command.
type AskOptions struct {
	ConfigPath string
	Prompt     string
	Provider   string
	Model      string
	APIKeyEnv  string
	As         string // "workflow" or "step"
	Identity   string
	NoCommit   bool
	JSON       bool
}

// NewAskCommand creates the ask command.
func NewAskCommand() *cobra.Command {
	opts := &AskOptions{}

	cmd := &cobra.Command{
		Use:   "ask",
		Short: "Generate workflows or steps using AI",
		Long: `Generate workflows or steps from natural language descriptions using AI.

The command will:
1. Prompt for your goal (natural language description)
2. Show redaction UI to review sensitive data
3. Send to AI provider for generation
4. Show result in workflow editor for review
5. Save to repository (unless --no-commit)

Provider selection:
- Use --provider to specify (openai, ollama, etc.)
- Use --model to specify the model name
- Use --api-key-env to specify the environment variable for API key

Output format:
- Use --as workflow to generate a full workflow (default)
- Use --as step to generate a single step
- Use --identity to set the workflow identity path`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAsk(opts)
		},
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.Flags().StringVar(&opts.Prompt, "prompt", "", "Natural language prompt for workflow/step generation")
	cmd.Flags().StringVar(&opts.Provider, "provider", "", "AI provider (openai, ollama, etc.)")
	cmd.Flags().StringVar(&opts.Model, "model", "", "Model name")
	cmd.Flags().StringVar(&opts.APIKeyEnv, "api-key-env", "", "Environment variable for API key")
	cmd.Flags().StringVar(&opts.As, "as", "workflow", "Output format: workflow or step")
	cmd.Flags().StringVar(&opts.Identity, "identity", "", "Identity path for the workflow")
	cmd.Flags().BoolVar(&opts.NoCommit, "no-commit", false, "Don't commit to git after saving")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output result as JSON")

	return cmd
}

func runAsk(opts *AskOptions) error {
	ctx := context.Background()

	// Load config
	cfg, err := config.LoadWithDefaults()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Open repo
	repo := gitrepo.New(cfg.Repo.Path)
	if !repo.IsInitialized(ctx) {
		return fmt.Errorf("repository not initialized. Run 'svf init' first")
	}

	// Check for --no-tui flag
	if IsNoTUI() {
		return runAskNonInteractive(ctx, opts, cfg)
	}

	// Interactive mode
	return runAskInteractive(ctx, opts, cfg)
}

// runAskInteractive runs ask command in TUI mode.
func runAskInteractive(ctx context.Context, opts *AskOptions, cfg *config.Config) error {
	// TODO: Integrate with TUI ask model
	// For now, fall back to non-interactive with error
	fmt.Println("Interactive mode requires TUI implementation.")
	fmt.Println("Please use --no-tui flag for non-interactive mode.")
	return runAskNonInteractive(ctx, opts, cfg)
}

// runAskNonInteractive runs ask command in non-interactive mode.
func runAskNonInteractive(ctx context.Context, opts *AskOptions, cfg *config.Config) error {
	// Check if prompt is provided
	if opts.Prompt == "" {
		return fmt.Errorf("prompt required for non-interactive mode.\nUsage: svf ask --prompt \"your prompt here\"")
	}

	// Build AI config
	aiCfg := buildAIConfig(opts, cfg)

	// Create provider
	provider, err := ai.NewProvider(aiCfg)
	if err != nil {
		os.Exit(30) // Provider not configured
		return fmt.Errorf("failed to create AI provider: %w", err)
	}

	// Check if provider is configured
	if provider == nil {
		os.Exit(30) // Provider not configured
		return fmt.Errorf("AI provider not configured. Please configure AI in settings or use --provider flag")
	}

	// Generate workflow
	wf, err := generateWorkflow(ctx, provider, opts.Prompt, opts)
	if err != nil {
		os.Exit(31) // Provider error
		return fmt.Errorf("failed to generate workflow: %w", err)
	}

	// Output result
	if opts.JSON {
		return outputWorkflowJSON(wf)
	}

	return outputWorkflowText(wf)
}

// outputWorkflowJSON outputs workflow as JSON.
func outputWorkflowJSON(wf *workflows.Workflow) error {
	// Use the export package to output JSON
	// For now, just print basic info
	fmt.Printf(`{"title":"%s","description":"%s","steps":%d}`, wf.Title, wf.Description, len(wf.Steps))
	return nil
}

// outputWorkflowText outputs workflow as text.
func outputWorkflowText(wf *workflows.Workflow) error {
	fmt.Printf("Title: %s\n", wf.Title)
	if wf.Description != "" {
		fmt.Printf("Description: %s\n", wf.Description)
	}
	fmt.Printf("Steps: %d\n", len(wf.Steps))
	for i, step := range wf.Steps {
		fmt.Printf("\n%d. %s\n", i+1, step.Name)
		fmt.Printf("   %s\n", step.Command)
	}
	return nil
}

// buildAIConfig builds AI config from options and global config.
func buildAIConfig(opts *AskOptions, cfg *config.Config) *ai.Config {
	aiCfg := ai.DefaultConfig()

	// Apply options
	if opts.Provider != "" {
		aiCfg.Provider = opts.Provider
	}
	if opts.Model != "" {
		aiCfg.Model = opts.Model
	}
	if opts.APIKeyEnv != "" {
		// Get API key from environment
		aiCfg.APIKey = os.Getenv(opts.APIKeyEnv)
	}

	// Apply global config if available
	if cfg.AI.Provider != "" {
		aiCfg.Provider = cfg.AI.Provider
	}
	if cfg.AI.Model != "" {
		aiCfg.Model = cfg.AI.Model
	}
	if cfg.AI.BaseURL != "" {
		aiCfg.BaseURL = cfg.AI.BaseURL
	}

	return aiCfg
}

// generateWorkflow generates a workflow from a prompt.
func generateWorkflow(ctx context.Context, provider ai.Provider, prompt string, opts *AskOptions) (*workflows.Workflow, error) {
	req := ai.GenerateRequest{
		Prompt: prompt,
		Options: ai.GenerateOptions{
			IncludePlaceholders: true,
		},
	}

	return provider.GenerateWorkflow(ctx, req)
}
