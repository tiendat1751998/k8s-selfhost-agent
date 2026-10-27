package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func TestAgentRegister_RBACAndTokenValidation(t *testing.T) {
	repo := newMockComputeHostRepo()
	handler := NewDockerHandler(nil, repo)

	r := chi.NewRouter()
	r.Post("/agents/register", handler.RegisterAgent)

	validPayload := map[string]interface{}{
		"hostname":   "agent-node-01",
		"ip_address": "192.168.1.150",
		"port":       9100,
		"labels": map[string]string{
			"auth_token": "super-agent-secret",
			"password":   "node-password",
			"region":     "us-central",
		},
	}

	t.Run("platform_admin allowed to register", func(t *testing.T) {
		body, _ := json.Marshal(validPayload)
		req := httptest.NewRequest(http.MethodPost, "/agents/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, "platform_admin")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		hostObj, ok := resp["host"].(map[string]interface{})
		require.True(t, ok)
		labels, ok := hostObj["labels"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "***REDACTED***", labels["auth_token"])
		assert.Equal(t, "***REDACTED***", labels["password"])
		assert.Equal(t, "us-central", labels["region"])
	})

	t.Run("Non-admin roles rejected with 403 Forbidden", func(t *testing.T) {
		nonAdmins := []string{"tenant_admin", "operator", "viewer", "developer"}
		for _, role := range nonAdmins {
			body, _ := json.Marshal(validPayload)
			req := httptest.NewRequest(http.MethodPost, "/agents/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			ctx := context.WithValue(req.Context(), tenancy.UserRoleKey, role)
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusForbidden, rec.Code, "expected 403 for role %s", role)
		}
	})

	t.Run("AGENT_AUTH_TOKEN validation", func(t *testing.T) {
		origEnv := os.Getenv("AGENT_AUTH_TOKEN")
		os.Setenv("AGENT_AUTH_TOKEN", "cluster-agent-token-999")
		defer os.Setenv("AGENT_AUTH_TOKEN", origEnv)

		// 1. Missing token
		body, _ := json.Marshal(validPayload)
		req := httptest.NewRequest(http.MethodPost, "/agents/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)

		// 2. Wrong token
		req = httptest.NewRequest(http.MethodPost, "/agents/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Agent-Token", "wrong-token")
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)

		// 3. Valid token via X-Agent-Token
		payload := map[string]interface{}{
			"hostname":   "agent-node-02",
			"ip_address": "192.168.1.151",
			"port":       9100,
		}
		b2, _ := json.Marshal(payload)
		req = httptest.NewRequest(http.MethodPost, "/agents/register", bytes.NewReader(b2))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Agent-Token", "cluster-agent-token-999")
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)

		// 4. Valid token via Authorization Bearer
		payload3 := map[string]interface{}{
			"hostname":   "agent-node-03",
			"ip_address": "192.168.1.152",
			"port":       9100,
		}
		b3, _ := json.Marshal(payload3)
		req = httptest.NewRequest(http.MethodPost, "/agents/register", bytes.NewReader(b3))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer cluster-agent-token-999")
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)
	})

	t.Run("Validation rejects invalid IP and Port", func(t *testing.T) {
		// Invalid IP
		bodyBadIP, _ := json.Marshal(map[string]interface{}{
			"hostname":   "bad-ip-node",
			"ip_address": "10.0.0.1; rm -rf /",
			"port":       9100,
		})
		req := httptest.NewRequest(http.MethodPost, "/agents/register", bytes.NewReader(bodyBadIP))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)

		// Invalid Port
		bodyBadPort, _ := json.Marshal(map[string]interface{}{
			"hostname":   "bad-port-node",
			"ip_address": "192.168.1.100",
			"port":       70000,
		})
		req = httptest.NewRequest(http.MethodPost, "/agents/register", bytes.NewReader(bodyBadPort))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
