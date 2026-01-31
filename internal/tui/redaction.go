// Package tui provides Bubble Tea models for svf.
package tui

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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

	// Edit mode fields
	editing      bool
	editInput    textinput.Model
	editIndex    int
	editOriginal string
	// Enhanced edit mode fields
	editField        editFieldType // Which field is being edited
	editPrompt       textinput.Model
	editReplacement  textinput.Model
	editQuickMode    bool   // Quick edit mode (single field)
	editPreview      string // Preview of replacement
	editAppliedStyle lipgloss.Style

	// styles
	normalStyle   lipgloss.Style
	selectedStyle lipgloss.Style
	redactedStyle lipgloss.Style
	warningStyle  lipgloss.Style
	headerStyle   lipgloss.Style
	labelStyle    lipgloss.Style
	infoStyle     lipgloss.Style
	successStyle  lipgloss.Style
	errorStyle    lipgloss.Style

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
	// RedactionStateEditing means user is editing a specific item.
	RedactionStateEditing
	// RedactionStateConfirming means user is confirming to send.
	RedactionStateConfirming
	// RedactionStateFinished means redaction is complete.
	RedactionStateFinished
)

// editFieldType represents which field is being edited in edit mode.
type editFieldType int

const (
	editFieldReplacement editFieldType = iota
	editFieldCustom
	editFieldPattern
)

// RedactedItem represents a detected sensitive item.
type RedactedItem struct {
	Original string
	Redacted string
	Type     SensitiveType
	StartPos int
	EndPos   int
}

// SensitiveType represents the type of sensitive data.
type SensitiveType string

const (
	TypeAPIKey     SensitiveType = "api-key"
	TypePassword   SensitiveType = "password"
	TypeToken      SensitiveType = "token"
	TypeBearer     SensitiveType = "bearer"
	TypeEmail      SensitiveType = "email"
	TypeSecret     SensitiveType = "secret"
	TypeCredential SensitiveType = "credential"
	TypePrivateKey SensitiveType = "private-key"
	TypeAuthHeader SensitiveType = "auth-header"
	TypeCookie     SensitiveType = "cookie"
	TypeSession    SensitiveType = "session"
	TypeUnknown    SensitiveType = "unknown"
)

// Pattern defines a regex pattern for detecting sensitive data.
type Pattern struct {
	Type    SensitiveType
	Regex   *regexp.Regexp
	Example string
}

