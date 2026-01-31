// Package tui provides Bubble Tea models for terminal UI interactions.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/chazuruo/svf/internal/index"
)

// SearchModel is a Bubble Tea model for fuzzy search workflows.
type SearchModel struct {
	// Index is the search index.
	Index *index.Index

	// Results is the current search results.
	Results []index.SearchResult

	// cursor is the current cursor position in the results list.
	cursor int

	// SearchInput is the text input for search query.
	SearchInput textinput.Model

	// Viewport shows the preview of the selected workflow.
	Viewport viewport.Model

	// ShowPreview controls whether to show the preview pane.
	ShowPreview bool

	// Quit indicates whether the user quit without selecting.
	Quit bool

	// Confirmed indicates whether the user confirmed selection.
	Confirmed bool

	// SelectedEntry is the selected workflow entry.
	SelectedEntry *index.WorkflowEntry

	// Filter options
	Tags   []string
	Mine   bool
	Shared bool

	// styles
	normalStyle   lipgloss.Style
	selectedStyle lipgloss.Style
	previewStyle  lipgloss.Style
	headerStyle   lipgloss.Style
	metadataStyle lipgloss.Style
}

// NewSearchModel creates a new search model.
func NewSearchModel(idx *index.Index) SearchModel {
	ti := textinput.New()
	ti.Placeholder = "Search workflows..."
	ti.Focus()

	vp := viewport.New(60, 20)

	// Initial search (empty query returns all)
	results := idx.FuzzySearch(index.SearchOptions{})

	// Styles
	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Bold(true)
	previewStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242"))
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)
	metadataStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	return SearchModel{
		Index:        idx,
		Results:      results,
		cursor:       0,
		SearchInput:  ti,
		Viewport:     vp,
		ShowPreview:  true,
		Quit:         false,
		Confirmed:    false,
		normalStyle:  normalStyle,
		selectedStyle: selectedStyle,
		previewStyle: previewStyle,
		headerStyle:  headerStyle,
		metadataStyle: metadataStyle,
	}
}

// Init implements tea.Model.
func (m SearchModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.Quit = true
			return m, tea.Quit

		case "enter":
			if len(m.Results) > 0 {
				m.Confirmed = true
				m.SelectedEntry = &m.Results[m.cursor].Entry
			}
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.Results)-1 {
				m.cursor++
			}

		case "home", "g":
			m.cursor = 0

		case "end", "G":
			m.cursor = len(m.Results) - 1

		case "ctrl+n":
			// Toggle --mine filter
			m.Mine = !m.Mine
			if m.Mine {
				m.Shared = false
			}
			m.PerformSearch()

		case "ctrl+s":
			// Toggle --shared filter
			m.Shared = !m.Shared
			if m.Shared {
				m.Mine = false
			}
			m.PerformSearch()
		}
	}

	// Update search input
	oldQuery := m.SearchInput.Value()
	m.SearchInput, cmd = m.SearchInput.Update(msg)
	newQuery := m.SearchInput.Value()
	if newQuery != oldQuery {
		m.PerformSearch()
	}

	// Update viewport if showing preview
	if m.ShowPreview && len(m.Results) > 0 {
		m.Viewport, cmd = m.Viewport.Update(msg)
	}

	return m, cmd
}

// View implements tea.Model.
func (m SearchModel) View() string {
	var b strings.Builder

	// Header
	b.WriteString("\n  ")
	b.WriteString(m.headerStyle.Render("Workflow Search"))
	b.WriteString("\n\n")

	// Help text
	b.WriteString("  ")
	b.WriteString(m.helpText())
	b.WriteString("\n\n")

	// Two column layout
	leftWidth := 50
	rightWidth := 60

	// Left column: search + results
	leftCol := m.renderResultsColumn(leftWidth)

	// Right column: preview
	rightCol := m.renderPreviewColumn(rightWidth)

	// Combine columns
	combined := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, rightCol)

	b.WriteString(combined)
	b.WriteString("\n")

	return b.String()
}

