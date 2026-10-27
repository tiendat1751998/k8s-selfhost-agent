package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObservabilityHandler_CustomSLO_SSRF_Rejection(t *testing.T) {
	repo := newMockObservabilityRepo()
	handler := NewObservabilityHandler(repo)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	blockedTargets := []struct {
		name  string
		query string
	}{
		{"loopback IPv4 HTTP", "http://127.0.0.1:8080/health"},
		{"loopback IPv4 with zero subnet", "http://127.0.0.2:8080/metrics"},
		{"loopback localhost HTTP", "http://localhost:8080/health"},
		{"cloud metadata IPv4", "http://169.254.169.254/latest/meta-data"},
		{"link local unicast", "http://169.254.1.1:9090/health"},
		{"RFC1918 class A", "http://10.10.10.60:9100/health"},
		{"RFC1918 class B", "http://172.16.0.5:8080/ping"},
		{"RFC1918 class C", "http://192.168.1.100:80/status"},
		{"unspecified IPv4", "http://0.0.0.0:8080/health"},
		{"loopback TCP", "tcp://127.0.0.1:6443"},
		{"private TCP", "tcp://10.0.0.1:5432"},
		{"cloud metadata TCP", "tcp://169.254.169.254:80"},
	}

	for _, tc := range blockedTargets {
		t.Run("CreateSLO rejects "+tc.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{
				"service": "test-service",
				"target":  99.9,
				"query":   tc.query,
			})
			req := httptest.NewRequest(http.MethodPost, "/slo/definitions", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusBadRequest, rec.Code, "expected 400 Bad Request for %s", tc.query)
		})

		t.Run("CreateSLO on /slos alias rejects "+tc.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{
				"service": "test-service",
				"target":  99.9,
				"query":   tc.query,
			})
			req := httptest.NewRequest(http.MethodPost, "/slos", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusBadRequest, rec.Code, "expected 400 Bad Request on /slos for %s", tc.query)
		})
	}

	t.Run("Non-URL queries like PromQL expressions are accepted", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"service": "metric-service",
			"target":  99.5,
			"query":   `rate(http_requests_total{status="500"}[5m]) == 0`,
		})
		req := httptest.NewRequest(http.MethodPost, "/slo/definitions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)
	})
}
