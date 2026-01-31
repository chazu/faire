// Package tui provides Bubble Tea models for terminal UI interactions.
package tui

import (
	"fmt"
	"io"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/chazuruo/faire/internal/workflows"
)

// PlaceholderEditorModel is the model for editing workflow placeholders.
type PlaceholderEditorModel struct {
	Placeholders           map[string]workflows.Placeholder
	DetectedPlaceholders   map[string][]string // Maps placeholder name to step indices
	list                   list.Model
	editing                bool
	currentPlaceholder     string
	promptInput            textinput.Model
	defaultInput           textinput.Model
	validateInput          textinput.Model
	secretToggle           bool
	Done                   bool
	Cancelled              bool
}

// placeholderItem represents a placeholder in the list.
type placeholderItem struct {
	name     string
	placeholder workflows.Placeholder
	usedIn   []string // Step indices where used
}

// NewPlaceholderEditor creates a new placeholder editor model.
func NewPlaceholderEditor(placeholders map[string]workflows.Placeholder, detected map[string][]string) *PlaceholderEditorModel {
	// Build list of placeholders
	items := make([]list.Item, 0, len(placeholders)+len(detected))

	// Add defined placeholders first
	for name, ph := range placeholders {
		usedIn := detected[name]
		items = append(items, placeholderItem{
			name:       name,
			placeholder: ph,
			usedIn:     usedIn,
		})
	}

	// Add detected but undefined placeholders
	for name, usedIn := range detected {
		if _, exists := placeholders[name]; !exists {
			items = append(items, placeholderItem{
				name:       name,
				placeholder: workflows.Placeholder{},
				usedIn:     usedIn,
			})
		}
	}

	// Sort items by name
	sort.Slice(items, func(i, j int) bool {
		return items[i].(placeholderItem).name < items[j].(placeholderItem).name
	})

	// Create list
	li := list.New(items, placeholderDelegate{}, 0, 0)
	li.SetShowStatusBar(false)
	li.SetFilteringEnabled(true)
	li.Title = "Placeholders"

	return &PlaceholderEditorModel{
		Placeholders:         placeholders,
		DetectedPlaceholders: detected,
		list:                 li,
		editing:              false,
		Done:                 false,
		Cancelled:            false,
	}
}

// Init initializes the placeholder editor.
func (m *PlaceholderEditorModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update updates the placeholder editor model.
func (m *PlaceholderEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.editing {
			return m.handleEditing(msg)
		}
		return m.handleNormalMode(msg)
	}

	// Update list when not editing
	if !m.editing {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleNormalMode handles key messages in normal mode.
func (m *PlaceholderEditorModel) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		m.Cancelled = true
		return m, tea.Quit

	case tea.KeyCtrlS:
		m.Done = true
		return m, tea.Quit

	case tea.KeyEnter:
		// Edit current placeholder
		if m.list.SelectedItem() != nil {
			item := m.list.SelectedItem().(placeholderItem)
			m.editing = true
			m.currentPlaceholder = item.name

			// Initialize inputs
			m.promptInput = textinput.New()
			m.promptInput.Placeholder = "Prompt text"
			m.promptInput.SetValue(item.placeholder.Prompt)
			m.promptInput.Focus()

			m.defaultInput = textinput.New()
			m.defaultInput.Placeholder = "Default value"
			m.defaultInput.SetValue(item.placeholder.Default)

			m.validateInput = textinput.New()
			m.validateInput.Placeholder = "Regex pattern"
			m.validateInput.SetValue(item.placeholder.Validate)

			m.secretToggle = item.placeholder.Secret
		}

	case tea.KeyCtrlN:
		// Add new placeholder
		newName := fmt.Sprintf("placeholder_%d", len(m.Placeholders)+1)
		m.Placeholders[newName] = workflows.Placeholder{}
		m.updateListItems()
		return m, nil

	case tea.KeyDelete, tea.KeyBackspace:
		// Delete current placeholder
		if m.list.SelectedItem() != nil {
			item := m.list.SelectedItem().(placeholderItem)
			delete(m.Placeholders, item.name)
			m.updateListItems()
		}
	}

	return m, nil
}