// getDetectionPatterns returns all regex patterns for detecting sensitive data.
func getDetectionPatterns() []Pattern {
	return []Pattern{
		// API Keys - common patterns
		{
			Type:    TypeAPIKey,
			Regex:   regexp.MustCompile(`(?i)(api[_-]?key|apikey|key)[\"']?\s*[:=]\s*[\"']?([a-zA-Z0-9_\-]{20,})[\"']?`),
			Example: `"api_key": "sk-1234567890abcdef..."`,
		},
		{
			Type:    TypeAPIKey,
			Regex:   regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9]{20,}|pk-[a-zA-Z0-9]{20,})`),
			Example: `sk-1234567890abcdef...`,
		},
		{
			Type:    TypeAPIKey,
			Regex:   regexp.MustCompile(`(?i)(AKIA[0-9A-Z]{16})`),
			Example: `AKIAIOSFODNN7EXAMPLE`,
		},

		// Passwords
		{
			Type:    TypePassword,
			Regex:   regexp.MustCompile(`(?i)(password|passwd|pass)[\"']?\s*[:=]\s*[\"']?([^\s\"']+)[\"']?`),
			Example: `"password": "secret123"`,
		},
		{
			Type:    TypePassword,
			Regex:   regexp.MustCompile(`(?i)(--password|-p)\s+(\S+)`),
			Example: `mysql -u user -p secret123`,
		},

		// Bearer tokens and authorization headers
		{
			Type:    TypeBearer,
			Regex:   regexp.MustCompile(`(?i)(authorization|auth)[\"']?\s*:\s*[\"']?(bearer\s+)([a-zA-Z0-9_\-\.~=]+)`),
			Example: `"Authorization": "Bearer eyJhbGciOi..."`,
		},
		{
			Type:    TypeAuthHeader,
			Regex:   regexp.MustCompile(`(?i)(authorization|auth)[\"']?\s*:\s*[\"']?([a-zA-Z0-9_\-\.~=]+)`),
			Example: `"Authorization": "Basic dXNlcjpwYXNz"`,
		},

		// Tokens
		{
			Type:    TypeToken,
			Regex:   regexp.MustCompile(`(?i)(token|access[_-]?token|refresh[_-]?token)[\"']?\s*[:=]\s*[\"']?([a-zA-Z0-9_\-\.~=]{20,})[\"']?`),
			Example: `"access_token": "eyJhbGciOi..."`,
		},
		{
			Type:    TypeToken,
			Regex:   regexp.MustCompile(`(?i)(github[_-]?token|git[_-]?token|gh[_-]?token)[\"']?\s*[:=]\s*[\"']?(gh[pousr]_[a-zA-Z0-9]{36,})`),
			Example: `"github_token": "ghp_1234567890abcdef..."`,
		},

		// Secrets
		{
			Type:    TypeSecret,
			Regex:   regexp.MustCompile(`(?i)(secret|secret[_-]?key|secret[_-]?id)[\"']?\s*[:=]\s*[\"']?([a-zA-Z0-9_\-]{16,})[\"']?`),
			Example: `"secret_key": "abcd1234efgh5678"`,
		},

		// Private keys (PEM format detection)
		{
			Type:    TypePrivateKey,
			Regex:   regexp.MustCompile(`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`),
			Example: `-----BEGIN PRIVATE KEY-----`,
		},

		// Cookies and session IDs
		{
			Type:    TypeCookie,
			Regex:   regexp.MustCompile(`(?i)(cookie)[\"']?\s*:\s*[\"']?([a-zA-Z0-9_\-\.=]+)`),
			Example: `"Cookie": "sessionid=abc123..."`,
		},
		{
			Type:    TypeSession,
			Regex:   regexp.MustCompile(`(?i)(session[_-]?id|session[_-]?token|sid)[\"']?\s*[:=]\s*[\"']?([a-zA-Z0-9_\-]{20,})[\"']?`),
			Example: `"session_id": "abc123def456..."`,
		},

		// Email addresses
		{
			Type:    TypeEmail,
			Regex:   regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
			Example: `user@example.com`,
		},

		// Generic credential pattern
		{
			Type:    TypeCredential,
			Regex:   regexp.MustCompile(`(?i)(credential|creds|auth[_-]?token)[\"']?\s*[:=]\s*[\"']?([a-zA-Z0-9_\-]{16,})[\"']?`),
			Example: `"creds": "abcd1234efgh5678"`,
		},
	}
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
			startPos: item.StartPos,
			endPos:   item.EndPos,
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

	// Create edit input
	ei := textinput.New()
	ei.Placeholder = "Enter replacement value (or leave blank for <REDACTED>)"

	// Create enhanced edit inputs
	editPrompt := textinput.New()
	editPrompt.Placeholder = "Quick replacement"
	editReplacement := textinput.New()
	editReplacement.Placeholder = "Custom redaction text"

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
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Width(18)
	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))
	successStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("203")).
		Bold(true)
	editAppliedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("212")).
		Background(lipgloss.Color("236")).
		Padding(0, 1)

	return RedactionModel{
		Content:          content,
		RedactedContent:  content,
		State:            RedactionStateReviewing,
		List:             l,
		Textarea:         ta,
		ConfirmInput:     ti,
		editInput:        ei,
		editPrompt:       editPrompt,
		editReplacement:  editReplacement,
		editing:          false,
		editQuickMode:    true,
		editField:        editFieldReplacement,
		normalStyle:      normalStyle,
		selectedStyle:    selectedStyle,
		redactedStyle:    redactedStyle,
		warningStyle:     warningStyle,
		headerStyle:      headerStyle,
		labelStyle:       labelStyle,
		infoStyle:        infoStyle,
		editAppliedStyle: editAppliedStyle,
		successStyle:     successStyle,
		errorStyle:       errorStyle,
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
		if m.editing {
			return m.handleEditingKey(msg)
		}
		return m.handleNormalKey(msg)
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

// handleNormalKey handles key messages in normal (non-editing) mode.
func (m RedactionModel) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		if m.State == RedactionStateReviewing || m.State == RedactionStateRedacting {
			// Redact selected item
			if len(m.List.Items()) > 0 {
				item := m.List.SelectedItem().(redactionItem)
				item.redacted = "<REDACTED>"
				m.List.SetItem(m.List.Index(), item)
				m.updateRedactedContent()
				m.State = RedactionStateRedacting
			}
		}

	case "e":
		if m.State == RedactionStateReviewing || m.State == RedactionStateRedacting {
			// Edit selected item manually
			if len(m.List.Items()) > 0 {
				item := m.List.SelectedItem().(redactionItem)
				m.editing = true
				m.editIndex = m.List.Index()
				m.editOriginal = item.original
				m.editField = editFieldReplacement
				m.editQuickMode = true

				// Initialize quick edit input with current value if already redacted
				if item.redacted != item.original && item.redacted != "<REDACTED>" {
					m.editPrompt.Reset()
					m.editPrompt.SetValue(item.redacted)
				} else {
					m.editPrompt.Reset()
				}
				m.editPrompt.Placeholder = fmt.Sprintf("Replace '%s' with", truncateString(item.original, 30))
				m.editPrompt.Focus()

				// Initialize advanced edit input
				m.editReplacement.Reset()
				m.editReplacement.Blur()

				// Set initial preview
				m.editPreview = ""
				if item.redacted != item.original {
					m.editPreview = item.redacted
				}
			}
		}

	case "c":
		if m.State == RedactionStateReviewing || m.State == RedactionStateRedacting {
			// Move to confirmation state
			m.State = RedactionStateConfirming
			m.ConfirmInput.Reset()
			m.ConfirmInput.Focus()
		}

	case "esc":
		if m.State == RedactionStateRedacting {
			// Go back to reviewing
			m.State = RedactionStateReviewing
		} else if m.State == RedactionStateConfirming {
			// Go back to redacting
			m.State = RedactionStateRedacting
		}

	case "a":
		if m.State == RedactionStateReviewing {
			// Redact all detected items
			for i := 0; i < len(m.List.Items()); i++ {
				item := m.List.Items()[i].(redactionItem)
				item.redacted = "<REDACTED>"
				m.List.SetItem(i, item)
			}
			m.updateRedactedContent()
			m.State = RedactionStateRedacting
		}

	case "u":
		if m.State == RedactionStateRedacting {
			// Undo all redactions
			for i := 0; i < len(m.List.Items()); i++ {
				item := m.List.Items()[i].(redactionItem)
				item.redacted = item.original
				m.List.SetItem(i, item)
			}
			m.updateRedactedContent()
			m.State = RedactionStateReviewing
		}
	}

	return m, nil
}

