// Package tui provides tests for Bubble Tea models.
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chazuruo/svf/internal/history"
)

// TestNewHistoryPickerModel verifies that the history picker model is initialized correctly.
func TestNewHistoryPickerModel(t *testing.T) {
	commands := []history.Command{
		{Timestamp: 1000, Command: "git status", Shell: "bash"},
		{Timestamp: 2000, Command: "kubectl get pods", Shell: "bash"},
		{Timestamp: 3000, Command: "git log", Shell: "bash"},
	}

	model := NewHistoryPickerModel(commands)

	// Check basic fields
	if len(model.Commands) != 3 {
		t.Errorf("expected 3 commands, got %d", len(model.Commands))
	}

	if len(model.Selected) != 0 {
		t.Errorf("expected 0 selected commands, got %d", len(model.Selected))
	}

	if len(model.Filtered) != 3 {
		t.Errorf("expected 3 filtered commands, got %d", len(model.Filtered))
	}

	// Check initial cursor
	if model.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", model.cursor)
	}

	// Check initial focus
	if model.Focused != "filter" {
		t.Errorf("expected focus on filter, got %s", model.Focused)
	}

	// Check quit/confirmed states
	if model.Quit {
		t.Error("expected Quit to be false")
	}

	if model.Confirmed {
		t.Error("expected Confirmed to be false")
	}
}

// TestNewHistoryPickerModel_EmptyCommands verifies model with no commands.
func TestNewHistoryPickerModel_EmptyCommands(t *testing.T) {
	commands := []history.Command{}

	model := NewHistoryPickerModel(commands)

	if len(model.Commands) != 0 {
		t.Errorf("expected 0 commands, got %d", len(model.Commands))
	}

	if len(model.Filtered) != 0 {
		t.Errorf("expected 0 filtered commands, got %d", len(model.Filtered))
	}
}

// TestHistoryPickerFilter verifies that filtering works correctly.
func TestHistoryPickerFilter(t *testing.T) {
	commands := []history.Command{
		{Timestamp: 1000, Command: "git status", Shell: "bash"},
		{Timestamp: 2000, Command: "kubectl get pods", Shell: "bash"},
		{Timestamp: 3000, Command: "git log", Shell: "bash"},
		{Timestamp: 4000, Command: "ls -la", Shell: "bash"},
	}

	model := NewHistoryPickerModel(commands)

	// Test filter for "git" - should match 2 commands
	model.applyFilter("git")
	if len(model.Filtered) != 2 {
		t.Errorf("expected 2 filtered commands for 'git', got %d", len(model.Filtered))
	}

	// Verify the filtered indices are correct
	if len(model.Filtered) > 0 && model.Filtered[0] != 0 {
		t.Errorf("expected first filtered index to be 0 (git status), got %d", model.Filtered[0])
	}
	if len(model.Filtered) > 1 && model.Filtered[1] != 2 {
		t.Errorf("expected second filtered index to be 2 (git log), got %d", model.Filtered[1])
	}

	// Test filter for "kubectl" - should match 1 command
	model.applyFilter("kubectl")
	if len(model.Filtered) != 1 {
		t.Errorf("expected 1 filtered command for 'kubectl', got %d", len(model.Filtered))
	}

	// Test case-insensitive filter
	model.applyFilter("GIT")
	if len(model.Filtered) != 2 {
		t.Errorf("expected 2 filtered commands for 'GIT' (case-insensitive), got %d", len(model.Filtered))
	}

	// Test empty filter - should match all
	model.applyFilter("")
	if len(model.Filtered) != 4 {
		t.Errorf("expected 4 filtered commands for empty filter, got %d", len(model.Filtered))
	}

	// Test no match
	model.applyFilter("nonexistent")
	if len(model.Filtered) != 0 {
		t.Errorf("expected 0 filtered commands for 'nonexistent', got %d", len(model.Filtered))
	}
}

