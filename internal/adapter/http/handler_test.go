package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"

	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/report"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"key": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected content-type application/json, got %s", w.Header().Get("Content-Type"))
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusNotFound, "not found", nil)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestParseIntParam(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=25&offset=10", nil)

	limit := parseIntParam(r, "limit", 50)
	if limit != 25 {
		t.Errorf("expected limit 25, got %d", limit)
	}

	offset := parseIntParam(r, "offset", 0)
	if offset != 10 {
		t.Errorf("expected offset 10, got %d", offset)
	}
}

func TestParseIntParam_Default(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	limit := parseIntParam(r, "limit", 50)
	if limit != 50 {
		t.Errorf("expected default limit 50, got %d", limit)
	}
}

func TestParseIntParam_Invalid(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=abc", nil)

	limit := parseIntParam(r, "limit", 50)
	if limit != 50 {
		t.Errorf("expected default limit 50 for invalid input, got %d", limit)
	}
}

func TestAuthMiddleware_NoToken(t *testing.T) {
	handler := AuthMiddleware("")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 when no token configured, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	handler := AuthMiddleware("secret-token")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer secret-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with valid token, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	handler := AuthMiddleware("secret-token")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 with invalid token, got %d", w.Code)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	handler := AuthMiddleware("secret-token")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with missing header, got %d", w.Code)
	}
}

func TestParseUUIDParam(t *testing.T) {
	t.Run("valid UUID", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/incidents/123e4567-e89b-12d3-a456-426614174000", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "123e4567-e89b-12d3-a456-426614174000")
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		id, err := parseUUIDParam(r, "id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "123e4567-e89b-12d3-a456-426614174000" {
			t.Errorf("expected UUID, got %s", id)
		}
	})

	t.Run("valid custom string ID", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/services/service-auth-prod", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "service-auth-prod")
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		id, err := parseUUIDParam(r, "id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "service-auth-prod" {
			t.Errorf("expected string ID, got %s", id)
		}
	})

	t.Run("missing parameter", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/incidents/", nil)
		rctx := chi.NewRouteContext()
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		_, err := parseUUIDParam(r, "id")
		if err == nil {
			t.Fatalf("expected error for missing param, got nil")
		}
	})

	t.Run("parameter too long", func(t *testing.T) {
		longID := strings.Repeat("a", 129)
		r := httptest.NewRequest(http.MethodGet, "/incidents/"+longID, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", longID)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		_, err := parseUUIDParam(r, "id")
		if err == nil {
			t.Fatalf("expected error for overly long param (>128 chars), got nil")
		}
	})
}

type mockIncidentRepo struct {
	mu        sync.Mutex
	incidents map[string]*incident.Incident
	created   []*incident.Incident
	updated   []*incident.Incident
}

func newMockIncidentRepo() *mockIncidentRepo {
	return &mockIncidentRepo{
		incidents: make(map[string]*incident.Incident),
	}
}

func (m *mockIncidentRepo) Create(ctx context.Context, inc *incident.Incident) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if inc.ID == "" {
		inc.ID = "inc-sim-123"
	}
	m.incidents[inc.ID] = inc
	m.created = append(m.created, inc)
	return nil
}

func (m *mockIncidentRepo) GetByID(ctx context.Context, id string) (*incident.Incident, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inc, ok := m.incidents[id]
	if !ok {
		return nil, errors.NewNotFound("incident", id)
	}
	return inc, nil
}

func (m *mockIncidentRepo) Update(ctx context.Context, inc *incident.Incident) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.incidents[inc.ID] = inc
	m.updated = append(m.updated, inc)
	return nil
}

func (m *mockIncidentRepo) List(ctx context.Context, filter incident.Filter) ([]*incident.Incident, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []*incident.Incident
	for _, inc := range m.incidents {
		res = append(res, inc)
	}
	return res, int64(len(res)), nil
}

func (m *mockIncidentRepo) GetByPodAndType(ctx context.Context, namespace, podName string, incidentType incident.Type) (*incident.Incident, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, inc := range m.incidents {
		if inc.Namespace == namespace && inc.PodName == podName && inc.Type == incidentType {
			return inc, nil
		}
	}
	return nil, nil
}

type mockReportRepo struct {
	mu      sync.Mutex
	reports map[string]*report.Report
	created []*report.Report
}

func newMockReportRepo() *mockReportRepo {
	return &mockReportRepo{
		reports: make(map[string]*report.Report),
	}
}

func (m *mockReportRepo) Create(ctx context.Context, rpt *report.Report) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rpt.ID == "" {
		rpt.ID = "rpt-sim-123"
	}
	m.reports[rpt.IncidentID] = rpt
	m.created = append(m.created, rpt)
	return nil
}

func (m *mockReportRepo) GetByID(ctx context.Context, id string) (*report.Report, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.reports {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, errors.NewNotFound("report", id)
}

func (m *mockReportRepo) GetByIncidentID(ctx context.Context, incidentID string) (*report.Report, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.reports[incidentID]
	if !ok {
		return nil, errors.NewNotFound("report", incidentID)
	}
	return r, nil
}

func (m *mockReportRepo) List(ctx context.Context, limit, offset int) ([]*report.Report, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []*report.Report
	for _, r := range m.reports {
		res = append(res, r)
	}
	return res, int64(len(res)), nil
}

type mockPRRepo struct {
	mu      sync.Mutex
	prs     map[string]*domainGitops.PullRequest
	created []*domainGitops.PullRequest
}

func newMockPRRepo() *mockPRRepo {
	return &mockPRRepo{
		prs: make(map[string]*domainGitops.PullRequest),
	}
}

func (m *mockPRRepo) Create(ctx context.Context, pr *domainGitops.PullRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if pr.ID == "" {
		pr.ID = "pr-sim-123"
	}
	m.prs[pr.ID] = pr
	m.created = append(m.created, pr)
	return nil
}

func (m *mockPRRepo) GetByID(ctx context.Context, id string) (*domainGitops.PullRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pr, ok := m.prs[id]
	if !ok {
		return nil, errors.NewNotFound("pr", id)
	}
	return pr, nil
}

func (m *mockPRRepo) GetByIncidentID(ctx context.Context, incidentID string) (*domainGitops.PullRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, pr := range m.prs {
		if pr.IncidentID == incidentID {
			return pr, nil
		}
	}
	return nil, errors.NewNotFound("pr", incidentID)
}

func (m *mockPRRepo) Update(ctx context.Context, pr *domainGitops.PullRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prs[pr.ID] = pr
	return nil
}

func (m *mockPRRepo) List(ctx context.Context, status *domainGitops.PRStatus, limit, offset int) ([]*domainGitops.PullRequest, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []*domainGitops.PullRequest
	for _, pr := range m.prs {
		if status == nil || pr.Status == *status {
			res = append(res, pr)
		}
	}
	return res, int64(len(res)), nil
}

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

