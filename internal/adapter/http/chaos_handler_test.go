package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestChaosHandler_MutationsReturn501(t *testing.T) {
	handler := NewChaosHandler()
	r := chi.NewRouter()
	r.Route("/chaos", handler.RegisterRoutes)

	testCases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "CreateExperiment POST /chaos/experiments",
			method: http.MethodPost,
			path:   "/chaos/experiments",
			body:   `{"name":"pod-kill-test","action":"pod-failure","target":"auth-service"}`,
		},
		{
			name:   "ListExperiments GET /chaos/experiments",
			method: http.MethodGet,
			path:   "/chaos/experiments",
			body:   "",
		},
		{
			name:   "SimulateFailure POST /chaos/simulate",
			method: http.MethodPost,
			path:   "/chaos/simulate",
			body:   `{"scenario":"network-latency","delay_ms":500}`,
		},
		{
			name:   "InjectFault POST /chaos/inject",
			method: http.MethodPost,
			path:   "/chaos/inject",
			body:   `{"fault":"cpu-stress","duration":"30s"}`,
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

			expectedErr := "Chaos engineering controller not configured in this edition"
			if errMsg, _ := resp["error"].(string); errMsg != expectedErr {
				t.Errorf("expected error %q, got %q", expectedErr, errMsg)
			}

			if code, _ := resp["code"].(float64); int(code) != http.StatusNotImplemented {
				t.Errorf("expected code 501, got %v", resp["code"])
			}
		})
	}
}
