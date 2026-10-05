package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/gorilla/websocket"
	"strings"
	"time"
)

func TestLogHandler_SEC21_TenantIsolation(t *testing.T) {
	t.Run("non-admin forged X-Tenant-ID on ingest uses context tenant", func(t *testing.T) {
		fake := &fakeLoggingService{}
		h := NewLogHandler(fake)

		entry := logging.LogEntry{
			ClusterID: "cluster-1",
			Message:   "audit log payload",
		}
		body, _ := json.Marshal(entry)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/logs/ingest", bytes.NewReader(body))
		ctx := withTenantContext(req.Context(), "tenant-real", "tenant_admin")
		req = req.WithContext(ctx)
		req.Header.Set("X-Tenant-ID", "tenant-attacker")

		w := httptest.NewRecorder()
		h.HandleIngest(w, req)

		if w.Code != http.StatusAccepted {
			t.Fatalf("expected 202 Accepted, got %d", w.Code)
		}

		fake.mu.RLock()
		defer fake.mu.RUnlock()
		if len(fake.ingested) != 1 {
			t.Fatalf("expected 1 ingested entry, got %d", len(fake.ingested))
		}
		if fake.ingested[0].TenantID != "tenant-real" {
			t.Fatalf("expected TenantID 'tenant-real', got %q", fake.ingested[0].TenantID)
		}
	})

	t.Run("non-admin with empty context tenant and forged header is rejected", func(t *testing.T) {
		fake := &fakeLoggingService{}
		h := NewLogHandler(fake)

		entry := logging.LogEntry{
			ClusterID: "cluster-1",
			Message:   "audit log payload",
		}
		body, _ := json.Marshal(entry)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/logs/ingest", bytes.NewReader(body))
		ctx := withTenantContext(req.Context(), "", "viewer")
		req = req.WithContext(ctx)
		req.Header.Set("X-Tenant-ID", "tenant-attacker")

		w := httptest.NewRecorder()
		h.HandleIngest(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized for non-admin with empty context tenant, got %d", w.Code)
		}
	})

	t.Run("platform_admin honors X-Tenant-ID on ingest", func(t *testing.T) {
		fake := &fakeLoggingService{}
		h := NewLogHandler(fake)

		entry := logging.LogEntry{
			ClusterID: "cluster-1",
			Message:   "admin log payload",
		}
		body, _ := json.Marshal(entry)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/logs/ingest", bytes.NewReader(body))
		ctx := withTenantContext(req.Context(), "admin-tenant", "platform_admin")
		req = req.WithContext(ctx)
		req.Header.Set("X-Tenant-ID", "override-tenant")

		w := httptest.NewRecorder()
		h.HandleIngest(w, req)

		if w.Code != http.StatusAccepted {
			t.Fatalf("expected 202 Accepted, got %d", w.Code)
		}

		fake.mu.RLock()
		defer fake.mu.RUnlock()
		if len(fake.ingested) != 1 {
			t.Fatalf("expected 1 ingested entry, got %d", len(fake.ingested))
		}
		if fake.ingested[0].TenantID != "override-tenant" {
			t.Fatalf("expected TenantID 'override-tenant', got %q", fake.ingested[0].TenantID)
		}
	})

	t.Run("non-admin forged X-Tenant-ID on search uses context tenant", func(t *testing.T) {
		fake := &fakeLoggingService{
			searchResult: &logging.LogSearchResult{
				Entries:    []logging.LogEntry{},
				TotalCount: 0,
			},
		}
		h := NewLogHandler(fake)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/search?q=test", nil)
		ctx := withTenantContext(req.Context(), "tenant-real", "tenant_admin")
		req = req.WithContext(ctx)
		req.Header.Set("X-Tenant-ID", "tenant-attacker")

		w := httptest.NewRecorder()
		h.HandleSearch(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		fake.mu.RLock()
		defer fake.mu.RUnlock()
		if fake.lastFilter.TenantID != "tenant-real" {
			t.Fatalf("expected search filter TenantID 'tenant-real', got %q", fake.lastFilter.TenantID)
		}
	})

	t.Run("platform_admin honors X-Tenant-ID on search", func(t *testing.T) {
		fake := &fakeLoggingService{
			searchResult: &logging.LogSearchResult{
				Entries:    []logging.LogEntry{},
				TotalCount: 0,
			},
		}
		h := NewLogHandler(fake)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/search?q=test", nil)
		ctx := withTenantContext(req.Context(), "admin-tenant", "platform_admin")
		req = req.WithContext(ctx)
		req.Header.Set("X-Tenant-ID", "override-tenant")

		w := httptest.NewRecorder()
		h.HandleSearch(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		fake.mu.RLock()
		defer fake.mu.RUnlock()
		if fake.lastFilter.TenantID != "override-tenant" {
			t.Fatalf("expected search filter TenantID 'override-tenant', got %q", fake.lastFilter.TenantID)
		}
	})

	t.Run("non-admin forged X-Tenant-ID on stream uses context tenant", func(t *testing.T) {
		fake := &fakeLoggingService{
			tailChan: make(chan logging.LogEntry, 5),
		}
		h := NewLogHandler(fake)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.HandleStream(w, r)
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?namespace=prod"
		header := http.Header{}
		header.Set("Origin", server.URL)
		header.Set("X-Tenant-ID", "tenant-attacker")

		// Create dialer and hook request context
		dialer := websocket.DefaultDialer
		req, _ := http.NewRequest(http.MethodGet, wsURL, nil)
		req.Header = header

		serverHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := withTenantContext(r.Context(), "tenant-real", "tenant_admin")
			h.HandleStream(w, r.WithContext(ctx))
		})
		s := httptest.NewServer(serverHandler)
		defer s.Close()

		targetURL := "ws" + strings.TrimPrefix(s.URL, "http") + "?namespace=prod"
		conn, _, err := dialer.Dial(targetURL, header)
		if err != nil {
			t.Fatalf("dial failed: %v", err)
		}
		defer conn.Close()

		time.Sleep(50 * time.Millisecond)
		fake.mu.RLock()
		defer fake.mu.RUnlock()
		if fake.lastFilter.TenantID != "tenant-real" {
			t.Fatalf("expected stream filter TenantID 'tenant-real', got %q", fake.lastFilter.TenantID)
		}
	})
}
