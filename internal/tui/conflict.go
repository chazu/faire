// Package tui provides Bubble Tea models for svf.
package tui

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DiffViewMode controls how diffs are displayed.
type DiffViewMode int

const (
	// DiffViewUnified shows a unified diff with conflict markers.
	DiffViewUnified DiffViewMode = iota
	// DiffViewSideBySide shows ours and theirs side by side.
	DiffViewSideBySide
)

// ConflictResolverModel is a Bubble Tea model for resolving merge conflicts.
type ConflictResolverModel struct {
	// ConflictedFiles is the list of files with conflicts.
	ConflictedFiles []string

	// CurrentFile is the index of the currently selected file.
	CurrentFile int

	// DiffContent is the diff content for the current file.
	DiffContent string

	// OursContent is the "ours" version of the file.
	OursContent string

	// TheirsContent is the "theirs" version of the file.
	TheirsContent string

	// BaseContent is the common ancestor version (optional).
	BaseContent string

	// DiffMode controls how diffs are displayed.
	DiffMode DiffViewMode

	// State is the current resolution state.
	State ResolutionState

	// List is the file list component.
	List list.Model

	// Viewport is the diff viewer (for unified view).
	Viewport viewport.Model

	// OursViewport is the "ours" diff viewer (for side-by-side).
	OursViewport viewport.Model

	// TheirsViewport is the "theirs" diff viewer (for side-by-side).
	TheirsViewport viewport.Model

	// Resolved indicates which files have been resolved.
	Resolved map[string]bool

	// Finished indicates if resolution is complete.
	Finished bool

	// Aborted indicates if user aborted.
	Aborted bool

	// styles
	normalStyle   lipgloss.Style
	selectedStyle lipgloss.Style
	oursStyle     lipgloss.Style
	theirsStyle   lipgloss.Style
	headerStyle   lipgloss.Style
	markerStyle   lipgloss.Style

	// width and height
	width  int
	height int
}

// ResolutionState represents the current resolution state.
type ResolutionState int

const (
	// ConflictStateSelecting means user is selecting a file to resolve.
	ConflictStateSelecting ResolutionState = iota
	// ConflictStateResolving means user is viewing and resolving a conflict.
	ConflictStateResolving
	// ConflictStateFinished means resolution is complete.
	ConflictStateFinished
)

// NewConflictResolverModel creates a new conflict resolver model.
func NewConflictResolverModel(conflictedFiles []string) ConflictResolverModel {
	// Create file list
	items := make([]list.Item, len(conflictedFiles))
	for i, file := range conflictedFiles {
		items[i] = conflictFileItem{
			index: i,
			path:  file,
		}
	}

	l := list.New(items, conflictFileDelegate{}, 0, 0)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.Title = "Conflicted Files"

	// Create viewports
	vp := viewport.New(80, 20)
	oursVp := viewport.New(40, 20)
	theirsVp := viewport.New(40, 20)

	// Styles
	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("251"))
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Bold(true)
	oursStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("green")).
		Background(lipgloss.Color("235"))
	theirsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("blue")).
		Background(lipgloss.Color("235"))
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)
	markerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)

	return ConflictResolverModel{
		ConflictedFiles: conflictedFiles,
		CurrentFile:     0,
		State:           ConflictStateSelecting,
		List:            l,
		Viewport:        vp,
		OursViewport:    oursVp,
		TheirsViewport:  theirsVp,
		Resolved:        make(map[string]bool),
		DiffMode:        DiffViewUnified,
		normalStyle:     normalStyle,
		selectedStyle:   selectedStyle,
		oursStyle:       oursStyle,
		theirsStyle:     theirsStyle,
		headerStyle:     headerStyle,
		markerStyle:     markerStyle,
	}
}