// handleEditingKey handles key messages in edit mode.
func (m RedactionModel) handleEditingKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		// Cancel editing
		m.editing = false
		m.editInput.Reset()
		m.editPrompt.Reset()
		m.editReplacement.Reset()
		m.editQuickMode = true
		return m, nil

	case tea.KeyEnter:
		// Apply edit
		var replacement string
		if m.editQuickMode {
			replacement = m.editPrompt.Value()
		} else {
			replacement = m.editReplacement.Value()
		}

		if replacement == "" {
			replacement = "<REDACTED>"
		}

		item := m.List.Items()[m.editIndex].(redactionItem)
		item.redacted = replacement
		m.List.SetItem(m.editIndex, item)
		m.updateRedactedContent()

		m.editing = false
		m.editInput.Reset()
		m.editPrompt.Reset()
		m.editReplacement.Reset()
		m.editQuickMode = true
		m.State = RedactionStateRedacting
		return m, nil

	case tea.KeyTab:
		// Toggle between quick and advanced mode
		m.editQuickMode = !m.editQuickMode
		if m.editQuickMode {
			m.editPrompt.Focus()
			m.editReplacement.Blur()
		} else {
			m.editPrompt.Blur()
			m.editReplacement.Focus()
		}
		return m, nil

	case tea.KeyCtrlR:
		// Quick redaction with <REDACTED>
		item := m.List.Items()[m.editIndex].(redactionItem)
		item.redacted = "<REDACTED>"
		m.List.SetItem(m.editIndex, item)
		m.updateRedactedContent()
		m.editing = false
		m.State = RedactionStateRedacting
		return m, nil

	case tea.KeyCtrlS:
		// Quick redaction with [SAME TYPE]
		item := m.List.Items()[m.editIndex].(redactionItem)
		item.redacted = fmt.Sprintf("<%s>", strings.ToUpper(string(item.itemType)))
		m.List.SetItem(m.editIndex, item)
		m.updateRedactedContent()
		m.editing = false
		m.State = RedactionStateRedacting
		return m, nil

	case tea.KeyCtrlU:
		// Quick redaction with asterisks (same length)
		item := m.List.Items()[m.editIndex].(redactionItem)
		item.redacted = strings.Repeat("*", len(item.original))
		m.List.SetItem(m.editIndex, item)
		m.updateRedactedContent()
		m.editing = false
		m.State = RedactionStateRedacting
		return m, nil
	}

	// Update edit inputs
	var cmd tea.Cmd
	if m.editQuickMode {
		m.editPrompt, cmd = m.editPrompt.Update(msg)
		// Update preview
		m.editPreview = m.editPrompt.Value()
		if m.editPreview == "" {
			m.editPreview = "<REDACTED>"
		}
	} else {
		m.editReplacement, cmd = m.editReplacement.Update(msg)
		// Update preview
		m.editPreview = m.editReplacement.Value()
		if m.editPreview == "" {
			m.editPreview = "<REDACTED>"
		}
	}
	return m, cmd
}

