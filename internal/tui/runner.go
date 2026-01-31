// Package tui provides Bubble Tea models for svf.
package tui

import (
	"context"
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/chazuruo/svf/internal/placeholders"
	"github.com/chazuruo/svf/internal/runner"
)

// RunnerModel is a Bubble Tea model for running workflows interactively.
type RunnerModel struct {
	// Plan is the execution plan.
	Plan runner.Plan

	// CurrentStep is the current step being executed.
	CurrentStep int

	// StepResults contains results for executed steps.
	StepResults []runner.StepResult

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

	// List is the step list component.
	List list.Model

	// Viewport is the output viewport.
	Viewport viewport.Model

	// Output contains the latest command output.
	Output strings.Builder

	// Finished indicates if the run is complete.
	Finished bool

	// Success indicates if the run succeeded.
	Success bool

	// Canceled indicates if the user canceled.
	Canceled bool

	// DangerChecker for dangerous command checking
	DangerChecker *runner.DangerChecker

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

	// width and height
	width  int
	height int
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
	Result runner.StepResult
}

// OutputMsg is sent when there's new output.
type OutputMsg string

// NewRunnerModel creates a new runner model.
func NewRunnerModel(plan runner.Plan, dangerChecker *runner.DangerChecker, autoConfirm bool, streamOutput bool) RunnerModel {
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

	return RunnerModel{
		Plan:            plan,
		CurrentStep:     0,
		StepResults:     make([]runner.StepResult, len(plan.Workflow.Steps)),
		Placeholders:    plan.Parameters,
		PlaceholderInfo: phInfo,
		State:           initialState,
		List:            l,
		Viewport:        vp,
		Finished:        false,
		normalStyle:     normalStyle,
		selectedStyle:   selectedStyle,
		successStyle:    successStyle,
		errorStyle:      errorStyle,
		runningStyle:    runningStyle,
		pendingStyle:    pendingStyle,
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

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.Canceled = true
			m.Finished = true
			m.State = StateFinished
			return m, tea.Quit

		case "enter":
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

		case "s":
			// Skip current step
			if m.State == StateReady || m.State == StateStepResult {
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

		case "r":
			// Re-run current step
			if m.State == StateStepResult {
				cmds = append(cmds, m.runStep(m.CurrentStep))
				return m, m.Batch(cmds...)
			}
		}

	case RunnerMsg:
		// Step finished
		m.StepResults[msg.Result.Step] = msg.Result
		m.Output.Reset()
		m.Output.WriteString(msg.Result.Output)
		m.State = StateStepResult

		if msg.Result.Success {
			m.CurrentStep++
			if m.CurrentStep < len(m.Plan.Workflow.Steps) {
				m.List.Select(m.CurrentStep)
			}
		} else {
			// Step failed
			m.Finished = true
			m.Success = false
			m.State = StateFinished
			return m, tea.Quit
		}

	case OutputMsg:
		// New output
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

// View implements tea.Model.
func (m RunnerModel) View() string {
	if m.Finished {
		return m.finishedView()
	}

	if m.State == StatePrompting {
		return m.promptingView()
	}

	// Layout: left panel (step list), right panel (output + help)
	leftPanel := m.stepListView()
	rightPanel := m.outputView()

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
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

	b.WriteString(" Output\n\n")
	b.WriteString(m.Viewport.View())
	b.WriteString("\n\n")
	b.WriteString(m.helpText())

	width := m.width - 40
	if width < 40 {
		width = 40
	}
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Render(b.String())
}

// helpText returns the help text.
func (m RunnerModel) helpText() string {
	var parts []string

	if m.State == StateStepResult {
		parts = append(parts, "[Enter] Next step", "[r] Rerun", "[s] Skip", "[q] Quit")
	} else {
		parts = append(parts, "[Enter] Run step", "[s] Skip", "[q] Quit")
	}

	return lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		strings.Join(parts, " • "),
	)
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
		b.WriteString("\n ✓ Workflow completed successfully!\n\n")
	} else if m.Canceled {
		b.WriteString("\n Workflow canceled.\n\n")
	} else {
		b.WriteString("\n ✗ Workflow failed.\n\n")
	}

	// Show summary
	for i, result := range m.StepResults {
		name := m.Plan.Workflow.Steps[i].Name
		if name == "" {
			name = fmt.Sprintf("Step %d", i+1)
		}

		status := "✓"
		if !result.Success {
			status = "✗"
		}
		b.WriteString(fmt.Sprintf("   %s %s\n", status, name))
	}

	b.WriteString("\n Press Enter to exit...\n")

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

		// Substitute placeholders
		cmd, err := runner.Substitute(step.Command, m.Placeholders)
		if err != nil && len(m.Placeholders) > 0 {
			// Substitution failed, use original
			cmd = step.Command
		}
		if cmd == "" && len(m.Placeholders) == 0 {
			// No substitution performed, use original
			cmd = step.Command
		}

		// Resolve working directory
		cwd := step.CWD
		if cwd == "" && m.Plan.Workflow.Defaults.CWD != "" {
			cwd = m.Plan.Workflow.Defaults.CWD
		}

		// Get shell
		shell := step.Shell
		if shell == "" {
			shell = "bash"
		}

		// Execute step using runner.Exec
		execConfig := runner.ExecConfig{
			Command:       cmd,
			Shell:         shell,
			CWD:           cwd,
			Env:           step.Env,
			Stream:        m.StreamOutput,
			DangerChecker: m.DangerChecker,
			AutoConfirm:   m.AutoConfirm,
		}

		execResult := runner.Exec(context.Background(), execConfig)

		// Convert to StepResult
		result := runner.StepResult{
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