// Init implements tea.Model.
func (m ConflictResolverModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m ConflictResolverModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.State == ConflictStateSelecting {
				m.Aborted = true
				m.Finished = true
				m.State = ConflictStateFinished
				return m, tea.Quit
			}

		case "enter":
			if m.State == ConflictStateSelecting && len(m.ConflictedFiles) > 0 {
				// Open selected file for resolution
				m.State = ConflictStateResolving
				m.CurrentFile = m.List.Index()
				m.loadDiffForFile(m.ConflictedFiles[m.CurrentFile])
			}

		case "o":
			if m.State == ConflictStateResolving {
				// Accept "ours" version
				m.resolveConflict(m.ConflictedFiles[m.CurrentFile], "ours")
			}

		case "t":
			if m.State == ConflictStateResolving {
				// Accept "theirs" version
				m.resolveConflict(m.ConflictedFiles[m.CurrentFile], "theirs")
			}

		case "m":
			if m.State == ConflictStateResolving {
				// Open in external editor (mg)
				m.openInEditor(m.ConflictedFiles[m.CurrentFile])
				m.resolveConflict(m.ConflictedFiles[m.CurrentFile], "manual")
			}

		case "a":
			if m.State == ConflictStateResolving {
				// Abort resolution
				m.Aborted = true
				m.Finished = true
				m.State = ConflictStateFinished
				return m, tea.Quit
			}

		case "esc":
			if m.State == ConflictStateResolving {
				// Go back to file list
				m.State = ConflictStateSelecting
			}

		case "v":
			if m.State == ConflictStateResolving {
				// Toggle diff view mode
				if m.DiffMode == DiffViewUnified {
					m.DiffMode = DiffViewSideBySide
				} else {
					m.DiffMode = DiffViewUnified
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.Viewport.Width = msg.Width - 40
		m.Viewport.Height = msg.Height - 10
		m.OursViewport.Width = (msg.Width - 50) / 2
		m.OursViewport.Height = msg.Height - 10
		m.TheirsViewport.Width = (msg.Width - 50) / 2
		m.TheirsViewport.Height = msg.Height - 10
	}

	// Update child components
	if m.State == ConflictStateSelecting {
		var listCmd tea.Cmd
		m.List, listCmd = m.List.Update(msg)
		if listCmd != nil {
			cmds = append(cmds, listCmd)
		}
	}

	var vpCmd tea.Cmd
	m.Viewport, vpCmd = m.Viewport.Update(msg)
	if vpCmd != nil {
		cmds = append(cmds, vpCmd)
	}

	var oursVpCmd tea.Cmd
	m.OursViewport, oursVpCmd = m.OursViewport.Update(msg)
	if oursVpCmd != nil {
		cmds = append(cmds, oursVpCmd)
	}

	var theirsVpCmd tea.Cmd
	m.TheirsViewport, theirsVpCmd = m.TheirsViewport.Update(msg)
	if theirsVpCmd != nil {
		cmds = append(cmds, theirsVpCmd)
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m ConflictResolverModel) View() string {
	if m.Finished {
		return m.finishedView()
	}

	// Layout based on state
	switch m.State {
	case ConflictStateSelecting:
		return m.fileListView()
	case ConflictStateResolving:
		return m.resolveView()
	default:
		return ""
	}
}

// fileListView shows the list of conflicted files.
func (m ConflictResolverModel) fileListView() string {
	var b strings.Builder

	b.WriteString(m.headerStyle.Render("⚠️  Merge Conflicts Detected"))
	b.WriteString("\n\n")

	b.WriteString("Select a file to resolve:\n\n")

	// Count unresolved
	unresolved := 0
	for _, file := range m.ConflictedFiles {
		if !m.Resolved[file] {
			unresolved++
		}
	}

	b.WriteString(fmt.Sprintf("  %d file(s) with conflicts\n\n", unresolved))

	b.WriteString(m.List.View())

	b.WriteString("\n\n[Enter] Resolve file  [q] Quit\n")

	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Render(b.String())
}

// resolveView shows the diff and resolution options.
func (m ConflictResolverModel) resolveView() string {
	var b strings.Builder

	filePath := m.ConflictedFiles[m.CurrentFile]

	b.WriteString(m.headerStyle.Render(fmt.Sprintf("Resolving: %s", filePath)))
	b.WriteString("\n\n")

	// Diff viewer
	leftPanel := m.diffView()

	// Actions
	rightPanel := m.actionsView()

	// Join panels
	layout := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	b.WriteString(layout)

	return b.String()
}

// diffView renders the diff content.
func (m ConflictResolverModel) diffView() string {
	if m.DiffMode == DiffViewSideBySide {
		return m.sideBySideDiffView()
	}
	return m.unifiedDiffView()
}

// unifiedDiffView renders the unified diff with conflict markers.
func (m ConflictResolverModel) unifiedDiffView() string {
	var b strings.Builder

	b.WriteString(" Diff Viewer (Unified) [v] Toggle view\n\n")

	if m.DiffContent != "" {
		m.Viewport.SetContent(m.DiffContent)
	} else {
		m.Viewport.SetContent("Loading diff...")
	}

	b.WriteString(m.Viewport.View())

	width := m.width - 40
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

// sideBySideDiffView renders ours and theirs side by side.
func (m ConflictResolverModel) sideBySideDiffView() string {
	var b strings.Builder

	b.WriteString(" Diff Viewer (Side-by-Side) [v] Toggle view\n\n")

	// Set content for both viewports
	if m.OursContent != "" {
		m.OursViewport.SetContent(m.oursStyle.Render("OURS (Your Changes)\n\n") + m.OursContent)
	} else {
		m.OursViewport.SetContent("Loading...")
	}

	if m.TheirsContent != "" {
		m.TheirsViewport.SetContent(m.theirsStyle.Render("THEIRS (Their Changes)\n\n") + m.TheirsContent)
	} else {
		m.TheirsViewport.SetContent("Loading...")
	}

	// Render side by side
	oursPanel := m.OursViewport.View()
	theirsPanel := m.TheirsViewport.View()

	panelWidth := (m.width - 50) / 2
	if panelWidth < 30 {
		panelWidth = 30
	}

	oursStyled := lipgloss.NewStyle().
		Width(panelWidth).
		Height(m.height - 10).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("green")).
		Render(oursPanel)

	theirsStyled := lipgloss.NewStyle().
		Width(panelWidth).
		Height(m.height - 10).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("blue")).
		Render(theirsPanel)

	return lipgloss.JoinHorizontal(lipgloss.Top, oursStyled, theirsStyled)
}

// actionsView renders the resolution actions.
func (m ConflictResolverModel) actionsView() string {
	var b strings.Builder

	b.WriteString(" Resolution Options\n\n")

	b.WriteString("  [o] Accept 'ours' (your changes)")
	b.WriteString("\n\n")
	b.WriteString("  [t] Accept 'theirs' (their changes)")
	b.WriteString("\n\n")
	b.WriteString("  [m] Manual edit (opens mg)")
	b.WriteString("\n\n")
	b.WriteString("  [v] Toggle diff view")
	b.WriteString("\n\n")
	b.WriteString("  [a] Abort resolution")
	b.WriteString("\n\n")
	b.WriteString("  [esc] Back to file list")

	width := 30
	return lipgloss.NewStyle().
		Width(width).
		Height(m.height - 10).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Render(b.String())
}

// finishedView shows the final state.
func (m ConflictResolverModel) finishedView() string {
	var b strings.Builder

	if m.Aborted {
		b.WriteString("\n Resolution aborted.\n\n")
		b.WriteString("Conflicts remain unresolved.\n")
	} else {
		resolvedCount := 0
		for _, file := range m.ConflictedFiles {
			if m.Resolved[file] {
				resolvedCount++
			}
		}

		if resolvedCount == len(m.ConflictedFiles) {
			b.WriteString("\n ✓ All conflicts resolved!\n\n")
		} else {
			b.WriteString(fmt.Sprintf("\n Resolved %d/%d conflicts.\n\n", resolvedCount, len(m.ConflictedFiles)))
		}

		b.WriteString("Resolved files:\n")
		for _, file := range m.ConflictedFiles {
			if m.Resolved[file] {
				b.WriteString(fmt.Sprintf("  ✓ %s\n", file))
			}
		}
	}

	b.WriteString("\n Press Enter to exit...\n")

	return lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center, lipgloss.Center).
		Render(b.String())
}

