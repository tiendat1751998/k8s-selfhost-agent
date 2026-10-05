package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

func TestComputeHost_RedactionInAPIResponses(t *testing.T) {
	hostRepo := newMockComputeHostRepo()
	origHost := &docker.ComputeHost{
		ID:       "host-sec-22",
		Name:     "worker-token-test",
		Endpoint: "http://192.168.1.10:9100",
		TenantID: "default-tenant",
		Labels: map[string]string{
			"auth_token": "super-secret-agent-token-123",
			"token":      "another-secret-token-456",
			"env":        "production",
		},
	}
	err := hostRepo.Create(context.Background(), origHost)
	require.NoError(t, err)

	handler := NewDockerHandler(nil, hostRepo)

	t.Run("ListHosts redacts token labels in response but keeps in storage", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/docker/hosts", nil)
		rec := httptest.NewRecorder()

		handler.ListHosts(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp struct {
			Data []docker.ComputeHost `json:"data"`
		}
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		require.Len(t, resp.Data, 1)

		respHost := resp.Data[0]
		assert.Equal(t, "***", respHost.Labels["auth_token"], "expected auth_token to be redacted to '***'")
		assert.Equal(t, "***", respHost.Labels["token"], "expected token to be redacted to '***'")
		assert.Equal(t, "production", respHost.Labels["env"], "expected non-token label to be preserved")

		// Verify storage still has the original raw token
		stored, err := hostRepo.GetByID(context.Background(), "host-sec-22")
		require.NoError(t, err)
		assert.Equal(t, "super-secret-agent-token-123", stored.Labels["auth_token"])
		assert.Equal(t, "another-secret-token-456", stored.Labels["token"])
	})

	t.Run("GetHost redacts token labels in response", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "host-sec-22")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/docker/hosts/host-sec-22", nil)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rec := httptest.NewRecorder()

		handler.GetHost(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var respHost docker.ComputeHost
		err := json.NewDecoder(rec.Body).Decode(&respHost)
		require.NoError(t, err)

		assert.Equal(t, "***", respHost.Labels["auth_token"], "expected auth_token to be redacted to '***'")
		assert.Equal(t, "***", respHost.Labels["token"], "expected token to be redacted to '***'")
		assert.Equal(t, "production", respHost.Labels["env"])
	})

	t.Run("CreateHost redacts token labels in response but keeps in storage", func(t *testing.T) {
		body := `{
			"name": "worker-new-create",
			"endpoint": "http://192.168.1.20:9100",
			"labels": {
				"auth_token": "raw-secret-token-789",
				"region": "us-east-1"
			}
		}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/docker/hosts", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.CreateHost(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)

		var respHost docker.ComputeHost
		err := json.NewDecoder(rec.Body).Decode(&respHost)
		require.NoError(t, err)

		assert.Equal(t, "***", respHost.Labels["auth_token"], "expected auth_token to be redacted to '***'")
		assert.Equal(t, "us-east-1", respHost.Labels["region"])

		// Check storage
		stored, err := hostRepo.GetByID(context.Background(), respHost.ID)
		require.NoError(t, err)
		assert.Equal(t, "raw-secret-token-789", stored.Labels["auth_token"])
	})

	t.Run("UpdateHost preserves existing token when updated with ***", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "host-sec-22")

		body := `{
			"name": "worker-token-test-renamed",
			"labels": {
				"auth_token": "***",
				"env": "staging"
			}
		}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/docker/hosts/host-sec-22", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rec := httptest.NewRecorder()

		handler.UpdateHost(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var respHost docker.ComputeHost
		err := json.NewDecoder(rec.Body).Decode(&respHost)
		require.NoError(t, err)

		assert.Equal(t, "***", respHost.Labels["auth_token"])
		assert.Equal(t, "staging", respHost.Labels["env"])

		// Check storage: must still hold the original token!
		stored, err := hostRepo.GetByID(context.Background(), "host-sec-22")
		require.NoError(t, err)
		assert.Equal(t, "super-secret-agent-token-123", stored.Labels["auth_token"])
	})
}

