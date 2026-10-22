package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
)

func TestLogHandler_TargetsRouteAndPayload(t *testing.T) {
	fake := &fakeLoggingService{services: []string{"api-gw", "auth-svc"}}
	h := NewLogHandler(fake)

	platform := &PlatformHandlers{CentralizedLogs: h}
	router := NewRouterWithWS(health.NewHandler(5*time.Second), nil, platform)
	token, _ := middleware.GenerateAccessToken("user-t", "viewer", "tenant-1")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/targets", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/v1/logs/targets, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp struct {
		Services []string `json:"services"`
		Targets  []string `json:"targets"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed decoding json: %v", err)
	}

	if len(resp.Targets) != 2 || resp.Targets[0] != "api-gw" {
		t.Errorf("expected targets to contain api-gw, got %+v", resp.Targets)
	}
	if len(resp.Services) != 2 || resp.Services[0] != "api-gw" {
		t.Errorf("expected services to contain api-gw, got %+v", resp.Services)
	}
}