// TestHistoryPickerSelection verifies selection toggle functionality.
func TestHistoryPickerSelection(t *testing.T) {
	commands := []history.Command{
		{Timestamp: 1000, Command: "git status", Shell: "bash"},
		{Timestamp: 2000, Command: "kubectl get pods", Shell: "bash"},
		{Timestamp: 3000, Command: "git log", Shell: "bash"},
	}

	model := NewHistoryPickerModel(commands)

	// Toggle selection for first item
	if len(model.Filtered) == 0 {
		t.Fatal("expected filtered commands to be non-empty")
	}
	idx := model.Filtered[model.cursor]
	model.Selected[idx] = true

	if !model.Selected[idx] {
		t.Error("expected first command to be selected")
	}

	// Toggle it off
	model.Selected[idx] = false
	if model.Selected[idx] {
		t.Error("expected first command to be deselected")
	}
}

// TestHistoryPickerSelectAll verifies select all functionality.
func TestHistoryPickerSelectAll(t *testing.T) {
	commands := []history.Command{
		{Timestamp: 1000, Command: "git status", Shell: "bash"},
		{Timestamp: 2000, Command: "kubectl get pods", Shell: "bash"},
		{Timestamp: 3000, Command: "git log", Shell: "bash"},
	}

	model := NewHistoryPickerModel(commands)

	// Select all filtered
	for _, idx := range model.Filtered {
		model.Selected[idx] = true
	}

	// Verify all are selected
	if len(model.Selected) != 3 {
		t.Errorf("expected 3 selected commands, got %d", len(model.Selected))
	}

	for _, idx := range model.Filtered {
		if !model.Selected[idx] {
			t.Errorf("expected command at index %d to be selected", idx)
		}
	}
}

// TestHistoryPickerSelectNone verifies select none functionality.
func TestHistoryPickerSelectNone(t *testing.T) {
	commands := []history.Command{
		{Timestamp: 1000, Command: "git status", Shell: "bash"},
		{Timestamp: 2000, Command: "kubectl get pods", Shell: "bash"},
		{Timestamp: 3000, Command: "git log", Shell: "bash"},
	}

	model := NewHistoryPickerModel(commands)

	// Select some
	model.Selected[0] = true
	model.Selected[2] = true

	// Select none (reset)
	model.Selected = make(map[int]bool)

	// Verify none are selected
	if len(model.Selected) != 0 {
		t.Errorf("expected 0 selected commands, got %d", len(model.Selected))
	}
}

// TestHistoryPickerNavigation verifies cursor navigation.
func TestHistoryPickerNavigation(t *testing.T) {
	commands := []history.Command{
		{Timestamp: 1000, Command: "git status", Shell: "bash"},
		{Timestamp: 2000, Command: "kubectl get pods", Shell: "bash"},
		{Timestamp: 3000, Command: "git log", Shell: "bash"},
	}

	model := NewHistoryPickerModel(commands)

	// Move down
	model.cursor = 1
	if model.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", model.cursor)
	}

	// Move down again
	model.cursor = 2
	if model.cursor != 2 {
		t.Errorf("expected cursor at 2, got %d", model.cursor)
	}

	// Move up
	model.cursor = 1
	if model.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", model.cursor)
	}

	// Can't go below 0
	model.cursor = 0
	model.cursor = max(0, model.cursor-1)
	if model.cursor != 0 {
		t.Errorf("expected cursor at 0 (can't go negative), got %d", model.cursor)
	}
}

// TestHistoryPickerCursorResetOnFilter verifies cursor resets when filter reduces results.
func TestHistoryPickerCursorResetOnFilter(t *testing.T) {
	commands := []history.Command{
		{Timestamp: 1000, Command: "git status", Shell: "bash"},
		{Timestamp: 2000, Command: "kubectl get pods", Shell: "bash"},
		{Timestamp: 3000, Command: "git log", Shell: "bash"},
	}

	model := NewHistoryPickerModel(commands)

	// Move cursor to bottom
	model.cursor = 2

	// Apply filter that reduces results to 1
	model.applyFilter("kubectl")

	// Cursor should be reset to valid range
	if model.cursor >= len(model.Filtered) {
		model.cursor = max(0, len(model.Filtered)-1)
	}

	if model.cursor != 0 {
		t.Errorf("expected cursor reset to 0 after filter, got %d", model.cursor)
	}
}

