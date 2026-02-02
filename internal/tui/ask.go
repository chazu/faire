// Package tui provides Bubble Tea models for terminal UI interactions.
package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chazuruo/svf/internal/ai"
	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/workflows"
)

// AskState represents the current state of the ask flow.
type AskState int

const (
	// AskStatePrompting means user is entering their prompt.
	AskStatePrompting AskState = iota
	// AskStateRedacting means user is reviewing/redacting sensitive data.
	AskStateRedacting
	// AskStatePrivacyConfirming means user is confirming to send to AI.
	AskStatePrivacyConfirming
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

	// Redaction model
	redactionModel RedactionModel

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
	Redact    string // "none", "basic", "strict"
	As        string // "workflow" or "step"
	Identity  string
	NoCommit  bool
}

// NewAskModel creates a new ask model.
func NewAskModel(ctx context.Context, cfg *config.Config, opts *AskOptions) *AskModel {
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

	case AskStateRedacting:
		var cmd tea.Cmd
		var model tea.Model
		model, cmd = m.redactionModel.Update(msg)
		m.redactionModel = model.(RedactionModel)
		cmds = append(cmds, cmd)

		// Check if redaction is done
		if m.redactionModel.DidConfirm() {
			// User confirmed, proceed to privacy confirmation
			m.state = AskStatePrivacyConfirming
			return m, nil
		}
		if m.redactionModel.DidCancel() {
			// User canceled, go back to prompt
			m.state = AskStatePrompting
			m.redactionModel = RedactionModel{}
			return m, nil
		}

	case AskStatePrivacyConfirming:
		// Privacy confirmation state - handled in handleKey
		// Just wait for user input

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
			// Submit prompt and move to redaction
			prompt := m.promptInput.Value()
			if strings.TrimSpace(prompt) == "" {
				m.errorMsg = "Please enter a description"
				return m, nil
		}
			m.errorMsg = ""
			m.promptInput.Blur()

			// Move to redaction
			m.redactionModel = NewRedactionModel(prompt)
			m.state = AskStateRedacting
			return m, nil
		}

	case tea.KeyEnter:
		if m.state == AskStatePrivacyConfirming {
			// User confirmed, proceed to generation
			m.state = AskStateGenerating
			return m, m.generateWorkflow()
		}

	case tea.KeyEsc:
		if m.state == AskStatePrivacyConfirming {
			// User went back, return to redaction
			m.state = AskStateRedacting
			return m, nil
		}

	case 'y', 'Y':
		if m.state == AskStatePrivacyConfirming {
			// User confirmed, proceed to generation
			m.state = AskStateGenerating
			return m, m.generateWorkflow()
		}

	case 'n', 'N':
		if m.state == AskStatePrivacyConfirming {
			// User declined, return to redaction
			m.state = AskStateRedacting
			return m, nil
		}
	}

	return m, nil
}

// generateWorkflow is a tea.Cmd that generates the workflow.
func (m *AskModel) generateWorkflow() tea.Cmd {
	return func() tea.Msg {
		// Get redacted prompt
		prompt := m.redactionModel.GetRedactedContent()

		// Build AI config
		aiCfg := m.buildAIConfig()

		// Create provider
		provider, err := ai.NewProvider(aiCfg)
		if err != nil {
			return generateWorkflowMsg{Error: err}
		}

		if provider == nil {
			return generateWorkflowMsg{Error: fmt.Errorf("AI provider not configured")}
		}

		m.provider = provider

		// Generate workflow
		req := ai.GenerateRequest{
			Prompt: prompt,
			Options: ai.GenerateOptions{
				IncludePlaceholders: true,
			},
		}

		wf, err := provider.GenerateWorkflow(m.ctx, req)
		if err != nil {
			return generateWorkflowMsg{Error: err}
		}

		return generateWorkflowMsg{Workflow: wf}
	}
}

// buildAIConfig builds AI config from options and global config.
func (m *AskModel) buildAIConfig() *ai.Config {
	aiCfg := ai.DefaultConfig()

	// Apply options
	if m.opts.Provider != "" {
		aiCfg.Provider = m.opts.Provider
	}
	if m.opts.Model != "" {
		aiCfg.Model = m.opts.Model
	}
	if m.opts.APIKeyEnv != "" {
		aiCfg.APIKey = os.Getenv(m.opts.APIKeyEnv)
	}

	// Apply global config if available
	if m.cfg.AI.Provider != "" {
		aiCfg.Provider = m.cfg.AI.Provider
	}
	if m.cfg.AI.Model != "" {
		aiCfg.Model = m.cfg.AI.Model
	}
	if m.cfg.AI.BaseURL != "" {
		aiCfg.BaseURL = m.cfg.AI.BaseURL
	}

	return aiCfg
}

// View renders the ask model.
func (m *AskModel) View() string {
	switch m.state {
	case AskStatePrompting:
		return m.renderPromptView()
	case AskStateRedacting:
		return m.redactionModel.View()
	case AskStatePrivacyConfirming:
		return m.renderPrivacyConfirmView()
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

// renderPrivacyConfirmView renders the privacy confirmation view.
func (m *AskModel) renderPrivacyConfirmView() string {
	var b strings.Builder

	b.WriteString(m.headerStyle.Render("🔐 Confirm Sending to AI"))
	b.WriteString("\n\n")

	// Warning
	warningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).
		Bold(true)
	b.WriteString(warningStyle.Render("⚠️  Your data will be sent to an external AI provider."))
	b.WriteString("\n\n")

	// Provider info section
	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)
	b.WriteString(infoStyle.Render("Provider Configuration:"))
	b.WriteString("\n\n")

	// Build AI config to get the actual values
	aiCfg := m.buildAIConfig()
	providerName := aiCfg.Provider
	if providerName == "" {
		providerName = m.cfg.AI.Provider
	}
	modelName := aiCfg.Model
	if modelName == "" {
		modelName = m.cfg.AI.Model
	}

	// Redaction level
	redactLevel := m.cfg.AI.Redact
	if m.opts.Redact != "" {
		redactLevel = m.opts.Redact
	}
	if redactLevel == "" {
		redactLevel = "basic"
	}

	statStyle := lipgloss.NewStyle().
		Width(35)

	b.WriteString(statStyle.Render(fmt.Sprintf("  Provider: %s", providerName)))
	b.WriteString(statStyle.Render(fmt.Sprintf("  Model: %s", modelName)))
	b.WriteString(statStyle.Render(fmt.Sprintf("  Redaction: %s", redactLevel)))
	b.WriteString("\n\n")

	// Content preview
	b.WriteString(infoStyle.Render("Your Prompt:"))
	b.WriteString("\n\n")

	// Show redacted content
	redactedContent := m.redactionModel.GetRedactedContent()

	// Create a box for the content
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1)

	// Truncate if too long
	maxPreviewLen := 500
	preview := redactedContent
	if len(preview) > maxPreviewLen {
		preview = preview[:maxPreviewLen] + "\n... (truncated)"
	}

	b.WriteString(boxStyle.Render(preview))
	b.WriteString("\n\n")

	// Confirmation prompt
	b.WriteString("Send to AI? ")
	confirmStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)
	b.WriteString(confirmStyle.Render("[y/N]"))
	b.WriteString("\n\n")

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	b.WriteString(footerStyle.Render(
		" [y]: Yes, send  [n/ESC]: No, go back  [Ctrl+C]: quit",
	))

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
	if m.provider != nil {
		providerName = m.provider.Name()
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

	if m.canceled {
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
