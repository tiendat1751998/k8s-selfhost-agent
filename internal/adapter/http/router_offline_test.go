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

func TestRouter_OfflineGracefulK8s(t *testing.T) {
	healthHandler := health.NewHandler(5 * time.Second)

	// Test with nil platform handlers (simulating offline K8s / standalone without K8s)
	router := NewRouterWithWS(healthHandler, nil, nil)
	token, err := middleware.GenerateJWT("test-user-id", "platform_admin", "")
	if err != nil {
		t.Fatalf("failed to generate JWT token: %v", err)
	}

	testCases := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/explorer"},
		{"POST", "/api/v1/explorer/sync"},
		{"GET", "/api/v1/deployments"},
		{"POST", "/api/v1/deployments"},
		{"GET", "/api/v1/capacity"},
		{"GET", "/api/v1/health"},
		{"POST", "/api/v1/health/ping"},
		{"GET", "/api/v1/k8s/local/namespaces"},
		{"POST", "/api/v1/k8s/local/namespaces"},
		{"GET", "/api/v1/k8s/local/resources/configmaps"},
		{"POST", "/api/v1/k8s/local/apply"},
	}

	for _, tc := range testCases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("[%s %s] expected status 503, got %d", tc.method, tc.path, rec.Code)
		}

		var resp map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Errorf("[%s %s] failed to decode json response: %v", tc.method, tc.path, err)
		}
		if resp["error"] != "kubernetes not connected" {
			t.Errorf("[%s %s] expected error 'kubernetes not connected', got '%s'", tc.method, tc.path, resp["error"])
		}
		if resp["message"] != "Import a kubeconfig via Fleet to enable this feature" {
			t.Errorf("[%s %s] expected message 'Import a kubeconfig via Fleet to enable this feature', got '%s'", tc.method, tc.path, resp["message"])
		}
	}
}
