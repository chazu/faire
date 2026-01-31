// Package tui provides Bubble Tea models for terminal UI interactions.
package tui

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/chazuruo/svf/internal/workflows"
)

// editingState represents the current editing mode.
type editingState int

const (
	editingNone editingState = iota
	editingStep
	editingPlaceholder
)

// WorkflowEditorModel is the main model for the workflow editor TUI.
type WorkflowEditorModel struct {
	ctx        context.Context
	workflow   *workflows.Workflow
	dirty      bool
	steps      list.Model
	title      textinput.Model
	desc       textinput.Model
	tags       textinput.Model
	currentStep int
	editing    editingState
	quit       bool
	saved      bool
	// Sub-models
	stepEditor       *StepEditorModel
	placeholderEditor *PlaceholderEditorModel
	// Placeholders detected in workflow
	detectedPlaceholders map[string][]string
}

// WorkflowEditorMsg is a message sent from sub-editors back to the main editor.
type WorkflowEditorMsg struct {
	Type  string // "step", "placeholder", "save", "quit"
	Data  interface{}
}

// NewWorkflowEditor creates a new workflow editor model.
func NewWorkflowEditor(ctx context.Context, wf *workflows.Workflow) WorkflowEditorModel {
	// Initialize title input
	ti := textinput.New()
	ti.Placeholder = "Workflow title"
	ti.SetValue(wf.Title)
	ti.Focus()

	// Initialize description input
	di := textinput.New()
	di.Placeholder = "Description (optional)"
	di.SetValue(wf.Description)

	// Initialize tags input
	tgi := textinput.New()
	tgi.Placeholder = "Tags (comma-separated)"
	tgi.SetValue(strings.Join(wf.Tags, ", "))

	// Initialize steps list
	items := make([]list.Item, len(wf.Steps))
	for i, step := range wf.Steps {
		name := step.Name
		if name == "" {
			name = fmt.Sprintf("Step %d", i+1)
		}
		items[i] = stepItem{index: i, name: name, command: step.Command}
	}

	li := list.New(items, stepDelegate{}, 0, 0)
	li.SetShowStatusBar(false)
	li.SetFilteringEnabled(false)
	li.Title = "Steps"

	return WorkflowEditorModel{
		ctx:                  ctx,
		workflow:             wf,
		dirty:                false,
		steps:                li,
		title:                ti,
		desc:                 di,
		tags:                 tgi,
		currentStep:          0,
		editing:              editingNone,
		quit:                 false,
		saved:                false,
		detectedPlaceholders: DetectPlaceholders(wf),
	}
}

// Init initializes the workflow editor.
func (m WorkflowEditorModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update updates the workflow editor model.
func (m WorkflowEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle key messages based on editing state
		switch m.editing {
		case editingStep:
			return m.handleStepEditing(msg)
		case editingPlaceholder:
			return m.handlePlaceholderEditing(msg)
		default:
			return m.handleNormalMode(msg)
		}

	case WorkflowEditorMsg:
		return m.handleEditorMsg(msg)
	}

	// Update text inputs
	var cmd tea.Cmd
	m.title, cmd = m.title.Update(msg)
	cmds = append(cmds, cmd)

	m.desc, cmd = m.desc.Update(msg)
	cmds = append(cmds, cmd)

	m.tags, cmd = m.tags.Update(msg)
	cmds = append(cmds, cmd)

	// Update steps list if not editing
	if m.editing == editingNone {
		m.steps, cmd = m.steps.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleNormalMode handles key messages in normal mode.
func (m WorkflowEditorModel) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quit = true
		return m, tea.Quit

	case tea.KeyCtrlS:
		// Save workflow
		return m, m.saveWorkflow()

	case tea.KeyCtrlQ:
		// Quit with dirty check
		if m.dirty {
			// TODO: Show confirmation dialog
			m.quit = true
			return m, tea.Quit
		}
		m.quit = true
		return m, tea.Quit

	case tea.KeyEnter:
		// Edit current step
		if len(m.steps.Items()) > 0 {
			m.editing = editingStep
			m.currentStep = m.steps.Index()
			m.stepEditor = NewStepEditor(m.workflow.Steps[m.currentStep], m.currentStep)
			return m, nil
		}

	case tea.KeyDelete, tea.KeyBackspace:
		// Delete current step
		if len(m.steps.Items()) > 0 {
			idx := m.steps.Index()
			m.workflow.Steps = append(m.workflow.Steps[:idx], m.workflow.Steps[idx+1:]...)
			m.updateStepItems()
			m.dirty = true
		}

	case tea.KeyCtrlN:
		// Add new step
		newStep := workflows.Step{
			Name:    fmt.Sprintf("Step %d", len(m.workflow.Steps)+1),
			Command: "",
		}
		m.workflow.Steps = append(m.workflow.Steps, newStep)
		m.updateStepItems()
		m.dirty = true
		// Switch to edit the new step
		m.editing = editingStep
		m.currentStep = len(m.workflow.Steps) - 1
		m.stepEditor = NewStepEditor(m.workflow.Steps[m.currentStep], m.currentStep)
		return m, nil

	case tea.KeyCtrlJ:
		// Move step down
		return m.moveStep(1), nil

	case tea.KeyCtrlK:
		// Move step up
		return m.moveStep(-1), nil

	case tea.KeyCtrlP:
		// Open placeholder editor
		m.editing = editingPlaceholder
		m.detectedPlaceholders = DetectPlaceholders(m.workflow)
		m.placeholderEditor = NewPlaceholderEditor(m.workflow.Placeholders, m.detectedPlaceholders)
		return m, nil

	case tea.KeyUp, tea.KeyDown:
		// Navigate steps
		steps, cmd := m.steps.Update(msg)
		m.steps = steps
		return m, cmd
	}

	// Update focused text input
	if m.title.Focused() {
		m.title, _ = m.title.Update(msg)
		m.workflow.Title = m.title.Value()
		m.dirty = true
	} else if m.desc.Focused() {
		m.desc, _ = m.desc.Update(msg)
		m.workflow.Description = m.desc.Value()
		m.dirty = true
	} else if m.tags.Focused() {
		m.tags, _ = m.tags.Update(msg)
		m.workflow.Tags = parseTags(m.tags.Value())
		m.dirty = true
	}

	return m, nil
}

