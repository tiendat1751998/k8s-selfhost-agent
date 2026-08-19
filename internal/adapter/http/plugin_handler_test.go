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
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/plugin"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type mockPluginRepo struct {
	plugins map[string]*plugin.Plugin
	failErr error
}

func newMockPluginRepo() *mockPluginRepo {
	return &mockPluginRepo{
		plugins: make(map[string]*plugin.Plugin),
	}
}

func (m *mockPluginRepo) Create(ctx context.Context, p *plugin.Plugin) error {
	if m.failErr != nil {
		return m.failErr
	}
	for _, existing := range m.plugins {
		if existing.Name == p.Name && existing.TenantID == p.TenantID {
			return plugin.ErrDuplicateName
		}
	}
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	p.UpdatedAt = time.Now().UTC()
	cp := *p
	m.plugins[p.ID] = &cp
	return nil
}

func (m *mockPluginRepo) GetByID(ctx context.Context, id string) (*plugin.Plugin, error) {
	if m.failErr != nil {
		return nil, m.failErr
	}
	p, ok := m.plugins[id]
	if !ok {
		return nil, nil
	}
	cp := *p
	return &cp, nil
}

func (m *mockPluginRepo) List(ctx context.Context, tenantID string, enabledOnly bool) ([]plugin.Plugin, error) {
	if m.failErr != nil {
		return nil, m.failErr
	}
	list := make([]plugin.Plugin, 0)
	for _, p := range m.plugins {
		if enabledOnly && !p.Enabled {
			continue
		}
		list = append(list, *p)
	}
	return list, nil
}

func (m *mockPluginRepo) Update(ctx context.Context, p *plugin.Plugin) error {
	if m.failErr != nil {
		return m.failErr
	}
	if _, ok := m.plugins[p.ID]; !ok {
		return plugin.ErrNotFound
	}
	p.UpdatedAt = time.Now().UTC()
	cp := *p
	m.plugins[p.ID] = &cp
	return nil
}

func (m *mockPluginRepo) Delete(ctx context.Context, id string) error {
	if m.failErr != nil {
		return m.failErr
	}
	if _, ok := m.plugins[id]; !ok {
		return plugin.ErrNotFound
	}
	delete(m.plugins, id)
	return nil
}

func (m *mockPluginRepo) Toggle(ctx context.Context, id string, enabled bool) error {
	if m.failErr != nil {
		return m.failErr
	}
	p, ok := m.plugins[id]
	if !ok {
		return plugin.ErrNotFound
	}
	p.Enabled = enabled
	p.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *mockPluginRepo) GetStats(ctx context.Context, tenantID string) (*plugin.PluginStats, error) {
	if m.failErr != nil {
		return nil, m.failErr
	}
	stats := &plugin.PluginStats{
		ByCategory: make(map[string]int),
	}
	for _, p := range m.plugins {
		stats.Total++
		if p.Enabled {
			stats.Enabled++
		} else {
			stats.Disabled++
		}
		stats.ByCategory[p.Category]++
	}
	return stats, nil
}

func setupPluginTestRouter(repo plugin.Repository) (*chi.Mux, *PluginHandler) {
	r := chi.NewRouter()
	handler := NewPluginHandler(repo, zap.NewNop())
	r.Route("/plugins", handler.RegisterRoutes)
	return r, handler
}

func TestPluginHandler_Register_Success(t *testing.T) {
	repo := newMockPluginRepo()
	router, _ := setupPluginTestRouter(repo)

	body := map[string]any{
		"name":        "cost-explorer",
		"version":     "1.0.0",
		"description": "Interactive FinOps Visualizer",
		"category":    "monitoring",
		"entry_point": "https://cdn.example.com/cost.js",
		"permissions": []string{"cost:read"},
		"config":      map[string]string{"currency": "USD"},
	}
	raw, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/plugins", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var resp plugin.Plugin
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID == "" {
		t.Errorf("expected generated ID")
	}
	if resp.Name != "cost-explorer" {
		t.Errorf("expected name cost-explorer, got %q", resp.Name)
	}
	if !resp.Enabled {
		t.Errorf("expected default enabled=true")
	}
}

func TestPluginHandler_Register_ValidationError(t *testing.T) {
	repo := newMockPluginRepo()
	router, _ := setupPluginTestRouter(repo)

	body := map[string]any{
		"name":     "",
		"category": "invalid-category",
	}
	raw, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/plugins", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPluginHandler_Register_DuplicateConflict(t *testing.T) {
	repo := newMockPluginRepo()
	router, _ := setupPluginTestRouter(repo)

	_ = repo.Create(context.Background(), &plugin.Plugin{
		Name:     "network-topology",
		TenantID: "default-tenant",
	})

	body := map[string]any{
		"name":     "network-topology",
		"category": "devtools",
	}
	raw, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/plugins", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409 Conflict, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPluginHandler_List_Success(t *testing.T) {
	repo := newMockPluginRepo()
	router, _ := setupPluginTestRouter(repo)

	_ = repo.Create(context.Background(), &plugin.Plugin{Name: "p1", Enabled: true})
	_ = repo.Create(context.Background(), &plugin.Plugin{Name: "p2", Enabled: false})

	req := httptest.NewRequest(http.MethodGet, "/plugins", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var list []plugin.Plugin
	_ = json.NewDecoder(w.Body).Decode(&list)
	if len(list) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(list))
	}
}

