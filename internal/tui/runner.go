// Package tui provides Bubble Tea models for svf.
package tui

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/chazuruo/svf/internal/config"
	"github.com/chazuruo/svf/internal/placeholders"
	//nolint:staticcheck // SA1019 - Using runner for Exec, DangerChecker, Plan types (deprecated but needed)
	runnerpkg "github.com/chazuruo/svf/internal/runner"
	"github.com/chazuruo/svf/internal/workflows"
)

// RunnerModel is a Bubble Tea model for running workflows interactively.
type RunnerModel struct {
	// Plan is the execution plan.
	Plan runnerpkg.Plan

	// Config is the application config
	Config *config.Config

	// CurrentStep is the current step being executed.
	CurrentStep int

	// StepResults contains results for executed steps.
	StepResults []runnerpkg.StepResult

	// Placeholders contains resolved placeholder values.
	Placeholders map[string]string

	// PlaceholderInfo contains metadata about placeholders.
	PlaceholderInfo map[string]placeholders.PlaceholderInfo

	// CurrentPlaceholder is the placeholder currently being prompted for.
	CurrentPlaceholder string

	// PlaceholderInput is the text input for placeholder values.
	PlaceholderInput textinput.Model

	// PlaceholderError is any error from placeholder validation.
	PlaceholderError string

	// State is the current runner state.
	State RunnerState

	// Sub-state for editing/viewing
	ShowPlaceholders bool // Showing placeholder values view
	EditingStep      bool // Editing current step
	EditedStep       workflows.Step // Temporary storage for edited step

	// List is the step list component.
	List list.Model

	// Viewport is the output viewport.
	Viewport viewport.Model

	// Output contains the latest command output.
	Output strings.Builder

	// Help is the keybindings help.
	Help help.Model

	// ShowHelp controls whether to show the help panel.
	ShowHelp bool

	// Finished indicates if the run is complete.
	Finished bool

	// Success indicates if the run succeeded.
	Success bool

	// Canceled indicates if the user canceled.
	Canceled bool

	// DangerChecker for dangerous command checking
	DangerChecker *runnerpkg.DangerChecker

	// AutoConfirm dangerous commands
	AutoConfirm bool

	// StreamOutput controls whether to stream command output
	StreamOutput bool

	// styles
	normalStyle    lipgloss.Style
	selectedStyle  lipgloss.Style
	successStyle   lipgloss.Style
	errorStyle     lipgloss.Style
	runningStyle   lipgloss.Style
	pendingStyle   lipgloss.Style
	dimStyle       lipgloss.Style
	accentStyle    lipgloss.Style
	borderStyle    lipgloss.Style

	// width and height
	width  int
	height int

	// Key bindings
	keyMap runnerKeyMap
}

// runnerKeyMap defines key bindings for the runner.
type runnerKeyMap struct {
	Run         key.Binding
	Skip        key.Binding
	Rerun       key.Binding
	Quit        key.Binding
	EditStep    key.Binding
	ToggleHelp  key.Binding
	ShowPlace   key.Binding
	Enter       key.Binding
}

// RunnerState represents the current state of the runner.
type RunnerState int

const (
	// StateReady means ready to run the workflow.
	StateReady RunnerState = iota
	// StatePrompting means prompting for placeholders.
	StatePrompting
	// StateRunning means a step is running.
	StateRunning
	// StateStepResult means showing step result.
	StateStepResult
	// StateFinished means the workflow is complete.
	StateFinished
)

// RunnerMsg is sent when a step finishes.
type RunnerMsg struct {
	Result runnerpkg.StepResult
}

// OutputMsg is sent when there's new output.
type OutputMsg string

// newRunnerKeyMap creates the key bindings for the runner.
func newRunnerKeyMap() runnerKeyMap {
	return runnerKeyMap{
		Run: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "run step"),
		),
		Skip: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "skip"),
		),
		Rerun: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "rerun"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		EditStep: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit step"),
		),
		ToggleHelp: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		ShowPlace: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "placeholders"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
	}
}