// View implements tea.Model.
func (m RedactionModel) View() string {
	if m.State == RedactionStateFinished {
		return m.finishedView()
	}

	if m.editing {
		return m.editView()
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

	// Instructions with better organization
	if m.State == RedactionStateReviewing {
		b.WriteString(m.infoStyle.Render("Review detected sensitive data:"))
		b.WriteString("\n\n")

		// Action groups
		actionStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("251")).
			Width(25)

		redactActions := actionStyle.Render(
			" [a]: Redact all\n" +
				" [r]: Redact selected\n" +
				" [e]: Edit selected",
		)

		navigateActions := actionStyle.Render(
			" [↑/↓]: Navigate\n" +
				" [Enter]: View details\n" +
				" [c]: Confirm changes",
		)

		otherActions := actionStyle.Render(
			" [q]: Quit\n" +
				" [?]: Help",
		)

		actionsRow := lipgloss.JoinHorizontal(lipgloss.Top, redactActions, navigateActions, otherActions)
		b.WriteString(actionsRow)
		b.WriteString("\n\n")

		// Status summary
		redactedCount := m.countRedactedItems()
		totalCount := len(m.List.Items())
		if totalCount > 0 {
			statusText := fmt.Sprintf("Status: %d/%d items redacted", redactedCount, totalCount)
			if redactedCount > 0 {
				b.WriteString(m.successStyle.Render("  ✓ " + statusText))
			} else {
				b.WriteString(m.warningStyle.Render("  ⚠ " + statusText))
			}
		}
		b.WriteString("\n\n")
	} else {
		b.WriteString(m.infoStyle.Render("Redaction complete. Review changes:"))
		b.WriteString("\n\n")

		// Action groups for redacting state
		actionStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("251")).
			Width(25)

		modifyActions := actionStyle.Render(
			" [r]: Redact selected\n" +
				" [e]: Edit selected\n" +
				" [u]: Undo all",
		)

		navigateActions := actionStyle.Render(
			" [↑/↓]: Navigate\n" +
				" [Enter]: View details\n" +
				" [c]: Confirm to send",
		)

		otherActions := actionStyle.Render(
			" [esc]: Go back\n" +
				" [q]: Quit",
		)

		actionsRow := lipgloss.JoinHorizontal(lipgloss.Top, modifyActions, navigateActions, otherActions)
		b.WriteString(actionsRow)
		b.WriteString("\n\n")

		// Status summary
		redactedCount := m.countRedactedItems()
		totalCount := len(m.List.Items())
		if totalCount > 0 {
			statusText := fmt.Sprintf("Status: %d/%d items redacted", redactedCount, totalCount)
			if redactedCount > 0 {
				b.WriteString(m.successStyle.Render("  ✓ " + statusText))
			} else {
				b.WriteString(m.warningStyle.Render("  ⚠ " + statusText))
			}
		}
		b.WriteString("\n\n")
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

// editView shows the edit dialog for a single item.
func (m RedactionModel) editView() string {
	var b strings.Builder

	b.WriteString(m.headerStyle.Render("✏️  Edit Redaction"))
	b.WriteString("\n\n")

	// Show original value
	item := m.List.Items()[m.editIndex].(redactionItem)

	// Type badge with color coding based on type
	typeColor := m.getTypeColor(item.itemType)
	typeBadge := lipgloss.NewStyle().
		Foreground(typeColor).
		Background(lipgloss.Color("235")).
		Padding(0, 1).
		Render(string(item.itemType))

	b.WriteString(m.labelStyle.Render("Type:"))
	b.WriteString(" ")
	b.WriteString(typeBadge)
	b.WriteString("\n\n")

	// Original value with truncation and visual distinction
	b.WriteString(m.labelStyle.Render("Original:"))
	b.WriteString("\n")
	originalDisplay := truncateString(item.original, 70)
	if len(item.original) > 70 {
		originalDisplay += m.infoStyle.Render("...")
	}
	b.WriteString("  " + m.redactedStyle.Render(originalDisplay))
	b.WriteString("\n\n")

	// Current value (if already edited)
	if item.redacted != item.original {
		b.WriteString(m.labelStyle.Render("Current:"))
		b.WriteString("\n")
		currentDisplay := truncateString(item.redacted, 70)
		if len(item.redacted) > 70 {
			currentDisplay += m.infoStyle.Render("...")
		}
		b.WriteString("  " + m.successStyle.Render(currentDisplay))
		b.WriteString("\n\n")
	}

	// Mode indicator
	modeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)
	modeText := "Quick Mode"
	if !m.editQuickMode {
		modeText = "Advanced Mode"
	}
	b.WriteString(modeStyle.Render("Mode: " + modeText + " (Tab to toggle)"))
	b.WriteString("\n\n")

	if m.editQuickMode {
		// Quick edit mode - single input field
		b.WriteString(m.labelStyle.Render("Quick Replace:"))
		b.WriteString("\n")
		b.WriteString(m.editPrompt.View())
		b.WriteString("\n\n")

		// Live preview
		b.WriteString(m.labelStyle.Render("Preview:"))
		b.WriteString("\n")
		previewText := m.editPreview
		if previewText == "" {
			previewText = "<REDACTED>"
		}
		b.WriteString("  " + m.editAppliedStyle.Render(previewText))
		b.WriteString("\n\n")

		// Quick action hints
		quickHintStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			MarginBottom(1)
		b.WriteString(quickHintStyle.Render("Quick Actions:"))
		b.WriteString("\n")
		b.WriteString("  [Ctrl+R]: <REDACTED>  [Ctrl+S]: <TYPE>  [Ctrl+U]: ***masked***")
		b.WriteString("\n\n")
	} else {
		// Advanced mode - custom replacement
		b.WriteString(m.labelStyle.Render("Custom Text:"))
		b.WriteString("\n")
		b.WriteString(m.editReplacement.View())
		b.WriteString("\n\n")

		// Live preview
		b.WriteString(m.labelStyle.Render("Preview:"))
		b.WriteString("\n")
		previewText := m.editReplacement.Value()
		if previewText == "" {
			previewText = "<REDACTED>"
		}
		b.WriteString("  " + m.editAppliedStyle.Render(previewText))
		b.WriteString("\n\n")
	}

	// Additional info
	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Italic(true)
	b.WriteString(infoStyle.Render(fmt.Sprintf("Length: %d characters", len(item.original))))
	b.WriteString("\n\n")

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)

	footer := footerStyle.Render(
		" [Enter]: apply  [Tab]: toggle mode  [Esc]: cancel",
	)

	b.WriteString(footer)

	return b.String()
}

