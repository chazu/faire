// Package app provides high-level application logic for the ask command.
package app

import (
	"context"
	"fmt"
	"os"

	"github.com/chazuruo/svf/internal/ai"
	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/workflows"
)

// RedactionLevel controls the redaction intensity.
type RedactionLevel string

const (
	// RedactNone disables redaction.
	RedactNone RedactionLevel = "none"
	// RedactBasic applies basic redaction for common sensitive patterns.
	RedactBasic RedactionLevel = "basic"
	// RedactStrict applies strict redaction for all detected patterns.
	RedactStrict RedactionLevel = "strict"
)

// AskOptions contains options for the ask command.
type AskOptions struct {
	// Prompt is the user's natural language description.
	Prompt string
	// Provider is the AI provider to use.
	Provider string
	// Model is the model name to use.
	Model string
	// APIKeyEnv is the environment variable containing the API key.
	APIKeyEnv string
	// As is the output format (workflow or step).
	As string
	// Identity is the workflow identity path.
	Identity string
	// Redact is the redaction level.
	Redact RedactionLevel
}

// GenerateResult contains the result of workflow generation.
type GenerateResult struct {
	Workflow   *workflows.Workflow
	Provider   string
	Model      string
	Redacted   bool
	Prompt     string
	RedactedPrompt string
}

// ConfigError indicates AI is not configured.
type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return e.Message
}

// IsConfigError returns true if err is a ConfigError.
func IsConfigError(err error) bool {
	_, ok := err.(*ConfigError)
	return ok
}

// ProviderError indicates the AI provider returned an error.
type ProviderError struct {
	Message string
	Cause   error
}

func (e *ProviderError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// IsProviderError returns true if err is a ProviderError.
func IsProviderError(err error) bool {
	_, ok := err.(*ProviderError)
	return ok
}

// CheckConfig checks if AI is properly configured.
// Returns a ConfigError if AI is not enabled or API key is missing.
func CheckConfig(cfg *config.Config) error {
	// Check if AI is enabled
	if !cfg.AI.Enabled {
		return &ConfigError{
			Message: "AI is not enabled.\n\nTo enable AI, set the following in your config:\n" +
				"  [ai]\n" +
				"  enabled = true\n" +
				"  provider = \"openai_compat\"  # or \"ollama\"\n" +
				"  base_url = \"http://localhost:11434/v1\"\n" +
				"  model = \"gpt-4o-mini\"\n" +
				"  api_key_env = \"OPENAI_API_KEY\"\n\n" +
				"Then run 'svf ask' again.",
		}
	}

	// Check for API key env var
	apiKeyEnv := cfg.AI.APIKeyEnv
	if apiKeyEnv == "" {
		apiKeyEnv = "OPENAI_API_KEY"
	}

	apiKey := os.Getenv(apiKeyEnv)
	if apiKey == "" && cfg.AI.Provider != "ollama" {
		// Ollama doesn't require an API key when running locally
		return &ConfigError{
			Message: fmt.Sprintf("AI API key not found.\n\n"+
				"Set the %s environment variable with your API key:\n\n"+
				"  export %s=\"your-api-key-here\"\n\n"+
				"Or configure a different env var in your config:\n"+
				"  [ai]\n"+
				"  api_key_env = \"YOUR_API_KEY_ENV_VAR\"\n",
				apiKeyEnv, apiKeyEnv),
		}
	}

	return nil
}

// GenerateWorkflow generates a workflow from a prompt using AI.
// Returns a GenerateResult with the generated workflow and metadata.
func GenerateWorkflow(ctx context.Context, cfg *config.Config, opts *AskOptions) (*GenerateResult, error) {
	// Check configuration first
	if err := CheckConfig(cfg); err != nil {
		return nil, err
	}

	// Build AI config
	aiCfg := buildAIConfig(cfg, opts)

	// Create provider
	provider, err := ai.NewProvider(aiCfg)
	if err != nil {
		return nil, &ProviderError{
			Message: "failed to create AI provider",
			Cause:   err,
		}
	}

	// Apply redaction if needed
	prompt := opts.Prompt
	redactedPrompt := prompt
	redacted := false

	if opts.Redact != RedactNone {
		redactedPrompt = ai.Redact(prompt)
		redacted = redactedPrompt != prompt
	}

	// Generate workflow
	req := ai.GenerateRequest{
		Prompt: redactedPrompt,
		Options: ai.GenerateOptions{
			IncludePlaceholders: true,
		},
	}

	wf, err := provider.GenerateWorkflow(ctx, req)
	if err != nil {
		return nil, &ProviderError{
			Message: "AI provider error",
			Cause:   err,
		}
	}

	return &GenerateResult{
		Workflow:       wf,
		Provider:       provider.Name(),
		Model:          aiCfg.Model,
		Redacted:       redacted,
		Prompt:         prompt,
		RedactedPrompt: redactedPrompt,
	}, nil
}

// buildAIConfig builds an AI config from options and global config.
func buildAIConfig(cfg *config.Config, opts *AskOptions) *ai.Config {
	aiCfg := ai.DefaultConfig()

	// Apply options (highest priority)
	if opts.Provider != "" {
		aiCfg.Provider = opts.Provider
	} else if cfg.AI.Provider != "" {
		aiCfg.Provider = cfg.AI.Provider
	}

	if opts.Model != "" {
		aiCfg.Model = opts.Model
	} else if cfg.AI.Model != "" {
		aiCfg.Model = cfg.AI.Model
	}

	if opts.APIKeyEnv != "" {
		aiCfg.APIKey = os.Getenv(opts.APIKeyEnv)
	} else if cfg.AI.APIKeyEnv != "" {
		aiCfg.APIKey = os.Getenv(cfg.AI.APIKeyEnv)
	}

	if cfg.AI.BaseURL != "" {
		aiCfg.BaseURL = cfg.AI.BaseURL
	}

	return aiCfg
}

// FormatPrivacyConfirmation formats the privacy confirmation message.
func FormatPrivacyConfirmation(result *GenerateResult) string {
	providerName := result.Provider
	modelName := result.Model

	redactionLevel := "none"
	if result.Redacted {
		redactionLevel = "basic"
	}

	return fmt.Sprintf("Sending to AI (provider: %s, model: %s)\nRedaction: %s\n\nYour prompt:\n---\n%s\n---\n\nSend to AI? [y/N]: ",
		providerName, modelName, redactionLevel, result.RedactedPrompt)
}
