package report

import "testing"

func TestNew_ValidReport(t *testing.T) {
	r, err := New(
		"incident-123",
		"Memory limit exceeded due to memory leak in handler",
		[]string{"OOMKilled event at 12:00", "Memory usage 98%"},
		0.85,
		RiskHigh,
		"Increase memory limit to 512Mi",
		"Revert to previous deployment",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.IncidentID != "incident-123" {
		t.Errorf("expected incident_id 'incident-123', got '%s'", r.IncidentID)
	}
	if r.Confidence != 0.85 {
		t.Errorf("expected confidence 0.85, got %f", r.Confidence)
	}
	if r.RiskLevel != RiskHigh {
		t.Errorf("expected risk 'high', got '%s'", r.RiskLevel)
	}
	if len(r.Evidence) != 2 {
		t.Errorf("expected 2 evidence items, got %d", len(r.Evidence))
	}
}

func TestNew_EmptyIncidentID(t *testing.T) {
	_, err := New("", "root cause", nil, 0.5, RiskLow, "fix it", "rollback")
	if err == nil {
		t.Fatal("expected validation error for empty incident_id")
	}
}

func TestNew_EmptyRootCause(t *testing.T) {
	_, err := New("inc-1", "", nil, 0.5, RiskLow, "fix it", "rollback")
	if err == nil {
		t.Fatal("expected validation error for empty root_cause")
	}
}

func TestNew_InvalidConfidence(t *testing.T) {
	_, err := New("inc-1", "cause", nil, 1.5, RiskLow, "fix it", "rollback")
	if err == nil {
		t.Fatal("expected validation error for confidence > 1")
	}

	_, err = New("inc-1", "cause", nil, -0.1, RiskLow, "fix it", "rollback")
	if err == nil {
		t.Fatal("expected validation error for negative confidence")
	}
}

func TestNew_EmptyRemediation(t *testing.T) {
	_, err := New("inc-1", "cause", nil, 0.5, RiskLow, "", "rollback")
	if err == nil {
		t.Fatal("expected validation error for empty remediation")
	}
}

func TestReport_IsHighConfidence(t *testing.T) {
	r, _ := New("inc-1", "cause", nil, 0.85, RiskHigh, "fix", "rollback")

	if !r.IsHighConfidence(0.8) {
		t.Error("expected 0.85 to be high confidence with threshold 0.8")
	}
	if r.IsHighConfidence(0.9) {
		t.Error("expected 0.85 to NOT be high confidence with threshold 0.9")
	}
}
