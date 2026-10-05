package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestMeshHandler_TrafficSplitReturns501(t *testing.T) {
	handler := NewMeshHandler()
	r := chi.NewRouter()
	r.Route("/mesh", handler.RegisterRoutes)

	testCases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "TrafficSplit POST /mesh/split",
			method: http.MethodPost,
			path:   "/mesh/split",
			body:   `{"service":"order-service","weights":{"v1":80,"v2":20}}`,
		},
		{
			name:   "TrafficSplit POST /mesh/traffic/split",
			method: http.MethodPost,
			path:   "/mesh/traffic/split",
			body:   `{"service":"order-service","weights":{"v1":50,"v2":50}}`,
		},
		{
			name:   "CanaryRelease POST /mesh/canary",
			method: http.MethodPost,
			path:   "/mesh/canary",
			body:   `{"service":"auth-service","step":10}`,
		},
		{
			name:   "BlueGreenSwitch POST /mesh/bluegreen",
			method: http.MethodPost,
			path:   "/mesh/bluegreen",
			body:   `{"service":"auth-service","active":"green"}`,
		},
		{
			name:   "ConfigureRoutes POST /mesh/routes",
			method: http.MethodPost,
			path:   "/mesh/routes",
			body:   `{"routes":[{"path":"/api/v2","destination":"v2"}]}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var bodyReader *strings.Reader
			if tc.body != "" {
				bodyReader = strings.NewReader(tc.body)
			} else {
				bodyReader = strings.NewReader("")
			}

			req := httptest.NewRequest(tc.method, tc.path, bodyReader)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("expected 501 Not Implemented, got %d: %s", rec.Code, rec.Body.String())
			}

			var resp map[string]interface{}
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode json response: %v", err)
			}

			expectedErr := "Service mesh controller not configured in this edition"
			if errMsg, _ := resp["error"].(string); errMsg != expectedErr {
				t.Errorf("expected error %q, got %q", expectedErr, errMsg)
			}

			if code, _ := resp["code"].(float64); int(code) != http.StatusNotImplemented {
				t.Errorf("expected code 501, got %v", resp["code"])
			}
		})
	}
}
