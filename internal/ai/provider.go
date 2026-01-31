// Package ai provides AI provider interfaces for workflow generation and explanation.
package ai

import (
	"context"
	"fmt"
	"regexp"

	"github.com/chazuruo/svf/internal/workflows"
)

// Provider is an AI provider that can generate and explain workflows.
type Provider interface {
	// Name returns the provider name.
	Name() string

	// GenerateWorkflow generates a workflow from a prompt.
	GenerateWorkflow(ctx context.Context, prompt GenerateRequest) (*workflows.Workflow, error)

	// Explain provides an explanation for a workflow or command.
	Explain(ctx context.Context, input ExplainRequest) (string, error)
}

// GenerateRequest contains parameters for workflow generation.
type GenerateRequest struct {
	// Prompt is the user's description of what they want.
	Prompt string

	// Context provides additional context about the user's environment.
	Context *GenerateContext

	// Options for generation.
	Options GenerateOptions
}

// GenerateContext provides environmental context.
type GenerateContext struct {
	// CurrentDirectory is the user's working directory.
	CurrentDirectory string

	// Shell is the user's shell (bash, zsh, etc.).
	Shell string

	// OS is the operating system.
	OS string

	// Architecture is the system architecture.
	Architecture string

	// ExistingWorkflows are workflows the user has for reference.
	ExistingWorkflows []*workflows.Workflow
}

// GenerateOptions controls generation behavior.
type GenerateOptions struct {
	// Steps suggests how many steps to include (0 for auto).
	Steps int

	// IncludePlaceholders whether to add placeholders.
	IncludePlaceholders bool

	// Style preference for the workflow (concise, verbose, etc.).
	Style string
}

// ExplainRequest contains parameters for explanation.
type ExplainRequest struct {
	// Type is what to explain.
	Type ExplainType

	// Workflow is the workflow to explain (if Type is ExplainWorkflow).
	Workflow *workflows.Workflow

	// Command is the command to explain (if Type is ExplainCommand).
	Command string

	// StepIndex is the specific step to explain (optional).
	StepIndex int

	// DetailLevel controls explanation depth.
	DetailLevel DetailLevel
}

// ExplainType is what to explain.
type ExplainType int

const (
	// ExplainWorkflow explains an entire workflow.
	ExplainWorkflow ExplainType = iota

	// ExplainCommand explains a single command.
	ExplainCommand

	// ExplainStep explains a specific workflow step.
	ExplainStep
)

// DetailLevel controls explanation depth.
type DetailLevel int

const (
	// DetailBrief provides a short summary.
	DetailBrief DetailLevel = iota

	// DetailNormal provides standard explanation.
	DetailNormal

	// DetailVerbose provides detailed explanation with examples.
	DetailVerbose
)

// Config contains provider configuration.
type Config struct {
	// Provider is the provider name (openai, ollama, etc.).
	Provider string

	// APIKey is the API key for the provider.
	APIKey string

	// BaseURL is the base URL for the API (for Ollama or custom endpoints).
	BaseURL string

	// Model is the model to use.
	Model string

	// Temperature controls randomness (0.0 to 1.0).
	Temperature float64

	// MaxTokens is the maximum tokens to generate.
	MaxTokens int
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	return &Config{
		Provider:    "ollama",
		BaseURL:     "http://localhost:11434",
		Model:       "llama2",
		Temperature: 0.7,
		MaxTokens:   2000,
	}
}

// Factory creates a provider from configuration.
type Factory func(cfg *Config) (Provider, error)

var providers = make(map[string]Factory)

// RegisterProvider registers a provider factory.
func RegisterProvider(name string, factory Factory) {
	providers[name] = factory
}

// NewProvider creates a provider from configuration.
func NewProvider(cfg *Config) (Provider, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	factory, ok := providers[cfg.Provider]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}

	return factory(cfg)
}

// ExplainError is an error from the provider.
type ExplainError struct {
	Provider string
	Message  string
	Cause    error
}

func (e *ExplainError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s provider error: %s: %v", e.Provider, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s provider error: %s", e.Provider, e.Message)
}

// Unwrap returns the underlying cause.
func (e *ExplainError) Unwrap() error {
	return e.Cause
}

// Redact redacts sensitive information from prompts.
// This provides automatic redaction using the same detection patterns as the TUI.
func Redact(s string) string {
	// Import redaction patterns from TUI package
	// For now, we'll use a simple implementation that can be enhanced
	if s == "" {
		return s
	}

	// Use the detectSensitiveItems function from the TUI package
	// Since we're in a different package, we'll re-implement the patterns here
	patterns := []string{
		// API Keys
		`(?i)(api[_-]?key|apikey|key)[\"']?\s*[:=]\s*[\"']?[a-zA-Z0-9_\-]{20,}[\"']?`,
		`(?i)sk-[a-zA-Z0-9]{20,}`,
		`(?i)pk-[a-zA-Z0-9]{20,}`,
		`(?i)AKIA[0-9A-Z]{16}`,
		// Passwords
		`(?i)(password|passwd|pass)[\"']?\s*[:=]\s*[\"']?[^\s\"']+[\"']?`,
		// Tokens
		`(?i)(token|access[_-]?token|refresh[_-]?token)[\"']?\s*[:=]\s*[\"']?[a-zA-Z0-9_\-\.~=]{20,}[\"']?`,
		`(?i)gh[pousr]_[a-zA-Z0-9]{36,}`,
		// Bearer tokens
		`(?i)bearer\s+[a-zA-Z0-9_\-\.~=]+`,
		// Secrets
		`(?i)(secret|secret[_-]?key|secret[_-]?id)[\"']?\s*[:=]\s*[\"']?[a-zA-Z0-9_\-]{16,}[\"']?`,
		// Email
		`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`,
		// Private keys
		`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`,
	}

	result := s
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		result = re.ReplaceAllString(result, "<REDACTED>")
	}

	return result
}
