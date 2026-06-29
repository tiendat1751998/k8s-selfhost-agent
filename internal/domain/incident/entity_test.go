package incident

import (
	"testing"
)

func TestNew_ValidIncident(t *testing.T) {
	inc, err := New("cluster-1", "default", "nginx-abc123", TypeCrashLoopBackOff, SeverityHigh, "pod is crash looping")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if inc.ClusterName != "cluster-1" {
		t.Errorf("expected cluster_name 'cluster-1', got '%s'", inc.ClusterName)
	}
	if inc.Namespace != "default" {
		t.Errorf("expected namespace 'default', got '%s'", inc.Namespace)
	}
	if inc.PodName != "nginx-abc123" {
		t.Errorf("expected pod_name 'nginx-abc123', got '%s'", inc.PodName)
	}
	if inc.Type != TypeCrashLoopBackOff {
		t.Errorf("expected type CrashLoopBackOff, got '%s'", inc.Type)
	}
	if inc.Status != StatusDetected {
		t.Errorf("expected status 'detected', got '%s'", inc.Status)
	}
	if inc.Severity != SeverityHigh {
		t.Errorf("expected severity 'high', got '%s'", inc.Severity)
	}
	if inc.RawData == nil {
		t.Error("expected raw_data to be initialized")
	}
}

func TestNew_EmptyClusterName(t *testing.T) {
	_, err := New("", "default", "nginx", TypeOOMKilled, SeverityCritical, "oom")
	if err == nil {
		t.Fatal("expected validation error for empty cluster_name")
	}
}

func TestNew_EmptyNamespace(t *testing.T) {
	_, err := New("cluster-1", "", "nginx", TypeOOMKilled, SeverityCritical, "oom")
	if err == nil {
		t.Fatal("expected validation error for empty namespace")
	}
}

func TestNew_EmptyPodName(t *testing.T) {
	_, err := New("cluster-1", "default", "", TypeOOMKilled, SeverityCritical, "oom")
	if err == nil {
		t.Fatal("expected validation error for empty pod_name")
	}
}

func TestNew_InvalidType(t *testing.T) {
	_, err := New("cluster-1", "default", "nginx", Type("InvalidType"), SeverityHigh, "msg")
	if err == nil {
		t.Fatal("expected validation error for invalid type")
	}
}

func TestNew_InvalidSeverity(t *testing.T) {
	_, err := New("cluster-1", "default", "nginx", TypeOOMKilled, Severity("unknown"), "msg")
	if err == nil {
		t.Fatal("expected validation error for invalid severity")
	}
}

func TestIncident_StateTransitions(t *testing.T) {
	inc, err := New("cluster-1", "default", "nginx", TypeCrashLoopBackOff, SeverityHigh, "crash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// detected -> analyzing
	if err := inc.MarkAnalyzing(); err != nil {
		t.Fatalf("expected no error transitioning to analyzing, got %v", err)
	}
	if inc.Status != StatusAnalyzing {
		t.Errorf("expected status 'analyzing', got '%s'", inc.Status)
	}

	// analyzing -> remediating
	if err := inc.MarkRemediating(); err != nil {
		t.Fatalf("expected no error transitioning to remediating, got %v", err)
	}
	if inc.Status != StatusRemediating {
		t.Errorf("expected status 'remediating', got '%s'", inc.Status)
	}

	// remediating -> resolved
	if err := inc.MarkResolved(); err != nil {
		t.Fatalf("expected no error transitioning to resolved, got %v", err)
	}
	if inc.Status != StatusResolved {
		t.Errorf("expected status 'resolved', got '%s'", inc.Status)
	}
	if inc.ResolvedAt == nil {
		t.Error("expected resolved_at to be set")
	}
}

func TestIncident_InvalidStateTransition(t *testing.T) {
	inc, _ := New("cluster-1", "default", "nginx", TypeCrashLoopBackOff, SeverityHigh, "crash")

	// Cannot remediate from detected
	if err := inc.MarkRemediating(); err == nil {
		t.Error("expected error when transitioning from detected to remediating")
	}

	// Cannot analyze twice
	_ = inc.MarkAnalyzing()
	if err := inc.MarkAnalyzing(); err == nil {
		t.Error("expected error when transitioning from analyzing to analyzing")
	}

	// Cannot resolve twice
	_ = inc.MarkRemediating()
	_ = inc.MarkResolved()
	if err := inc.MarkResolved(); err == nil {
		t.Error("expected error when resolving an already resolved incident")
	}
}

func TestIncident_AddRawData(t *testing.T) {
	inc, _ := New("cluster-1", "default", "nginx", TypeOOMKilled, SeverityCritical, "oom")
	inc.AddRawData("pod_logs", "error: out of memory")
	inc.AddRawData("events", "OOMKilled at 12:00")

	if len(inc.RawData) != 2 {
		t.Errorf("expected 2 raw data entries, got %d", len(inc.RawData))
	}
	if inc.RawData["pod_logs"] != "error: out of memory" {
		t.Errorf("unexpected pod_logs value: %s", inc.RawData["pod_logs"])
	}
}

func TestIncident_MarkFailed(t *testing.T) {
	inc, _ := New("cluster-1", "default", "nginx", TypeCrashLoopBackOff, SeverityHigh, "crash")
	if err := inc.MarkFailed(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inc.Status != StatusFailed {
		t.Errorf("expected status 'failed', got '%s'", inc.Status)
	}
}