// NewRunnerModel creates a new runner model.
func NewRunnerModel(plan runnerpkg.Plan, dangerChecker *runnerpkg.DangerChecker, autoConfirm bool, streamOutput bool) RunnerModel {
	return newRunnerModelWithContext(plan, dangerChecker, autoConfirm, streamOutput, nil)
}

// NewRunnerModelWithConfig creates a new runner model with full config support.
func NewRunnerModelWithConfig(plan runnerpkg.Plan, cfg *config.Config) RunnerModel {
	dangerChecker := runnerpkg.NewDangerChecker(cfg.Runner.DangerousCommandWarnings)
	return newRunnerModelWithContext(plan, dangerChecker, false, cfg.Runner.StreamOutput, cfg)
}

func newRunnerModelWithContext(plan runnerpkg.Plan, dangerChecker *runnerpkg.DangerChecker, autoConfirm bool, streamOutput bool, cfg *config.Config) RunnerModel {
	// Extract placeholder info
	phInfo := placeholders.ExtractWithMetadata(plan.Workflow)

	// Create step list
	items := []list.Item{}
	for i, step := range plan.Workflow.Steps {
		name := step.Name
		if name == "" {
			name = fmt.Sprintf("Step %d", i+1)
		}
		items = append(items, runnerStepItem{index: i, name: name})
	}

	// Create list
	l := list.New(items, runnerStepDelegate{}, 0, 0)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	// Create viewport
	vp := viewport.New(80, 20)

	// Create help
	h := help.New()

	// Determine initial state - start with prompting if we have placeholders
	initialState := StateReady
	if len(phInfo) > 0 && len(plan.Parameters) == 0 {
		// We have placeholders but no values, start in prompting mode
		initialState = StatePrompting
	}

	// Styles
	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Bold(true)
	successStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("green"))
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("red"))
	runningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("yellow"))
	pendingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242"))
	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))
	accentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")) // Pink/cyan
	borderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	return RunnerModel{
		Plan:            plan,
		Config:          cfg,
		CurrentStep:     0,
		StepResults:     make([]runnerpkg.StepResult, len(plan.Workflow.Steps)),
		Placeholders:    plan.Parameters,
		PlaceholderInfo: phInfo,
		State:           initialState,
		List:            l,
		Viewport:        vp,
		Help:            h,
		ShowHelp:        true,
		Finished:        false,
		DangerChecker:   dangerChecker,
		AutoConfirm:     autoConfirm,
		StreamOutput:    streamOutput,
		keyMap:          newRunnerKeyMap(),
		normalStyle:     normalStyle,
		selectedStyle:   selectedStyle,
		successStyle:    successStyle,
		errorStyle:      errorStyle,
		runningStyle:    runningStyle,
		pendingStyle:    pendingStyle,
		dimStyle:        dimStyle,
		accentStyle:     accentStyle,
		borderStyle:     borderStyle,
	}
}