// getTypeColor returns a color for the given sensitive type.
func (m RedactionModel) getTypeColor(st SensitiveType) lipgloss.Color {
	switch st {
	case TypeAPIKey:
		return lipgloss.Color("203") // Red/pink
	case TypePassword, TypeSecret, TypePrivateKey:
		return lipgloss.Color("196") // Bright red
	case TypeToken, TypeBearer, TypeAuthHeader:
		return lipgloss.Color("226") // Yellow
	case TypeEmail:
		return lipgloss.Color("86") // Cyan
	case TypeCookie, TypeSession:
		return lipgloss.Color("212") // Pink/purple
	case TypeCredential:
		return lipgloss.Color("208") // Orange
	default:
		return lipgloss.Color("245") // Grey
	}
}

// confirmView shows the confirmation dialog.
func (m RedactionModel) confirmView() string {
	var b strings.Builder

	b.WriteString(m.headerStyle.Render("⚠️  Confirm Sending to AI"))
	b.WriteString("\n\n")

	b.WriteString(m.warningStyle.Render("WARNING: This data will be sent to an external AI provider."))
	b.WriteString("\n\n")

	// Summary section
	summaryStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true).
		MarginBottom(1)
	b.WriteString(summaryStyle.Render("Summary:"))
	b.WriteString("\n")

	// Stats grid
	statStyle := lipgloss.NewStyle().
		Width(30)

	stat1 := statStyle.Render(
		fmt.Sprintf("  Original length: %d chars", len(m.Content)),
	)
	stat2 := statStyle.Render(
		fmt.Sprintf("  Redacted length: %d chars", len(m.RedactedContent)),
	)

	redactedCount := m.countRedactedItems()
	totalCount := len(m.List.Items())

	stat3 := statStyle.Render(
		fmt.Sprintf("  Items redacted: %d/%d", redactedCount, totalCount),
	)

	// Status indicator
	statusStyle := lipgloss.NewStyle().
		Width(30)
	if redactedCount > 0 {
		stat4 := statusStyle.Render(
			m.successStyle.Render("  ✓ Data protected"),
		)
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, stat1, stat2))
		b.WriteString("\n")
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, stat3, stat4))
	} else {
		stat4 := statusStyle.Render(
			m.errorStyle.Render("  ⚠ No protection!"),
		)
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, stat1, stat2))
		b.WriteString("\n")
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, stat3, stat4))
	}
	b.WriteString("\n\n")

	// Detailed changes if any
	if redactedCount > 0 {
		detailStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true).
			MarginBottom(1)
		b.WriteString(detailStyle.Render("Redacted Items:"))
		b.WriteString("\n")

		// Show first few redacted items
		count := 0
		maxShow := 5
		for _, listItem := range m.List.Items() {
			if count >= maxShow {
				break
			}
			ri := listItem.(redactionItem)
			if ri.redacted != ri.original {
				count++
				typeColor := m.getTypeColor(ri.itemType)
				typeBadge := lipgloss.NewStyle().
					Foreground(typeColor).
					Render(fmt.Sprintf("[%s]", ri.itemType))
				b.WriteString(fmt.Sprintf("  %s %s → %s\n",
					typeBadge,
					truncateString(ri.original, 25),
					m.successStyle.Render(ri.redacted),
				))
			}
		}

		if redactedCount > maxShow {
			b.WriteString(fmt.Sprintf("  ... and %d more\n", redactedCount-maxShow))
		}
		b.WriteString("\n")
	}

	// Content preview
	previewStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true).
		MarginBottom(1)
	b.WriteString(previewStyle.Render("Redacted Content Preview:"))
	b.WriteString("\n")

	previewBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1)

	// Show preview
	preview := m.RedactedContent
	maxPreviewLen := 500
	if len(preview) > maxPreviewLen {
		preview = preview[:maxPreviewLen] + m.infoStyle.Render("...")
	}
	b.WriteString(previewBoxStyle.Render(preview))

	b.WriteString("\n\n")

	// Confirmation input
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

	// Header with count
	totalCount := len(m.List.Items())
	redactedCount := m.countRedactedItems()

	headerText := fmt.Sprintf(" Detected Items (%d/%d redacted)", redactedCount, totalCount)
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	b.WriteString(headerStyle.Render(headerText))
	b.WriteString("\n\n")

	if len(m.List.Items()) == 0 {
		noItemsStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Italic(true)
		b.WriteString(noItemsStyle.Render("No sensitive items detected."))
	} else {
		b.WriteString(m.List.View())
	}

	width := 55
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

	// Header with stats
	redactedCount := m.countRedactedItems()

	headerText := fmt.Sprintf(" Content Preview (%d changes)", redactedCount)
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	b.WriteString(headerStyle.Render(headerText))
	b.WriteString("\n\n")

	// Show redacted content with highlighting
	content := m.RedactedContent

	// Get selected item for highlighting
	var selectedItem *redactionItem
	if m.State == RedactionStateRedacting || m.State == RedactionStateReviewing {
		if len(m.List.Items()) > 0 && m.List.SelectedItem() != nil {
			item := m.List.SelectedItem().(redactionItem)
			selectedItem = &item
		}
	}

	// Highlight redacted items and selected item
	if m.State == RedactionStateRedacting {
		// Highlight all redacted items
		for _, listItem := range m.List.Items() {
			ri := listItem.(redactionItem)
			if ri.redacted != ri.original {
				// Use different color for selected vs non-selected
				if selectedItem != nil && selectedItem.index == ri.index {
					content = strings.ReplaceAll(content, ri.redacted,
						m.editAppliedStyle.Render(ri.redacted))
				} else {
					content = strings.ReplaceAll(content, ri.redacted,
						m.redactedStyle.Render(ri.redacted))
				}
			}
		}
	}

	// Highlight selected item in original content (if not redacted yet)
	if selectedItem != nil && selectedItem.redacted == selectedItem.original {
		content = strings.ReplaceAll(content, selectedItem.original,
			m.selectedStyle.Render(selectedItem.original))
	}

	// Truncate content if too long
	maxPreviewLen := 2000
	if len(content) > maxPreviewLen {
		content = content[:maxPreviewLen] + m.infoStyle.Render("\n\n... (content truncated)")
	}

	b.WriteString(content)

	width := m.width - 65
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

