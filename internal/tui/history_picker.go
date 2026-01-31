// Package tui provides Bubble Tea models for svf.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/chazuruo/svf/internal/history"
)

// HistoryPickerModel is a Bubble Tea model for selecting commands from shell history.
type HistoryPickerModel struct {
	// Commands is the full list of commands from history.
	Commands []history.Command

	// Selected is the set of selected command indices.
	Selected map[int]bool

	// Filtered is the list of commands after filtering.
	Filtered []int

	// cursor is the current cursor position in the filtered list.
	cursor int

	// FilterInput is the text input for filtering.
	FilterInput textinput.Model

	// Viewport shows the preview of the selected command.
	Viewport viewport.Model

	// ShowPreview controls whether to show the preview pane.
	ShowPreview bool

	// Focused indicates which component is focused ("filter" or "list").
	Focused string

	// Quit indicates whether the user quit without selecting.
	Quit bool

	// Confirmed indicates whether the user confirmed selection.
	Confirmed bool

	// styles
	normalStyle   lipgloss.Style
	selectedStyle lipgloss.Style
	previewStyle  lipgloss.Style
}

// NewHistoryPickerModel creates a new history picker model.
func NewHistoryPickerModel(commands []history.Command) HistoryPickerModel {
	ti := textinput.New()
	ti.Placeholder = "Filter commands..."
	ti.Focus()

	vp := viewport.New(60, 20)

	// Build initial filtered list (all indices)
	filtered := make([]int, len(commands))
	for i := range commands {
		filtered[i] = i
	}

	// Styles
	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Bold(true)
	previewStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242"))

	return HistoryPickerModel{
		Commands:     commands,
		Selected:     make(map[int]bool),
		Filtered:     filtered,
		cursor:       0,
		FilterInput:  ti,
		Viewport:     vp,
		ShowPreview:  true,
		Focused:      "filter",
		Quit:         false,
		Confirmed:    false,
		normalStyle:  normalStyle,
		selectedStyle: selectedStyle,
		previewStyle:  previewStyle,
	}
}

// Init implements tea.Model.
func (m HistoryPickerModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m HistoryPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.Quit = true
			return m, tea.Quit

		case "enter":
			if m.Focused == "filter" {
				m.Focused = "list"
				m.FilterInput.Blur()
			} else {
				m.Confirmed = true
				return m, tea.Quit
			}

		case "/":
			m.Focused = "filter"
			m.FilterInput.Focus()
			return m, nil

		case " ":
			// Toggle selection
			if len(m.Filtered) > 0 {
				idx := m.Filtered[m.cursor]
				m.Selected[idx] = !m.Selected[idx]
			}
			return m, nil

		case "a":
			// Select all filtered
			for _, idx := range m.Filtered {
				m.Selected[idx] = true
			}
			return m, nil

		case "n":
			// Select none
			m.Selected = make(map[int]bool)
			return m, nil

		case "up", "k":
			if m.Focused == "list" && m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.Focused == "list" && m.cursor < len(m.Filtered)-1 {
				m.cursor++
			}

		case "home", "g":
			if m.Focused == "list" {
				m.cursor = 0
			}

		case "end", "G":
			if m.Focused == "list" {
				m.cursor = len(m.Filtered) - 1
			}
		}
	}

	// Update filter input
	if m.Focused == "filter" {
		oldFilter := m.FilterInput.Value()
		m.FilterInput, cmd = m.FilterInput.Update(msg)
		newFilter := m.FilterInput.Value()
		if newFilter != oldFilter {
			m.applyFilter(newFilter)
		}
	}

	// Update viewport if showing preview
	if m.ShowPreview && len(m.Filtered) > 0 {
		m.Viewport, cmd = m.Viewport.Update(msg)
	}

	return m, cmd
}

// View implements tea.Model.
func (m HistoryPickerModel) View() string {
	if len(m.Commands) == 0 {
		return "\n  No commands found in shell history.\n"
	}

	var b strings.Builder

	// Header
	b.WriteString("\n  ")
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Render("Shell History Picker"))
	b.WriteString("\n\n")

	// Help text
	b.WriteString("  ")
	b.WriteString(m.helpText())
	b.WriteString("\n\n")

	// Two column layout
	leftWidth := 50
	rightWidth := 60

	// Left column: filter + list
	leftCol := m.renderListColumn(leftWidth)

	// Right column: preview
	rightCol := m.renderPreviewColumn(rightWidth)

	// Combine columns
	combined := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, rightCol)

	b.WriteString(combined)
	b.WriteString("\n")

	return b.String()
}