// handleEditing handles key messages when editing a placeholder.
func (m *PlaceholderEditorModel) handleEditing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		m.editing = false
		return m, nil

	case tea.KeyCtrlS:
		// Save placeholder
		ph := workflows.Placeholder{
			Prompt:   m.promptInput.Value(),
			Default:  m.defaultInput.Value(),
			Validate: m.validateInput.Value(),
			Secret:   m.secretToggle,
		}
		m.Placeholders[m.currentPlaceholder] = ph
		m.editing = false
		m.updateListItems()
		return m, nil

	case tea.KeyCtrlT:
		// Toggle secret
		m.secretToggle = !m.secretToggle
		return m, nil

	case tea.KeyTab:
		// Cycle through fields
		if m.promptInput.Focused() {
			m.promptInput.Blur()
			m.defaultInput.Focus()
		} else if m.defaultInput.Focused() {
			m.defaultInput.Blur()
			m.validateInput.Focus()
		} else if m.validateInput.Focused() {
			m.validateInput.Blur()
			m.promptInput.Focus()
		}
		return m, nil
	}

	// Update inputs
	var cmd tea.Cmd
	if m.promptInput.Focused() {
		m.promptInput, cmd = m.promptInput.Update(msg)
		return m, cmd
	}
	if m.defaultInput.Focused() {
		m.defaultInput, cmd = m.defaultInput.Update(msg)
		return m, cmd
	}
	if m.validateInput.Focused() {
		m.validateInput, cmd = m.validateInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

// updateListItems updates the list items from current placeholders.
func (m *PlaceholderEditorModel) updateListItems() {
	items := make([]list.Item, 0, len(m.Placeholders)+len(m.DetectedPlaceholders))

	// Add defined placeholders first
	for name, ph := range m.Placeholders {
		usedIn := m.DetectedPlaceholders[name]
		items = append(items, placeholderItem{
			name:        name,
			placeholder: ph,
			usedIn:      usedIn,
		})
	}

	// Add detected but undefined placeholders
	for name, usedIn := range m.DetectedPlaceholders {
		if _, exists := m.Placeholders[name]; !exists {
			items = append(items, placeholderItem{
				name:        name,
				placeholder: workflows.Placeholder{},
				usedIn:      usedIn,
			})
		}
	}

	// Sort items by name
	sort.Slice(items, func(i, j int) bool {
		return items[i].(placeholderItem).name < items[j].(placeholderItem).name
	})

	m.list.SetItems(items)
}

// View renders the placeholder editor.
func (m *PlaceholderEditorModel) View() string {
	if m.editing {
		return m.renderEditView()
	}
	return m.renderListView()
}

// renderListView renders the placeholder list view.
func (m *PlaceholderEditorModel) renderListView() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")). // Pink
		Bold(true).
		MarginBottom(1)

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")). // Grey
		MarginTop(1)

	title := titleStyle.Render("Placeholder Editor")

	if m.list.Height() == 0 {
		m.list.SetHeight(10)
	}

	footer := footerStyle.Render(
		" [Ctrl+S]: save and close [Esc]: cancel [Enter]: edit [Ctrl+N]: new [Del]: delete",
	)

	return title + "\n" + m.list.View() + "\n" + footer
}

// renderEditView renders the edit view for a single placeholder.
func (m *PlaceholderEditorModel) renderEditView() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")). // Pink
		Bold(true).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")). // Grey
		Width(12)

	highlightStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")). // Yellow
		Bold(true)

	title := titleStyle.Render("Edit Placeholder: " + m.currentPlaceholder)

	// Used in steps
	usedInText := ""
	if len(m.DetectedPlaceholders[m.currentPlaceholder]) > 0 {
		usedInText = "Used in steps: " + strings.Join(m.DetectedPlaceholders[m.currentPlaceholder], ", ")
	}

	// Secret status
	secretStatus := "No"
	if m.secretToggle {
		secretStatus = highlightStyle.Render("Yes")
	}

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")). // Grey
		MarginTop(1)

	footer := footerStyle.Render(
		" [Ctrl+S]: save [Esc]: cancel [Tab]: next field [Ctrl+T]: toggle secret",
	)

	return title + "\n\n" +
		labelStyle.Render("Prompt:") + " " + m.promptInput.View() + "\n\n" +
		labelStyle.Render("Default:") + " " + m.defaultInput.View() + "\n\n" +
		labelStyle.Render("Validate:") + " " + m.validateInput.View() + "\n\n" +
		labelStyle.Render("Secret:") + " " + secretStatus + "\n\n" +
		usedInText + "\n\n" +
		footer
}

// placeholderDelegate defines how placeholders are rendered in the list.
type placeholderDelegate struct{}

func (d placeholderDelegate) Height() int                               { return 2 }
func (d placeholderDelegate) Spacing() int                              { return 0 }
func (d placeholderDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d placeholderDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	p, ok := listItem.(placeholderItem)
	if !ok {
		return
	}

	// Styles
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("251"))

	undefStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("203"))

	// Choose style based on selection and definition
	var nameStyle lipgloss.Style
	if index == m.Index() {
		nameStyle = selectedStyle
	} else if p.placeholder.Prompt == "" && p.placeholder.Default == "" {
		nameStyle = undefStyle
	} else {
		nameStyle = normalStyle
	}

	// Format: name - prompt
	name := nameStyle.Render(p.name)
	promptText := p.placeholder.Prompt
	if promptText == "" {
		promptText = "(undefined)"
	}

	// Used in text
	usedIn := ""
	if len(p.usedIn) > 0 {
		usedIn = fmt.Sprintf(" [steps: %s]", strings.Join(p.usedIn, ","))
	}

	fmt.Fprintf(w, "%s - %s%s\n", name, promptText, usedIn)

	// Second line: default value
	if p.placeholder.Default != "" {
		defaultText := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("  Default: " + p.placeholder.Default)
		fmt.Fprintf(w, "%s\n", defaultText)
	} else if p.placeholder.Secret {
		secretText := lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render("  [secret]")
		fmt.Fprintf(w, "%s\n", secretText)
	}
}

func (p placeholderItem) Title() string {
	return p.name
}

func (p placeholderItem) Description() string {
	return p.placeholder.Prompt
}

func (p placeholderItem) FilterValue() string {
	return p.name + " " + p.placeholder.Prompt + " " + p.placeholder.Default
}