// loadDiffForFile loads the diff content for a conflicted file.
func (m *ConflictResolverModel) loadDiffForFile(filePath string) {
	// Read the actual conflicted file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		m.DiffContent = fmt.Sprintf("Error reading file: %v", err)
		m.OursContent = m.DiffContent
		m.TheirsContent = ""
		return
	}

	// Parse conflict markers for unified view
	m.DiffContent = m.formatUnifiedDiff(string(content))

	// Get ours and theirs versions for side-by-side view
	m.OursContent = m.getOursVersion(filePath)
	m.TheirsContent = m.getTheirsVersion(filePath)

	// Reset viewport to top
	m.Viewport.SetContent(m.DiffContent)
	m.Viewport.GotoTop()
	m.OursViewport.SetContent(m.OursContent)
	m.OursViewport.GotoTop()
	m.TheirsViewport.SetContent(m.TheirsContent)
	m.TheirsViewport.GotoTop()
}

// formatUnifiedDiff formats the conflicted content with highlighted markers.
func (m *ConflictResolverModel) formatUnifiedDiff(content string) string {
	lines := strings.Split(content, "\n")
	var result strings.Builder

	inOurs := false
	inTheirs := false

	for i, line := range lines {
		// Detect conflict markers
		if strings.HasPrefix(line, "<<<<<<<") {
			inOurs = true
			result.WriteString(m.markerStyle.Render(line))
			result.WriteString("\n")
			continue
		}
		if strings.HasPrefix(line, "=======") && (inOurs || inTheirs) {
			inOurs = false
			inTheirs = true
			result.WriteString(m.markerStyle.Render(line))
			result.WriteString("\n")
			continue
		}
		if strings.HasPrefix(line, ">>>>>>>") {
			inTheirs = false
			result.WriteString(m.markerStyle.Render(line))
			result.WriteString("\n")
			continue
		}

		// Style content based on section
		if inOurs {
			result.WriteString(m.oursStyle.Render(fmt.Sprintf("%4d | %s", i+1, line)))
		} else if inTheirs {
			result.WriteString(m.theirsStyle.Render(fmt.Sprintf("%4d | %s", i+1, line)))
		} else {
			result.WriteString(m.normalStyle.Render(fmt.Sprintf("%4d | %s", i+1, line)))
		}
		result.WriteString("\n")
	}

	return result.String()
}

