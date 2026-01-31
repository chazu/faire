package history

import (
	"testing"
	"time"
)

func TestFilterLines(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	lines := []HistoryLine{
		{Timestamp: baseTime.Add(-1 * time.Hour), Command: "old command"},
		{Timestamp: baseTime, Command: "kubectl get pods"},
		{Timestamp: baseTime.Add(1 * time.Minute), Command: "ls"},
		{Timestamp: baseTime.Add(2 * time.Minute), Command: "kubectl get pods"},
		{Timestamp: baseTime.Add(3 * time.Minute), Command: "git status"},
	}

	t.Run("filters by max lines", func(t *testing.T) {
		result := FilterLines(lines, FilterOptions{MaxLines: 2})
		if len(result) != 2 {
			t.Errorf("expected 2 lines, got %d", len(result))
		}
		// Should get the most recent non-ignored commands
		if result[1].Command != "git status" {
			t.Errorf("expected 'git status' as last command, got '%s'", result[1].Command)
		}
	})

	t.Run("filters by time", func(t *testing.T) {
		since := baseTime.Add(1 * time.Minute)
		result := FilterLines(lines, FilterOptions{Since: since})
		// Should get git status (12:03) and the second kubectl get pods (12:02), excluding ls and earlier
		// Actually ls is filtered as unwanted, so we get kubectl get pods and git status
		if len(result) != 2 {
			t.Errorf("expected 2 lines after time filter, got %d: %v", len(result), result)
		}
	})

	t.Run("removes duplicates", func(t *testing.T) {
		result := FilterLines(lines, FilterOptions{RemoveDup: true})
		// Count unique "kubectl get pods"
		count := 0
		for _, line := range result {
			if line.Command == "kubectl get pods" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected 1 occurrence of 'kubectl get pods', got %d", count)
		}
	})

	t.Run("filters unwanted commands", func(t *testing.T) {
		linesWithUnwanted := []HistoryLine{
			{Command: "ls"},
			{Command: "cd /tmp"},
			{Command: "kubectl get pods"},
			{Command: "  space prefixed"},
			{Command: ""},
		}
		result := FilterLines(linesWithUnwanted, FilterOptions{})
		if len(result) != 1 {
			t.Errorf("expected 1 line after filtering unwanted commands, got %d", len(result))
		}
		if result[0].Command != "kubectl get pods" {
			t.Errorf("expected 'kubectl get pods', got '%s'", result[0].Command)
		}
	})
}

func TestIsUnwantedCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		unwanted bool
	}{
		{"ls command", "ls", true},
		{"ll command", "ll", true},
		{"cd command", "cd /tmp", true},
		{"pwd command", "pwd", true},
		{"empty command", "", true},
		{"space prefixed", "  kubectl get pods", true},
		{"kubectl command", "kubectl get pods", false},
		{"git command", "git status", false},
		{"docker command", "docker ps", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isUnwantedCommand(tt.command)
			if result != tt.unwanted {
				t.Errorf("isUnwantedCommand(%q) = %v, want %v", tt.command, result, tt.unwanted)
			}
		})
	}
}

func TestRemoveConsecutiveDuplicates(t *testing.T) {
	lines := []HistoryLine{
		{Command: "kubectl get pods"},
		{Command: "kubectl get pods"},
		{Command: "git status"},
		{Command: "git status"},
		{Command: "kubectl get pods"}, // Not consecutive duplicate
	}

	result := RemoveConsecutiveDuplicates(lines)
	if len(result) != 3 {
		t.Errorf("expected 3 lines, got %d", len(result))
	}
}

func TestFilterByLimit(t *testing.T) {
	lines := []HistoryLine{
		{Command: "command1"},
		{Command: "command2"},
		{Command: "command3"},
		{Command: "command4"},
		{Command: "command5"},
	}

	result := FilterByLimit(lines, 3)
	if len(result) != 3 {
		t.Errorf("expected 3 lines, got %d", len(result))
	}
	// Should get the last 3
	if result[0].Command != "command3" {
		t.Errorf("expected first command to be 'command3', got '%s'", result[0].Command)
	}
}
