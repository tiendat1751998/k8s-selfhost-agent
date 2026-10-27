package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func TestAcknowledgeIncident_Success(t *testing.T) {
	incRepo := newMockIncidentRepo()
	r, _ := setupIncidentTestRouter(incRepo, nil, nil)

	inc := &incident.Incident{
		ID:          "inc-ack-1",
		TenantID:    "tenant-alpha",
		ClusterID:   "cluster-prod",
		ClusterName: "cluster-prod",
		Namespace:   "default",
		PodName:     "payment-service-abc",
		Type:        incident.TypeCrashLoopBackOff,
		Status:      incident.StatusDetected,
		Severity:    incident.SeverityCritical,
		Message:     "Pod crashing continuously",
		CreatedAt:   time.Now().UTC().Add(-10 * time.Minute),
		UpdatedAt:   time.Now().UTC().Add(-10 * time.Minute),
	}
	incRepo.incidents[inc.ID] = inc

	payload := map[string]string{
		"notes": "Acknowledged by SRE on-call. Checking logs.",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/incidents/inc-ack-1/acknowledge", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := tenancy.WithTenantID(req.Context(), "tenant-alpha")
	ctx = tenancy.WithUserRole(ctx, "operator")
	ctx = tenancy.WithUserID(ctx, "alice-sre")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp incident.Incident
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != incident.StatusInvestigating && resp.Status != "acknowledged" {
		t.Errorf("expected status investigating or acknowledged, got %s", resp.Status)
	}
	if resp.RawData == nil || resp.RawData["acknowledged_by"] != "alice-sre" {
		t.Errorf("expected acknowledged_by alice-sre, got %v", resp.RawData)
	}
	if resp.RawData["acknowledgement_notes"] != "Acknowledged by SRE on-call. Checking logs." {
		t.Errorf("expected acknowledgement_notes matching, got %s", resp.RawData["acknowledgement_notes"])
	}
}

func TestAcknowledgeIncident_NotFound(t *testing.T) {
	incRepo := newMockIncidentRepo()
	r, _ := setupIncidentTestRouter(incRepo, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/incidents/inc-missing/acknowledge", bytes.NewReader([]byte("{}")))
	ctx := tenancy.WithTenantID(req.Context(), "tenant-alpha")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", w.Code)
	}
}

func TestAcknowledgeIncident_AlreadyResolved(t *testing.T) {
	incRepo := newMockIncidentRepo()
	r, _ := setupIncidentTestRouter(incRepo, nil, nil)

	now := time.Now().UTC()
	inc := &incident.Incident{
		ID:          "inc-resolved-1",
		TenantID:    "tenant-alpha",
		Status:      incident.StatusResolved,
		Severity:    incident.SeverityMedium,
		ResolvedAt:  &now,
		CreatedAt:   now.Add(-1 * time.Hour),
		UpdatedAt:   now,
	}
	incRepo.incidents[inc.ID] = inc

	req := httptest.NewRequest(http.MethodPost, "/incidents/inc-resolved-1/acknowledge", bytes.NewReader([]byte("{}")))
	ctx := tenancy.WithTenantID(req.Context(), "tenant-alpha")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict when acknowledging resolved incident, got %d", w.Code)
	}
}

func TestGetPostmortem_Success(t *testing.T) {
	incRepo := newMockIncidentRepo()
	r, _ := setupIncidentTestRouter(incRepo, nil, nil)

	created := time.Now().UTC().Add(-30 * time.Minute)
	resolved := time.Now().UTC()
	inc := &incident.Incident{
		ID:          "inc-pm-1",
		TenantID:    "tenant-alpha",
		ClusterID:   "cluster-prod",
		ClusterName: "cluster-prod",
		Namespace:   "payments",
		PodName:     "gateway-xyz",
		Type:        incident.TypeOOMKilled,
		Status:      incident.StatusResolved,
		Severity:    incident.SeverityCritical,
		Message:     "Gateway OOMKilled under traffic spike",
		CreatedAt:   created,
		UpdatedAt:   resolved,
		ResolvedAt:  &resolved,
		RawData: map[string]string{
			"resolution_notes": "Increased memory limits to 2Gi",
			"resolved_by":      "bob-sre",
		},
	}
	incRepo.incidents[inc.ID] = inc

	req := httptest.NewRequest(http.MethodGet, "/incidents/inc-pm-1/postmortem", nil)
	ctx := tenancy.WithTenantID(req.Context(), "tenant-alpha")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}

	md, ok := resp["markdown"].(string)
	if !ok || md == "" {
		t.Fatalf("expected non-empty markdown field in postmortem response")
	}

	if !strings.Contains(md, "Postmortem") && !strings.Contains(md, "Incident") {
		t.Errorf("markdown does not contain Postmortem header: %s", md)
	}
	if !strings.Contains(md, "gateway-xyz") {
		t.Errorf("markdown does not contain pod name: %s", md)
	}
	if !strings.Contains(md, "OOMKilled") {
		t.Errorf("markdown does not contain incident type: %s", md)
	}
}

func TestGetPostmortem_NotFound(t *testing.T) {
	incRepo := newMockIncidentRepo()
	r, _ := setupIncidentTestRouter(incRepo, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/incidents/inc-missing/postmortem", nil)
	ctx := tenancy.WithTenantID(req.Context(), "tenant-alpha")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", w.Code)
	}
}

func TestAnalyzeIncident_OnResolvedIncident_Allowed(t *testing.T) {
	incRepo := newMockIncidentRepo()
	r, _ := setupIncidentTestRouter(incRepo, nil, nil)

	now := time.Now().UTC()
	inc := &incident.Incident{
		ID:         "inc-resolved-forensics",
		TenantID:   "tenant-alpha",
		Status:     incident.StatusResolved,
		Severity:   incident.SeverityHigh,
		ResolvedAt: &now,
		CreatedAt:  now.Add(-2 * time.Hour),
		UpdatedAt:  now,
	}
	incRepo.incidents[inc.ID] = inc

	req := httptest.NewRequest(http.MethodPost, "/incidents/inc-resolved-forensics/analyze", nil)
	ctx := tenancy.WithTenantID(req.Context(), "tenant-alpha")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Must NOT return 409 Conflict - post-incident forensics on resolved incidents is allowed!
	if w.Code == http.StatusConflict {
		t.Fatalf("unexpected 409 Conflict when analyzing resolved incident for forensics: %s", w.Body.String())
	}
	if w.Code != http.StatusOK && w.Code != http.StatusAccepted {
		t.Fatalf("expected 200 OK or 202 Accepted, got %d: %s", w.Code, w.Body.String())
	}
}
