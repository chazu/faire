// Package tui provides Bubble Tea models for svf.
package tui

import (
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// RedactionModel is a Bubble Tea model for redacting sensitive data.
type RedactionModel struct {
	// Content is the original content to be redacted.
	Content string

	// RedactedContent is the content after redaction.
	RedactedContent string

	// State is the current redaction state.
	State RedactionState

	// List is the list of detected sensitive items.
	List list.Model

	// Textarea is for viewing/editing content.
	Textarea textarea.Model

	// ConfirmInput is for confirmation.
	ConfirmInput textinput.Model

	// Confirmed indicates if user has confirmed.
	Confirmed bool
	// Canceled indicates if user canceled.
	Canceled bool

	// styles
	normalStyle    lipgloss.Style
	selectedStyle  lipgloss.Style
	redactedStyle  lipgloss.Style
	warningStyle   lipgloss.Style
	headerStyle    lipgloss.Style

	// width and height
	width  int
	height int
}

// RedactionState represents the current redaction state.
type RedactionState int

const (
	// RedactionStateReviewing means user is reviewing content.
	RedactionStateReviewing RedactionState = iota
	// RedactionStateRedacting means user is redacting items.
	RedactionStateRedacting
	// RedactionStateConfirming means user is confirming to send.
	RedactionStateConfirming
	// RedactionStateFinished means redaction is complete.
	RedactionStateFinished
)

// RedactedItem represents a detected sensitive item.
type RedactedItem struct {
	Original string
	Redacted string
	Type     string // "api-key", "password", "token", "email", etc.
}

// NewRedactionModel creates a new redaction model.
func NewRedactionModel(content string) RedactionModel {
	// Detect sensitive items
	items := detectSensitiveItems(content)

	// Create list
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = redactionItem{
			index:    i,
			original: item.Original,
			redacted: item.Original,
		 itemType: item.Type,
		}
	}

	l := list.New(listItems, redactionDelegate{}, 0, 0)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	// Create textarea
	ta := textarea.New()
	ta.SetValue(content)
	ta.ShowLineNumbers = false
	ta.SetWidth(80)
	ta.SetHeight(20)

	// Create confirm input
	ti := textinput.New()
	ti.Placeholder = "Type 'confirm' to send data to AI"
	ti.Focus()

	// Styles
	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("251"))
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Bold(true)
	redactedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Background(lipgloss.Color("235"))
	warningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).
		Bold(true)
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	return RedactionModel{
		Content:         content,
		RedactedContent: content,
		State:          RedactionStateReviewing,
		List:           l,
		Textarea:       ta,
		ConfirmInput:   ti,
		normalStyle:    normalStyle,
		selectedStyle:  selectedStyle,
		redactedStyle:  redactedStyle,
		warningStyle:   warningStyle,
		headerStyle:    headerStyle,
	}
}

