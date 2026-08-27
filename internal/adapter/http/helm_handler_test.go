package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"helm.sh/helm/v3/pkg/repo"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/audit"
	"github.com/datdt/k8sselfhost/internal/infrastructure/helm"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type mockHelmAuditRepo struct {
	mu      sync.Mutex
	actions []map[string]interface{}
}

func (m *mockHelmAuditRepo) ListFindings(ctx context.Context, status string) ([]audit.AuditFinding, error) {
	return nil, nil
}
func (m *mockHelmAuditRepo) GetFinding(ctx context.Context, id string) (*audit.AuditFinding, error) {
	return nil, nil
}
func (m *mockHelmAuditRepo) ResolveFinding(ctx context.Context, id string) error {
	return nil
}
func (m *mockHelmAuditRepo) RecordRun(ctx context.Context, run *audit.AuditRun) error {
	return nil
}
func (m *mockHelmAuditRepo) GetLastRun(ctx context.Context) (*audit.AuditRun, error) {
	return nil, nil
}
func (m *mockHelmAuditRepo) RecordAction(ctx context.Context, actor, action, targetType, targetID, targetName, result string, details map[string]interface{}, ipAddress, userAgent string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.actions = append(m.actions, map[string]interface{}{
		"actor":      actor,
		"action":     action,
		"targetType": targetType,
		"targetID":   targetID,
		"targetName": targetName,
		"result":     result,
		"details":    details,
	})
	return nil
}

func setupHelmTestHandler(t *testing.T) (*HelmHandler, *mockHelmAuditRepo, chi.Router) {
	t.Helper()
	tempDir := t.TempDir()
	rm := helm.NewReleaseManager(nil, nil, tempDir)
	auditRepo := &mockHelmAuditRepo{}
	h := NewHelmHandler(rm, auditRepo)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	return h, auditRepo, r
}

