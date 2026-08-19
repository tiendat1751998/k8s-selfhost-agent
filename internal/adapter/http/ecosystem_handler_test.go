package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/ecosystem"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type mockEcosystemUsecase struct {
	tools   []ecosystem.DetectedTool
	failErr error
}

func (m *mockEcosystemUsecase) Scan(ctx context.Context, tenantID string) ([]ecosystem.DetectedTool, error) {
	if m.failErr != nil {
		return nil, m.failErr
	}
	return m.tools, nil
}

func (m *mockEcosystemUsecase) GetAll(ctx context.Context, tenantID string) ([]ecosystem.DetectedTool, error) {
	if m.failErr != nil {
		return nil, m.failErr
	}
	return m.tools, nil
}

func (m *mockEcosystemUsecase) GetSummary(ctx context.Context, tenantID string) (*ecosystem.EcosystemSummary, error) {
	if m.failErr != nil {
		return nil, m.failErr
	}
	return &ecosystem.EcosystemSummary{
		Total:      len(m.tools),
		Healthy:    len(m.tools),
		Degraded:   0,
		ByCategory: map[string]int{ecosystem.CategoryGitOps: len(m.tools)},
	}, nil
}

func (m *mockEcosystemUsecase) CreateTool(ctx context.Context, tool *ecosystem.DetectedTool) error {
	if m.failErr != nil {
		return m.failErr
	}
	tool.ID = "new-tool-id"
	m.tools = append(m.tools, *tool)
	return nil
}

func (m *mockEcosystemUsecase) DeleteTool(ctx context.Context, tenantID, id string) error {
	if m.failErr != nil {
		return m.failErr
	}
	if id == "not-found" {
		return errors.New("tool not found")
	}
	var remaining []ecosystem.DetectedTool
	for _, t := range m.tools {
		if t.ID != id {
			remaining = append(remaining, t)
		}
	}
	m.tools = remaining
	return nil
}

func setupEcosystemRouter(uc *mockEcosystemUsecase) *chi.Mux {
	r := chi.NewRouter()
	handler := NewEcosystemHandler(uc, zap.NewNop())
	r.Route("/ecosystem", handler.RegisterRoutes)
	return r
}

func TestEcosystem_GetTools(t *testing.T) {
	uc := &mockEcosystemUsecase{
		tools: []ecosystem.DetectedTool{
			{ID: "1", Name: "ArgoCD", Category: ecosystem.CategoryGitOps, Health: ecosystem.HealthHealthy, Status: ecosystem.StatusDetected},
			{ID: "2", Name: "Trivy", Category: ecosystem.CategorySecurity, Health: ecosystem.HealthHealthy, Status: ecosystem.StatusDetected},
		},
	}

	router := setupEcosystemRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/ecosystem", nil)
	req = req.WithContext(context.WithValue(req.Context(), tenancy.TenantIDKey, "tenant-test"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var res []ecosystem.DetectedTool
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(res) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(res))
	}
}

func TestEcosystem_GetTools_FilterCategory(t *testing.T) {
	uc := &mockEcosystemUsecase{
		tools: []ecosystem.DetectedTool{
			{ID: "1", Name: "ArgoCD", Category: ecosystem.CategoryGitOps, Health: ecosystem.HealthHealthy, Status: ecosystem.StatusDetected},
			{ID: "2", Name: "Trivy", Category: ecosystem.CategorySecurity, Health: ecosystem.HealthHealthy, Status: ecosystem.StatusDetected},
		},
	}

	router := setupEcosystemRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/ecosystem?category=gitops", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var res []ecosystem.DetectedTool
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(res) != 1 || res[0].Name != "ArgoCD" {
		t.Fatalf("expected 1 ArgoCD tool, got %+v", res)
	}
}

func TestEcosystem_GetSummary(t *testing.T) {
	uc := &mockEcosystemUsecase{
		tools: []ecosystem.DetectedTool{
			{ID: "1", Name: "ArgoCD", Category: ecosystem.CategoryGitOps, Health: ecosystem.HealthHealthy},
		},
	}

	router := setupEcosystemRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/ecosystem/summary", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var summary ecosystem.EcosystemSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if summary.Total != 1 || summary.Healthy != 1 {
		t.Errorf("unexpected summary: %+v", summary)
	}
}

func TestEcosystem_TriggerScan(t *testing.T) {
	uc := &mockEcosystemUsecase{
		tools: []ecosystem.DetectedTool{
			{ID: "1", Name: "ArgoCD", Category: ecosystem.CategoryGitOps, Status: ecosystem.StatusDetected, LastChecked: time.Now()},
		},
	}

	router := setupEcosystemRouter(uc)

	req := httptest.NewRequest(http.MethodPost, "/ecosystem/scan", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var res []ecosystem.DetectedTool
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(res) != 1 {
		t.Errorf("expected 1 tool, got %d", len(res))
	}
}

func TestEcosystem_CreateTool(t *testing.T) {
	uc := &mockEcosystemUsecase{}
	router := setupEcosystemRouter(uc)

	body := `{
		"name": "Kyverno",
		"category": "policy",
		"endpoint": "https://kyverno.internal",
		"version": "v1.11.0"
	}`

	req := httptest.NewRequest(http.MethodPost, "/ecosystem/tools", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var res ecosystem.DetectedTool
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if res.Name != "Kyverno" || res.Category != "policy" {
		t.Errorf("unexpected created tool: %+v", res)
	}
}

func TestEcosystem_CreateTool_ValidationFailed(t *testing.T) {
	uc := &mockEcosystemUsecase{}
	router := setupEcosystemRouter(uc)

	body := `{"endpoint": "https://no-name.internal"}`

	req := httptest.NewRequest(http.MethodPost, "/ecosystem/tools", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestEcosystem_DeleteTool(t *testing.T) {
	uc := &mockEcosystemUsecase{
		tools: []ecosystem.DetectedTool{
			{ID: "tool-123", Name: "Istio", Category: ecosystem.CategoryMesh},
		},
	}
	router := setupEcosystemRouter(uc)

	req := httptest.NewRequest(http.MethodDelete, "/ecosystem/tools/tool-123", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}

	// Delete non-existent
	reqNotFound := httptest.NewRequest(http.MethodDelete, "/ecosystem/tools/not-found", nil)
	recNotFound := httptest.NewRecorder()
	router.ServeHTTP(recNotFound, reqNotFound)

	if recNotFound.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 on delete error, got %d", recNotFound.Code)
	}
}