// countRedactedItems returns the number of items that have been redacted.
func (m RedactionModel) countRedactedItems() int {
	count := 0
	for _, item := range m.List.Items() {
		ri := item.(redactionItem)
		if ri.redacted != ri.original {
			count++
		}
	}
	return count
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

// truncateString truncates a string to a maximum length.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// redactionItem is a list item for a detected sensitive item.
type redactionItem struct {
	index    int
	original string
	redacted string
	itemType SensitiveType
	startPos int
	endPos   int
}

func (r redactionItem) FilterValue() string {
	return r.original
}

// Title implements list.Item.
func (r redactionItem) Title() string {
	return string(r.itemType)
}

// Description implements list.Item.
func (r redactionItem) Description() string {
	return ""
}

// redactionDelegate defines how items are rendered in the list.
type redactionDelegate struct{}

func (d redactionDelegate) Height() int  { return 3 }
func (d redactionDelegate) Spacing() int { return 0 }
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

	// Type color coding
	typeColor := d.getTypeColor(r.itemType)

	// First line: index indicator and type
	var prefix string
	if index == m.Index() {
		prefix = selectedStyle.Render("→")
	} else {
		prefix = normalStyle.Render(" ")
	}

	indexStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Width(3).
		Align(lipgloss.Right)

	typeBadge := lipgloss.NewStyle().
		Foreground(typeColor).
		Render(fmt.Sprintf("[%s]", r.itemType))

	_, _ = fmt.Fprintf(w, "%s %s %s\n", prefix, indexStyle.Render(fmt.Sprintf("%d.", index+1)), typeBadge)

	// Second line: original value (truncated)
	originalText := truncateString(r.original, 45)
	if index == m.Index() {
		originalText = selectedStyle.Render(originalText)
	} else {
		originalText = normalStyle.Render(originalText)
	}
	_, _ = fmt.Fprintf(w, "    %s\n", originalText)

	// Third line: status/replacement
	if r.redacted == "<REDACTED>" {
		statusText := lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Render("  [REDACTED]")
		_, _ = fmt.Fprintf(w, "%s\n", statusText)
	} else if r.redacted != r.original {
		statusText := lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(fmt.Sprintf("  → %s", truncateString(r.redacted, 40)))
		_, _ = fmt.Fprintf(w, "%s\n", statusText)
	} else {
		// Show "[unchanged]" in dim color
		statusText := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("  [unchanged]")
		_, _ = fmt.Fprintf(w, "%s\n", statusText)
	}
}

