package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/report"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func TestGetIncidentReport_PendingWhenIncidentExists(t *testing.T) {
	incRepo := newMockIncidentRepo()
	reportRepo := newMockReportRepo()
	r, _ := setupIncidentTestRouter(incRepo, reportRepo, nil)

	// Seed existing incident
	inc, _ := incident.New("cluster-1", "prod", "pod-1", incident.TypeCrashLoopBackOff, incident.SeverityHigh, "crashloop")
	inc.ID = "inc-existing-1"
	ctx := tenancy.WithTenantID(context.Background(), "default-tenant")
	_ = incRepo.Create(ctx, inc)

	req := httptest.NewRequest(http.MethodGet, "/incidents/inc-existing-1/report", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for pending report on existing incident, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode json response: %v", err)
	}

	if resp["available"] != false {
		t.Errorf("expected available == false, got %v", resp["available"])
	}
	if resp["incident_id"] != "inc-existing-1" {
		t.Errorf("expected incident_id == 'inc-existing-1', got %v", resp["incident_id"])
	}
	if resp["status"] != "pending" {
		t.Errorf("expected status == 'pending', got %v", resp["status"])
	}
}

func TestGetIncidentReport_404WhenIncidentDoesNotExist(t *testing.T) {
	incRepo := newMockIncidentRepo()
	reportRepo := newMockReportRepo()
	r, _ := setupIncidentTestRouter(incRepo, reportRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/incidents/inc-ghost-999/report", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found for non-existent incident, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetIncidentPR_PendingWhenIncidentExists(t *testing.T) {
	incRepo := newMockIncidentRepo()
	prRepo := newMockPRRepo()
	r, _ := setupIncidentTestRouter(incRepo, nil, prRepo)

	// Seed existing incident
	inc, _ := incident.New("cluster-1", "prod", "pod-2", incident.TypeOOMKilled, incident.SeverityCritical, "oom")
	inc.ID = "inc-existing-2"
	ctx := tenancy.WithTenantID(context.Background(), "default-tenant")
	_ = incRepo.Create(ctx, inc)

	req := httptest.NewRequest(http.MethodGet, "/incidents/inc-existing-2/pr", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for pending PR on existing incident, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode json response: %v", err)
	}

	if resp["available"] != false {
		t.Errorf("expected available == false, got %v", resp["available"])
	}
	if resp["incident_id"] != "inc-existing-2" {
		t.Errorf("expected incident_id == 'inc-existing-2', got %v", resp["incident_id"])
	}
	if resp["status"] != "pending" {
		t.Errorf("expected status == 'pending', got %v", resp["status"])
	}
}

func TestGetIncidentPR_404WhenIncidentDoesNotExist(t *testing.T) {
	incRepo := newMockIncidentRepo()
	prRepo := newMockPRRepo()
	r, _ := setupIncidentTestRouter(incRepo, nil, prRepo)

	req := httptest.NewRequest(http.MethodGet, "/incidents/inc-ghost-999/pr", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found for non-existent incident, got %d: %s", w.Code, w.Body.String())
	}
}

func TestEnsureRemediationPR_HonestWhenGitOpsNotConfigured(t *testing.T) {
	t.Setenv("GITOPS_REPO_URL", "")
	t.Setenv("K8S_GITOPS_REPO_URL", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("K8S_GITOPS_GITHUB_TOKEN", "")

	incRepo := newMockIncidentRepo()
	prRepo := newMockPRRepo()
	_, h := setupIncidentTestRouter(incRepo, nil, prRepo)

	inc, _ := incident.New("cluster-1", "prod", "pod-3", incident.TypeNodeNotReady, incident.SeverityCritical, "node unreachable")
	inc.ID = "inc-gitops-none"
	rpt, _ := report.New(inc.ID, "agent down", []string{"timeout"}, 0.9, report.RiskCritical, "restart", "none")

	h.ensureRemediationPR(context.Background(), inc, rpt)

	// Since GitOps environment is not configured, NO fake PR should be persisted
	pr, err := prRepo.GetByIncidentID(context.Background(), inc.ID)
	if err == nil || pr != nil {
		t.Fatalf("expected no PR to be created when GitOps is unconfigured, but found: %+v", pr)
	}
}

func TestEnsureRemediationPR_CreatesPRWhenGitOpsConfigured(t *testing.T) {
	t.Setenv("GITOPS_REPO_URL", "https://github.com/my-org/gitops-deploy")
	t.Setenv("GITHUB_TOKEN", "ghp_realtoken123456789")

	incRepo := newMockIncidentRepo()
	prRepo := newMockPRRepo()
	_, h := setupIncidentTestRouter(incRepo, nil, prRepo)

	inc, _ := incident.New("cluster-1", "prod", "pod-4", incident.TypeCrashLoopBackOff, incident.SeverityHigh, "crash")
	inc.ID = "inc-gitops-active"
	rpt, _ := report.New(inc.ID, "crash root cause", []string{"exit 1"}, 0.9, report.RiskHigh, "fix env", "rollback")

	h.ensureRemediationPR(context.Background(), inc, rpt)

	pr, err := prRepo.GetByIncidentID(context.Background(), inc.ID)
	if err != nil || pr == nil {
		t.Fatalf("expected PR to be created when GitOps is configured, got err: %v", err)
	}
	if pr.RepoURL != "https://github.com/my-org/gitops-deploy" {
		t.Errorf("expected configured RepoURL, got %q", pr.RepoURL)
	}
}
