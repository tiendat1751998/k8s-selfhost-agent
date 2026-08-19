package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/catalog"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type mockCatalogRepo struct {
	services     map[string]*catalog.ServiceEntry
	failErr      error
	lastFilter   catalog.ListFilter
	lastTenantID string
}

func newMockCatalogRepo() *mockCatalogRepo {
	return &mockCatalogRepo{
		services: make(map[string]*catalog.ServiceEntry),
	}
}

func (m *mockCatalogRepo) Create(ctx context.Context, entry *catalog.ServiceEntry) error {
	if m.failErr != nil {
		return m.failErr
	}
	if entry.ID == "" {
		entry.ID = "srv-" + uuid.NewString()[:8]
	}
	if entry.Type == "" {
		entry.Type = catalog.TypeService
	}
	if entry.Lifecycle == "" {
		entry.Lifecycle = catalog.LifecycleDevelopment
	}
	if entry.Tags == nil {
		entry.Tags = []string{}
	}
	if entry.Annotations == nil {
		entry.Annotations = map[string]string{}
	}
	now := time.Now().UTC()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now
	m.services[entry.ID] = entry
	return nil
}

func (m *mockCatalogRepo) GetByID(ctx context.Context, id string) (*catalog.ServiceEntry, error) {
	if m.failErr != nil {
		return nil, m.failErr
	}
	srv, ok := m.services[id]
	if !ok {
		return nil, nil
	}
	copied := *srv
	return &copied, nil
}

func (m *mockCatalogRepo) List(ctx context.Context, tenantID string, filter catalog.ListFilter) ([]catalog.ServiceEntry, error) {
	m.lastTenantID = tenantID
	m.lastFilter = filter
	if m.failErr != nil {
		return nil, m.failErr
	}
	list := make([]catalog.ServiceEntry, 0)
	for _, srv := range m.services {
		if filter.Type != "" && srv.Type != filter.Type {
			continue
		}
		if filter.Lifecycle != "" && srv.Lifecycle != filter.Lifecycle {
			continue
		}
		if filter.OwnerTeam != "" && srv.OwnerTeam != filter.OwnerTeam {
			continue
		}
		if filter.Search != "" {
			term := strings.ToLower(filter.Search)
			if !strings.Contains(strings.ToLower(srv.Name), term) && !strings.Contains(strings.ToLower(srv.Description), term) {
				continue
			}
		}
		copied := *srv
		list = append(list, copied)
	}
	return list, nil
}

func (m *mockCatalogRepo) Update(ctx context.Context, entry *catalog.ServiceEntry) error {
	if m.failErr != nil {
		return m.failErr
	}
	if _, ok := m.services[entry.ID]; !ok {
		return errors.New("service catalog entry not found: " + entry.ID)
	}
	entry.UpdatedAt = time.Now().UTC()
	m.services[entry.ID] = entry
	return nil
}

func (m *mockCatalogRepo) Delete(ctx context.Context, id string) error {
	if m.failErr != nil {
		return m.failErr
	}
	if _, ok := m.services[id]; !ok {
		return errors.New("service catalog entry not found: " + id)
	}
	delete(m.services, id)
	return nil
}

func (m *mockCatalogRepo) Stats(ctx context.Context, tenantID string) (*catalog.CatalogStats, error) {
	m.lastTenantID = tenantID
	if m.failErr != nil {
		return nil, m.failErr
	}
	stats := &catalog.CatalogStats{
		Total:       len(m.services),
		ByType:      make(map[string]int),
		ByLifecycle: make(map[string]int),
	}
	for _, srv := range m.services {
		stats.ByType[srv.Type]++
		stats.ByLifecycle[srv.Lifecycle]++
	}
	return stats, nil
}

func setupCatalogTestRouter(repo catalog.Repository) *chi.Mux {
	r := chi.NewRouter()
	handler := NewCatalogHandler(repo, zap.NewNop())
	r.Route("/catalog", handler.RegisterRoutes)
	return r
}

