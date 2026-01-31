package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chazuruo/faire/internal/index"
)

// WorkflowSearchModel is the TUI model for workflow search.
type WorkflowSearchModel struct {
	index      *index.Index
	query      string
	filter     index.SearchOptions
	results    []index.Entry
	selected   int
	quit       bool
	confirmed  bool
	width      int
	height     int
	scrollOffset int
}

// NewWorkflowSearch creates a new workflow search TUI.
func NewWorkflowSearch(idx *index.Index, initialQuery string) WorkflowSearchModel {
	results := idx.Search(index.SearchOptions{Query: initialQuery, Limit: 50})

	return WorkflowSearchModel{
		index:        idx,
		query:        initialQuery,
		filter: index.SearchOptions{
			Query: initialQuery,
			Limit: 50,
		},
		results:      results,
		selected:     0,
		width:        80,
		height:       24,
		scrollOffset: 0,
		confirmed:    false,
		quit:         false,
	}
}

// Init initializes the workflow search model.
func (m WorkflowSearchModel) Init() tea.Cmd {
	return nil
}

// Update updates the workflow search model.
func (m WorkflowSearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			m.quit = true
			return m, tea.Quit

		case "enter", "ctrl+j":
			if len(m.results) > 0 && m.selected >= 0 && m.selected < len(m.results) {
				m.confirmed = true
			}
			return m, tea.Quit

		case "ctrl+c":
			m.quit = true
			return m, tea.Quit

		case "up", "k":
			if m.selected > 0 {
				m.selected--
				m.updateScrollOffset()
			}

		case "down", "j":
			if m.selected < len(m.results)-1 {
				m.selected++
				m.updateScrollOffset()
			}

		case "home", "g":
			m.selected = 0
			m.scrollOffset = 0

		case "end", "G":
			m.selected = len(m.results) - 1
			m.updateScrollOffset()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateScrollOffset()
	}

	return m, nil
}

// updateScrollOffset updates the scroll offset to keep the selected item visible.
func (m *WorkflowSearchModel) updateScrollOffset() {
	maxVisible := m.maxVisibleItems()
	if m.selected < m.scrollOffset {
		m.scrollOffset = m.selected
	} else if m.selected >= m.scrollOffset+maxVisible {
		m.scrollOffset = m.selected - maxVisible + 1
	}
}

// maxVisibleItems returns the maximum number of visible items.
func (m *WorkflowSearchModel) maxVisibleItems() int {
	// Reserve space for header (2), search bar (2), footer (2), padding (2)
	return m.height - 8
}

// View renders the workflow search model.
func (m WorkflowSearchModel) View() string {
	if m.width == 0 {
		m.width = 80
	}
	if m.height == 0 {
		m.height = 24
	}

	// Build the UI
	var b strings.Builder

	// Title
	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true).
		Render("Workflows")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Search bar
	searchBar := m.renderSearchBar()
	b.WriteString(searchBar)
	b.WriteString("\n\n")

	// Results
	b.WriteString(m.renderResults())
	b.WriteString("\n")

	// Footer
	b.WriteString(m.renderFooter())

	return b.String()
}

// renderSearchBar renders the search input bar.
func (m WorkflowSearchModel) renderSearchBar() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Background(lipgloss.Color("235")).
		Padding(0, 1)

	prompt := "/"
	if m.query != "" {
		prompt = "/" + m.query
	}

	searchBar := style.Render(prompt)

	// Add info on the right
	info := ""
	if len(m.results) > 0 {
		info = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render(fmt.Sprintf("%d result(s)", len(m.results)))
	}

	if info != "" {
		width := m.width - lipgloss.Width(searchBar) - lipgloss.Width(info) - 2
		if width < 0 {
			width = 0
		}
		middle := lipgloss.NewStyle().Width(width).Render(" ")
		searchBar = lipgloss.JoinHorizontal(lipgloss.Top, searchBar, middle, info)
	}

	return searchBar
}

// renderResults renders the results list.
func (m WorkflowSearchModel) renderResults() string {
	if len(m.results) == 0 {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		return style.Render("No results found.")
	}

	maxVisible := m.maxVisibleItems()
	start := m.scrollOffset
	end := start + maxVisible
	if end > len(m.results) {
		end = len(m.results)
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		entry := m.results[i]
		isSelected := i == m.selected

		item := m.renderItem(entry, isSelected)
		b.WriteString(item)
		b.WriteString("\n")
	}

	return b.String()
}

// renderItem renders a single result item.
func (m WorkflowSearchModel) renderItem(entry index.Entry, selected bool) string {
	// Base style
	style := lipgloss.NewStyle()
	if selected {
		style = style.
			Foreground(lipgloss.Color("252")).
			Background(lipgloss.Color("59")).
			Padding(0, 1)
	} else {
		style = style.
			Foreground(lipgloss.Color("242"))
	}

	// Title
	title := entry.Title
	if title == "" {
		title = entry.Slug
	}
	if lipgloss.Width(title) > m.width-10 {
		title = lipgloss.NewStyle().Width(m.width - 13).Render(title)
	}

	// Tags
	tags := ""
	if len(entry.Tags) > 0 {
		tags = strings.Join(entry.Tags, ", ")
	}

	// Format: "title  tags"
	itemStr := title
	if tags != "" {
		tagsStyle := lipgloss.NewStyle().Faint(true)
		itemStr += "  " + tagsStyle.Render(tags)
	}

	return style.Render(itemStr)
}

// renderFooter renders the help footer.
func (m WorkflowSearchModel) renderFooter() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	help := "↑/k: up • ↓/j: down • enter: select • q: quit"
	return style.Render(help)
}

// DidQuit returns true if the user quit without selecting.
func (m WorkflowSearchModel) DidQuit() bool {
	return m.quit
}

// DidConfirm returns true if the user confirmed a selection.
func (m WorkflowSearchModel) DidConfirm() bool {
	return m.confirmed
}

// GetSelected returns the selected workflow entry.
func (m WorkflowSearchModel) GetSelected() *index.Entry {
	if len(m.results) == 0 || m.selected < 0 || m.selected >= len(m.results) {
		return nil
	}
	return &m.results[m.selected]
}
