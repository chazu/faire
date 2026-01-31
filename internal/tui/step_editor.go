// Package tui provides Bubble Tea models for terminal UI interactions.
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chazuruo/svf/internal/workflows"
)

// StepEditorModel is the model for editing a single workflow step.
type StepEditorModel struct {
	Step          workflows.Step
	StepIndex     int
	name          textinput.Model
	command       textarea.Model
	shell         textinput.Model
	cwd           textinput.Model
	continueOnErr bool
	Done          bool
	Cancelled     bool
}

// NewStepEditor creates a new step editor model.
func NewStepEditor(step workflows.Step, index int) *StepEditorModel {
	// Name input
	name := textinput.New()
	name.Placeholder = "Step name"
	name.SetValue(step.Name)
	name.Focus()

	// Command textarea
	cmd := textarea.New()
	cmd.Placeholder = "Command to execute"
	cmd.SetValue(step.Command)
	cmd.SetHeight(5)

	// Shell input
	shell := textinput.New()
	shell.Placeholder = "Shell (e.g., bash, zsh, sh)"
	shell.SetValue(step.Shell)

	// CWD input
	cwd := textinput.New()
	cwd.Placeholder = "Working directory (optional)"
	cwd.SetValue(step.CWD)

	return &StepEditorModel{
		Step:          step,
		StepIndex:     index,
		name:          name,
		command:       cmd,
		shell:         shell,
		cwd:           cwd,
		continueOnErr: step.ContinueOnError,
		Done:          false,
		Cancelled:     false,
	}
}

// Init initializes the step editor.
func (m *StepEditorModel) Init() tea.Cmd {
	return textarea.Blink
}

// Update updates the step editor model.
func (m *StepEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.Cancelled = true
			return m, tea.Quit

		case tea.KeyCtrlS:
			// Save and return
			m.Step.Name = m.name.Value()
			m.Step.Command = m.command.Value()
			m.Step.Shell = m.shell.Value()
			m.Step.CWD = m.cwd.Value()
			m.Step.ContinueOnError = m.continueOnErr
			m.Done = true
			return m, tea.Quit

		case tea.KeyCtrlT:
			// Toggle continue on error
			m.continueOnErr = !m.continueOnErr
			return m, nil

		case tea.KeyTab:
			// Cycle through fields
			if m.name.Focused() {
				m.name.Blur()
				m.command.Focus()
			} else if m.command.Focused() {
				m.command.Blur()
				m.shell.Focus()
			} else if m.shell.Focused() {
				m.shell.Blur()
				m.cwd.Focus()
			} else if m.cwd.Focused() {
				m.cwd.Blur()
				m.name.Focus()
			}
			return m, nil
		}
	}

	// Update inputs
	var cmd tea.Cmd
	if m.name.Focused() {
		m.name, cmd = m.name.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.command.Focused() {
		m.command, cmd = m.command.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.shell.Focused() {
		m.shell, cmd = m.shell.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.cwd.Focused() {
		m.cwd, cmd = m.cwd.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the step editor.
func (m *StepEditorModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Width(12)

	highlightStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).
		Bold(true)

	title := titleStyle.Render(fmt.Sprintf("Edit Step %d", m.StepIndex+1))

	// Field labels
	nameLabel := labelStyle.Render("Name:")
	cmdLabel := labelStyle.Render("Command:")
	shellLabel := labelStyle.Render("Shell:")
	cwdLabel := labelStyle.Render("Directory:")

	// Continue on error status
	continueStatus := "No"
	if m.continueOnErr {
		continueStatus = highlightStyle.Render("Yes")
	}

	// Input fields
	nameInput := m.name.View()
	cmdInput := m.command.View()
	shellInput := m.shell.View()
	cwdInput := m.cwd.View()

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)

	footer := footerStyle.Render(
		" [Ctrl+S]: save [Esc]: cancel [Tab]: next field [Ctrl+T]: toggle continue on error",
	)

	return title + "\n\n" +
		nameLabel + " " + nameInput + "\n\n" +
		cmdLabel + "\n" + cmdInput + "\n\n" +
		shellLabel + " " + shellInput + "\n" +
		cwdLabel + " " + cwdInput + "\n\n" +
		labelStyle.Render("Continue on error:") + " " + continueStatus + "\n\n" +
		footer
}

// GetStep returns the edited step.
func (m *StepEditorModel) GetStep() workflows.Step {
	return workflows.Step{
		Name:            m.name.Value(),
		Command:         m.command.Value(),
		Shell:           m.shell.Value(),
		CWD:             m.cwd.Value(),
		ContinueOnError: m.continueOnErr,
	}
}
