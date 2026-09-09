package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/report"
	"github.com/datdt/k8sselfhost/internal/usecase/rca"
)

func TestSimulateIncident_Default_OOMKilled(t *testing.T) {
	incRepo := newMockIncidentRepo()
	reportRepo := newMockReportRepo()
	prRepo := newMockPRRepo()

	h := NewHandler(incRepo, reportRepo, prRepo, nil, nil)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/incidents/simulate", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var inc incident.Incident
	if err := json.NewDecoder(w.Body).Decode(&inc); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if inc.Type != incident.TypeOOMKilled {
		t.Errorf("expected type %s, got %s", incident.TypeOOMKilled, inc.Type)
	}
	if inc.Severity != incident.SeverityCritical {
		t.Errorf("expected severity %s, got %s", incident.SeverityCritical, inc.Severity)
	}
	if inc.Namespace != "production" {
		t.Errorf("expected namespace 'production', got '%s'", inc.Namespace)
	}

	// Verify report creation
	reportRepo.mu.Lock()
	reportsCount := len(reportRepo.created)
	var createdRpt *report.Report
	if reportsCount > 0 {
		createdRpt = reportRepo.created[0]
	}
	reportRepo.mu.Unlock()

	if reportsCount != 1 {
		t.Fatalf("expected 1 report created, got %d", reportsCount)
	}
	if createdRpt.RiskLevel != report.RiskHigh {
		t.Errorf("expected report risk level %s, got %s", report.RiskHigh, createdRpt.RiskLevel)
	}
	if createdRpt.Confidence < 0.9 {
		t.Errorf("expected report confidence >= 0.9, got %f", createdRpt.Confidence)
	}

	// Verify GitOps PR creation
	prRepo.mu.Lock()
	prsCount := len(prRepo.created)
	var createdPR *domainGitops.PullRequest
	if prsCount > 0 {
		createdPR = prRepo.created[0]
	}
	prRepo.mu.Unlock()

	if prsCount != 1 {
		t.Fatalf("expected 1 PR created, got %d", prsCount)
	}
	if len(createdPR.FilesChanged) == 0 {
		t.Errorf("expected at least 1 file changed in PR")
	}
}

