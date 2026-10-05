package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestNetworkHandler_PolicyMutationReturns501(t *testing.T) {
	handler := NewNetworkHandler()
	r := chi.NewRouter()
	r.Route("/network", handler.RegisterRoutes)

	testCases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "CreatePolicy POST /network/policies",
			method: http.MethodPost,
			path:   "/network/policies",
			body:   `{"name":"isolate-db","namespace":"prod"}`,
		},
		{
			name:   "UpdatePolicy PUT /network/policies/isolate-db",
			method: http.MethodPut,
			path:   "/network/policies/isolate-db",
			body:   `{"name":"isolate-db","namespace":"prod"}`,
		},
		{
			name:   "DeletePolicy DELETE /network/policies/isolate-db",
			method: http.MethodDelete,
			path:   "/network/policies/isolate-db",
			body:   "",
		},
		{
			name:   "MutatePolicy POST /network/mutate",
			method: http.MethodPost,
			path:   "/network/mutate",
			body:   `{"action":"deny-all","namespace":"default"}`,
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

			expectedErr := "Network policy controller not configured in this edition"
			if errMsg, _ := resp["error"].(string); errMsg != expectedErr {
				t.Errorf("expected error %q, got %q", expectedErr, errMsg)
			}

			if code, _ := resp["code"].(float64); int(code) != http.StatusNotImplemented {
				t.Errorf("expected code 501, got %v", resp["code"])
			}
		})
	}
}