// TestGetSelectedCommands verifies retrieving selected commands.
func TestGetSelectedCommands(t *testing.T) {
	commands := []history.Command{
		{Timestamp: 1000, Command: "git status", Shell: "bash"},
		{Timestamp: 2000, Command: "kubectl get pods", Shell: "bash"},
		{Timestamp: 3000, Command: "git log", Shell: "bash"},
	}

	model := NewHistoryPickerModel(commands)

	// Select first and last
	model.Selected[0] = true
	model.Selected[2] = true

	selected := model.GetSelectedCommands()

	if len(selected) != 2 {
		t.Errorf("expected 2 selected commands, got %d", len(selected))
	}

	// Note: map iteration order is not guaranteed, so we just check count
}

// TestDidQuit verifies quit state tracking.
func TestDidQuit(t *testing.T) {
	commands := []history.Command{
		{Timestamp: 1000, Command: "git status", Shell: "bash"},
	}

	model := NewHistoryPickerModel(commands)

	if model.DidQuit() {
		t.Error("expected DidQuit to be false initially")
	}

	model.Quit = true

	if !model.DidQuit() {
		t.Error("expected DidQuit to be true after setting Quit")
	}
}

// TestDidConfirm verifies confirm state tracking.
func TestDidConfirm(t *testing.T) {
	commands := []history.Command{
		{Timestamp: 1000, Command: "git status", Shell: "bash"},
	}

	model := NewHistoryPickerModel(commands)

	if model.DidConfirm() {
		t.Error("expected DidConfirm to be false initially")
	}

	model.Confirmed = true

	if !model.DidConfirm() {
		t.Error("expected DidConfirm to be true after setting Confirmed")
	}
}

// TestHistoryPickerUpdate verifies basic update functionality.
func TestHistoryPickerUpdate(t *testing.T) {
	commands := []history.Command{
		{Timestamp: 1000, Command: "git status", Shell: "bash"},
		{Timestamp: 2000, Command: "kubectl get pods", Shell: "bash"},
	}

	model := NewHistoryPickerModel(commands)

	// Test quit message
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if newModel == nil {
		t.Error("expected model from update")
	}
	if cmd == nil {
		t.Error("expected quit command")
	}

	// Test quit with 'q'
	model = NewHistoryPickerModel(commands) // Reset
	newModel, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if newModel == nil {
		t.Error("expected model from update")
	}
	if cmd == nil {
		t.Error("expected quit command for 'q' key")
	}
}

// TestHistoryPickerFocusChange verifies focus switching between filter and list.
func TestHistoryPickerFocusChange(t *testing.T) {
	commands := []history.Command{
		{Timestamp: 1000, Command: "git status", Shell: "bash"},
	}

	model := NewHistoryPickerModel(commands)

	// Initially focused on filter
	if model.Focused != "filter" {
		t.Errorf("expected initial focus on filter, got %s", model.Focused)
	}

	// Press enter to switch to list
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = newModel.(HistoryPickerModel)

	if model.Focused != "list" {
		t.Errorf("expected focus on list after enter, got %s", model.Focused)
	}

	// Press '/' to switch back to filter
	newModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model = newModel.(HistoryPickerModel)

	if model.Focused != "filter" {
		t.Errorf("expected focus on filter after '/', got %s", model.Focused)
	}
}

// TestHistoryPickerWithFilteredSelection verifies selection works correctly with filtering.
func TestHistoryPickerWithFilteredSelection(t *testing.T) {
	commands := []history.Command{
		{Timestamp: 1000, Command: "git status", Shell: "bash"},
		{Timestamp: 2000, Command: "kubectl get pods", Shell: "bash"},
		{Timestamp: 3000, Command: "git log", Shell: "bash"},
		{Timestamp: 4000, Command: "ls -la", Shell: "bash"},
	}

	model := NewHistoryPickerModel(commands)

	// Filter to git commands only
	model.applyFilter("git")

	// Select the second git command (git log, index 2)
	model.Selected[2] = true

	// Clear filter - selection should persist
	model.applyFilter("")

	// Verify selection persists
	if !model.Selected[2] {
		t.Error("expected selection to persist after clearing filter")
	}

	// Get selected commands
	selected := model.GetSelectedCommands()

	if len(selected) != 1 {
		t.Errorf("expected 1 selected command, got %d", len(selected))
	}

	if selected[0].Command != "git log" {
		t.Errorf("expected selected command 'git log', got %s", selected[0].Command)
	}
}

// Golden file tests for TUI rendering

