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
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/settings"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type mockSettingsRepo struct {
	items   map[string]settings.Setting
	failErr error
}

func newMockSettingsRepo() *mockSettingsRepo {
	return &mockSettingsRepo{
		items: make(map[string]settings.Setting),
	}
}

func (m *mockSettingsRepo) key(tenantID, category, key string) string {
	if tenantID == "" {
		tenantID = "default-tenant"
	}
	return tenantID + ":" + category + ":" + key
}

func (m *mockSettingsRepo) GetByCategory(ctx context.Context, tenantID, category string) ([]settings.Setting, error) {
	if m.failErr != nil {
		return nil, m.failErr
	}
	if tenantID == "" {
		tenantID = "default-tenant"
	}
	result := make([]settings.Setting, 0)
	for _, s := range m.items {
		if s.TenantID == tenantID && s.Category == category {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockSettingsRepo) GetByKey(ctx context.Context, tenantID, category, key string) (*settings.Setting, error) {
	if m.failErr != nil {
		return nil, m.failErr
	}
	k := m.key(tenantID, category, key)
	s, ok := m.items[k]
	if !ok {
		return nil, nil
	}
	copied := s
	return &copied, nil
}

func (m *mockSettingsRepo) Upsert(ctx context.Context, setting *settings.Setting) error {
	if m.failErr != nil {
		return m.failErr
	}
	if setting == nil {
		return errors.New("setting cannot be nil")
	}
	if setting.ID == "" {
		setting.ID = uuid.NewString()
	}
	if setting.TenantID == "" {
		setting.TenantID = "default-tenant"
	}
	setting.UpdatedAt = time.Now().UTC()
	k := m.key(setting.TenantID, setting.Category, setting.Key)
	m.items[k] = *setting
	return nil
}

func (m *mockSettingsRepo) BulkUpsert(ctx context.Context, items []settings.Setting) error {
	if m.failErr != nil {
		return m.failErr
	}
	for i := range items {
		if err := m.Upsert(ctx, &items[i]); err != nil {
			return err
		}
	}
	return nil
}

func (m *mockSettingsRepo) GetAll(ctx context.Context, tenantID string) ([]settings.Setting, error) {
	if m.failErr != nil {
		return nil, m.failErr
	}
	if tenantID == "" {
		tenantID = "default-tenant"
	}
	result := make([]settings.Setting, 0)
	for _, s := range m.items {
		if s.TenantID == tenantID {
			result = append(result, s)
		}
	}
	return result, nil
}

func setupSettingsTestRouter(repo settings.Repository) *chi.Mux {
	r := chi.NewRouter()
	handler := NewSettingsHandler(repo, zap.NewNop())
	r.Route("/settings", handler.RegisterRoutes)
	return r
}

func TestSettings_GetAll_Empty(t *testing.T) {
	repo := newMockSettingsRepo()
	router := setupSettingsTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	ctx := context.WithValue(req.Context(), tenancy.TenantIDKey, "test-tenant")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var items []settings.Setting
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("failed to unmarshal response: %v, body: %s", err, rec.Body.String())
	}

	if len(items) != 0 {
		t.Errorf("expected empty array, got %d items", len(items))
	}
}

func TestSettings_GetByCategory(t *testing.T) {
	repo := newMockSettingsRepo()
	_ = repo.Upsert(context.Background(), &settings.Setting{
		Category: settings.CategoryPlatform,
		Key:      "name",
		Value:    "PlatformName",
		TenantID: "test-tenant",
	})
	_ = repo.Upsert(context.Background(), &settings.Setting{
		Category: settings.CategorySecurity,
		Key:      "require_2fa",
		Value:    "true",
		TenantID: "test-tenant",
	})

	router := setupSettingsTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/settings?category=platform", nil)
	ctx := context.WithValue(req.Context(), tenancy.TenantIDKey, "test-tenant")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var items []settings.Setting
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Category != settings.CategoryPlatform || items[0].Key != "name" || items[0].Value != "PlatformName" {
		t.Errorf("unexpected item: %+v", items[0])
	}
}

func TestSettings_UpdateBulk(t *testing.T) {
	repo := newMockSettingsRepo()
	router := setupSettingsTestRouter(repo)

	body := map[string]interface{}{
		"settings": []map[string]interface{}{
			{
				"category": "platform",
				"key":      "name",
				"value":    "My Platform",
			},
			{
				"category": "security",
				"key":      "session_timeout_minutes",
				"value":    "120",
			},
		},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/settings", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), tenancy.TenantIDKey, "test-tenant")
	ctx = context.WithValue(ctx, tenancy.UserIDKey, "user-123")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated []settings.Setting
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(updated) != 2 {
		t.Fatalf("expected 2 updated settings, got %d", len(updated))
	}

	// Verify items stored in repo
	saved, err := repo.GetAll(context.Background(), "test-tenant")
	if err != nil {
		t.Fatalf("failed to get all from repo: %v", err)
	}
	if len(saved) != 2 {
		t.Fatalf("expected 2 items in repo, got %d", len(saved))
	}

	for _, s := range saved {
		if s.TenantID != "test-tenant" {
			t.Errorf("expected tenant_id 'test-tenant', got '%s'", s.TenantID)
		}
		if s.UpdatedBy != "user-123" {
			t.Errorf("expected updated_by 'user-123', got '%s'", s.UpdatedBy)
		}
	}
}

func TestSettings_UpdateBulk_BadRequest(t *testing.T) {
	repo := newMockSettingsRepo()
	router := setupSettingsTestRouter(repo)

	req := httptest.NewRequest(http.MethodPut, "/settings", bytes.NewBufferString("{invalid json"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSettings_GetDefaults(t *testing.T) {
	repo := newMockSettingsRepo()
	router := setupSettingsTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/settings/defaults", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var defaults map[string]map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &defaults); err != nil {
		t.Fatalf("failed to unmarshal defaults: %v", err)
	}

	if len(defaults) == 0 {
		t.Fatalf("expected non-empty defaults map")
	}
	if defaults[settings.CategoryPlatform]["name"] != "K8s Self-Host Platform" {
		t.Errorf("expected default platform name 'K8s Self-Host Platform', got '%s'", defaults[settings.CategoryPlatform]["name"])
	}
}

func TestSettings_TestIntegration_Unreachable(t *testing.T) {
	repo := newMockSettingsRepo()
	router := setupSettingsTestRouter(repo)

	body := map[string]string{
		"url": "http://127.0.0.1:59999/unreachable-endpoint-test",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/settings/integrations/test", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Reachable  bool  `json:"reachable"`
		StatusCode int   `json:"status_code"`
		LatencyMS  int64 `json:"latency_ms"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Reachable {
		t.Errorf("expected reachable false, got true")
	}
	if resp.StatusCode != 0 {
		t.Errorf("expected status code 0, got %d", resp.StatusCode)
	}
}

func TestSettings_TestIntegration_Reachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	repo := newMockSettingsRepo()
	router := setupSettingsTestRouter(repo)

	body := map[string]string{
		"url": server.URL,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/settings/integrations/test", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Reachable  bool  `json:"reachable"`
		StatusCode int   `json:"status_code"`
		LatencyMS  int64 `json:"latency_ms"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Reachable {
		t.Errorf("expected reachable true, got false")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}
}
