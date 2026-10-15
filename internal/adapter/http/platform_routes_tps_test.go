package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/usecase/metrics"
)

func TestRegisterPlatformRoutes_MetricsTPS(t *testing.T) {
	collector := metrics.NewCollector(nil, nil, nil, zap.NewNop())
	tpsCollector := metrics.NewTPSCollector(collector, nil, zap.NewNop())
	overviewHandler := NewOverviewHandler(collector, zap.NewNop(), tpsCollector)

	platform := &PlatformHandlers{
		Overview: overviewHandler,
	}

	r := chi.NewRouter()
	RegisterPlatformRoutes(r, platform)

	// 1. Verify GET /metrics/tps
	req := httptest.NewRequest(http.MethodGet, "/metrics/tps", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected GET /metrics/tps status 200, got %d", w.Code)
	}

	var tpsResp metrics.TPSSnapshot
	if err := json.NewDecoder(w.Body).Decode(&tpsResp); err != nil {
		t.Fatalf("failed to decode TPSSnapshot: %v", err)
	}

	// 2. Verify GET /overview
	reqOverview := httptest.NewRequest(http.MethodGet, "/overview", nil)
	wOverview := httptest.NewRecorder()
	r.ServeHTTP(wOverview, reqOverview)

	if wOverview.Code != http.StatusOK {
		t.Fatalf("expected GET /overview status 200, got %d", wOverview.Code)
	}

	// 3. Verify GET /overview/tps parity
	reqOverviewTPS := httptest.NewRequest(http.MethodGet, "/overview/tps", nil)
	wOverviewTPS := httptest.NewRecorder()
	r.ServeHTTP(wOverviewTPS, reqOverviewTPS)

	if wOverviewTPS.Code != http.StatusOK {
		t.Fatalf("expected GET /overview/tps status 200, got %d", wOverviewTPS.Code)
	}
}