// getOursVersion extracts the "ours" version of the file.
func (m *ConflictResolverModel) getOursVersion(filePath string) string {
	// Use git show to get our version
	cmd := exec.Command("git", "show", ":2:"+filePath)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("Error loading ours: %v", err)
	}

	return m.formatWithLineNumbers(string(output))
}

// getTheirsVersion extracts the "theirs" version of the file.
func (m *ConflictResolverModel) getTheirsVersion(filePath string) string {
	// Use git show to get their version
	cmd := exec.Command("git", "show", ":3:"+filePath)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("Error loading theirs: %v", err)
	}

	return m.formatWithLineNumbers(string(output))
}

// formatWithLineNumbers adds line numbers to content.
func (m *ConflictResolverModel) formatWithLineNumbers(content string) string {
	lines := strings.Split(content, "\n")
	var result strings.Builder

	for i, line := range lines {
		result.WriteString(fmt.Sprintf("%4d | %s", i+1, line))
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// resolveConflict resolves a conflict using the specified strategy.
func (m *ConflictResolverModel) resolveConflict(filePath, strategy string) {
	var cmd *exec.Cmd

	switch strategy {
	case "ours":
		cmd = exec.Command("git", "checkout", "--ours", filePath)
	case "theirs":
		cmd = exec.Command("git", "checkout", "--theirs", filePath)
	case "manual":
		// File was edited externally, just mark as resolved
		cmd = exec.Command("git", "add", filePath)
	}

	if cmd != nil {
		if err := cmd.Run(); err != nil {
			// Log error but continue
			fmt.Fprintf(os.Stderr, "Error resolving %s: %v\n", filePath, err)
		}
	}

	// Mark file as resolved
	m.Resolved[filePath] = true

	// If using ours/theirs, need to add the file
	if strategy == "ours" || strategy == "theirs" {
		cmd = exec.Command("git", "add", filePath)
		_ = cmd.Run()
	}

	// Go back to file list
	m.State = ConflictStateSelecting

	// Check if all conflicts are resolved
	allResolved := true
	for _, file := range m.ConflictedFiles {
		if !m.Resolved[file] {
			allResolved = false
			break
		}
	}

	if allResolved {
		m.Finished = true
		m.State = ConflictStateFinished
	}
}

// openInEditor opens a file in the configured editor.
func (m *ConflictResolverModel) openInEditor(filePath string) {
	// Use mg as the editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "mg"
	}

	cmd := exec.Command(editor, filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	_ = cmd.Run()
}

// GetResolvedFiles returns the list of resolved files.
func (m *ConflictResolverModel) GetResolvedFiles() []string {
	var resolved []string
	for _, file := range m.ConflictedFiles {
		if m.Resolved[file] {
			resolved = append(resolved, file)
		}
	}
	return resolved
}

// DidAbort returns true if user aborted.
func (m *ConflictResolverModel) DidAbort() bool {
	return m.Aborted
}

// DidFinish returns true if resolution finished (aborted or complete).
func (m *ConflictResolverModel) DidFinish() bool {
	return m.Finished
}

// conflictFileItem is a list item for a conflicted file.
type conflictFileItem struct {
	index int
	path  string
}

func (c conflictFileItem) FilterValue() string {
	return c.path
}

// Title implements list.Item.
func (c conflictFileItem) Title() string {
	return c.path
}

// Description implements list.Item.
func (c conflictFileItem) Description() string {
	return ""
}

// conflictFileDelegate defines how files are rendered in the list.
type conflictFileDelegate struct{}

func (d conflictFileDelegate) Height() int  { return 1 }
func (d conflictFileDelegate) Spacing() int { return 0 }
func (d conflictFileDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d conflictFileDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	c, ok := listItem.(conflictFileItem)
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
		text = selectedStyle.Render("→ " + c.Title())
	} else {
		text = normalStyle.Render("  " + c.Title())
	}

	_, _ = fmt.Fprint(w, text)
}

// ConflictResolverResult is the result of running the conflict resolver.
type ConflictResolverResult struct {
	// Aborted indicates if the user aborted the resolution.
	Aborted bool

	// ResolvedCount is the number of files resolved.
	ResolvedCount int

	// TotalCount is the total number of conflicted files.
	TotalCount int
}

// RunConflictResolver runs the conflict resolver TUI.
// Returns the result of the resolution session.
func RunConflictResolver(conflictedFiles []string) (*ConflictResolverResult, error) {
	if len(conflictedFiles) == 0 {
		return &ConflictResolverResult{
			Aborted:       false,
			ResolvedCount: 0,
			TotalCount:    0,
		}, nil
	}

	// Create the model
	model := NewConflictResolverModel(conflictedFiles)

	// Create the program
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Run the program
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run conflict resolver: %w", err)
	}

	// Extract the result
	result := finalModel.(ConflictResolverModel)
	resolvedCount := 0
	for _, file := range conflictedFiles {
		if result.Resolved[file] {
			resolvedCount++
		}
	}

	return &ConflictResolverResult{
		Aborted:       result.Aborted,
		ResolvedCount: resolvedCount,
		TotalCount:    len(conflictedFiles),
	}, nil
}
