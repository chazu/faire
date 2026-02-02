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
	"github.com/chazuruo/svf/internal/workflows/store"
)

// ExplainOptions contains the options for the explain command.
type ExplainOptions struct {
	ConfigPath  string
	Offline     bool
	Provider    string
	Model       string
	WorkflowRef string
	StepIdx     int
	StepName    string
}

// NewExplainCommand creates the explain command.
func NewExplainCommand() *cobra.Command {
	opts := &ExplainOptions{}

	cmd := &cobra.Command{
		Use:   "explain <command>",
		Short: "Explain shell commands in plain language",
		Long: `Explain shell commands in plain language.

Supports two modes:
1. Direct command explanation: svf explain "kubectl get pods"
2. Workflow step explanation: svf explain <workflow-ref> --step <index|name>

Rule-based mode (--offline):
- Uses pattern matching for common commands
- Works offline without AI
- Shows risk levels for commands

AI mode (when configured):
- Provides detailed, context-aware explanations
- Requires AI provider configuration in config

Exit codes:
- 0: Success
- 30: AI not configured (when using AI mode)
- 31: AI provider error`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && opts.WorkflowRef == "" {
				return fmt.Errorf("command argument or workflow reference required\nUsage: svf explain <command> OR svf explain <workflow-ref> --step <index>")
			}
			if len(args) > 0 && opts.WorkflowRef == "" {
				return runExplainCommand(args[0], opts)
			}
			return runExplainWorkflow(opts)
		},
	}

	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.Flags().BoolVar(&opts.Offline, "offline", false, "use rule-based explanation only (no AI)")
	cmd.Flags().StringVar(&opts.Provider, "provider", "", "override AI provider")
	cmd.Flags().StringVar(&opts.Model, "model", "", "override AI model")
	cmd.Flags().StringVar(&opts.WorkflowRef, "workflow", "", "workflow reference for step explanation")
	cmd.Flags().IntVar(&opts.StepIdx, "step", -1, "step index (0-based) for workflow step explanation")
	cmd.Flags().StringVar(&opts.StepName, "step-name", "", "step name for workflow step explanation")

	return cmd
}

// runExplainCommand explains a direct command.
func runExplainCommand(command string, opts *ExplainOptions) error {
	// Load config
	cfg, err := config.LoadWithDefaults()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create explainer
	explainer, err := createExplainer(opts, cfg)
	if err != nil {
		return err
	}

	// Check if AI is available and not in offline mode
	useAI := !opts.Offline && cfg.AI.Enabled

	// Privacy confirmation for AI mode
	if useAI && cfg.AI.ConfirmSend {
		redactedCmd := command
		if cfg.AI.Redact != "none" {
			redactedCmd = ai.Redact(command)
		}

		fmt.Println("Sending command to AI for explanation...")
		fmt.Printf("\nCommand (redacted):\n%s\n\n", redactedCmd)
		fmt.Print("Send to AI? [y/N]: ")

		var response string
		fmt.Scanln(&response)
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println("Falling back to rule-based explanation.")
			useAI = false
		}
	}

	// Get explanation
	ctx := context.Background()
	result := explainer.ExplainCommand(ctx, command)

	// Format and display
	printExplanation(result, command)

	return nil
}

