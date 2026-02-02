// Package cli provides Cobra command definitions for svf.
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/chazuruo/svf/internal/ai"
	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/explain"
	"github.com/chazuruo/svf/internal/gitrepo"
	"github.com/chazuruo/svf/internal/workflows"
	"github.com/chazuruo/svf/internal/workflows/store"
)

// ExplainOptions contains the options for the explain command.
type ExplainOptions struct {
	ConfigPath string
	Command    string
	Workflow   string
	Step       string
	Provider   string
	Model      string
	APIKeyEnv  string
	Offline    bool
	JSON       bool
	Detail     string
}

// NewExplainCommand creates the explain command.
func NewExplainCommand() *cobra.Command {
	opts := &ExplainOptions{}

	cmd := &cobra.Command{
		Use:   "explain",
		Short: "Explain commands in plain language",
		Long: `Explain shell commands, workflow steps, or entire workflows in plain language.

The command can explain:
- A shell command directly: svf explain "kubectl get pods"
- A workflow step: svf explain my-workflow --step 2
- An entire workflow: svf explain my-workflow

Provider modes:
- AI mode (default): Uses configured AI provider for detailed explanations
- Offline mode (--offline): Uses rule-based patterns without AI

Privacy:
- AI mode shows what will be sent before transmitting
- Use --offline to keep everything local

Exit codes:
- 0: Success
- 30: AI provider not configured
- 31: AI provider error`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExplain(opts, args)
		},
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.Flags().StringVar(&opts.Workflow, "workflow", "", "Workflow name or ID to explain")
	cmd.Flags().StringVar(&opts.Step, "step", "", "Step index or name to explain")
	cmd.Flags().StringVar(&opts.Provider, "provider", "", "AI provider (openai, ollama, etc.)")
	cmd.Flags().StringVar(&opts.Model, "model", "", "Model name")
	cmd.Flags().StringVar(&opts.APIKeyEnv, "api-key-env", "", "Environment variable for API key")
	cmd.Flags().BoolVar(&opts.Offline, "offline", false, "Use rule-based explanations only (no AI)")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output result as JSON")
	cmd.Flags().StringVar(&opts.Detail, "detail", "normal", "Detail level: brief, normal, verbose")

	return cmd
}

func runExplain(opts *ExplainOptions, args []string) error {
	ctx := context.Background()

	// Load config
	cfg, err := config.LoadWithDefaults()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Determine what to explain
	// Priority: workflow ref > command argument
	if opts.Workflow != "" {
		// Explain workflow or workflow step
		return explainWorkflow(ctx, opts, cfg)
	}

	// Get command from args or stdin
	if len(args) > 0 {
		opts.Command = strings.Join(args, " ")
	} else if opts.Command == "" {
		// Try to read from stdin
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// stdin is piped
			var input strings.Builder
			var buf [1024]byte
			for {
				n, err := os.Stdin.Read(buf[:])
				if n > 0 {
					input.Write(buf[:n])
				}
				if err != nil {
					break
				}
			}
			opts.Command = strings.TrimSpace(input.String())
		}
	}

	if opts.Command == "" {
		return fmt.Errorf("command or workflow required\n\nUsage: svf explain \"<command>\"\n   or: svf explain <workflow> [--step <index|name>]")
	}

	// Explain command
	return explainCommand(ctx, opts, cfg)
}

// explainCommand explains a single shell command.
func explainCommand(ctx context.Context, opts *ExplainOptions, cfg *config.Config) error {
	// Build explainer
	exp, provider, err := buildExplainer(opts, cfg)
	if err != nil {
		os.Exit(30)
		return fmt.Errorf("failed to create explainer: %w", err)
	}

	// If using AI and not offline, show privacy confirmation
	if provider != nil && !opts.Offline {
		redacted := ai.Redact(opts.Command)
		if redacted != opts.Command {
			fmt.Fprintf(os.Stderr, "Sending command to AI for explanation...\n\n")
			fmt.Fprintf(os.Stderr, "Command (redacted):\n%s\n\n", redacted)
			fmt.Fprintf(os.Stderr, "Send to AI? [y/N]: ")

			var response string
			fmt.Scanln(&response)
			if strings.ToLower(strings.TrimSpace(response)) != "y" {
				fmt.Println("Canceled.")
				return nil
			}
		}
	}

	// Get explanation
	result := exp.ExplainCommand(ctx, opts.Command)

	// Output result
	if opts.JSON {
		return outputExplanationJSON(result)
	}

	return outputExplanationText(result, "")
}

// explainWorkflow explains a workflow or workflow step.
func explainWorkflow(ctx context.Context, opts *ExplainOptions, cfg *config.Config) error {
	// Open repo
	repo := gitrepo.New(cfg.Repo.Path)
	if !repo.IsInitialized(ctx) {
		return fmt.Errorf("repository not initialized. Run 'svf init' first")
	}

	// Create store
	fs, err := store.New(repo, cfg)
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}
	var st store.Store = fs

	// Load workflow
	wf, err := loadWorkflow(st, opts.Workflow)
	if err != nil {
		return fmt.Errorf("failed to load workflow: %w", err)
	}

	// If step specified, explain that step
	if opts.Step != "" {
		return explainWorkflowStep(ctx, opts, cfg, wf)
	}

	// Explain entire workflow (requires AI)
	if opts.Offline {
		return fmt.Errorf("workflow explanation requires AI. Remove --offline flag or specify --step")
	}

	// Build explainer
	exp, provider, err := buildExplainer(opts, cfg)
	if err != nil {
		os.Exit(30)
		return fmt.Errorf("failed to create explainer: %w", err)
	}

	if provider == nil {
		os.Exit(30)
		return fmt.Errorf("AI provider not configured. Workflow explanation requires AI")
	}

	// Get detail level
	detailLevel := parseDetailLevel(opts.Detail)

	// Get explanation
	explanation, err := exp.ExplainWorkflow(ctx, wf, detailLevel)
	if err != nil {
		os.Exit(31)
		return fmt.Errorf("failed to explain workflow: %w", err)
	}

	// Output result
	fmt.Printf("Workflow: %s\n", wf.Title)
	if wf.Description != "" {
		fmt.Printf("Description: %s\n", wf.Description)
	}
	fmt.Printf("\n%s\n", explanation)

	return nil
}

