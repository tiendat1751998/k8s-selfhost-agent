package rca

import (
	"testing"

	"github.com/datdt/k8sselfhost/internal/pkg/stringutil"
)


func TestRenderPrompt_Basic(t *testing.T) {
	data := PromptData{
		IncidentType: "CrashLoopBackOff",
		Namespace:    "production",
		PodName:      "api-server-xyz123",
		Severity:     "high",
		Message:      "Container 'api' is in CrashLoopBackOff (restarts: 15)",
		PodDescribe:  "Name: api-server-xyz123\nNamespace: production\nPhase: Running",
		PodLogs: map[string]string{
			"api": "panic: runtime error: nil pointer dereference\ngoroutine 1 [running]:\nmain.main()\n\t/app/main.go:42",
		},
		Events: []EventData{
			{Type: "Warning", Reason: "BackOff", Message: "Back-off restarting failed container", Count: 15, LastSeen: "2024-01-01T12:00:00Z"},
		},
	}

	prompt, err := RenderPrompt(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}

	// Verify key data is present in the prompt
	checks := []string{
		"CrashLoopBackOff",
		"production",
		"api-server-xyz123",
		"nil pointer dereference",
		"BackOff",
	}
	for _, check := range checks {
		if !containsStr(prompt, check) {
			t.Errorf("expected prompt to contain '%s'", check)
		}
	}
}

func TestRenderPrompt_WithDeployment(t *testing.T) {
	data := PromptData{
		IncidentType:   "OOMKilled",
		Namespace:      "default",
		PodName:        "worker-abc",
		Severity:       "critical",
		Message:        "Container 'worker' was OOMKilled",
		DeploymentYAML: "Name: worker\nReplicas: 3/3\n",
	}

	prompt, err := RenderPrompt(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsStr(prompt, "Deployment Configuration") {
		t.Error("expected prompt to contain deployment section")
	}
}

func TestRenderPrompt_WithNodeMetrics(t *testing.T) {
	data := PromptData{
		IncidentType: "FailedScheduling",
		Namespace:    "default",
		PodName:      "worker-abc",
		Severity:     "high",
		Message:      "Pod cannot be scheduled",
		NodeMetrics: &NodeMetricsData{
			NodeName:    "node-1",
			CPUUsage:    "4",
			MemoryUsage: "16Gi",
			PodCount:    110,
			Allocatable: "CPU: 4, Memory: 16Gi, Pods: 110",
		},
	}

	prompt, err := RenderPrompt(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsStr(prompt, "node-1") {
		t.Error("expected prompt to contain node name")
	}
}

func TestSystemPrompt_NotEmpty(t *testing.T) {
	sp := SystemPrompt()
	if sp == "" {
		t.Fatal("expected non-empty system prompt")
	}
	if !containsStr(sp, "Root Cause Analysis") {
		t.Error("expected system prompt to mention RCA")
	}
	if !containsStr(sp, "JSON") {
		t.Error("expected system prompt to mention JSON output format")
	}
}

func TestExtractJSON_CleanJSON(t *testing.T) {
	input := `{"root_cause": "OOM", "confidence": 0.9}`
	result := extractJSON(input)
	if result != input {
		t.Errorf("expected same JSON, got '%s'", result)
	}
}

func TestExtractJSON_WrappedInMarkdown(t *testing.T) {
	input := "Here is the analysis:\n```json\n{\"root_cause\": \"OOM\", \"confidence\": 0.9}\n```"
	result := extractJSON(input)
	if result != `{"root_cause": "OOM", "confidence": 0.9}` {
		t.Errorf("expected extracted JSON, got '%s'", result)
	}
}

func TestExtractJSON_NoJSON(t *testing.T) {
	input := "No JSON here"
	result := extractJSON(input)
	if result != input {
		t.Errorf("expected original string, got '%s'", result)
	}
}

func TestMapRiskLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"critical", "critical"},
		{"high", "high"},
		{"medium", "medium"},
		{"low", "low"},
		{"unknown", "medium"},
		{"", "medium"},
	}

	for _, tc := range tests {
		result := mapRiskLevel(tc.input)
		if string(result) != tc.expected {
			t.Errorf("mapRiskLevel(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestTruncate(t *testing.T) {
	if stringutil.Truncate("short", 10) != "short" {
		t.Error("short string should not be truncated")
	}
	result := stringutil.Truncate("this is a long string", 10)
	if result != "this is a ..." {
		t.Errorf("expected truncated string, got '%s'", result)
	}
}


func TestParseRCAResponse_Valid(t *testing.T) {
	input := `{
		"root_cause": "Memory limit exceeded",
		"evidence": ["OOMKilled event", "Memory at 98%"],
		"confidence": 0.85,
		"risk_level": "high",
		"remediation": "Increase memory limit",
		"rollback_plan": "Revert deployment"
	}`

	resp, err := parseRCAResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.RootCause != "Memory limit exceeded" {
		t.Errorf("unexpected root_cause: %s", resp.RootCause)
	}
	if resp.Confidence != 0.85 {
		t.Errorf("unexpected confidence: %f", resp.Confidence)
	}
	if len(resp.Evidence) != 2 {
		t.Errorf("expected 2 evidence items, got %d", len(resp.Evidence))
	}
}

func TestParseRCAResponse_MissingRootCause(t *testing.T) {
	input := `{"confidence": 0.5, "remediation": "fix"}`
	_, err := parseRCAResponse(input)
	if err == nil {
		t.Fatal("expected error for missing root_cause")
	}
}

func TestParseRCAResponse_InvalidJSON(t *testing.T) {
	_, err := parseRCAResponse("not json at all")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseRCAResponse_ConfidenceOutOfRange(t *testing.T) {
	input := `{"root_cause": "test", "confidence": 5.0, "remediation": "fix"}`
	resp, err := parseRCAResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Confidence != 0.5 {
		t.Errorf("expected confidence to be clamped to 0.5, got %f", resp.Confidence)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
