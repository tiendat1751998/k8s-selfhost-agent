package incident_test

import (
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
)

func TestIncident_Mitigate(t *testing.T) {
	inc, err := incident.New("cluster-1", "default", "payment-service", incident.TypeCrashLoopBackOff, incident.SeverityCritical, "Pod crashing")
	if err != nil {
		t.Fatalf("failed to create incident: %v", err)
	}

	err = inc.Mitigate("operator-alice", "Triggered pod restart and increased memory limit", incident.StatusMitigating)
	if err != nil {
		t.Fatalf("expected no error on mitigate, got: %v", err)
	}

	if inc.Status != incident.StatusMitigating {
		t.Errorf("expected status 'mitigating', got '%s'", inc.Status)
	}
	if inc.RawData["mitigated_by"] != "operator-alice" {
		t.Errorf("expected mitigated_by 'operator-alice', got '%s'", inc.RawData["mitigated_by"])
	}
	if inc.RawData["mitigation_notes"] != "Triggered pod restart and increased memory limit" {
		t.Errorf("expected mitigation_notes, got '%s'", inc.RawData["mitigation_notes"])
	}
}

func TestIncident_Resolve(t *testing.T) {
	inc, err := incident.New("cluster-1", "default", "payment-service", incident.TypeCrashLoopBackOff, incident.SeverityCritical, "Pod crashing")
	if err != nil {
		t.Fatalf("failed to create incident: %v", err)
	}

	before := time.Now().UTC().Add(-1 * time.Second)
	err = inc.Resolve("operator-bob", "Verified fix deployed, crash loop resolved")
	if err != nil {
		t.Fatalf("expected no error on resolve, got: %v", err)
	}

	if inc.Status != incident.StatusResolved {
		t.Errorf("expected status 'resolved', got '%s'", inc.Status)
	}
	if inc.ResolvedAt == nil || inc.ResolvedAt.Before(before) {
		t.Errorf("expected valid resolved_at timestamp, got %v", inc.ResolvedAt)
	}
	if inc.RawData["resolved_by"] != "operator-bob" {
		t.Errorf("expected resolved_by 'operator-bob', got '%s'", inc.RawData["resolved_by"])
	}
	if inc.RawData["resolution_notes"] != "Verified fix deployed, crash loop resolved" {
		t.Errorf("expected resolution_notes, got '%s'", inc.RawData["resolution_notes"])
	}

	// Resolving already resolved incident should return error
	err = inc.Resolve("operator-bob", "Duplicate resolve")
	if err == nil {
		t.Error("expected error when resolving already resolved incident")
	}
}
