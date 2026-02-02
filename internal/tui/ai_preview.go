// Package tui provides Bubble Tea models for terminal UI interactions.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// AIPreviewModel is a simple privacy confirmation model for the ask flow.
// It shows the prompt that will be sent to AI and asks for confirmation.
type AIPreviewModel struct {
	Provider    string
	Model       string
	RedactLevel string
	Prompt      string
	confirmed   bool
	canceled    bool

	confirmInput textinput.Model

	// Styles
	headerStyle   lipgloss.Style
	labelStyle    lipgloss.Style
	infoStyle     lipgloss.Style
	successStyle  lipgloss.Style
	warningStyle  lipgloss.Style
	promptBoxStyle lipgloss.Style
}

// NewAIPreviewModel creates a new AI preview model.
func NewAIPreviewModel(provider, model, redactLevel, prompt string) AIPreviewModel {
	ti := textinput.New()
	ti.Placeholder = "Type 'y' to confirm"
	ti.Focus()
	ti.CharLimit = 1

	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242"))

	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	successStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	warningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).
		Bold(true)

	promptBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1)

	return AIPreviewModel{
		Provider:       provider,
		Model:          model,
		RedactLevel:    redactLevel,
		Prompt:         prompt,
		confirmInput:   ti,
		headerStyle:    headerStyle,
		labelStyle:     labelStyle,
		infoStyle:      infoStyle,
		successStyle:   successStyle,
		warningStyle:   warningStyle,
		promptBoxStyle: promptBoxStyle,
	}
}

// Init implements tea.Model.
func (m AIPreviewModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m AIPreviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "n", "N":
			m.canceled = true
			return m, tea.Quit

		case "y", "Y":
			m.confirmed = true
			return m, tea.Quit

		case "enter":
			// Check if user typed 'y' in the input
			val := strings.ToLower(m.confirmInput.Value())
			if val == "y" || val == "yes" {
				m.confirmed = true
				return m, tea.Quit
			}
			m.canceled = true
			return m, tea.Quit
		}

		var cmd tea.Cmd
		m.confirmInput, cmd = m.confirmInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View implements tea.Model.
func (m AIPreviewModel) View() string {
	var b strings.Builder

	// Header
	b.WriteString(m.headerStyle.Render("Sending to AI"))
	b.WriteString("\n\n")

	// Provider info line
	b.WriteString(fmt.Sprintf("Sending to AI (provider: %s, model: %s)\n", m.Provider, m.Model))

	// Redaction level
	b.WriteString(fmt.Sprintf("Redaction: %s", m.RedactLevel))
	if m.RedactLevel == "none" {
		b.WriteString(m.warningStyle.Render(" ⚠️ no redaction"))
	}
	b.WriteString("\n\n")

	// Your prompt section
	b.WriteString(m.labelStyle.Render("Your prompt:"))
	b.WriteString("\n")
	b.WriteString(m.infoStyle.Render("---"))
	b.WriteString("\n")

	// The prompt content (truncated if too long)
	promptText := m.Prompt
	maxLen := 500
	if len(promptText) > maxLen {
		promptText = promptText[:maxLen] + m.infoStyle.Render("...\n(truncated)")
	}
	b.WriteString(promptText)
	b.WriteString("\n")

	b.WriteString(m.infoStyle.Render("---"))
	b.WriteString("\n\n")

	// Confirmation prompt
	b.WriteString(fmt.Sprintf("Send to AI? [y/N]: "))
	b.WriteString(m.confirmInput.View())
	b.WriteString("\n\n")

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)

	b.WriteString(helpStyle.Render("Press 'y' or Enter to confirm, 'n' or Ctrl+C to cancel"))

	return b.String()
}

// DidConfirm returns true if user confirmed.
func (m *AIPreviewModel) DidConfirm() bool {
	return m.confirmed
}

// DidCancel returns true if user canceled.
func (m *AIPreviewModel) DidCancel() bool {
	return m.canceled
}
