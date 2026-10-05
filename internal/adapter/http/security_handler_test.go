package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestSecurityHandler_ScanReturns501(t *testing.T) {
	handler := NewSecurityHandler()
	r := chi.NewRouter()
	r.Route("/security", handler.RegisterRoutes)

	testCases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "TriggerScan POST /security/scan",
			method: http.MethodPost,
			path:   "/security/scan",
			body:   `{"target":"production/auth-api"}`,
		},
		{
			name:   "GetScanStatus GET /security/scan",
			method: http.MethodGet,
			path:   "/security/scan",
			body:   "",
		},
		{
			name:   "TriggerScan POST /security/scans",
			method: http.MethodPost,
			path:   "/security/scans",
			body:   `{"cluster":"local"}`,
		},
		{
			name:   "GetScanStatus GET /security/scans",
			method: http.MethodGet,
			path:   "/security/scans",
			body:   "",
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

			expectedErr := "Security scanner controller not configured in this edition"
			if errMsg, _ := resp["error"].(string); errMsg != expectedErr {
				t.Errorf("expected error %q, got %q", expectedErr, errMsg)
			}

			if code, _ := resp["code"].(float64); int(code) != http.StatusNotImplemented {
				t.Errorf("expected code 501, got %v", resp["code"])
			}
		})
	}
}
