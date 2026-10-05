package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/report"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func setupIncidentTestRouter(incRepo incident.Repository, reportRepo report.Repository, prRepo domainGitops.Repository) (*chi.Mux, *Handler) {
	h := NewHandler(incRepo, reportRepo, prRepo, nil, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, h
}

func TestCreateIncident_Success(t *testing.T) {
	incRepo := newMockIncidentRepo()
	r, _ := setupIncidentTestRouter(incRepo, nil, nil)

	payload := map[string]interface{}{
		"title":        "High Error Rate on Checkout",
		"severity":     "critical",
		"description":  "5xx rate jumped to 12% in production",
		"cluster_id":   "cluster-prod-1",
		"service_name": "checkout-api",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/incidents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Authenticated context with operator role and specific tenant
	ctx := tenancy.WithTenantID(req.Context(), "tenant-payment")
	ctx = tenancy.WithUserRole(ctx, "operator")
	ctx = tenancy.WithUserID(ctx, "user-ops-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var resp incident.Incident
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID == "" {
		t.Error("expected non-empty incident ID")
	}
	if resp.Title != "High Error Rate on Checkout" {
		t.Errorf("expected title 'High Error Rate on Checkout', got '%s'", resp.Title)
	}
	if resp.Severity != incident.SeverityCritical {
		t.Errorf("expected severity critical, got '%s'", resp.Severity)
	}
	if resp.ClusterID != "cluster-prod-1" {
		t.Errorf("expected cluster_id 'cluster-prod-1', got '%s'", resp.ClusterID)
	}
	if resp.ServiceName != "checkout-api" {
		t.Errorf("expected service_name 'checkout-api', got '%s'", resp.ServiceName)
	}
	if resp.TenantID != "tenant-payment" {
		t.Errorf("expected tenant_id 'tenant-payment', got '%s'", resp.TenantID)
	}
	if resp.Status != incident.StatusDetected {
		t.Errorf("expected status 'detected', got '%s'", resp.Status)
	}
}

func TestCreateIncident_ValidationErrors(t *testing.T) {
	incRepo := newMockIncidentRepo()
	r, _ := setupIncidentTestRouter(incRepo, nil, nil)

	testCases := []struct {
		name    string
		payload map[string]interface{}
	}{
		{
			name: "missing title",
			payload: map[string]interface{}{
				"severity":     "high",
				"description":  "desc",
				"cluster_id":   "c1",
				"service_name": "s1",
			},
		},
		{
			name: "missing severity",
			payload: map[string]interface{}{
				"title":        "title",
				"description":  "desc",
				"cluster_id":   "c1",
				"service_name": "s1",
			},
		},
		{
			name: "invalid severity",
			payload: map[string]interface{}{
				"title":        "title",
				"severity":     "extreme",
				"description":  "desc",
				"cluster_id":   "c1",
				"service_name": "s1",
			},
		},
		{
			name: "missing description",
			payload: map[string]interface{}{
				"title":        "title",
				"severity":     "high",
				"cluster_id":   "c1",
				"service_name": "s1",
			},
		},
		{
			name: "missing cluster_id",
			payload: map[string]interface{}{
				"title":        "title",
				"severity":     "high",
				"description":  "desc",
				"service_name": "s1",
			},
		},
		{
			name: "missing service_name",
			payload: map[string]interface{}{
				"title":       "title",
				"severity":    "high",
				"description": "desc",
				"cluster_id":  "c1",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest(http.MethodPost, "/incidents", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 Bad Request for %s, got %d: %s", tc.name, w.Code, w.Body.String())
			}
		})
	}
}

func TestMitigateIncident_Success(t *testing.T) {
	incRepo := newMockIncidentRepo()
	r, _ := setupIncidentTestRouter(incRepo, nil, nil)

	inc, _ := incident.New("cluster-prod", "default", "payment-service", incident.TypeCrashLoopBackOff, incident.SeverityHigh, "crash")
	inc.ID = "inc-mitigate-1"
	incRepo.incidents[inc.ID] = inc

	payload := map[string]string{
		"notes": "Restarted pod with increased CPU limits",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/incidents/inc-mitigate-1/mitigate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := tenancy.WithUserID(req.Context(), "user-operator-1")
	ctx = tenancy.WithUserRole(ctx, "operator")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var updated incident.Incident
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if updated.Status != incident.StatusMitigating {
		t.Errorf("expected status 'mitigating', got '%s'", updated.Status)
	}
	if updated.RawData["mitigated_by"] != "user-operator-1" {
		t.Errorf("expected mitigated_by 'user-operator-1', got '%s'", updated.RawData["mitigated_by"])
	}
	if updated.RawData["mitigation_notes"] != "Restarted pod with increased CPU limits" {
		t.Errorf("expected mitigation_notes, got '%s'", updated.RawData["mitigation_notes"])
	}
}

func TestMitigateIncident_NotFound(t *testing.T) {
	incRepo := newMockIncidentRepo()
	r, _ := setupIncidentTestRouter(incRepo, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/incidents/nonexistent-id/mitigate", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d: %s", w.Code, w.Body.String())
	}
}

func TestResolveIncident_Success(t *testing.T) {
	incRepo := newMockIncidentRepo()
	r, _ := setupIncidentTestRouter(incRepo, nil, nil)

	inc, _ := incident.New("cluster-prod", "default", "payment-service", incident.TypeCrashLoopBackOff, incident.SeverityHigh, "crash")
	inc.ID = "inc-resolve-1"
	incRepo.incidents[inc.ID] = inc

	payload := map[string]string{
		"notes": "Fixed database deadlocks and verified service health",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/incidents/inc-resolve-1/resolve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := tenancy.WithUserID(req.Context(), "user-lead-1")
	ctx = tenancy.WithUserRole(ctx, "operator")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var updated incident.Incident
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if updated.Status != incident.StatusResolved {
		t.Errorf("expected status 'resolved', got '%s'", updated.Status)
	}
	if updated.ResolvedAt == nil {
		t.Error("expected non-nil resolved_at timestamp")
	}
	if updated.RawData["resolved_by"] != "user-lead-1" {
		t.Errorf("expected resolved_by 'user-lead-1', got '%s'", updated.RawData["resolved_by"])
	}
	if updated.RawData["resolution_notes"] != "Fixed database deadlocks and verified service health" {
		t.Errorf("expected resolution_notes, got '%s'", updated.RawData["resolution_notes"])
	}
}

func TestResolveIncident_NotFound(t *testing.T) {
	incRepo := newMockIncidentRepo()
	r, _ := setupIncidentTestRouter(incRepo, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/incidents/nonexistent-id/resolve", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetIncidentReport_404WhenNotFound(t *testing.T) {
	reportRepo := newMockReportRepo()
	r, _ := setupIncidentTestRouter(nil, reportRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/incidents/inc-no-report/report", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found when RCA report does not exist, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "RCA report not found" {
		t.Errorf("expected error message 'RCA report not found', got '%s'", errResp["error"])
	}
}

func TestGetIncidentPR_404WhenNotFound(t *testing.T) {
	prRepo := newMockPRRepo()
	r, _ := setupIncidentTestRouter(nil, nil, prRepo)

	req := httptest.NewRequest(http.MethodGet, "/incidents/inc-no-pr/pr", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found when PR does not exist, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "PR not found" {
		t.Errorf("expected error message 'PR not found', got '%s'", errResp["error"])
	}
}

func TestIncidentLifecycle_TenantIsolation(t *testing.T) {
	incRepo := newMockIncidentRepo()
	r, _ := setupIncidentTestRouter(incRepo, nil, nil)

	// Tenant A creates incident
	createPayload := map[string]interface{}{
		"title":        "Database Latency Surge",
		"severity":     "medium",
		"description":  "Query latency exceeds 200ms",
		"cluster_id":   "cluster-a",
		"service_name": "db-service",
	}
	body, _ := json.Marshal(createPayload)

	req := httptest.NewRequest(http.MethodPost, "/incidents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctxA := tenancy.WithTenantID(req.Context(), "tenant-a")
	ctxA = tenancy.WithUserRole(ctxA, "operator")
	req = req.WithContext(ctxA)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var created incident.Incident
	_ = json.NewDecoder(w.Body).Decode(&created)

	// Tenant B attempts to mitigate Tenant A's incident
	mitigateReq := httptest.NewRequest(http.MethodPost, "/incidents/"+created.ID+"/mitigate", bytes.NewReader([]byte("{}")))
	mitigateReq.Header.Set("Content-Type", "application/json")
	ctxB := tenancy.WithTenantID(mitigateReq.Context(), "tenant-b")
	ctxB = tenancy.WithUserRole(ctxB, "operator")
	mitigateReq = mitigateReq.WithContext(ctxB)

	wMitigate := httptest.NewRecorder()
	r.ServeHTTP(wMitigate, mitigateReq)

	if wMitigate.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for cross-tenant mitigate attempt, got %d: %s", wMitigate.Code, wMitigate.Body.String())
	}
}
