// Package tui provides Bubble Tea models for terminal UI interactions.
package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chazuruo/svf/internal/ai"
	"github.com/chazuruo/svf/internal/app"
	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/workflows"
)

// AskState represents the current state of the ask flow.
type AskState int

const (
	// AskStatePrompting means user is entering their prompt.
	AskStatePrompting AskState = iota
	// AskStatePreviewing means user is previewing what will be sent to AI.
	AskStatePreviewing
	// AskStateGenerating means AI is generating the workflow.
	AskStateGenerating
	// AskStateReviewing means user is reviewing the generated workflow.
	AskStateReviewing
	// AskStateFinished means the flow is complete.
	AskStateFinished
)

// AskModel is the main model for the ask command TUI.
type AskModel struct {
	ctx    context.Context
	cfg    *config.Config
	opts   *AskOptions
	state  AskState

	// Prompt input
	promptInput textarea.Model

	// AI preview model (simple privacy confirmation)
	aiPreviewModel AIPreviewModel

	// Generated workflow
	generatedWorkflow *workflows.Workflow

	// Workflow editor
	workflowEditor WorkflowEditorModel

	// AI provider
	provider ai.Provider

	// Error state
	errorMsg string
	quit     bool
	saved    bool
	canceled bool

	// Styles
	headerStyle  lipgloss.Style
	labelStyle   lipgloss.Style
	infoStyle    lipgloss.Style
	errorStyle   lipgloss.Style
	successStyle lipgloss.Style
}

// AskOptions contains options for the ask command.
type AskOptions struct {
	Provider  string
	Model     string
	APIKeyEnv string
	As        string // "workflow" or "step"
	Identity  string
	Redact    app.RedactionLevel
	NoCommit  bool
}

// NewAskModel creates a new ask model.
func NewAskModel(ctx context.Context, cfg *config.Config, opts *AskOptions) *AskModel {
	// Check if AI is configured
	if err := app.CheckConfig(cfg); err != nil {
		// Return a model that will show the error
		ti := textarea.New()
		ti.SetHeight(8)

		headerStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("203")).
			Bold(true)

		return &AskModel{
			ctx:       ctx,
			cfg:       cfg,
			opts:      opts,
			state:     AskStateFinished,
			canceled:  true,
			errorMsg:  err.Error(),
			promptInput: ti,
			headerStyle: headerStyle,
			errorStyle: errorStyle,
		}
	}

	// Create prompt textarea
	ti := textarea.New()
	ti.Placeholder = "Describe the workflow or step you want to create..."
	ti.SetHeight(8)
	ti.Focus()
	ti.ShowLineNumbers = false

	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Width(15)

	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("203")).
		Bold(true)

	successStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	return &AskModel{
		ctx:          ctx,
		cfg:          cfg,
		opts:         opts,
		state:        AskStatePrompting,
		promptInput:  ti,
		headerStyle:  headerStyle,
		labelStyle:   labelStyle,
		infoStyle:    infoStyle,
		errorStyle:   errorStyle,
		successStyle: successStyle,
	}
}

// Init initializes the ask model.
func (m *AskModel) Init() tea.Cmd {
	return textarea.Blink
}

