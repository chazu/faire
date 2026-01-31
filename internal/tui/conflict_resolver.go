package tui

import (
	"context"
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/chazuruo/faire/internal/gitrepo"
)

// ConflictResolverModel is the main model for resolving merge conflicts.
type ConflictResolverModel struct {
	ctx          context.Context
	repo         gitrepo.Repo
	conflicts    []gitrepo.ConflictFile
	currentIdx   int
	diffs        map[string]string
	resolutions  map[string]gitrepo.ResolutionChoice
	mergeState   gitrepo.MergeState
	list         list.Model
	viewport     viewport.Model
	diffViewType DiffViewType
	quit         bool
	aborted      bool
	completed    bool
	err          error
	width        int
	height       int
	listInit     bool
}

// DiffViewType represents which diff to show.
type DiffViewType int

const (
	DiffViewCombined DiffViewType = iota
	DiffViewOurs
	DiffViewTheirs
)

// ConflictResolverMsg is a message returned when resolution is complete.
type ConflictResolverMsg struct {
	Completed bool
	Aborted   bool
	Error     error
}

// NewConflictResolver creates a new conflict resolver model.
func NewConflictResolver(ctx context.Context, repo gitrepo.Repo) ConflictResolverModel {
	m := ConflictResolverModel{
		ctx:          ctx,
		repo:         repo,
		currentIdx:   0,
		diffs:        make(map[string]string),
		resolutions:  make(map[string]gitrepo.ResolutionChoice),
		diffViewType: DiffViewCombined,
		quit:         false,
		aborted:      false,
		completed:    false,
		listInit:     false,
	}

	// Initialize viewport
	vp := viewport.New(0, 0)
	m.viewport = vp

	return m
}

// Init initializes the conflict resolver.
func (m ConflictResolverModel) Init() tea.Cmd {
	return m.loadConflicts
}

// loadConflicts loads the list of conflicting files.
func (m ConflictResolverModel) loadConflicts() tea.Msg {
	// Get merge state
	state, err := m.repo.GetMergeState(m.ctx)
	if err != nil {
		return errMsg{err}
	}
	m.mergeState = state

	// Get conflict files
	conflictPaths, err := m.repo.GetConflictFiles(m.ctx)
	if err != nil {
		return errMsg{err}
	}

	// Build conflict file details
	conflicts := make([]gitrepo.ConflictFile, 0, len(conflictPaths))
	for _, path := range conflictPaths {
		details, err := m.repo.GetConflictDetails(m.ctx, path)
		if err != nil {
			return errMsg{fmt.Errorf("failed to get conflict details for %s: %w", path, err)}
		}
		conflicts = append(conflicts, details)
	}

	return conflictsLoadedMsg{conflicts}
}

type conflictsLoadedMsg struct {
	conflicts []gitrepo.ConflictFile
}

type errMsg struct {
	error
}

type resolveFileMsg struct {
	path string
	err  error
}

type applyAndContinueMsg struct {
	err error
}

// resolveFileCmd returns a command that resolves a single file.
func resolveFileCmd(ctx context.Context, repo gitrepo.Repo, path string, choice gitrepo.ResolutionChoice) tea.Cmd {
	return func() tea.Msg {
		err := repo.ResolveFile(ctx, path, choice)
		return resolveFileMsg{path: path, err: err}
	}
}

// applyAndContinueCmd returns a command that applies all resolutions and completes.
func applyAndContinueCmd(ctx context.Context, repo gitrepo.Repo, mergeState gitrepo.MergeState, resolutions map[string]gitrepo.ResolutionChoice) tea.Cmd {
	return func() tea.Msg {
		// First apply any pending resolutions
		for path, choice := range resolutions {
			if choice != gitrepo.ManualEdit {
				if err := repo.ResolveFile(ctx, path, choice); err != nil {
					return applyAndContinueMsg{err}
				}
			}
		}

		// Check if all conflicts are resolved
		hasConflicts, err := repo.HasConflicts(ctx)
		if err != nil {
			return applyAndContinueMsg{err}
		}

		if hasConflicts {
			return applyAndContinueMsg{fmt.Errorf("not all conflicts are resolved")}
		}

		// Complete the merge or rebase
		if mergeState.InRebase {
			err = repo.ContinueRebase(ctx)
		} else {
			err = repo.ContinueMerge(ctx)
		}

		return applyAndContinueMsg{err}
	}
}

