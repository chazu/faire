// Package explain provides tests for command explanation.
package explain

import (
	"context"
	"testing"

	"github.com/chazuruo/svf/internal/ai"
	"github.com/chazuruo/svf/internal/workflows"
)

func TestNewExplainer(t *testing.T) {
	tests := []struct {
		name    string
		opts    *Options
		wantNil bool
	}{
		{
			name:    "nil options",
			opts:    nil,
			wantNil: false,
		},
		{
			name: "empty options",
			opts: &Options{},
			wantNil: false,
		},
		{
			name: "offline mode",
			opts: &Options{Offline: true},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewExplainer(tt.opts)
			if (e == nil) != tt.wantNil {
				t.Errorf("NewExplainer() = %v, wantNil %v", e, tt.wantNil)
			}
		})
	}
}

func TestExplainer_ExplainCommand_Offline(t *testing.T) {
	e := NewExplainer(&Options{Offline: true})

	tests := []struct {
		name            string
		command         string
		wantCategory    string
		wantRisk        string
		wantExplanation bool
	}{
		{
			name:            "git status",
			command:         "git status",
			wantCategory:    "git",
			wantRisk:        "safe",
			wantExplanation: true,
		},
		{
			name:            "kubectl get pods",
			command:         "kubectl get pods",
			wantCategory:    "kubectl",
			wantRisk:        "safe",
			wantExplanation: true,
		},
		{
			name:            "docker build",
			command:         "docker build -t myapp .",
			wantCategory:    "docker",
			wantRisk:        "safe",
			wantExplanation: true,
		},
		{
			name:            "rm -rf dangerous",
			command:         "rm -rf /some/path",
			wantCategory:    "filesystem",
			wantRisk:        "high",
			wantExplanation: true,
		},
		{
			name:            "unknown command",
			command:         "xyzzy plugh",
			wantCategory:    "other",
			wantRisk:        "unknown",
			wantExplanation: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.ExplainCommand(context.Background(), tt.command)

			if result.Command != tt.command {
				t.Errorf("Command = %q, want %q", result.Command, tt.command)
			}
			if result.Category != tt.wantCategory {
				t.Errorf("Category = %q, want %q", result.Category, tt.wantCategory)
			}
			if result.Risk != tt.wantRisk {
				t.Errorf("Risk = %q, want %q", result.Risk, tt.wantRisk)
			}
			if (result.Explanation == "") == tt.wantExplanation {
				t.Errorf("Explanation empty = %v, wantExplanation %v", result.Explanation == "", tt.wantExplanation)
			}
		})
	}
}

func TestExplainer_ExplainWorkflowStep_Offline(t *testing.T) {
	e := NewExplainer(&Options{Offline: true})

	result := e.ExplainWorkflowStep(context.Background(), "Deploy app", "kubectl apply -f deployment.yaml", 0)

	if result.Command != "kubectl apply -f deployment.yaml" {
		t.Errorf("Command = %q, want %q", result.Command, "kubectl apply -f deployment.yaml")
	}
	if result.StepName != "Deploy app" {
		t.Errorf("StepName = %q, want %q", result.StepName, "Deploy app")
	}
	if result.StepIndex != 0 {
		t.Errorf("StepIndex = %d, want 0", result.StepIndex)
	}
	if result.Context != "Step 1: Deploy app" {
		t.Errorf("Context = %q, want %q", result.Context, "Step 1: Deploy app")
	}
	if result.Explanation == "" {
		t.Error("Explanation should not be empty")
	}
}

func TestExplainer_AddRule(t *testing.T) {
	e := NewExplainer(&Options{})

	// Add a custom rule
	err := e.AddRule(Rule{
		Pattern:     `(?i)\btestcmd\b`,
		Explanation: "Test command explanation",
		Risk:       "low",
		Category:   "test",
	})
	if err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}

	result := e.ExplainCommand(context.Background(), "testcmd")
	if result.Explanation != "Test command explanation" {
		t.Errorf("Explanation = %q, want %q", result.Explanation, "Test command explanation")
	}
	if result.Category != "test" {
		t.Errorf("Category = %q, want %q", result.Category, "test")
	}
}

func TestExplainer_AddRule_InvalidPattern(t *testing.T) {
	e := NewExplainer(&Options{})

	err := e.AddRule(Rule{
		Pattern:     `[invalid(`,
		Explanation: "Test",
		Risk:       "low",
		Category:   "test",
	})
	if err == nil {
		t.Error("AddRule() should return error for invalid pattern")
	}
}

func TestParseRiskLevel(t *testing.T) {
	tests := []struct {
		input string
		want  RiskLevel
	}{
		{"safe", RiskSafe},
		{"SAFE", RiskSafe},
		{"low", RiskLow},
		{"medium", RiskMedium},
		{"high", RiskHigh},
		{"unknown", RiskUnknown},
		{"", RiskUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseRiskLevel(tt.input); got != tt.want {
				t.Errorf("ParseRiskLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestRiskLevel_String(t *testing.T) {
	tests := []struct {
		risk RiskLevel
		want string
	}{
		{RiskSafe, "safe"},
		{RiskLow, "low"},
		{RiskMedium, "medium"},
		{RiskHigh, "high"},
		{RiskUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.risk.String(); got != tt.want {
				t.Errorf("RiskLevel.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRiskLevel_Icon(t *testing.T) {
	tests := []struct {
		risk RiskLevel
		want string
	}{
		{RiskSafe, "✓"},
		{RiskLow, "ⓘ"},
		{RiskMedium, "⚠"},
		{RiskHigh, "☠"},
		{RiskUnknown, "?"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.risk.Icon(); got != tt.want {
				t.Errorf("RiskLevel.Icon() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Mock provider for testing
type mockProvider struct {
	explainFunc func(context.Context, ai.ExplainRequest) (string, error)
}

func (m *mockProvider) Name() string {
	return "mock"
}

func (m *mockProvider) GenerateWorkflow(ctx context.Context, req ai.GenerateRequest) (*workflows.Workflow, error) {
	return nil, nil
}

func (m *mockProvider) Explain(ctx context.Context, input ai.ExplainRequest) (string, error) {
	if m.explainFunc != nil {
		return m.explainFunc(ctx, input)
	}
	return "Mock explanation", nil
}

func TestExplainer_ExplainCommand_WithProvider(t *testing.T) {
	mock := &mockProvider{
		explainFunc: func(ctx context.Context, req ai.ExplainRequest) (string, error) {
			return "AI explanation for: " + req.Command, nil
		},
	}

	e := NewExplainer(&Options{Provider: mock})

	result := e.ExplainCommand(context.Background(), "test command")

	if result.Explanation != "AI explanation for: test command" {
		t.Errorf("Explanation = %q, want %q", result.Explanation, "AI explanation for: test command")
	}
	if result.Category != "ai-generated" {
		t.Errorf("Category = %q, want %q", result.Category, "ai-generated")
	}
}

func TestExplainer_ExplainWorkflow_WithProvider(t *testing.T) {
	// Test with nil provider (offline mode with no AI)
	e := NewExplainer(&Options{Offline: true})

	// ExplainWorkflow requires AI provider, so it should fail
	_, err := e.ExplainWorkflow(context.Background(), nil, ai.DetailNormal)
	if err == nil {
		t.Error("ExplainWorkflow should return error when provider is nil")
	}
}