func TestPluginHandler_List_EnabledFilter(t *testing.T) {
	repo := newMockPluginRepo()
	router, _ := setupPluginTestRouter(repo)

	_ = repo.Create(context.Background(), &plugin.Plugin{Name: "p1", Enabled: true})
	_ = repo.Create(context.Background(), &plugin.Plugin{Name: "p2", Enabled: false})

	req := httptest.NewRequest(http.MethodGet, "/plugins?enabled=true", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var list []plugin.Plugin
	_ = json.NewDecoder(w.Body).Decode(&list)
	if len(list) != 1 {
		t.Errorf("expected 1 enabled plugin, got %d", len(list))
	}
}

func TestPluginHandler_GetByID(t *testing.T) {
	repo := newMockPluginRepo()
	router, _ := setupPluginTestRouter(repo)

	targetID := uuid.NewString()
	_ = repo.Create(context.Background(), &plugin.Plugin{ID: targetID, Name: "lookup-plugin"})

	// Success
	req := httptest.NewRequest(http.MethodGet, "/plugins/"+targetID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Not Found
	nonExistentID := uuid.NewString()
	reqNF := httptest.NewRequest(http.MethodGet, "/plugins/"+nonExistentID, nil)
	wNF := httptest.NewRecorder()
	router.ServeHTTP(wNF, reqNF)

	if wNF.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", wNF.Code)
	}
}

func TestPluginHandler_Update(t *testing.T) {
	repo := newMockPluginRepo()
	router, _ := setupPluginTestRouter(repo)

	targetID := uuid.NewString()
	_ = repo.Create(context.Background(), &plugin.Plugin{ID: targetID, Name: "initial-name", Version: "1.0.0"})

	body := map[string]any{
		"name":        "updated-name",
		"version":     "2.0.0",
		"description": "new description",
		"category":    "security",
	}
	raw, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/plugins/"+targetID, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated plugin.Plugin
	_ = json.NewDecoder(w.Body).Decode(&updated)
	if updated.Name != "updated-name" || updated.Version != "2.0.0" {
		t.Errorf("expected updated fields, got %+v", updated)
	}
}

func TestPluginHandler_Delete(t *testing.T) {
	repo := newMockPluginRepo()
	router, _ := setupPluginTestRouter(repo)

	targetID := uuid.NewString()
	_ = repo.Create(context.Background(), &plugin.Plugin{ID: targetID, Name: "delete-me"})

	req := httptest.NewRequest(http.MethodDelete, "/plugins/"+targetID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content, got %d", w.Code)
	}

	// Verify deleted
	reqCheck := httptest.NewRequest(http.MethodGet, "/plugins/"+targetID, nil)
	wCheck := httptest.NewRecorder()
	router.ServeHTTP(wCheck, reqCheck)
	if wCheck.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after deletion, got %d", wCheck.Code)
	}
}

func TestPluginHandler_Toggle(t *testing.T) {
	repo := newMockPluginRepo()
	router, _ := setupPluginTestRouter(repo)

	targetID := uuid.NewString()
	_ = repo.Create(context.Background(), &plugin.Plugin{ID: targetID, Name: "toggle-me", Enabled: true})

	// Toggle without body (inverts)
	req := httptest.NewRequest(http.MethodPost, "/plugins/"+targetID+"/toggle", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var toggled plugin.Plugin
	_ = json.NewDecoder(w.Body).Decode(&toggled)
	if toggled.Enabled != false {
		t.Errorf("expected enabled=false, got true")
	}

	// Toggle with explicit body
	body := map[string]any{"enabled": true}
	raw, _ := json.Marshal(body)
	reqExplicit := httptest.NewRequest(http.MethodPost, "/plugins/"+targetID+"/toggle", bytes.NewReader(raw))
	reqExplicit.Header.Set("Content-Type", "application/json")
	wExplicit := httptest.NewRecorder()
	router.ServeHTTP(wExplicit, reqExplicit)

	if wExplicit.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", wExplicit.Code)
	}
	var toggled2 plugin.Plugin
	_ = json.NewDecoder(wExplicit.Body).Decode(&toggled2)
	if toggled2.Enabled != true {
		t.Errorf("expected enabled=true, got false")
	}
}

func TestPluginHandler_Stats(t *testing.T) {
	repo := newMockPluginRepo()
	router, _ := setupPluginTestRouter(repo)

	_ = repo.Create(context.Background(), &plugin.Plugin{Name: "p1", Category: "monitoring", Enabled: true})
	_ = repo.Create(context.Background(), &plugin.Plugin{Name: "p2", Category: "security", Enabled: false})

	req := httptest.NewRequest(http.MethodGet, "/plugins/stats", nil)
	req = req.WithContext(context.WithValue(req.Context(), tenancy.TenantIDKey, "default-tenant"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var stats plugin.PluginStats
	_ = json.NewDecoder(w.Body).Decode(&stats)
	if stats.Total != 2 || stats.Enabled != 1 || stats.Disabled != 1 {
		t.Errorf("unexpected stats: %+v", stats)
	}
}
