// Package openai provides an OpenAI-compatible AI provider.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/chazuruo/svf/internal/ai"
	"github.com/chazuruo/svf/internal/workflows"
)

// Provider is an OpenAI-compatible AI provider.
type Provider struct {
	config *ai.Config
	client *http.Client
}

// NewProvider creates a new OpenAI-compatible provider.
func NewProvider(cfg *ai.Config) (*Provider, error) {
	if cfg == nil {
		cfg = ai.DefaultConfig()
	}

	// Get API key from config or environment
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("OPENAI_API_KEY")
		if cfg.APIKey == "" && cfg.BaseURL == "" {
			// Not using OpenAI, probably local
			cfg.APIKey = "not-needed"
		}
	}

	return &Provider{
		config: cfg,
		client: &http.Client{},
	}, nil
}

// Name returns the provider name.
func (p *Provider) Name() string {
	if p.config.BaseURL != "" {
		return "openai-compatible"
	}
	return "openai"
}

// GenerateWorkflow generates a workflow from a prompt.
func (p *Provider) GenerateWorkflow(ctx context.Context, req ai.GenerateRequest) (*workflows.Workflow, error) {
	// Build the prompt
	systemPrompt := `You are a workflow automation assistant. Generate a workflow with clear, executable steps.
The workflow should be in YAML format with the following structure:
- title: Brief descriptive title
- description: What this workflow does
- tags: Relevant tags (comma-separated)
- steps: Array of steps with:
  - name: Step name
  - command: Shell command to execute
  - confirm: Whether to prompt before running (optional)`

	userPrompt := fmt.Sprintf("Generate a workflow for: %s", req.Prompt)
	if req.Context != nil {
		userPrompt += fmt.Sprintf("\n\nContext:")
		if req.Context.CurrentDirectory != "" {
			userPrompt += fmt.Sprintf("\n- Working directory: %s", req.Context.CurrentDirectory)
		}
		if req.Context.Shell != "" {
			userPrompt += fmt.Sprintf("\n- Shell: %s", req.Context.Shell)
		}
		if req.Context.OS != "" {
			userPrompt += fmt.Sprintf("\n- OS: %s", req.Context.OS)
		}
	}

	// Call the API
	response, err := p.callAPI(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, &ai.ExplainError{
			Provider: p.Name(),
			Message:  "failed to generate workflow",
			Cause:    err,
		}
	}

	// Parse the response as YAML
	wf, err := workflows.UnmarshalWorkflow([]byte(response))
	if err != nil {
		// Try to extract YAML from markdown code blocks
		yamlStr := extractYAML(response)
		if yamlStr != "" {
			wf, err = workflows.UnmarshalWorkflow([]byte(yamlStr))
		}
		if err != nil {
			return nil, &ai.ExplainError{
				Provider: p.Name(),
				Message:  "failed to parse generated workflow",
				Cause:    err,
			}
		}
	}

	return wf, nil
}

// Explain provides an explanation for a workflow or command.
func (p *Provider) Explain(ctx context.Context, req ai.ExplainRequest) (string, error) {
	var systemPrompt, userPrompt string

	switch req.Type {
	case ai.ExplainWorkflow:
		systemPrompt = "You are a technical documentation assistant. Explain workflows clearly and concisely."
		userPrompt = fmt.Sprintf("Explain this workflow:\n\nTitle: %s\nDescription: %s\n",
			req.Workflow.Title, req.Workflow.Description)
		for i, step := range req.Workflow.Steps {
			userPrompt += fmt.Sprintf("\nStep %d: %s\nCommand: %s\n", i+1, step.Name, step.Command)
		}

	case ai.ExplainCommand:
		systemPrompt = "You are a command-line expert. Explain shell commands clearly, including what they do and any risks."
		userPrompt = fmt.Sprintf("Explain this command: %s", req.Command)

	case ai.ExplainStep:
		systemPrompt = "You are a command-line expert. Explain shell commands clearly."
		step := req.Workflow.Steps[req.StepIndex]
		userPrompt = fmt.Sprintf("Explain this step:\nName: %s\nCommand: %s", step.Name, step.Command)
	}

	// Add detail level guidance
	switch req.DetailLevel {
	case ai.DetailBrief:
		systemPrompt += " Provide a brief, one-sentence explanation."
	case ai.DetailVerbose:
		systemPrompt += " Provide a detailed explanation with examples and alternatives."
	default:
		systemPrompt += " Provide a standard explanation."
	}

	response, err := p.callAPI(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", &ai.ExplainError{
			Provider: p.Name(),
			Message:  "failed to get explanation",
			Cause:    err,
		}
	}

	return response, nil
}

// chatRequest represents a chat API request.
type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

// message represents a chat message.
type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse represents a chat API response.
type chatResponse struct {
 Choices []choice `json:"choices"`
 Error   *apiError `json:"error,omitempty"`
}

// choice represents a choice in the response.
type choice struct {
	Message message `json:"message"`
}

// apiError represents an API error.
type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// callAPI makes a chat completion API call.
func (p *Provider) callAPI(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Determine base URL
	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	// Build request
	reqBody := chatRequest{
		Model: p.config.Model,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/chat/completions", baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return "", err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if p.config.APIKey != "" && p.config.APIKey != "not-needed" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.config.APIKey))
	}

	// Make request
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", err
	}

	// Check for API error
	if chatResp.Error != nil {
		return "", fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// extractYAML extracts YAML from a markdown code block.
func extractYAML(s string) string {
	// Look for ```yaml or ``` code blocks
	inCodeBlock := false
	var yamlBlock strings.Builder
	var result strings.Builder

	lines := strings.Split(s, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End of code block
				inCodeBlock = false
				if yamlBlock.Len() > 0 {
					return yamlBlock.String()
				}
			} else {
				// Start of code block
				inCodeBlock = true
				yamlBlock.Reset()
			}
		} else if inCodeBlock {
			yamlBlock.WriteString(line)
			yamlBlock.WriteString("\n")
		}
	}

	return result.String()
}

func init() {
	// Register the provider
	ai.RegisterProvider("openai", func(cfg *ai.Config) (ai.Provider, error) {
		return NewProvider(cfg)
	})
	ai.RegisterProvider("ollama", func(cfg *ai.Config) (ai.Provider, error) {
		if cfg == nil {
			cfg = ai.DefaultConfig()
		}
		cfg.Provider = "ollama"
		return NewProvider(cfg)
	})
}