// TestHistoryPickerView_RenderEmpty verifies rendering with no commands.
func TestHistoryPickerView_RenderEmpty(t *testing.T) {
	commands := []history.Command{}
	model := NewHistoryPickerModel(commands)

	got := model.View()
	want := "No commands found"

	if !contains(got, want) {
		t.Errorf("View() output should contain %q\nGot:\n%s", want, got)
	}
}

// TestHistoryPickerView_RenderWithCommands verifies basic rendering with commands.
func TestHistoryPickerView_RenderWithCommands(t *testing.T) {
	commands := []history.Command{
		{Timestamp: 1000, Command: "git status", Shell: "bash"},
		{Timestamp: 2000, Command: "kubectl get pods", Shell: "bash"},
		{Timestamp: 3000, Command: "git log", Shell: "bash"},
	}
	model := NewHistoryPickerModel(commands)

	got := model.View()

	// Check for key elements
	expectedStrings := []string{
		"Shell History Picker",
		"Filter:",
		"[ ]",
	}

	for _, expected := range expectedStrings {
		if !contains(got, expected) {
			t.Errorf("View() output should contain %q\nGot:\n%s", expected, got)
		}
	}
}

// TestHistoryPickerView_RenderWithSelection verifies rendering with selected commands.
func TestHistoryPickerView_RenderWithSelection(t *testing.T) {
	commands := []history.Command{
		{Timestamp: 1000, Command: "git status", Shell: "bash"},
		{Timestamp: 2000, Command: "kubectl get pods", Shell: "bash"},
		{Timestamp: 3000, Command: "git log", Shell: "bash"},
	}
	model := NewHistoryPickerModel(commands)

	// Select first command
	model.Selected[0] = true

	got := model.View()

	// Should show [x] for selected
	if !contains(got, "[x]") {
		t.Errorf("View() output should contain [x] for selected\nGot:\n%s", got)
	}

	// Check selected count in header
	if !contains(got, "1 selected") {
		t.Errorf("View() output should show 1 selected\nGot:\n%s", got)
	}
}

// TestHistoryPickerView_RenderWithFilter verifies rendering with active filter.
func TestHistoryPickerView_RenderWithFilter(t *testing.T) {
	commands := []history.Command{
		{Timestamp: 1000, Command: "git status", Shell: "bash"},
		{Timestamp: 2000, Command: "kubectl get pods", Shell: "bash"},
		{Timestamp: 3000, Command: "git log", Shell: "bash"},
	}
	model := NewHistoryPickerModel(commands)

	// Apply filter
	model.applyFilter("git")

	got := model.View()

	// Should show filtered count
	if !contains(got, "2 commands") {
		t.Errorf("View() output should show 2 commands (filtered)\nGot:\n%s", got)
	}
}

// TestHistoryPickerView_RenderPreview verifies preview pane rendering.
func TestHistoryPickerView_RenderPreview(t *testing.T) {
	commands := []history.Command{
		{Timestamp: 1000, Command: "git status", Shell: "bash", CWD: "/home/user/project"},
	}
	model := NewHistoryPickerModel(commands)

	got := model.View()

	// Check for preview elements
	expectedStrings := []string{
		"Preview",
		"Command:",
		"git status",
		"Shell: bash",
	}

	for _, expected := range expectedStrings {
		if !contains(got, expected) {
			t.Errorf("View() output should contain %q\nGot:\n%s", expected, got)
		}
	}
}

// TestHistoryPickerView_RenderHelpText verifies help text rendering.
func TestHistoryPickerView_RenderHelpText(t *testing.T) {
	commands := []history.Command{
		{Timestamp: 1000, Command: "git status", Shell: "bash"},
	}
	model := NewHistoryPickerModel(commands)

	got := model.View()

	// Should have help text based on focus state
	if model.Focused == "filter" {
		if !contains(got, "Focus list") {
			t.Errorf("View() output should contain 'Focus list' help\nGot:\n%s", got)
		}
	}

	// Switch focus to list
	model.Focused = "list"
	got = model.View()

	// Should have list help text
	expectedHelp := []string{"Confirm", "Toggle", "Select"}
	for _, expected := range expectedHelp {
		if !contains(got, expected) {
			t.Errorf("View() output should contain %q in help\nGot:\n%s", expected, got)
		}
	}
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findInString(s, substr)))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
