package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/observability"
	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

func TestObservabilityHandler_FleetNodeAllowlist(t *testing.T) {
	repo := newMockObservabilityRepo()
	hostRepo := newMockComputeHostRepo()

	// Register a real fleet node (worker)
	_ = hostRepo.Create(context.Background(), &docker.ComputeHost{
		ID:        "host-worker-80",
		Name:      "k8sworker",
		Endpoint:  "http://10.10.10.80:9100",
		HostType:  "agent",
		Status:    "online",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})

	handler := NewObservabilityHandler(repo, WithComputeHostRepo(hostRepo))
	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	r.Route("/slo", handler.RegisterSLORoutes)
	r.Route("/slos", handler.RegisterSLORoutes)

	t.Run("CreateSLO allows registered fleet node IP", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"service": "node-agent",
			"target":  99.9,
			"query":   "http://10.10.10.80:9100/metrics",
		})
		req := httptest.NewRequest(http.MethodPost, "/slo/definitions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code, "expected 201 Created for registered fleet node: %s", rec.Body.String())
	})

	t.Run("CreateSLO allows registered fleet node TCP", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"service": "node-agent-tcp",
			"target":  99.9,
			"query":   "tcp://10.10.10.80:9100",
		})
		req := httptest.NewRequest(http.MethodPost, "/slos", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code, "expected 201 Created for registered fleet node tcp: %s", rec.Body.String())
	})

	t.Run("CreateSLO rejects unregistered private IP", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"service": "rogue-agent",
			"target":  99.9,
			"query":   "http://10.10.10.99:9100/metrics",
		})
		req := httptest.NewRequest(http.MethodPost, "/slo/definitions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "expected 400 Bad Request for unregistered private IP")
	})

	t.Run("CreateSLO rejects metadata service even if attempted", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"service": "metadata-probe",
			"target":  99.9,
			"query":   "http://169.254.169.254/latest/meta-data",
		})
		req := httptest.NewRequest(http.MethodPost, "/slo/definitions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "expected 400 Bad Request for metadata service")
	})

	t.Run("UpdateSLO on top-level /slo/definitions/{id} allows registered fleet node", func(t *testing.T) {
		// First create an SLO definition
		def := &observability.SLODefinition{
			ID:        "slo-update-1",
			Service:   "test-update",
			Target:    99.5,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		require.NoError(t, repo.CreateSLODefinition(context.Background(), def))

		updateBody, _ := json.Marshal(map[string]interface{}{
			"query": "http://10.10.10.80:9100/metrics",
		})
		req := httptest.NewRequest(http.MethodPut, "/slo/definitions/slo-update-1", bytes.NewReader(updateBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "expected 200 OK for update with registered fleet node: %s", rec.Body.String())
	})

	t.Run("UpdateSLO on top-level /slos/{id} rejects unregistered private IP", func(t *testing.T) {
		updateBody, _ := json.Marshal(map[string]interface{}{
			"query": "http://10.10.10.99:9100/metrics",
		})
		req := httptest.NewRequest(http.MethodPut, "/slos/slo-update-1", bytes.NewReader(updateBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "expected 400 Bad Request for unregistered private IP")
	})
}