// Update updates the conflict resolver model.
func (m ConflictResolverModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case conflictsLoadedMsg:
		m.conflicts = msg.conflicts
		m.initList()
		if len(m.conflicts) > 0 {
			m.loadDiffForCurrent()
		}
		return m, nil

	case errMsg:
		m.err = msg.error
		return m, nil

	case resolveFileMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		return m, nil

	case applyAndContinueMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.completed = true
		m.quit = true
		return m, m.finish

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSizes()
		return m, nil
	}

	// Update list
	if m.listInit {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// handleKeyMsg handles keyboard input.
func (m ConflictResolverModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quit = true
		m.aborted = true
		return m, m.finish

	case tea.KeyEsc:
		// Cancel current action or go back
		return m, nil

	case 'q':
		// Abort merge/rebase
		m.aborted = true
		m.quit = true
		return m, m.finish

	case 'a':
		// Apply resolution and continue
		return m.applyResolutionAndContinue()

	case 'o', '1':
		// Choose ours for current file
		return m.chooseOurs()

	case 't', '2':
		// Choose theirs for current file
		return m.chooseTheirs()

	case 'm', 'e':
		// Manual edit
		return m.manualEdit()

	case 'v', 'd':
		// Toggle diff view
		m.toggleDiffView()
		m.loadDiffForCurrent()
		return m, nil

	case tea.KeyUp, 'k':
		// Navigate up
		if m.listInit && m.list.Index() > 0 {
			m.list.Select(m.list.Index() - 1)
			m.currentIdx = m.list.Index()
			m.loadDiffForCurrent()
		}
		return m, nil

	case tea.KeyDown, 'j':
		// Navigate down
		if m.listInit && m.list.Index() < len(m.list.Items())-1 {
			m.list.Select(m.list.Index() + 1)
			m.currentIdx = m.list.Index()
			m.loadDiffForCurrent()
		}
		return m, nil

	case tea.KeyEnter:
		// Apply current resolution and move to next
		return m.applyCurrentResolution()
	}

	return m, nil
}

// chooseOurs selects our version for the current file.
func (m ConflictResolverModel) chooseOurs() (tea.Model, tea.Cmd) {
	if len(m.conflicts) == 0 {
		return m, nil
	}

	path := m.conflicts[m.currentIdx].Path
	m.resolutions[path] = gitrepo.ChooseOurs
	return m, nil
}

// chooseTheirs selects their version for the current file.
func (m ConflictResolverModel) chooseTheirs() (tea.Model, tea.Cmd) {
	if len(m.conflicts) == 0 {
		return m, nil
	}

	path := m.conflicts[m.currentIdx].Path
	m.resolutions[path] = gitrepo.ChooseTheirs
	return m, nil
}

// manualEdit opens the editor for manual resolution.
func (m ConflictResolverModel) manualEdit() (tea.Model, tea.Cmd) {
	if len(m.conflicts) == 0 {
		return m, nil
	}

	path := m.conflicts[m.currentIdx].Path
	return m, resolveFileCmd(m.ctx, m.repo, path, gitrepo.ManualEdit)
}

// applyCurrentResolution applies the resolution choice for the current file.
func (m ConflictResolverModel) applyCurrentResolution() (tea.Model, tea.Cmd) {
	if len(m.conflicts) == 0 {
		return m, nil
	}

	path := m.conflicts[m.currentIdx].Path
	choice, hasChoice := m.resolutions[path]

	if !hasChoice {
		return m, nil
	}

	return m, resolveFileCmd(m.ctx, m.repo, path, choice)
}