// Init implements tea.Model.
func (m RunnerModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m RunnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle prompting state separately
	if m.State == StatePrompting {
		return m.handlePrompting(msg)
	}

	// Handle finished state
	if m.Finished {
		if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle key messages based on current sub-state
		if m.ShowPlaceholders {
			switch msg.String() {
			case "q", "esc", "p", "enter":
				m.ShowPlaceholders = false
				return m, nil
			}
			return m, nil
		}

		if m.EditingStep {
			return m.handleStepEditing(msg)
		}

		// Normal mode key bindings
		switch {
		case key.Matches(msg, m.keyMap.Quit):
			m.Canceled = true
			m.Finished = true
			m.State = StateFinished
			return m, tea.Quit

		case key.Matches(msg, m.keyMap.Run):
			if m.State == StateReady || m.State == StateStepResult {
				// Run next step
				if m.CurrentStep < len(m.Plan.Workflow.Steps) {
					cmds = append(cmds, m.runStep(m.CurrentStep))
				} else {
					// All steps done
					m.Finished = true
					m.Success = true
					m.State = StateFinished
					return m, tea.Quit
				}
			}

		case key.Matches(msg, m.keyMap.Skip):
			// Skip current step
			if m.State == StateReady || m.State == StateStepResult {
				// Mark as skipped
				if m.CurrentStep < len(m.StepResults) {
					m.StepResults[m.CurrentStep] = runnerpkg.StepResult{
						Step:    m.CurrentStep,
						Success: true, // Treat skip as success
						Output:   "(skipped)",
					}
				}
				m.CurrentStep++
				if m.CurrentStep >= len(m.Plan.Workflow.Steps) {
					m.Finished = true
					m.Success = true
					m.State = StateFinished
					return m, tea.Quit
				}
				m.List.Select(m.CurrentStep)
				return m, nil
			}

		case key.Matches(msg, m.keyMap.Rerun):
			// Re-run current step
			if m.State == StateStepResult {
				// Reset to previous step for rerun
				if m.CurrentStep > 0 {
					m.CurrentStep--
				}
				cmds = append(cmds, m.runStep(m.CurrentStep))
				return m, m.Batch(cmds...)
			}

		case key.Matches(msg, m.keyMap.EditStep):
			// Edit current step
			if m.State == StateReady || m.State == StateStepResult {
				if m.CurrentStep < len(m.Plan.Workflow.Steps) {
					m.EditingStep = true
					// Copy the current step for editing
					step := m.Plan.Workflow.Steps[m.CurrentStep]
					m.EditedStep = step
					return m, nil
				}
			}

		case key.Matches(msg, m.keyMap.ToggleHelp):
			m.ShowHelp = !m.ShowHelp
			return m, nil

		case key.Matches(msg, m.keyMap.ShowPlace):
			m.ShowPlaceholders = true
			return m, nil
		}

	case RunnerMsg:
		// Step finished
		m.StepResults[msg.Result.Step] = msg.Result
		m.Output.Reset()
		m.Output.WriteString(msg.Result.Output)
		m.Viewport.SetContent(m.Output.String())
		m.Viewport.GotoBottom()
		m.State = StateStepResult

		if msg.Result.Success {
			m.CurrentStep++
			if m.CurrentStep < len(m.Plan.Workflow.Steps) {
				m.List.Select(m.CurrentStep)
			} else {
				// All steps done
				m.Finished = true
				m.Success = true
				m.State = StateFinished
				return m, tea.Quit
			}
		} else {
			// Step failed - check if continue on error
			if m.CurrentStep < len(m.Plan.Workflow.Steps) {
				step := m.Plan.Workflow.Steps[m.CurrentStep]
				if step.ContinueOnError {
					// Continue to next step
					m.CurrentStep++
					if m.CurrentStep < len(m.Plan.Workflow.Steps) {
						m.List.Select(m.CurrentStep)
						m.State = StateReady
						return m, nil
					}
				}
			}
			// Stop execution
			m.Finished = true
			m.Success = false
			m.State = StateFinished
			return m, tea.Quit
		}

	case OutputMsg:
		// New output during execution
		m.Output.WriteString(string(msg))
		m.Viewport.SetContent(m.Output.String())
		m.Viewport.GotoBottom()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.Viewport.Width = msg.Width - 40 // Leave room for step list
		m.Viewport.Height = msg.Height - 10
	}

	// Update child components
	var listCmd tea.Cmd
	m.List, listCmd = m.List.Update(msg)
	if listCmd != nil {
		cmds = append(cmds, listCmd)
	}

	var vpCmd tea.Cmd
	m.Viewport, vpCmd = m.Viewport.Update(msg)
	if vpCmd != nil {
		cmds = append(cmds, listCmd)
	}

	return m, m.Batch(cmds...)
}