func TestCatalog_CreateService_Success(t *testing.T) {
	repo := newMockCatalogRepo()
	router := setupCatalogTestRouter(repo)

	body := map[string]interface{}{
		"name":        "payment-service",
		"description": "Handles payment processing",
		"type":        catalog.TypeService,
		"lifecycle":   catalog.LifecycleProduction,
		"owner_team":  "billing-team",
		"owner_email": "billing@example.com",
		"repo_url":    "https://github.com/example/payment-service",
		"docs_url":    "https://docs.example.com/payment",
		"tags":        []string{"billing", "payments", "core"},
		"annotations": map[string]string{"k8s/namespace": "prod", "k8s/deployment": "payment-api"},
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/catalog/services", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), tenancy.TenantIDKey, "tenant-test-1")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var created catalog.ServiceEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if created.ID == "" {
		t.Errorf("expected generated service ID in response")
	}
	if created.Name != "payment-service" {
		t.Errorf("expected name 'payment-service', got %q", created.Name)
	}
	if created.Type != catalog.TypeService {
		t.Errorf("expected type %q, got %q", catalog.TypeService, created.Type)
	}
	if created.Lifecycle != catalog.LifecycleProduction {
		t.Errorf("expected lifecycle %q, got %q", catalog.LifecycleProduction, created.Lifecycle)
	}
	if created.TenantID != "tenant-test-1" {
		t.Errorf("expected tenant_id 'tenant-test-1', got %q", created.TenantID)
	}
	if len(created.Tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(created.Tags))
	}
	if created.Annotations["k8s/namespace"] != "prod" {
		t.Errorf("expected annotation k8s/namespace='prod', got %q", created.Annotations["k8s/namespace"])
	}
}

func TestCatalog_CreateService_MissingName(t *testing.T) {
	repo := newMockCatalogRepo()
	router := setupCatalogTestRouter(repo)

	body := map[string]interface{}{
		"name":        "",
		"description": "Missing name",
		"type":        catalog.TypeService,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/catalog/services", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCatalog_CreateService_InvalidType(t *testing.T) {
	repo := newMockCatalogRepo()
	router := setupCatalogTestRouter(repo)

	body := map[string]interface{}{
		"name": "invalid-service",
		"type": "unsupported_type_xyz",
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/catalog/services", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCatalog_ListServices_Empty(t *testing.T) {
	repo := newMockCatalogRepo()
	router := setupCatalogTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/catalog/services", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	bodyStr := strings.TrimSpace(rec.Body.String())
	if bodyStr != "[]" {
		t.Fatalf("expected empty JSON array '[]', got %s", bodyStr)
	}
}

func TestCatalog_ListServices_WithFilters(t *testing.T) {
	repo := newMockCatalogRepo()
	_ = repo.Create(context.Background(), &catalog.ServiceEntry{
		ID:          "srv-1",
		Name:        "payment-api",
		Description: "Payment gateway",
		Type:        catalog.TypeAPI,
		Lifecycle:   catalog.LifecycleProduction,
		OwnerTeam:   "platform",
	})
	_ = repo.Create(context.Background(), &catalog.ServiceEntry{
		ID:          "srv-2",
		Name:        "auth-service",
		Description: "Authentication provider",
		Type:        catalog.TypeService,
		Lifecycle:   catalog.LifecycleProduction,
		OwnerTeam:   "security",
	})

	router := setupCatalogTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/catalog/services?type=api&lifecycle=production&owner_team=platform&search=payment", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	if repo.lastFilter.Type != "api" {
		t.Errorf("expected filter type 'api', got %q", repo.lastFilter.Type)
	}
	if repo.lastFilter.Lifecycle != "production" {
		t.Errorf("expected filter lifecycle 'production', got %q", repo.lastFilter.Lifecycle)
	}
	if repo.lastFilter.OwnerTeam != "platform" {
		t.Errorf("expected filter owner_team 'platform', got %q", repo.lastFilter.OwnerTeam)
	}
	if repo.lastFilter.Search != "payment" {
		t.Errorf("expected filter search 'payment', got %q", repo.lastFilter.Search)
	}

	var results []catalog.ServiceEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "srv-1" {
		t.Errorf("expected srv-1, got %s", results[0].ID)
	}
}

func TestCatalog_GetService_NotFound(t *testing.T) {
	repo := newMockCatalogRepo()
	router := setupCatalogTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/catalog/services/nonexistent-id", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 Not Found, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCatalog_GetService_Success(t *testing.T) {
	repo := newMockCatalogRepo()
	_ = repo.Create(context.Background(), &catalog.ServiceEntry{
		ID:          "srv-100",
		Name:        "order-service",
		Description: "Order management service",
		Type:        catalog.TypeService,
		Lifecycle:   catalog.LifecycleStaging,
		Tags:        []string{"orders"},
		Annotations: map[string]string{"tier": "critical"},
	})

	router := setupCatalogTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/catalog/services/srv-100", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var srv catalog.ServiceEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &srv); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if srv.ID != "srv-100" {
		t.Errorf("expected ID srv-100, got %s", srv.ID)
	}
	if srv.Name != "order-service" {
		t.Errorf("expected name 'order-service', got %s", srv.Name)
	}
}