// applyResolutionAndContinue applies all resolutions and completes the merge/rebase.
func (m ConflictResolverModel) applyResolutionAndContinue() (tea.Model, tea.Cmd) {
	return m, applyAndContinueCmd(m.ctx, m.repo, m.mergeState, m.resolutions)
}

// finish sends the completion message.
func (m ConflictResolverModel) finish() tea.Msg {
	return ConflictResolverMsg{
		Completed: m.completed,
		Aborted:   m.aborted,
		Error:     m.err,
	}
}

// toggleDiffView switches between combined, ours, and theirs diff views.
func (m *ConflictResolverModel) toggleDiffView() {
	switch m.diffViewType {
	case DiffViewCombined:
		m.diffViewType = DiffViewOurs
	case DiffViewOurs:
		m.diffViewType = DiffViewTheirs
	case DiffViewTheirs:
		m.diffViewType = DiffViewCombined
	}
}

// loadDiffForCurrent loads the diff content for the currently selected file.
func (m *ConflictResolverModel) loadDiffForCurrent() {
	if len(m.conflicts) == 0 {
		return
	}

	path := m.conflicts[m.currentIdx].Path
	var content string

	switch m.diffViewType {
	case DiffViewCombined:
		// Show combined diff with conflict markers
		diff, err := m.repo.GetDiff(m.ctx, path, gitrepo.DiffCombined)
		if err == nil {
			content = diff.Content
		} else {
			content = fmt.Sprintf("Error loading diff: %v", err)
		}
	case DiffViewOurs:
		content = m.conflicts[m.currentIdx].Ours
	case DiffViewTheirs:
		content = m.conflicts[m.currentIdx].Theirs
	}

	m.diffs[path] = content
	m.viewport.SetContent(content)
	m.viewport.GotoTop()
}

// initList initializes the file list.
func (m *ConflictResolverModel) initList() {
	items := make([]list.Item, len(m.conflicts))
	for i, cf := range m.conflicts {
		items[i] = conflictFileItem{
			path:       cf.Path,
			resolution: m.resolutions[cf.Path],
		}
	}

	// Default list styles
	l := list.New(items, conflictFileDelegate{}, 0, 0)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Title = "Conflicting Files"
	m.list = l
	m.listInit = true
}

// updateSizes updates component sizes based on window size.
func (m *ConflictResolverModel) updateSizes() {
	// List takes 1/3 of width, viewport takes 2/3
	listWidth := m.width / 3
	viewportWidth := (m.width * 2) / 3

	if m.listInit {
		m.list.SetWidth(listWidth)
		m.list.SetHeight(m.height - 6) // Leave room for header/footer
	}

	m.viewport.Width = viewportWidth
	m.viewport.Height = m.height - 6
}

// View renders the conflict resolver.
func (m ConflictResolverModel) View() string {
	if m.err != nil {
		return m.renderError()
	}

	if len(m.conflicts) == 0 && m.err == nil {
		return "No conflicts found or conflicts already resolved."
	}

	header := m.renderHeader()
	body := m.renderBody()
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

// renderHeader renders the header.
func (m ConflictResolverModel) renderHeader() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).
		Bold(true).
		MarginBottom(1)

	var stateText string
	if m.mergeState.InRebase {
		stateText = "REBASE"
	} else {
		stateText = "MERGE"
	}

	return titleStyle.Render("Resolve Conflicts - " + stateText)
}

// renderBody renders the main content area.
func (m ConflictResolverModel) renderBody() string {
	if len(m.conflicts) == 0 {
		return ""
	}

	// File list (left side)
	listView := m.list.View()

	// Diff view (right side)
	diffStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238"))

	diffHeader := m.renderDiffHeader()
	diffContent := m.viewport.View()

	diffView := diffStyle.Render(diffHeader + "\n" + diffContent)

	// Combine side by side
	listWidth := m.width / 3
	listPane := lipgloss.NewStyle().Width(listWidth).MaxHeight(m.height - 6).Render(listView)

	return lipgloss.JoinHorizontal(lipgloss.Top, listPane, diffView)
}