func TestSimulateIncident_NodeDown(t *testing.T) {
	incRepo := newMockIncidentRepo()
	reportRepo := newMockReportRepo()
	prRepo := newMockPRRepo()

	h := NewHandler(incRepo, reportRepo, prRepo, nil, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	payload := `{"scenario": "node_down", "pod_name": "worker-srv-99", "namespace": "infrastructure"}`
	req := httptest.NewRequest(http.MethodPost, "/incidents/simulate", bytes.NewReader([]byte(payload)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var inc incident.Incident
	_ = json.NewDecoder(w.Body).Decode(&inc)

	if inc.Type != incident.TypeNodeNotReady {
		t.Errorf("expected type %s, got %s", incident.TypeNodeNotReady, inc.Type)
	}
	if inc.PodName != "worker-srv-99" {
		t.Errorf("expected pod_name worker-srv-99, got %s", inc.PodName)
	}
	if inc.Namespace != "infrastructure" {
		t.Errorf("expected namespace infrastructure, got %s", inc.Namespace)
	}
	if inc.Severity != incident.SeverityCritical {
		t.Errorf("expected severity critical, got %s", inc.Severity)
	}
}

func TestSimulateIncident_CrashLoop(t *testing.T) {
	incRepo := newMockIncidentRepo()
	reportRepo := newMockReportRepo()
	prRepo := newMockPRRepo()

	h := NewHandler(incRepo, reportRepo, prRepo, nil, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	payload := `{"scenario": "crash_loop", "pod_name": "backend-auth-77", "namespace": "staging"}`
	req := httptest.NewRequest(http.MethodPost, "/incidents/simulate", bytes.NewReader([]byte(payload)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var inc incident.Incident
	_ = json.NewDecoder(w.Body).Decode(&inc)

	if inc.Type != incident.TypeCrashLoopBackOff {
		t.Errorf("expected type %s, got %s", incident.TypeCrashLoopBackOff, inc.Type)
	}
	if inc.PodName != "backend-auth-77" {
		t.Errorf("expected pod_name backend-auth-77, got %s", inc.PodName)
	}
}

func TestSimulateIncident_ResourceExhaustion(t *testing.T) {
	incRepo := newMockIncidentRepo()
	reportRepo := newMockReportRepo()
	prRepo := newMockPRRepo()

	h := NewHandler(incRepo, reportRepo, prRepo, nil, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	payload := `{"scenario": "resource_exhaustion"}`
	req := httptest.NewRequest(http.MethodPost, "/incidents/simulate", bytes.NewReader([]byte(payload)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var inc incident.Incident
	_ = json.NewDecoder(w.Body).Decode(&inc)

	if inc.Type != incident.TypeResourceExhaust {
		t.Errorf("expected type %s, got %s", incident.TypeResourceExhaust, inc.Type)
	}
}

func TestSimulateIncident_InvalidJSON(t *testing.T) {
	incRepo := newMockIncidentRepo()
	h := NewHandler(incRepo, nil, nil, nil, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/incidents/simulate", bytes.NewReader([]byte("{invalid-json")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", w.Code)
	}
}

func TestAnalyzeIncident_Standalone_FallbackAndPRCreation(t *testing.T) {
	incRepo := newMockIncidentRepo()
	reportRepo := newMockReportRepo()
	prRepo := newMockPRRepo()

	h := NewHandler(incRepo, reportRepo, prRepo, nil, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	inc, err := incident.New("fleet-primary", "infrastructure", "k8sworker3", incident.TypeNodeNotReady, incident.SeverityCritical, "Node unreachable")
	if err != nil {
		t.Fatalf("failed to create incident: %v", err)
	}
	inc.ID = "inc-node-down-1"
	incRepo.incidents[inc.ID] = inc

	req := httptest.NewRequest(http.MethodPost, "/incidents/inc-node-down-1/analyze", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d: %s", w.Code, w.Body.String())
	}

	// Wait for background fallback synthesis and PR creation
	var (
		rpt *report.Report
		pr  *domainGitops.PullRequest
	)
	ctx := context.Background()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if r, err := reportRepo.GetByIncidentID(ctx, "inc-node-down-1"); err == nil && r != nil {
			rpt = r
		}
		if p, err := prRepo.GetByIncidentID(ctx, "inc-node-down-1"); err == nil && p != nil {
			pr = p
		}
		if rpt != nil && pr != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if rpt == nil {
		t.Fatal("expected fallback report to be created in report repository")
	}
	if rpt.Confidence != 0.92 {
		t.Errorf("expected confidence 0.92 for critical incident, got %f", rpt.Confidence)
	}
	if rpt.RiskLevel != report.RiskCritical {
		t.Errorf("expected risk level critical, got %s", rpt.RiskLevel)
	}
	if rpt.LLMModel != "heuristic-deterministic-v1" {
		t.Errorf("expected LLMModel 'heuristic-deterministic-v1', got %s", rpt.LLMModel)
	}

	if pr == nil {
		t.Fatal("expected automated remediation PR to be created in PR repository")
	}
	if len(pr.FilesChanged) == 0 {
		t.Errorf("expected files changed in PR")
	}

	// Verify incident status transitioned to remediating
	updatedInc, err := incRepo.GetByID(ctx, "inc-node-down-1")
	if err != nil {
		t.Fatalf("failed to get updated incident: %v", err)
	}
	if updatedInc.Status != incident.StatusRemediating {
		t.Errorf("expected incident status %s, got %s", incident.StatusRemediating, updatedInc.Status)
	}
}

func TestAnalyzeIncident_WithRCAPipeline_LLMFailureFallback(t *testing.T) {
	incRepo := newMockIncidentRepo()
	reportRepo := newMockReportRepo()
	prRepo := newMockPRRepo()

	// Pipeline without registry or collector to trigger deterministic fallback
	pipeline := rca.NewPipeline(nil, nil, reportRepo, incRepo, nil)

	h := NewHandler(incRepo, reportRepo, prRepo, nil, nil)
	h.SetRCAPipeline(pipeline)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	inc, _ := incident.New("fleet-primary", "production", "payment-api-9f8d", incident.TypeOOMKilled, incident.SeverityCritical, "Container OOM")
	inc.ID = "inc-oom-pipeline-1"
	incRepo.incidents[inc.ID] = inc

	req := httptest.NewRequest(http.MethodPost, "/incidents/inc-oom-pipeline-1/analyze", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d: %s", w.Code, w.Body.String())
	}

	var (
		rpt *report.Report
		pr  *domainGitops.PullRequest
	)
	ctx := context.Background()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if r, err := reportRepo.GetByIncidentID(ctx, "inc-oom-pipeline-1"); err == nil && r != nil {
			rpt = r
		}
		if p, err := prRepo.GetByIncidentID(ctx, "inc-oom-pipeline-1"); err == nil && p != nil {
			pr = p
		}
		if rpt != nil && pr != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if rpt == nil {
		t.Fatal("expected report to be created via pipeline fallback")
	}
	if rpt.LLMModel != "heuristic-deterministic-v1" {
		t.Errorf("expected model 'heuristic-deterministic-v1', got %s", rpt.LLMModel)
	}
	if pr == nil {
		t.Fatal("expected remediation PR to be created")
	}

	updatedInc, _ := incRepo.GetByID(ctx, "inc-oom-pipeline-1")
	if updatedInc.Status != incident.StatusRemediating {
		t.Errorf("expected status %s, got %s", incident.StatusRemediating, updatedInc.Status)
	}
}

func TestAnalyzeIncident_NotFound(t *testing.T) {
	incRepo := newMockIncidentRepo()
	h := NewHandler(incRepo, nil, nil, nil, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/incidents/non-existent/analyze", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", w.Code)
	}
}

func TestAnalyzeIncident_AlreadyAnalyzing(t *testing.T) {
	incRepo := newMockIncidentRepo()
	reportRepo := newMockReportRepo()
	h := NewHandler(incRepo, reportRepo, nil, nil, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	inc, _ := incident.New("fleet-primary", "production", "worker", incident.TypeResourceExhaust, incident.SeverityHigh, "Resource exhaust")
	inc.ID = "inc-already-analyzing"
	inc.Status = incident.StatusAnalyzing
	incRepo.incidents[inc.ID] = inc

	req := httptest.NewRequest(http.MethodPost, "/incidents/inc-already-analyzing/analyze", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202 Accepted when already analyzing, got %d", w.Code)
	}
}