// handleStepEditing handles messages when editing a step.
func (m WorkflowEditorModel) handleStepEditing(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.stepEditor == nil {
		m.editing = editingNone
		return m, nil
	}

	var cmd tea.Cmd
	var model tea.Model
	model, cmd = m.stepEditor.Update(msg)
	m.stepEditor = model.(*StepEditorModel)

	// Check if step editor is done
	if m.stepEditor.Done {
		// Save the step
		m.workflow.Steps[m.currentStep] = m.stepEditor.Step
		m.updateStepItems()
		m.dirty = true
		m.editing = editingNone
		m.stepEditor = nil
		return m, nil
	}

	if m.stepEditor.Cancelled {
		m.editing = editingNone
		m.stepEditor = nil
		return m, nil
	}

	return m, cmd
}

// handlePlaceholderEditing handles messages when editing placeholders.
func (m WorkflowEditorModel) handlePlaceholderEditing(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.placeholderEditor == nil {
		m.editing = editingNone
		return m, nil
	}

	var cmd tea.Cmd
	var model tea.Model
	model, cmd = m.placeholderEditor.Update(msg)
	m.placeholderEditor = model.(*PlaceholderEditorModel)

	// Check if placeholder editor is done
	if m.placeholderEditor.Done || m.placeholderEditor.Cancelled {
		m.workflow.Placeholders = m.placeholderEditor.Placeholders
		m.dirty = true
		m.editing = editingNone
		m.placeholderEditor = nil
		return m, nil
	}

	return m, cmd
}

// handleEditorMsg handles messages from sub-editors.
func (m WorkflowEditorModel) handleEditorMsg(msg WorkflowEditorMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case "save":
		m.saved = true
		m.quit = true
		return m, tea.Quit
	case "quit":
		m.quit = true
		return m, tea.Quit
	}
	return m, nil
}

// moveStep moves a step up or down in the list.
func (m WorkflowEditorModel) moveStep(dir int) WorkflowEditorModel {
	if len(m.steps.Items()) == 0 {
		return m
	}

	idx := m.steps.Index()
	newIdx := idx + dir

	if newIdx < 0 || newIdx >= len(m.workflow.Steps) {
		return m
	}

	// Swap steps
	m.workflow.Steps[idx], m.workflow.Steps[newIdx] = m.workflow.Steps[newIdx], m.workflow.Steps[idx]
	m.updateStepItems()
	m.steps.Select(newIdx)
	m.dirty = true

	return m
}

// updateStepItems updates the steps list items from the workflow.
func (m *WorkflowEditorModel) updateStepItems() {
	items := make([]list.Item, len(m.workflow.Steps))
	for i, step := range m.workflow.Steps {
		name := step.Name
		if name == "" {
			name = fmt.Sprintf("Step %d", i+1)
		}
		items[i] = stepItem{index: i, name: name, command: step.Command}
	}
	m.steps.SetItems(items)
}