// renderResultsColumn renders the search input and results column.
func (m SearchModel) renderResultsColumn(width int) string {
	var b strings.Builder

	// Search input
	b.WriteString("  Search: ")
	b.WriteString(m.SearchInput.View())
	b.WriteString("\n\n")

	// Filter indicators
	filters := m.getFilterIndicators()
	if filters != "" {
		b.WriteString("  ")
		b.WriteString(m.metadataStyle.Render("Filters: "+filters))
		b.WriteString("\n\n")
	}

	// Results header
	b.WriteString("  ")
	b.WriteString(m.metadataStyle.Render(
		fmt.Sprintf("%d result(s)", len(m.Results)),
	))
	b.WriteString("\n\n")

	// Results list
	if len(m.Results) == 0 {
		b.WriteString("  (no matches)")
	} else {
		// Show visible window around cursor
		start := max(0, m.cursor-10)
		end := min(len(m.Results), m.cursor+11)

		for i := start; i < end; i++ {
			result := m.Results[i]
			isCursor := i == m.cursor

			line := "  "

			// Title
			line += result.Entry.Title

			// Style
			style := m.normalStyle
			if isCursor {
				style = m.selectedStyle
			}

			// Add score indicator
			score := ""
			if result.Score > 1 {
				score = fmt.Sprintf(" (%.0f)", result.Score)
			}

			// Truncate if too long
			maxLen := 40
			if len(line)+len(score) > maxLen {
				line = line[:maxLen-len(score)-3] + "..."
			}

			line += score
			b.WriteString(style.Render(line) + "\n")

			// Show tags if cursor
			if isCursor && len(result.Entry.Tags) > 0 {
				b.WriteString("    ")
				b.WriteString(m.metadataStyle.Render(
					fmt.Sprintf("[%s]", strings.Join(result.Entry.Tags, ", ")),
				))
				b.WriteString("\n")
			}
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
func (m SearchModel) renderPreviewColumn(width int) string {
	if !m.ShowPreview || len(m.Results) == 0 {
		return ""
	}

	entry := m.Results[m.cursor].Entry

	var b strings.Builder

	b.WriteString("  Preview\n\n")

	// Title
	b.WriteString("  Title:\n")
	b.WriteString("    " + m.previewStyle.Render(entry.Title) + "\n\n")

	// ID
	b.WriteString("  ID:\n")
	b.WriteString("    " + m.metadataStyle.Render(entry.ID) + "\n\n")

	// Path
	b.WriteString("  Path:\n")
	b.WriteString("    " + m.metadataStyle.Render(entry.Path) + "\n\n")

	// Tags
	if len(entry.Tags) > 0 {
		b.WriteString("  Tags:\n")
		for _, tag := range entry.Tags {
			b.WriteString("    • " + tag + "\n")
		}
		b.WriteString("\n")
	}

	// Updated
	b.WriteString("  Updated:\n")
	b.WriteString("    " + m.metadataStyle.Render(entry.UpdatedAt) + "\n")

	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Render(b.String())
}

// helpText returns the help text.
func (m SearchModel) helpText() string {
	var parts []string

	parts = append(parts,
		"[Enter] Select",
		"[Ctrl+N] Mine only",
		"[Ctrl+S] Shared only",
		"[q] Quit",
	)

	return m.metadataStyle.Render(strings.Join(parts, " • "))
}

// getFilterIndicators returns the current filter indicators.
func (m SearchModel) getFilterIndicators() string {
	var filters []string

	if m.Mine {
		filters = append(filters, "mine")
	}
	if m.Shared {
		filters = append(filters, "shared")
	}
	if len(m.Tags) > 0 {
		filters = append(filters, fmt.Sprintf("tags:%s", strings.Join(m.Tags, ",")))
	}

	return strings.Join(filters, ", ")
}

// PerformSearch performs a search with the current query and filters.
func (m *SearchModel) PerformSearch() {
	query := m.SearchInput.Value()

	opts := index.SearchOptions{
		Query:      query,
		Tags:       m.Tags,
		Mine:       m.Mine,
		Shared:     m.Shared,
		MaxResults: 0,
	}

	m.Results = m.Index.FuzzySearch(opts)

	// Reset cursor if out of bounds
	if m.cursor >= len(m.Results) {
		m.cursor = max(0, len(m.Results)-1)
	}
}

// GetSelectedEntry returns the selected workflow entry.
func (m SearchModel) GetSelectedEntry() *index.WorkflowEntry {
	return m.SelectedEntry
}

// DidQuit returns true if the user quit without selecting.
func (m SearchModel) DidQuit() bool {
	return m.Quit
}

// DidConfirm returns true if the user confirmed selection.
func (m SearchModel) DidConfirm() bool {
	return m.Confirmed
}