func TestHelmHandler_ReposEndpoints(t *testing.T) {
	_, _, r := setupHelmTestHandler(t)
	ctx := context.WithValue(context.Background(), tenancy.UserRoleKey, "platform_admin")

	// 1. GET /repos - empty
	req := httptest.NewRequest(http.MethodGet, "/repos", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	var repos []*repo.Entry
	if err := json.Unmarshal(w.Body.Bytes(), &repos); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(repos) != 0 {
		t.Fatalf("expected 0 repos, got %d", len(repos))
	}

	// 2. POST /repos - invalid payload
	req = httptest.NewRequest(http.MethodPost, "/repos", bytes.NewBufferString(`{"name":""}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for empty name, got %d", w.Code)
	}

	// 3. PUT /repos - update empty repo list succeeds
	req = httptest.NewRequest(http.MethodPut, "/repos", nil).WithContext(ctx)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 on update repos, got %d", w.Code)
	}
}

func TestHelmHandler_ChartsSearch(t *testing.T) {
	tempDir := t.TempDir()
	rm := helm.NewReleaseManager(nil, nil, tempDir)

	// Create mock repo and cache index
	repoFile := filepath.Join(tempDir, "repository", "repositories.yaml")
	rf := repo.NewFile()
	rf.Add(&repo.Entry{
		Name: "testrepo",
		URL:  "https://example.com/charts",
	})
	_ = rf.WriteFile(repoFile, 0644)

	mockIndexYAML := `apiVersion: v1
entries:
  web-app:
  - apiVersion: v2
    appVersion: 1.0.0
    description: Web Application
    name: web-app
    version: 0.1.0
`
	indexPath := filepath.Join(tempDir, "cache", "testrepo-index.yaml")
	_ = os.WriteFile(indexPath, []byte(mockIndexYAML), 0644)

	auditRepo := &mockHelmAuditRepo{}
	h := NewHelmHandler(rm, auditRepo)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/charts?keyword=web", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHelmHandler_ReleaseOperationsValidationAndAudit(t *testing.T) {
	_, auditRepo, r := setupHelmTestHandler(t)

	ctx := context.WithValue(context.Background(), tenancy.UserIDKey, "user-123")
	ctx = context.WithValue(ctx, tenancy.UserRoleKey, "platform_admin")

	// 1. POST /local/releases - empty body
	req := httptest.NewRequest(http.MethodPost, "/local/releases", bytes.NewBufferString(`{}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty release install, got %d", w.Code)
	}

	// 2. POST /local/releases - missing chart
	req = httptest.NewRequest(http.MethodPost, "/local/releases", bytes.NewBufferString(`{"releaseName":"my-release"}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing chart, got %d", w.Code)
	}

	// 3. POST /local/releases - valid payload but cluster disconnected
	req = httptest.NewRequest(http.MethodPost, "/local/releases", bytes.NewBufferString(`{"releaseName":"my-release","chart":"nginx","namespace":"default"}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError && w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 500 or 503 for disconnected cluster, got %d", w.Code)
	}

	// Verify audit was recorded
	auditRepo.mu.Lock()
	if len(auditRepo.actions) == 0 {
		t.Fatal("expected audit log entry recorded on install attempt")
	}
	lastAction := auditRepo.actions[len(auditRepo.actions)-1]
	auditRepo.mu.Unlock()

	if lastAction["action"] != "install" || lastAction["targetName"] != "my-release" {
		t.Fatalf("unexpected audit entry: %+v", lastAction)
	}

	// 4. PUT /local/releases/my-release - upgrade
	req = httptest.NewRequest(http.MethodPut, "/local/releases/my-release", bytes.NewBufferString(`{"chart":"nginx","namespace":"default"}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError && w.Code != http.StatusServiceUnavailable && w.Code != http.StatusNotFound {
		t.Fatalf("expected error status for disconnected cluster, got %d", w.Code)
	}

	// 5. POST /local/releases/my-release/rollback - rollback
	req = httptest.NewRequest(http.MethodPost, "/local/releases/my-release/rollback", bytes.NewBufferString(`{"revision":1}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError && w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected error status for disconnected cluster, got %d", w.Code)
	}

	// 6. DELETE /local/releases/my-release - uninstall
	req = httptest.NewRequest(http.MethodDelete, "/local/releases/my-release", nil).WithContext(ctx)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError && w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected error status for disconnected cluster, got %d", w.Code)
	}
}

func TestHelmHandler_RBACProtection(t *testing.T) {
	tempDir := t.TempDir()
	rm := helm.NewReleaseManager(nil, nil, tempDir)
	auditRepo := &mockHelmAuditRepo{}
	h := NewHelmHandler(rm, auditRepo)

	r := chi.NewRouter()
	r.With(middleware.RequireRolesForMutations("platform_admin", "tenant_admin")).Route("/helm", h.RegisterRoutes)

	// GET is allowed for any authenticated role
	ctxGuest := context.WithValue(context.Background(), tenancy.UserRoleKey, "guest")
	req := httptest.NewRequest(http.MethodGet, "/helm/repos", nil).WithContext(ctxGuest)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET with guest role, got %d", w.Code)
	}

	// POST /helm/repos is forbidden for guest role
	req = httptest.NewRequest(http.MethodPost, "/helm/repos", bytes.NewBufferString(`{"name":"test","url":"https://example.com"}`)).WithContext(ctxGuest)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for POST with guest role, got %d", w.Code)
	}

	// POST /helm/repos is allowed for platform_admin
	ctxAdmin := context.WithValue(context.Background(), tenancy.UserRoleKey, "platform_admin")
	req = httptest.NewRequest(http.MethodPost, "/helm/repos", bytes.NewBufferString(`{"name":"","url":""}`)).WithContext(ctxAdmin)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Passes RBAC and reaches handler validation -> 400 Bad Request
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body from admin, got %d", w.Code)
	}
}