// getTypeColor returns a color for the given sensitive type.
func (d redactionDelegate) getTypeColor(st SensitiveType) lipgloss.Color {
	switch st {
	case TypeAPIKey:
		return lipgloss.Color("203") // Red/pink
	case TypePassword, TypeSecret, TypePrivateKey:
		return lipgloss.Color("196") // Bright red
	case TypeToken, TypeBearer, TypeAuthHeader:
		return lipgloss.Color("226") // Yellow
	case TypeEmail:
		return lipgloss.Color("86") // Cyan
	case TypeCookie, TypeSession:
		return lipgloss.Color("212") // Pink/purple
	case TypeCredential:
		return lipgloss.Color("208") // Orange
	default:
		return lipgloss.Color("245") // Grey
	}
}

// detectSensitiveItems detects potential sensitive items in content using regex patterns.
func detectSensitiveItems(content string) []RedactedItem {
	var items []RedactedItem
	seen := make(map[string]bool) // Track unique values to avoid duplicates

	patterns := getDetectionPatterns()

	for _, pattern := range patterns {
		matches := pattern.Regex.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			// Extract the sensitive value (usually the last capturing group or specific group)
			var sensitiveValue string
			if len(match) > 2 {
				// For patterns with capturing groups, use the last group
				sensitiveValue = match[len(match)-1]
			} else if len(match) > 1 {
				sensitiveValue = match[1]
			} else {
				sensitiveValue = match[0]
			}

			// Skip empty values or already seen values
			if sensitiveValue == "" || seen[sensitiveValue] {
				continue
			}

			// Find position in content
			startPos := strings.Index(content, match[0])
			if startPos == -1 {
				continue
			}

			seen[sensitiveValue] = true

			items = append(items, RedactedItem{
				Original: sensitiveValue,
				Redacted: sensitiveValue,
				Type:     pattern.Type,
				StartPos: startPos,
				EndPos:   startPos + len(sensitiveValue),
			})
		}
	}

	return items
}