func TestCatalog_UpdateService_Success(t *testing.T) {
	repo := newMockCatalogRepo()
	_ = repo.Create(context.Background(), &catalog.ServiceEntry{
		ID:          "srv-200",
		Name:        "legacy-service",
		Description: "Old service",
		Type:        catalog.TypeService,
		Lifecycle:   catalog.LifecycleDevelopment,
	})

	router := setupCatalogTestRouter(repo)

	body := map[string]interface{}{
		"name":        "updated-service",
		"description": "Now modernized",
		"type":        catalog.TypeAPI,
		"lifecycle":   catalog.LifecycleProduction,
		"owner_team":  "core-team",
		"tags":        []string{"v2"},
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/catalog/services/srv-200", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated catalog.ServiceEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if updated.Name != "updated-service" {
		t.Errorf("expected name 'updated-service', got %s", updated.Name)
	}
	if updated.Lifecycle != catalog.LifecycleProduction {
		t.Errorf("expected lifecycle 'production', got %s", updated.Lifecycle)
	}
}

func TestCatalog_DeleteService_Success(t *testing.T) {
	repo := newMockCatalogRepo()
	_ = repo.Create(context.Background(), &catalog.ServiceEntry{
		ID:   "srv-300",
		Name: "to-delete",
	})

	router := setupCatalogTestRouter(repo)

	req := httptest.NewRequest(http.MethodDelete, "/catalog/services/srv-300", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204 No Content, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify it was deleted
	if _, ok := repo.services["srv-300"]; ok {
		t.Errorf("expected service to be deleted from repo")
	}
}

func TestCatalog_GetStats(t *testing.T) {
	repo := newMockCatalogRepo()
	_ = repo.Create(context.Background(), &catalog.ServiceEntry{
		ID:        "srv-1",
		Name:      "svc-1",
		Type:      catalog.TypeService,
		Lifecycle: catalog.LifecycleProduction,
	})
	_ = repo.Create(context.Background(), &catalog.ServiceEntry{
		ID:        "srv-2",
		Name:      "svc-2",
		Type:      catalog.TypeAPI,
		Lifecycle: catalog.LifecycleProduction,
	})
	_ = repo.Create(context.Background(), &catalog.ServiceEntry{
		ID:        "srv-3",
		Name:      "svc-3",
		Type:      catalog.TypeDatabase,
		Lifecycle: catalog.LifecycleDevelopment,
	})

	router := setupCatalogTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/catalog/stats", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var stats catalog.CatalogStats
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if stats.Total != 3 {
		t.Errorf("expected total 3, got %d", stats.Total)
	}
	if stats.ByType[catalog.TypeService] != 1 {
		t.Errorf("expected 1 service type, got %d", stats.ByType[catalog.TypeService])
	}
	if stats.ByType[catalog.TypeAPI] != 1 {
		t.Errorf("expected 1 api type, got %d", stats.ByType[catalog.TypeAPI])
	}
	if stats.ByType[catalog.TypeDatabase] != 1 {
		t.Errorf("expected 1 database type, got %d", stats.ByType[catalog.TypeDatabase])
	}
	if stats.ByLifecycle[catalog.LifecycleProduction] != 2 {
		t.Errorf("expected 2 production lifecycle, got %d", stats.ByLifecycle[catalog.LifecycleProduction])
	}
	if stats.ByLifecycle[catalog.LifecycleDevelopment] != 1 {
		t.Errorf("expected 1 development lifecycle, got %d", stats.ByLifecycle[catalog.LifecycleDevelopment])
	}
}
