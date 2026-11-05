package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
)

type catalogFakeLoggingService struct {
	allServices []string
	byNode      map[string][]string
	err         error
}

func (c *catalogFakeLoggingService) ListServicesCatalog(ctx context.Context) ([]string, map[string][]string, error) {
	if c.err != nil {
		return nil, nil, c.err
	}
	return c.allServices, c.byNode, nil
}

func (c *catalogFakeLoggingService) ListServices(ctx context.Context) ([]string, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.allServices, nil
}

func (c *catalogFakeLoggingService) Ingest(ctx context.Context, entries []logging.LogEntry) error {
	return nil
}

func (c *catalogFakeLoggingService) QueryLogs(ctx context.Context, filter logging.LogFilter) (*logging.LogSearchResult, error) {
	return &logging.LogSearchResult{}, nil
}

func (c *catalogFakeLoggingService) GetHistogram(ctx context.Context, filter logging.LogFilter, intervalSeconds int) ([]logging.LogAggregationBucket, error) {
	return []logging.LogAggregationBucket{}, nil
}

func (c *catalogFakeLoggingService) TailLogs(ctx context.Context, filter logging.LogFilter) (<-chan logging.LogEntry, error) {
	ch := make(chan logging.LogEntry)
	close(ch)
	return ch, nil
}

func (c *catalogFakeLoggingService) QuerySurroundingContext(ctx context.Context, service string, timestamp time.Time, window int) ([]logging.LogEntry, error) {
	return []logging.LogEntry{}, nil
}

func setupTestLogRouter(handler *LogHandler) http.Handler {
	platform := &PlatformHandlers{CentralizedLogs: handler}
	return NewRouterWithWS(health.NewHandler(5*time.Second), nil, platform)
}

func TestLogHandler_ServicesRoute_CatalogAndNodeFiltering(t *testing.T) {
	fake := &catalogFakeLoggingService{
		allServices: []string{"auth-svc", "cart-svc", "cron.service", "k8s-agent.service", "kubelet.service"},
		byNode: map[string][]string{
			"k8smater133":  {"auth-svc", "cart-svc"},
			"k8sworker":    {"cron.service", "k8s-agent.service", "kubelet.service"},
			"k8smasterdeb": {},
		},
	}

	h := NewLogHandler(fake)
	router := setupTestLogRouter(h)
	token, _ := middleware.GenerateAccessToken("user-test", "viewer", "tenant-1")

	type servicesResp struct {
		Services []string            `json:"services"`
		Targets  []string            `json:"targets"`
		ByNode   map[string][]string `json:"by_node"`
	}

	// 1. GET /api/v1/logs/services without node filter -> all services and full by_node
	t.Run("All services when no node specified", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/services", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp servicesResp
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed decoding json: %v", err)
		}

		if resp.Services == nil || resp.Targets == nil || resp.ByNode == nil {
			t.Fatalf("expected non-nil fields in response, got services=%v, targets=%v, by_node=%v", resp.Services, resp.Targets, resp.ByNode)
		}

		if len(resp.Services) != 5 {
			t.Errorf("expected 5 services, got %d: %v", len(resp.Services), resp.Services)
		}
		if len(resp.Targets) != 5 {
			t.Errorf("expected 5 targets, got %d: %v", len(resp.Targets), resp.Targets)
		}
		if len(resp.ByNode) != 3 {
			t.Errorf("expected 3 nodes in by_node, got %d: %v", len(resp.ByNode), resp.ByNode)
		}
		if resp.ByNode["k8smasterdeb"] == nil {
			t.Errorf("expected non-nil slice for offline node k8smasterdeb in by_node")
		}
	})

	// 2. GET /api/v1/logs/services?node=k8sworker -> scoped to k8sworker
	t.Run("Scoped services for k8sworker", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/services?node=k8sworker", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp servicesResp
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed decoding json: %v", err)
		}

		if len(resp.Services) != 3 {
			t.Fatalf("expected 3 services for k8sworker, got %d: %v", len(resp.Services), resp.Services)
		}
		if resp.Services[0] != "cron.service" || resp.Services[1] != "k8s-agent.service" || resp.Services[2] != "kubelet.service" {
			t.Errorf("unexpected services for k8sworker: %v", resp.Services)
		}
		if len(resp.Targets) != 3 {
			t.Errorf("expected 3 targets for k8sworker, got %d: %v", len(resp.Targets), resp.Targets)
		}
		if len(resp.ByNode) != 3 {
			t.Errorf("expected full by_node catalog retained, got %d", len(resp.ByNode))
		}
	})

	// 3. GET /api/v1/logs/services?node=k8smasterdeb (offline host) -> empty slice fast, no leak
	t.Run("Offline host returns empty array fast without leak", func(t *testing.T) {
		start := time.Now()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/services?node=k8smasterdeb", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		duration := time.Since(start)

		if duration > 50*time.Millisecond {
			t.Errorf("offline node query took too long (%v)", duration)
		}

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp servicesResp
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed decoding json: %v", err)
		}

		if resp.Services == nil {
			t.Fatalf("expected non-nil empty slice for offline node, got nil")
		}
		if len(resp.Services) != 0 {
			t.Fatalf("expected 0 services for offline node, got %d: %v (leaked!)", len(resp.Services), resp.Services)
		}
		if resp.Targets == nil || len(resp.Targets) != 0 {
			t.Fatalf("expected 0 targets for offline node, got %d", len(resp.Targets))
		}
	})

	// 4. GET /api/v1/logs/services?node=all -> returns merged
	t.Run("Explicit node=all returns all services", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/services?node=all", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp servicesResp
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed decoding json: %v", err)
		}

		if len(resp.Services) != 5 {
			t.Errorf("expected 5 services for node=all, got %d", len(resp.Services))
		}
	})

	// 5. GET /api/v1/logs/targets?node=k8sworker -> verifies /targets alias
	t.Run("Targets route alias supports node param", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/targets?node=k8sworker", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp servicesResp
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed decoding json: %v", err)
		}

		if len(resp.Targets) != 3 || len(resp.Services) != 3 {
			t.Errorf("expected 3 targets/services for /targets?node=k8sworker, got %d/%d", len(resp.Targets), len(resp.Services))
		}
		if len(resp.ByNode) != 3 {
			t.Errorf("expected by_node returned on /targets route")
		}
	})
}