// runExplainWorkflow explains a workflow step.
func runExplainWorkflow(opts *ExplainOptions) error {
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

	// Create store
	str, err := store.New(repo, cfg)
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}

	// Resolve workflow reference (using existing function from view.go)
	ref, err := resolveWorkflowRef(ctx, str, opts.WorkflowRef)
	if err != nil {
		return err
	}

	// Load workflow
	wf, err := str.Load(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to load workflow: %w", err)
	}

	// Find the step
	var stepIdx int
	var stepName string
	var stepCmd string

	if opts.StepName != "" {
		// Find by name
		found := false
		for i, s := range wf.Steps {
			if s.Name == opts.StepName {
				stepIdx = i
				stepName = s.Name
				stepCmd = s.Command
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("step not found: %s", opts.StepName)
		}
	} else if opts.StepIdx >= 0 {
		// Find by index
		if opts.StepIdx >= len(wf.Steps) {
			return fmt.Errorf("step index out of range: %d (workflow has %d steps)", opts.StepIdx, len(wf.Steps))
		}
		stepIdx = opts.StepIdx
		stepName = wf.Steps[stepIdx].Name
		stepCmd = wf.Steps[stepIdx].Command
	} else {
		return fmt.Errorf("must specify --step <index> or --step-name <name>")
	}

	// Create explainer
	explainer, err := createExplainer(opts, cfg)
	if err != nil {
		return err
	}

	// Check if AI is available and not in offline mode
	useAI := !opts.Offline && cfg.AI.Enabled

	// Privacy confirmation for AI mode
	if useAI && cfg.AI.ConfirmSend {
		redactedCmd := stepCmd
		if cfg.AI.Redact != "none" {
			redactedCmd = ai.Redact(stepCmd)
		}

		fmt.Println("Sending step command to AI for explanation...")
		fmt.Printf("\nWorkflow: %s\n", wf.Title)
		fmt.Printf("Step %d: %s\n", stepIdx+1, stepName)
		fmt.Printf("\nCommand (redacted):\n%s\n\n", redactedCmd)
		fmt.Print("Send to AI? [y/N]: ")

		var response string
		fmt.Scanln(&response)
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println("Falling back to rule-based explanation.")
			useAI = false
		}
	}

	// Get explanation
	result := explainer.ExplainWorkflowStep(ctx, stepName, stepCmd, stepIdx)

	// Format and display
	printWorkflowExplanation(result, wf.Title, stepIdx, stepName)

	return nil
}

// createExplainer creates an explainer with appropriate configuration.
func createExplainer(opts *ExplainOptions, cfg *config.Config) (*explain.Explainer, error) {
	explainerOpts := &explain.Options{
		Offline: opts.Offline,
	}

	// Only set up AI provider if not in offline mode and AI is configured
	if !opts.Offline && cfg.AI.Enabled {
		providerCfg := &ai.Config{
			Provider: cfg.AI.Provider,
			BaseURL:  cfg.AI.BaseURL,
			Model:    cfg.AI.Model,
		}

		// Apply overrides from command line
		if opts.Provider != "" {
			providerCfg.Provider = opts.Provider
		}
		if opts.Model != "" {
			providerCfg.Model = opts.Model
		}

		// Get API key
		if cfg.AI.APIKeyEnv != "" {
			providerCfg.APIKey = os.Getenv(cfg.AI.APIKeyEnv)
		}

		provider, err := ai.NewProvider(providerCfg)
		if err != nil {
			// Don't fail on provider error, fall back to rule-based
			explainerOpts.Offline = true
		} else {
			explainerOpts.Provider = provider
		}
	}

	return explain.NewExplainer(explainerOpts), nil
}

// printExplanation formats and prints a command explanation.
func printExplanation(result explain.Explanation, command string) {
	fmt.Printf("Command: %s\n\n", result.Command)

	// Show risk level with icon
	riskIcon := explain.ParseRiskLevel(result.Risk).Icon()
	fmt.Printf("[%s] Risk: %s\n", riskIcon, result.Risk)
	fmt.Printf("Category: %s\n\n", result.Category)

	fmt.Printf("Explanation:\n%s\n", result.Explanation)

	// Show additional context if available
	if result.Context != "" {
		fmt.Printf("\nContext: %s\n", result.Context)
	}

	// Show alternatives if available
	if len(result.Alternatives) > 0 {
		fmt.Printf("\nAlternatives:\n")
		for _, alt := range result.Alternatives {
			fmt.Printf("  - %s\n", alt)
		}
	}

	// Show see also if available
	if len(result.SeeAlso) > 0 {
		fmt.Printf("\nSee also:\n")
		for _, ref := range result.SeeAlso {
			fmt.Printf("  - %s\n", ref)
		}
	}
}

// printWorkflowExplanation formats and prints a workflow step explanation.
func printWorkflowExplanation(result explain.Explanation, workflowName string, stepIdx int, stepName string) {
	fmt.Printf("Workflow: %s\n", workflowName)
	fmt.Printf("Step %d: %s\n\n", stepIdx+1, stepName)

	fmt.Printf("Command: %s\n\n", result.Command)

	// Show risk level with icon
	riskIcon := explain.ParseRiskLevel(result.Risk).Icon()
	fmt.Printf("[%s] Risk: %s\n", riskIcon, result.Risk)
	fmt.Printf("Category: %s\n\n", result.Category)

	fmt.Printf("Explanation:\n%s\n", result.Explanation)

	// Show additional context if available
	if result.Context != "" {
		fmt.Printf("\nContext: %s\n", result.Context)
	}
}