// handleStepEditing handles key messages when editing a step.
func (m RunnerModel) handleStepEditing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		// Cancel editing
		m.EditingStep = false
		m.EditedStep = workflows.Step{}
		return m, nil

	case "enter":
		// Save edited step
		if m.CurrentStep < len(m.Plan.Workflow.Steps) {
			m.Plan.Workflow.Steps[m.CurrentStep] = m.EditedStep
			// Update list item name if changed
			if m.EditedStep.Name != "" {
				items := m.List.Items()
				if m.CurrentStep < len(items) {
					items[m.CurrentStep] = runnerStepItem{index: m.CurrentStep, name: m.EditedStep.Name}
					m.List.SetItems(items)
				}
			}
		}
		m.EditingStep = false
		m.EditedStep = workflows.Step{}
		return m, nil
	}

	// Simple editing - just capture command input
	// For a full editor, you'd want textinput fields
	return m, nil
}

// View implements tea.Model.
func (m RunnerModel) View() string {
	if m.Finished {
		return m.finishedView()
	}

	if m.State == StatePrompting {
		return m.promptingView()
	}

	if m.ShowPlaceholders {
		return m.placeholdersView()
	}

	if m.EditingStep {
		return m.editingStepView()
	}

	// Layout: left panel (step list), right panel (output + help)
	leftPanel := m.stepListView()
	rightPanel := m.outputView()

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

// placeholdersView renders the placeholder values view.
func (m RunnerModel) placeholdersView() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("Placeholder Values\n\n"))

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Bold(true)

	b.WriteString(headerStyle.Render(fmt.Sprintf("%-20s %s", "Name", "Value")))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("-", 60))
	b.WriteString("\n")

	// List all placeholders with their values
	for name, info := range m.PlaceholderInfo {
		value := m.Placeholders[name]
		if value == "" {
			value = info.Default
		}
		if value == "" {
			value = "(not set)"
		}

		// Mask secret values
		if info.Secret && value != "(not set)" {
			value = "***"
		}

		nameStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("251")).
			Width(20)

		valueStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

		b.WriteString(nameStyle.Render(name))
		b.WriteString(valueStyle.Render(value))
		b.WriteString("\n")
	}

	// Footer help
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(2)

	b.WriteString(footerStyle.Render("[Enter/Esc/P] Close"))

	return lipgloss.NewStyle().
		Width(70).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Render(b.String())
}

// editingStepView renders the step editing view.
func (m RunnerModel) editingStepView() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true).
		MarginBottom(1)

	b.WriteString(titleStyle.Render(fmt.Sprintf("Edit Step: %s\n\n", m.Plan.Workflow.Steps[m.CurrentStep].Name)))

	// Current command
	cmdStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("251")).
		MarginBottom(1)

	if m.EditedStep.Command != "" {
		b.WriteString(cmdStyle.Render("Command:"))
		b.WriteString("\n")
		b.WriteString(m.dimStyle.Render(m.EditedStep.Command))
		b.WriteString("\n\n")
	} else {
		originalCmd := m.Plan.Workflow.Steps[m.CurrentStep].Command
		b.WriteString(cmdStyle.Render("Original Command:"))
		b.WriteString("\n")
		b.WriteString(m.dimStyle.Render(originalCmd))
		b.WriteString("\n\n")
	}

	// Note about editing
	noteStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Italic(true)

	b.WriteString(noteStyle.Render("Note: Full step editing not yet implemented.\nThis view shows the current step for reference.\n\n"))

	// Footer help
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)

	b.WriteString(footerStyle.Render("[Esc] Cancel  [Enter] Continue"))

	return lipgloss.NewStyle().
		Width(70).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Render(b.String())
}