// Update updates the ask model.
func (m *AskModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case generateWorkflowMsg:
		// Workflow generation complete
		if msg.Error != nil {
			m.errorMsg = fmt.Sprintf("Failed to generate workflow: %v", msg.Error)
			m.state = AskStatePrompting
			return m, nil
		}
		m.generatedWorkflow = msg.Workflow
		m.state = AskStateReviewing
		m.workflowEditor = NewWorkflowEditor(m.ctx, msg.Workflow)
		return m, nil
	}

	// Update child components based on state
	switch m.state {
	case AskStatePrompting:
		var cmd tea.Cmd
		m.promptInput, cmd = m.promptInput.Update(msg)
		cmds = append(cmds, cmd)

	case AskStatePreviewing:
		var cmd tea.Cmd
		var model tea.Model
		model, cmd = m.aiPreviewModel.Update(msg)
		m.aiPreviewModel = model.(AIPreviewModel)
		cmds = append(cmds, cmd)

		// Check if preview is done
		if m.aiPreviewModel.DidConfirm() {
			// User confirmed, proceed to generation
			m.state = AskStateGenerating
			return m, m.generateWorkflow()
		}
		if m.aiPreviewModel.DidCancel() {
			// User canceled, go back to prompt
			m.state = AskStatePrompting
			m.promptInput.Focus()
			return m, nil
		}

	case AskStateReviewing:
		var cmd tea.Cmd
		var model tea.Model
		model, cmd = m.workflowEditor.Update(msg)
		m.workflowEditor = model.(WorkflowEditorModel)
		cmds = append(cmds, cmd)

		// Check if editor is done
		if m.workflowEditor.DidSave() {
			m.saved = true
			m.generatedWorkflow = m.workflowEditor.GetWorkflow()
			m.state = AskStateFinished
			m.quit = true
			return m, tea.Quit
		}
		if m.workflowEditor.DidQuit() {
			m.canceled = true
			m.state = AskStateFinished
			m.quit = true
			return m, tea.Quit
		}
	}

	return m, tea.Batch(cmds...)
}

// handleKey handles key messages.
func (m *AskModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.canceled = true
		m.state = AskStateFinished
		return m, tea.Quit

	case tea.KeyCtrlS:
		if m.state == AskStatePrompting {
			// Submit prompt and move to preview
			prompt := m.promptInput.Value()
			if strings.TrimSpace(prompt) == "" {
				m.errorMsg = "Please enter a description"
				return m, nil
			}
			m.errorMsg = ""
			m.promptInput.Blur()

			// Build provider info for preview
			provider := m.opts.Provider
			if provider == "" {
				provider = m.cfg.AI.Provider
			}
			if provider == "" {
				provider = "openai_compat"
			}

			model := m.opts.Model
			if model == "" {
				model = m.cfg.AI.Model
			}
			if model == "" {
				model = "gpt-4o-mini"
			}

			// Apply redaction if needed
			redactedPrompt := prompt
			if m.opts.Redact != app.RedactNone {
				redactedPrompt = ai.Redact(prompt)
			}

			// Move to preview
			m.aiPreviewModel = NewAIPreviewModel(provider, model, string(m.opts.Redact), redactedPrompt)
			m.state = AskStatePreviewing
			return m, nil
		}
	}

	return m, nil
}

// generateWorkflow is a tea.Cmd that generates the workflow.
func (m *AskModel) generateWorkflow() tea.Cmd {
	return func() tea.Msg {
		// Get the prompt from the preview (already redacted)
		prompt := m.aiPreviewModel.Prompt

		// Build app options
		appOpts := &app.AskOptions{
			Prompt:    prompt,
			Provider:  m.opts.Provider,
			Model:     m.opts.Model,
			APIKeyEnv: m.opts.APIKeyEnv,
			As:        m.opts.As,
			Identity:  m.opts.Identity,
			Redact:    m.opts.Redact,
		}

		// Generate workflow using app layer
		result, err := app.GenerateWorkflow(m.ctx, m.cfg, appOpts)
		if err != nil {
			return generateWorkflowMsg{Error: err}
		}

		return generateWorkflowMsg{Workflow: result.Workflow}
	}
}

// View renders the ask model.
func (m *AskModel) View() string {
	switch m.state {
	case AskStatePrompting:
		return m.renderPromptView()
	case AskStatePreviewing:
		return m.aiPreviewModel.View()
	case AskStateGenerating:
		return m.renderGeneratingView()
	case AskStateReviewing:
		return m.workflowEditor.View()
	case AskStateFinished:
		return m.renderFinishedView()
	}
	return ""
}