// renderListColumn renders the filter and list column.
func (m HistoryPickerModel) renderListColumn(width int) string {
	var b strings.Builder

	// Filter input
	b.WriteString("  Filter: ")
	b.WriteString(m.FilterInput.View())
	b.WriteString("\n\n")

	// List header
	b.WriteString("  ")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		fmt.Sprintf("%d commands, %d selected", len(m.Filtered), len(m.Selected)),
	))
	b.WriteString("\n\n")

	// Command list
	if len(m.Filtered) == 0 {
		b.WriteString("  (no matches)")
	} else {
		// Show visible window around cursor
		start := max(0, m.cursor-10)
		end := min(len(m.Filtered), m.cursor+11)

		for i := start; i < end; i++ {
			cmdIdx := m.Filtered[i]
			cmd := m.Commands[cmdIdx]
			isSelected := m.Selected[cmdIdx]
			isCursor := i == m.cursor

			line := "  "

			// Selection indicator
			if isSelected {
				line += "[x] "
			} else {
				line += "[ ] "
			}

			// Command (truncated)
			cmdText := cmd.Command
			if len(cmdText) > 40 {
				cmdText = cmdText[:37] + "..."
			}

			// Style
			style := m.normalStyle
			if isCursor {
				style = m.selectedStyle
			}
			if isSelected {
				style = style.Foreground(lipgloss.Color("green"))
			}

			line += style.Render(cmdText)
			b.WriteString(line + "\n")
		}
	}

	// Wrap in border
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Render(b.String())
}

// renderPreviewColumn renders the preview column.
func (m HistoryPickerModel) renderPreviewColumn(width int) string {
	if !m.ShowPreview || len(m.Filtered) == 0 {
		return ""
	}

	cmdIdx := m.Filtered[m.cursor]
	cmd := m.Commands[cmdIdx]

	var b strings.Builder

	b.WriteString("  Preview\n\n")

	// Command
	b.WriteString("  Command:\n")
	b.WriteString("    " + m.previewStyle.Render(cmd.Command) + "\n\n")

	// Metadata
	b.WriteString("  Shell: " + cmd.Shell + "\n")
	if cmd.CWD != "" {
		b.WriteString("  CWD:   " + cmd.CWD + "\n")
	}

	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Render(b.String())
}

// helpText returns the help text.
func (m HistoryPickerModel) helpText() string {
	var parts []string

	if m.Focused == "filter" {
		parts = append(parts, "[Enter] Focus list", "[Esc] Quit")
	} else {
		parts = append(parts,
			"[Enter] Confirm",
			"[Space] Toggle",
			"[a/n] Select all/none",
			"[/] Focus filter",
			"[q] Quit",
		)
	}

	return lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		strings.Join(parts, " • "),
	)
}

// applyFilter filters the command list based on the query.
func (m *HistoryPickerModel) applyFilter(query string) {
	query = strings.ToLower(query)

	m.Filtered = nil
	for i, cmd := range m.Commands {
		if strings.Contains(strings.ToLower(cmd.Command), query) {
			m.Filtered = append(m.Filtered, i)
		}
	}

	// Reset cursor if out of bounds
	if m.cursor >= len(m.Filtered) {
		m.cursor = max(0, len(m.Filtered)-1)
	}
}

// GetSelectedCommands returns the selected commands.
func (m HistoryPickerModel) GetSelectedCommands() []history.Command {
	var result []history.Command
	for idx := range m.Selected {
		result = append(result, m.Commands[idx])
	}
	return result
}

// DidQuit returns true if the user quit without selecting.
func (m HistoryPickerModel) DidQuit() bool {
	return m.Quit
}

// DidConfirm returns true if the user confirmed selection.
func (m HistoryPickerModel) DidConfirm() bool {
	return m.Confirmed
}