// promptingView renders the placeholder prompting view.
func (m RunnerModel) promptingView() string {
	var b strings.Builder

	// If we don't have a current placeholder set, find one
	if m.CurrentPlaceholder == "" {
		for name := range m.PlaceholderInfo {
			if _, ok := m.Placeholders[name]; !ok {
				m.CurrentPlaceholder = name
				m.setupPlaceholderInput()
				break
			}
		}
	}

	// If still no placeholder, we're done
	if m.CurrentPlaceholder == "" {
		m.State = StateReady
		return m.View()
	}

	info := m.PlaceholderInfo[m.CurrentPlaceholder]

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")). // Pink
		Bold(true).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("Workflow Placeholders\n\n"))

	// Prompt text
	promptText := info.Prompt
	if promptText == "" {
		promptText = fmt.Sprintf("Enter value for <%s>", info.Name)
	}

	promptStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("251")).
		MarginBottom(1)

	b.WriteString(promptStyle.Render(promptText + "\n"))

	// Usage info
	if len(info.UsedIn) > 0 {
		usageStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")).
			MarginBottom(1)

		b.WriteString(usageStyle.Render("Used in: " + strings.Join(info.UsedIn, ", ") + "\n\n"))
	}

	// Default value hint
	if info.Default != "" {
		hintStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			MarginBottom(1)

		b.WriteString(hintStyle.Render(fmt.Sprintf("Default: %s\n\n", info.Default)))
	}

	// Error message
	if m.PlaceholderError != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("203")).
			MarginBottom(1)

		b.WriteString(errorStyle.Render("Error: " + m.PlaceholderError + "\n\n"))
	}

	// Input field
	b.WriteString(m.PlaceholderInput.View())

	// Help footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(2)

	remaining := len(m.PlaceholderInfo) - len(m.Placeholders)
	b.WriteString(footerStyle.Render(
		fmt.Sprintf("\n\n[Enter] Submit (%d remaining) [Ctrl+C] Cancel", remaining),
	))

	return lipgloss.NewStyle().
		Width(80).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Render(b.String())
}

// stepListView renders the step list.
func (m RunnerModel) stepListView() string {
	var b strings.Builder

	b.WriteString(" Steps\n\n")

	// Render list with custom styling
	for i, item := range m.List.Items() {
		step := item.(runnerStepItem)
		var style lipgloss.Style

		// Apply status-based styling
		if i < m.CurrentStep {
			// Already completed
			if m.StepResults[i].Success {
				style = m.successStyle
			} else {
				style = m.errorStyle
			}
		} else if i == m.CurrentStep {
			// Current step
			if m.State == StateRunning {
				style = m.runningStyle
			} else {
				style = m.selectedStyle
			}
		} else {
			// Pending
			style = m.pendingStyle
		}

		// Status icon
		icon := " "
		if i < m.CurrentStep {
			if m.StepResults[i].Success {
				icon = "✓"
			} else {
				icon = "✗"
			}
		} else if i == m.CurrentStep && m.State == StateRunning {
			icon = "▶"
		}

		line := fmt.Sprintf("%s %s", icon, step.name)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	width := 30
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Render(b.String())
}

// outputView renders the output viewport and help.
func (m RunnerModel) outputView() string {
	var b strings.Builder

	// Build dynamic key bindings based on state
	var keys []key.Binding
	if m.State == StateStepResult {
		keys = []key.Binding{m.keyMap.Run, m.keyMap.Skip, m.keyMap.Rerun, m.keyMap.Quit}
	} else {
		keys = []key.Binding{m.keyMap.Run, m.keyMap.Skip, m.keyMap.Quit}
	}
	keys = append(keys, m.keyMap.EditStep, m.keyMap.ShowPlace, m.keyMap.ToggleHelp)

	// Calculate viewport height (leave room for help if shown)
	viewportHeight := m.height - 8
	if !m.ShowHelp {
		viewportHeight = m.height - 4
	}
	if viewportHeight < 10 {
		viewportHeight = 10
	}

	m.Viewport.Height = viewportHeight

	b.WriteString(" Output\n\n")
	b.WriteString(m.Viewport.View())

	if m.ShowHelp {
		b.WriteString("\n\n")
		// Render help text manually since we're using dynamic bindings
		helpParts := []string{}
		for _, k := range keys {
			if k.Help().Key != "" && k.Help().Desc != "" {
				helpParts = append(helpParts, fmt.Sprintf("[%s] %s", k.Help().Key, k.Help().Desc))
			}
		}
		helpText := strings.Join(helpParts, " • ")
		b.WriteString(m.dimStyle.Render(helpText))
	}

	width := m.width - 40
	if width < 40 {
		width = 40
	}
	return lipgloss.NewStyle().
		Width(width).
		Height(m.height - 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Render(b.String())
}

// handlePrompting handles key messages when prompting for placeholders.
func (m RunnerModel) handlePrompting(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.Canceled = true
			m.Finished = true
			m.State = StateFinished
			return m, tea.Quit

		case "enter":
			// Submit current placeholder value
			value := m.PlaceholderInput.Value()

			// Use default if empty
			if value == "" && m.PlaceholderInfo[m.CurrentPlaceholder].Default != "" {
				value = m.PlaceholderInfo[m.CurrentPlaceholder].Default
			}

			// Validate if pattern is provided
			if m.PlaceholderInfo[m.CurrentPlaceholder].Validate != "" {
				if err := placeholders.Validate(value, m.PlaceholderInfo[m.CurrentPlaceholder].Validate); err != nil {
					m.PlaceholderError = err.Error()
					return m, nil
				}
			}

			// Store the value
			if m.Placeholders == nil {
				m.Placeholders = make(map[string]string)
			}
			m.Placeholders[m.CurrentPlaceholder] = value

			// Clear error and find next placeholder
			m.PlaceholderError = ""
			m.CurrentPlaceholder = ""

			// Find next placeholder without a value
			for name := range m.PlaceholderInfo {
				if _, ok := m.Placeholders[name]; !ok {
					m.CurrentPlaceholder = name
					break
				}
			}

			// If no more placeholders, move to ready state
			if m.CurrentPlaceholder == "" {
				m.State = StateReady
				return m, nil
			}

			// Setup input for next placeholder
			m.setupPlaceholderInput()
			return m, nil
		}

	// Update text input
	var cmd tea.Cmd
	m.PlaceholderInput, cmd = m.PlaceholderInput.Update(msg)
	return m, cmd
	}

	return m, nil
}