// renderPromptView renders the prompt input view.
func (m *AskModel) renderPromptView() string {
	var b strings.Builder

	b.WriteString(m.headerStyle.Render("✨ Generate Workflow with AI"))
	b.WriteString("\n\n")

	b.WriteString("Describe the workflow or step you want to create.\n")
	b.WriteString("Be specific about what you want to accomplish.\n\n")

	// Error message
	if m.errorMsg != "" {
		b.WriteString(m.errorStyle.Render("⚠️  " + m.errorMsg))
		b.WriteString("\n\n")
	}

	// Prompt input
	b.WriteString(m.labelStyle.Render("Description:"))
	b.WriteString("\n")
	b.WriteString(m.promptInput.View())
	b.WriteString("\n\n")

	// Redaction info
	redactInfo := fmt.Sprintf("Redaction: %s", m.opts.Redact)
	if m.opts.Redact == app.RedactNone {
		redactInfo += " (no data will be redacted)"
	}
	b.WriteString(m.infoStyle.Render(redactInfo))
	b.WriteString("\n\n")

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)

	footer := footerStyle.Render(
		" [Ctrl+S]: submit and continue  [Ctrl+C]: quit",
	)

	b.WriteString(footer)

	return b.String()
}

// renderGeneratingView renders the generating view.
func (m *AskModel) renderGeneratingView() string {
	var b strings.Builder

	b.WriteString(m.headerStyle.Render("🔄 Generating Workflow"))
	b.WriteString("\n\n")

	b.WriteString("Sending your description to the AI provider...\n\n")

	// Show provider info
	providerName := "unknown"
	if m.opts.Provider != "" {
		providerName = m.opts.Provider
	} else if m.cfg.AI.Provider != "" {
		providerName = m.cfg.AI.Provider
	}
	b.WriteString(m.infoStyle.Render(fmt.Sprintf("Provider: %s", providerName)))
	b.WriteString("\n")

	b.WriteString(m.infoStyle.Render("This may take a few moments..."))
	b.WriteString("\n\n")

	return lipgloss.NewStyle().
		Width(60).
		Align(lipgloss.Center, lipgloss.Center).
		Render(b.String())
}

// renderFinishedView renders the finished state.
func (m *AskModel) renderFinishedView() string {
	var b strings.Builder

	b.WriteString("\n\n")

	if m.errorMsg != "" && m.canceled {
		b.WriteString(m.errorStyle.Render("⚠️  AI Configuration Error"))
		b.WriteString("\n\n")
		b.WriteString(m.errorMsg)
	} else if m.canceled {
		b.WriteString(m.headerStyle.Render("✖ Canceled"))
		b.WriteString("\n\n")
		b.WriteString("The operation was canceled.")
	} else if m.saved {
		b.WriteString(m.successStyle.Render("✓ Workflow Saved"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("Title: %s", m.generatedWorkflow.Title))
		b.WriteString("\n")
		if m.generatedWorkflow.Description != "" {
			b.WriteString(fmt.Sprintf("Description: %s", m.generatedWorkflow.Description))
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("Steps: %d", len(m.generatedWorkflow.Steps)))
	}

	b.WriteString("\n\n")

	return lipgloss.NewStyle().
		Width(60).
		Align(lipgloss.Center, lipgloss.Center).
		Render(b.String())
}

// GetWorkflow returns the generated workflow.
func (m *AskModel) GetWorkflow() *workflows.Workflow {
	return m.generatedWorkflow
}

// DidSave returns true if the user saved the workflow.
func (m *AskModel) DidSave() bool {
	return m.saved
}

// DidCancel returns true if the user canceled.
func (m *AskModel) DidCancel() bool {
	return m.canceled
}

// generateWorkflowMsg is a message sent when workflow generation is complete.
type generateWorkflowMsg struct {
	Workflow *workflows.Workflow
	Error    error
}