// explainWorkflowStep explains a specific workflow step.
func explainWorkflowStep(ctx context.Context, opts *ExplainOptions, cfg *config.Config, wf *workflows.Workflow) error {
	// Find the step
	step, stepIndex, err := findStep(wf, opts.Step)
	if err != nil {
		return err
	}

	// Build explainer
	exp, _, err := buildExplainer(opts, cfg)
	if err != nil {
		return fmt.Errorf("failed to create explainer: %w", err)
	}

	// Get explanation - pass step by value
	result := exp.ExplainWorkflowStep(ctx, step.Name, step.Command, stepIndex)

	// Output result
	if opts.JSON {
		return outputExplanationJSON(result)
	}

	fmt.Printf("Workflow: %s\n", wf.Title)
	fmt.Printf("Step %d: %s\n\n", stepIndex+1, step.Name)

	return outputExplanationText(result, result.Context)
}

// buildExplainer builds an explainer with the given options.
func buildExplainer(opts *ExplainOptions, cfg *config.Config) (*explain.Explainer, ai.Provider, error) {
	var provider ai.Provider
	var err error

	// Build provider if not offline
	if !opts.Offline {
		aiCfg := buildExplainAIConfig(opts, cfg)
		provider, err = ai.NewProvider(aiCfg)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create AI provider: %w", err)
		}
	}

	// Create explainer
	expOpts := &explain.Options{
		Provider: provider,
		Offline:  opts.Offline,
	}

	return explain.NewExplainer(expOpts), provider, nil
}

// buildExplainAIConfig builds AI config from options and global config for explain command.
func buildExplainAIConfig(opts *ExplainOptions, cfg *config.Config) *ai.Config {
	aiCfg := ai.DefaultConfig()

	// Apply options
	if opts.Provider != "" {
		aiCfg.Provider = opts.Provider
	}
	if opts.Model != "" {
		aiCfg.Model = opts.Model
	}
	if opts.APIKeyEnv != "" {
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
	if cfg.AI.APIKeyEnv != "" {
		aiCfg.APIKey = os.Getenv(cfg.AI.APIKeyEnv)
	}

	return aiCfg
}

// loadWorkflow loads a workflow by name or ID.
func loadWorkflow(st store.Store, ref string) (*workflows.Workflow, error) {
	// Resolve workflow reference
	workflowRef, err := resolveWorkflowRef(context.Background(), st, ref)
	if err != nil {
		return nil, err
	}

	// Load workflow
	return st.Load(context.Background(), workflowRef)
}

// findStep finds a step by index or name.
func findStep(wf *workflows.Workflow, stepRef string) (workflows.Step, int, error) {
	// Try parsing as index
	var index int
	if _, err := fmt.Sscanf(stepRef, "%d", &index); err == nil {
		if index < 1 || index > len(wf.Steps) {
			return workflows.Step{}, 0, fmt.Errorf("step index out of range: %d (valid: 1-%d)", index, len(wf.Steps))
		}
		return wf.Steps[index-1], index - 1, nil
	}

	// Try finding by name
	for i, step := range wf.Steps {
		if step.Name == stepRef {
			return step, i, nil
		}
	}

	return workflows.Step{}, 0, fmt.Errorf("step not found: %s", stepRef)
}

// parseDetailLevel parses a detail level string.
func parseDetailLevel(s string) ai.DetailLevel {
	switch strings.ToLower(s) {
	case "brief":
		return ai.DetailBrief
	case "verbose":
		return ai.DetailVerbose
	default:
		return ai.DetailNormal
	}
}

// outputExplanationText outputs explanation as text.
func outputExplanationText(result explain.Explanation, context string) error {
	if context != "" {
		fmt.Printf("%s\n\n", context)
	}

	fmt.Printf("Command: %s\n\n", result.Command)
	fmt.Printf("Explanation:\n%s\n", result.Explanation)

	if result.Risk != "" && result.Risk != "unknown" {
		risk := explain.ParseRiskLevel(result.Risk)
		fmt.Printf("\nRisk: %s %s", risk.Icon(), risk.String())
	}

	if result.Category != "" && result.Category != "ai-generated" {
		fmt.Printf(" | Category: %s", result.Category)
	}

	fmt.Println()

	return nil
}

// outputExplanationJSON outputs explanation as JSON.
func outputExplanationJSON(result explain.Explanation) error {
	fmt.Printf(`{"command":"%s","explanation":"%s","risk":"%s","category":"%s"}`,
		result.Command,
		escapeJSON(result.Explanation),
		result.Risk,
		result.Category)
	return nil
}

// escapeJSON escapes a string for JSON.
func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}