// setupPlaceholderInput sets up the text input for the current placeholder.
func (m *RunnerModel) setupPlaceholderInput() {
	info := m.PlaceholderInfo[m.CurrentPlaceholder]

	// Create input
	ti := textinput.New()
	ti.Placeholder = "Enter value"

	// Set default value in the input
	if info.Default != "" {
		ti.SetValue(info.Default)
		ti.Placeholder = fmt.Sprintf("Default: %s", info.Default)
	}

	// Set prompt text
	if info.Prompt != "" {
		ti.Placeholder = info.Prompt
	}

	ti.Focus()
	m.PlaceholderInput = ti
}

// finishedView renders the finished state.
func (m RunnerModel) finishedView() string {
	var b strings.Builder

	if m.Success {
		b.WriteString("\n")
		b.WriteString(m.successStyle.Render("✓ Workflow completed successfully!"))
		b.WriteString("\n\n")
	} else if m.Canceled {
		b.WriteString("\n")
		b.WriteString(m.dimStyle.Render("Workflow canceled."))
		b.WriteString("\n\n")
	} else {
		b.WriteString("\n")
		b.WriteString(m.errorStyle.Render("✗ Workflow failed."))
		b.WriteString("\n\n")
	}

	// Show summary
	b.WriteString(m.dimStyle.Render("Step Results:\n\n"))
	for i, result := range m.StepResults {
		if i >= len(m.Plan.Workflow.Steps) {
			break
		}
		name := m.Plan.Workflow.Steps[i].Name
		if name == "" {
			name = fmt.Sprintf("Step %d", i+1)
		}

		status := "✓"
		style := m.successStyle
		if !result.Success && result.Output != "" {
			status = "✗"
			style = m.errorStyle
		}
		if result.Output == "" && i >= m.CurrentStep {
			status = "○"
			style = m.dimStyle
		}

		b.WriteString(fmt.Sprintf("   %s %s\n", style.Render(status), name))
	}

	b.WriteString("\n")
	b.WriteString(m.dimStyle.Render("Press Enter to exit...\n"))

	return lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center, lipgloss.Center).
		Render(b.String())
}