// renderDiffHeader renders the diff view header.
func (m ConflictResolverModel) renderDiffHeader() string {
	if len(m.conflicts) == 0 {
		return ""
	}

	path := m.conflicts[m.currentIdx].Path

	// Show diff type indicator
	var typeIndicator string
	switch m.diffViewType {
	case DiffViewCombined:
		typeIndicator = "[Combined]"
	case DiffViewOurs:
		typeIndicator = "[Ours]"
	case DiffViewTheirs:
		typeIndicator = "[Theirs]"
	}

	// Check if resolved
	resolution, ok := m.resolutions[path]
	var resolvedIndicator string
	if ok && resolution != gitrepo.Unresolved {
		resolvedIndicator = " ✓"
	} else {
		resolvedIndicator = " ✗"
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Bold(true)

	return headerStyle.Render("Diff: " + path + resolvedIndicator + " " + typeIndicator)
}

// renderFooter renders the footer with keybindings.
func (m ConflictResolverModel) renderFooter() string {
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)

	help := " [j/k ↑/↓]: navigate [o]: ours [t]: theirs [m]: manual [v]: toggle diff [a]: apply & continue [q]: abort"

	return footerStyle.Render(help)
}

// renderError renders an error message.
func (m ConflictResolverModel) renderError() string {
	if m.err == nil {
		return ""
	}

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)

	return errorStyle.Render("Error: " + m.err.Error())
}

// conflictFileItem represents a conflicting file in the list.
type conflictFileItem struct {
	path       string
	resolution gitrepo.ResolutionChoice
}

func (c conflictFileItem) Title() string {
	status := "✗"
	if c.resolution != gitrepo.Unresolved {
		status = "✓"
	}
	return status + " " + c.path
}

func (c conflictFileItem) Description() string {
	return ""
}

func (c conflictFileItem) FilterValue() string {
	return c.path
}

// conflictFileDelegate defines how conflict files are rendered.
type conflictFileDelegate struct{}

func (d conflictFileDelegate) Height() int                               { return 1 }
func (d conflictFileDelegate) Spacing() int                              { return 0 }
func (d conflictFileDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d conflictFileDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	cf, ok := listItem.(conflictFileItem)
	if !ok {
		return
	}

	// Styles
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("251"))

	resolvedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86"))

	// Build the item text
	var status string
	if cf.resolution != gitrepo.Unresolved {
		status = resolvedStyle.Render("✓")
	} else {
		status = "✗"
	}

	text := cf.path
	if index == m.Index() {
		text = selectedStyle.Render("→ " + text)
	} else {
		text = normalStyle.Render("  " + text)
	}

	fmt.Fprint(w, status+" "+text)
}

// DidComplete returns true if the merge/rebase was completed successfully.
func (m *ConflictResolverModel) DidComplete() bool {
	return m.completed
}

// DidAbort returns true if the user aborted the merge/rebase.
func (m *ConflictResolverModel) DidAbort() bool {
	return m.aborted
}

// Error returns any error that occurred.
func (m *ConflictResolverModel) Error() error {
	return m.err
}

// RunConflictResolver starts and runs the conflict resolver TUI.
// Returns true if the merge/rebase was completed, false if aborted.
func RunConflictResolver(ctx context.Context, repo gitrepo.Repo) (bool, error) {
	model := NewConflictResolver(ctx, repo)
	program := tea.NewProgram(model, tea.WithAltScreen())

	result, err := program.Run()
	if err != nil {
		return false, fmt.Errorf("failed to run TUI: %w", err)
	}

	finalModel, ok := result.(ConflictResolverModel)
	if !ok {
		return false, fmt.Errorf("unexpected final model type")
	}

	if finalModel.Error() != nil {
		return false, finalModel.Error()
	}

	return finalModel.DidComplete(), nil
}