// Init implements tea.Model.
func (m RedactionModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m RedactionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.Canceled = true
			m.State = RedactionStateFinished
			return m, tea.Quit

		case "enter":
			if m.State == RedactionStateReviewing {
				// Move to redaction state
				m.State = RedactionStateRedacting
			} else if m.State == RedactionStateConfirming {
				// Check confirmation
				if strings.ToLower(m.ConfirmInput.Value()) == "confirm" {
					m.Confirmed = true
					m.State = RedactionStateFinished
					return m, tea.Quit
				}
			}

		case "r":
			if m.State == RedactionStateReviewing {
				// Redact selected item
				if len(m.List.Items()) > 0 {
					item := m.List.SelectedItem().(redactionItem)
					item.redacted = "<REDACTED>"
					m.List.SetItem(m.List.Index(), item)
					m.updateRedactedContent()
				}
			}

		case "e":
			if m.State == RedactionStateReviewing {
				// Edit selected item manually
				if len(m.List.Items()) > 0 {
					// For now, just mark for manual edit
					// TODO: Implement proper edit UI
				}
			}

		case "c":
			if m.State == RedactionStateRedacting {
				// Move to confirmation state
				m.State = RedactionStateConfirming
				m.ConfirmInput.Focus()
			}

		case "esc":
			if m.State == RedactionStateRedacting {
				// Go back to reviewing
				m.State = RedactionStateReviewing
			}
		}
	}

	// Update child components based on state
	if m.State == RedactionStateReviewing || m.State == RedactionStateRedacting {
		var listCmd tea.Cmd
		m.List, listCmd = m.List.Update(msg)
		if listCmd != nil {
			cmds = append(cmds, listCmd)
		}
	}

	if m.State == RedactionStateConfirming {
		var inputCmd tea.Cmd
		m.ConfirmInput, inputCmd = m.ConfirmInput.Update(msg)
		if inputCmd != nil {
			cmds = append(cmds, inputCmd)
		}
	}

	var taCmd tea.Cmd
	m.Textarea, taCmd = m.Textarea.Update(msg)
	if taCmd != nil {
		cmds = append(cmds, taCmd)
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m RedactionModel) View() string {
	if m.State == RedactionStateFinished {
		return m.finishedView()
	}

	// Layout based on state
	switch m.State {
	case RedactionStateReviewing, RedactionStateRedacting:
		return m.reviewView()
	case RedactionStateConfirming:
		return m.confirmView()
	default:
		return ""
	}
}

// reviewView shows the content and detected sensitive items.
func (m RedactionModel) reviewView() string {
	var b strings.Builder

	b.WriteString(m.headerStyle.Render("🔒 Data Redaction"))
	b.WriteString("\n\n")

	// Instructions
	if m.State == RedactionStateReviewing {
		b.WriteString("Review detected sensitive data:\n\n")
		b.WriteString("  [r] Redact selected  [e] Edit  [c] Continue to confirmation  [q] Quit\n\n")
	} else {
		b.WriteString("Redaction complete. Review changes:\n\n")
		b.WriteString("  [c] Confirm to send  [esc] Go back  [q] Quit\n\n")
	}

	// Left panel: detected items
	leftPanel := m.itemListView()

	// Right panel: content preview
	rightPanel := m.contentPreviewView()

	// Join panels
	layout := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	b.WriteString(layout)

	return b.String()
}

// confirmView shows the confirmation dialog.
func (m RedactionModel) confirmView() string {
	var b strings.Builder

	b.WriteString(m.headerStyle.Render("⚠️  Confirm Sending to AI"))
	b.WriteString("\n\n")

	b.WriteString(m.warningStyle.Render("WARNING: This data will be sent to an external AI provider."))
	b.WriteString("\n\n")

	b.WriteString("Summary:\n")
	b.WriteString(fmt.Sprintf("  Original length: %d characters\n", len(m.Content)))
	b.WriteString(fmt.Sprintf("  Redacted length: %d characters\n", len(m.RedactedContent)))

	if m.RedactedContent != m.Content {
		b.WriteString("  Status: Some data has been redacted\n")
	} else {
		b.WriteString("  Status: No changes made (data will be sent as-is)\n")
	}

	b.WriteString("\n")
	b.WriteString("Redacted content preview:\n")
	b.WriteString("```\n")

	// Show preview
	preview := m.RedactedContent
	if len(preview) > 500 {
		preview = preview[:500] + "..."
	}
	b.WriteString(preview)

	b.WriteString("\n```\n\n")

	b.WriteString("Type 'confirm' to send, or [q] to quit:\n\n")
	b.WriteString(m.ConfirmInput.View())

	return b.String()
}

// finishedView shows the final state.
func (m RedactionModel) finishedView() string {
	var b strings.Builder

	if m.Canceled {
		b.WriteString("\n Redaction canceled. No data was sent.\n\n")
	} else if m.Confirmed {
		b.WriteString("\n ✓ Data confirmed and sent to AI.\n\n")
	}

	b.WriteString("Press Enter to exit...\n")

	return lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center, lipgloss.Center).
		Render(b.String())
}