// runStep executes a step and returns a command.
func (m RunnerModel) runStep(stepIndex int) tea.Cmd {
	return func() tea.Msg {
		// Mark state as running
		m.State = StateRunning

		// Get the step
		step := m.Plan.Workflow.Steps[stepIndex]

		// Substitute placeholders using placeholders package
		cmd, err := placeholders.Substitute(step.Command, m.Placeholders)
		if err != nil {
			// Check if we have any placeholders at all
			phNames := placeholders.CollectFromSteps(m.Plan.Workflow.Steps)
			if len(phNames) == 0 {
				// No placeholders in workflow, use original command
				cmd = step.Command
			} else {
				// We have placeholders but substitution failed
				result := runnerpkg.StepResult{
					Step:     stepIndex,
					Success:  false,
					ExitCode: 21,
					Output:   fmt.Sprintf("Placeholder substitution failed: %v", err),
					Duration: 0,
					Error:    err,
				}
				return RunnerMsg{Result: result}
			}
		}

		// Resolve working directory
		cwd := step.CWD
		if cwd == "" && m.Plan.Workflow.Defaults.CWD != "" {
			cwd = m.Plan.Workflow.Defaults.CWD
		}
		if !filepath.IsAbs(cwd) && m.Plan.RepoRoot != "" {
			cwd = filepath.Join(m.Plan.RepoRoot, cwd)
		}

		// Get shell
		shell := step.Shell
		if shell == "" {
			shell = "bash"
			// Check config for default shell if available
			if m.Config != nil && m.Config.Runner.DefaultShell != "" {
				shell = m.Config.Runner.DefaultShell
			}
		}

		// Execute step using runner.Exec
		execConfig := runnerpkg.ExecConfig{
			Command:       cmd,
			Shell:         shell,
			CWD:           cwd,
			Env:           step.Env,
			Stream:        m.StreamOutput,
			DangerChecker: m.DangerChecker,
			AutoConfirm:   m.AutoConfirm,
		}

		execResult := runnerpkg.Exec(context.Background(), execConfig)

		// Convert to StepResult
		result := runnerpkg.StepResult{
			Step:     stepIndex,
			Success:  execResult.Success,
			ExitCode: execResult.ExitCode,
			Output:   execResult.Output,
			Duration: execResult.Duration,
			Error:    execResult.Error,
		}

		return RunnerMsg{Result: result}
	}
}

// Batch combines multiple commands.
func (m RunnerModel) Batch(cmds ...tea.Cmd) tea.Cmd {
	return tea.Batch(cmds...)
}

// DidSucceed returns true if the workflow succeeded.
func (m RunnerModel) DidSucceed() bool {
	return m.Success
}

// DidCancel returns true if the user canceled.
func (m RunnerModel) DidCancel() bool {
	return m.Canceled
}

// runnerStepItem is a list item for a workflow step in the runner.
type runnerStepItem struct {
	index int
	name  string
}

func (s runnerStepItem) FilterValue() string {
	return s.name
}

// Title implements list.Item.
func (s runnerStepItem) Title() string {
	return s.name
}

// Description implements list.Item.
func (s runnerStepItem) Description() string {
	return ""
}

// runnerStepDelegate defines how steps are rendered in the runner list.
type runnerStepDelegate struct{}

func (d runnerStepDelegate) Height() int { return 1 }
func (d runnerStepDelegate) Spacing() int  { return 0 }
func (d runnerStepDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d runnerStepDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	s, ok := listItem.(runnerStepItem)
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

	_, _ = fmt.Fprint(w, text)
}

