package gitops

import (
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/report"
)

func TestBuildPRBody(t *testing.T) {
	inc, _ := incident.New("default", "production", "api-server-xyz123", incident.TypeOOMKilled, incident.SeverityCritical, "Container 'api' was OOMKilled")
	inc.ID = "abc12345-6789-0123-4567-890abcdef012"

	rpt, _ := report.New(
		inc.ID,
		"Memory limit exceeded due to memory leak in handler",
		[]string{"OOMKilled event at 12:00", "Memory usage at 98%"},
		0.85,
		report.RiskHigh,
		"Increase memory limit to 512Mi",
		"Revert to previous deployment",
	)

	body := buildPRBody(inc, rpt)

	if body == "" {
		t.Fatal("expected non-empty PR body")
	}

	checks := []string{
		"Auto-Remediation PR",
		"Incident Details",
		"Root Cause Analysis",
		"Memory limit exceeded",
		"Rollback Plan",
		"K8S Self-Healing Agent",
	}

	for _, check := range checks {
		if !contains(body, check) {
			t.Errorf("expected PR body to contain '%s'", check)
		}
	}
}

func TestFormatEvidence(t *testing.T) {
	evidence := []string{"OOMKilled event", "Memory at 98%"}
	result := formatEvidence(evidence)

	if !contains(result, "- OOMKilled event") {
		t.Error("expected formatted evidence to contain first item")
	}
	if !contains(result, "- Memory at 98%") {
		t.Error("expected formatted evidence to contain second item")
	}
}

func TestFormatEvidence_Empty(t *testing.T) {
	result := formatEvidence(nil)
	if result != "" {
		t.Errorf("expected empty string for nil evidence, got '%s'", result)
	}
}

func TestTruncateStr(t *testing.T) {
	if truncateStr("short", 10) != "short" {
		t.Error("short string should not be truncated")
	}

	result := truncateStr("this is a very long string that needs truncation", 20)
	if len(result) > 24 {
		t.Errorf("truncated string too long: %s", result)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