// itemListView renders the list of detected items.
func (m RedactionModel) itemListView() string {
	var b strings.Builder

	b.WriteString(" Detected Items\n\n")

	if len(m.List.Items()) == 0 {
		b.WriteString(" No sensitive items detected.")
	} else {
		b.WriteString(m.List.View())
	}

	width := 40
	return lipgloss.NewStyle().
		Width(width).
		Height(m.height - 10).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Render(b.String())
}

// contentPreviewView renders the content preview.
func (m RedactionModel) contentPreviewView() string {
	var b strings.Builder

	b.WriteString(" Content Preview\n\n")

	// Show redacted content with highlighting
	content := m.RedactedContent
	if m.State == RedactionStateRedacting {
		// Highlight redacted items
		content = strings.ReplaceAll(content, "<REDACTED>", m.redactedStyle.Render("<REDACTED>"))
	}

	b.WriteString(content)

	width := m.width - 50
	if width < 40 {
		width = 40
	}
	return lipgloss.NewStyle().
		Width(width).
		Height(m.height - 10).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Render(b.String())
}

// updateRedactedContent updates the redacted content from list items.
func (m *RedactionModel) updateRedactedContent() {
	content := m.Content

	for _, item := range m.List.Items() {
		ri := item.(redactionItem)
		if ri.redacted != ri.original {
			content = strings.ReplaceAll(content, ri.original, ri.redacted)
		}
	}

	m.RedactedContent = content
	m.Textarea.SetValue(content)
}

// GetRedactedContent returns the redacted content.
func (m *RedactionModel) GetRedactedContent() string {
	return m.RedactedContent
}

// DidConfirm returns true if user confirmed.
func (m *RedactionModel) DidConfirm() bool {
	return m.Confirmed
}

// DidCancel returns true if user canceled.
func (m *RedactionModel) DidCancel() bool {
	return m.Canceled
}

// redactionItem is a list item for a detected sensitive item.
type redactionItem struct {
	index    int
	original string
	redacted string
	itemType string
}

func (r redactionItem) FilterValue() string {
	return r.original
}

// Title implements list.Item.
func (r redactionItem) Title() string {
	if r.redacted == "<REDACTED>" {
		return fmt.Sprintf("[%s] <REDACTED>", r.itemType)
	}
	return fmt.Sprintf("[%s] %s", r.itemType, r.original)
}

// Description implements list.Item.
func (r redactionItem) Description() string {
	return ""
}

// redactionDelegate defines how items are rendered in the list.
type redactionDelegate struct{}

func (d redactionDelegate) Height() int { return 1 }
func (d redactionDelegate) Spacing() int  { return 0 }
func (d redactionDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d redactionDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	r, ok := listItem.(redactionItem)
	if !ok {
		return
	}

	// Styles
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("251"))

	var text string
	if index == m.Index() {
		text = selectedStyle.Render("→ " + r.Title())
	} else {
		text = normalStyle.Render("  " + r.Title())
	}

	fmt.Fprint(w, text)
}

// detectSensitiveItems detects potential sensitive items in content.
func detectSensitiveItems(content string) []RedactedItem {
	var items []RedactedItem

	// Simple detection patterns
	// TODO: Add more sophisticated detection

	// Look for API keys (common patterns)
	if strings.Contains(content, "api_key") || strings.Contains(content, "apiKey") {
		// Extract key values
		// This is a simplified detection
	}

	// Look for passwords
	if strings.Contains(content, "password") || strings.Contains(content, "PASS") {
		// Extract password values
	}

	// Look for tokens
	if strings.Contains(content, "token") || strings.Contains(content, "TOKEN") {
		// Extract token values
	}

	// Look for emails
	// Simple email pattern
	// TODO: Use proper regex

	return items
}