// saveWorkflow saves the workflow and returns a quit command.
func (m WorkflowEditorModel) saveWorkflow() tea.Cmd {
	return func() tea.Msg {
		return WorkflowEditorMsg{Type: "save", Data: m.workflow}
	}
}

// View renders the workflow editor.
func (m WorkflowEditorModel) View() string {
	switch m.editing {
	case editingStep:
		if m.stepEditor != nil {
			return m.stepEditor.View()
		}
	case editingPlaceholder:
		if m.placeholderEditor != nil {
			return m.placeholderEditor.View()
		}
	}

	// Main view
	header := m.renderHeader()
	steps := m.renderSteps()
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, header, steps, footer)
}

// renderHeader renders the header with title, description, and tags inputs.
func (m WorkflowEditorModel) renderHeader() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Width(8)

	titleLabel := labelStyle.Render("Title:")
	descLabel := labelStyle.Render("Desc:")
	tagsLabel := labelStyle.Render("Tags:")

	title := m.title.View()
	desc := m.desc.View()
	tags := m.tags.View()

	return titleStyle.Render("Workflow Editor") + "\n\n" +
		titleLabel + " " + title + "\n" +
		descLabel + " " + desc + "\n" +
		tagsLabel + " " + tags + "\n"
}

// renderSteps renders the steps list.
func (m WorkflowEditorModel) renderSteps() string {
	if m.steps.Height() == 0 {
		m.steps.SetHeight(10)
	}
	return m.steps.View()
}

// renderFooter renders the footer with help text.
func (m WorkflowEditorModel) renderFooter() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)

	help := " [Ctrl+S]: save [Ctrl+Q]: quit [Enter]: edit step [Ctrl+N]: new step\n" +
		" [Ctrl+J/K]: move step [Del]: delete step [Ctrl+P]: placeholders [↑/↓]: navigate"

	if m.dirty {
		help = " [Ctrl+S]: save* [Ctrl+Q]: quit [Enter]: edit step [Ctrl+N]: new step\n" +
			" [Ctrl+J/K]: move step [Del]: delete step [Ctrl+P]: placeholders [↑/↓]: navigate"
	}

	return helpStyle.Render(help)
}

// stepItem represents a workflow step in the list.
type stepItem struct {
	index   int
	name    string
	command string
}

func (s stepItem) Title() string {
	return s.name
}

func (s stepItem) Description() string {
	// Truncate command if too long
	const maxLen = 50
	cmd := s.command
	if len(cmd) > maxLen {
		cmd = cmd[:maxLen] + "..."
	}
	return cmd
}

func (s stepItem) FilterValue() string {
	return s.name + " " + s.command
}

// stepDelegate defines how steps are rendered in the list.
type stepDelegate struct{}

func (d stepDelegate) Height() int                               { return 1 }
func (d stepDelegate) Spacing() int                              { return 0 }
func (d stepDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d stepDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	s, ok := listItem.(stepItem)
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
		text = selectedStyle.Render("→ " + s.Title())
	} else {
		text = normalStyle.Render("  " + s.Title())
	}

	fmt.Fprint(w, text)
}

// parseTags parses a comma-separated string into a slice of tags.
func parseTags(s string) []string {
	tags := strings.Split(s, ",")
	var result []string
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			result = append(result, tag)
		}
	}
	return result
}

// DetectPlaceholders finds all placeholder tokens in workflow steps.
// Returns a map of placeholder name to list of step indices where used.
func DetectPlaceholders(workflow *workflows.Workflow) map[string][]string {
	found := make(map[string][]string)
	re := regexp.MustCompile(`<([a-zA-Z_][a-zA-Z0-9_-]*)>`)

	for i, step := range workflow.Steps {
		matches := re.FindAllStringSubmatch(step.Command, -1)
		for _, m := range matches {
			if len(m) > 1 {
				found[m[1]] = append(found[m[1]], fmt.Sprintf("%d", i))
			}
		}
	}
	return found
}

// GetWorkflow returns the edited workflow.
func (m *WorkflowEditorModel) GetWorkflow() *workflows.Workflow {
	return m.workflow
}

// IsDirty returns true if the workflow has been modified.
func (m *WorkflowEditorModel) IsDirty() bool {
	return m.dirty
}

// DidQuit returns true if the user quit without saving.
func (m *WorkflowEditorModel) DidQuit() bool {
	return m.quit && !m.saved
}

// DidSave returns true if the user saved the workflow.
func (m *WorkflowEditorModel) DidSave() bool {
	return m.saved
}
